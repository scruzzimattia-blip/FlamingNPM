package proxy

import (
	"net"
	"strings"
)

// normalizeConfigHost bringt einen Host aus der Konfiguration auf den gleichen Schlussel wie hostKeyFromRequest.
func normalizeConfigHost(host string) string {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" {
		return ""
	}
	if strings.Contains(h, ":") {
		if hostPart, _, err := net.SplitHostPort(h); err == nil {
			return hostPart
		}
		if idx := strings.LastIndex(h, ":"); idx > 0 {
			return h[:idx]
		}
	}
	return h
}
