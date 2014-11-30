package main

import (
	"time"

	"github.com/pmylund/go-cache"
)

// A cache message.
type CacheMessage struct {
	operation int // 0 for read, anything else for write.
	data      interface{}
	key       string
}

var CacheBus = make(chan CacheMessage, 100)

func Cacher() {
	// A cache manager that communicates reads and
	// writes through a channel, so they are atomic.
	var messages []interface{}
	c := cache.New(ItemLifetime, 5*time.Minute)

	for busMessage := range CacheBus {
		value, found := c.Get(busMessage.key)
		if !found {
			messages = make([]interface{}, 0)
		} else {
			messages = value.([]interface{})
		}

		if busMessage.operation == 0 {
			// Read from the cache.
			busMessage.data.(chan []interface{}) <- messages
		} else {
			// Write to the cache.
			messages = append(messages, busMessage.data)

			// Truncate the queue if it's too long.
			if len(messages) > MaxQueueLength {
				messages = messages[1:len(messages)]
			}

			c.Set(busMessage.key, messages, 0)

			// Broadcast the message to streamers.
			TopicMap.RLock()
			if currentTopic, ok := TopicMap.m[busMessage.key]; ok {
				currentTopic.t.Broadcast <- busMessage.data
			}
			TopicMap.RUnlock()
		}
	}
}
