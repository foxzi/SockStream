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
