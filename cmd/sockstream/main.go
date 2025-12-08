package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"sockstream/internal/config"
	"sockstream/internal/proxy"
	"sockstream/internal/server"
)

func main() {
	flags := parseFlags()

	overrides := config.Overrides{
		Listen:   flags.listen,
		HostName: flags.hostName,
		Target:   flags.target,
		Proxy: config.ProxyOverride{
			Type:     flags.proxyType,
			Address:  flags.proxyAddress,
			Username: flags.proxyUser,
			Password: flags.proxyPass,
		},
		AllowCIDRs:         flags.allowCIDR,
		CORSOrigins:        flags.corsOrigins,
		AddHeaders:         flags.headers,
		TLSCertFile:        flags.tlsCert,
		TLSKeyFile:         flags.tlsKey,
		ACMEDomain:         flags.acmeDomain,
		ACMEEmail:          flags.acmeEmail,
		ACMECacheDir:       flags.acmeCache,
		DisableRewriteHost: flags.disableRewriteHost,
	}

	cfg, err := config.Load(flags.configPath, "SOCKSTREAM", overrides)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	targetURL, err := url.Parse(cfg.Target)
	if err != nil {
		slog.Error("invalid target url", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLogLevel(cfg.Logging.Level),
	}))

	transport, err := proxy.NewTransport(cfg.Proxy)
	if err != nil {
		logger.Error("failed to create transport", "error", err)
		os.Exit(1)
	}

	reverseProxy := proxy.NewReverseProxy(targetURL, cfg, transport, logger)
	srv, err := server.New(cfg, logger, reverseProxy)
	if err != nil {
		logger.Error("failed to init server", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("starting server", "listen", cfg.Listen, "target", cfg.Target)
	if cfg.Proxy.Type != "" {
		logger.Info("using upstream proxy", "type", cfg.Proxy.Type, "address", cfg.Proxy.Address)
	}
	if cfg.TLS.HasCertificates() {
		logger.Info("serving TLS with provided certificate")
	} else if cfg.TLS.ACME.Enabled {
		logger.Info("serving TLS via ACME", "domain", cfg.TLS.ACME.Domain)
	}

	if err := srv.Start(ctx); err != nil && err != http.ErrServerClosed {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}

func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

type cliFlags struct {
	configPath         string
	listen             string
	hostName           string
	target             string
	proxyType          string
	proxyAddress       string
	proxyUser          string
	proxyPass          string
	allowCIDR          []string
	corsOrigins        []string
	headers            map[string]string
	tlsCert            string
	tlsKey             string
	acmeDomain         string
	acmeEmail          string
	acmeCache          string
	disableRewriteHost bool
}

func parseFlags() cliFlags {
	var f cliFlags
	allowCIDR := multiFlag{}
	corsOrigins := multiFlag{}
	headerPairs := multiFlag{}

	flag.StringVar(&f.configPath, "config", "", "path to config file (yaml or toml)")
	flag.StringVar(&f.listen, "listen", "", "listen address override")
	flag.StringVar(&f.hostName, "host-name", "", "override Host header to this value")
	flag.StringVar(&f.target, "target", "", "target URL to proxy to")
	flag.StringVar(&f.proxyType, "proxy-type", "", "upstream proxy type (socks5/http/https)")
	flag.StringVar(&f.proxyAddress, "proxy-address", "", "upstream proxy address host:port")
	flag.StringVar(&f.proxyUser, "proxy-user", "", "upstream proxy username")
	flag.StringVar(&f.proxyPass, "proxy-pass", "", "upstream proxy password")
	flag.Var(&allowCIDR, "allow", "allow CIDR (can repeat)")
	flag.Var(&corsOrigins, "cors-origin", "allowed CORS origin (can repeat)")
	flag.Var(&headerPairs, "add-header", "header to add key=value (can repeat)")
	flag.StringVar(&f.tlsCert, "tls-cert", "", "path to TLS certificate")
	flag.StringVar(&f.tlsKey, "tls-key", "", "path to TLS private key")
	flag.StringVar(&f.acmeDomain, "acme-domain", "", "enable ACME and set domain")
	flag.StringVar(&f.acmeEmail, "acme-email", "", "ACME registration email")
	flag.StringVar(&f.acmeCache, "acme-cache", "", "ACME cache directory")
	flag.BoolVar(&f.disableRewriteHost, "no-rewrite-host", false, "disable rewriting Host header to target")
	flag.Parse()

	f.allowCIDR = allowCIDR.values
	f.corsOrigins = corsOrigins.values
	f.headers = parseHeaders(headerPairs.values)
	return f
}

type multiFlag struct {
	values []string
}

func (m *multiFlag) String() string {
	return strings.Join(m.values, ",")
}

func (m *multiFlag) Set(value string) error {
	m.values = append(m.values, value)
	return nil
}

func parseHeaders(values []string) map[string]string {
	h := make(map[string]string)
	for _, v := range values {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if key != "" {
			h[key] = val
		}
	}
	return h
}
