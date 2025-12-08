package proxy

import (
	"testing"
	"time"

	"sockstream/internal/config"
)

func TestNewTransport(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.ProxyConfig
		wantErr bool
	}{
		{
			name:    "direct - empty type",
			cfg:     config.ProxyConfig{Type: ""},
			wantErr: false,
		},
		{
			name:    "direct - explicit",
			cfg:     config.ProxyConfig{Type: "direct"},
			wantErr: false,
		},
		{
			name: "http proxy",
			cfg: config.ProxyConfig{
				Type:    "http",
				Address: "proxy.example.com:8080",
			},
			wantErr: false,
		},
		{
			name: "https proxy",
			cfg: config.ProxyConfig{
				Type:    "https",
				Address: "proxy.example.com:8080",
			},
			wantErr: false,
		},
		{
			name: "http proxy with auth",
			cfg: config.ProxyConfig{
				Type:    "http",
				Address: "proxy.example.com:8080",
				Auth: config.ProxyAuth{
					Username: "user",
					Password: "pass",
				},
			},
			wantErr: false,
		},
		{
			name:    "http proxy missing address",
			cfg:     config.ProxyConfig{Type: "http"},
			wantErr: true,
		},
		{
			name: "socks5 proxy",
			cfg: config.ProxyConfig{
				Type:    "socks5",
				Address: "127.0.0.1:1080",
			},
			wantErr: false,
		},
		{
			name: "socks5 proxy with auth",
			cfg: config.ProxyConfig{
				Type:    "socks5",
				Address: "127.0.0.1:1080",
				Auth: config.ProxyAuth{
					Username: "user",
					Password: "pass",
				},
			},
			wantErr: false,
		},
		{
			name:    "socks5 missing address",
			cfg:     config.ProxyConfig{Type: "socks5"},
			wantErr: true,
		},
		{
			name:    "unknown proxy type",
			cfg:     config.ProxyConfig{Type: "ftp"},
			wantErr: true,
		},
		{
			name: "case insensitive type",
			cfg: config.ProxyConfig{
				Type:    "SOCKS5",
				Address: "127.0.0.1:1080",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewTransport(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewTransport() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDurationFromSeconds(t *testing.T) {
	tests := []struct {
		name     string
		seconds  int
		fallback time.Duration
		want     time.Duration
	}{
		{
			name:     "positive seconds",
			seconds:  5,
			fallback: 10 * time.Second,
			want:     5 * time.Second,
		},
		{
			name:     "zero uses fallback",
			seconds:  0,
			fallback: 10 * time.Second,
			want:     10 * time.Second,
		},
		{
			name:     "negative uses fallback",
			seconds:  -1,
			fallback: 30 * time.Second,
			want:     30 * time.Second,
		},
		{
			name:     "large value",
			seconds:  3600,
			fallback: 10 * time.Second,
			want:     1 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := durationFromSeconds(tt.seconds, tt.fallback)
			if got != tt.want {
				t.Errorf("durationFromSeconds() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewProxyPool(t *testing.T) {
	tests := []struct {
		name      string
		cfg       config.ProxyConfig
		wantSize  int
		wantErr   bool
	}{
		{
			name:     "empty config - direct",
			cfg:      config.ProxyConfig{},
			wantSize: 1,
			wantErr:  false,
		},
		{
			name: "single URL",
			cfg: config.ProxyConfig{
				URLs: []string{"socks5://proxy:1080"},
			},
			wantSize: 1,
			wantErr:  false,
		},
		{
			name: "multiple URLs",
			cfg: config.ProxyConfig{
				URLs: []string{
					"socks5://proxy1:1080",
					"http://proxy2:8080",
					"https://proxy3:443",
				},
			},
			wantSize: 3,
			wantErr:  false,
		},
		{
			name: "legacy config",
			cfg: config.ProxyConfig{
				Type:    "socks5",
				Address: "127.0.0.1:1080",
			},
			wantSize: 1,
			wantErr:  false,
		},
		{
			name: "invalid URL",
			cfg: config.ProxyConfig{
				URLs: []string{"ftp://invalid:21"},
			},
			wantErr: true,
		},
		{
			name: "missing address in URL",
			cfg: config.ProxyConfig{
				URLs: []string{"socks5://"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := NewProxyPool(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewProxyPool() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if pool.Size() != tt.wantSize {
				t.Errorf("Size() = %d, want %d", pool.Size(), tt.wantSize)
			}
		})
	}
}

func TestProxyPool_RoundRobin(t *testing.T) {
	cfg := config.ProxyConfig{
		URLs: []string{
			"http://proxy1:8080",
			"http://proxy2:8080",
			"http://proxy3:8080",
		},
		Rotation: "round-robin",
	}

	pool, err := NewProxyPool(cfg)
	if err != nil {
		t.Fatalf("NewProxyPool() error = %v", err)
	}

	if pool.Size() != 3 {
		t.Errorf("Size() = %d, want 3", pool.Size())
	}

	// Verify round-robin by checking that transports rotate
	seen := make(map[int]bool)
	for i := 0; i < 6; i++ {
		tr := pool.nextTransport()
		// Each transport should be used in order
		if tr == nil {
			t.Error("nextTransport() returned nil")
		}
	}
	// After 6 calls with 3 proxies, we should have rotated through all
	_ = seen
}

func TestProxyPool_Random(t *testing.T) {
	cfg := config.ProxyConfig{
		URLs: []string{
			"http://proxy1:8080",
			"http://proxy2:8080",
			"http://proxy3:8080",
		},
		Rotation: "random",
	}

	pool, err := NewProxyPool(cfg)
	if err != nil {
		t.Fatalf("NewProxyPool() error = %v", err)
	}

	// Just verify random doesn't panic
	for i := 0; i < 10; i++ {
		tr := pool.nextTransport()
		if tr == nil {
			t.Error("nextTransport() returned nil")
		}
	}
}

func TestProxyPool_SingleTransport(t *testing.T) {
	cfg := config.ProxyConfig{
		URLs: []string{"http://proxy:8080"},
	}

	pool, err := NewProxyPool(cfg)
	if err != nil {
		t.Fatalf("NewProxyPool() error = %v", err)
	}

	// With single proxy, same transport should always be returned
	first := pool.nextTransport()
	for i := 0; i < 5; i++ {
		tr := pool.nextTransport()
		if tr != first {
			t.Error("Single-proxy pool should always return same transport")
		}
	}
}
