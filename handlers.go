package main

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/pmylund/go-cache"
)

var c = cache.New(ItemLifetime, 5*time.Minute)

func makeMessage(key string, values *url.Values) interface{} {
	message := make(map[string]interface{})
	message["name"] = key
	message["values"] = values
	message["created"] = time.Now().Format(time.RFC3339Nano)
	return message
}

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Please read the documentation on how to talk to this server.")
}

func StreamsStreamingGetHandler(w http.ResponseWriter, r *http.Request) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Webserver doesn't support hijacking.", http.StatusInternalServerError)
		return
	}
	conn, bufrw, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Don't forget to close the connection:
	defer conn.Close()
	defer conn.Write(Chunk(""))

	fmt.Fprintf(bufrw, "HTTP/1.1 200 OK\r\n")
	fmt.Fprintf(bufrw, "Transfer-Encoding: chunked\r\n")
	fmt.Fprintf(bufrw, "Content-Type: application/json\r\n\r\n")
	bufrw.Flush()

	key := mux.Vars(r)["key"]
	messageBus := TopicMap.Register(key)
	defer TopicMap.Unregister(key, messageBus)

	// Keepalive ticker
	ticker := time.Tick(30 * time.Second)
	for {
		var err error
		select {
		case message, ok := <-messageBus:
			if !ok {
				return
			}
			assertedMessage := message.(map[string]interface{})
			_, err = conn.Write(Chunk(JSONToString(assertedMessage) + "\n"))
		case _ = <-ticker:
			// Send the keepalive.
			_, err = conn.Write(Chunk("\n"))
		}

		// An error means the connection was closed, return.
		if err != nil {
			return
		}
	}
}

func StreamsGetHandler(w http.ResponseWriter, r *http.Request) {
	key := mux.Vars(r)["key"]

	latest, err := strconv.Atoi(r.FormValue("latest"))
	if err != nil || latest <= 0 || latest >= MaxQueueLength {
		latest = MaxQueueLength
	}

	// Get the messages from the cache.
	backchan := make(chan []interface{}, 1)
	CacheBus <- CacheMessage{0, backchan, key}
	messages := <-backchan

	w.Header().Set("Content-Type", "application/json")
	lowerBound := int(math.Max(0, float64(len(messages)-latest)))
	upperBound := len(messages)
	fmt.Fprint(w, JSONResponse{"messages": messages[lowerBound:upperBound]})
}

func StreamsPostHandler(w http.ResponseWriter, r *http.Request) {
	key := mux.Vars(r)["key"]

	r.ParseForm()
	message := makeMessage(key, &r.Form)

	// Write the message to the cache.
	CacheBus <- CacheMessage{1, message, key}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, JSONResponse{"status": "success", "message": message})
}
