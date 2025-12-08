package proxy

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"sockstream/internal/config"
)

// NewReverseProxy constructs a reverse proxy with header rewrites and custom transport.
func NewReverseProxy(target *url.URL, cfg config.Config, transport http.RoundTripper, logger *slog.Logger) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(target)
	if transport != nil {
		proxy.Transport = transport
	}

	origDirector := proxy.Director
	proxy.Director = func(r *http.Request) {
		origDirector(r)
		applyRewrites(r, target, cfg.Headers)
		applyAddHeaders(r, cfg.Headers.Add)
		if cfg.HostName != "" {
			r.Host = cfg.HostName
		}
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		logger.Error("proxy error", "error", err, "url", r.URL.String())
		http.Error(w, "proxy error", http.StatusBadGateway)
	}

	return proxy
}

func applyRewrites(r *http.Request, target *url.URL, cfg config.HeaderConfig) {
	if cfg.RewriteHost {
		r.Host = target.Host
		r.Header.Set("Host", target.Host)
	}
	if cfg.RewriteOrigin && r.Header.Get("Origin") != "" {
		r.Header.Set("Origin", target.String())
	}
	if cfg.RewriteReferer && r.Header.Get("Referer") != "" {
		r.Header.Set("Referer", target.String())
	}
}

func applyAddHeaders(r *http.Request, headers map[string]string) {
	for k, v := range headers {
		if strings.TrimSpace(k) == "" {
			continue
		}
		r.Header.Set(k, v)
	}
}
