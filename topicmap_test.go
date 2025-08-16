package main

import (
	"sync"
	"testing"
	"time"
)

func TestMultipleConcurrentWritersOnSameTopic(t *testing.T) {
	// Test that multiple writers can connect to the same topic
	// and all receive broadcasts without premature disconnection.
	testKey := "test-topic-concurrent"

	// Reset the TopicMap to ensure clean state.
	TopicMap = TopicMapStruct{m: make(map[string]*TopicMapEntry)}

	// Create multiple writer channels.
	numWriters := 3
	writerChannels := make([]chan interface{}, numWriters)
	var wg sync.WaitGroup

	// Track if any writer got unexpectedly closed.
	closedChannels := make([]bool, numWriters)

	// Register multiple writers on the same topic.
	for i := 0; i < numWriters; i++ {
		writerChannels[i] = TopicMap.Register(testKey)
		t.Logf("Registered writer %d", i)
	}

	// Verify topic exists and has correct count.
	TopicMap.RLock()
	if entry, ok := TopicMap.m[testKey]; ok {
		t.Logf("Topic exists with count: %d", entry.count)
	} else {
		t.Logf("Topic does not exist after registration!")
	}
	TopicMap.RUnlock()

	// Defer cleanup of all channels at once.
	defer func() {
		for i := 0; i < numWriters; i++ {
			TopicMap.Unregister(testKey, writerChannels[i])
		}
	}()

	// Start goroutines to listen on each writer channel.
	readyChannels := make([]chan bool, numWriters)
	for i := 0; i < numWriters; i++ {
		readyChannels[i] = make(chan bool)
		wg.Add(1)
		go func(idx int, ch chan interface{}, ready chan bool) {
			defer wg.Done()

			// Signal that we're ready to receive.
			ready <- true

			messageCount := 0
			for {
				select {
				case msg, ok := <-ch:
					if !ok {
						// Channel was closed unexpectedly.
						closedChannels[idx] = true
						t.Logf("Writer %d channel closed after %d messages", idx, messageCount)
						return
					}
					messageCount++
					t.Logf("Writer %d received message: %v", idx, msg)

					// Exit after receiving expected messages.
					if messageCount >= 3 {
						return
					}
				case <-time.After(2 * time.Second):
					// Timeout waiting for messages.
					t.Logf("Writer %d timed out after %d messages", idx, messageCount)
					return
				}
			}
		}(i, writerChannels[i], readyChannels[i])
	}

	// Wait for all goroutines to be ready.
	for i := 0; i < numWriters; i++ {
		<-readyChannels[i]
	}
	t.Logf("All writers ready to receive messages")

	// Small delay to ensure goroutines are in select loop.
	time.Sleep(10 * time.Millisecond)

	// Send messages through the cache bus to trigger broadcasts.
	messages := []string{"message1", "message2", "message3"}
	for _, msg := range messages {
		// Simulate the cache broadcast mechanism.
		TopicMap.RLock()
		if currentTopic, ok := TopicMap.m[testKey]; ok {
			currentTopic.t.Broadcast <- msg
		} else {
			t.Errorf("Topic %s not found in map when it should exist", testKey)
		}
		TopicMap.RUnlock()

		// Small delay between messages.
		time.Sleep(100 * time.Millisecond)
	}

	// Wait for all goroutines to finish.
	wg.Wait()

	// Check if any channels were closed unexpectedly.
	for i, closed := range closedChannels {
		if closed {
			t.Errorf("Writer %d channel was closed unexpectedly", i)
		}
	}
}

func TestConcurrentRegisterUnregister(t *testing.T) {
	// Test that concurrent registrations and unregistrations
	// properly maintain the connection count.
	testKey := "test-topic-register-unregister"

	var wg sync.WaitGroup
	numOperations := 10

	// Perform concurrent register/unregister operations.
	for i := 0; i < numOperations; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// Register a channel.
			ch := TopicMap.Register(testKey)

			// Simulate some work.
			time.Sleep(10 * time.Millisecond)

			// Send a test message.
			TopicMap.RLock()
			if currentTopic, ok := TopicMap.m[testKey]; ok {
				select {
				case currentTopic.t.Broadcast <- idx:
					// Message sent successfully.
				default:
					// Broadcast channel might be full.
				}
			}
			TopicMap.RUnlock()

			// Unregister the channel.
			TopicMap.Unregister(testKey, ch)
		}(i)
	}

	// Wait for all operations to complete.
	wg.Wait()

	// Verify the topic has been cleaned up.
	TopicMap.RLock()
	_, exists := TopicMap.m[testKey]
	TopicMap.RUnlock()

	if exists {
		t.Errorf("Topic %s still exists in map after all unregistrations", testKey)
	}
}

func TestTopicCountTracking(t *testing.T) {
	// Test that the count field correctly tracks the number of registered channels.
	testKey := "test-topic-count"

	// Register first writer.
	ch1 := TopicMap.Register(testKey)

	// Check count should be 1.
	TopicMap.RLock()
	entry1, ok := TopicMap.m[testKey]
	TopicMap.RUnlock()
	if !ok {
		t.Fatal("Topic not found after first registration")
	}
	if entry1.count != 1 {
		t.Errorf("Expected count 1 after first registration, got %d", entry1.count)
	}

	// Register second writer.
	ch2 := TopicMap.Register(testKey)

	// Check count should be 2.
	TopicMap.RLock()
	entry2, ok := TopicMap.m[testKey]
	TopicMap.RUnlock()
	if !ok {
		t.Fatal("Topic not found after second registration")
	}
	if entry2.count != 2 {
		t.Errorf("Expected count 2 after second registration, got %d", entry2.count)
	}

	// Register third writer.
	ch3 := TopicMap.Register(testKey)

	// Check count should be 3.
	TopicMap.RLock()
	entry3, ok := TopicMap.m[testKey]
	TopicMap.RUnlock()
	if !ok {
		t.Fatal("Topic not found after third registration")
	}
	if entry3.count != 3 {
		t.Errorf("Expected count 3 after third registration, got %d", entry3.count)
	}

	// Unregister first writer.
	TopicMap.Unregister(testKey, ch1)

	// Check count should be 2.
	TopicMap.RLock()
	entry4, ok := TopicMap.m[testKey]
	TopicMap.RUnlock()
	if !ok {
		t.Fatal("Topic should still exist after first unregistration")
	}
	if entry4.count != 2 {
		t.Errorf("Expected count 2 after first unregistration, got %d", entry4.count)
	}

	// Unregister second writer.
	TopicMap.Unregister(testKey, ch2)

	// Check count should be 1.
	TopicMap.RLock()
	entry5, ok := TopicMap.m[testKey]
	TopicMap.RUnlock()
	if !ok {
		t.Fatal("Topic should still exist after second unregistration")
	}
	if entry5.count != 1 {
		t.Errorf("Expected count 1 after second unregistration, got %d", entry5.count)
	}

	// Unregister third writer.
	TopicMap.Unregister(testKey, ch3)

	// Topic should be removed from map.
	TopicMap.RLock()
	_, exists := TopicMap.m[testKey]
	TopicMap.RUnlock()
	if exists {
		t.Error("Topic should be removed from map after all unregistrations")
	}
}
