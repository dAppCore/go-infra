package infra

import (
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"strconv"
	"sync"
	"time"

	core "dappco.re/go/core"
)

// RetryConfig controls exponential backoff retry behaviour.
// Usage: cfg := infra.RetryConfig{}
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts (0 = no retries).
	MaxRetries int
	// InitialBackoff is the delay before the first retry.
	InitialBackoff time.Duration
	// MaxBackoff is the upper bound on backoff duration.
	MaxBackoff time.Duration
}

// DefaultRetryConfig returns sensible defaults: 3 retries, 100ms initial, 5s max.
// Usage: cfg := infra.DefaultRetryConfig()
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     5 * time.Second,
	}
}

// APIClient is a shared HTTP client with retry, rate-limit handling,
// and configurable authentication. Provider-specific clients embed or
// delegate to this struct.
// Usage: client := infra.NewAPIClient()
type APIClient struct {
	client       *http.Client
	retry        RetryConfig
	authFn       func(req *http.Request)
	prefix       string // error prefix, e.g. "hcloud API"
	mu           sync.Mutex
	blockedUntil time.Time // rate-limit window
}

// APIClientOption configures an APIClient.
// Usage: client := infra.NewAPIClient(infra.WithPrefix("api"))
type APIClientOption func(*APIClient)

// WithHTTPClient sets a custom http.Client.
// Usage: client := infra.NewAPIClient(infra.WithHTTPClient(http.DefaultClient))
func WithHTTPClient(c *http.Client) APIClientOption {
	return func(a *APIClient) { a.client = c }
}

// WithRetry sets the retry configuration.
// Usage: client := infra.NewAPIClient(infra.WithRetry(infra.DefaultRetryConfig()))
func WithRetry(cfg RetryConfig) APIClientOption {
	return func(a *APIClient) { a.retry = cfg }
}

// WithAuth sets the authentication function applied to every request.
// Usage: client := infra.NewAPIClient(infra.WithAuth(func(req *http.Request) {}))
func WithAuth(fn func(req *http.Request)) APIClientOption {
	return func(a *APIClient) { a.authFn = fn }
}

// WithPrefix sets the error message prefix (e.g. "hcloud API").
// Usage: client := infra.NewAPIClient(infra.WithPrefix("hcloud API"))
func WithPrefix(p string) APIClientOption {
	return func(a *APIClient) { a.prefix = p }
}

// NewAPIClient creates a new APIClient with the given options.
// Usage: client := infra.NewAPIClient(infra.WithPrefix("cloudns API"))
func NewAPIClient(opts ...APIClientOption) *APIClient {
	a := &APIClient{
		client: &http.Client{Timeout: 30 * time.Second},
		retry:  DefaultRetryConfig(),
		prefix: "api",
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Do executes an HTTP request with authentication, retry logic, and
// rate-limit handling. If result is non-nil, the response body is
// JSON-decoded into it.
// Usage: err := client.Do(req, &result)
func (a *APIClient) Do(req *http.Request, result any) error {
	if a.authFn != nil {
		a.authFn(req)
	}

	var lastErr error
	attempts := 1 + a.retry.MaxRetries

	for attempt := range attempts {
		// Respect rate-limit backoff window.
		a.mu.Lock()
		wait := time.Until(a.blockedUntil)
		a.mu.Unlock()
		if wait > 0 {
			select {
			case <-req.Context().Done():
				return req.Context().Err()
			case <-time.After(wait):
			}
		}

		resp, err := a.client.Do(req)
		if err != nil {
			lastErr = core.E(a.prefix, "request failed", err)
			if attempt < attempts-1 {
				a.backoff(attempt, req)
			}
			continue
		}

		data, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			lastErr = core.E("client.Do", "read response", err)
			if attempt < attempts-1 {
				a.backoff(attempt, req)
			}
			continue
		}

		// Rate-limited: honour Retry-After and retry.
		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
			a.mu.Lock()
			a.blockedUntil = time.Now().Add(retryAfter)
			a.mu.Unlock()

			lastErr = core.E(a.prefix, core.Sprintf("rate limited: HTTP %d", resp.StatusCode), nil)
			if attempt < attempts-1 {
				select {
				case <-req.Context().Done():
					return req.Context().Err()
				case <-time.After(retryAfter):
				}
			}
			continue
		}

		// Server errors are retryable.
		if resp.StatusCode >= 500 {
			lastErr = core.E(a.prefix, core.Sprintf("HTTP %d: %s", resp.StatusCode, truncateBody(data, maxErrBodyLen)), nil)
			if attempt < attempts-1 {
				a.backoff(attempt, req)
			}
			continue
		}

		// Client errors (4xx, except 429 handled above) are not retried.
		if resp.StatusCode >= 400 {
			return core.E(a.prefix, core.Sprintf("HTTP %d: %s", resp.StatusCode, truncateBody(data, maxErrBodyLen)), nil)
		}

		// Success — decode if requested.
		if result != nil {
			if r := core.JSONUnmarshal(data, result); !r.OK {
				return core.E("client.Do", "decode response", coreResultErr(r, "client.Do"))
			}
		}
		return nil
	}

	return lastErr
}

// DoRaw executes a request and returns the raw response body.
// Same retry/rate-limit logic as Do but without JSON decoding.
// Usage: body, err := client.DoRaw(req)
func (a *APIClient) DoRaw(req *http.Request) ([]byte, error) {
	if a.authFn != nil {
		a.authFn(req)
	}

	var lastErr error
	attempts := 1 + a.retry.MaxRetries

	for attempt := range attempts {
		// Respect rate-limit backoff window.
		a.mu.Lock()
		wait := time.Until(a.blockedUntil)
		a.mu.Unlock()
		if wait > 0 {
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(wait):
			}
		}

		resp, err := a.client.Do(req)
		if err != nil {
			lastErr = core.E(a.prefix, "request failed", err)
			if attempt < attempts-1 {
				a.backoff(attempt, req)
			}
			continue
		}

		data, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			lastErr = core.E("client.DoRaw", "read response", err)
			if attempt < attempts-1 {
				a.backoff(attempt, req)
			}
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
			a.mu.Lock()
			a.blockedUntil = time.Now().Add(retryAfter)
			a.mu.Unlock()

			lastErr = core.E(a.prefix, core.Sprintf("rate limited: HTTP %d", resp.StatusCode), nil)
			if attempt < attempts-1 {
				select {
				case <-req.Context().Done():
					return nil, req.Context().Err()
				case <-time.After(retryAfter):
				}
			}
			continue
		}

		if resp.StatusCode >= 500 {
			lastErr = core.E(a.prefix, core.Sprintf("HTTP %d: %s", resp.StatusCode, truncateBody(data, maxErrBodyLen)), nil)
			if attempt < attempts-1 {
				a.backoff(attempt, req)
			}
			continue
		}

		if resp.StatusCode >= 400 {
			return nil, core.E(a.prefix, core.Sprintf("HTTP %d: %s", resp.StatusCode, truncateBody(data, maxErrBodyLen)), nil)
		}

		return data, nil
	}

	return nil, lastErr
}

// backoff sleeps for exponential backoff with jitter, respecting context cancellation.
func (a *APIClient) backoff(attempt int, req *http.Request) {
	base := float64(a.retry.InitialBackoff) * math.Pow(2, float64(attempt))
	if base > float64(a.retry.MaxBackoff) {
		base = float64(a.retry.MaxBackoff)
	}
	// Add jitter: 50-100% of calculated backoff
	jitter := base * (0.5 + rand.Float64()*0.5)
	d := time.Duration(jitter)

	select {
	case <-req.Context().Done():
	case <-time.After(d):
	}
}

// maxErrBodyLen is the maximum number of bytes from a response body included in error messages.
const maxErrBodyLen = 256

// truncateBody limits response body length in error messages to prevent sensitive data leakage.
func truncateBody(data []byte, max int) string {
	if len(data) <= max {
		return string(data)
	}
	return string(data[:max]) + "...(truncated)"
}

// parseRetryAfter interprets the Retry-After header value.
// Supports seconds (integer) format. Falls back to 1 second.
func parseRetryAfter(val string) time.Duration {
	if val == "" {
		return 1 * time.Second
	}
	seconds, err := strconv.Atoi(val)
	if err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	// Could also try HTTP-date format here, but seconds is typical for APIs.
	return 1 * time.Second
}
