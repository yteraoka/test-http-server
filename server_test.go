package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// --- sleepTime ---

func TestSleepTime(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"1s", 1 * time.Second},
		{"500ms", 500 * time.Millisecond},
		{"2m", 2 * time.Minute},
		{"invalid", 1 * time.Second}, // fallback to 1s
		{"", 1 * time.Second},        // fallback to 1s
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sleepTime(tt.input)
			if got != tt.want {
				t.Errorf("sleepTime(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// --- parseStatusCode ---

func TestParseStatusCode(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"200", 200},
		{"404", 404},
		{"500", 500},
		{"invalid", http.StatusOK}, // fallback to 200
		{"", http.StatusOK},        // fallback to 200
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseStatusCode(tt.input)
			if got != tt.want {
				t.Errorf("parseStatusCode(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// --- isSensitiveEnvKey ---

func TestIsSensitiveEnvKey(t *testing.T) {
	sensitive := []string{
		"SECRET", "MY_SECRET", "secret_key",
		"SESSION", "SESSION_ID",
		"TOKEN", "ACCESS_TOKEN", "auth_token",
		"PASSWORD", "DB_PASSWORD",
		"PASSWD", "DB_PASSWD",
		"APIKEY", "MY_APIKEY",
		"API_KEY", "STRIPE_API_KEY",
		"CREDENTIAL", "GCP_CREDENTIAL",
		"PRIVATE_KEY", "RSA_PRIVATE_KEY",
	}
	for _, key := range sensitive {
		t.Run(key, func(t *testing.T) {
			if !isSensitiveEnvKey(key) {
				t.Errorf("isSensitiveEnvKey(%q) = false, want true", key)
			}
		})
	}

	notSensitive := []string{
		"PATH", "HOME", "USER", "PORT", "HOST",
		"LISTEN_ADDR", "DEBUG", "HTTP_STATUS_CODE",
	}
	for _, key := range notSensitive {
		t.Run(key, func(t *testing.T) {
			if isSensitiveEnvKey(key) {
				t.Errorf("isSensitiveEnvKey(%q) = true, want false", key)
			}
		})
	}
}

// --- debugEnv ---

func TestDebugEnv(t *testing.T) {
	t.Run("DEBUG not set", func(t *testing.T) {
		t.Setenv("DEBUG", "")
		if debugEnv() {
			t.Error("debugEnv() = true, want false")
		}
	})
	t.Run("DEBUG set", func(t *testing.T) {
		t.Setenv("DEBUG", "1")
		if !debugEnv() {
			t.Error("debugEnv() = false, want true")
		}
	})
}

// --- getEnv ---

func TestGetEnv(t *testing.T) {
	t.Run("env not set returns default", func(t *testing.T) {
		t.Setenv("TEST_GET_ENV_KEY", "")
		got := getEnv("TEST_GET_ENV_KEY", "default")
		if got != "default" {
			t.Errorf("getEnv() = %q, want %q", got, "default")
		}
	})
	t.Run("env set returns value", func(t *testing.T) {
		t.Setenv("TEST_GET_ENV_KEY", "myvalue")
		got := getEnv("TEST_GET_ENV_KEY", "default")
		if got != "myvalue" {
			t.Errorf("getEnv() = %q, want %q", got, "myvalue")
		}
	})
}

// --- sortedHeaderKeys ---

func TestSortedHeaderKeys(t *testing.T) {
	headers := http.Header{
		"Zebra":   []string{"z"},
		"Alpha":   []string{"a"},
		"Content": []string{"c"},
	}
	keys := sortedHeaderKeys(headers)
	want := []string{"Alpha", "Content", "Zebra"}
	if len(keys) != len(want) {
		t.Fatalf("len = %d, want %d", len(keys), len(want))
	}
	for i, k := range keys {
		if k != want[i] {
			t.Errorf("keys[%d] = %q, want %q", i, k, want[i])
		}
	}
}

// --- ハンドラーテスト用ヘルパー ---

func newRequest(method, path, body string) *http.Request {
	var bodyReader *strings.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	} else {
		bodyReader = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, bodyReader)
	return req
}

func doRequest(req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	handler(w, req)
	return w
}

// --- /hostname ---

func TestHandlerHostname(t *testing.T) {
	req := newRequest(http.MethodGet, "/hostname", "")
	w := doRequest(req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}
	if !strings.Contains(w.Body.String(), "Hostname:") {
		t.Errorf("body %q does not contain 'Hostname:'", w.Body.String())
	}
}

func TestHandlerHostnameCustomStatus(t *testing.T) {
	req := newRequest(http.MethodGet, "/hostname?status=503", "")
	w := doRequest(req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

// --- /env ---

func TestHandlerEnvMasksSensitiveKeys(t *testing.T) {
	t.Setenv("MY_SECRET_KEY", "supersecretvalue")
	t.Setenv("NORMAL_VAR", "normalvalue")

	req := newRequest(http.MethodGet, "/env", "")
	w := doRequest(req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()

	if strings.Contains(body, "MY_SECRET_KEY: supersecretvalue") {
		t.Error("sensitive value was not masked")
	}
	if !strings.Contains(body, "MY_SECRET_KEY: sup*****") {
		t.Errorf("expected masked output not found in body:\n%s", body)
	}
	if !strings.Contains(body, "NORMAL_VAR: normalvalue") {
		t.Errorf("normal var not found in body:\n%s", body)
	}
}

func TestHandlerEnvMasksShortValue(t *testing.T) {
	t.Setenv("MY_TOKEN", "ab") // 3文字未満でパニックしないこと
	req := newRequest(http.MethodGet, "/env", "")
	w := doRequest(req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), "MY_TOKEN:") {
		t.Error("MY_TOKEN not found in body")
	}
}

// --- ?status パラメータ ---

func TestHandlerStatusParam(t *testing.T) {
	tests := []struct {
		path string
		want int
	}{
		{"/", http.StatusOK},
		{"/?status=404", http.StatusNotFound},
		{"/?status=500", http.StatusInternalServerError},
		{"/?status=invalid", http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := newRequest(http.MethodGet, tt.path, "")
			w := doRequest(req)
			if w.Code != tt.want {
				t.Errorf("status = %d, want %d", w.Code, tt.want)
			}
		})
	}
}

// --- plain text レスポンス ---

func TestHandlerPlainText(t *testing.T) {
	req := newRequest(http.MethodGet, "/", "")
	w := doRequest(req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}

	body := w.Body.String()
	for _, want := range []string{"[Request]", "Method:", "Host:", "[Received Headers]", "[Server Generated]", "uuid:", "time:"} {
		if !strings.Contains(body, want) {
			t.Errorf("body does not contain %q", want)
		}
	}
}

func TestHandlerPlainTextEcho(t *testing.T) {
	req := newRequest(http.MethodPost, "/?echo", "hello world")
	w := doRequest(req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), "hello world") {
		t.Errorf("echo body not found: %s", w.Body.String())
	}
}

// --- JSON レスポンス ---

func TestHandlerJSON(t *testing.T) {
	req := newRequest(http.MethodGet, "/test.json", "")
	req.Header.Set("X-Custom-Header", "testvalue")
	w := doRequest(req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var resp jsonResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Request.Method != http.MethodGet {
		t.Errorf("request.method = %q, want GET", resp.Request.Method)
	}
	if resp.Generated.UUID == "" {
		t.Error("generated.uuid is empty")
	}
	if resp.Generated.Time == "" {
		t.Error("generated.time is empty")
	}
	if _, ok := resp.Headers["X-Custom-Header"]; !ok {
		t.Error("X-Custom-Header not found in response headers")
	}
}

func TestHandlerJSONEcho(t *testing.T) {
	req := newRequest(http.MethodPost, "/test.json?echo", "request body content")
	w := doRequest(req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp jsonResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Body != "request body content" {
		t.Errorf("body = %q, want %q", resp.Body, "request body content")
	}
}

func TestHandlerJSONNoBodyOnGet(t *testing.T) {
	req := newRequest(http.MethodGet, "/test.json?echo", "")
	w := doRequest(req)

	var resp jsonResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Body != "" {
		t.Errorf("body should be empty for GET, got %q", resp.Body)
	}
}

// --- Cookie ---

func TestHandlerCookieCreated(t *testing.T) {
	req := newRequest(http.MethodGet, "/", "")
	w := doRequest(req)

	cookies := w.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == sessionCookieName {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("session cookie not set")
	}
	if sessionCookie.Value == "" {
		t.Error("session cookie value is empty")
	}
	if !sessionCookie.HttpOnly {
		t.Error("session cookie HttpOnly is false")
	}
}

func TestHandlerCookiePreserved(t *testing.T) {
	existingID := "existing-session-id"
	req := newRequest(http.MethodGet, "/", "")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: existingID})
	w := doRequest(req)

	cookies := w.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == sessionCookieName {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("session cookie not set")
	}
	if sessionCookie.Value != existingID {
		t.Errorf("session cookie value = %q, want %q", sessionCookie.Value, existingID)
	}
}

// --- ボディサイズ制限 ---

func TestHandlerBodySizeLimit(t *testing.T) {
	original := maxBodySize
	maxBodySize = 10
	defer func() { maxBodySize = original }()

	req := newRequest(http.MethodPost, "/", strings.Repeat("a", 20))
	w := doRequest(req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want %d", w.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestHandlerBodySizeWithinLimit(t *testing.T) {
	original := maxBodySize
	maxBodySize = 100
	defer func() { maxBodySize = original }()

	req := newRequest(http.MethodPost, "/?echo", strings.Repeat("a", 50))
	w := doRequest(req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// --- /stream ---

func TestHandlerStream(t *testing.T) {
	req := newRequest(http.MethodGet, "/stream?count=2&interval=0", "")
	w := doRequest(req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	if !strings.Contains(body, "chunk #0") {
		t.Errorf("body does not contain 'chunk #0': %s", body)
	}
	if !strings.Contains(body, "chunk #1") {
		t.Errorf("body does not contain 'chunk #1': %s", body)
	}
	if strings.Contains(body, "chunk #2") {
		t.Errorf("body should not contain 'chunk #2' with count=2: %s", body)
	}
}

// --- HTTP_STATUS_CODE 環境変数 ---

func TestHandlerEnvStatusCode(t *testing.T) {
	t.Setenv("HTTP_STATUS_CODE", "503")
	req := newRequest(http.MethodGet, "/", "")
	w := doRequest(req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestHandlerEnvStatusCodeInvalid(t *testing.T) {
	t.Setenv("HTTP_STATUS_CODE", "invalid")
	req := newRequest(http.MethodGet, "/", "")
	w := doRequest(req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// --- /stream のヘッダ・リクエスト情報 ---

func TestHandlerStreamContainsRequestInfo(t *testing.T) {
	req := newRequest(http.MethodGet, "/stream?count=0", "")
	req.Header.Set("X-Test-Header", "testval")
	w := doRequest(req)

	body := w.Body.String()
	for _, want := range []string{"[Request]", "Method:", "[Received Headers]", "X-Test-Header"} {
		if !strings.Contains(body, want) {
			t.Errorf("body does not contain %q:\n%s", want, body)
		}
	}
}

// --- writeHeaders のソート順 ---

func TestWriteHeaders(t *testing.T) {
	headers := http.Header{
		"Zebra": []string{"z"},
		"Alpha": []string{"a"},
		"Mango": []string{"m"},
	}
	var sb strings.Builder
	writeHeaders(&sb, headers)
	body := sb.String()

	lines := strings.Split(strings.TrimSpace(body), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %s", len(lines), body)
	}
	wantOrder := []string{"Alpha", "Mango", "Zebra"}
	for i, line := range lines {
		if !strings.HasPrefix(line, wantOrder[i]) {
			t.Errorf("line[%d] = %q, want prefix %q", i, line, wantOrder[i])
		}
	}
}

// --- writeRequestInfo ---

func TestWriteRequestInfo(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test?foo=bar", nil)
	req.Host = "example.com"

	var sb strings.Builder
	writeRequestInfo(&sb, req)
	body := sb.String()

	for _, want := range []string{
		"[Request]",
		fmt.Sprintf("Method: %s", http.MethodPost),
		"Host: example.com",
		"RequestURI: /test?foo=bar",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("output does not contain %q:\n%s", want, body)
		}
	}
}
