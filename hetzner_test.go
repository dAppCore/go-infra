package infra

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHCloudClient_Good(t *testing.T) {
	c := NewHCloudClient("my-token")
	assert.NotNil(t, c)
	assert.Equal(t, "my-token", c.token)
	assert.NotNil(t, c.api)
}

func TestHCloudClient_ListServers_Good(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/servers", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, http.MethodGet, r.Method)

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
		_ = json.NewEncoder(w).Encode(resp)
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
	require.NoError(t, err)
	require.Len(t, servers, 2)
	assert.Equal(t, "de1", servers[0].Name)
	assert.Equal(t, "de2", servers[1].Name)
}

func TestHCloudClient_Do_Good_ParsesJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
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
	require.NoError(t, err)
	require.Len(t, result.Servers, 1)
	assert.Equal(t, 1, result.Servers[0].ID)
	assert.Equal(t, "test", result.Servers[0].Name)
	assert.Equal(t, "running", result.Servers[0].Status)
}

func TestHCloudClient_Do_Bad_APIError(t *testing.T) {
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
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "hcloud API 403")
}

func TestHCloudClient_Do_Bad_APIErrorNoJSON(t *testing.T) {
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
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "hcloud API 500")
}

func TestHCloudClient_Do_Good_NilResult(t *testing.T) {
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
	assert.NoError(t, err)
}

// --- Hetzner Robot API ---

func TestNewHRobotClient_Good(t *testing.T) {
	c := NewHRobotClient("user", "pass")
	assert.NotNil(t, c)
	assert.Equal(t, "user", c.user)
	assert.Equal(t, "pass", c.password)
	assert.NotNil(t, c.api)
}

func TestHRobotClient_ListServers_Good(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "testuser", user)
		assert.Equal(t, "testpass", pass)

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
	require.NoError(t, err)
	require.Len(t, servers, 1)
	assert.Equal(t, "1.2.3.4", servers[0].ServerIP)
	assert.Equal(t, "test", servers[0].ServerName)
	assert.Equal(t, "EX44", servers[0].Product)
}

func TestHRobotClient_Get_Bad_HTTPError(t *testing.T) {
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
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "hrobot API 401")
}

// --- Type serialisation ---

func TestHCloudServer_JSON_Good(t *testing.T) {
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
	err := json.Unmarshal([]byte(data), &server)

	require.NoError(t, err)
	assert.Equal(t, 123, server.ID)
	assert.Equal(t, "web-1", server.Name)
	assert.Equal(t, "running", server.Status)
	assert.Equal(t, "10.0.0.1", server.PublicNet.IPv4.IP)
	assert.Len(t, server.PrivateNet, 1)
	assert.Equal(t, "10.0.1.1", server.PrivateNet[0].IP)
	assert.Equal(t, "cx22", server.ServerType.Name)
	assert.Equal(t, 2, server.ServerType.Cores)
	assert.Equal(t, 4.0, server.ServerType.Memory)
	assert.Equal(t, "fsn1-dc14", server.Datacenter.Name)
	assert.Equal(t, "prod", server.Labels["env"])
}

func TestHCloudLoadBalancer_JSON_Good(t *testing.T) {
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
	err := json.Unmarshal([]byte(data), &lb)

	require.NoError(t, err)
	assert.Equal(t, 789, lb.ID)
	assert.Equal(t, "hermes", lb.Name)
	assert.True(t, lb.PublicNet.Enabled)
	assert.Equal(t, "5.6.7.8", lb.PublicNet.IPv4.IP)
	assert.Equal(t, "round_robin", lb.Algorithm.Type)
	require.Len(t, lb.Services, 1)
	assert.Equal(t, 443, lb.Services[0].ListenPort)
	assert.True(t, lb.Services[0].Proxyprotocol)
	require.Len(t, lb.Targets, 1)
	assert.Equal(t, "ip", lb.Targets[0].Type)
	assert.Equal(t, "10.0.0.1", lb.Targets[0].IP.IP)
	assert.Equal(t, "healthy", lb.Targets[0].HealthStatus[0].Status)
}

func TestHRobotServer_JSON_Good(t *testing.T) {
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
	err := json.Unmarshal([]byte(data), &server)

	require.NoError(t, err)
	assert.Equal(t, "1.2.3.4", server.ServerIP)
	assert.Equal(t, "noc", server.ServerName)
	assert.Equal(t, "EX44", server.Product)
	assert.Equal(t, "FSN1-DC14", server.Datacenter)
	assert.Equal(t, "ready", server.Status)
	assert.False(t, server.Cancelled)
	assert.Equal(t, "2026-03-01", server.PaidUntil)
}
