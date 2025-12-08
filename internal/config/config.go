package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

// Config holds top-level settings loaded from file/env/flags.
type Config struct {
	Listen   string       `yaml:"listen" toml:"listen"`
	HostName string       `yaml:"host_name" toml:"host_name"`
	Target   string       `yaml:"target" toml:"target"`
	Proxy    ProxyConfig  `yaml:"proxy" toml:"proxy"`
	Access   AccessConfig `yaml:"access" toml:"access"`
	CORS     CORSConfig   `yaml:"cors" toml:"cors"`
	Headers  HeaderConfig `yaml:"headers" toml:"headers"`
	Logging  Logging      `yaml:"logging" toml:"logging"`
	TLS      TLSConfig    `yaml:"tls" toml:"tls"`
}

type ProxyConfig struct {
	Type     string        `yaml:"type" toml:"type"`
	Address  string        `yaml:"address" toml:"address"`
	Auth     ProxyAuth     `yaml:"auth" toml:"auth"`
	Timeouts TimeoutConfig `yaml:"timeouts" toml:"timeouts"`
	// URLs is a list of proxy URLs in format: socks5://user:pass@host:port or http://user:pass@host:port
	URLs     []string      `yaml:"urls" toml:"urls"`
	// Rotation strategy: "round-robin" (default), "random"
	Rotation string        `yaml:"rotation" toml:"rotation"`
}

type ProxyAuth struct {
	Username string `yaml:"username" toml:"username"`
	Password string `yaml:"password" toml:"password"`
}

// ParsedProxy represents a parsed proxy URL
type ParsedProxy struct {
	Type     string
	Address  string
	Username string
	Password string
}

// ParseProxyURL parses a proxy URL like socks5://user:pass@host:port
func ParseProxyURL(rawURL string) (ParsedProxy, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ParsedProxy{}, fmt.Errorf("invalid proxy URL: %w", err)
	}

	proxyType := strings.ToLower(u.Scheme)
	switch proxyType {
	case "socks5", "http", "https":
	default:
		return ParsedProxy{}, fmt.Errorf("unsupported proxy scheme: %s", proxyType)
	}

	p := ParsedProxy{
		Type:    proxyType,
		Address: u.Host,
	}

	if u.User != nil {
		p.Username = u.User.Username()
		p.Password, _ = u.User.Password()
	}

	return p, nil
}

// GetProxies returns list of parsed proxies from config
func (c ProxyConfig) GetProxies() ([]ParsedProxy, error) {
	var proxies []ParsedProxy

	// First, check URL list
	for _, rawURL := range c.URLs {
		p, err := ParseProxyURL(rawURL)
		if err != nil {
			return nil, err
		}
		p.Type = strings.ToLower(p.Type)
		proxies = append(proxies, p)
	}

	// If no URLs, use legacy config
	if len(proxies) == 0 && c.Type != "" && c.Type != "direct" {
		proxies = append(proxies, ParsedProxy{
			Type:     strings.ToLower(c.Type),
			Address:  c.Address,
			Username: c.Auth.Username,
			Password: c.Auth.Password,
		})
	}

	return proxies, nil
}

type TimeoutConfig struct {
	ConnectSeconds int `yaml:"connect_seconds" toml:"connect_seconds"`
	IdleSeconds    int `yaml:"idle_seconds" toml:"idle_seconds"`
}

type AccessConfig struct {
	AllowCIDRs []string `yaml:"allow" toml:"allow"`
	BlockCIDRs []string `yaml:"block" toml:"block"`
}

type CORSConfig struct {
	AllowedOrigins   []string `yaml:"allowed_origins" toml:"allowed_origins"`
	AllowedHeaders   []string `yaml:"allowed_headers" toml:"allowed_headers"`
	AllowCredentials bool     `yaml:"allow_credentials" toml:"allow_credentials"`
	ExposeHeaders    []string `yaml:"expose_headers" toml:"expose_headers"`
	AllowMethods     []string `yaml:"allow_methods" toml:"allow_methods"`
	MaxAgeSeconds    int      `yaml:"max_age_seconds" toml:"max_age_seconds"`
}

type HeaderConfig struct {
	RewriteHost    bool              `yaml:"rewrite_host" toml:"rewrite_host"`
	RewriteOrigin  bool              `yaml:"rewrite_origin" toml:"rewrite_origin"`
	RewriteReferer bool              `yaml:"rewrite_referer" toml:"rewrite_referer"`
	Add            map[string]string `yaml:"add" toml:"add"`
}

type Logging struct {
	Level string `yaml:"level" toml:"level"`
}

type TLSConfig struct {
	CertFile string     `yaml:"cert_file" toml:"cert_file"`
	KeyFile  string     `yaml:"key_file" toml:"key_file"`
	ACME     ACMEConfig `yaml:"acme" toml:"acme"`
}

type ACMEConfig struct {
	Enabled    bool   `yaml:"enabled" toml:"enabled"`
	Domain     string `yaml:"domain" toml:"domain"`
	Email      string `yaml:"email" toml:"email"`
	CacheDir   string `yaml:"cache_dir" toml:"cache_dir"`
	HTTP01Port string `yaml:"http01_port" toml:"http01_port"`
}

func (t TLSConfig) HasCertificates() bool {
	return t.CertFile != "" && t.KeyFile != ""
}

type Overrides struct {
	Listen             string
	HostName           string
	Target             string
	Proxy              ProxyOverride
	AllowCIDRs         []string
	CORSOrigins        []string
	AddHeaders         map[string]string
	TLSCertFile        string
	TLSKeyFile         string
	ACMEDomain         string
	ACMEEmail          string
	ACMECacheDir       string
	DisableRewriteHost bool
}

type ProxyOverride struct {
	Type     string
	Address  string
	Username string
	Password string
}

// DefaultConfig returns sane defaults for the application.
func DefaultConfig() Config {
	return Config{
		Listen:   "0.0.0.0:8080",
		HostName: "",
		Target:   "",
		Proxy: ProxyConfig{
			Type:    "",
			Address: "",
			Timeouts: TimeoutConfig{
				ConnectSeconds: 10,
				IdleSeconds:    30,
			},
		},
		CORS: CORSConfig{
			AllowedOrigins:   []string{"*"},
			AllowedHeaders:   []string{"*"},
			AllowCredentials: false,
			AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			MaxAgeSeconds:    600,
		},
		Headers: HeaderConfig{
			RewriteHost:    true,
			RewriteOrigin:  true,
			RewriteReferer: true,
			Add:            map[string]string{},
		},
		Logging: Logging{Level: "info"},
		TLS: TLSConfig{
			ACME: ACMEConfig{
				CacheDir:   "acme-cache",
				HTTP01Port: "80",
			},
		},
	}
}

// Load merges defaults with file contents, env overrides, and flag overrides.
func Load(path string, envPrefix string, overrides Overrides) (Config, error) {
	cfg := DefaultConfig()

	if path != "" {
		if err := parseFile(path, &cfg); err != nil {
			return cfg, err
		}
	}

	applyEnv(&cfg, envPrefix)
	applyOverrides(&cfg, overrides)

	if err := cfg.Validate(); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func (c Config) Validate() error {
	if c.Target == "" {
		return errors.New("target is required")
	}
	if c.Listen == "" {
		return errors.New("listen is required")
	}
	switch strings.ToLower(c.Proxy.Type) {
	case "", "direct", "socks5", "http", "https":
	default:
		return fmt.Errorf("unsupported proxy type: %s", c.Proxy.Type)
	}
	// Validate proxy URLs
	for _, rawURL := range c.Proxy.URLs {
		if _, err := ParseProxyURL(rawURL); err != nil {
			return fmt.Errorf("invalid proxy URL %q: %w", rawURL, err)
		}
	}
	switch strings.ToLower(c.Proxy.Rotation) {
	case "", "round-robin", "random":
	default:
		return fmt.Errorf("unsupported proxy rotation: %s", c.Proxy.Rotation)
	}
	if c.TLS.ACME.Enabled && c.TLS.ACME.Domain == "" {
		return errors.New("acme enabled but domain is empty")
	}
	return nil
}

func parseFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return yaml.Unmarshal(data, cfg)
	case ".toml":
		return toml.Unmarshal(data, cfg)
	default:
		if err := yaml.Unmarshal(data, cfg); err == nil {
			return nil
		}
		if err := toml.Unmarshal(data, cfg); err == nil {
			return nil
		}
		return fmt.Errorf("failed to parse config: unknown format")
	}
}

func applyOverrides(cfg *Config, overrides Overrides) {
	if overrides.Listen != "" {
		cfg.Listen = overrides.Listen
	}
	if overrides.HostName != "" {
		cfg.HostName = overrides.HostName
	}
	if overrides.Target != "" {
		cfg.Target = overrides.Target
	}
	if overrides.Proxy.Type != "" {
		cfg.Proxy.Type = overrides.Proxy.Type
	}
	if overrides.Proxy.Address != "" {
		cfg.Proxy.Address = overrides.Proxy.Address
	}
	if overrides.Proxy.Username != "" {
		cfg.Proxy.Auth.Username = overrides.Proxy.Username
	}
	if overrides.Proxy.Password != "" {
		cfg.Proxy.Auth.Password = overrides.Proxy.Password
	}
	if len(overrides.AllowCIDRs) > 0 {
		cfg.Access.AllowCIDRs = overrides.AllowCIDRs
	}
	if len(overrides.CORSOrigins) > 0 {
		cfg.CORS.AllowedOrigins = overrides.CORSOrigins
	}
	if len(overrides.AddHeaders) > 0 {
		if cfg.Headers.Add == nil {
			cfg.Headers.Add = map[string]string{}
		}
		for k, v := range overrides.AddHeaders {
			cfg.Headers.Add[k] = v
		}
	}
	if overrides.DisableRewriteHost {
		cfg.Headers.RewriteHost = false
	}
	if overrides.TLSCertFile != "" {
		cfg.TLS.CertFile = overrides.TLSCertFile
	}
	if overrides.TLSKeyFile != "" {
		cfg.TLS.KeyFile = overrides.TLSKeyFile
	}
	if overrides.ACMEDomain != "" {
		cfg.TLS.ACME.Domain = overrides.ACMEDomain
		cfg.TLS.ACME.Enabled = true
	}
	if overrides.ACMEEmail != "" {
		cfg.TLS.ACME.Email = overrides.ACMEEmail
	}
	if overrides.ACMECacheDir != "" {
		cfg.TLS.ACME.CacheDir = overrides.ACMECacheDir
	}
}

func applyEnv(cfg *Config, prefix string) {
	p := strings.ToUpper(prefix)
	if p == "" {
		p = "SOCKSTREAM"
	}

	get := func(key string) (string, bool) {
		return os.LookupEnv(fmt.Sprintf("%s_%s", p, key))
	}

	if v, ok := get("LISTEN"); ok {
		cfg.Listen = v
	}
	if v, ok := get("HOST_NAME"); ok {
		cfg.HostName = v
	}
	if v, ok := get("TARGET"); ok {
		cfg.Target = v
	}
	if v, ok := get("PROXY_TYPE"); ok {
		cfg.Proxy.Type = v
	}
	if v, ok := get("PROXY_ADDRESS"); ok {
		cfg.Proxy.Address = v
	}
	if v, ok := get("PROXY_USERNAME"); ok {
		cfg.Proxy.Auth.Username = v
	}
	if v, ok := get("PROXY_PASSWORD"); ok {
		cfg.Proxy.Auth.Password = v
	}
	if v, ok := get("PROXY_URLS"); ok {
		cfg.Proxy.URLs = splitAndClean(v)
	}
	if v, ok := get("PROXY_ROTATION"); ok {
		cfg.Proxy.Rotation = v
	}
	if v, ok := get("ALLOW_IPS"); ok {
		cfg.Access.AllowCIDRs = splitAndClean(v)
	}
	if v, ok := get("BLOCK_IPS"); ok {
		cfg.Access.BlockCIDRs = splitAndClean(v)
	}
	if v, ok := get("CORS_ORIGINS"); ok {
		cfg.CORS.AllowedOrigins = splitAndClean(v)
	}
	if v, ok := get("ADD_HEADERS"); ok {
		if cfg.Headers.Add == nil {
			cfg.Headers.Add = map[string]string{}
		}
		for _, kv := range splitAndClean(v) {
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) == 2 {
				cfg.Headers.Add[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
	}
	if v, ok := get("TLS_CERT_FILE"); ok {
		cfg.TLS.CertFile = v
	}
	if v, ok := get("TLS_KEY_FILE"); ok {
		cfg.TLS.KeyFile = v
	}
	if v, ok := get("ACME_DOMAIN"); ok {
		cfg.TLS.ACME.Enabled = true
		cfg.TLS.ACME.Domain = v
	}
	if v, ok := get("ACME_EMAIL"); ok {
		cfg.TLS.ACME.Email = v
	}
	if v, ok := get("ACME_CACHE_DIR"); ok {
		cfg.TLS.ACME.CacheDir = v
	}
}

func splitAndClean(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}
