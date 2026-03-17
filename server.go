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
	version           = "unknown"
	commit            = "unknown"
	date              = "unknown"
	debug             = false
	sessionCookieName = "SESSION_ID"
)

var sensitiveEnvKeywords = []string{
	"SECRET", "SESSION", "TOKEN", "PASSWORD", "PASSWD", "APIKEY", "API_KEY", "CREDENTIAL", "PRIVATE_KEY",
}

func sleepTime(s string) time.Duration {
	t, err := time.ParseDuration(s)
	if err != nil {
		t = time.Duration(1) * time.Second
	}
	return t
}

func parseStatusCode(s string) int {
	code, err := strconv.Atoi(s)
	if err != nil {
		code = http.StatusOK
	}
	return code
}

func stress(t time.Duration, cores int) {
	done := make(chan struct{})

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

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func isSensitiveEnvKey(key string) bool {
	upper := strings.ToUpper(key)
	for _, kw := range sensitiveEnvKeywords {
		if strings.Contains(upper, kw) {
			return true
		}
	}
	return false
}

func sortedHeaderKeys(headers http.Header) []string {
	keys := make([]string, 0, len(headers))
	for k := range headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func writeRequestInfo(w io.Writer, r *http.Request) {
	fmt.Fprintf(w, "\n[Request]\n")
	fmt.Fprintf(w, "Method: %s\n", r.Method)
	fmt.Fprintf(w, "Host: %s\n", r.Host)
	fmt.Fprintf(w, "RequestURI: %s\n", r.RequestURI)
	fmt.Fprintf(w, "Proto: %s\n", r.Proto)
	fmt.Fprintf(w, "Content-Length: %d\n", r.ContentLength)
	fmt.Fprintf(w, "Close: %v\n", r.Close)
	fmt.Fprintf(w, "RemoteAddr: %v\n", r.RemoteAddr)
}

func writeHeaders(w io.Writer, headers http.Header) {
	for _, k := range sortedHeaderKeys(headers) {
		fmt.Fprintf(w, "%s: %s\n", k, strings.Join(headers[k], ", "))
	}
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
	requestId, err := uuid.NewRandom()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to generate request ID")
	}
	preProcessLog(r, requestId.String())
	status := innerHandler(w, r, requestId.String())
	postProcessLog(r, requestId.String(), status, time.Since(start))
}

type jsonRequest struct {
	Method        string `json:"method"`
	URI           string `json:"uri"`
	Proto         string `json:"proto"`
	ContentLength int64  `json:"content-length"`
	RemoteAddr    string `json:"remote-addr"`
	Close         bool   `json:"close"`
}

type jsonGenerated struct {
	UUID string `json:"uuid"`
	Time string `json:"time"`
}

type jsonResponse struct {
	Request   jsonRequest          `json:"request"`
	Headers   map[string][]string  `json:"headers"`
	Generated jsonGenerated        `json:"generated"`
	Body      string               `json:"body,omitempty"`
}

func innerHandler(w http.ResponseWriter, r *http.Request, requestId string) int {
	buf, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error().Err(err).Str("requestId", requestId).Msg("Failed to read request body")
		w.WriteHeader(http.StatusBadRequest)
		return http.StatusBadRequest
	}

	sSleep, ok := r.URL.Query()["sleep"]
	if ok {
		time.Sleep(sleepTime(sSleep[0]))
	}

	statusCode := http.StatusOK
	statusParam, ok := r.URL.Query()["status"]
	if ok {
		statusCode = parseStatusCode(statusParam[0])
	}
	if envCode := os.Getenv("HTTP_STATUS_CODE"); envCode != "" {
		if code, err := strconv.Atoi(envCode); err == nil {
			statusCode = code
		}
	}

	cores := 0
	sCores, ok := r.URL.Query()["cores"]
	if ok {
		if n, err := strconv.Atoi(sCores[0]); err == nil {
			cores = n
		}
	}

	stressParam, ok := r.URL.Query()["stress"]
	if ok {
		stress(sleepTime(stressParam[0]), cores)
	}

	newSessID, err := uuid.NewRandom()
	if err != nil {
		log.Warn().Err(err).Str("requestId", requestId).Msg("Failed to generate session ID")
	}

	sessionCookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    newSessID.String(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}

	if sessID, err := r.Cookie(sessionCookieName); err == nil {
		sessionCookie.Value = sessID.Value
	}

	http.SetCookie(w, sessionCookie)

	if strings.HasPrefix(r.RequestURI, "/hostname") {
		hostname, err := os.Hostname()
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "%s\n", err)
			return 500
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(statusCode)
		fmt.Fprintf(w, "Hostname: %s\n", hostname)
		return statusCode
	} else if strings.HasPrefix(r.RequestURI, "/env") {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		for _, e := range os.Environ() {
			pair := strings.SplitN(e, "=", 2)
			if isSensitiveEnvKey(pair[0]) {
				prefix := pair[1]
				if len(prefix) > 3 {
					prefix = prefix[:3]
				}
				fmt.Fprintf(w, "%s: %s*****\n", pair[0], prefix)
			} else {
				fmt.Fprintf(w, "%s: %s\n", pair[0], pair[1])
			}
		}
		return statusCode
	} else if strings.HasPrefix(r.RequestURI, "/stream") {
		intervalSec := 1
		if sInterval, ok := r.URL.Query()["interval"]; ok {
			if n, err := strconv.Atoi(sInterval[0]); err == nil {
				intervalSec = n
			}
		}

		count := 5
		if sCount, ok := r.URL.Query()["count"]; ok {
			if n, err := strconv.Atoi(sCount[0]); err == nil {
				count = n
			}
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(statusCode)
		flusher, canFlush := w.(http.Flusher)
		if !canFlush {
			log.Warn().Str("requestId", requestId).Msg("ResponseWriter does not support flushing")
		}

		writeRequestInfo(w, r)
		fmt.Fprintf(w, "\n[Received Headers]\n")
		writeHeaders(w, r.Header)

		if canFlush {
			flusher.Flush()
		}

		for i := 0; i < count; i++ {
			time.Sleep(time.Duration(intervalSec) * time.Second)
			fmt.Fprintf(w, "%s chunk #%d\n", time.Now().Format("2006-01-02T15:04:05Z07:00"), i)
			if canFlush {
				flusher.Flush()
			}
		}
		return statusCode
	}

	if strings.HasSuffix(r.URL.Path, ".json") {
		u, err := uuid.NewRandom()
		if err != nil {
			log.Warn().Err(err).Str("requestId", requestId).Msg("Failed to generate UUID")
		}

		resp := jsonResponse{
			Request: jsonRequest{
				Method:        r.Method,
				URI:           r.RequestURI,
				Proto:         r.Proto,
				ContentLength: r.ContentLength,
				RemoteAddr:    r.RemoteAddr,
				Close:         r.Close,
			},
			Headers: map[string][]string(r.Header),
			Generated: jsonGenerated{
				UUID: u.String(),
				Time: time.Now().String(),
			},
		}

		_, hasEcho := r.URL.Query()["echo"]
		if hasEcho {
			if r.Method == http.MethodPost {
				resp.Body = string(buf)
			}
		} else if debug {
			if r.Method == http.MethodPost {
				fmt.Printf("----- BEGIN BODY -----\n")
				fmt.Printf("%s", buf)
				fmt.Printf("\n----- END BODY -----\n")
			}
		}

		s, err := json.Marshal(resp)
		if err != nil {
			log.Error().Err(err).Str("requestId", requestId).Msg("Failed to marshal JSON response")
			w.WriteHeader(http.StatusInternalServerError)
			return http.StatusInternalServerError
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		fmt.Fprint(w, string(s))
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(statusCode)

		writeRequestInfo(w, r)
		fmt.Fprintf(w, "\n[Received Headers]\n")
		writeHeaders(w, r.Header)

		u, err := uuid.NewRandom()
		if err != nil {
			log.Warn().Err(err).Str("requestId", requestId).Msg("Failed to generate UUID")
		}
		fmt.Fprintf(w, "\n[Server Generated]\n")
		fmt.Fprintf(w, "uuid: %s\n", u.String())
		fmt.Fprintf(w, "time: %s\n", time.Now().String())

		_, hasEcho := r.URL.Query()["echo"]
		if hasEcho {
			fmt.Fprintf(w, "\n[Received Body]\n")
			if r.Method == http.MethodPost {
				fmt.Fprintf(w, "%s", buf)
			}
		} else if debug {
			if r.Method == http.MethodPost {
				fmt.Printf("----- BEGIN HEADERS -----\n")
				for _, k := range sortedHeaderKeys(r.Header) {
					fmt.Printf("%s: %s\n", k, strings.Join(r.Header[k], ", "))
				}
				fmt.Printf("----- END HEADERS -----\n")
				fmt.Printf("----- BEGIN BODY -----\n")
				fmt.Printf("%s", buf)
				fmt.Printf("\n----- END BODY -----\n")
			}
		}
	}

	return statusCode
}

func debugEnv() bool {
	return os.Getenv("DEBUG") != ""
}

func main() {
	zerolog.TimeFieldFormat = time.RFC3339Nano
	zerolog.LevelFieldName = "severity"
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	var versionFlag bool

	flag.BoolVar(&versionFlag, "version", false, "show version and exit")
	flag.BoolVar(&debug, "debug", debugEnv(), "set log level to DEBUG")

	flag.Parse()

	if versionFlag {
		fmt.Printf("version: %s (commit: %s, date, %s)\n", version, commit, date)
		os.Exit(1)
	}

	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	listenPort := getEnv("PORT", "8080")
	listenAddr := getEnv("LISTEN_ADDR", "0.0.0.0")

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
			log.Error().Err(err).Msg("HTTP server Shutdown error")
		}
		close(idleConnsClosed)
	}()

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Error().Err(err).Msg("HTTP server ListenAndServe error")
	}

	<-idleConnsClosed
}
