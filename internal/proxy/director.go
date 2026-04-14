package proxy

import "net/http"

// enrichProxyRequestHeaders setzt konsistente Forwarding-Header fuer Upstream-Services.
func enrichProxyRequestHeaders(req *http.Request) {
	clientIP := extractIP(req)
	if clientIP != "" {
		req.Header.Set("X-Real-IP", clientIP)
	}
	scheme := "http"
	if req.TLS != nil {
		scheme = "https"
	}
	if forwardedProto := req.Header.Get("X-Forwarded-Proto"); forwardedProto != "" {
		scheme = forwardedProto
	}
	req.Header.Set("X-Forwarded-Proto", scheme)
	if req.Host != "" {
		req.Header.Set("X-Forwarded-Host", req.Host)
	}
}
