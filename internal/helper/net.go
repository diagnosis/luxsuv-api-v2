package helper

import (
	"net"
	"net/http"
	"strings"
)

func ClientIPNet(r *http.Request) net.IP {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if ip := net.ParseIP(strings.TrimSpace(parts[0])); ip != nil {
			return ip
		}
	}
	if xr := r.Header.Get("X-Real-IP"); xr != "" {
		if ip := net.ParseIP(strings.TrimSpace(xr)); ip != nil {
			return ip
		}
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return net.ParseIP(host)
}

func ClientIP(r *http.Request) string {
	ip := ClientIPNet(r)
	if ip == nil {
		return "unknown"
	}
	return ip.String()
}
