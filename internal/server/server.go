package server

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/acme/autocert"

	"sockstream/internal/config"
)

type Server struct {
	cfg     config.Config
	logger  *log.Logger
	handler http.Handler
}

func New(cfg config.Config, logger *log.Logger, proxyHandler http.Handler) (*Server, error) {
	ac, err := NewAccessControl(cfg.Access)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/", proxyHandler)

	handler := chain(mux,
		accessMiddleware(ac),
		corsMiddleware(cfg.CORS),
		loggingMiddleware(logger),
	)

	return &Server{
		cfg:     cfg,
		logger:  logger,
		handler: handler,
	}, nil
}

func (s *Server) Start(ctx context.Context) error {
	httpSrv := &http.Server{
		Addr:         s.cfg.Listen,
		Handler:      s.handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	var acmeSrv *http.Server
	if s.cfg.TLS.ACME.Enabled {
		manager := s.acmeManager()
		httpSrv.TLSConfig = manager.TLSConfig()

		acmeAddr := s.acmeAddr()
		acmeSrv = &http.Server{
			Addr:    acmeAddr,
			Handler: manager.HTTPHandler(nil),
		}
		go func() {
			<-ctx.Done()
			shutdownWithLog(acmeSrv, s.logger)
		}()
		go func() {
			if err := acmeSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				s.logger.Printf("acme http server error: %v", err)
			}
		}()
	}

	go func() {
		<-ctx.Done()
		shutdownWithLog(httpSrv, s.logger)
	}()

	if s.cfg.TLS.HasCertificates() {
		return httpSrv.ListenAndServeTLS(s.cfg.TLS.CertFile, s.cfg.TLS.KeyFile)
	}
	if s.cfg.TLS.ACME.Enabled {
		return httpSrv.ListenAndServeTLS("", "")
	}
	return httpSrv.ListenAndServe()
}

func (s *Server) acmeManager() *autocert.Manager {
	host := s.cfg.TLS.ACME.Domain
	policy := autocert.HostWhitelist(host)
	return &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: policy,
		Cache:      autocert.DirCache(s.cfg.TLS.ACME.CacheDir),
		Email:      s.cfg.TLS.ACME.Email,
	}
}

func (s *Server) acmeAddr() string {
	addr := s.cfg.TLS.ACME.HTTP01Port
	if addr == "" {
		return ":80"
	}
	if strings.HasPrefix(addr, ":") {
		return addr
	}
	return ":" + addr
}

func chain(h http.Handler, m ...middleware) http.Handler {
	for i := len(m) - 1; i >= 0; i-- {
		h = m[i](h)
	}
	return h
}

func shutdownWithLog(srv *http.Server, logger *log.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil && err != context.Canceled {
		logger.Printf("shutdown: %v", err)
	}
}
