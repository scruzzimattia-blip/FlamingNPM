package models

import "time"

// FirewallRule repraesentiert eine benutzerdefinierte WAF-Regel.
type FirewallRule struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Pattern     string    `json:"pattern"`
	Target      string    `json:"target"`       // "header", "body", "param", "uri", "all"
	Action      string    `json:"action"`       // "block", "allow", "sanitize"
	ScoreWeight int       `json:"score_weight"` // Gewicht fuer das Bedrohungs-Score (action block)
	Enabled     bool      `json:"enabled"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// BlockedRequest speichert ein Log eines blockierten Requests.
type BlockedRequest struct {
	ID          int64     `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	SourceIP    string    `json:"source_ip"`
	Method      string    `json:"method"`
	Path        string    `json:"path"`
	RuleName    string    `json:"rule_name"`
	MatchedData string    `json:"matched_data"`
	UserAgent   string    `json:"user_agent"`
	StatusCode  int       `json:"status_code"`
}

// IPBlock repraesentiert eine gesperrte IP-Adresse.
type IPBlock struct {
	ID        int64      `json:"id"`
	IP        string     `json:"ip"`
	Reason    string     `json:"reason"`
	ExpiresAt *time.Time `json:"expires_at"` // nil = permanente Sperre
	CreatedAt time.Time  `json:"created_at"`
}

// RateLimitEntry verwaltet temporaere Rate-Limit-Zaehler.
type RateLimitEntry struct {
	IP        string
	Count     int
	WindowEnd time.Time
}

// DashboardStats liefert Uebersichtszahlen fuers Dashboard.
type DashboardStats struct {
	TotalBlocked   int64 `json:"total_blocked"`
	BlockedToday   int64 `json:"blocked_today"`
	ActiveRules    int64 `json:"active_rules"`
	BlockedIPs     int64 `json:"blocked_ips"`
	RequestsPerMin int64 `json:"requests_per_min"`
}

// WSMessage ist das Format fuer WebSocket-Nachrichten an das Dashboard.
type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}
