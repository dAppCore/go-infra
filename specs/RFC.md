# infra
**Import:** `forge.lthn.ai/core/go-infra`
**Files:** 5

## Types

### `RetryConfig`
Controls exponential backoff retry behaviour for `APIClient`.
- `MaxRetries int`: Maximum number of retry attempts after the initial request. `0` disables retries.
- `InitialBackoff time.Duration`: Delay before the first retry.
- `MaxBackoff time.Duration`: Upper bound for the computed backoff delay.

### `APIClient`
Shared HTTP client with retry handling, shared rate-limit blocking, pluggable authentication, and configurable error prefixes. All struct fields are unexported.
- `func (a *APIClient) Do(req *http.Request, result any) error`: Applies authentication, honours the shared rate-limit window, retries transport failures plus `429` and `5xx` responses, and JSON-decodes into `result` when `result` is non-nil.
- `func (a *APIClient) DoRaw(req *http.Request) ([]byte, error)`: Runs the same request pipeline as `Do` but returns the raw response body instead of decoding JSON.

### `APIClientOption`
`type APIClientOption func(*APIClient)`

Functional option consumed by `NewAPIClient` to mutate a newly created `APIClient`.

### `CloudNSClient`
HTTP client for the CloudNS DNS API. Authentication details, base URL, and delegated `APIClient` state are stored in unexported fields.
- `func (c *CloudNSClient) ListZones(ctx context.Context) ([]CloudNSZone, error)`: Fetches all zones visible to the configured CloudNS sub-user.
- `func (c *CloudNSClient) ListRecords(ctx context.Context, domain string) (map[string]CloudNSRecord, error)`: Fetches all records for a zone and returns them keyed by CloudNS record ID.
- `func (c *CloudNSClient) CreateRecord(ctx context.Context, domain, host, recordType, value string, ttl int) (string, error)`: Creates a record and returns the created record ID.
- `func (c *CloudNSClient) UpdateRecord(ctx context.Context, domain, recordID, host, recordType, value string, ttl int) error`: Replaces an existing CloudNS record with the supplied values.
- `func (c *CloudNSClient) DeleteRecord(ctx context.Context, domain, recordID string) error`: Deletes a record by CloudNS record ID.
- `func (c *CloudNSClient) EnsureRecord(ctx context.Context, domain, host, recordType, value string, ttl int) (bool, error)`: Idempotently creates or updates a record and reports whether a change was applied.
- `func (c *CloudNSClient) SetACMEChallenge(ctx context.Context, domain, value string) (string, error)`: Creates the `_acme-challenge` TXT record used for DNS-01 validation.
- `func (c *CloudNSClient) ClearACMEChallenge(ctx context.Context, domain string) error`: Deletes all `_acme-challenge` TXT records in the zone.

### `CloudNSZone`
CloudNS zone metadata returned by `ListZones`.
- `Name string`: Zone name as returned by CloudNS.
- `Type string`: Zone type reported by the API.
- `Zone string`: Zone domain name or zone identifier.
- `Status string`: Current CloudNS status string for the zone.

### `CloudNSRecord`
CloudNS DNS record model returned by `ListRecords`.
- `ID string`: CloudNS record ID.
- `Type string`: DNS record type such as `A`, `CNAME`, or `TXT`.
- `Host string`: Relative host label stored in the zone.
- `Record string`: Record payload or answer value.
- `TTL string`: TTL reported by CloudNS. The API returns it as a string.
- `Priority string`: Optional priority value for record types that support it.
- `Status int`: CloudNS numeric status flag for the record.

### `Config`
Top-level `infra.yaml` model loaded by `Load` and `Discover`.
- `Hosts map[string]*Host`: Named infrastructure hosts keyed by logical host name.
- `LoadBalancer LoadBalancer`: Managed load balancer definition.
- `Network Network`: Shared private network definition.
- `DNS DNS`: DNS provider configuration and desired zone state.
- `SSL SSL`: Certificate and TLS settings.
- `Database Database`: Database cluster settings.
- `Cache Cache`: Cache or session cluster settings.
- `Containers map[string]*Container`: Named container deployment definitions.
- `S3 S3Config`: Object storage configuration.
- `CDN CDN`: CDN provider configuration.
- `CICD CICD`: CI/CD service configuration.
- `Monitoring Monitoring`: Health-check and alert thresholds.
- `Backups Backups`: Backup job schedules.
- `func (c *Config) HostsByRole(role string) map[string]*Host`: Returns the subset of `Hosts` whose `Role` matches `role`.
- `func (c *Config) AppServers() map[string]*Host`: Convenience wrapper around `HostsByRole("app")`.

### `Host`
Infrastructure host definition loaded from `infra.yaml`.
- `FQDN string`: Public hostname for the machine.
- `IP string`: Primary public IP address.
- `PrivateIP string`: Optional private network IP.
- `Type string`: Provider class, such as `hcloud` or `hrobot`.
- `Role string`: Functional role such as `bastion`, `app`, or `builder`.
- `SSH SSHConf`: SSH connection settings for the host.
- `Services []string`: Services expected to run on the host.

### `SSHConf`
SSH connection settings associated with a `Host`.
- `User string`: SSH username.
- `Key string`: Path to the private key file. `Load` expands `~` and defaults are applied before use.
- `Port int`: SSH port. `Load` defaults this to `22` when omitted.

### `LoadBalancer`
Desired Hetzner managed load balancer configuration loaded from `infra.yaml`.
- `Name string`: Hetzner load balancer name.
- `FQDN string`: DNS name expected to point at the load balancer.
- `Provider string`: Provider identifier for the managed load balancer service.
- `Type string`: Hetzner load balancer type name.
- `Location string`: Hetzner location or datacenter slug.
- `Algorithm string`: Load-balancing algorithm name.
- `Backends []Backend`: Backend targets referenced by host name and port.
- `Health HealthCheck`: Health-check policy applied to listeners.
- `Listeners []Listener`: Frontend-to-backend listener mappings.
- `SSL LBCert`: TLS certificate settings for the load balancer.

### `Backend`
Load balancer backend target declared in `infra.yaml`.
- `Host string`: Host key in `Config.Hosts` to use as the backend target.
- `Port int`: Backend port to route traffic to.

### `HealthCheck`
Load balancer health-check settings.
- `Protocol string`: Protocol used for checks.
- `Path string`: HTTP path for health checks when the protocol is HTTP-based.
- `Interval int`: Probe interval in seconds.

### `Listener`
Frontend listener mapping for the managed load balancer.
- `Frontend int`: Exposed listener port.
- `Backend int`: Destination backend port.
- `Protocol string`: Listener protocol.
- `ProxyProtocol bool`: Whether the Hetzner listener should enable PROXY protocol forwarding.

### `LBCert`
TLS certificate settings for the load balancer.
- `Certificate string`: Certificate identifier or path.
- `SAN []string`: Subject alternative names covered by the certificate.

### `Network`
Private network definition from `infra.yaml`.
- `CIDR string`: Network CIDR block.
- `Name string`: Logical network name.

### `DNS`
DNS provider settings and desired zone contents.
- `Provider string`: DNS provider identifier.
- `Nameservers []string`: Authoritative nameservers for the managed domains.
- `Zones map[string]*Zone`: Desired DNS zones keyed by zone name.

### `Zone`
Desired DNS zone contents.
- `Records []DNSRecord`: Desired records for the zone.

### `DNSRecord`
Desired DNS record entry in `infra.yaml`.
- `Name string`: Record name or host label.
- `Type string`: DNS record type.
- `Value string`: Record value.
- `TTL int`: Record TTL in seconds.

### `SSL`
Top-level TLS configuration.
- `Wildcard WildcardCert`: Wildcard certificate settings.

### `WildcardCert`
Wildcard certificate request and deployment settings.
- `Domains []string`: Domain names covered by the wildcard certificate.
- `Method string`: Certificate issuance method.
- `DNSProvider string`: DNS provider used for validation.
- `Termination string`: Termination point for the certificate.

### `Database`
Database cluster configuration.
- `Engine string`: Database engine name.
- `Version string`: Engine version.
- `Cluster string`: Cluster mode or cluster identifier.
- `Nodes []DBNode`: Database nodes in the cluster.
- `SSTMethod string`: State snapshot transfer method.
- `Backup BackupConfig`: Automated backup settings for the database cluster.

### `DBNode`
Single database node definition.
- `Host string`: Host identifier for the database node.
- `Port int`: Database port.

### `BackupConfig`
Backup settings attached to `Database`.
- `Schedule string`: Backup schedule expression.
- `Destination string`: Backup destination type or endpoint.
- `Bucket string`: Bucket name when backups target object storage.
- `Prefix string`: Object key prefix for stored backups.

### `Cache`
Cache or session cluster configuration.
- `Engine string`: Cache engine name.
- `Version string`: Engine version.
- `Sentinel bool`: Whether Redis Sentinel style high availability is enabled.
- `Nodes []CacheNode`: Cache nodes in the cluster.

### `CacheNode`
Single cache node definition.
- `Host string`: Host identifier for the cache node.
- `Port int`: Cache service port.

### `Container`
Named container deployment definition.
- `Image string`: Container image reference.
- `Port int`: Optional exposed service port.
- `Runtime string`: Optional container runtime identifier.
- `Command string`: Optional command override.
- `Replicas int`: Optional replica count.
- `DependsOn []string`: Optional dependency list naming other services or components.

### `S3Config`
Object storage configuration.
- `Endpoint string`: S3-compatible endpoint URL or host.
- `Buckets map[string]*S3Bucket`: Named bucket definitions keyed by logical bucket name.

### `S3Bucket`
Single S3 bucket definition.
- `Purpose string`: Intended bucket role or usage.
- `Paths []string`: Managed paths or prefixes within the bucket.

### `CDN`
CDN configuration.
- `Provider string`: CDN provider identifier.
- `Origin string`: Origin hostname or endpoint.
- `Zones []string`: Zones served through the CDN.

### `CICD`
CI/CD service configuration.
- `Provider string`: CI/CD provider identifier.
- `URL string`: Service URL.
- `Runner string`: Runner type or runner host reference.
- `Registry string`: Container registry endpoint.
- `DeployHook string`: Deploy hook URL or tokenized endpoint.

### `Monitoring`
Monitoring and alert configuration.
- `HealthEndpoints []HealthEndpoint`: Endpoints that should be polled for health.
- `Alerts map[string]int`: Numeric thresholds keyed by alert name.

### `HealthEndpoint`
Health endpoint monitored by the platform.
- `URL string`: Endpoint URL.
- `Interval int`: Polling interval in seconds.

### `Backups`
Backup schedules grouped by cadence.
- `Daily []BackupJob`: Jobs that run daily.
- `Weekly []BackupJob`: Jobs that run weekly.

### `BackupJob`
Scheduled backup job definition.
- `Name string`: Job name.
- `Type string`: Backup type or mechanism.
- `Destination string`: Optional destination override.
- `Hosts []string`: Optional host list associated with the job.

### `HCloudClient`
HTTP client for the Hetzner Cloud API. Token, base URL, and delegated `APIClient` state are stored in unexported fields.
- `func (c *HCloudClient) ListServers(ctx context.Context) ([]HCloudServer, error)`: Lists cloud servers.
- `func (c *HCloudClient) ListLoadBalancers(ctx context.Context) ([]HCloudLoadBalancer, error)`: Lists managed load balancers.
- `func (c *HCloudClient) GetLoadBalancer(ctx context.Context, id int) (*HCloudLoadBalancer, error)`: Fetches a load balancer by numeric ID.
- `func (c *HCloudClient) CreateLoadBalancer(ctx context.Context, req HCloudLBCreateRequest) (*HCloudLoadBalancer, error)`: Creates a load balancer from the supplied request payload.
- `func (c *HCloudClient) DeleteLoadBalancer(ctx context.Context, id int) error`: Deletes a load balancer by numeric ID.
- `func (c *HCloudClient) CreateSnapshot(ctx context.Context, serverID int, description string) error`: Creates a snapshot image for a server.

### `HCloudServer`
Hetzner Cloud server model returned by `ListServers`.
- `ID int`: Hetzner server ID.
- `Name string`: Server name.
- `Status string`: Provisioning or runtime status string.
- `PublicNet HCloudPublicNet`: Public networking information.
- `PrivateNet []HCloudPrivateNet`: Attached private network interfaces.
- `ServerType HCloudServerType`: Server flavour metadata.
- `Datacenter HCloudDatacenter`: Datacenter metadata.
- `Labels map[string]string`: Hetzner labels attached to the server.

### `HCloudPublicNet`
Hetzner Cloud public network metadata.
- `IPv4 HCloudIPv4`: Primary public IPv4 address.

### `HCloudIPv4`
Hetzner IPv4 model.
- `IP string`: IPv4 address string.

### `HCloudPrivateNet`
Hetzner private network attachment.
- `IP string`: Private IP assigned to the server on the network.
- `Network int`: Numeric network ID.

### `HCloudServerType`
Hetzner server flavour metadata.
- `Name string`: Server type name.
- `Description string`: Provider description for the server type.
- `Cores int`: Number of vCPUs.
- `Memory float64`: RAM size reported by the API.
- `Disk int`: Disk size reported by the API.

### `HCloudDatacenter`
Hetzner datacenter or location metadata.
- `Name string`: Datacenter or location name.
- `Description string`: Provider description.

### `HCloudLoadBalancer`
Hetzner managed load balancer model.
- `ID int`: Load balancer ID.
- `Name string`: Load balancer name.
- `PublicNet HCloudLBPublicNet`: Public network state, including the IPv4 address.
- `Algorithm HCloudLBAlgorithm`: Load-balancing algorithm.
- `Services []HCloudLBService`: Configured listeners and service definitions.
- `Targets []HCloudLBTarget`: Attached backend targets.
- `Location HCloudDatacenter`: Location metadata.
- `Labels map[string]string`: Hetzner labels attached to the load balancer.

### `HCloudLBPublicNet`
Public network state for a Hetzner load balancer.
- `Enabled bool`: Whether public networking is enabled.
- `IPv4 HCloudIPv4`: Assigned public IPv4 address.

### `HCloudLBAlgorithm`
Hetzner load-balancing algorithm descriptor.
- `Type string`: Algorithm name.

### `HCloudLBService`
Hetzner load balancer listener or service definition.
- `Protocol string`: Listener protocol.
- `ListenPort int`: Frontend port exposed by the load balancer.
- `DestinationPort int`: Backend port targeted by the service.
- `Proxyprotocol bool`: Whether PROXY protocol forwarding is enabled. The API field name is `proxyprotocol`.
- `HTTP *HCloudLBHTTP`: Optional HTTP-specific settings.
- `HealthCheck *HCloudLBHealthCheck`: Optional health-check configuration.

### `HCloudLBHTTP`
HTTP-specific load balancer service settings.
- `RedirectHTTP bool`: Whether plain HTTP requests should be redirected.

### `HCloudLBHealthCheck`
Hetzner load balancer health-check definition.
- `Protocol string`: Health-check protocol.
- `Port int`: Backend port to probe.
- `Interval int`: Probe interval.
- `Timeout int`: Probe timeout.
- `Retries int`: Failure threshold before a target is considered unhealthy.
- `HTTP *HCloudLBHCHTTP`: Optional HTTP-specific health-check options.

### `HCloudLBHCHTTP`
HTTP-specific Hetzner health-check options.
- `Path string`: HTTP path used for the probe.
- `StatusCode string`: Expected status code matcher serialized as `status_codes`.

### `HCloudLBTarget`
Backend target attached to a Hetzner load balancer.
- `Type string`: Target type, such as `ip` or `server`.
- `IP *HCloudLBTargetIP`: IP target metadata when the target type is IP-based.
- `Server *HCloudLBTargetServer`: Server reference when the target type is server-based.
- `HealthStatus []HCloudLBHealthStatus`: Health status entries for the target.

### `HCloudLBTargetIP`
IP-based load balancer target.
- `IP string`: Backend IP address.

### `HCloudLBTargetServer`
Server-based load balancer target reference.
- `ID int`: Hetzner server ID.

### `HCloudLBHealthStatus`
Health state for one listening port on a load balancer target.
- `ListenPort int`: Listener port associated with the status.
- `Status string`: Health status string such as `healthy`.

### `HCloudLBCreateRequest`
Request payload for `HCloudClient.CreateLoadBalancer`.
- `Name string`: New load balancer name.
- `LoadBalancerType string`: Hetzner load balancer type slug.
- `Location string`: Hetzner location or datacenter slug.
- `Algorithm HCloudLBAlgorithm`: Algorithm selection.
- `Services []HCloudLBService`: Listener definitions to create.
- `Targets []HCloudLBCreateTarget`: Backend targets to attach at creation time.
- `Labels map[string]string`: Labels to apply to the new load balancer.

### `HCloudLBCreateTarget`
Target entry used during load balancer creation.
- `Type string`: Target type.
- `IP *HCloudLBTargetIP`: IP target metadata when the target is IP-based.

### `HRobotClient`
HTTP client for the Hetzner Robot API. Credentials, base URL, and delegated `APIClient` state are stored in unexported fields.
- `func (c *HRobotClient) ListServers(ctx context.Context) ([]HRobotServer, error)`: Lists dedicated servers available from Robot.
- `func (c *HRobotClient) GetServer(ctx context.Context, ip string) (*HRobotServer, error)`: Fetches one Robot server by server IP.

### `HRobotServer`
Hetzner Robot dedicated server model.
- `ServerIP string`: Public server IP address.
- `ServerName string`: Server hostname.
- `Product string`: Product or hardware plan name.
- `Datacenter string`: Datacenter code returned by the API field `dc`.
- `Status string`: Robot status string.
- `Cancelled bool`: Whether the server is cancelled.
- `PaidUntil string`: Billing paid-through date string.

## Functions

### `func DefaultRetryConfig() RetryConfig`
Returns the library defaults used by `NewAPIClient`: three retries, `100ms` initial backoff, and `5s` maximum backoff.

### `func WithHTTPClient(c *http.Client) APIClientOption`
Injects a custom `http.Client` into an `APIClient`.

### `func WithRetry(cfg RetryConfig) APIClientOption`
Injects a retry policy into an `APIClient`.

### `func WithAuth(fn func(req *http.Request)) APIClientOption`
Registers a callback that mutates each outgoing request before it is sent.

### `func WithPrefix(p string) APIClientOption`
Sets the error prefix used when wrapping client errors.

### `func NewAPIClient(opts ...APIClientOption) *APIClient`
Builds an `APIClient` with a `30s` HTTP timeout, `DefaultRetryConfig`, default prefix `api`, and any supplied options.

### `func NewCloudNSClient(authID, password string) *CloudNSClient`
Builds a CloudNS client configured for `auth-id` and `auth-password` query-parameter authentication.

### `func Load(path string) (*Config, error)`
Reads and unmarshals `infra.yaml`, expands host SSH key paths, and defaults missing SSH ports to `22`.

### `func Discover(startDir string) (*Config, string, error)`
Searches `startDir` and its parents for `infra.yaml`, returning the parsed config together with the discovered file path.

### `func NewHCloudClient(token string) *HCloudClient`
Builds a Hetzner Cloud client that authenticates requests with a bearer token.

### `func NewHRobotClient(user, password string) *HRobotClient`
Builds a Hetzner Robot client that authenticates requests with HTTP basic auth.
