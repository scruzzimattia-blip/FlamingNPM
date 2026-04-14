package database

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/flamingnpm/waf/internal/models"
)

// DB kapselt die SQLite-Verbindung und bietet threadsichere Zugriffsmethoden.
type DB struct {
	conn *sql.DB
	mu   sync.RWMutex
}

func New(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("datenbank oeffnen fehlgeschlagen: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("migration fehlgeschlagen: %w", err)
	}
	if err := db.ensureRuleScoreColumn(); err != nil {
		return nil, fmt.Errorf("schema-aktualisierung fehlgeschlagen: %w", err)
	}
	if err := db.seedDefaultRules(); err != nil {
		return nil, fmt.Errorf("standard-regeln einfuegen fehlgeschlagen: %w", err)
	}
	return db, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS firewall_rules (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		pattern TEXT NOT NULL,
		target TEXT NOT NULL DEFAULT 'all',
		action TEXT NOT NULL DEFAULT 'block',
		enabled BOOLEAN NOT NULL DEFAULT 1,
		description TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS blocked_requests (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		source_ip TEXT NOT NULL,
		method TEXT NOT NULL,
		path TEXT NOT NULL,
		rule_name TEXT NOT NULL,
		matched_data TEXT DEFAULT '',
		user_agent TEXT DEFAULT '',
		status_code INTEGER DEFAULT 403
	);

	CREATE TABLE IF NOT EXISTS ip_blocks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		ip TEXT NOT NULL UNIQUE,
		reason TEXT DEFAULT '',
		expires_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS rate_limits (
		ip TEXT PRIMARY KEY,
		count INTEGER NOT NULL DEFAULT 1,
		window_end DATETIME NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_blocked_requests_timestamp ON blocked_requests(timestamp);
	CREATE INDEX IF NOT EXISTS idx_blocked_requests_source_ip ON blocked_requests(source_ip);
	CREATE INDEX IF NOT EXISTS idx_ip_blocks_ip ON ip_blocks(ip);
	`
	_, err := db.conn.Exec(schema)
	return err
}

func (db *DB) ensureRuleScoreColumn() error {
	var n int
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('firewall_rules') WHERE name='score_weight'`).Scan(&n)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	_, err = db.conn.Exec(`ALTER TABLE firewall_rules ADD COLUMN score_weight INTEGER NOT NULL DEFAULT 10`)
	return err
}

func (db *DB) seedDefaultRules() error {
	var count int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM firewall_rules").Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	defaults := []models.FirewallRule{
		{
			Name:        "SQL Injection - Union Based",
			Pattern:     `(?i)(union\s+(all\s+)?select|select\s+.*\s+from|insert\s+into|delete\s+from|drop\s+table|alter\s+table)`,
			Target:      "all",
			Action:      "block",
			ScoreWeight: 25,
			Enabled:     true,
			Description: "Erkennt Union-basierte SQL-Injection-Angriffe",
		},
		{
			Name:        "SQL Injection - Boolean Based",
			Pattern:     `(?i)(\bor\b\s+\d+\s*=\s*\d+|\band\b\s+\d+\s*=\s*\d+|'\s*(or|and)\s+'[^']*'\s*=\s*'[^']*')`,
			Target:      "all",
			Action:      "block",
			ScoreWeight: 25,
			Enabled:     true,
			Description: "Erkennt Boolean-basierte SQL-Injection-Versuche",
		},
		{
			Name:        "SQL Injection - Comment/Stacked",
			Pattern:     `(?i)('\s*--|'\s*#|\bexec\s*\(|;\s*(drop|alter|create|truncate|exec)\b|/\*![\s\S]*?\*/)`,
			Target:      "param",
			Action:      "block",
			ScoreWeight: 25,
			Enabled:     true,
			Description: "Erkennt SQL-Kommentar- und Stacked-Query-Angriffe in Parametern",
		},
		{
			Name:        "XSS - Script Tags",
			Pattern:     `(?i)(<script[^>]*>|</script>|javascript\s*:|on(load|error|click|mouseover|submit|focus|blur)\s*=)`,
			Target:      "all",
			Action:      "block",
			ScoreWeight: 25,
			Enabled:     true,
			Description: "Erkennt Cross-Site-Scripting ueber Script-Tags und Event-Handler",
		},
		{
			Name:        "XSS - Data URIs und Encoding",
			Pattern:     `(?i)(data\s*:\s*text/html|&#x?[0-9a-f]+;|%3[Cc]script|<\s*img[^>]+onerror)`,
			Target:      "all",
			Action:      "block",
			ScoreWeight: 25,
			Enabled:     true,
			Description: "Erkennt XSS ueber Data-URIs und HTML-Encoding",
		},
		{
			Name:        "Path Traversal",
			Pattern:     `(?i)(\.\./|\.\.\\|%2[Ee]%2[Ee]|%252[Ee]|/etc/passwd|/etc/shadow|/proc/self)`,
			Target:      "all",
			Action:      "block",
			ScoreWeight: 30,
			Enabled:     true,
			Description: "Erkennt Verzeichnistraversierungs-Angriffe (Directory Traversal)",
		},
		{
			Name:        "Command Injection",
			Pattern:     `(?i)(\||;|\$\(|` + "`" + `|&&|\|\|)\s*(cat|ls|whoami|id|uname|curl|wget|nc|bash|sh|python|perl|ruby)`,
			Target:      "all",
			Action:      "block",
			ScoreWeight: 35,
			Enabled:     true,
			Description: "Erkennt Betriebssystem-Command-Injection-Versuche",
		},
		{
			Name:        "Log4Shell / JNDI",
			Pattern:     `(?i)\$\{(jndi|lower|upper|env|sys|java):`,
			Target:      "all",
			Action:      "block",
			ScoreWeight: 40,
			Enabled:     true,
			Description: "Erkennt Log4Shell (CVE-2021-44228) JNDI-Injection-Versuche",
		},
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO firewall_rules (name, pattern, target, action, enabled, description, score_weight) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, r := range defaults {
		sw := r.ScoreWeight
		if sw <= 0 {
			sw = 10
		}
		_, err := stmt.Exec(r.Name, r.Pattern, r.Target, r.Action, r.Enabled, r.Description, sw)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// --- Firewall-Regeln ---

func (db *DB) GetRules() ([]models.FirewallRule, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	rows, err := db.conn.Query("SELECT id, name, pattern, target, action, enabled, description, score_weight, created_at, updated_at FROM firewall_rules ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []models.FirewallRule
	for rows.Next() {
		var r models.FirewallRule
		if err := rows.Scan(&r.ID, &r.Name, &r.Pattern, &r.Target, &r.Action, &r.Enabled, &r.Description, &r.ScoreWeight, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, nil
}

func (db *DB) GetEnabledRules() ([]models.FirewallRule, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	rows, err := db.conn.Query("SELECT id, name, pattern, target, action, enabled, description, score_weight, created_at, updated_at FROM firewall_rules WHERE enabled = 1 ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []models.FirewallRule
	for rows.Next() {
		var r models.FirewallRule
		if err := rows.Scan(&r.ID, &r.Name, &r.Pattern, &r.Target, &r.Action, &r.Enabled, &r.Description, &r.ScoreWeight, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, nil
}

func (db *DB) CreateRule(rule *models.FirewallRule) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	sw := rule.ScoreWeight
	if sw <= 0 {
		sw = 10
	}
	result, err := db.conn.Exec(
		"INSERT INTO firewall_rules (name, pattern, target, action, enabled, description, score_weight) VALUES (?, ?, ?, ?, ?, ?, ?)",
		rule.Name, rule.Pattern, rule.Target, rule.Action, rule.Enabled, rule.Description, sw,
	)
	if err != nil {
		return err
	}
	rule.ID, _ = result.LastInsertId()
	return nil
}

func (db *DB) UpdateRule(rule *models.FirewallRule) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	sw := rule.ScoreWeight
	if sw <= 0 {
		sw = 10
	}
	_, err := db.conn.Exec(
		"UPDATE firewall_rules SET name=?, pattern=?, target=?, action=?, enabled=?, description=?, score_weight=?, updated_at=CURRENT_TIMESTAMP WHERE id=?",
		rule.Name, rule.Pattern, rule.Target, rule.Action, rule.Enabled, rule.Description, sw, rule.ID,
	)
	return err
}

func (db *DB) DeleteRule(id int64) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	_, err := db.conn.Exec("DELETE FROM firewall_rules WHERE id=?", id)
	return err
}

// --- Blockierte Requests (Logs) ---

func (db *DB) LogBlockedRequest(req *models.BlockedRequest) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	result, err := db.conn.Exec(
		"INSERT INTO blocked_requests (source_ip, method, path, rule_name, matched_data, user_agent, status_code) VALUES (?, ?, ?, ?, ?, ?, ?)",
		req.SourceIP, req.Method, req.Path, req.RuleName, req.MatchedData, req.UserAgent, req.StatusCode,
	)
	if err != nil {
		return err
	}
	req.ID, _ = result.LastInsertId()
	return nil
}

func (db *DB) GetBlockedRequests(limit, offset int) ([]models.BlockedRequest, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	rows, err := db.conn.Query(
		"SELECT id, timestamp, source_ip, method, path, rule_name, matched_data, user_agent, status_code FROM blocked_requests ORDER BY timestamp DESC LIMIT ? OFFSET ?",
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var requests []models.BlockedRequest
	for rows.Next() {
		var r models.BlockedRequest
		if err := rows.Scan(&r.ID, &r.Timestamp, &r.SourceIP, &r.Method, &r.Path, &r.RuleName, &r.MatchedData, &r.UserAgent, &r.StatusCode); err != nil {
			return nil, err
		}
		requests = append(requests, r)
	}
	return requests, nil
}

// --- IP-Sperren ---

func (db *DB) GetBlockedIPs() ([]models.IPBlock, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	rows, err := db.conn.Query("SELECT id, ip, reason, expires_at, created_at FROM ip_blocks ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var blocks []models.IPBlock
	for rows.Next() {
		var b models.IPBlock
		if err := rows.Scan(&b.ID, &b.IP, &b.Reason, &b.ExpiresAt, &b.CreatedAt); err != nil {
			return nil, err
		}
		blocks = append(blocks, b)
	}
	return blocks, nil
}

func (db *DB) BlockIP(block *models.IPBlock) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	result, err := db.conn.Exec(
		"INSERT OR REPLACE INTO ip_blocks (ip, reason, expires_at) VALUES (?, ?, ?)",
		block.IP, block.Reason, block.ExpiresAt,
	)
	if err != nil {
		return err
	}
	block.ID, _ = result.LastInsertId()
	return nil
}

func (db *DB) UnblockIP(id int64) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	_, err := db.conn.Exec("DELETE FROM ip_blocks WHERE id=?", id)
	return err
}

func (db *DB) IsIPBlocked(ip string) (bool, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var count int
	err := db.conn.QueryRow(
		"SELECT COUNT(*) FROM ip_blocks WHERE ip=? AND (expires_at IS NULL OR expires_at > datetime('now'))",
		ip,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (db *DB) CleanExpiredBlocks() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	_, err := db.conn.Exec("DELETE FROM ip_blocks WHERE expires_at IS NOT NULL AND expires_at <= datetime('now')")
	return err
}

// --- Rate-Limiting ---

func (db *DB) CheckRateLimit(ip string, maxRequests int, windowSeconds int) (bool, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	now := time.Now()

	var count int
	var windowEnd time.Time
	err := db.conn.QueryRow("SELECT count, window_end FROM rate_limits WHERE ip=?", ip).Scan(&count, &windowEnd)

	if err == sql.ErrNoRows {
		newEnd := now.Add(time.Duration(windowSeconds) * time.Second)
		_, err = db.conn.Exec("INSERT INTO rate_limits (ip, count, window_end) VALUES (?, 1, ?)", ip, newEnd)
		return false, err
	}
	if err != nil {
		return false, err
	}

	if now.After(windowEnd) {
		newEnd := now.Add(time.Duration(windowSeconds) * time.Second)
		_, err = db.conn.Exec("UPDATE rate_limits SET count=1, window_end=? WHERE ip=?", newEnd, ip)
		return false, err
	}

	count++
	_, err = db.conn.Exec("UPDATE rate_limits SET count=? WHERE ip=?", count, ip)
	if err != nil {
		return false, err
	}

	return count > maxRequests, nil
}

// --- Statistiken ---

func (db *DB) GetStats() (*models.DashboardStats, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	stats := &models.DashboardStats{}

	db.conn.QueryRow("SELECT COUNT(*) FROM blocked_requests").Scan(&stats.TotalBlocked)
	db.conn.QueryRow("SELECT COUNT(*) FROM blocked_requests WHERE timestamp >= date('now')").Scan(&stats.BlockedToday)
	db.conn.QueryRow("SELECT COUNT(*) FROM firewall_rules WHERE enabled=1").Scan(&stats.ActiveRules)
	db.conn.QueryRow("SELECT COUNT(*) FROM ip_blocks WHERE expires_at IS NULL OR expires_at > datetime('now')").Scan(&stats.BlockedIPs)
	db.conn.QueryRow("SELECT COUNT(*) FROM blocked_requests WHERE timestamp >= datetime('now', '-1 minute')").Scan(&stats.RequestsPerMin)

	if err := db.conn.QueryRow("SELECT COUNT(*) FROM blocked_requests").Scan(&stats.TotalBlocked); err != nil {
		log.Printf("Statistik-Abfrage fehlgeschlagen: %v", err)
	}

	return stats, nil
}
