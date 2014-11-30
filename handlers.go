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

func StreamsGetHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	latest, err := strconv.Atoi(r.FormValue("latest"))
	if err != nil || latest <= 0 || latest >= MaxQueueLength {
		latest = MaxQueueLength
	}

	// Get the messages from the cache.
	backchan := make(chan []interface{}, 1)
	CacheBus <- CacheMessage{0, backchan, vars["key"]}
	messages := <-backchan

	w.Header().Set("Content-Type", "application/json")
	lowerBound := int(math.Max(0, float64(len(messages)-latest)))
	upperBound := len(messages)
	fmt.Fprint(w, JSONResponse{"messages": messages[lowerBound:upperBound]})
}

func StreamsPostHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	r.ParseForm()
	message := makeMessage(vars["key"], &r.Form)

	// Write the message to the cache.
	CacheBus <- CacheMessage{1, message, vars["key"]}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, JSONResponse{"status": "success", "message": message})
}