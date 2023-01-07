package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
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

func stress(t time.Duration, cores int) {
	done := make(chan int)

	if cores == 0 {
		cores = runtime.NumCPU()
	}

	for i := 0; i < cores; i++ {
		go func() {
			for {
				select {
				case <-done:
					return
				default:
				}
			}
		}()
	}

	time.Sleep(t)

	close(done)
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

	cores := 0
	s_cores, ok := r.URL.Query()["cores"]
	if ok {
		var err error
		cores, err = strconv.Atoi(s_cores[0])
		if err != nil {
			cores = 0
		}
	}

	s_sleep, ok = r.URL.Query()["stress"]
	if ok {
		stress(sleepTime(s_sleep[0]), cores)
	}

	log.Printf("Request: %s %s %s\n", r.Method, r.RequestURI, r.Proto)
	log.Printf("RemoteAddr: %s\n", r.RemoteAddr)
	log.Printf("Host: %s\n", r.Host)

	if strings.HasPrefix(r.RequestURI, "/hostname") {
		hostname, err := os.Hostname()
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "%s\n", err)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(response_code)
		fmt.Fprintf(w, "Hostname: %s\n", hostname)
		return
	}

	if strings.HasSuffix(r.URL.Path, ".json") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(response_code)

		j := map[string]interface{}{
			"request":   map[string]interface{}{},
			"headers":   map[string]interface{}{},
			"generated": map[string]interface{}{},
		}

		j["request"].(map[string]interface{})["method"] = r.Method
		j["request"].(map[string]interface{})["uri"] = r.RequestURI
		j["request"].(map[string]interface{})["proto"] = r.Proto
		j["request"].(map[string]interface{})["content-length"] = r.ContentLength
		j["request"].(map[string]interface{})["remote-addr"] = r.RemoteAddr
		j["request"].(map[string]interface{})["close"] = r.Close

		u, _ := uuid.NewRandom()
		j["generated"].(map[string]interface{})["uuid"] = u.String()
		j["generated"].(map[string]interface{})["time"] = time.Now().String()

		for k, v := range r.Header {
			j["headers"].(map[string]interface{})[k] = v
			log.Printf("%s: %v\n", k, v)
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
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
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
			log.Printf("%s: %v\n", k, strings.Join(r.Header[k], ", "))
		}

		u, _ := uuid.NewRandom()
		fmt.Fprintf(w, "\n[Server Generated]\n")
		fmt.Fprintf(w, "uuid: %s\n", u.String())
		fmt.Fprintf(w, "time: %s\n", time.Now().String())

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
	listenPort := os.Getenv("PORT")
	if listenPort == "" {
		listenPort = "8080"
	}
	listenAddr := os.Getenv("LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = "0.0.0.0"
	}

	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%s", listenAddr, listenPort),
		Handler:           http.HandlerFunc(handler),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       3660 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      10 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	idleConnsClosed := make(chan struct{})

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		log.Printf("Starting server shutdown\n")

		ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
		if err := server.Shutdown(ctx); err != nil {
			// Error from closing listeners, or context timeout:
			log.Printf("HTTP server Shutdown: %v\n", err)
		}
		close(idleConnsClosed)
	}()

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		// Error starting or closing listener:
		log.Fatalf("HTTP server ListenAndServe: %v\n", err)
	}

	<-idleConnsClosed
}
