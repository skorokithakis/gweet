package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/pmylund/go-cache"
)

type JSONResponse map[string]interface{}

func (r JSONResponse) String() (s string) {

	b, err := json.Marshal(r)
	if err != nil {
		s = ""
		return
	}
	s = string(b)
	return
}

var (
	DEBUG   *log.Logger
	INFO    *log.Logger
	WARNING *log.Logger
	ERROR   *log.Logger
)

func InitLogging(
	debugHandle io.Writer,
	infoHandle io.Writer,
	warningHandle io.Writer,
	errorHandle io.Writer) {

	DEBUG = log.New(debugHandle,
		"DEBUG: ",
		log.Ldate|log.Ltime)

	INFO = log.New(infoHandle,
		"INFO: ",
		log.Ldate|log.Ltime)

	WARNING = log.New(warningHandle,
		"WARNING: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	ERROR = log.New(errorHandle,
		"ERROR: ",
		log.Ldate|log.Ltime|log.Lshortfile)
}

func Log(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		handler.ServeHTTP(w, r)
		INFO.Printf("%v %v %v (%v)\n", r.RemoteAddr, r.Method, r.URL, time.Now().Sub(startTime))
	})
}

// A chunked response helper.
func Chunk(s string) string {
	return fmt.Sprintf("%x\r\n%v\r\n", len(s), s)
}

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
		}
	}
}
