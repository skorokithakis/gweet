package main

import (
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

const MaxQueueLength = 100
const ItemLifetime = 5 * 24 * time.Hour

func main() {
	var debug_enabled = flag.Bool("debug", false, "Enable debug logging")
	var intf = flag.String("interface", "0.0.0.0", "The interface to listen on")
	var port = flag.Int("port", 9835, "The port to listen on")
	flag.Parse()
	if *debug_enabled {
		InitLogging(os.Stdout, os.Stdout, os.Stdout, os.Stderr)
	} else {
		InitLogging(ioutil.Discard, os.Stdout, os.Stdout, os.Stderr)
	}

	r := mux.NewRouter()
	r.HandleFunc("/", HomeHandler)
	r.HandleFunc("/stream/{key}/", StreamsStreamingGetHandler).Methods("GET").Queries("streaming", "1")
	r.HandleFunc("/stream/{key}/", StreamsGetHandler).Methods("GET")
	r.HandleFunc("/stream/{key}/", StreamsPostHandler).Methods("POST")

	go Cacher()
	INFO.Println("Listening on " + *intf + ":" + strconv.Itoa(*port))
	log.Fatal(http.ListenAndServe(*intf+":"+strconv.Itoa(*port), Log(r)))
}
