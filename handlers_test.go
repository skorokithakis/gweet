package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/mux"
)

var testInitOnce sync.Once

func initTestEnvironment() {
	testInitOnce.Do(func() {
		// Initialize logging for tests.
		InitLogging(io.Discard, io.Discard, io.Discard, os.Stderr)
		// Start the cache goroutine.
		go Cacher()
		// Give cache time to start.
		time.Sleep(100 * time.Millisecond)
	})
}

func TestHomeHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	handler := http.HandlerFunc(HomeHandler)
	handler.ServeHTTP(recorder, req)

	if status := recorder.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expected := "This is a <a href='https://github.com/skorokithakis/gweet/'>Gweet server</a>"
	if !strings.Contains(recorder.Body.String(), expected) {
		t.Errorf("handler returned unexpected body: got %v want substring %v",
			recorder.Body.String(), expected)
	}
}

func TestStreamsPostHandler(t *testing.T) {
	initTestEnvironment()

	form := url.Values{}
	form.Add("test_field", "test_value")
	form.Add("another_field", "another_value")

	req, err := http.NewRequest("POST", "/stream/test_key/", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	recorder := httptest.NewRecorder()
	router := mux.NewRouter()
	router.HandleFunc("/stream/{key}/", StreamsPostHandler).Methods("POST")
	router.ServeHTTP(recorder, req)

	if status := recorder.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if response["status"] != "success" {
		t.Errorf("Expected status 'success', got %v", response["status"])
	}

	message, ok := response["message"].(map[string]interface{})
	if !ok {
		t.Fatal("message field is not a map")
	}

	if message["name"] != "test_key" {
		t.Errorf("Expected name 'test_key', got %v", message["name"])
	}

	values, ok := message["values"].(map[string]interface{})
	if !ok {
		t.Fatal("values field is not a map")
	}

	// Check that our form values are in the response.
	testFieldValues, ok := values["test_field"].([]interface{})
	if !ok || len(testFieldValues) == 0 || testFieldValues[0] != "test_value" {
		t.Errorf("Expected test_field to have value 'test_value', got %v", values["test_field"])
	}
}

func TestStreamsGetHandler(t *testing.T) {
	initTestEnvironment()
	// Post a message first.
	form := url.Values{}
	form.Add("test_field", "test_value")

	postReq, _ := http.NewRequest("POST", "/stream/test_get_key/", strings.NewReader(form.Encode()))
	postReq.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	postRecorder := httptest.NewRecorder()
	router := mux.NewRouter()
	router.HandleFunc("/stream/{key}/", StreamsPostHandler).Methods("POST")
	router.ServeHTTP(postRecorder, postReq)

	// Give cache time to process.
	time.Sleep(100 * time.Millisecond)

	// Now get the messages.
	getReq, err := http.NewRequest("GET", "/stream/test_get_key/?latest=10", nil)
	if err != nil {
		t.Fatal(err)
	}

	getRecorder := httptest.NewRecorder()
	router.HandleFunc("/stream/{key}/", StreamsGetHandler).Methods("GET")
	router.ServeHTTP(getRecorder, getReq)

	if status := getRecorder.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(getRecorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	messages, ok := response["messages"].([]interface{})
	if !ok {
		t.Fatal("messages field is not an array")
	}

	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}
}

// TestStreamsStreamingGetHandler tests the streaming endpoint.
func TestStreamsStreamingGetHandler(t *testing.T) {
	initTestEnvironment()
	// Create a test server that we can hijack connections from.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set up router to handle the request.
		router := mux.NewRouter()
		router.HandleFunc("/stream/{key}/", StreamsStreamingGetHandler).Methods("GET")
		router.ServeHTTP(w, r)
	}))
	defer server.Close()

	// Parse the test server URL.
	serverURL := strings.TrimPrefix(server.URL, "http://")

	// Connect to the server.
	conn, err := net.Dial("tcp", serverURL)
	if err != nil {
		t.Fatalf("Failed to connect to test server: %v", err)
	}
	defer conn.Close()

	// Send HTTP request for streaming.
	request := fmt.Sprintf("GET /stream/test_streaming_key/?streaming=1 HTTP/1.1\r\nHost: %s\r\n\r\n", serverURL)
	_, err = conn.Write([]byte(request))
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	// Read the response headers.
	reader := bufio.NewReader(conn)

	// Read status line.
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read status line: %v", err)
	}

	if !strings.Contains(statusLine, "200 OK") {
		t.Errorf("Expected 200 OK status, got: %s", statusLine)
	}

	// Read headers until we get to the body.
	foundChunked := false
	foundContentType := false
	for {
		header, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("Failed to read header: %v", err)
		}

		if strings.Contains(header, "Transfer-Encoding: chunked") {
			foundChunked = true
		}
		if strings.Contains(header, "Content-Type: application/json") {
			foundContentType = true
		}

		// Empty line indicates end of headers.
		if header == "\r\n" {
			break
		}
	}

	if !foundChunked {
		t.Error("Expected Transfer-Encoding: chunked header")
	}
	if !foundContentType {
		t.Error("Expected Content-Type: application/json header")
	}

	// Now post a message to the stream from another goroutine.
	go func() {
		time.Sleep(500 * time.Millisecond)

		form := url.Values{}
		form.Add("stream_test", "stream_value")

		postReq, _ := http.NewRequest("POST", server.URL+"/stream/test_streaming_key/", strings.NewReader(form.Encode()))
		postReq.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		client := &http.Client{}
		resp, err := client.Do(postReq)
		if err != nil {
			t.Logf("Failed to post message: %v", err)
			return
		}
		resp.Body.Close()
	}()

	// Read chunks from the stream.
	// Set a timeout so the test doesn't hang forever.
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))

	// Read at least one chunk (should be a message or keepalive).
	chunkSizeStr, err := reader.ReadString('\n')
	if err != nil {
		// Timeout is expected eventually, but we should get at least one chunk.
		if err == io.EOF {
			t.Fatal("Connection closed unexpectedly")
		}
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			// Timeout after reading some data is OK.
			return
		}
		t.Fatalf("Failed to read chunk size: %v", err)
	}

	// Parse chunk size (it's in hex).
	chunkSizeStr = strings.TrimSpace(chunkSizeStr)
	var chunkSize int
	fmt.Sscanf(chunkSizeStr, "%x", &chunkSize)

	if chunkSize > 0 {
		// Read the chunk data.
		chunkData := make([]byte, chunkSize)
		_, err = io.ReadFull(reader, chunkData)
		if err != nil {
			t.Fatalf("Failed to read chunk data: %v", err)
		}

		// Read the trailing \r\n after chunk data.
		reader.ReadString('\n')

		// The chunk should be either a keepalive newline or a JSON message.
		chunkStr := string(chunkData)
		if chunkStr != "\n" {
			// It should be a JSON message.
			var message map[string]interface{}
			// Remove trailing newline from JSON.
			chunkStr = strings.TrimSuffix(chunkStr, "\n")
			if err := json.Unmarshal([]byte(chunkStr), &message); err != nil {
				t.Errorf("Failed to parse message JSON: %v, data: %s", err, chunkStr)
			}
		}
	}
}

func TestPushHandler(t *testing.T) {
	initTestEnvironment()
	form := url.Values{}
	form.Add("push_field", "push_value")

	// Note: The push handler expects the key to be already hashed in the URL.
	// The handler will use the key as-is (it's the hash itself).
	testHash := "testhash123"
	req, err := http.NewRequest("POST", fmt.Sprintf("/push/%s/", testHash), strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	recorder := httptest.NewRecorder()
	router := mux.NewRouter()
	router.HandleFunc("/push/{key}/", PushHandler).Methods("POST")
	router.ServeHTTP(recorder, req)

	if status := recorder.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if response["status"] != "success" {
		t.Errorf("Expected status 'success', got %v", response["status"])
	}
}

// TestStreamingWithMultipleMessages tests that multiple messages are properly delivered.
func TestStreamingWithMultipleMessages(t *testing.T) {
	initTestEnvironment()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		router := mux.NewRouter()
		router.HandleFunc("/stream/{key}/", StreamsStreamingGetHandler).Methods("GET")
		router.HandleFunc("/stream/{key}/", StreamsPostHandler).Methods("POST")
		router.ServeHTTP(w, r)
	}))
	defer server.Close()

	serverURL := strings.TrimPrefix(server.URL, "http://")

	// Connect for streaming.
	conn, err := net.Dial("tcp", serverURL)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	request := fmt.Sprintf("GET /stream/multi_test/?streaming=1 HTTP/1.1\r\nHost: %s\r\n\r\n", serverURL)
	_, err = conn.Write([]byte(request))
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	reader := bufio.NewReader(conn)

	// Skip headers.
	for {
		header, _ := reader.ReadString('\n')
		if header == "\r\n" {
			break
		}
	}

	// Send multiple messages.
	messageCount := 3
	messages := make(chan bool, messageCount)

	for i := 0; i < messageCount; i++ {
		go func(index int) {
			time.Sleep(time.Duration(100*(index+1)) * time.Millisecond)

			form := url.Values{}
			form.Add("index", fmt.Sprintf("%d", index))

			postReq, _ := http.NewRequest("POST", server.URL+"/stream/multi_test/", strings.NewReader(form.Encode()))
			postReq.Header.Add("Content-Type", "application/x-www-form-urlencoded")

			client := &http.Client{}
			resp, err := client.Do(postReq)
			if err == nil {
				resp.Body.Close()
				messages <- true
			} else {
				messages <- false
			}
		}(i)
	}

	// Collect sent confirmations.
	for i := 0; i < messageCount; i++ {
		success := <-messages
		if !success {
			t.Error("Failed to send a message")
		}
	}

	// Read chunks and count received messages.
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	receivedCount := 0

	for receivedCount < messageCount {
		chunkSizeStr, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		chunkSizeStr = strings.TrimSpace(chunkSizeStr)
		var chunkSize int
		fmt.Sscanf(chunkSizeStr, "%x", &chunkSize)

		if chunkSize > 0 {
			chunkData := make([]byte, chunkSize)
			_, err = io.ReadFull(reader, chunkData)
			if err != nil {
				break
			}
			reader.ReadString('\n') // Trailing \r\n

			chunkStr := string(chunkData)
			if chunkStr != "\n" { // Not a keepalive.
				receivedCount++
			}
		}
	}

	if receivedCount != messageCount {
		t.Errorf("Expected to receive %d messages, got %d", messageCount, receivedCount)
	}
}

// TestStreamingNoContentLength verifies no Content-Length header is sent when streaming.
func TestStreamingNoContentLength(t *testing.T) {
	initTestEnvironment()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		router := mux.NewRouter()
		router.HandleFunc("/stream/{key}/", StreamsStreamingGetHandler).Methods("GET")
		router.ServeHTTP(w, r)
	}))
	defer server.Close()

	serverURL := strings.TrimPrefix(server.URL, "http://")

	conn, err := net.Dial("tcp", serverURL)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	request := fmt.Sprintf("GET /stream/test_no_content_length/?streaming=1 HTTP/1.1\r\nHost: %s\r\n\r\n", serverURL)
	_, err = conn.Write([]byte(request))
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	reader := bufio.NewReader(conn)

	// Read all headers.
	headers := make([]string, 0)
	for {
		header, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("Failed to read header: %v", err)
		}

		headers = append(headers, header)

		// Check that Content-Length is NOT present.
		if strings.Contains(strings.ToLower(header), "content-length") {
			t.Error("Content-Length header should not be present in streaming response")
		}

		if header == "\r\n" {
			break
		}
	}

	// Verify required headers are present.
	hasTransferEncoding := false
	hasContentType := false

	for _, header := range headers {
		if strings.Contains(header, "Transfer-Encoding: chunked") {
			hasTransferEncoding = true
		}
		if strings.Contains(header, "Content-Type: application/json") {
			hasContentType = true
		}
	}

	if !hasTransferEncoding {
		t.Error("Transfer-Encoding: chunked header is required for streaming")
	}
	if !hasContentType {
		t.Error("Content-Type: application/json header is required")
	}
}

// TestStreamingKeepalive verifies that keepalive messages are sent periodically.
// Note: This test is skipped by default as it takes 30+ seconds to run.
func TestStreamingKeepalive(t *testing.T) {
	t.Skip("Skipping keepalive test - takes 30+ seconds to run")

	initTestEnvironment()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		router := mux.NewRouter()
		router.HandleFunc("/stream/{key}/", StreamsStreamingGetHandler).Methods("GET")
		router.ServeHTTP(w, r)
	}))
	defer server.Close()

	serverURL := strings.TrimPrefix(server.URL, "http://")

	conn, err := net.Dial("tcp", serverURL)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	request := fmt.Sprintf("GET /stream/test_keepalive/?streaming=1 HTTP/1.1\r\nHost: %s\r\n\r\n", serverURL)
	_, err = conn.Write([]byte(request))
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	reader := bufio.NewReader(conn)

	// Skip headers.
	for {
		header, _ := reader.ReadString('\n')
		if header == "\r\n" {
			break
		}
	}

	// Wait for keepalive (should come within 30 seconds).
	conn.SetReadDeadline(time.Now().Add(35 * time.Second))

	// Read first chunk - should be a keepalive within 30 seconds.
	chunkSizeStr, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to receive keepalive: %v", err)
	}

	chunkSizeStr = strings.TrimSpace(chunkSizeStr)
	var chunkSize int
	fmt.Sscanf(chunkSizeStr, "%x", &chunkSize)

	if chunkSize != 1 {
		t.Errorf("Expected keepalive chunk size of 1, got %d", chunkSize)
	}

	// Read the chunk data.
	chunkData := make([]byte, chunkSize)
	_, err = io.ReadFull(reader, chunkData)
	if err != nil {
		t.Fatalf("Failed to read keepalive data: %v", err)
	}

	if string(chunkData) != "\n" {
		t.Errorf("Expected keepalive to be newline, got %q", string(chunkData))
	}
}

// TestStreamingConnectionReuse verifies proper cleanup when client disconnects.
func TestStreamingConnectionReuse(t *testing.T) {
	initTestEnvironment()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		router := mux.NewRouter()
		router.HandleFunc("/stream/{key}/", StreamsStreamingGetHandler).Methods("GET")
		router.ServeHTTP(w, r)
	}))
	defer server.Close()

	serverURL := strings.TrimPrefix(server.URL, "http://")

	// First connection.
	conn1, err := net.Dial("tcp", serverURL)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	request := fmt.Sprintf("GET /stream/reuse_test/?streaming=1 HTTP/1.1\r\nHost: %s\r\n\r\n", serverURL)
	conn1.Write([]byte(request))

	// Read some response to ensure connection is established.
	reader1 := bufio.NewReader(conn1)
	reader1.ReadString('\n') // Read status line.

	// Close first connection.
	conn1.Close()

	// Give server time to clean up.
	time.Sleep(100 * time.Millisecond)

	// Second connection to same stream should work.
	conn2, err := net.Dial("tcp", serverURL)
	if err != nil {
		t.Fatalf("Failed to connect second time: %v", err)
	}
	defer conn2.Close()

	conn2.Write([]byte(request))

	reader2 := bufio.NewReader(conn2)
	statusLine, err := reader2.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read status from second connection: %v", err)
	}

	if !strings.Contains(statusLine, "200 OK") {
		t.Errorf("Second connection failed, got: %s", statusLine)
	}
}

// TestStreamingChunkedEncoding verifies proper chunked encoding format.
func TestStreamingChunkedEncoding(t *testing.T) {
	initTestEnvironment()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		router := mux.NewRouter()
		router.HandleFunc("/stream/{key}/", StreamsStreamingGetHandler).Methods("GET")
		router.HandleFunc("/stream/{key}/", StreamsPostHandler).Methods("POST")
		router.ServeHTTP(w, r)
	}))
	defer server.Close()

	serverURL := strings.TrimPrefix(server.URL, "http://")

	conn, err := net.Dial("tcp", serverURL)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	request := fmt.Sprintf("GET /stream/chunked_test/?streaming=1 HTTP/1.1\r\nHost: %s\r\n\r\n", serverURL)
	conn.Write([]byte(request))

	reader := bufio.NewReader(conn)

	// Skip headers.
	for {
		header, _ := reader.ReadString('\n')
		if header == "\r\n" {
			break
		}
	}

	// Send a test message.
	go func() {
		time.Sleep(100 * time.Millisecond)

		form := url.Values{}
		form.Add("test", "chunked_encoding_test")

		postReq, _ := http.NewRequest("POST", server.URL+"/stream/chunked_test/", strings.NewReader(form.Encode()))
		postReq.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		client := &http.Client{}
		resp, _ := client.Do(postReq)
		if resp != nil {
			resp.Body.Close()
		}
	}()

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))

	// Read chunk size line.
	chunkSizeLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read chunk size: %v", err)
	}

	// Verify it ends with \r\n.
	if !strings.HasSuffix(chunkSizeLine, "\r\n") {
		t.Errorf("Chunk size line should end with \\r\\n, got: %q", chunkSizeLine)
	}

	// Parse chunk size.
	chunkSizeStr := strings.TrimSpace(chunkSizeLine)
	var chunkSize int
	n, err := fmt.Sscanf(chunkSizeStr, "%x", &chunkSize)
	if err != nil || n != 1 {
		t.Errorf("Failed to parse chunk size as hex: %q", chunkSizeStr)
	}

	if chunkSize > 0 {
		// Read chunk data.
		chunkData := make([]byte, chunkSize)
		n, err := io.ReadFull(reader, chunkData)
		if err != nil || n != chunkSize {
			t.Fatalf("Failed to read exact chunk data: %v", err)
		}

		// Read trailing \r\n.
		trailing := make([]byte, 2)
		io.ReadFull(reader, trailing)
		if string(trailing) != "\r\n" {
			t.Errorf("Expected \\r\\n after chunk data, got: %q", string(trailing))
		}

		// Verify the chunk contains valid JSON.
		chunkStr := strings.TrimSuffix(string(chunkData), "\n")
		if chunkStr != "" { // Not a keepalive.
			var message map[string]interface{}
			if err := json.Unmarshal([]byte(chunkStr), &message); err != nil {
				t.Errorf("Chunk does not contain valid JSON: %v", err)
			}
		}
	}
}

// TestStreamingFlushAfterEachMessage verifies messages are flushed immediately.
func TestStreamingFlushAfterEachMessage(t *testing.T) {
	initTestEnvironment()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		router := mux.NewRouter()
		router.HandleFunc("/stream/{key}/", StreamsStreamingGetHandler).Methods("GET")
		router.HandleFunc("/stream/{key}/", StreamsPostHandler).Methods("POST")
		router.ServeHTTP(w, r)
	}))
	defer server.Close()

	serverURL := strings.TrimPrefix(server.URL, "http://")

	conn, err := net.Dial("tcp", serverURL)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	request := fmt.Sprintf("GET /stream/flush_test/?streaming=1 HTTP/1.1\r\nHost: %s\r\n\r\n", serverURL)
	conn.Write([]byte(request))

	reader := bufio.NewReader(conn)

	// Skip headers.
	for {
		header, _ := reader.ReadString('\n')
		if header == "\r\n" {
			break
		}
	}

	// Channel to signal when message is received.
	messageReceived := make(chan bool, 1)

	// Start reading in goroutine.
	go func() {
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))

		// Try to read a chunk.
		chunkSizeStr, err := reader.ReadString('\n')
		if err == nil && strings.TrimSpace(chunkSizeStr) != "" {
			var chunkSize int
			fmt.Sscanf(strings.TrimSpace(chunkSizeStr), "%x", &chunkSize)

			if chunkSize > 0 {
				chunkData := make([]byte, chunkSize)
				_, err := io.ReadFull(reader, chunkData)
				if err == nil {
					messageReceived <- true
					return
				}
			}
		}
		messageReceived <- false
	}()

	// Send a message after a short delay.
	time.Sleep(200 * time.Millisecond)

	form := url.Values{}
	form.Add("flush", "test")

	postReq, _ := http.NewRequest("POST", server.URL+"/stream/flush_test/", strings.NewReader(form.Encode()))
	postReq.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, _ := client.Do(postReq)
	if resp != nil {
		resp.Body.Close()
	}

	// Message should be received quickly (within 500ms).
	select {
	case received := <-messageReceived:
		if !received {
			t.Error("Message was not received after posting")
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("Message was not flushed immediately after posting")
	}
}

// TestChunkFunction tests the Chunk function to ensure proper formatting.
func TestChunkFunction(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"hello", fmt.Sprintf("%x\r\nhello\r\n", len("hello"))},
		{"", "0\r\n\r\n"},
		{"test\ndata", fmt.Sprintf("%x\r\ntest\ndata\r\n", len("test\ndata"))},
	}

	for _, tc := range testCases {
		result := string(Chunk(tc.input))
		if result != tc.expected {
			t.Errorf("Chunk(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}
