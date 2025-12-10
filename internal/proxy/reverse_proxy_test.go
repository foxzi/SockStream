package proxy

import (
	"net/http"
	"net/url"
	"testing"

	"sockstream/internal/config"
)

func TestApplyRewrites(t *testing.T) {
	target, _ := url.Parse("https://target.example.com")

	tests := []struct {
		name       string
		cfg        config.HeaderConfig
		reqHeaders map[string]string
		wantHost   string
		wantOrigin string
		wantRef    string
	}{
		{
			name: "rewrite host enabled",
			cfg:  config.HeaderConfig{RewriteHost: true},
			reqHeaders: map[string]string{
				"Host": "original.com",
			},
			wantHost: "target.example.com",
		},
		{
			name: "rewrite host disabled",
			cfg:  config.HeaderConfig{RewriteHost: false},
			reqHeaders: map[string]string{
				"Host": "original.com",
			},
			wantHost: "original.com",
		},
		{
			name: "rewrite origin enabled",
			cfg:  config.HeaderConfig{RewriteOrigin: true},
			reqHeaders: map[string]string{
				"Origin": "https://original.com",
			},
			wantOrigin: "https://target.example.com",
		},
		{
			name: "rewrite origin disabled",
			cfg:  config.HeaderConfig{RewriteOrigin: false},
			reqHeaders: map[string]string{
				"Origin": "https://original.com",
			},
			wantOrigin: "https://original.com",
		},
		{
			name: "rewrite origin skipped when empty",
			cfg:  config.HeaderConfig{RewriteOrigin: true},
			reqHeaders: map[string]string{
				"Origin": "",
			},
			wantOrigin: "",
		},
		{
			name: "rewrite referer enabled",
			cfg:  config.HeaderConfig{RewriteReferer: true},
			reqHeaders: map[string]string{
				"Referer": "https://original.com/page",
			},
			wantRef: "https://target.example.com",
		},
		{
			name: "rewrite referer disabled",
			cfg:  config.HeaderConfig{RewriteReferer: false},
			reqHeaders: map[string]string{
				"Referer": "https://original.com/page",
			},
			wantRef: "https://original.com/page",
		},
		{
			name: "rewrite referer skipped when empty",
			cfg:  config.HeaderConfig{RewriteReferer: true},
			reqHeaders: map[string]string{
				"Referer": "",
			},
			wantRef: "",
		},
		{
			name: "all rewrites enabled",
			cfg: config.HeaderConfig{
				RewriteHost:    true,
				RewriteOrigin:  true,
				RewriteReferer: true,
			},
			reqHeaders: map[string]string{
				"Host":    "original.com",
				"Origin":  "https://original.com",
				"Referer": "https://original.com/page",
			},
			wantHost:   "target.example.com",
			wantOrigin: "https://target.example.com",
			wantRef:    "https://target.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				Header: make(http.Header),
				Host:   tt.reqHeaders["Host"],
			}
			for k, v := range tt.reqHeaders {
				if k != "Host" && v != "" {
					req.Header.Set(k, v)
				}
			}

			applyRewrites(req, target, tt.cfg)

			if tt.wantHost != "" && req.Host != tt.wantHost {
				t.Errorf("Host = %q, want %q", req.Host, tt.wantHost)
			}
			if tt.wantOrigin != "" && req.Header.Get("Origin") != tt.wantOrigin {
				t.Errorf("Origin = %q, want %q", req.Header.Get("Origin"), tt.wantOrigin)
			}
			if tt.wantRef != "" && req.Header.Get("Referer") != tt.wantRef {
				t.Errorf("Referer = %q, want %q", req.Header.Get("Referer"), tt.wantRef)
			}
		})
	}
}

func TestApplyAddHeaders(t *testing.T) {
	tests := []struct {
		name        string
		headers     []string
		wantHeaders map[string]string
	}{
		{
			name:        "empty headers",
			headers:     []string{},
			wantHeaders: map[string]string{},
		},
		{
			name:    "add single header",
			headers: []string{"X-Custom-Header: value"},
			wantHeaders: map[string]string{
				"X-Custom-Header": "value",
			},
		},
		{
			name:    "add multiple headers",
			headers: []string{"X-Custom-Header: value1", "X-Another: value2"},
			wantHeaders: map[string]string{
				"X-Custom-Header": "value1",
				"X-Another":       "value2",
			},
		},
		{
			name:    "skip invalid format",
			headers: []string{"no-colon", "X-Custom-Header: value"},
			wantHeaders: map[string]string{
				"X-Custom-Header": "value",
			},
		},
		{
			name:    "skip empty key",
			headers: []string{": value1", "X-Custom-Header: value2"},
			wantHeaders: map[string]string{
				"X-Custom-Header": "value2",
			},
		},
		{
			name:    "allow empty value",
			headers: []string{"X-Custom-Header:"},
			wantHeaders: map[string]string{
				"X-Custom-Header": "",
			},
		},
		{
			name:    "trim spaces",
			headers: []string{"  X-Custom-Header  :  value  "},
			wantHeaders: map[string]string{
				"X-Custom-Header": "value",
			},
		},
		{
			name:    "value with colon",
			headers: []string{"Authorization: Bearer: token:123"},
			wantHeaders: map[string]string{
				"Authorization": "Bearer: token:123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				Header: make(http.Header),
			}

			applyAddHeaders(req, tt.headers)

			for k, want := range tt.wantHeaders {
				got := req.Header.Get(k)
				if got != want {
					t.Errorf("Header[%s] = %q, want %q", k, got, want)
				}
			}
		})
	}
}
