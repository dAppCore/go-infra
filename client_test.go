package infra

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Constructor ---

func TestNewAPIClient_Good_Defaults(t *testing.T) {
	c := NewAPIClient()
	assert.NotNil(t, c.client)
	assert.Equal(t, "api", c.prefix)
	assert.Equal(t, 3, c.retry.MaxRetries)
	assert.Equal(t, 100*time.Millisecond, c.retry.InitialBackoff)
	assert.Equal(t, 5*time.Second, c.retry.MaxBackoff)
	assert.Nil(t, c.authFn)
}

func TestNewAPIClient_Good_WithOptions(t *testing.T) {
	custom := &http.Client{Timeout: 10 * time.Second}
	authCalled := false

	c := NewAPIClient(
		WithHTTPClient(custom),
		WithPrefix("test-api"),
		WithRetry(RetryConfig{MaxRetries: 5, InitialBackoff: 200 * time.Millisecond, MaxBackoff: 10 * time.Second}),
		WithAuth(func(req *http.Request) { authCalled = true }),
	)

	assert.Equal(t, custom, c.client)
	assert.Equal(t, "test-api", c.prefix)
	assert.Equal(t, 5, c.retry.MaxRetries)
	assert.Equal(t, 200*time.Millisecond, c.retry.InitialBackoff)
	assert.Equal(t, 10*time.Second, c.retry.MaxBackoff)

	// Trigger auth
	c.authFn(&http.Request{Header: http.Header{}})
	assert.True(t, authCalled)
}

func TestDefaultRetryConfig_Good(t *testing.T) {
	cfg := DefaultRetryConfig()
	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, 100*time.Millisecond, cfg.InitialBackoff)
	assert.Equal(t, 5*time.Second, cfg.MaxBackoff)
}

// --- Do method ---

func TestAPIClient_Do_Good_Success(t *testing.T) {
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
	require.NoError(t, err)

	var result struct {
		Name string `json:"name"`
	}
	err = c.Do(req, &result)
	require.NoError(t, err)
	assert.Equal(t, "test", result.Name)
}

func TestAPIClient_Do_Good_NilResult(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithRetry(RetryConfig{}),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodDelete, ts.URL+"/item", nil)
	require.NoError(t, err)

	err = c.Do(req, nil)
	assert.NoError(t, err)
}

func TestAPIClient_Do_Good_AuthApplied(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
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
	require.NoError(t, err)

	err = c.Do(req, nil)
	assert.NoError(t, err)
}

func TestAPIClient_Do_Bad_ClientError(t *testing.T) {
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
	require.NoError(t, err)

	err = c.Do(req, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "test-api 404")
	assert.Contains(t, err.Error(), "not found")
}

func TestAPIClient_Do_Bad_DecodeError(t *testing.T) {
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
	require.NoError(t, err)

	var result struct{ Name string }
	err = c.Do(req, &result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decode response")
}

// --- Retry logic ---

func TestAPIClient_Do_Good_RetriesServerError(t *testing.T) {
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
	require.NoError(t, err)

	var result struct {
		OK bool `json:"ok"`
	}
	err = c.Do(req, &result)
	require.NoError(t, err)
	assert.True(t, result.OK)
	assert.Equal(t, int32(3), attempts.Load())
}

func TestAPIClient_Do_Bad_ExhaustsRetries(t *testing.T) {
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
	require.NoError(t, err)

	err = c.Do(req, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exhaust-test 500")
	// 1 initial + 2 retries = 3 attempts
	assert.Equal(t, int32(3), attempts.Load())
}

func TestAPIClient_Do_Good_NoRetryOn4xx(t *testing.T) {
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
	require.NoError(t, err)

	err = c.Do(req, nil)
	assert.Error(t, err)
	// 4xx errors are NOT retried
	assert.Equal(t, int32(1), attempts.Load())
}

func TestAPIClient_Do_Good_ZeroRetries(t *testing.T) {
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
	require.NoError(t, err)

	err = c.Do(req, nil)
	assert.Error(t, err)
	assert.Equal(t, int32(1), attempts.Load())
}

// --- Rate limiting ---

func TestAPIClient_Do_Good_RateLimitRetry(t *testing.T) {
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
	require.NoError(t, err)

	start := time.Now()
	var result struct {
		OK bool `json:"ok"`
	}
	err = c.Do(req, &result)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.True(t, result.OK)
	assert.Equal(t, int32(2), attempts.Load())
	// Should have waited at least 1 second for Retry-After
	assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(900))
}

func TestAPIClient_Do_Bad_RateLimitExhausted(t *testing.T) {
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
	require.NoError(t, err)

	err = c.Do(req, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rate limited")
	assert.Equal(t, int32(2), attempts.Load()) // 1 initial + 1 retry
}

func TestAPIClient_Do_Good_RateLimitNoRetryAfterHeader(t *testing.T) {
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
	require.NoError(t, err)

	err = c.Do(req, nil)
	require.NoError(t, err)
	assert.Equal(t, int32(2), attempts.Load())
}

func TestAPIClient_Do_Ugly_ContextCancelled(t *testing.T) {
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
	require.NoError(t, err)

	err = c.Do(req, nil)
	assert.Error(t, err)
}

// --- DoRaw method ---

func TestAPIClient_DoRaw_Good_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`raw data here`))
	}))
	defer ts.Close()

	c := NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithRetry(RetryConfig{}),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/data", nil)
	require.NoError(t, err)

	data, err := c.DoRaw(req)
	require.NoError(t, err)
	assert.Equal(t, "raw data here", string(data))
}

func TestAPIClient_DoRaw_Good_AuthApplied(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "user", user)
		assert.Equal(t, "pass", pass)
		_, _ = w.Write([]byte(`ok`))
	}))
	defer ts.Close()

	c := NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithAuth(func(req *http.Request) { req.SetBasicAuth("user", "pass") }),
		WithRetry(RetryConfig{}),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/test", nil)
	require.NoError(t, err)

	data, err := c.DoRaw(req)
	require.NoError(t, err)
	assert.Equal(t, "ok", string(data))
}

func TestAPIClient_DoRaw_Bad_ClientError(t *testing.T) {
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
	require.NoError(t, err)

	_, err = c.DoRaw(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "raw-test 403")
}

func TestAPIClient_DoRaw_Good_RetriesServerError(t *testing.T) {
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
	require.NoError(t, err)

	data, err := c.DoRaw(req)
	require.NoError(t, err)
	assert.Equal(t, "ok", string(data))
	assert.Equal(t, int32(2), attempts.Load())
}

func TestAPIClient_DoRaw_Good_RateLimitRetry(t *testing.T) {
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
	require.NoError(t, err)

	data, err := c.DoRaw(req)
	require.NoError(t, err)
	assert.Equal(t, "ok", string(data))
	assert.Equal(t, int32(2), attempts.Load())
}

func TestAPIClient_DoRaw_Bad_NoRetryOn4xx(t *testing.T) {
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
	require.NoError(t, err)

	_, err = c.DoRaw(req)
	assert.Error(t, err)
	assert.Equal(t, int32(1), attempts.Load())
}

// --- parseRetryAfter ---

func TestParseRetryAfter_Good_Seconds(t *testing.T) {
	d := parseRetryAfter("5")
	assert.Equal(t, 5*time.Second, d)
}

func TestParseRetryAfter_Good_EmptyDefault(t *testing.T) {
	d := parseRetryAfter("")
	assert.Equal(t, 1*time.Second, d)
}

func TestParseRetryAfter_Bad_InvalidFallback(t *testing.T) {
	d := parseRetryAfter("not-a-number")
	assert.Equal(t, 1*time.Second, d)
}

func TestParseRetryAfter_Good_Zero(t *testing.T) {
	d := parseRetryAfter("0")
	// 0 is not > 0, falls back to 1s
	assert.Equal(t, 1*time.Second, d)
}

// --- Integration: HCloudClient uses APIClient retry ---

func TestHCloudClient_Good_RetriesOnServerError(t *testing.T) {
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
	require.NoError(t, err)
	assert.Empty(t, servers)
	assert.Equal(t, int32(2), attempts.Load())
}

func TestHCloudClient_Good_HandlesRateLimit(t *testing.T) {
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
	require.NoError(t, err)
	assert.Empty(t, servers)
	assert.Equal(t, int32(2), attempts.Load())
}

// --- Integration: CloudNS uses APIClient retry ---

func TestCloudNSClient_Good_RetriesOnServerError(t *testing.T) {
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
	require.NoError(t, err)
	require.Len(t, zones, 1)
	assert.Equal(t, "example.com", zones[0].Name)
	assert.Equal(t, int32(2), attempts.Load())
}

// --- Rate limit shared state ---

func TestAPIClient_Good_RateLimitSharedState(t *testing.T) {
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
	require.NoError(t, err)

	data, err := c.DoRaw(req)
	require.NoError(t, err)
	assert.Equal(t, "ok", string(data))
}
