package infra

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// RetryConfig controls exponential backoff retry behaviour.
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts (0 = no retries).
	MaxRetries int
	// InitialBackoff is the delay before the first retry.
	InitialBackoff time.Duration
	// MaxBackoff is the upper bound on backoff duration.
	MaxBackoff time.Duration
}

// DefaultRetryConfig returns sensible defaults: 3 retries, 100ms initial, 5s max.
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
type APIClient struct {
	client  *http.Client
	retry   RetryConfig
	authFn  func(req *http.Request)
	prefix  string // error prefix, e.g. "hcloud API"
	mu      sync.Mutex
	blockedUntil time.Time // rate-limit window
}

// APIClientOption configures an APIClient.
type APIClientOption func(*APIClient)

// WithHTTPClient sets a custom http.Client.
func WithHTTPClient(c *http.Client) APIClientOption {
	return func(a *APIClient) { a.client = c }
}

// WithRetry sets the retry configuration.
func WithRetry(cfg RetryConfig) APIClientOption {
	return func(a *APIClient) { a.retry = cfg }
}

// WithAuth sets the authentication function applied to every request.
func WithAuth(fn func(req *http.Request)) APIClientOption {
	return func(a *APIClient) { a.authFn = fn }
}

// WithPrefix sets the error message prefix (e.g. "hcloud API").
func WithPrefix(p string) APIClientOption {
	return func(a *APIClient) { a.prefix = p }
}

// NewAPIClient creates a new APIClient with the given options.
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
			lastErr = fmt.Errorf("%s: %w", a.prefix, err)
			if attempt < attempts-1 {
				a.backoff(attempt, req)
			}
			continue
		}

		data, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("read response: %w", err)
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

			lastErr = fmt.Errorf("%s %d: rate limited", a.prefix, resp.StatusCode)
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
			lastErr = fmt.Errorf("%s %d: %s", a.prefix, resp.StatusCode, string(data))
			if attempt < attempts-1 {
				a.backoff(attempt, req)
			}
			continue
		}

		// Client errors (4xx, except 429 handled above) are not retried.
		if resp.StatusCode >= 400 {
			return fmt.Errorf("%s %d: %s", a.prefix, resp.StatusCode, string(data))
		}

		// Success — decode if requested.
		if result != nil {
			if err := json.Unmarshal(data, result); err != nil {
				return fmt.Errorf("decode response: %w", err)
			}
		}
		return nil
	}

	return lastErr
}

// DoRaw executes a request and returns the raw response body.
// Same retry/rate-limit logic as Do but without JSON decoding.
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
			lastErr = fmt.Errorf("%s: %w", a.prefix, err)
			if attempt < attempts-1 {
				a.backoff(attempt, req)
			}
			continue
		}

		data, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("read response: %w", err)
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

			lastErr = fmt.Errorf("%s %d: rate limited", a.prefix, resp.StatusCode)
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
			lastErr = fmt.Errorf("%s %d: %s", a.prefix, resp.StatusCode, string(data))
			if attempt < attempts-1 {
				a.backoff(attempt, req)
			}
			continue
		}

		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("%s %d: %s", a.prefix, resp.StatusCode, string(data))
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
