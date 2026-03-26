package infra

import (
	"testing"

	core "dappco.re/go/core"
)

func TestConfig_Load_Good(t *testing.T) {
	// Find infra.yaml relative to test
	// Walk up from test dir to find it
	dir := core.Env("DIR_CWD")
	if dir == "" {
		t.Fatal(core.E("TestLoad_Good", "DIR_CWD unavailable", nil))
	}

	cfg, path, err := Discover(dir)
	if err != nil {
		t.Skipf("infra.yaml not found from %s: %v", dir, err)
	}

	t.Logf("Loaded %s", path)

	if len(cfg.Hosts) == 0 {
		t.Error("expected at least one host")
	}

	// Check required hosts exist
	for _, name := range []string{"noc", "de", "de2", "build"} {
		if _, ok := cfg.Hosts[name]; !ok {
			t.Errorf("expected host %q in config", name)
		}
	}

	// Check de host details
	de := cfg.Hosts["de"]
	if de.IP != "116.202.82.115" {
		t.Errorf("de IP = %q, want 116.202.82.115", de.IP)
	}
	if de.Role != "app" {
		t.Errorf("de role = %q, want app", de.Role)
	}

	// Check LB config
	if cfg.LoadBalancer.Name != "hermes" {
		t.Errorf("LB name = %q, want hermes", cfg.LoadBalancer.Name)
	}
	if cfg.LoadBalancer.Type != "lb11" {
		t.Errorf("LB type = %q, want lb11", cfg.LoadBalancer.Type)
	}
	if len(cfg.LoadBalancer.Backends) != 2 {
		t.Errorf("LB backends = %d, want 2", len(cfg.LoadBalancer.Backends))
	}

	// Check app servers helper
	apps := cfg.AppServers()
	if len(apps) != 2 {
		t.Errorf("AppServers() = %d, want 2", len(apps))
	}
}

func TestConfig_Load_Bad(t *testing.T) {
	_, err := Load("/nonexistent/infra.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestConfig_Load_Ugly(t *testing.T) {
	// Invalid YAML
	tmp := core.JoinPath(t.TempDir(), "infra.yaml")
	if r := localFS.WriteMode(tmp, "{{invalid yaml", 0644); !r.OK {
		t.Fatal(coreResultErr(r, "TestConfig_Load_Ugly"))
	}

	_, err := Load(tmp)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestConfig_HostsByRole_Good(t *testing.T) {
	cfg := &Config{
		Hosts: map[string]*Host{
			"de":    {FQDN: "de.example.com", Role: "app"},
			"de2":   {FQDN: "de2.example.com", Role: "app"},
			"noc":   {FQDN: "noc.example.com", Role: "bastion"},
			"build": {FQDN: "build.example.com", Role: "builder"},
		},
	}

	apps := cfg.HostsByRole("app")
	if len(apps) != 2 {
		t.Errorf("HostsByRole(app) = %d, want 2", len(apps))
	}
	if _, ok := apps["de"]; !ok {
		t.Error("expected de in app hosts")
	}
	if _, ok := apps["de2"]; !ok {
		t.Error("expected de2 in app hosts")
	}

	bastions := cfg.HostsByRole("bastion")
	if len(bastions) != 1 {
		t.Errorf("HostsByRole(bastion) = %d, want 1", len(bastions))
	}

	empty := cfg.HostsByRole("nonexistent")
	if len(empty) != 0 {
		t.Errorf("HostsByRole(nonexistent) = %d, want 0", len(empty))
	}
}

func TestConfig_AppServers_Good(t *testing.T) {
	cfg := &Config{
		Hosts: map[string]*Host{
			"de":  {FQDN: "de.example.com", Role: "app"},
			"noc": {FQDN: "noc.example.com", Role: "bastion"},
		},
	}

	apps := cfg.AppServers()
	if len(apps) != 1 {
		t.Errorf("AppServers() = %d, want 1", len(apps))
	}
	if _, ok := apps["de"]; !ok {
		t.Error("expected de in AppServers()")
	}
}

func TestConfig_ExpandPath_Good(t *testing.T) {
	home := core.Env("DIR_HOME")

	tests := []struct {
		input string
		want  string
	}{
		{"~/.ssh/id_rsa", core.JoinPath(home, ".ssh", "id_rsa")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
	}

	for _, tt := range tests {
		got := expandPath(tt.input)
		if got != tt.want {
			t.Errorf("expandPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
