package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Listen != "0.0.0.0:8080" {
		t.Errorf("Listen = %q, want %q", cfg.Listen, "0.0.0.0:8080")
	}
	if cfg.Target != "" {
		t.Errorf("Target = %q, want empty", cfg.Target)
	}
	if !cfg.Headers.RewriteHost {
		t.Error("RewriteHost should be true by default")
	}
	if !cfg.Headers.RewriteOrigin {
		t.Error("RewriteOrigin should be true by default")
	}
	if !cfg.Headers.RewriteReferer {
		t.Error("RewriteReferer should be true by default")
	}
	if len(cfg.CORS.AllowedOrigins) != 1 || cfg.CORS.AllowedOrigins[0] != "*" {
		t.Errorf("CORS.AllowedOrigins = %v, want [*]", cfg.CORS.AllowedOrigins)
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				Listen: "0.0.0.0:8080",
				Target: "https://example.com",
			},
			wantErr: false,
		},
		{
			name: "missing target",
			cfg: Config{
				Listen: "0.0.0.0:8080",
				Target: "",
			},
			wantErr: true,
		},
		{
			name: "missing listen",
			cfg: Config{
				Listen: "",
				Target: "https://example.com",
			},
			wantErr: true,
		},
		{
			name: "valid proxy type - socks5",
			cfg: Config{
				Listen: "0.0.0.0:8080",
				Target: "https://example.com",
				Proxy:  ProxyConfig{Type: "socks5"},
			},
			wantErr: false,
		},
		{
			name: "valid proxy type - http",
			cfg: Config{
				Listen: "0.0.0.0:8080",
				Target: "https://example.com",
				Proxy:  ProxyConfig{Type: "http"},
			},
			wantErr: false,
		},
		{
			name: "invalid proxy type",
			cfg: Config{
				Listen: "0.0.0.0:8080",
				Target: "https://example.com",
				Proxy:  ProxyConfig{Type: "ftp"},
			},
			wantErr: true,
		},
		{
			name: "ACME enabled without domain",
			cfg: Config{
				Listen: "0.0.0.0:8080",
				Target: "https://example.com",
				TLS:    TLSConfig{ACME: ACMEConfig{Enabled: true, Domain: ""}},
			},
			wantErr: true,
		},
		{
			name: "ACME enabled with domain",
			cfg: Config{
				Listen: "0.0.0.0:8080",
				Target: "https://example.com",
				TLS:    TLSConfig{ACME: ACMEConfig{Enabled: true, Domain: "example.com"}},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTLSConfig_HasCertificates(t *testing.T) {
	tests := []struct {
		name string
		cfg  TLSConfig
		want bool
	}{
		{
			name: "both present",
			cfg:  TLSConfig{CertFile: "/path/to/cert", KeyFile: "/path/to/key"},
			want: true,
		},
		{
			name: "cert only",
			cfg:  TLSConfig{CertFile: "/path/to/cert", KeyFile: ""},
			want: false,
		},
		{
			name: "key only",
			cfg:  TLSConfig{CertFile: "", KeyFile: "/path/to/key"},
			want: false,
		},
		{
			name: "both empty",
			cfg:  TLSConfig{CertFile: "", KeyFile: ""},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.HasCertificates(); got != tt.want {
				t.Errorf("HasCertificates() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSplitAndClean(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "simple list",
			input: "a,b,c",
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "with spaces",
			input: " a , b , c ",
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "empty parts",
			input: "a,,b,  ,c",
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "single item",
			input: "a",
			want:  []string{"a"},
		},
		{
			name:  "empty string",
			input: "",
			want:  []string{},
		},
		{
			name:  "only commas",
			input: ",,,",
			want:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitAndClean(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("splitAndClean() len = %d, want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitAndClean()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestApplyOverrides(t *testing.T) {
	cfg := DefaultConfig()
	overrides := Overrides{
		Listen:   "127.0.0.1:9090",
		HostName: "custom.host",
		Target:   "https://target.com",
		Proxy: ProxyOverride{
			Type:     "socks5",
			Address:  "127.0.0.1:1080",
			Username: "user",
			Password: "pass",
		},
		AllowCIDRs:  []string{"10.0.0.0/8"},
		CORSOrigins: []string{"https://example.com"},
		AddHeaders: map[string]string{
			"X-Custom": "value",
		},
		DisableRewriteHost: true,
		TLSCertFile:        "/path/cert",
		TLSKeyFile:         "/path/key",
		ACMEDomain:         "acme.example.com",
		ACMEEmail:          "admin@example.com",
		ACMECacheDir:       "/cache",
	}

	applyOverrides(&cfg, overrides)

	if cfg.Listen != "127.0.0.1:9090" {
		t.Errorf("Listen = %q, want %q", cfg.Listen, "127.0.0.1:9090")
	}
	if cfg.HostName != "custom.host" {
		t.Errorf("HostName = %q, want %q", cfg.HostName, "custom.host")
	}
	if cfg.Target != "https://target.com" {
		t.Errorf("Target = %q, want %q", cfg.Target, "https://target.com")
	}
	if cfg.Proxy.Type != "socks5" {
		t.Errorf("Proxy.Type = %q, want %q", cfg.Proxy.Type, "socks5")
	}
	if cfg.Proxy.Auth.Username != "user" {
		t.Errorf("Proxy.Auth.Username = %q, want %q", cfg.Proxy.Auth.Username, "user")
	}
	if cfg.Headers.RewriteHost != false {
		t.Error("RewriteHost should be false after DisableRewriteHost")
	}
	if cfg.TLS.ACME.Enabled != true {
		t.Error("ACME should be enabled when ACMEDomain is set")
	}
	if cfg.Headers.Add["X-Custom"] != "value" {
		t.Errorf("Headers.Add[X-Custom] = %q, want %q", cfg.Headers.Add["X-Custom"], "value")
	}
}

func TestParseFile_YAML(t *testing.T) {
	content := `
listen: 127.0.0.1:9000
target: https://yaml-target.com
proxy:
  type: http
  address: proxy:8080
`
	tmpFile := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig()
	if err := parseFile(tmpFile, &cfg); err != nil {
		t.Fatalf("parseFile() error = %v", err)
	}

	if cfg.Listen != "127.0.0.1:9000" {
		t.Errorf("Listen = %q, want %q", cfg.Listen, "127.0.0.1:9000")
	}
	if cfg.Target != "https://yaml-target.com" {
		t.Errorf("Target = %q, want %q", cfg.Target, "https://yaml-target.com")
	}
	if cfg.Proxy.Type != "http" {
		t.Errorf("Proxy.Type = %q, want %q", cfg.Proxy.Type, "http")
	}
}

func TestParseFile_TOML(t *testing.T) {
	content := `
listen = "127.0.0.1:9001"
target = "https://toml-target.com"

[proxy]
type = "socks5"
address = "proxy:1080"
`
	tmpFile := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig()
	if err := parseFile(tmpFile, &cfg); err != nil {
		t.Fatalf("parseFile() error = %v", err)
	}

	if cfg.Listen != "127.0.0.1:9001" {
		t.Errorf("Listen = %q, want %q", cfg.Listen, "127.0.0.1:9001")
	}
	if cfg.Target != "https://toml-target.com" {
		t.Errorf("Target = %q, want %q", cfg.Target, "https://toml-target.com")
	}
	if cfg.Proxy.Type != "socks5" {
		t.Errorf("Proxy.Type = %q, want %q", cfg.Proxy.Type, "socks5")
	}
}

func TestApplyEnv(t *testing.T) {
	os.Setenv("SOCKSTREAM_LISTEN", "env:8080")
	os.Setenv("SOCKSTREAM_TARGET", "https://env-target.com")
	os.Setenv("SOCKSTREAM_PROXY_TYPE", "socks5")
	os.Setenv("SOCKSTREAM_ALLOW_IPS", "10.0.0.0/8, 192.168.0.0/16")
	os.Setenv("SOCKSTREAM_ADD_HEADERS", "X-Env=value1, X-Another=value2")
	defer func() {
		os.Unsetenv("SOCKSTREAM_LISTEN")
		os.Unsetenv("SOCKSTREAM_TARGET")
		os.Unsetenv("SOCKSTREAM_PROXY_TYPE")
		os.Unsetenv("SOCKSTREAM_ALLOW_IPS")
		os.Unsetenv("SOCKSTREAM_ADD_HEADERS")
	}()

	cfg := DefaultConfig()
	applyEnv(&cfg, "")

	if cfg.Listen != "env:8080" {
		t.Errorf("Listen = %q, want %q", cfg.Listen, "env:8080")
	}
	if cfg.Target != "https://env-target.com" {
		t.Errorf("Target = %q, want %q", cfg.Target, "https://env-target.com")
	}
	if cfg.Proxy.Type != "socks5" {
		t.Errorf("Proxy.Type = %q, want %q", cfg.Proxy.Type, "socks5")
	}
	if len(cfg.Access.AllowCIDRs) != 2 {
		t.Errorf("AllowCIDRs len = %d, want 2", len(cfg.Access.AllowCIDRs))
	}
	if cfg.Headers.Add["X-Env"] != "value1" {
		t.Errorf("Headers.Add[X-Env] = %q, want %q", cfg.Headers.Add["X-Env"], "value1")
	}
}

func TestLoad(t *testing.T) {
	content := `
listen: 127.0.0.1:7000
target: https://load-target.com
`
	tmpFile := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(tmpFile, "", Overrides{})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Listen != "127.0.0.1:7000" {
		t.Errorf("Listen = %q, want %q", cfg.Listen, "127.0.0.1:7000")
	}
	if cfg.Target != "https://load-target.com" {
		t.Errorf("Target = %q, want %q", cfg.Target, "https://load-target.com")
	}
}

func TestLoad_ValidationError(t *testing.T) {
	content := `
listen: 127.0.0.1:7000
target: ""
`
	tmpFile := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(tmpFile, "", Overrides{})
	if err == nil {
		t.Error("Load() expected validation error for missing target")
	}
}

func TestParseProxyURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		wantType string
		wantAddr string
		wantUser string
		wantPass string
		wantErr  bool
	}{
		{
			name:     "socks5 with auth",
			url:      "socks5://user:pass@127.0.0.1:1080",
			wantType: "socks5",
			wantAddr: "127.0.0.1:1080",
			wantUser: "user",
			wantPass: "pass",
			wantErr:  false,
		},
		{
			name:     "socks5 without auth",
			url:      "socks5://proxy.example.com:1080",
			wantType: "socks5",
			wantAddr: "proxy.example.com:1080",
			wantUser: "",
			wantPass: "",
			wantErr:  false,
		},
		{
			name:     "http proxy with auth",
			url:      "http://admin:secret@proxy.local:8080",
			wantType: "http",
			wantAddr: "proxy.local:8080",
			wantUser: "admin",
			wantPass: "secret",
			wantErr:  false,
		},
		{
			name:     "https proxy",
			url:      "https://secure-proxy.com:443",
			wantType: "https",
			wantAddr: "secure-proxy.com:443",
			wantUser: "",
			wantPass: "",
			wantErr:  false,
		},
		{
			name:     "password with special chars",
			url:      "socks5://user:p%40ss%3Aword@host:1080",
			wantType: "socks5",
			wantAddr: "host:1080",
			wantUser: "user",
			wantPass: "p@ss:word",
			wantErr:  false,
		},
		{
			name:    "unsupported scheme",
			url:     "ftp://proxy:21",
			wantErr: true,
		},
		{
			name:    "invalid URL",
			url:     "://invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := ParseProxyURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseProxyURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if p.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", p.Type, tt.wantType)
			}
			if p.Address != tt.wantAddr {
				t.Errorf("Address = %q, want %q", p.Address, tt.wantAddr)
			}
			if p.Username != tt.wantUser {
				t.Errorf("Username = %q, want %q", p.Username, tt.wantUser)
			}
			if p.Password != tt.wantPass {
				t.Errorf("Password = %q, want %q", p.Password, tt.wantPass)
			}
		})
	}
}

func TestProxyConfig_GetProxies(t *testing.T) {
	tests := []struct {
		name      string
		cfg       ProxyConfig
		wantCount int
		wantErr   bool
	}{
		{
			name:      "empty config",
			cfg:       ProxyConfig{},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "legacy config",
			cfg: ProxyConfig{
				Type:    "socks5",
				Address: "127.0.0.1:1080",
				Auth:    ProxyAuth{Username: "user", Password: "pass"},
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "URL list",
			cfg: ProxyConfig{
				URLs: []string{
					"socks5://proxy1:1080",
					"http://proxy2:8080",
					"https://proxy3:443",
				},
			},
			wantCount: 3,
			wantErr:   false,
		},
		{
			name: "URL list takes precedence over legacy",
			cfg: ProxyConfig{
				Type:    "socks5",
				Address: "legacy:1080",
				URLs: []string{
					"http://new1:8080",
					"http://new2:8080",
				},
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "invalid URL in list",
			cfg: ProxyConfig{
				URLs: []string{
					"socks5://valid:1080",
					"ftp://invalid:21",
				},
			},
			wantErr: true,
		},
		{
			name: "direct type returns empty",
			cfg: ProxyConfig{
				Type:    "direct",
				Address: "ignored",
			},
			wantCount: 0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxies, err := tt.cfg.GetProxies()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetProxies() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if len(proxies) != tt.wantCount {
				t.Errorf("GetProxies() count = %d, want %d", len(proxies), tt.wantCount)
			}
		})
	}
}

func TestConfig_Validate_ProxyURLs(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid proxy URLs",
			cfg: Config{
				Listen: "0.0.0.0:8080",
				Target: "https://example.com",
				Proxy: ProxyConfig{
					URLs: []string{
						"socks5://proxy1:1080",
						"http://proxy2:8080",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid proxy URL",
			cfg: Config{
				Listen: "0.0.0.0:8080",
				Target: "https://example.com",
				Proxy: ProxyConfig{
					URLs: []string{
						"ftp://invalid:21",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid rotation",
			cfg: Config{
				Listen: "0.0.0.0:8080",
				Target: "https://example.com",
				Proxy: ProxyConfig{
					Rotation: "random",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid rotation",
			cfg: Config{
				Listen: "0.0.0.0:8080",
				Target: "https://example.com",
				Proxy: ProxyConfig{
					Rotation: "invalid",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
