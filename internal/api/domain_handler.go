package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/logankrause16/email_processing/internal/domain"
	"github.com/logankrause16/email_processing/internal/service"
)

// DomainHandler handles HTTP requests related to domains
type DomainHandler struct {
	domainService *service.DomainService
	logger        *log.Logger
}

// NewDomainHandler creates a new domain handler
func NewDomainHandler(domainService *service.DomainService, logger *log.Logger) *DomainHandler {
	return &DomainHandler{
		domainService: domainService,
		logger:        logger,
	}
}

// StatsHandler handles HTTP requests related to domain statistics
type StatsHandler struct {
	statsService *service.StatsService
	logger       *log.Logger
}

// NewStatsHandler creates a new stats handler
func NewStatsHandler(statsService *service.StatsService, logger *log.Logger) *StatsHandler {
	return &StatsHandler{
		statsService: statsService,
		logger:       logger,
	}
}

// GetDomainStats handles requests to get domain statistics
func (h *StatsHandler) GetDomainStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.statsService.GetDomainStats(r.Context())
	if err != nil {
		h.logger.Printf("Error getting domain stats: %v", err)
		http.Error(w, "Failed to get domain stats", http.StatusInternalServerError)
		return
	}

	respondJSON(w, stats)
}

// GetDomainStatus handles requests to check a domain's status
func (h *DomainHandler) GetDomainStatus(w http.ResponseWriter, r *http.Request) {
	domainName := chi.URLParam(r, "domainName")
	if domainName == "" {
		http.Error(w, "Domain name is required", http.StatusBadRequest)
		return
	}

	statusResponse, err := h.domainService.GetDomainStatus(r.Context(), domainName)
	if err != nil {
		h.logger.Printf("Error getting domain status for %s: %v", domainName, err)
		http.Error(w, "Failed to get domain status", http.StatusInternalServerError)
		return
	}

	respondJSON(w, statusResponse)
}

// RecordDeliveredEvent handles requests to record a delivered event
func (h *DomainHandler) RecordDeliveredEvent(w http.ResponseWriter, r *http.Request) {
	domainName := chi.URLParam(r, "domainName")
	if domainName == "" {
		http.Error(w, "Domain name is required", http.StatusBadRequest)
		return
	}

	err := h.domainService.RecordEvent(r.Context(), domainName, domain.EventDelivered)
	if err != nil {
		h.logger.Printf("Error recording delivered event for %s: %v", domainName, err)
		http.Error(w, "Failed to record event", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// RecordBouncedEvent handles requests to record a bounced event
func (h *DomainHandler) RecordBouncedEvent(w http.ResponseWriter, r *http.Request) {
	domainName := chi.URLParam(r, "domainName")
	if domainName == "" {
		http.Error(w, "Domain name is required", http.StatusBadRequest)
		return
	}

	err := h.domainService.RecordEvent(r.Context(), domainName, domain.EventBounced)
	if err != nil {
		h.logger.Printf("Error recording bounced event for %s: %v", domainName, err)
		http.Error(w, "Failed to record event", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// respondJSON sends a JSON response
func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
