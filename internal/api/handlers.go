package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"github.com/flamingnpm/waf/internal/database"
	"github.com/flamingnpm/waf/internal/models"
	"github.com/flamingnpm/waf/internal/waf"
)

// Handler buendelt alle REST-API-Endpunkte fuer das Dashboard.
type Handler struct {
	db     *database.DB
	engine *waf.Engine
	hub    *Hub
}

func NewHandler(db *database.DB, engine *waf.Engine, hub *Hub) *Handler {
	return &Handler{db: db, engine: engine, hub: hub}
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
