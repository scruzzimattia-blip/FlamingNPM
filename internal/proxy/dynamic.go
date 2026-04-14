package proxy

import (
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/flamingnpm/waf/internal/database"
	"github.com/flamingnpm/waf/internal/waf"
)

// runtimeRoute ist eine zur Laufzeit aufgeloeste Upstream-Zuordnung.
type runtimeRoute struct {
	hostLower  string
	target     *url.URL
	pathPrefix string
}

// DynamicRouter waehlt das Backend anhand des Host-Headers (Host-Key ohne Port).
// Ohne Treffer wird der konfigurierte Standard-Backend verwendet (falls gesetzt),
// sonst wird 404 zurueckgegeben.
type DynamicRouter struct {
	engine     *waf.Engine
	defaultURL *url.URL
	db         *database.DB
	mu         sync.RWMutex
	hostRoutes map[string]runtimeRoute
	proxyMu    sync.Mutex
	proxies    map[string]*httputil.ReverseProxy
	transport  *http.Transport
}

// NewDynamicRouter erstellt den Router und laedt die Routen aus der Datenbank.
func NewDynamicRouter(defaultBackend string, engine *waf.Engine, db *database.DB) (*DynamicRouter, error) {
	var u *url.URL
	if strings.TrimSpace(defaultBackend) != "" {
		parsed, err := url.Parse(defaultBackend)
		if err != nil {
			return nil, err
		}
		if parsed.Scheme == "" || parsed.Host == "" {
			return nil, &url.Error{Op: "parse", URL: defaultBackend, Err: url.InvalidHostError("default backend muss schema://host enthalten")}
		}
		u = parsed
	}

	dr := &DynamicRouter{
		engine:     engine,
		defaultURL: u,
		db:         db,
		proxies:    make(map[string]*httputil.ReverseProxy),
		transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 50,
			IdleConnTimeout:     90 * time.Second,
		},
	}
	if err := dr.ReloadRoutes(); err != nil {
		return nil, err
	}
	return dr, nil
}

// ReloadRoutes laedt aktive proxy_routes neu (z.B. nach Aenderungen in der Web-UI).
func (dr *DynamicRouter) ReloadRoutes() error {
	dbRoutes, err := dr.db.GetEnabledProxyRoutes()
	if err != nil {
		return err
	}

	m := make(map[string]runtimeRoute, len(dbRoutes))
	for _, r := range dbRoutes {
		t, err := url.Parse(r.BackendURL)
		if err != nil {
			log.Printf("WARNUNG: Route '%s' hat ungueltige backend_url: %v", r.Host, err)
			continue
		}
		hk := normalizeConfigHost(r.Host)
		if hk == "" {
			continue
		}
		m[hk] = runtimeRoute{
			hostLower:  hk,
			target:     t,
			pathPrefix: NormalizePathPrefix(r.PathPrefix),
		}
	}

	dr.mu.Lock()
	dr.hostRoutes = m
	dr.mu.Unlock()

	dr.proxyMu.Lock()
	dr.proxies = make(map[string]*httputil.ReverseProxy)
	dr.proxyMu.Unlock()

	log.Printf("Dynamisches Routing: %d aktive Host-Regeln geladen", len(m))
	return nil
}

func (dr *DynamicRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	allowed, ruleName, matched := dr.engine.CheckRequest(r)
	if !allowed {
		log.Printf("BLOCKIERT: %s %s von %s — Regel: %s, Match: %s",
			r.Method, r.URL.Path, extractIP(r), ruleName, truncateLog(matched, 100))
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-WAF-Block-Reason", ruleName)
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("403 Forbidden — Anfrage durch WAF blockiert"))
		return
	}

	hostKey := hostKeyFromRequest(r)
	var chosen *url.URL
	var pathPrefix string

	dr.mu.RLock()
	if rt, ok := dr.hostRoutes[hostKey]; ok {
		chosen = rt.target
		pathPrefix = rt.pathPrefix
	}
	dr.mu.RUnlock()

	if chosen == nil {
		if dr.defaultURL == nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("404 Not Found — keine Proxy-Route fuer diesen Host konfiguriert"))
			return
		}
		chosen = dr.defaultURL
	} else if pathPrefix != "" {
		r.URL.Path = StripPathPrefixIfMatches(r.URL.Path, pathPrefix)
	}

	p := dr.reverseProxyFor(chosen)
	p.ServeHTTP(w, r)

	log.Printf("WEITERGELEITET: %s %s Host=%s -> %s (Dauer: %v)",
		r.Method, r.URL.Path, hostKey, chosen.String(), time.Since(start))
}

func hostKeyFromRequest(r *http.Request) string {
	h := strings.ToLower(strings.TrimSpace(r.Host))
	if host, _, err := net.SplitHostPort(h); err == nil {
		return host
	}
	return h
}

func (dr *DynamicRouter) reverseProxyFor(target *url.URL) *httputil.ReverseProxy {
	key := target.String()

	dr.proxyMu.Lock()
	defer dr.proxyMu.Unlock()

	if p, ok := dr.proxies[key]; ok {
		return p
	}

	p := httputil.NewSingleHostReverseProxy(target)
	orig := p.Director
	p.Director = func(req *http.Request) {
		orig(req)
		enrichProxyRequestHeaders(req)
	}
	p.Transport = dr.transport
	p.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		log.Printf("Proxy-Fehler: %v (Ziel: %s%s)", err, target.Host, req.URL.Path)
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("502 Bad Gateway"))
	}

	dr.proxies[key] = p
	return p
}

// DefaultBackendURL liefert den Standard-Upstream (Umgebungsvariable BACKEND_URL).
func (dr *DynamicRouter) DefaultBackendURL() string {
	if dr.defaultURL == nil {
		return ""
	}
	return dr.defaultURL.String()
}
