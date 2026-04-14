package waf

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/flamingnpm/waf/internal/database"
	"github.com/flamingnpm/waf/internal/models"
)

// compiledRule haelt eine vorcompilierte Regex zusammen mit der zugehoerigen Regel.
type compiledRule struct {
	rule  models.FirewallRule
	regex *regexp.Regexp
}

// Engine ist die zentrale WAF-Pruefinstanz.
type Engine struct {
	db             *database.DB
	rules          []compiledRule
	mu             sync.RWMutex
	maxBodySize    int64
	rateLimitMax   int
	rateLimitWindow int
	onBlock        func(*models.BlockedRequest)
}

type Config struct {
	MaxBodySize     int64
	RateLimitMax    int
	RateLimitWindow int
}

func NewEngine(db *database.DB, cfg Config) (*Engine, error) {
	e := &Engine{
		db:              db,
		maxBodySize:     cfg.MaxBodySize,
		rateLimitMax:    cfg.RateLimitMax,
		rateLimitWindow: cfg.RateLimitWindow,
	}
	if err := e.ReloadRules(); err != nil {
		return nil, err
	}
	return e, nil
}

// SetOnBlock registriert einen Callback, der bei jedem Block aufgerufen wird.
func (e *Engine) SetOnBlock(fn func(*models.BlockedRequest)) {
	e.onBlock = fn
}

// ReloadRules laedt alle aktiven Regeln aus der Datenbank und compiliert die Regex-Patterns.
func (e *Engine) ReloadRules() error {
	dbRules, err := e.db.GetEnabledRules()
	if err != nil {
		return fmt.Errorf("regeln laden fehlgeschlagen: %w", err)
	}

	compiled := make([]compiledRule, 0, len(dbRules))
	for _, r := range dbRules {
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			log.Printf("WARNUNG: Regel '%s' hat ungueltiges Pattern '%s': %v", r.Name, r.Pattern, err)
			continue
		}
		compiled = append(compiled, compiledRule{rule: r, regex: re})
	}

	e.mu.Lock()
	e.rules = compiled
	e.mu.Unlock()

	log.Printf("WAF-Engine: %d Regeln geladen", len(compiled))
	return nil
}

// CheckRequest prueft einen eingehenden Request gegen alle aktiven Regeln.
// Gibt (erlaubt, Regelname, gefundener Match) zurueck.
func (e *Engine) CheckRequest(r *http.Request) (bool, string, string) {
	clientIP := extractClientIP(r)

	blocked, err := e.db.IsIPBlocked(clientIP)
	if err != nil {
		log.Printf("IP-Sperre pruefen fehlgeschlagen: %v", err)
	}
	if blocked {
		return false, "IP-Sperre", clientIP
	}

	if e.rateLimitMax > 0 {
		limited, err := e.db.CheckRateLimit(clientIP, e.rateLimitMax, e.rateLimitWindow)
		if err != nil {
			log.Printf("Rate-Limit pruefen fehlgeschlagen: %v", err)
		}
		if limited {
			return false, "Rate-Limit ueberschritten", clientIP
		}
	}

	e.mu.RLock()
	rules := e.rules
	e.mu.RUnlock()

	uri := r.URL.RequestURI()
	queryParams := r.URL.RawQuery
	headers := flattenHeaders(r.Header)

	var body string
	if r.Body != nil && r.ContentLength > 0 && r.ContentLength <= e.maxBodySize {
		bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, e.maxBodySize))
		if err == nil {
			body = string(bodyBytes)
			r.Body = io.NopCloser(strings.NewReader(body))
		}
	}

	for _, cr := range rules {
		if cr.rule.Action == "allow" {
			if matchTarget(cr, uri, queryParams, headers, body) {
				return true, cr.rule.Name, ""
			}
			continue
		}

		if match := matchTarget(cr, uri, queryParams, headers, body); match {
			matched := cr.regex.FindString(buildCheckString(cr.rule.Target, uri, queryParams, headers, body))

			blocked := &models.BlockedRequest{
				SourceIP:    clientIP,
				Method:      r.Method,
				Path:        r.URL.Path,
				RuleName:    cr.rule.Name,
				MatchedData: truncate(matched, 500),
				UserAgent:   r.UserAgent(),
				StatusCode:  403,
			}

			if err := e.db.LogBlockedRequest(blocked); err != nil {
				log.Printf("Log-Eintrag schreiben fehlgeschlagen: %v", err)
			}

			if e.onBlock != nil {
				e.onBlock(blocked)
			}

			return false, cr.rule.Name, matched
		}
	}

	return true, "", ""
}

func matchTarget(cr compiledRule, uri, params, headers, body string) bool {
	check := buildCheckString(cr.rule.Target, uri, params, headers, body)
	return cr.regex.MatchString(check)
}

func buildCheckString(target, uri, params, headers, body string) string {
	switch target {
	case "uri":
		return uri
	case "param":
		return params
	case "header":
		return headers
	case "body":
		return body
	default:
		return uri + "\n" + params + "\n" + headers + "\n" + body
	}
}

func flattenHeaders(h http.Header) string {
	var sb strings.Builder
	for key, values := range h {
		for _, v := range values {
			sb.WriteString(key)
			sb.WriteString(": ")
			sb.WriteString(v)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func extractClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return strings.Trim(ip, "[]")
}

func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
