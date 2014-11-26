package main

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/mux"
	"github.com/pmylund/go-cache"
)

var c = cache.New(ItemLifetime, 5*time.Minute)

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Please read the documentation on how to talk to this server.")
}

func StreamsGetHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	value, found := c.Get(vars["key"])
	if !found {
		value = make([]url.Values, 0)
	}

	fmt.Fprint(w, JSONResponse{"messages": value})
}
func StreamsPostHandler(w http.ResponseWriter, r *http.Request) {
	var messages []url.Values
	vars := mux.Vars(r)
	value, found := c.Get(vars["key"])
	if !found {
		messages = make([]url.Values, 0)
	} else {
		messages = value.([]url.Values)
	}
	r.ParseForm()
	messages = append(messages, r.Form)

	if len(messages) > MaxQueueLength {
		messages = messages[1:len(messages)]
	}
	c.Set(vars["key"], messages, 0)
	fmt.Fprint(w, JSONResponse{"status": "success"})
}
