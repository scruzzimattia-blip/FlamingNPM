package proxy

import "strings"

// NormalizePathPrefix bereitet einen konfigurierten Pfad-Prefix auf (fuehrendes /, kein trailing /).
func NormalizePathPrefix(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return strings.TrimSuffix(p, "/")
}

// StripPathPrefixIfMatches entfernt prefix nur bei echter Pfad-Segment-Grenze (/api/foo, nicht /api1).
func StripPathPrefixIfMatches(path, prefix string) string {
	prefix = NormalizePathPrefix(prefix)
	if prefix == "" {
		return path
	}
	if path == prefix {
		return "/"
	}
	if !strings.HasPrefix(path, prefix) {
		return path
	}
	if len(path) > len(prefix) && path[len(prefix)] != '/' {
		return path
	}
	rest := path[len(prefix):]
	if rest == "" {
		return "/"
	}
	if !strings.HasPrefix(rest, "/") {
		return "/" + rest
	}
	return rest
}
