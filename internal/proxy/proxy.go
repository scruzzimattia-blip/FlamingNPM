package proxy

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/flamingnpm/waf/internal/waf"
)

// ReverseProxy leitet validierte Anfragen an das konfigurierte Backend weiter.
type ReverseProxy struct {
	target  *url.URL
	proxy   *httputil.ReverseProxy
	engine  *waf.Engine
}

func New(targetURL string, engine *waf.Engine) (*ReverseProxy, error) {
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	proxy.Transport = &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 50,
		IdleConnTimeout:     90 * time.Second,
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("Proxy-Fehler: %v (Ziel: %s%s)", err, target.Host, r.URL.Path)
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("502 Bad Gateway"))
	}

	return &ReverseProxy{
		target: target,
		proxy:  proxy,
		engine: engine,
	}, nil
}

func (rp *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	allowed, ruleName, matched := rp.engine.CheckRequest(r)

	if !allowed {
		log.Printf("BLOCKIERT: %s %s von %s — Regel: %s, Match: %s",
			r.Method, r.URL.Path, extractIP(r), ruleName, truncateLog(matched, 100))

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-WAF-Block-Reason", ruleName)
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("403 Forbidden — Anfrage durch WAF blockiert"))
		return
	}

	rp.proxy.ServeHTTP(w, r)

	log.Printf("WEITERGELEITET: %s %s von %s (Dauer: %v)",
		r.Method, r.URL.Path, extractIP(r), time.Since(start))
}

func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	return r.RemoteAddr
}

func truncateLog(s string, max int) string {
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
