package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gorilla/mux"

	"github.com/flamingnpm/waf/internal/database"
	"github.com/flamingnpm/waf/internal/models"
	"github.com/flamingnpm/waf/internal/proxy"
	"github.com/flamingnpm/waf/internal/waf"
)

// Handler buendelt alle REST-API-Endpunkte fuer das Dashboard.
type Handler struct {
	db      *database.DB
	engine  *waf.Engine
	hub     *Hub
	dynamic *proxy.DynamicRouter
}

func NewHandler(db *database.DB, engine *waf.Engine, hub *Hub, dynamic *proxy.DynamicRouter) *Handler {
	return &Handler{db: db, engine: engine, hub: hub, dynamic: dynamic}
}

func (h *Handler) RegisterRoutes(r *mux.Router) {
	api := r.PathPrefix("/api").Subrouter()

	api.HandleFunc("/stats", h.getStats).Methods("GET")

	api.HandleFunc("/rules", h.getRules).Methods("GET")
	api.HandleFunc("/rules", h.createRule).Methods("POST")
	api.HandleFunc("/rules/{id}", h.updateRule).Methods("PUT")
	api.HandleFunc("/rules/{id}", h.deleteRule).Methods("DELETE")
	api.HandleFunc("/rules/reload", h.reloadRules).Methods("POST")

	api.HandleFunc("/logs", h.getLogs).Methods("GET")

	api.HandleFunc("/ip-blocks", h.getBlockedIPs).Methods("GET")
	api.HandleFunc("/ip-blocks", h.blockIP).Methods("POST")
	api.HandleFunc("/ip-blocks/{id}", h.unblockIP).Methods("DELETE")

	api.HandleFunc("/proxy-routes", h.getProxyRoutes).Methods("GET")
	api.HandleFunc("/proxy-routes", h.createProxyRoute).Methods("POST")
	api.HandleFunc("/proxy-routes/{id}", h.updateProxyRoute).Methods("PUT")
	api.HandleFunc("/proxy-routes/{id}", h.deleteProxyRoute).Methods("DELETE")

	r.HandleFunc("/api/ws", h.hub.HandleWS)
}

func (h *Handler) getStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.db.GetStats()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Statistiken laden fehlgeschlagen")
		return
	}
	respondJSON(w, http.StatusOK, stats)
}

func (h *Handler) getRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.db.GetRules()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Regeln laden fehlgeschlagen")
		return
	}
	if rules == nil {
		rules = []models.FirewallRule{}
	}
	respondJSON(w, http.StatusOK, rules)
}

func (h *Handler) createRule(w http.ResponseWriter, r *http.Request) {
	var rule models.FirewallRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		respondError(w, http.StatusBadRequest, "Ungueltiges JSON-Format")
		return
	}

	if rule.Name == "" || rule.Pattern == "" {
		respondError(w, http.StatusBadRequest, "Name und Pattern sind Pflichtfelder")
		return
	}

	if rule.Action == "" {
		rule.Action = "block"
	}

	if !validRuleAction(rule.Action) {
		respondError(w, http.StatusBadRequest, "Aktion muss block, allow oder sanitize sein")
		return
	}

	if err := h.db.CreateRule(&rule); err != nil {
		respondError(w, http.StatusInternalServerError, "Regel erstellen fehlgeschlagen")
		return
	}

	if err := h.engine.ReloadRules(); err != nil {
		log.Printf("Regeln neuladen fehlgeschlagen: %v", err)
	}

	h.hub.Broadcast(models.WSMessage{Type: "rule_created", Data: rule})
	respondJSON(w, http.StatusCreated, rule)
}

func (h *Handler) updateRule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Ungueltige ID")
		return
	}

	var rule models.FirewallRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		respondError(w, http.StatusBadRequest, "Ungueltiges JSON-Format")
		return
	}
	rule.ID = id

	if !validRuleAction(rule.Action) {
		respondError(w, http.StatusBadRequest, "Aktion muss block, allow oder sanitize sein")
		return
	}

	if err := h.db.UpdateRule(&rule); err != nil {
		respondError(w, http.StatusInternalServerError, "Regel aktualisieren fehlgeschlagen")
		return
	}

	if err := h.engine.ReloadRules(); err != nil {
		log.Printf("Regeln neuladen fehlgeschlagen: %v", err)
	}

	h.hub.Broadcast(models.WSMessage{Type: "rule_updated", Data: rule})
	respondJSON(w, http.StatusOK, rule)
}

func (h *Handler) deleteRule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Ungueltige ID")
		return
	}

	if err := h.db.DeleteRule(id); err != nil {
		respondError(w, http.StatusInternalServerError, "Regel loeschen fehlgeschlagen")
		return
	}

	if err := h.engine.ReloadRules(); err != nil {
		log.Printf("Regeln neuladen fehlgeschlagen: %v", err)
	}

	h.hub.Broadcast(models.WSMessage{Type: "rule_deleted", Data: map[string]int64{"id": id}})
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) reloadRules(w http.ResponseWriter, r *http.Request) {
	if err := h.engine.ReloadRules(); err != nil {
		respondError(w, http.StatusInternalServerError, "Regeln neuladen fehlgeschlagen")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) getLogs(w http.ResponseWriter, r *http.Request) {
	limit := 100
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 1000 {
			limit = v
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	logs, err := h.db.GetBlockedRequests(limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Logs laden fehlgeschlagen")
		return
	}
	if logs == nil {
		logs = []models.BlockedRequest{}
	}
	respondJSON(w, http.StatusOK, logs)
}

func (h *Handler) getBlockedIPs(w http.ResponseWriter, r *http.Request) {
	ips, err := h.db.GetBlockedIPs()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "IP-Sperren laden fehlgeschlagen")
		return
	}
	if ips == nil {
		ips = []models.IPBlock{}
	}
	respondJSON(w, http.StatusOK, ips)
}

func (h *Handler) blockIP(w http.ResponseWriter, r *http.Request) {
	var block models.IPBlock
	if err := json.NewDecoder(r.Body).Decode(&block); err != nil {
		respondError(w, http.StatusBadRequest, "Ungueltiges JSON-Format")
		return
	}

	if block.IP == "" {
		respondError(w, http.StatusBadRequest, "IP-Adresse ist ein Pflichtfeld")
		return
	}

	if err := h.db.BlockIP(&block); err != nil {
		respondError(w, http.StatusInternalServerError, "IP sperren fehlgeschlagen")
		return
	}

	h.hub.Broadcast(models.WSMessage{Type: "ip_blocked", Data: block})
	respondJSON(w, http.StatusCreated, block)
}

func (h *Handler) unblockIP(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Ungueltige ID")
		return
	}

	if err := h.db.UnblockIP(id); err != nil {
		respondError(w, http.StatusInternalServerError, "IP-Sperre aufheben fehlgeschlagen")
		return
	}

	h.hub.Broadcast(models.WSMessage{Type: "ip_unblocked", Data: map[string]int64{"id": id}})
	w.WriteHeader(http.StatusNoContent)
}

// --- Hilfsfunktionen ---

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

func validRuleAction(action string) bool {
	switch action {
	case "block", "allow", "sanitize":
		return true
	default:
		return false
	}
}

func (h *Handler) getProxyRoutes(w http.ResponseWriter, r *http.Request) {
	routes, err := h.db.GetProxyRoutes()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Proxy-Routen laden fehlgeschlagen")
		return
	}
	if routes == nil {
		routes = []models.ProxyRoute{}
	}
	respondJSON(w, http.StatusOK, routes)
}

func (h *Handler) createProxyRoute(w http.ResponseWriter, r *http.Request) {
	var rt models.ProxyRoute
	if err := json.NewDecoder(r.Body).Decode(&rt); err != nil {
		respondError(w, http.StatusBadRequest, "Ungueltiges JSON-Format")
		return
	}
	rt.Host = normalizeRouteHost(rt.Host)
	if rt.Host == "" {
		respondError(w, http.StatusBadRequest, "Host ist ein Pflichtfeld")
		return
	}
	if err := validateBackendURL(rt.BackendURL); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.db.CreateProxyRoute(&rt); err != nil {
		respondError(w, http.StatusInternalServerError, "Proxy-Route speichern fehlgeschlagen (Host evtl. schon vergeben)")
		return
	}
	if err := h.dynamic.ReloadRoutes(); err != nil {
		log.Printf("Routen neu laden fehlgeschlagen: %v", err)
	}
	h.hub.Broadcast(models.WSMessage{Type: "proxy_route_created", Data: rt})
	respondJSON(w, http.StatusCreated, rt)
}

func (h *Handler) updateProxyRoute(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Ungueltige ID")
		return
	}
	var rt models.ProxyRoute
	if err := json.NewDecoder(r.Body).Decode(&rt); err != nil {
		respondError(w, http.StatusBadRequest, "Ungueltiges JSON-Format")
		return
	}
	rt.ID = id
	rt.Host = normalizeRouteHost(rt.Host)
	if rt.Host == "" {
		respondError(w, http.StatusBadRequest, "Host ist ein Pflichtfeld")
		return
	}
	if err := validateBackendURL(rt.BackendURL); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.db.UpdateProxyRoute(&rt); err != nil {
		respondError(w, http.StatusInternalServerError, "Proxy-Route aktualisieren fehlgeschlagen")
		return
	}
	if err := h.dynamic.ReloadRoutes(); err != nil {
		log.Printf("Routen neu laden fehlgeschlagen: %v", err)
	}
	h.hub.Broadcast(models.WSMessage{Type: "proxy_route_updated", Data: rt})
	respondJSON(w, http.StatusOK, rt)
}

func (h *Handler) deleteProxyRoute(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Ungueltige ID")
		return
	}
	if err := h.db.DeleteProxyRoute(id); err != nil {
		respondError(w, http.StatusInternalServerError, "Proxy-Route loeschen fehlgeschlagen")
		return
	}
	if err := h.dynamic.ReloadRoutes(); err != nil {
		log.Printf("Routen neu laden fehlgeschlagen: %v", err)
	}
	h.hub.Broadcast(models.WSMessage{Type: "proxy_route_deleted", Data: map[string]int64{"id": id}})
	w.WriteHeader(http.StatusNoContent)
}

func normalizeRouteHost(host string) string {
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

func validateBackendURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("backend_url muss eine gueltige URL sein")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("backend_url braucht Schema http oder https")
	}
	if u.Host == "" {
		return fmt.Errorf("backend_url braucht einen Host")
	}
	return nil
}
