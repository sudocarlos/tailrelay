package caddy

// CaddyConfig represents the root Caddy configuration
type CaddyConfig struct {
	Apps CaddyApps `json:"apps"`
}

// CaddyApps contains all Caddy app configurations
type CaddyApps struct {
	HTTP *HTTPApp `json:"http,omitempty"`
	TLS  *TLSApp  `json:"tls,omitempty"`
}

// HTTPApp represents Caddy's HTTP app configuration
type HTTPApp struct {
	Servers map[string]*HTTPServer `json:"servers,omitempty"`
}

// HTTPServer represents a Caddy HTTP server
type HTTPServer struct {
	Listen []string    `json:"listen,omitempty"`
	Routes []Route     `json:"routes,omitempty"`
	Logs   *ServerLogs `json:"logs,omitempty"`
}

// Route represents a Caddy route with matchers and handlers
type Route struct {
	ID       string       `json:"@id,omitempty"`
	Match    []MatcherSet `json:"match,omitempty"`
	Handle   []Handler    `json:"handle"`
	Group    string       `json:"group,omitempty"`
	Terminal bool         `json:"terminal,omitempty"`
}

// MatcherSet represents a set of matchers (all must match)
type MatcherSet struct {
	Host []string `json:"host,omitempty"`
	Path []string `json:"path,omitempty"`
}

// Handler represents a Caddy handler (interface type)
// Concrete types include ReverseProxyHandler, StaticResponseHandler, etc.
type Handler map[string]interface{}

// ReverseProxyHandler represents a reverse_proxy handler configuration
type ReverseProxyHandler struct {
	ID            string         `json:"@id,omitempty"`
	Handler       string         `json:"handler"`
	Upstreams     []Upstream     `json:"upstreams"`
	LoadBalancing *LoadBalancing `json:"load_balancing,omitempty"`
	HealthChecks  *HealthChecks  `json:"health_checks,omitempty"`
	Headers       *HeaderConfig  `json:"headers,omitempty"`
	Transport     interface{}    `json:"transport,omitempty"`
	FlushInterval string         `json:"flush_interval,omitempty"`
}

// Upstream represents an upstream server
type Upstream struct {
	Dial        string `json:"dial"`
	MaxRequests int    `json:"max_requests,omitempty"`
}

// LoadBalancing represents load balancing configuration
type LoadBalancing struct {
	SelectionPolicy interface{} `json:"selection_policy,omitempty"`
	TryDuration     string      `json:"try_duration,omitempty"`
	TryInterval     string      `json:"try_interval,omitempty"`
	Retries         int         `json:"retries,omitempty"`
}

// HealthChecks represents active and passive health check configuration
type HealthChecks struct {
	Active  *ActiveHealthChecks  `json:"active,omitempty"`
	Passive *PassiveHealthChecks `json:"passive,omitempty"`
}

// ActiveHealthChecks represents active health check configuration
type ActiveHealthChecks struct {
	URI          string            `json:"uri,omitempty"`
	Port         int               `json:"port,omitempty"`
	Interval     string            `json:"interval,omitempty"`
	Timeout      string            `json:"timeout,omitempty"`
	ExpectStatus int               `json:"expect_status,omitempty"`
	ExpectBody   string            `json:"expect_body,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
}

// PassiveHealthChecks represents passive health check configuration
type PassiveHealthChecks struct {
	FailDuration          string `json:"fail_duration,omitempty"`
	MaxFails              int    `json:"max_fails,omitempty"`
	UnhealthyStatus       []int  `json:"unhealthy_status,omitempty"`
	UnhealthyLatency      string `json:"unhealthy_latency,omitempty"`
	UnhealthyRequestCount int    `json:"unhealthy_request_count,omitempty"`
}

// HeaderConfig represents header manipulation configuration
type HeaderConfig struct {
	Request  *HeaderOps `json:"request,omitempty"`
	Response *HeaderOps `json:"response,omitempty"`
}

// HeaderOps represents header operations (add, set, delete)
type HeaderOps struct {
	Add    map[string][]string `json:"add,omitempty"`
	Set    map[string][]string `json:"set,omitempty"`
	Delete []string            `json:"delete,omitempty"`
}

// HeaderOperation represents a single header operation
type HeaderOperation struct {
	Field  string   `json:"field"`
	Values []string `json:"value"`
}

// HTTPTransport represents HTTP transport configuration for reverse proxy
type HTTPTransport struct {
	Protocol        string     `json:"protocol"`
	TLS             *TLSConfig `json:"tls,omitempty"`
	KeepAlive       *KeepAlive `json:"keep_alive,omitempty"`
	Compression     *bool      `json:"compression,omitempty"`
	MaxConnsPerHost int        `json:"max_conns_per_host,omitempty"`
	Versions        []string   `json:"versions,omitempty"`
}

// TLSConfig represents TLS configuration for transport
type TLSConfig struct {
	RootCAPool            []string `json:"root_ca_pool,omitempty"`
	RootCAPEMFiles        []string `json:"root_ca_pem_files,omitempty"`
	ClientCertificateFile string   `json:"client_certificate_file,omitempty"`
	ClientCertificateKey  string   `json:"client_certificate_key_file,omitempty"`
	InsecureSkipVerify    bool     `json:"insecure_skip_verify,omitempty"`
	ServerName            string   `json:"server_name,omitempty"`
	HandshakeTimeout      string   `json:"handshake_timeout,omitempty"`
}

// KeepAlive represents keep-alive configuration
type KeepAlive struct {
	Enabled             *bool  `json:"enabled,omitempty"`
	ProbeInterval       string `json:"probe_interval,omitempty"`
	MaxIdleConns        int    `json:"max_idle_conns,omitempty"`
	MaxIdleConnsPerHost int    `json:"max_idle_conns_per_host,omitempty"`
	IdleConnTimeout     string `json:"idle_conn_timeout,omitempty"`
}

// ServerLogs represents server-level logging configuration
type ServerLogs struct {
	DefaultLoggerName string            `json:"default_logger_name,omitempty"`
	LoggerNames       map[string]string `json:"logger_names,omitempty"`
}

// TLSApp represents Caddy's TLS app configuration
type TLSApp struct {
	Automation   *TLSAutomation   `json:"automation,omitempty"`
	Certificates *TLSCertificates `json:"certificates,omitempty"`
}

// TLSAutomation represents TLS automation configuration
type TLSAutomation struct {
	Policies []TLSPolicy `json:"policies,omitempty"`
}

// TLSPolicy represents a TLS automation policy
type TLSPolicy struct {
	Subjects []string    `json:"subjects,omitempty"`
	Issuers  []TLSIssuer `json:"issuers,omitempty"`
}

// TLSIssuer represents a certificate issuer configuration
type TLSIssuer struct {
	Module string                 `json:"module"`
	Config map[string]interface{} `json:"config,omitempty"`
}

// TLSCertificates represents TLS certificate configuration
type TLSCertificates struct {
	LoadFiles []TLSCertificateFile `json:"load_files,omitempty"`
}

// TLSCertificateFile represents a TLS certificate file pair
type TLSCertificateFile struct {
	Certificate string   `json:"certificate"`
	Key         string   `json:"key"`
	Tags        []string `json:"tags,omitempty"`
}
