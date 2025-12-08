package proxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/proxy"

	"sockstream/internal/config"
)

const (
	defaultHealthCheckInterval = 5 * time.Minute
	defaultHealthCheckTimeout  = 10 * time.Second
	healthCheckURL             = "https://www.google.com/generate_204"
)

// proxyEntry holds a proxy transport and its health status
type proxyEntry struct {
	transport http.RoundTripper
	proxy     config.ParsedProxy
	healthy   atomic.Bool
	lastCheck time.Time
	lastError string
	mu        sync.RWMutex
}

func (e *proxyEntry) isHealthy() bool {
	return e.healthy.Load()
}

func (e *proxyEntry) setHealthy(healthy bool, err string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.healthy.Store(healthy)
	e.lastCheck = time.Now()
	e.lastError = err
}

func (e *proxyEntry) getLastError() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.lastError
}

// ProxyPool manages a pool of proxy transports with rotation and health checks
type ProxyPool struct {
	entries  []*proxyEntry
	rotation string
	counter  atomic.Uint64
	mu       sync.RWMutex
	logger   *slog.Logger
	stopCh   chan struct{}
	isDirect bool
}

// NewProxyPool creates a new proxy pool from config
func NewProxyPool(cfg config.ProxyConfig) (*ProxyPool, error) {
	proxies, err := cfg.GetProxies()
	if err != nil {
		return nil, err
	}

	pool := &ProxyPool{
		rotation: strings.ToLower(cfg.Rotation),
		stopCh:   make(chan struct{}),
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
		pool.entries = []*proxyEntry{{
			transport: tr,
			proxy:     config.ParsedProxy{Type: "direct", Address: "direct"},
		}}
		pool.entries[0].healthy.Store(true)
		pool.isDirect = true
		return pool, nil
	}

	// Create transport for each proxy
	for _, p := range proxies {
		tr, err := newProxyTransport(p, cfg.Timeouts)
		if err != nil {
			return nil, fmt.Errorf("create transport for %s://%s: %w", p.Type, p.Address, err)
		}
		entry := &proxyEntry{
			transport: tr,
			proxy:     p,
		}
		entry.healthy.Store(true) // assume healthy until checked
		pool.entries = append(pool.entries, entry)
	}

	return pool, nil
}

// SetLogger sets the logger for health check logging
func (p *ProxyPool) SetLogger(logger *slog.Logger) {
	p.logger = logger
}

// StartHealthCheck starts the health check routine
func (p *ProxyPool) StartHealthCheck(ctx context.Context) {
	if p.isDirect {
		return
	}

	// Initial health check
	p.checkAllProxies()

	// Periodic health check
	ticker := time.NewTicker(defaultHealthCheckInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-p.stopCh:
				return
			case <-ticker.C:
				p.checkAllProxies()
			}
		}
	}()
}

// Stop stops the health check routine
func (p *ProxyPool) Stop() {
	close(p.stopCh)
}

func (p *ProxyPool) checkAllProxies() {
	p.mu.RLock()
	entries := make([]*proxyEntry, len(p.entries))
	copy(entries, p.entries)
	p.mu.RUnlock()

	var wg sync.WaitGroup
	for _, entry := range entries {
		wg.Add(1)
		go func(e *proxyEntry) {
			defer wg.Done()
			p.checkProxy(e)
		}(entry)
	}
	wg.Wait()

	// Log summary
	p.logHealthSummary()
}

func (p *ProxyPool) checkProxy(entry *proxyEntry) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultHealthCheckTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthCheckURL, nil)
	if err != nil {
		entry.setHealthy(false, fmt.Sprintf("create request: %v", err))
		p.logProxyStatus(entry, false, entry.getLastError())
		return
	}

	client := &http.Client{
		Transport: entry.transport,
		Timeout:   defaultHealthCheckTimeout,
	}

	resp, err := client.Do(req)
	if err != nil {
		entry.setHealthy(false, err.Error())
		p.logProxyStatus(entry, false, err.Error())
		return
	}
	defer resp.Body.Close()

	// Google's generate_204 returns 204, but any 2xx is OK
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		wasUnhealthy := !entry.isHealthy()
		entry.setHealthy(true, "")
		if wasUnhealthy {
			p.logProxyStatus(entry, true, "recovered")
		} else {
			p.logProxyStatus(entry, true, "")
		}
	} else {
		errMsg := fmt.Sprintf("unexpected status: %d", resp.StatusCode)
		entry.setHealthy(false, errMsg)
		p.logProxyStatus(entry, false, errMsg)
	}
}

func (p *ProxyPool) logProxyStatus(entry *proxyEntry, healthy bool, errMsg string) {
	if p.logger == nil {
		return
	}

	proxyAddr := fmt.Sprintf("%s://%s", entry.proxy.Type, entry.proxy.Address)
	if healthy {
		if errMsg == "recovered" {
			p.logger.Info("proxy recovered", "proxy", proxyAddr)
		} else {
			p.logger.Debug("proxy healthy", "proxy", proxyAddr)
		}
	} else {
		p.logger.Warn("proxy unhealthy", "proxy", proxyAddr, "error", errMsg)
	}
}

func (p *ProxyPool) logHealthSummary() {
	if p.logger == nil {
		return
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	healthy := 0
	unhealthy := 0
	for _, e := range p.entries {
		if e.isHealthy() {
			healthy++
		} else {
			unhealthy++
		}
	}

	p.logger.Info("proxy pool health check complete",
		"healthy", healthy,
		"unhealthy", unhealthy,
		"total", len(p.entries),
	)
}

// RoundTrip implements http.RoundTripper with proxy rotation and retry on timeout
func (p *ProxyPool) RoundTrip(req *http.Request) (*http.Response, error) {
	entries := p.getHealthyEntries()
	if len(entries) == 0 {
		return nil, fmt.Errorf("no proxies available")
	}

	// For single proxy or direct connection, no retry needed
	if len(entries) == 1 || p.isDirect {
		return entries[0].transport.RoundTrip(req)
	}

	// Buffer request body for potential retries
	var bodyBytes []byte
	if req.Body != nil && req.Body != http.NoBody {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read request body: %w", err)
		}
	}

	tried := make(map[int]bool)
	var lastErr error

	for len(tried) < len(entries) {
		idx := p.selectProxyIndex(entries, tried)
		if idx < 0 {
			break
		}
		tried[idx] = true
		entry := entries[idx]

		// Restore body for retry
		if bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		resp, err := entry.transport.RoundTrip(req)
		if err == nil {
			return resp, nil
		}

		lastErr = err

		// Only retry on timeout errors
		if !isTimeoutError(err) {
			if p.logger != nil {
				p.logger.Error("proxy request failed (not retrying)",
					"proxy", fmt.Sprintf("%s://%s", entry.proxy.Type, entry.proxy.Address),
					"error", err)
			}
			return nil, err
		}

		// Log timeout and mark proxy as unhealthy
		if p.logger != nil {
			p.logger.Warn("proxy timeout, trying next",
				"proxy", fmt.Sprintf("%s://%s", entry.proxy.Type, entry.proxy.Address),
				"tried", len(tried),
				"total", len(entries))
		}
		entry.setHealthy(false, err.Error())
	}

	return nil, fmt.Errorf("all proxies failed: %w", lastErr)
}

func (p *ProxyPool) getHealthyEntries() []*proxyEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var healthyEntries []*proxyEntry
	for _, e := range p.entries {
		if e.isHealthy() {
			healthyEntries = append(healthyEntries, e)
		}
	}

	// Fallback to all entries if none healthy
	if len(healthyEntries) == 0 {
		if p.logger != nil && len(p.entries) > 0 {
			p.logger.Warn("no healthy proxies, using fallback")
		}
		return p.entries
	}
	return healthyEntries
}

func (p *ProxyPool) selectProxyIndex(entries []*proxyEntry, tried map[int]bool) int {
	available := make([]int, 0, len(entries))
	for i := range entries {
		if !tried[i] {
			available = append(available, i)
		}
	}
	if len(available) == 0 {
		return -1
	}

	switch p.rotation {
	case "random":
		return available[rand.Intn(len(available))]
	default: // round-robin
		idx := int(p.counter.Add(1)-1) % len(available)
		return available[idx]
	}
}

// isTimeoutError checks if the error is a timeout
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}

	// Check for context deadline exceeded
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	// Check for os.ErrDeadlineExceeded
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}

	// Check for net.Error timeout
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	// Check for url.Error wrapping a timeout
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return isTimeoutError(urlErr.Err)
	}

	return false
}

func (p *ProxyPool) nextTransport() (http.RoundTripper, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Get healthy entries
	var healthyEntries []*proxyEntry
	for _, e := range p.entries {
		if e.isHealthy() {
			healthyEntries = append(healthyEntries, e)
		}
	}

	// If no healthy proxies, try all (fallback)
	if len(healthyEntries) == 0 {
		if len(p.entries) == 0 {
			return nil, fmt.Errorf("no proxies available")
		}
		// Use any proxy as fallback
		healthyEntries = p.entries
		if p.logger != nil {
			p.logger.Warn("no healthy proxies, using fallback")
		}
	}

	if len(healthyEntries) == 1 {
		return healthyEntries[0].transport, nil
	}

	var idx int
	switch p.rotation {
	case "random":
		idx = rand.Intn(len(healthyEntries))
	default: // round-robin
		idx = int(p.counter.Add(1)-1) % len(healthyEntries)
	}

	return healthyEntries[idx].transport, nil
}

// Size returns the total number of proxies in the pool
func (p *ProxyPool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.entries)
}

// HealthyCount returns the number of healthy proxies
func (p *ProxyPool) HealthyCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	count := 0
	for _, e := range p.entries {
		if e.isHealthy() {
			count++
		}
	}
	return count
}

// GetStatus returns status of all proxies
func (p *ProxyPool) GetStatus() []ProxyStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var statuses []ProxyStatus
	for _, e := range p.entries {
		e.mu.RLock()
		statuses = append(statuses, ProxyStatus{
			Address:   fmt.Sprintf("%s://%s", e.proxy.Type, e.proxy.Address),
			Healthy:   e.isHealthy(),
			LastCheck: e.lastCheck,
			LastError: e.lastError,
		})
		e.mu.RUnlock()
	}
	return statuses
}

// ProxyStatus represents the status of a single proxy
type ProxyStatus struct {
	Address   string
	Healthy   bool
	LastCheck time.Time
	LastError string
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
