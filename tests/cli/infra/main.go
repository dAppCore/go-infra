// AX-10 CLI driver for go-infra. It exercises the public configuration and
// HTTP client surfaces without depending on the repository's unit test package.
//
//	task -d tests/cli/infra
//	go run ./tests/cli/infra
package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"time"

	infra "dappco.re/go/infra"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if err := verifyConfigLoad(); err != nil {
		return fmt.Errorf("config load: %w", err)
	}
	if err := verifyAPIClientJSON(); err != nil {
		return fmt.Errorf("api client json: %w", err)
	}
	if err := verifyAPIClientRetryAndRaw(); err != nil {
		return fmt.Errorf("api client retry/raw: %w", err)
	}
	return nil
}

func verifyConfigLoad() error {
	dir, err := os.MkdirTemp("", "go-infra-ax10-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	path := dir + "/infra.yaml"
	body := strings.TrimSpace(`
hosts:
  app1:
    fqdn: app1.example.test
    ip: 192.0.2.10
    role: app
    ssh:
      user: deploy
      key: /tmp/ax10.key
  build:
    fqdn: build.example.test
    ip: 192.0.2.20
    role: builder
    ssh:
      user: builder
      key: /tmp/ax10-build.key
      port: 2222
load_balancer:
  name: ax10-lb
  backends:
    - host: app1
      port: 8080
dns:
  provider: cloudns
  zones:
    example.test:
      records:
        - name: app
          type: A
          value: 192.0.2.10
          ttl: 300
`) + "\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return err
	}

	cfg, err := infra.Load(path)
	if err != nil {
		return err
	}
	if got := len(cfg.Hosts); got != 2 {
		return fmt.Errorf("hosts = %d, want 2", got)
	}
	if got := cfg.Hosts["app1"].SSH.Port; got != 22 {
		return fmt.Errorf("default ssh port = %d, want 22", got)
	}
	if got := cfg.Hosts["build"].SSH.Port; got != 2222 {
		return fmt.Errorf("custom ssh port = %d, want 2222", got)
	}
	if got := len(cfg.AppServers()); got != 1 {
		return fmt.Errorf("app servers = %d, want 1", got)
	}
	if cfg.LoadBalancer.Name != "ax10-lb" {
		return fmt.Errorf("load balancer name = %q", cfg.LoadBalancer.Name)
	}

	discovered, discoveredPath, err := infra.Discover(dir)
	if err != nil {
		return err
	}
	if discoveredPath != path {
		return fmt.Errorf("discover path = %q, want %q", discoveredPath, path)
	}
	if discovered.Hosts["app1"].FQDN != "app1.example.test" {
		return fmt.Errorf("discovered app host = %q", discovered.Hosts["app1"].FQDN)
	}

	return nil
}

func verifyAPIClientJSON() error {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true,"name":"ax10"}`)
	}))
	defer server.Close()

	client := infra.NewAPIClient(
		infra.WithHTTPClient(server.Client()),
		infra.WithAuth(func(req *http.Request) {
			req.Header.Set("Authorization", "Bearer ax10")
		}),
		infra.WithRetry(infra.RetryConfig{
			MaxRetries:     0,
			InitialBackoff: time.Millisecond,
			MaxBackoff:     time.Millisecond,
		}),
		infra.WithPrefix("ax10 API"),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	if err != nil {
		return err
	}

	var result struct {
		OK   bool   `json:"ok"`
		Name string `json:"name"`
	}
	if err := client.Do(req, &result); err != nil {
		return err
	}
	if gotAuth != "Bearer ax10" {
		return fmt.Errorf("auth header = %q", gotAuth)
	}
	if !result.OK || result.Name != "ax10" {
		return fmt.Errorf("json result = %+v", result)
	}
	return nil
}

func verifyAPIClientRetryAndRaw() error {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) == 1 {
			http.Error(w, "temporary", http.StatusBadGateway)
			return
		}
		fmt.Fprint(w, "raw-ok")
	}))
	defer server.Close()

	client := infra.NewAPIClient(
		infra.WithHTTPClient(server.Client()),
		infra.WithRetry(infra.RetryConfig{
			MaxRetries:     1,
			InitialBackoff: time.Millisecond,
			MaxBackoff:     time.Millisecond,
		}),
		infra.WithPrefix("ax10 API"),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	if err != nil {
		return err
	}

	body, err := client.DoRaw(req)
	if err != nil {
		return err
	}
	if string(body) != "raw-ok" {
		return fmt.Errorf("raw body = %q", body)
	}
	if atomic.LoadInt32(&attempts) != 2 {
		return fmt.Errorf("attempts = %d, want 2", attempts)
	}
	return nil
}
