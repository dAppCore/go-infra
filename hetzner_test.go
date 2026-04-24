package infra

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "dappco.re/go/core"
)

func TestHetzner_NewHCloudClient_Good(t *testing.T) {
	c := NewHCloudClient("my-token")
	if c == nil {
		t.Fatal("expected non-nil")
	}
	if "my-token" != c.token {
		t.Fatalf("want %v, got %v", "my-token", c.token)
	}
	if c.api == nil {
		t.Fatal("expected non-nil")
	}
}

func TestHetzner_HCloudClient_ListServers_Good(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/servers", func(w http.ResponseWriter, r *http.Request) {
		if "Bearer test-token" != r.Header.Get("Authorization") {
			t.Fatalf("want %v, got %v", "Bearer test-token", r.Header.Get("Authorization"))
		}
		if http.MethodGet != r.Method {
			t.Fatalf("want %v, got %v", http.MethodGet, r.Method)
		}

		resp := map[string]any{
			"servers": []map[string]any{
				{
					"id": 1, "name": "de1", "status": "running",
					"public_net":  map[string]any{"ipv4": map[string]any{"ip": "1.2.3.4"}},
					"server_type": map[string]any{"name": "cx22", "cores": 2, "memory": 4.0, "disk": 40},
					"datacenter":  map[string]any{"name": "fsn1-dc14"},
				},
				{
					"id": 2, "name": "de2", "status": "running",
					"public_net":  map[string]any{"ipv4": map[string]any{"ip": "5.6.7.8"}},
					"server_type": map[string]any{"name": "cx32", "cores": 4, "memory": 8.0, "disk": 80},
					"datacenter":  map[string]any{"name": "nbg1-dc3"},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		writeCoreJSON(t, w, resp)
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := NewHCloudClient("test-token")
	client.baseURL = ts.URL
	client.api = NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithAuth(func(req *http.Request) {
			req.Header.Set("Authorization", "Bearer "+client.token)
		}),
		WithPrefix("hcloud API"),
		WithRetry(RetryConfig{}), // no retries in tests
	)

	servers, err := client.ListServers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(servers) != 2 {
		t.Fatalf("want length %v, got %v", 2, len(servers))
	}
	if "de1" != servers[0].Name {
		t.Fatalf("want %v, got %v", "de1", servers[0].Name)
	}
	if "de2" != servers[1].Name {
		t.Fatalf("want %v, got %v", "de2", servers[1].Name)
	}
}

func TestHetzner_HCloudClient_Do_ParsesJSON_Good(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "Bearer test-token" != r.Header.Get("Authorization") {
			t.Fatalf("want %v, got %v", "Bearer test-token", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"servers":[{"id":1,"name":"test","status":"running"}]}`))
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
		WithRetry(RetryConfig{}),
	)

	var result struct {
		Servers []HCloudServer `json:"servers"`
	}
	err := client.get(context.Background(), "/servers", &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Servers) != 1 {
		t.Fatalf("want length %v, got %v", 1, len(result.Servers))
	}
	if 1 != result.Servers[0].ID {
		t.Fatalf("want %v, got %v", 1, result.Servers[0].ID)
	}
	if "test" != result.Servers[0].Name {
		t.Fatalf("want %v, got %v", "test", result.Servers[0].Name)
	}
	if "running" != result.Servers[0].Status {
		t.Fatalf("want %v, got %v", "running", result.Servers[0].Status)
	}
}

func TestHetzner_HCloudClient_Do_APIError_Bad(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"code":"forbidden","message":"insufficient permissions"}}`))
	}))
	defer ts.Close()

	client := NewHCloudClient("bad-token")
	client.baseURL = ts.URL
	client.api = NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithAuth(func(req *http.Request) {
			req.Header.Set("Authorization", "Bearer bad-token")
		}),
		WithPrefix("hcloud API"),
		WithRetry(RetryConfig{}),
	)

	var result struct{}
	err := client.get(context.Background(), "/servers", &result)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "hcloud API: HTTP 403") {
		t.Fatalf("expected %v to contain %v", err.Error(), "hcloud API: HTTP 403")
	}
}

func TestHetzner_HCloudClient_Do_APIErrorNoJSON_Bad(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`Internal Server Error`))
	}))
	defer ts.Close()

	client := NewHCloudClient("test-token")
	client.baseURL = ts.URL
	client.api = NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithPrefix("hcloud API"),
		WithRetry(RetryConfig{}),
	)

	err := client.get(context.Background(), "/servers", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "hcloud API: HTTP 500") {
		t.Fatalf("expected %v to contain %v", err.Error(), "hcloud API: HTTP 500")
	}
}

func TestHetzner_HCloudClient_Do_NilResult_Good(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	client := NewHCloudClient("test-token")
	client.baseURL = ts.URL
	client.api = NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithPrefix("hcloud API"),
		WithRetry(RetryConfig{}),
	)

	err := client.delete(context.Background(), "/servers/1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Hetzner Robot API ---

func TestHetzner_NewHRobotClient_Good(t *testing.T) {
	c := NewHRobotClient("user", "pass")
	if c == nil {
		t.Fatal("expected non-nil")
	}
	if "user" != c.user {
		t.Fatalf("want %v, got %v", "user", c.user)
	}
	if "pass" != c.password {
		t.Fatalf("want %v, got %v", "pass", c.password)
	}
	if c.api == nil {
		t.Fatal("expected non-nil")
	}
}

func TestHetzner_HRobotClient_ListServers_Good(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok {
			t.Fatal("expected true")
		}
		if "testuser" != user {
			t.Fatalf("want %v, got %v", "testuser", user)
		}
		if "testpass" != pass {
			t.Fatalf("want %v, got %v", "testpass", pass)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"server":{"server_ip":"1.2.3.4","server_name":"test","product":"EX44","dc":"FSN1","status":"ready","cancelled":false}}]`))
	}))
	defer ts.Close()

	client := NewHRobotClient("testuser", "testpass")
	client.baseURL = ts.URL
	client.api = NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithAuth(func(req *http.Request) {
			req.SetBasicAuth("testuser", "testpass")
		}),
		WithPrefix("hrobot API"),
		WithRetry(RetryConfig{}),
	)

	servers, err := client.ListServers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("want length %v, got %v", 1, len(servers))
	}
	if "1.2.3.4" != servers[0].ServerIP {
		t.Fatalf("want %v, got %v", "1.2.3.4", servers[0].ServerIP)
	}
	if "test" != servers[0].ServerName {
		t.Fatalf("want %v, got %v", "test", servers[0].ServerName)
	}
	if "EX44" != servers[0].Product {
		t.Fatalf("want %v, got %v", "EX44", servers[0].Product)
	}
}

func TestHetzner_HRobotClient_Get_HTTPError_Bad(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"status":401,"code":"UNAUTHORIZED","message":"Invalid credentials"}}`))
	}))
	defer ts.Close()

	client := NewHRobotClient("bad", "creds")
	client.baseURL = ts.URL
	client.api = NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithAuth(func(req *http.Request) {
			req.SetBasicAuth("bad", "creds")
		}),
		WithPrefix("hrobot API"),
		WithRetry(RetryConfig{}),
	)

	err := client.get(context.Background(), "/server", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "hrobot API: HTTP 401") {
		t.Fatalf("expected %v to contain %v", err.Error(), "hrobot API: HTTP 401")
	}
}

func TestHetzner_HCloudClient_ListLoadBalancers_Good(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if http.MethodGet != r.Method {
			t.Fatalf("want %v, got %v", http.MethodGet, r.Method)
		}
		if "/load_balancers" != r.URL.Path {
			t.Fatalf("want %v, got %v", "/load_balancers", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"load_balancers":[{"id":789,"name":"hermes","public_net":{"enabled":true,"ipv4":{"ip":"5.6.7.8"}}}]}`))
	}))
	defer ts.Close()

	client := NewHCloudClient("test-token")
	client.baseURL = ts.URL
	client.api = NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithPrefix("hcloud API"),
		WithRetry(RetryConfig{}),
	)

	lbs, err := client.ListLoadBalancers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lbs) != 1 {
		t.Fatalf("want length %v, got %v", 1, len(lbs))
	}
	if "hermes" != lbs[0].Name {
		t.Fatalf("want %v, got %v", "hermes", lbs[0].Name)
	}
	if 789 != lbs[0].ID {
		t.Fatalf("want %v, got %v", 789, lbs[0].ID)
	}
}

func TestHetzner_HCloudClient_GetLoadBalancer_Good(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "/load_balancers/789" != r.URL.Path {
			t.Fatalf("want %v, got %v", "/load_balancers/789", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"load_balancer":{"id":789,"name":"hermes","public_net":{"enabled":true,"ipv4":{"ip":"5.6.7.8"}}}}`))
	}))
	defer ts.Close()

	client := NewHCloudClient("test-token")
	client.baseURL = ts.URL
	client.api = NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithPrefix("hcloud API"),
		WithRetry(RetryConfig{}),
	)

	lb, err := client.GetLoadBalancer(context.Background(), 789)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if "hermes" != lb.Name {
		t.Fatalf("want %v, got %v", "hermes", lb.Name)
	}
}

func TestHetzner_HCloudClient_CreateLoadBalancer_Good(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if http.MethodPost != r.Method {
			t.Fatalf("want %v, got %v", http.MethodPost, r.Method)
		}
		if "/load_balancers" != r.URL.Path {
			t.Fatalf("want %v, got %v", "/load_balancers", r.URL.Path)
		}

		var body HCloudLBCreateRequest
		decodeCoreJSONBody(t, r, &body)
		if "hermes" != body.Name {
			t.Fatalf("want %v, got %v", "hermes", body.Name)
		}
		if "lb11" != body.LoadBalancerType {
			t.Fatalf("want %v, got %v", "lb11", body.LoadBalancerType)
		}
		if "round_robin" != body.Algorithm.Type {
			t.Fatalf("want %v, got %v", "round_robin", body.Algorithm.Type)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"load_balancer":{"id":789,"name":"hermes","public_net":{"enabled":true,"ipv4":{"ip":"5.6.7.8"}}}}`))
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
		WithRetry(RetryConfig{}),
	)

	lb, err := client.CreateLoadBalancer(context.Background(), HCloudLBCreateRequest{
		Name:             "hermes",
		LoadBalancerType: "lb11",
		Location:         "fsn1",
		Algorithm:        HCloudLBAlgorithm{Type: "round_robin"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if "hermes" != lb.Name {
		t.Fatalf("want %v, got %v", "hermes", lb.Name)
	}
	if 789 != lb.ID {
		t.Fatalf("want %v, got %v", 789, lb.ID)
	}
}

func TestHetzner_HCloudClient_DeleteLoadBalancer_Good(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if http.MethodDelete != r.Method {
			t.Fatalf("want %v, got %v", http.MethodDelete, r.Method)
		}
		if "/load_balancers/789" != r.URL.Path {
			t.Fatalf("want %v, got %v", "/load_balancers/789", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	client := NewHCloudClient("test-token")
	client.baseURL = ts.URL
	client.api = NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithPrefix("hcloud API"),
		WithRetry(RetryConfig{}),
	)

	err := client.DeleteLoadBalancer(context.Background(), 789)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHetzner_HCloudClient_CreateSnapshot_Good(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if http.MethodPost != r.Method {
			t.Fatalf("want %v, got %v", http.MethodPost, r.Method)
		}
		if "/servers/123/actions/create_image" != r.URL.Path {
			t.Fatalf("want %v, got %v", "/servers/123/actions/create_image", r.URL.Path)
		}

		var body map[string]string
		decodeCoreJSONBody(t, r, &body)
		if "daily backup" != body["description"] {
			t.Fatalf("want %v, got %v", "daily backup", body["description"])
		}
		if "snapshot" != body["type"] {
			t.Fatalf("want %v, got %v", "snapshot", body["type"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"image":{"id":456}}`))
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
		WithRetry(RetryConfig{}),
	)

	err := client.CreateSnapshot(context.Background(), 123, "daily backup")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHetzner_HRobotClient_GetServer_Good(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "/server/1.2.3.4" != r.URL.Path {
			t.Fatalf("want %v, got %v", "/server/1.2.3.4", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"server":{"server_ip":"1.2.3.4","server_name":"noc","product":"EX44","dc":"FSN1","status":"ready","cancelled":false}}`))
	}))
	defer ts.Close()

	client := NewHRobotClient("testuser", "testpass")
	client.baseURL = ts.URL
	client.api = NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithAuth(func(req *http.Request) {
			req.SetBasicAuth("testuser", "testpass")
		}),
		WithPrefix("hrobot API"),
		WithRetry(RetryConfig{}),
	)

	server, err := client.GetServer(context.Background(), "1.2.3.4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if "noc" != server.ServerName {
		t.Fatalf("want %v, got %v", "noc", server.ServerName)
	}
	if "EX44" != server.Product {
		t.Fatalf("want %v, got %v", "EX44", server.Product)
	}
}

// --- Type serialisation ---

func TestHetzner_HCloudServer_JSON_Good(t *testing.T) {
	data := `{
		"id": 123,
		"name": "web-1",
		"status": "running",
		"public_net": {"ipv4": {"ip": "10.0.0.1"}},
		"private_net": [{"ip": "10.0.1.1", "network": 456}],
		"server_type": {"name": "cx22", "cores": 2, "memory": 4.0, "disk": 40},
		"datacenter": {"name": "fsn1-dc14"},
		"labels": {"env": "prod"}
	}`

	var server HCloudServer
	requireHetznerJSON(t, data, &server)
	if 123 != server.ID {
		t.Fatalf("want %v, got %v", 123, server.ID)
	}
	if "web-1" != server.Name {
		t.Fatalf("want %v, got %v", "web-1", server.Name)
	}
	if "running" != server.Status {
		t.Fatalf("want %v, got %v", "running", server.Status)
	}
	if "10.0.0.1" != server.PublicNet.IPv4.IP {
		t.Fatalf("want %v, got %v", "10.0.0.1", server.PublicNet.IPv4.IP)
	}
	if len(server.PrivateNet) != 1 {
		t.Fatalf("want length %v, got %v", 1, len(server.PrivateNet))
	}
	if "10.0.1.1" != server.PrivateNet[0].IP {
		t.Fatalf("want %v, got %v", "10.0.1.1", server.PrivateNet[0].IP)
	}
	if "cx22" != server.ServerType.Name {
		t.Fatalf("want %v, got %v", "cx22", server.ServerType.Name)
	}
	if 2 != server.ServerType.Cores {
		t.Fatalf("want %v, got %v", 2, server.ServerType.Cores)
	}
	if 4.0 != server.ServerType.Memory {
		t.Fatalf("want %v, got %v", 4.0, server.ServerType.Memory)
	}
	if "fsn1-dc14" != server.Datacenter.Name {
		t.Fatalf("want %v, got %v", "fsn1-dc14", server.Datacenter.Name)
	}
	if "prod" != server.Labels["env"] {
		t.Fatalf("want %v, got %v", "prod", server.Labels["env"])
	}
}

func TestHetzner_HCloudLoadBalancer_JSON_Good(t *testing.T) {
	data := `{
		"id": 789,
		"name": "hermes",
		"public_net": {"enabled": true, "ipv4": {"ip": "5.6.7.8"}},
		"algorithm": {"type": "round_robin"},
		"services": [
			{"protocol": "https", "listen_port": 443, "destination_port": 8080, "proxyprotocol": true}
		],
		"targets": [
			{"type": "ip", "ip": {"ip": "10.0.0.1"}, "health_status": [{"listen_port": 443, "status": "healthy"}]}
		],
		"labels": {"role": "lb"}
	}`

	var lb HCloudLoadBalancer
	requireHetznerJSON(t, data, &lb)
	if 789 != lb.ID {
		t.Fatalf("want %v, got %v", 789, lb.ID)
	}
	if "hermes" != lb.Name {
		t.Fatalf("want %v, got %v", "hermes", lb.Name)
	}
	if !lb.PublicNet.Enabled {
		t.Fatal("expected true")
	}
	if "5.6.7.8" != lb.PublicNet.IPv4.IP {
		t.Fatalf("want %v, got %v", "5.6.7.8", lb.PublicNet.IPv4.IP)
	}
	if "round_robin" != lb.Algorithm.Type {
		t.Fatalf("want %v, got %v", "round_robin", lb.Algorithm.Type)
	}
	if len(lb.Services) != 1 {
		t.Fatalf("want length %v, got %v", 1, len(lb.Services))
	}
	if 443 != lb.Services[0].ListenPort {
		t.Fatalf("want %v, got %v", 443, lb.Services[0].ListenPort)
	}
	if !lb.Services[0].Proxyprotocol {
		t.Fatal("expected true")
	}
	if len(lb.Targets) != 1 {
		t.Fatalf("want length %v, got %v", 1, len(lb.Targets))
	}
	if "ip" != lb.Targets[0].Type {
		t.Fatalf("want %v, got %v", "ip", lb.Targets[0].Type)
	}
	if "10.0.0.1" != lb.Targets[0].IP.IP {
		t.Fatalf("want %v, got %v", "10.0.0.1", lb.Targets[0].IP.IP)
	}
	if "healthy" != lb.Targets[0].HealthStatus[0].Status {
		t.Fatalf("want %v, got %v", "healthy", lb.Targets[0].HealthStatus[0].Status)
	}
}

func TestHetzner_HRobotServer_JSON_Good(t *testing.T) {
	data := `{
		"server_ip": "1.2.3.4",
		"server_name": "noc",
		"product": "EX44",
		"dc": "FSN1-DC14",
		"status": "ready",
		"cancelled": false,
		"paid_until": "2026-03-01"
	}`

	var server HRobotServer
	requireHetznerJSON(t, data, &server)
	if "1.2.3.4" != server.ServerIP {
		t.Fatalf("want %v, got %v", "1.2.3.4", server.ServerIP)
	}
	if "noc" != server.ServerName {
		t.Fatalf("want %v, got %v", "noc", server.ServerName)
	}
	if "EX44" != server.Product {
		t.Fatalf("want %v, got %v", "EX44", server.Product)
	}
	if "FSN1-DC14" != server.Datacenter {
		t.Fatalf("want %v, got %v", "FSN1-DC14", server.Datacenter)
	}
	if "ready" != server.Status {
		t.Fatalf("want %v, got %v", "ready", server.Status)
	}
	if server.Cancelled {
		t.Fatal("expected false")
	}
	if "2026-03-01" != server.PaidUntil {
		t.Fatalf("want %v, got %v", "2026-03-01", server.PaidUntil)
	}
}

func requireHetznerJSON(t *testing.T, data string, target any) {
	t.Helper()

	r := core.JSONUnmarshal([]byte(data), target)
	if !r.OK {
		t.Fatal("expected true")
	}
}

func writeCoreJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()

	r := core.JSONMarshal(value)
	if !r.OK {
		t.Fatal("expected true")
	}
	_, err := w.Write(r.Value.([]byte))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func decodeCoreJSONBody(t *testing.T, r *http.Request, target any) {
	t.Helper()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result := core.JSONUnmarshal(body, target)
	if !result.OK {
		t.Fatal("expected true")
	}
}
