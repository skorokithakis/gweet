package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"
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
