package proxy

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/proxy"

	"sockstream/internal/config"
)

// ProxyPool manages a pool of proxy transports with rotation
type ProxyPool struct {
	transports []http.RoundTripper
	rotation   string
	counter    atomic.Uint64
	mu         sync.RWMutex
}

// NewProxyPool creates a new proxy pool from config
func NewProxyPool(cfg config.ProxyConfig) (*ProxyPool, error) {
	proxies, err := cfg.GetProxies()
	if err != nil {
		return nil, err
	}

	pool := &ProxyPool{
		rotation: strings.ToLower(cfg.Rotation),
	}

	if pool.rotation == "" {
		pool.rotation = "round-robin"
	}

	// If no proxies configured, use direct connection
	if len(proxies) == 0 {
		tr, err := newDirectTransport(cfg.Timeouts)
		if err != nil {
			return nil, err
		}
		pool.transports = []http.RoundTripper{tr}
		return pool, nil
	}

	// Create transport for each proxy
	for _, p := range proxies {
		tr, err := newProxyTransport(p, cfg.Timeouts)
		if err != nil {
			return nil, fmt.Errorf("create transport for %s://%s: %w", p.Type, p.Address, err)
		}
		pool.transports = append(pool.transports, tr)
	}

	return pool, nil
}

// RoundTrip implements http.RoundTripper with proxy rotation
func (p *ProxyPool) RoundTrip(req *http.Request) (*http.Response, error) {
	tr := p.nextTransport()
	return tr.RoundTrip(req)
}

func (p *ProxyPool) nextTransport() http.RoundTripper {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.transports) == 1 {
		return p.transports[0]
	}

	var idx int
	switch p.rotation {
	case "random":
		idx = rand.Intn(len(p.transports))
	default: // round-robin
		idx = int(p.counter.Add(1)-1) % len(p.transports)
	}

	return p.transports[idx]
}

// Size returns the number of proxies in the pool
func (p *ProxyPool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.transports)
}

// NewTransport builds an HTTP transport configured with optional upstream proxy.
// For backward compatibility, returns a single transport.
func NewTransport(cfg config.ProxyConfig) (http.RoundTripper, error) {
	return NewProxyPool(cfg)
}

func newDirectTransport(timeouts config.TimeoutConfig) (*http.Transport, error) {
	dialer := &net.Dialer{
		Timeout:   durationFromSeconds(timeouts.ConnectSeconds, 10*time.Second),
		KeepAlive: 30 * time.Second,
	}

	return &http.Transport{
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       durationFromSeconds(timeouts.IdleSeconds, 30*time.Second),
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		Proxy:                 http.ProxyFromEnvironment,
	}, nil
}

func newProxyTransport(p config.ParsedProxy, timeouts config.TimeoutConfig) (*http.Transport, error) {
	dialer := &net.Dialer{
		Timeout:   durationFromSeconds(timeouts.ConnectSeconds, 10*time.Second),
		KeepAlive: 30 * time.Second,
	}

	tr := &http.Transport{
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       durationFromSeconds(timeouts.IdleSeconds, 30*time.Second),
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	switch p.Type {
	case "http", "https":
		if p.Address == "" {
			return nil, fmt.Errorf("proxy address required for http/https proxy")
		}
		u, err := url.Parse(fmt.Sprintf("%s://%s", p.Type, p.Address))
		if err != nil {
			return nil, fmt.Errorf("parse proxy url: %w", err)
		}
		if p.Username != "" {
			u.User = url.UserPassword(p.Username, p.Password)
		}
		tr.Proxy = http.ProxyURL(u)
		return tr, nil

	case "socks5":
		if p.Address == "" {
			return nil, fmt.Errorf("proxy address required for socks5")
		}
		var auth *proxy.Auth
		if p.Username != "" {
			auth = &proxy.Auth{
				User:     p.Username,
				Password: p.Password,
			}
		}
		socksDialer, err := proxy.SOCKS5("tcp", p.Address, auth, dialer)
		if err != nil {
			return nil, fmt.Errorf("create socks5 dialer: %w", err)
		}
		tr.DialContext = dialContextFromDialer(socksDialer)
		tr.Proxy = nil
		return tr, nil

	default:
		return nil, fmt.Errorf("unknown proxy type: %s", p.Type)
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
