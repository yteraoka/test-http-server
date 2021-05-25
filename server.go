package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

func sleepTime(s string) time.Duration {
	t, err := time.ParseDuration(s)
	if err != nil {
		t = time.Duration(1) * time.Second
	}
	return t
}

func responseCode(s string) int {
	code, err := strconv.Atoi(s)
	if err != nil {
		code = http.StatusOK
	}
	return code
}

func handler(w http.ResponseWriter, r *http.Request) {
	s_sleep, ok := r.URL.Query()["sleep"]
	if ok {
		time.Sleep(sleepTime(s_sleep[0]))
	}

	response_code := http.StatusOK
	status, ok := r.URL.Query()["status"]
	if ok {
		response_code = responseCode(status[0])
	}

	if strings.HasSuffix(r.URL.Path, ".json") {
		w.Header().Set("Content-Type","application/json")
		w.WriteHeader(response_code)

		j := map[string]interface{}{
			"request": map[string]interface{}{},
			"headers": map[string]interface{}{},
		}

		j["request"].(map[string]interface{})["method"] = r.Method
		j["request"].(map[string]interface{})["uri"] = r.RequestURI
		j["request"].(map[string]interface{})["proto"] = r.Proto
		j["request"].(map[string]interface{})["content-length"] = r.ContentLength
		j["request"].(map[string]interface{})["remote-addr"] = r.RemoteAddr
		j["request"].(map[string]interface{})["close"] = r.Close

		for k, v := range r.Header {
			j["headers"].(map[string]interface{})[k] = v
		}

		_, ok = r.URL.Query()["echo"]
		if ok {
			if r.Method == http.MethodPost {
				buf, err := ioutil.ReadAll(r.Body)
				if err == nil {
					j["body"] = string(buf)
				}
			}
		} else {
			if r.Method == http.MethodPost {
				io.Copy(ioutil.Discard, r.Body)
			}
		}

		s, _ := json.Marshal(j)
		fmt.Fprintf(w, string(s))
	} else {
		w.Header().Set("Content-Type","text/plain; charset=utf-8")
		w.WriteHeader(response_code)
		fmt.Fprintf(w, "\n[Request]\n")
		fmt.Fprintf(w, "Method: %s\n", r.Method)
		fmt.Fprintf(w, "Host: %s\n", r.Host)
		fmt.Fprintf(w, "RequestURI: %s\n", r.RequestURI)
		fmt.Fprintf(w, "Proto: %s\n", r.Proto)
		fmt.Fprintf(w, "Content-Length: %d\n", r.ContentLength)
		fmt.Fprintf(w, "Close: %v\n", r.Close)
		fmt.Fprintf(w, "RemoteAddr: %v\n", r.RemoteAddr)

		fmt.Fprintf(w, "\n[Received Headers]\n")
		keys := make([]string, 0, len(r.Header))
		for k := range r.Header {
			keys = append(keys, k)
		}

		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(w, "%s: %s\n", k, strings.Join(r.Header[k], ", "))
		}

		_, ok = r.URL.Query()["echo"]
		if ok {
			fmt.Fprintf(w, "\n[Received Body]\n")
			if r.Method == http.MethodPost {
				io.Copy(w, r.Body)
			}
		} else {
			if r.Method == http.MethodPost {
				io.Copy(ioutil.Discard, r.Body)
			}
		}
	}
}

func main() {
	http.HandleFunc("/", handler)
	listenAddr := os.Getenv("LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = ":8080"
	}
	http.ListenAndServe(listenAddr, nil)
}
