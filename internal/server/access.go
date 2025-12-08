package server

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"sockstream/internal/config"
)

type AccessControl struct {
	allow []*net.IPNet
	block []*net.IPNet
}

func NewAccessControl(cfg config.AccessConfig) (*AccessControl, error) {
	ac := &AccessControl{}
	for _, cidr := range cfg.AllowCIDRs {
		_, n, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("parse allow cidr %s: %w", cidr, err)
		}
		ac.allow = append(ac.allow, n)
	}
	for _, cidr := range cfg.BlockCIDRs {
		_, n, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("parse block cidr %s: %w", cidr, err)
		}
		ac.block = append(ac.block, n)
	}
	return ac, nil
}

// Allowed returns true when the client IP is permitted by allow/block lists.
func (a *AccessControl) Allowed(ip net.IP) bool {
	if ip == nil {
		return false
	}

	for _, n := range a.block {
		if n.Contains(ip) {
			return false
		}
	}
	if len(a.allow) == 0 {
		return true
	}
	for _, n := range a.allow {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func clientIP(r *http.Request) net.IP {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			if ip := net.ParseIP(strings.TrimSpace(parts[0])); ip != nil {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return net.ParseIP(r.RemoteAddr)
	}
	return net.ParseIP(host)
}
