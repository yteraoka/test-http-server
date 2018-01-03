package main

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

const DEFAULT_SLEEP_SECONDS = 5

func handler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/sleep" {
		var sleep_sec int = DEFAULT_SLEEP_SECONDS
		secs, ok := r.URL.Query()["s"]
		if ok && len(secs) >= 1 {
			tmp_sec, err := strconv.Atoi(secs[0])
			if err == nil {
				sleep_sec = tmp_sec
			}
		}
		time.Sleep(time.Duration(sleep_sec) * time.Second)
		fmt.Fprintf(w, "Sleep %d\n", sleep_sec)
	} else if r.URL.Path == "/304" {
		w.WriteHeader(http.StatusNotModified)
		fmt.Fprintf(w, "304 File Not Modified\n")
	} else if r.URL.Path == "/400" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "400 Bad Request\n")
	} else if r.URL.Path == "/404" {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "404 File Not Found\n")
	} else if r.URL.Path == "/500" {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "500 Internal Server Error\n")
	} else if r.URL.Path == "/503" {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, "503 Service Unavailable\n")
	} else if r.URL.Path == "/json1" {
		w.Header().Set("Content-Type","application/json")
		fmt.Fprintf(w, "{\"status\": \"ok\"}\n")
	} else if r.URL.Path == "/json2" {
		w.Header().Set("Content-Type","application/json")
		fmt.Fprintf(w, "{\"server\":{\"status\":\"ok\"}}\n")
	} else {
		fmt.Fprintf(w, "Hello, World: " + r.URL.Path + "\n")
	}
}

func main() {
	http.HandleFunc("/", handler)
	http.ListenAndServe(":8080", nil)
}
