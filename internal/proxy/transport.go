package proxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/proxy"

	"sockstream/internal/config"
)

// NewTransport builds an HTTP transport configured with optional upstream proxy.
func NewTransport(cfg config.ProxyConfig) (http.RoundTripper, error) {
	dialer := &net.Dialer{
		Timeout:   durationFromSeconds(cfg.Timeouts.ConnectSeconds, 10*time.Second),
		KeepAlive: 30 * time.Second,
	}

	tr := &http.Transport{
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       durationFromSeconds(cfg.Timeouts.IdleSeconds, 30*time.Second),
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		Proxy:                 http.ProxyFromEnvironment,
	}

	switch strings.ToLower(cfg.Type) {
	case "", "direct":
		return tr, nil
	case "http", "https":
		if cfg.Address == "" {
			return nil, fmt.Errorf("proxy address required for http/https proxy")
		}
		u, err := url.Parse(fmt.Sprintf("%s://%s", cfg.Type, cfg.Address))
		if err != nil {
			return nil, fmt.Errorf("parse proxy url: %w", err)
		}
		if cfg.Auth.Username != "" {
			u.User = url.UserPassword(cfg.Auth.Username, cfg.Auth.Password)
		}
		tr.Proxy = http.ProxyURL(u)
		return tr, nil
	case "socks5":
		if cfg.Address == "" {
			return nil, fmt.Errorf("proxy address required for socks5")
		}
		var auth *proxy.Auth
		if cfg.Auth.Username != "" {
			auth = &proxy.Auth{
				User:     cfg.Auth.Username,
				Password: cfg.Auth.Password,
			}
		}
		socksDialer, err := proxy.SOCKS5("tcp", cfg.Address, auth, dialer)
		if err != nil {
			return nil, fmt.Errorf("create socks5 dialer: %w", err)
		}
		tr.DialContext = dialContextFromDialer(socksDialer)
		tr.Proxy = nil
		return tr, nil
	default:
		return nil, fmt.Errorf("unknown proxy type: %s", cfg.Type)
	}
}

func dialContextFromDialer(d proxy.Dialer) func(ctx context.Context, network, addr string) (net.Conn, error) {
	if ctxDialer, ok := d.(proxy.ContextDialer); ok {
		return ctxDialer.DialContext
	}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := d.Dial(network, addr)
		if err != nil {
			return nil, err
		}
		select {
		case <-ctx.Done():
			_ = conn.Close()
			return nil, ctx.Err()
		default:
			return conn, nil
		}
	}
}

func durationFromSeconds(seconds int, fallback time.Duration) time.Duration {
	if seconds <= 0 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}
