package server

import (
	"net"
	"net/http"
	"testing"

	"sockstream/internal/config"
)

func TestNewAccessControl(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.AccessConfig
		wantErr bool
	}{
		{
			name:    "empty config",
			cfg:     config.AccessConfig{},
			wantErr: false,
		},
		{
			name: "valid allow CIDRs",
			cfg: config.AccessConfig{
				AllowCIDRs: []string{"192.168.0.0/16", "10.0.0.0/8"},
			},
			wantErr: false,
		},
		{
			name: "valid block CIDRs",
			cfg: config.AccessConfig{
				BlockCIDRs: []string{"192.168.1.0/24"},
			},
			wantErr: false,
		},
		{
			name: "invalid allow CIDR",
			cfg: config.AccessConfig{
				AllowCIDRs: []string{"invalid-cidr"},
			},
			wantErr: true,
		},
		{
			name: "invalid block CIDR",
			cfg: config.AccessConfig{
				BlockCIDRs: []string{"256.0.0.0/8"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewAccessControl(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewAccessControl() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAccessControl_Allowed(t *testing.T) {
	tests := []struct {
		name   string
		allow  []string
		block  []string
		ip     string
		want   bool
	}{
		{
			name:  "nil IP",
			allow: []string{},
			block: []string{},
			ip:    "",
			want:  false,
		},
		{
			name:  "empty lists allow all",
			allow: []string{},
			block: []string{},
			ip:    "192.168.1.1",
			want:  true,
		},
		{
			name:  "IP in allow list",
			allow: []string{"192.168.0.0/16"},
			block: []string{},
			ip:    "192.168.1.100",
			want:  true,
		},
		{
			name:  "IP not in allow list",
			allow: []string{"192.168.0.0/16"},
			block: []string{},
			ip:    "10.0.0.1",
			want:  false,
		},
		{
			name:  "IP in block list",
			allow: []string{},
			block: []string{"192.168.1.0/24"},
			ip:    "192.168.1.50",
			want:  false,
		},
		{
			name:  "block takes precedence over allow",
			allow: []string{"192.168.0.0/16"},
			block: []string{"192.168.1.0/24"},
			ip:    "192.168.1.50",
			want:  false,
		},
		{
			name:  "allowed when not blocked and in allow list",
			allow: []string{"192.168.0.0/16"},
			block: []string{"192.168.1.0/24"},
			ip:    "192.168.2.50",
			want:  true,
		},
		{
			name:  "IPv6 address allowed",
			allow: []string{"2001:db8::/32"},
			block: []string{},
			ip:    "2001:db8::1",
			want:  true,
		},
		{
			name:  "IPv6 address blocked",
			allow: []string{},
			block: []string{"2001:db8::/32"},
			ip:    "2001:db8::1",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ac, err := NewAccessControl(config.AccessConfig{
				AllowCIDRs: tt.allow,
				BlockCIDRs: tt.block,
			})
			if err != nil {
				t.Fatalf("NewAccessControl() error = %v", err)
			}

			var ip net.IP
			if tt.ip != "" {
				ip = net.ParseIP(tt.ip)
			}

			if got := ac.Allowed(ip); got != tt.want {
				t.Errorf("Allowed(%s) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		wantIP     string
	}{
		{
			name:       "from RemoteAddr with port",
			remoteAddr: "192.168.1.1:12345",
			xff:        "",
			wantIP:     "192.168.1.1",
		},
		{
			name:       "from RemoteAddr without port",
			remoteAddr: "192.168.1.1",
			xff:        "",
			wantIP:     "192.168.1.1",
		},
		{
			name:       "from X-Forwarded-For single IP",
			remoteAddr: "127.0.0.1:12345",
			xff:        "203.0.113.50",
			wantIP:     "203.0.113.50",
		},
		{
			name:       "from X-Forwarded-For multiple IPs",
			remoteAddr: "127.0.0.1:12345",
			xff:        "203.0.113.50, 70.41.3.18, 150.172.238.178",
			wantIP:     "203.0.113.50",
		},
		{
			name:       "from X-Forwarded-For with spaces",
			remoteAddr: "127.0.0.1:12345",
			xff:        "  203.0.113.50  ",
			wantIP:     "203.0.113.50",
		},
		{
			name:       "IPv6 RemoteAddr",
			remoteAddr: "[::1]:12345",
			xff:        "",
			wantIP:     "::1",
		},
		{
			name:       "IPv6 in X-Forwarded-For",
			remoteAddr: "127.0.0.1:12345",
			xff:        "2001:db8::1",
			wantIP:     "2001:db8::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				RemoteAddr: tt.remoteAddr,
				Header:     make(http.Header),
			}
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}

			got := clientIP(req)
			want := net.ParseIP(tt.wantIP)

			if !got.Equal(want) {
				t.Errorf("clientIP() = %v, want %v", got, want)
			}
		})
	}
}
