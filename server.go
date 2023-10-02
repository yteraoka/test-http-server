package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
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
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	version = "unknown"
	commit  = "unknown"
	date    = "unknown"
	debug   = false
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

func preProcessLog(r *http.Request, requestId string) {
	log.Debug().Dict("httpRequest", zerolog.Dict().
		Str("requestMethod", r.Method).
		Str("requestUrl", fmt.Sprintf("http://%s%s", r.Host, r.RequestURI)).
		Str("remoteIp", r.RemoteAddr).
		Int64("requestSize", r.ContentLength).
		Str("userAgent", r.Header.Get("User-Agent"))).
		Str("pharse", "pre").
		Str("requestId", requestId).
		Msg("")
}

func postProcessLog(r *http.Request, requestId string, status int, duration time.Duration) {
	log.Info().Dict("httpRequest", zerolog.Dict().
		Str("requestMethod", r.Method).
		Str("requestUrl", fmt.Sprintf("http://%s%s", r.Host, r.RequestURI)).
		Str("remoteIp", r.RemoteAddr).
		Int64("requestSize", r.ContentLength).
		Str("userAgent", r.Header.Get("User-Agent")).
		Int("status", status).
		Str("latency", fmt.Sprintf("%.3fs", float64(duration.Milliseconds())/1000))).
		Str("pharse", "post").
		Str("requestId", requestId).
		Msg("")
}

func handler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestId, _ := uuid.NewRandom()
	preProcessLog(r, requestId.String())
	status := innerHandler(w, r, requestId.String())
	postProcessLog(r, requestId.String(), status, time.Since(start))
}

func innerHandler(w http.ResponseWriter, r *http.Request, requestId string) int {
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

	//log.Printf("Request: %s %s %s\n", r.Method, r.RequestURI, r.Proto)
	//log.Printf("RemoteAddr: %s\n", r.RemoteAddr)
	//log.Printf("Host: %s\n", r.Host)

	if strings.HasPrefix(r.RequestURI, "/hostname") {
		hostname, err := os.Hostname()
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "%s\n", err)
			return 500
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(response_code)
		fmt.Fprintf(w, "Hostname: %s\n", hostname)
		return response_code
	} else if strings.HasPrefix(r.RequestURI, "/env") {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		for _, e := range os.Environ() {
			pair := strings.SplitN(e, "=", 2)
			if strings.Contains(pair[0], "SECRET") || strings.Contains(pair[0], "SESSION") || strings.Contains(pair[0], "TOKEN") {
				fmt.Fprintf(w, "%s: %s*****\n", pair[0], pair[1][0:3])
			} else {
				fmt.Fprintf(w, "%s: %s\n", pair[0], pair[1])
			}
		}
		return response_code
	} else if strings.HasPrefix(r.RequestURI, "/stream") {
		intervalSec := 1
		s_interval, ok := r.URL.Query()["interval"]
		if ok {
			var err error
			intervalSec, err = strconv.Atoi(s_interval[0])
			if err != nil {
				intervalSec = 1
			}
		}

		count := 5
		s_count, ok := r.URL.Query()["count"]
		if ok {
			var err error
			count, err = strconv.Atoi(s_count[0])
			if err != nil {
				count = 1
			}
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(response_code)
		flusher, _ := w.(http.Flusher)

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

		flusher.Flush()

		for i := 0; i < count; i++ {
			time.Sleep(time.Duration(intervalSec) * time.Second)
			fmt.Fprintf(w, "%s chunk #%d\n", time.Now().Format("2006-01-02T15:04:05Z07:00"), i)
			flusher.Flush()
		}
		return response_code
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
			//log.Printf("%s: %v\n", k, v)
		}

		_, ok = r.URL.Query()["echo"]
		if ok {
			if r.Method == http.MethodPost {
				buf, err := io.ReadAll(r.Body)
				if err == nil {
					j["body"] = string(buf)
				}
			}
		} else if debug {
			if r.Method == http.MethodPost {
				fmt.Printf("----- BEGIN BODY -----\n")
				_, err := io.Copy(os.Stdout, r.Body)
				if err != nil {
					log.Error().Err(err).Msgf("")
				}
				fmt.Printf("\n----- END BODY -----\n")
			}
		} else {
			if r.Method == http.MethodPost {
				_, err := io.Copy(io.Discard, r.Body)
				if err != nil {
					log.Error().Err(err).Msgf("")
				}
			}
		}

		s, _ := json.Marshal(j)
		fmt.Fprint(w, string(s))
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
			//log.Printf("%s: %v\n", k, strings.Join(r.Header[k], ", "))
		}

		u, _ := uuid.NewRandom()
		fmt.Fprintf(w, "\n[Server Generated]\n")
		fmt.Fprintf(w, "uuid: %s\n", u.String())
		fmt.Fprintf(w, "time: %s\n", time.Now().String())

		_, ok = r.URL.Query()["echo"]
		if ok {
			fmt.Fprintf(w, "\n[Received Body]\n")
			if r.Method == http.MethodPost {
				_, err := io.Copy(w, r.Body)
				if err != nil {
					log.Error().Err(err).Msgf("")
				}
			}
		} else if debug {
			if r.Method == http.MethodPost {
				fmt.Printf("----- BEGIN HEADERS -----\n")
				for _, k := range keys {
					fmt.Printf("%s: %s\n", k, strings.Join(r.Header[k], ", "))
				}
				fmt.Printf("----- END HEADERS -----\n")
				fmt.Printf("----- BEGIN BODY -----\n")
				_, err := io.Copy(os.Stdout, r.Body)
				if err != nil {
					log.Error().Err(err).Msgf("")
				}
				fmt.Printf("\n----- END BODY -----\n")
			}
		} else {
			if r.Method == http.MethodPost {
				_, err := io.Copy(io.Discard, r.Body)
				if err != nil {
					log.Error().Err(err).Msgf("")
				}
			}
		}
	}

	return response_code
}

func debug_env() bool {
	if os.Getenv("DEBUG") != "" {
		return true
	} else {
		return false
	}
}

func main() {
	zerolog.TimeFieldFormat = time.RFC3339Nano
	zerolog.LevelFieldName = "severity"
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	var version_flag bool

	flag.BoolVar(&version_flag, "version", false, "show version and exit")
	flag.BoolVar(&debug, "debug", debug_env(), "set log level to DEBUG")

	flag.Parse()

	if version_flag {
		fmt.Printf("version: %s (commit: %s, date, %s)\n", version, commit, date)
		os.Exit(1)
	}

	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

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
		WriteTimeout:      60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	idleConnsClosed := make(chan struct{})

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		log.Info().Msg("Starting server shutdown")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			// Error from closing listeners, or context timeout:
			log.Error().Err(err).Msgf("HTTP server Shutdown: %v", err)
		}
		close(idleConnsClosed)
	}()

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		// Error starting or closing listener:
		log.Error().Err(err).Msgf("HTTP server ListenAndServe: %v", err)
	}

	<-idleConnsClosed
}
