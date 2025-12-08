package server

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"sockstream/internal/config"
)

type middleware func(http.Handler) http.Handler

func loggingMiddleware(logger *log.Logger) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			start := time.Now()
			next.ServeHTTP(rec, r)
			logger.Printf("%s %s %d %s", r.Method, r.URL.String(), rec.status, time.Since(start))
		})
	}
}

func corsMiddleware(cfg config.CORSConfig) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if originAllowed(cfg.AllowedOrigins, origin) {
				if len(cfg.AllowedOrigins) == 1 && cfg.AllowedOrigins[0] == "*" {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else if origin != "" {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}
				if cfg.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
				if len(cfg.ExposeHeaders) > 0 {
					w.Header().Set("Access-Control-Expose-Headers", strings.Join(cfg.ExposeHeaders, ","))
				}
				if len(cfg.AllowedHeaders) > 0 {
					w.Header().Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ","))
				}
				if len(cfg.AllowMethods) > 0 {
					w.Header().Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowMethods, ","))
				}
			}

			if cfg.MaxAgeSeconds > 0 {
				w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", cfg.MaxAgeSeconds))
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func accessMiddleware(ac *AccessControl) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if ac == nil {
				next.ServeHTTP(w, r)
				return
			}
			if ip := clientIP(r); !ac.Allowed(ip) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func originAllowed(allowed []string, origin string) bool {
	if len(allowed) == 0 {
		return false
	}
	if allowed[0] == "*" {
		return true
	}
	for _, o := range allowed {
		if strings.EqualFold(strings.TrimSpace(o), origin) {
			return true
		}
	}
	return false
}
