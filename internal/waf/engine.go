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
	db              *database.DB
	allowRules      []compiledRule
	sanitizeRules   []compiledRule
	blockRules      []compiledRule
	mu              sync.RWMutex
	maxBodySize     int64
	rateLimitMax    int
	rateLimitWindow int
	scoreThreshold  int
	onBlock         func(*models.BlockedRequest)
}

type Config struct {
	MaxBodySize     int64
	RateLimitMax    int
	RateLimitWindow int
	ScoreThreshold  int // Summe der Regel-Gewichte ab der blockiert wird
}

func NewEngine(db *database.DB, cfg Config) (*Engine, error) {
	th := cfg.ScoreThreshold
	if th <= 0 {
		th = 50
	}
	e := &Engine{
		db:              db,
		maxBodySize:     cfg.MaxBodySize,
		rateLimitMax:    cfg.RateLimitMax,
		rateLimitWindow: cfg.RateLimitWindow,
		scoreThreshold:  th,
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

// ScoreThreshold liefert die konfigurierte Schwelle (fuer Tests und Observability).
func (e *Engine) ScoreThreshold() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.scoreThreshold
}

// ReloadRules laedt alle aktiven Regeln aus der Datenbank und compiliert die Regex-Patterns.
func (e *Engine) ReloadRules() error {
	dbRules, err := e.db.GetEnabledRules()
	if err != nil {
		return fmt.Errorf("regeln laden fehlgeschlagen: %w", err)
	}

	var allow, sanitize, block []compiledRule
	for _, r := range dbRules {
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			log.Printf("WARNUNG: Regel '%s' hat ungueltiges Pattern '%s': %v", r.Name, r.Pattern, err)
			continue
		}
		cr := compiledRule{rule: r, regex: re}
		switch r.Action {
		case "allow":
			allow = append(allow, cr)
		case "sanitize":
			sanitize = append(sanitize, cr)
		default:
			block = append(block, cr)
		}
	}

	e.mu.Lock()
	e.allowRules = allow
	e.sanitizeRules = sanitize
	e.blockRules = block
	e.mu.Unlock()

	log.Printf("WAF-Engine: %d Allow-, %d Sanitize-, %d Block-Regeln geladen (Schwelle: %d)",
		len(allow), len(sanitize), len(block), e.scoreThreshold)
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
	allowRules := e.allowRules
	sanitizeRules := e.sanitizeRules
	blockRules := e.blockRules
	threshold := e.scoreThreshold
	e.mu.RUnlock()

	uri := r.URL.RequestURI()
	queryParams := r.URL.RawQuery
	headers := flattenHeaders(r.Header)

	var body string
	bodyRead := false
	if r.Body != nil && r.ContentLength > 0 && r.ContentLength <= e.maxBodySize {
		bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, e.maxBodySize))
		if err == nil {
			body = string(bodyBytes)
			bodyRead = true
			r.Body = io.NopCloser(strings.NewReader(body))
		}
	}

	for _, cr := range allowRules {
		if matchTarget(cr, uri, queryParams, headers, body) {
			return true, cr.rule.Name, ""
		}
	}

	for _, cr := range sanitizeRules {
		applySanitize(cr, r, &body, &uri, &queryParams, &headers)
	}

	if bodyRead {
		r.Body = io.NopCloser(strings.NewReader(body))
	}

	uri = r.URL.RequestURI()
	queryParams = r.URL.RawQuery
	headers = flattenHeaders(r.Header)

	score, ruleNames, matchedSample := accumulateBlockScore(blockRules, uri, queryParams, headers, body)
	if score >= threshold {
		detail := fmt.Sprintf("score=%d Schwelle=%d Treffer: %s", score, threshold, strings.Join(ruleNames, ", "))
		if len(matchedSample) > 200 {
			matchedSample = matchedSample[:200] + "..."
		}

		blockedReq := &models.BlockedRequest{
			SourceIP:    clientIP,
			Method:      r.Method,
			Path:        r.URL.Path,
			RuleName:    "WAF-Threat-Score",
			MatchedData: detail,
			UserAgent:   r.UserAgent(),
			StatusCode:  403,
		}

		if err := e.db.LogBlockedRequest(blockedReq); err != nil {
			log.Printf("Log-Eintrag schreiben fehlgeschlagen: %v", err)
		}
		if e.onBlock != nil {
			e.onBlock(blockedReq)
		}
		return false, "WAF-Threat-Score", matchedSample
	}

	return true, "", ""
}

func accumulateBlockScore(blockRules []compiledRule, uri, params, headers, body string) (score int, names []string, firstMatch string) {
	for _, cr := range blockRules {
		if !matchTarget(cr, uri, params, headers, body) {
			continue
		}
		w := cr.rule.ScoreWeight
		if w <= 0 {
			w = 10
		}
		score += w
		names = append(names, cr.rule.Name)
		if firstMatch == "" {
			firstMatch = cr.regex.FindString(buildCheckString(cr.rule.Target, uri, params, headers, body))
		}
	}
	return score, names, firstMatch
}

func applySanitize(cr compiledRule, r *http.Request, body, uri, queryParams, headers *string) {
	switch cr.rule.Target {
	case "param":
		q := r.URL.RawQuery
		if cr.regex.MatchString(q) {
			r.URL.RawQuery = cr.regex.ReplaceAllString(q, "")
			*queryParams = r.URL.RawQuery
		}
	case "uri":
		p := r.URL.Path
		if cr.regex.MatchString(p) {
			r.URL.Path = cr.regex.ReplaceAllString(p, "")
		}
		q := r.URL.RawQuery
		if cr.regex.MatchString(q) {
			r.URL.RawQuery = cr.regex.ReplaceAllString(q, "")
		}
		*uri = r.URL.RequestURI()
		*queryParams = r.URL.RawQuery
	case "body":
		if body != nil && *body != "" && cr.regex.MatchString(*body) {
			*body = cr.regex.ReplaceAllString(*body, "")
		}
	case "header":
		clone := r.Header.Clone()
		for key, vals := range clone {
			for _, v := range vals {
				if cr.regex.MatchString(key + ": " + v) {
					r.Header.Set(key, cr.regex.ReplaceAllString(v, ""))
				}
			}
		}
		*headers = flattenHeaders(r.Header)
	case "all":
		p := r.URL.Path
		if cr.regex.MatchString(p) {
			r.URL.Path = cr.regex.ReplaceAllString(p, "")
		}
		q := r.URL.RawQuery
		if cr.regex.MatchString(q) {
			r.URL.RawQuery = cr.regex.ReplaceAllString(q, "")
		}
		if body != nil && *body != "" && cr.regex.MatchString(*body) {
			*body = cr.regex.ReplaceAllString(*body, "")
		}
		hclone := r.Header.Clone()
		for key, vals := range hclone {
			for _, v := range vals {
				if cr.regex.MatchString(key + ": " + v) {
					r.Header.Set(key, cr.regex.ReplaceAllString(v, ""))
				}
			}
		}
		*uri = r.URL.RequestURI()
		*queryParams = r.URL.RawQuery
		*headers = flattenHeaders(r.Header)
	default:
		p := r.URL.Path
		if cr.regex.MatchString(p) {
			r.URL.Path = cr.regex.ReplaceAllString(p, "")
		}
		q := r.URL.RawQuery
		if cr.regex.MatchString(q) {
			r.URL.RawQuery = cr.regex.ReplaceAllString(q, "")
		}
		if body != nil && *body != "" && cr.regex.MatchString(*body) {
			*body = cr.regex.ReplaceAllString(*body, "")
		}
		*uri = r.URL.RequestURI()
		*queryParams = r.URL.RawQuery
	}
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
