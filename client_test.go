package infra

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// --- Constructor ---

func TestClient_NewAPIClient_Defaults_Good(t *testing.T) {
	c := NewAPIClient()
	if c.client == nil {
		t.Fatal("expected non-nil")
	}
	if "api" != c.prefix {
		t.Fatalf("want %v, got %v", "api", c.prefix)
	}
	if 3 != c.retry.MaxRetries {
		t.Fatalf("want %v, got %v", 3, c.retry.MaxRetries)
	}
	if 100*time.Millisecond != c.retry.InitialBackoff {
		t.Fatalf("want %v, got %v", 100*time.Millisecond, c.retry.InitialBackoff)
	}
	if 5*time.Second != c.retry.MaxBackoff {
		t.Fatalf("want %v, got %v", 5*time.Second, c.retry.MaxBackoff)
	}
	if c.authFn != nil {
		t.Fatal("expected nil, got non-nil")
	}
}

func TestClient_NewAPIClient_WithOptions_Good(t *testing.T) {
	custom := &http.Client{Timeout: 10 * time.Second}
	authCalled := false

	c := NewAPIClient(
		WithHTTPClient(custom),
		WithPrefix("test-api"),
		WithRetry(RetryConfig{MaxRetries: 5, InitialBackoff: 200 * time.Millisecond, MaxBackoff: 10 * time.Second}),
		WithAuth(func(req *http.Request) { authCalled = true }),
	)
	if custom != c.client {
		t.Fatalf("want %v, got %v", custom, c.client)
	}
	if "test-api" != c.prefix {
		t.Fatalf("want %v, got %v", "test-api", c.prefix)
	}
	if 5 != c.retry.MaxRetries {
		t.Fatalf("want %v, got %v", 5, c.retry.MaxRetries)
	}
	if 200*time.Millisecond != c.retry.InitialBackoff {
		t.Fatalf("want %v, got %v", 200*time.Millisecond, c.retry.InitialBackoff)
	}
	if 10*time.Second != c.retry.MaxBackoff {
		t.Fatalf("want %v, got %v", 10*time.Second, c.retry.MaxBackoff)
	}

	// Trigger auth
	c.authFn(&http.Request{Header: http.Header{}})
	if !authCalled {
		t.Fatal("expected true")
	}
}

func TestClient_DefaultRetryConfig_Good(t *testing.T) {
	cfg := DefaultRetryConfig()
	if 3 != cfg.MaxRetries {
		t.Fatalf("want %v, got %v", 3, cfg.MaxRetries)
	}
	if 100*time.Millisecond != cfg.InitialBackoff {
		t.Fatalf("want %v, got %v", 100*time.Millisecond, cfg.InitialBackoff)
	}
	if 5*time.Second != cfg.MaxBackoff {
		t.Fatalf("want %v, got %v", 5*time.Second, cfg.MaxBackoff)
	}
}

// --- Do method ---

func TestClient_APIClient_Do_Success_Good(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"test"}`))
	}))
	defer ts.Close()

	c := NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithRetry(RetryConfig{}),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result struct {
		Name string `json:"name"`
	}
	err = c.Do(req, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if "test" != result.Name {
		t.Fatalf("want %v, got %v", "test", result.Name)
	}
}

func TestClient_APIClient_Do_NilResult_Good(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithRetry(RetryConfig{}),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodDelete, ts.URL+"/item", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = c.Do(req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_APIClient_Do_AuthApplied_Good(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "Bearer my-token" != r.Header.Get("Authorization") {
			t.Fatalf("want %v, got %v", "Bearer my-token", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	c := NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithAuth(func(req *http.Request) {
			req.Header.Set("Authorization", "Bearer my-token")
		}),
		WithRetry(RetryConfig{}),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = c.Do(req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_APIClient_Do_ClientError_Bad(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`not found`))
	}))
	defer ts.Close()

	c := NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithPrefix("test-api"),
		WithRetry(RetryConfig{}),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/missing", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = c.Do(req, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "test-api: HTTP 404") {
		t.Fatalf("expected %v to contain %v", err.Error(), "test-api: HTTP 404")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected %v to contain %v", err.Error(), "not found")
	}
}

func TestClient_APIClient_Do_DecodeError_Bad(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not json`))
	}))
	defer ts.Close()

	c := NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithRetry(RetryConfig{}),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result struct{ Name string }
	err = c.Do(req, &result)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "decode response") {
		t.Fatalf("expected %v to contain %v", err.Error(), "decode response")
	}
}

// --- Retry logic ---

func TestClient_APIClient_Do_RetriesServerError_Good(t *testing.T) {
	var attempts atomic.Int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`server error`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	c := NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithPrefix("retry-test"),
		WithRetry(RetryConfig{
			MaxRetries:     3,
			InitialBackoff: 1 * time.Millisecond,
			MaxBackoff:     10 * time.Millisecond,
		}),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result struct {
		OK bool `json:"ok"`
	}
	err = c.Do(req, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.OK {
		t.Fatal("expected true")
	}
	if int32(3) != attempts.Load() {
		t.Fatalf("want %v, got %v", int32(3), attempts.Load())
	}
}

func TestClient_APIClient_Do_ExhaustsRetries_Bad(t *testing.T) {
	var attempts atomic.Int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`always fails`))
	}))
	defer ts.Close()

	c := NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithPrefix("exhaust-test"),
		WithRetry(RetryConfig{
			MaxRetries:     2,
			InitialBackoff: 1 * time.Millisecond,
			MaxBackoff:     5 * time.Millisecond,
		}),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = c.Do(req, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "exhaust-test: HTTP 500") {
		t.Fatalf("expected %v to contain %v", err.Error(), "exhaust-test: HTTP 500")
	}
	// 1 initial + 2 retries = 3 attempts
	if int32(3) != attempts.Load() {
		t.Fatalf("want %v, got %v", int32(3), attempts.Load())
	}
}

func TestClient_APIClient_Do_NoRetryOn4xx_Good(t *testing.T) {
	var attempts atomic.Int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`bad request`))
	}))
	defer ts.Close()

	c := NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithRetry(RetryConfig{
			MaxRetries:     3,
			InitialBackoff: 1 * time.Millisecond,
			MaxBackoff:     5 * time.Millisecond,
		}),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = c.Do(req, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// 4xx errors are NOT retried
	if int32(1) != attempts.Load() {
		t.Fatalf("want %v, got %v", int32(1), attempts.Load())
	}
}

func TestClient_APIClient_Do_ZeroRetries_Good(t *testing.T) {
	var attempts atomic.Int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`fail`))
	}))
	defer ts.Close()

	c := NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithRetry(RetryConfig{}), // Zero retries
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = c.Do(req, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if int32(1) != attempts.Load() {
		t.Fatalf("want %v, got %v", int32(1), attempts.Load())
	}
}

// --- Rate limiting ---

func TestClient_APIClient_Do_RateLimitRetry_Good(t *testing.T) {
	var attempts atomic.Int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`rate limited`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	c := NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithRetry(RetryConfig{
			MaxRetries:     2,
			InitialBackoff: 1 * time.Millisecond,
			MaxBackoff:     5 * time.Millisecond,
		}),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	start := time.Now()
	var result struct {
		OK bool `json:"ok"`
	}
	err = c.Do(req, &result)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.OK {
		t.Fatal("expected true")
	}
	if int32(2) != attempts.Load() {
		t.Fatalf("want %v, got %v", int32(2), attempts.Load())
	}
	// Should have waited at least 1 second for Retry-After
	if elapsed.Milliseconds() < int64(900) {
		t.Fatalf("want >= %v, got %v", int64(900), elapsed.Milliseconds())
	}
}

func TestClient_APIClient_Do_RateLimitExhausted_Bad(t *testing.T) {
	var attempts atomic.Int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`rate limited`))
	}))
	defer ts.Close()

	c := NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithRetry(RetryConfig{
			MaxRetries:     1,
			InitialBackoff: 1 * time.Millisecond,
			MaxBackoff:     5 * time.Millisecond,
		}),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = c.Do(req, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("expected %v to contain %v", err.Error(), "rate limited")
	}
	// 1 initial + 1 retry
	if int32(2) != attempts.Load() {
		t.Fatalf("want %v, got %v", int32(2), attempts.Load())
	}
}

func TestClient_APIClient_Do_RateLimitNoRetryAfterHeader_Good(t *testing.T) {
	var attempts atomic.Int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n == 1 {
			// 429 without Retry-After header — falls back to 1s
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`rate limited`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	c := NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithRetry(RetryConfig{
			MaxRetries:     1,
			InitialBackoff: 1 * time.Millisecond,
			MaxBackoff:     5 * time.Millisecond,
		}),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = c.Do(req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if int32(2) != attempts.Load() {
		t.Fatalf("want %v, got %v", int32(2), attempts.Load())
	}
}

func TestClient_APIClient_Do_ContextCancelled_Ugly(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`fail`))
	}))
	defer ts.Close()

	c := NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithRetry(RetryConfig{
			MaxRetries:     5,
			InitialBackoff: 5 * time.Second, // long backoff
			MaxBackoff:     10 * time.Second,
		}),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = c.Do(req, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- DoRaw method ---

func TestClient_APIClient_DoRaw_Success_Good(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`raw data here`))
	}))
	defer ts.Close()

	c := NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithRetry(RetryConfig{}),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/data", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := c.DoRaw(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if "raw data here" != string(data) {
		t.Fatalf("want %v, got %v", "raw data here", string(data))
	}
}

func TestClient_APIClient_DoRaw_AuthApplied_Good(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok {
			t.Fatal("expected true")
		}
		if "user" != user {
			t.Fatalf("want %v, got %v", "user", user)
		}
		if "pass" != pass {
			t.Fatalf("want %v, got %v", "pass", pass)
		}
		_, _ = w.Write([]byte(`ok`))
	}))
	defer ts.Close()

	c := NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithAuth(func(req *http.Request) { req.SetBasicAuth("user", "pass") }),
		WithRetry(RetryConfig{}),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := c.DoRaw(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if "ok" != string(data) {
		t.Fatalf("want %v, got %v", "ok", string(data))
	}
}

func TestClient_APIClient_DoRaw_ClientError_Bad(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`forbidden`))
	}))
	defer ts.Close()

	c := NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithPrefix("raw-test"),
		WithRetry(RetryConfig{}),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/secret", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = c.DoRaw(req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "raw-test: HTTP 403") {
		t.Fatalf("expected %v to contain %v", err.Error(), "raw-test: HTTP 403")
	}
}

func TestClient_APIClient_DoRaw_RetriesServerError_Good(t *testing.T) {
	var attempts atomic.Int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n < 2 {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`bad gateway`))
			return
		}
		_, _ = w.Write([]byte(`ok`))
	}))
	defer ts.Close()

	c := NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithRetry(RetryConfig{
			MaxRetries:     2,
			InitialBackoff: 1 * time.Millisecond,
			MaxBackoff:     5 * time.Millisecond,
		}),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := c.DoRaw(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if "ok" != string(data) {
		t.Fatalf("want %v, got %v", "ok", string(data))
	}
	if int32(2) != attempts.Load() {
		t.Fatalf("want %v, got %v", int32(2), attempts.Load())
	}
}

func TestClient_APIClient_DoRaw_RateLimitRetry_Good(t *testing.T) {
	var attempts atomic.Int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`rate limited`))
			return
		}
		_, _ = w.Write([]byte(`ok`))
	}))
	defer ts.Close()

	c := NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithRetry(RetryConfig{
			MaxRetries:     2,
			InitialBackoff: 1 * time.Millisecond,
			MaxBackoff:     5 * time.Millisecond,
		}),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := c.DoRaw(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if "ok" != string(data) {
		t.Fatalf("want %v, got %v", "ok", string(data))
	}
	if int32(2) != attempts.Load() {
		t.Fatalf("want %v, got %v", int32(2), attempts.Load())
	}
}

func TestClient_APIClient_DoRaw_NoRetryOn4xx_Bad(t *testing.T) {
	var attempts atomic.Int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`validation error`))
	}))
	defer ts.Close()

	c := NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithRetry(RetryConfig{
			MaxRetries:     3,
			InitialBackoff: 1 * time.Millisecond,
			MaxBackoff:     5 * time.Millisecond,
		}),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = c.DoRaw(req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if int32(1) != attempts.Load() {
		t.Fatalf("want %v, got %v", int32(1), attempts.Load())
	}
}

// --- parseRetryAfter ---

func TestClient_ParseRetryAfter_Seconds_Good(t *testing.T) {
	d := parseRetryAfter("5")
	if 5*time.Second != d {
		t.Fatalf("want %v, got %v", 5*time.Second, d)
	}
}

func TestClient_ParseRetryAfter_EmptyDefault_Good(t *testing.T) {
	d := parseRetryAfter("")
	if 1*time.Second != d {
		t.Fatalf("want %v, got %v", 1*time.Second, d)
	}
}

func TestClient_ParseRetryAfter_InvalidFallback_Bad(t *testing.T) {
	d := parseRetryAfter("not-a-number")
	if 1*time.Second != d {
		t.Fatalf("want %v, got %v", 1*time.Second, d)
	}
}

func TestClient_ParseRetryAfter_Zero_Good(t *testing.T) {
	d := parseRetryAfter("0")
	// 0 is not > 0, falls back to 1s
	if 1*time.Second != d {
		t.Fatalf("want %v, got %v", 1*time.Second, d)
	}
}

// --- Integration: HCloudClient uses APIClient retry ---

func TestClient_HCloudClient_RetriesOnServerError_Good(t *testing.T) {
	var attempts atomic.Int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`unavailable`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"servers":[]}`))
	}))
	defer ts.Close()

	client := NewHCloudClient("test-token")
	client.baseURL = ts.URL
	client.api = NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithAuth(func(req *http.Request) {
			req.Header.Set("Authorization", "Bearer test-token")
		}),
		WithPrefix("hcloud API"),
		WithRetry(RetryConfig{
			MaxRetries:     2,
			InitialBackoff: 1 * time.Millisecond,
			MaxBackoff:     5 * time.Millisecond,
		}),
	)

	servers, err := client.ListServers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(servers) != 0 {
		t.Fatalf("expected empty, got %v", servers)
	}
	if int32(2) != attempts.Load() {
		t.Fatalf("want %v, got %v", int32(2), attempts.Load())
	}
}

func TestClient_HCloudClient_HandlesRateLimit_Good(t *testing.T) {
	var attempts atomic.Int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`rate limited`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"servers":[]}`))
	}))
	defer ts.Close()

	client := NewHCloudClient("test-token")
	client.baseURL = ts.URL
	client.api = NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithAuth(func(req *http.Request) {
			req.Header.Set("Authorization", "Bearer test-token")
		}),
		WithPrefix("hcloud API"),
		WithRetry(RetryConfig{
			MaxRetries:     2,
			InitialBackoff: 1 * time.Millisecond,
			MaxBackoff:     5 * time.Millisecond,
		}),
	)

	servers, err := client.ListServers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(servers) != 0 {
		t.Fatalf("expected empty, got %v", servers)
	}
	if int32(2) != attempts.Load() {
		t.Fatalf("want %v, got %v", int32(2), attempts.Load())
	}
}

// --- Integration: CloudNS uses APIClient retry ---

func TestClient_CloudNSClient_RetriesOnServerError_Good(t *testing.T) {
	var attempts atomic.Int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`internal error`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"name":"example.com","type":"master","zone":"domain","status":"1"}]`))
	}))
	defer ts.Close()

	client := NewCloudNSClient("12345", "secret")
	client.baseURL = ts.URL
	client.api = NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithPrefix("cloudns API"),
		WithRetry(RetryConfig{
			MaxRetries:     2,
			InitialBackoff: 1 * time.Millisecond,
			MaxBackoff:     5 * time.Millisecond,
		}),
	)

	zones, err := client.ListZones(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(zones) != 1 {
		t.Fatalf("want length %v, got %v", 1, len(zones))
	}
	if "example.com" != zones[0].Name {
		t.Fatalf("want %v, got %v", "example.com", zones[0].Name)
	}
	if int32(2) != attempts.Load() {
		t.Fatalf("want %v, got %v", int32(2), attempts.Load())
	}
}

// --- Rate limit shared state ---

func TestClient_APIClient_RateLimitSharedState_Good(t *testing.T) {
	// Verify that the blockedUntil state is respected across requests
	var attempts atomic.Int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`ok`))
	}))
	defer ts.Close()

	c := NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithRetry(RetryConfig{
			MaxRetries:     1,
			InitialBackoff: 1 * time.Millisecond,
			MaxBackoff:     5 * time.Millisecond,
		}),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/first", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := c.DoRaw(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if "ok" != string(data) {
		t.Fatalf("want %v, got %v", "ok", string(data))
	}
}
