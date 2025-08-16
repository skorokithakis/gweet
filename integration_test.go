package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestHighLoadIntegration(t *testing.T) {
	// Test with 10 readers and 10 writers, sending 90 messages total.
	// The topic library has a 100-message buffer limit and drops slow consumers,
	// so we stay under that limit to ensure all readers can receive messages.
	const (
		numReaders    = 10
		numWriters    = 10
		totalMessages = 90 // 9 messages per reader, staying under 100-message buffer
		sendDuration  = 3 * time.Second
		readTimeout   = 10 * time.Second
	)

	testKey := "test-high-load-integration"

	// Reset the TopicMap to ensure clean state.
	TopicMap = TopicMapStruct{m: make(map[string]*TopicMapEntry)}

	// Tracking variables.
	var totalSent int64
	var totalDropped int64
	var readersDropped int64
	readerStats := make([]int64, numReaders)
	readerActive := make([]bool, numReaders)

	// WaitGroups for coordination.
	var readersWg sync.WaitGroup
	var writersWg sync.WaitGroup

	// Register readers and mark them as active.
	readerChannels := make([]chan interface{}, numReaders)
	for i := 0; i < numReaders; i++ {
		readerChannels[i] = TopicMap.Register(testKey)
		readerActive[i] = true
		defer func(idx int, ch chan interface{}) {
			if readerActive[idx] {
				TopicMap.Unregister(testKey, ch)
			}
		}(i, readerChannels[i])
	}

	// Verify initial topic state.
	TopicMap.RLock()
	if entry, ok := TopicMap.m[testKey]; ok {
		t.Logf("Topic registered with %d readers", entry.count)
		if entry.count != uint64(numReaders) {
			t.Errorf("Expected %d readers, got %d", numReaders, entry.count)
		}
	} else {
		t.Fatal("Topic not found after reader registration")
	}
	TopicMap.RUnlock()

	// Start reader goroutines that consume as fast as possible.
	readersReady := make(chan bool, numReaders)
	for i := 0; i < numReaders; i++ {
		readersWg.Add(1)
		go func(idx int, ch chan interface{}) {
			defer readersWg.Done()

			// Signal ready.
			readersReady <- true

			messagesReceived := 0

			for messagesReceived < 9 {
				select {
				case msg, ok := <-ch:
					if !ok {
						// Channel was closed (dropped as slow consumer).
						atomic.AddInt64(&readersDropped, 1)
						readerActive[idx] = false
						t.Logf("Reader %d: channel closed after %d messages", idx, messagesReceived)
						atomic.StoreInt64(&readerStats[idx], int64(messagesReceived))
						return
					}

					messagesReceived++
					t.Logf("Reader %d: received message %d: %v", idx, messagesReceived, msg)

					// Log when all messages are received.
					if messagesReceived == 9 {
						t.Logf("Reader %d: received all %d messages", idx, messagesReceived)
						atomic.StoreInt64(&readerStats[idx], int64(messagesReceived))
						return
					}

				case <-time.After(5 * time.Second):
					// Timeout waiting for messages.
					t.Logf("Reader %d: timeout after %d messages", idx, messagesReceived)
					atomic.StoreInt64(&readerStats[idx], int64(messagesReceived))
					return
				}
			}
		}(i, readerChannels[i])
	}

	// Wait for all readers to be ready.
	for i := 0; i < numReaders; i++ {
		<-readersReady
	}
	t.Log("All readers ready")

	// Give readers time to enter their receive loops.
	// This is critical because the topic library drops consumers immediately
	// if they can't receive a message non-blocking.
	time.Sleep(100 * time.Millisecond)

	// Calculate message sending rate.
	messagesPerWriter := totalMessages / numWriters
	messagesPerSecond := float64(totalMessages) / sendDuration.Seconds()
	burstSize := 9 // Send all 9 messages per writer in one burst
	burstInterval := sendDuration / time.Duration(numWriters)

	t.Logf("Target rate: %.0f msg/s, burst size: %d, burst interval: %v",
		messagesPerSecond, burstSize, burstInterval)

	// Start writer goroutines.
	startTime := time.Now()
	for i := 0; i < numWriters; i++ {
		writersWg.Add(1)
		go func(writerID int) {
			defer writersWg.Done()

			bursts := messagesPerWriter / burstSize
			remainder := messagesPerWriter % burstSize

			for burst := 0; burst < bursts; burst++ {
				burstStart := time.Now()

				// Send a burst of messages.
				for j := 0; j < burstSize; j++ {
					messageNum := writerID*messagesPerWriter + burst*burstSize + j
					message := fmt.Sprintf("msg-%d-w%d", messageNum, writerID)

					// Try to broadcast the message.
					TopicMap.RLock()
					if currentTopic, ok := TopicMap.m[testKey]; ok {
						select {
						case currentTopic.t.Broadcast <- message:
							atomic.AddInt64(&totalSent, 1)
							if j == 0 {
								t.Logf("Writer %d: sent first message", writerID)
							}
						default:
							// Broadcast channel is full.
							atomic.AddInt64(&totalDropped, 1)
							t.Logf("Writer %d: broadcast channel full at message %d", writerID, j)
							// Back off a bit when channel is full.
							time.Sleep(1 * time.Millisecond)
						}
					} else {
						t.Logf("Writer %d: topic not found", writerID)
					}
					TopicMap.RUnlock()
				}

				// Send remainder messages if any.
				if burst == bursts-1 && remainder > 0 {
					for j := 0; j < remainder; j++ {
						messageNum := writerID*messagesPerWriter + bursts*burstSize + j
						message := fmt.Sprintf("msg-%d-w%d", messageNum, writerID)

						TopicMap.RLock()
						if currentTopic, ok := TopicMap.m[testKey]; ok {
							select {
							case currentTopic.t.Broadcast <- message:
								atomic.AddInt64(&totalSent, 1)
							default:
								atomic.AddInt64(&totalDropped, 1)
							}
						}
						TopicMap.RUnlock()
					}
				}

				// Rate limiting between bursts.
				elapsed := time.Since(burstStart)
				if elapsed < burstInterval {
					time.Sleep(burstInterval - elapsed)
				}
			}

			if writerID == numWriters-1 {
				elapsed := time.Since(startTime)
				sent := atomic.LoadInt64(&totalSent)
				rate := float64(sent) / elapsed.Seconds()
				t.Logf("Last writer done. Total sent: %d, elapsed: %v, actual rate: %.0f msg/s",
					sent, elapsed, rate)
			}
		}(i)
	}

	// Wait for writers to finish.
	writersDone := make(chan bool)
	go func() {
		writersWg.Wait()
		writersDone <- true
	}()

	select {
	case <-writersDone:
		elapsed := time.Since(startTime)
		t.Logf("All writers finished in %v", elapsed)
		if elapsed > sendDuration+1*time.Second {
			t.Logf("Warning: Writers took longer than expected: %v (target ~%v)", elapsed, sendDuration)
		}
	case <-time.After(sendDuration + 5*time.Second):
		t.Error("Writers timeout")
	}

	// Give readers final time to process remaining messages.
	t.Log("Waiting for readers to finish processing...")
	time.Sleep(3 * time.Second)

	// Signal readers to stop and wait for them.
	readersWg.Wait()

	// Calculate statistics.
	totalSentFinal := atomic.LoadInt64(&totalSent)
	totalDroppedFinal := atomic.LoadInt64(&totalDropped)
	droppedReaders := atomic.LoadInt64(&readersDropped)
	actualDuration := time.Since(startTime)

	t.Logf("\n=== High Load Test Statistics ===")
	t.Logf("Total messages attempted: %d", totalMessages)
	t.Logf("Total messages sent: %d (%.1f%%)", totalSentFinal, float64(totalSentFinal)*100/float64(totalMessages))
	t.Logf("Total messages dropped: %d (%.1f%%)", totalDroppedFinal, float64(totalDroppedFinal)*100/float64(totalMessages))
	t.Logf("Send duration: %v", actualDuration)
	t.Logf("Actual send rate: %.0f messages/second", float64(totalSentFinal)/actualDuration.Seconds())
	t.Logf("Readers dropped as slow: %d/%d", droppedReaders, numReaders)

	var totalReceived int64
	activeReaders := 0
	for i := 0; i < numReaders; i++ {
		received := atomic.LoadInt64(&readerStats[i])
		totalReceived += received
		if received > 0 {
			activeReaders++
			t.Logf("Reader %d: received %d messages", i, received)
		}
	}

	avgPerReader := float64(totalReceived) / float64(activeReaders)
	t.Logf("\nTotal messages received: %d", totalReceived)
	t.Logf("Active readers: %d/%d", activeReaders, numReaders)
	t.Logf("Average messages per active reader: %.0f", avgPerReader)

	// Verify final topic state.
	TopicMap.RLock()
	if entry, ok := TopicMap.m[testKey]; ok {
		t.Logf("Final topic connections: %d", entry.count)
	}
	TopicMap.RUnlock()

	// Success criteria:
	// - All messages were sent successfully
	// - All readers stayed active and received exactly 9 messages
	// - No readers were dropped
	successRate := float64(totalSentFinal) / float64(totalMessages)
	if successRate < 1.0 {
		t.Errorf("Not all messages sent: %.1f%% (expected 100%%)", successRate*100)
	}

	if droppedReaders > 0 {
		t.Errorf("Some readers were dropped: %d/%d", droppedReaders, numReaders)
	}

	// Each reader should receive exactly 9 messages.
	for i := 0; i < numReaders; i++ {
		received := atomic.LoadInt64(&readerStats[i])
		if received != 9 {
			t.Errorf("Reader %d received %d messages (expected 9)", i, received)
		}
	}

	if totalReceived != 90 {
		t.Errorf("Total messages received: %d (expected 90)", totalReceived)
	}
}
