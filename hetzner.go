package infra

import (
	"context"
	"net/http"

	core "dappco.re/go/core"
)

const (
	hcloudBaseURL = "https://api.hetzner.cloud/v1"
	hrobotBaseURL = "https://robot-ws.your-server.de"
)

// HCloudClient is an HTTP client for the Hetzner Cloud API.
// Usage: hc := infra.NewHCloudClient(token)
type HCloudClient struct {
	token   string
	baseURL string
	api     *APIClient
}

// NewHCloudClient creates a new Hetzner Cloud API client.
// Usage: hc := infra.NewHCloudClient(token)
func NewHCloudClient(token string) *HCloudClient {
	c := &HCloudClient{
		token:   token,
		baseURL: hcloudBaseURL,
	}
	c.api = NewAPIClient(
		WithAuth(func(req *http.Request) {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}),
		WithPrefix("hcloud API"),
	)
	return c
}

// HCloudServer represents a Hetzner Cloud server.
// Usage: server := infra.HCloudServer{}
type HCloudServer struct {
	ID         int                `json:"id"`
	Name       string             `json:"name"`
	Status     string             `json:"status"`
	PublicNet  HCloudPublicNet    `json:"public_net"`
	PrivateNet []HCloudPrivateNet `json:"private_net"`
	ServerType HCloudServerType   `json:"server_type"`
	Datacenter HCloudDatacenter   `json:"datacenter"`
	Labels     map[string]string  `json:"labels"`
}

// HCloudPublicNet holds public network info.
// Usage: net := infra.HCloudPublicNet{}
type HCloudPublicNet struct {
	IPv4 HCloudIPv4 `json:"ipv4"`
}

// HCloudIPv4 holds an IPv4 address.
// Usage: ip := infra.HCloudIPv4{}
type HCloudIPv4 struct {
	IP string `json:"ip"`
}

// HCloudPrivateNet holds private network info.
// Usage: net := infra.HCloudPrivateNet{}
type HCloudPrivateNet struct {
	IP      string `json:"ip"`
	Network int    `json:"network"`
}

// HCloudServerType holds server type info.
// Usage: serverType := infra.HCloudServerType{}
type HCloudServerType struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Cores       int     `json:"cores"`
	Memory      float64 `json:"memory"`
	Disk        int     `json:"disk"`
}

// HCloudDatacenter holds datacenter info.
// Usage: dc := infra.HCloudDatacenter{}
type HCloudDatacenter struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// HCloudLoadBalancer represents a Hetzner Cloud load balancer.
// Usage: lb := infra.HCloudLoadBalancer{}
type HCloudLoadBalancer struct {
	ID        int               `json:"id"`
	Name      string            `json:"name"`
	PublicNet HCloudLBPublicNet `json:"public_net"`
	Algorithm HCloudLBAlgorithm `json:"algorithm"`
	Services  []HCloudLBService `json:"services"`
	Targets   []HCloudLBTarget  `json:"targets"`
	Location  HCloudDatacenter  `json:"location"`
	Labels    map[string]string `json:"labels"`
}

// HCloudLBPublicNet holds LB public network info.
// Usage: net := infra.HCloudLBPublicNet{}
type HCloudLBPublicNet struct {
	Enabled bool       `json:"enabled"`
	IPv4    HCloudIPv4 `json:"ipv4"`
}

// HCloudLBAlgorithm holds the LB algorithm.
// Usage: algo := infra.HCloudLBAlgorithm{}
type HCloudLBAlgorithm struct {
	Type string `json:"type"`
}

// HCloudLBService describes an LB listener.
// Usage: service := infra.HCloudLBService{}
type HCloudLBService struct {
	Protocol        string               `json:"protocol"`
	ListenPort      int                  `json:"listen_port"`
	DestinationPort int                  `json:"destination_port"`
	Proxyprotocol   bool                 `json:"proxyprotocol"`
	HTTP            *HCloudLBHTTP        `json:"http,omitempty"`
	HealthCheck     *HCloudLBHealthCheck `json:"health_check,omitempty"`
}

// HCloudLBHTTP holds HTTP-specific LB options.
// Usage: httpCfg := infra.HCloudLBHTTP{}
type HCloudLBHTTP struct {
	RedirectHTTP bool `json:"redirect_http"`
}

// HCloudLBHealthCheck holds LB health check config.
// Usage: check := infra.HCloudLBHealthCheck{}
type HCloudLBHealthCheck struct {
	Protocol string          `json:"protocol"`
	Port     int             `json:"port"`
	Interval int             `json:"interval"`
	Timeout  int             `json:"timeout"`
	Retries  int             `json:"retries"`
	HTTP     *HCloudLBHCHTTP `json:"http,omitempty"`
}

// HCloudLBHCHTTP holds HTTP health check options.
// Usage: httpCheck := infra.HCloudLBHCHTTP{}
type HCloudLBHCHTTP struct {
	Path       string `json:"path"`
	StatusCode string `json:"status_codes"`
}

// HCloudLBTarget is a load balancer backend target.
// Usage: target := infra.HCloudLBTarget{}
type HCloudLBTarget struct {
	Type         string                 `json:"type"`
	IP           *HCloudLBTargetIP      `json:"ip,omitempty"`
	Server       *HCloudLBTargetServer  `json:"server,omitempty"`
	HealthStatus []HCloudLBHealthStatus `json:"health_status"`
}

// HCloudLBTargetIP is an IP-based LB target.
// Usage: target := infra.HCloudLBTargetIP{}
type HCloudLBTargetIP struct {
	IP string `json:"ip"`
}

// HCloudLBTargetServer is a server-based LB target.
// Usage: target := infra.HCloudLBTargetServer{}
type HCloudLBTargetServer struct {
	ID int `json:"id"`
}

// HCloudLBHealthStatus holds target health info.
// Usage: status := infra.HCloudLBHealthStatus{}
type HCloudLBHealthStatus struct {
	ListenPort int    `json:"listen_port"`
	Status     string `json:"status"`
}

// HCloudLBCreateRequest holds load balancer creation params.
// Usage: req := infra.HCloudLBCreateRequest{}
type HCloudLBCreateRequest struct {
	Name             string                 `json:"name"`
	LoadBalancerType string                 `json:"load_balancer_type"`
	Location         string                 `json:"location"`
	Algorithm        HCloudLBAlgorithm      `json:"algorithm"`
	Services         []HCloudLBService      `json:"services"`
	Targets          []HCloudLBCreateTarget `json:"targets"`
	Labels           map[string]string      `json:"labels"`
}

// HCloudLBCreateTarget is a target for LB creation.
// Usage: target := infra.HCloudLBCreateTarget{}
type HCloudLBCreateTarget struct {
	Type string            `json:"type"`
	IP   *HCloudLBTargetIP `json:"ip,omitempty"`
}

// ListServers returns all Hetzner Cloud servers.
// Usage: servers, err := hc.ListServers(ctx)
func (c *HCloudClient) ListServers(ctx context.Context) ([]HCloudServer, error) {
	var result struct {
		Servers []HCloudServer `json:"servers"`
	}
	if err := c.get(ctx, "/servers", &result); err != nil {
		return nil, err
	}
	return result.Servers, nil
}

// ListLoadBalancers returns all load balancers.
// Usage: lbs, err := hc.ListLoadBalancers(ctx)
func (c *HCloudClient) ListLoadBalancers(ctx context.Context) ([]HCloudLoadBalancer, error) {
	var result struct {
		LoadBalancers []HCloudLoadBalancer `json:"load_balancers"`
	}
	if err := c.get(ctx, "/load_balancers", &result); err != nil {
		return nil, err
	}
	return result.LoadBalancers, nil
}

// GetLoadBalancer returns a load balancer by ID.
// Usage: lb, err := hc.GetLoadBalancer(ctx, 1)
func (c *HCloudClient) GetLoadBalancer(ctx context.Context, id int) (*HCloudLoadBalancer, error) {
	var result struct {
		LoadBalancer HCloudLoadBalancer `json:"load_balancer"`
	}
	if err := c.get(ctx, core.Sprintf("/load_balancers/%d", id), &result); err != nil {
		return nil, err
	}
	return &result.LoadBalancer, nil
}

// CreateLoadBalancer creates a new load balancer.
// Usage: lb, err := hc.CreateLoadBalancer(ctx, req)
func (c *HCloudClient) CreateLoadBalancer(ctx context.Context, req HCloudLBCreateRequest) (*HCloudLoadBalancer, error) {
	marshaled := core.JSONMarshal(req)
	if !marshaled.OK {
		return nil, core.E("HCloudClient.CreateLoadBalancer", "marshal request", coreResultErr(marshaled, "HCloudClient.CreateLoadBalancer"))
	}
	body := marshaled.Value.([]byte)

	var result struct {
		LoadBalancer HCloudLoadBalancer `json:"load_balancer"`
	}
	if err := c.post(ctx, "/load_balancers", body, &result); err != nil {
		return nil, err
	}
	return &result.LoadBalancer, nil
}

// DeleteLoadBalancer deletes a load balancer by ID.
// Usage: err := hc.DeleteLoadBalancer(ctx, 1)
func (c *HCloudClient) DeleteLoadBalancer(ctx context.Context, id int) error {
	return c.delete(ctx, core.Sprintf("/load_balancers/%d", id))
}

// CreateSnapshot creates a server snapshot.
// Usage: err := hc.CreateSnapshot(ctx, 1, "daily snapshot")
func (c *HCloudClient) CreateSnapshot(ctx context.Context, serverID int, description string) error {
	marshaled := core.JSONMarshal(map[string]string{
		"description": description,
		"type":        "snapshot",
	})
	if !marshaled.OK {
		return core.E("HCloudClient.CreateSnapshot", "marshal request", coreResultErr(marshaled, "HCloudClient.CreateSnapshot"))
	}
	return c.post(ctx, core.Sprintf("/servers/%d/actions/create_image", serverID), marshaled.Value.([]byte), nil)
}

func (c *HCloudClient) get(ctx context.Context, path string, result any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	return c.do(req, result)
}

func (c *HCloudClient) post(ctx context.Context, path string, body []byte, result any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, core.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, result)
}

func (c *HCloudClient) delete(ctx context.Context, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

func (c *HCloudClient) do(req *http.Request, result any) error {
	return c.api.Do(req, result)
}

// --- Hetzner Robot API ---

// HRobotClient is an HTTP client for the Hetzner Robot API.
// Usage: hr := infra.NewHRobotClient(user, password)
type HRobotClient struct {
	user     string
	password string
	baseURL  string
	api      *APIClient
}

// NewHRobotClient creates a new Hetzner Robot API client.
// Usage: hr := infra.NewHRobotClient(user, password)
func NewHRobotClient(user, password string) *HRobotClient {
	c := &HRobotClient{
		user:     user,
		password: password,
		baseURL:  hrobotBaseURL,
	}
	c.api = NewAPIClient(
		WithAuth(func(req *http.Request) {
			req.SetBasicAuth(c.user, c.password)
		}),
		WithPrefix("hrobot API"),
	)
	return c
}

// HRobotServer represents a Hetzner Robot dedicated server.
// Usage: server := infra.HRobotServer{}
type HRobotServer struct {
	ServerIP   string `json:"server_ip"`
	ServerName string `json:"server_name"`
	Product    string `json:"product"`
	Datacenter string `json:"dc"`
	Status     string `json:"status"`
	Cancelled  bool   `json:"cancelled"`
	PaidUntil  string `json:"paid_until"`
}

// ListServers returns all Robot dedicated servers.
// Usage: servers, err := hr.ListServers(ctx)
func (c *HRobotClient) ListServers(ctx context.Context) ([]HRobotServer, error) {
	var raw []struct {
		Server HRobotServer `json:"server"`
	}
	if err := c.get(ctx, "/server", &raw); err != nil {
		return nil, err
	}

	servers := make([]HRobotServer, len(raw))
	for i, s := range raw {
		servers[i] = s.Server
	}
	return servers, nil
}

// GetServer returns a Robot server by IP.
// Usage: server, err := hr.GetServer(ctx, "203.0.113.10")
func (c *HRobotClient) GetServer(ctx context.Context, ip string) (*HRobotServer, error) {
	var raw struct {
		Server HRobotServer `json:"server"`
	}
	if err := c.get(ctx, "/server/"+ip, &raw); err != nil {
		return nil, err
	}
	return &raw.Server, nil
}

func (c *HRobotClient) get(ctx context.Context, path string, result any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	return c.api.Do(req, result)
}
