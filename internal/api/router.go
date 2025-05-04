package api

import (
	"log"
	"net/http"

	"email_processing/internal/service"
	"email_processing/pkg/metrics"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter sets up the HTTP routes for the API
func NewRouter(
	domainService *service.DomainService,
	statsService *service.StatsService,
	metricsCollector *metrics.MetricsCollector,
	logger *log.Logger,
) http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(metricsMiddleware(metricsCollector))

	// Handler initialization
	domainHandler := NewDomainHandler(domainService, logger)
	statsHandler := NewStatsHandler(statsService, logger)

	// Routes
	r.Get("/healthz", healthCheckHandler)
	r.Get("/metrics", metricsHandler(metricsCollector))

	// Domain routes
	r.Route("/domains", func(r chi.Router) {
		r.Get("/{domainName}", domainHandler.GetDomainStatus)
		r.Get("/stats", statsHandler.GetDomainStats)
	})

	// Event routes
	r.Route("/events", func(r chi.Router) {
		r.Put("/{domainName}/delivered", domainHandler.RecordDeliveredEvent)
		r.Put("/{domainName}/bounced", domainHandler.RecordBouncedEvent)
	})

	return r
}

// healthCheckHandler provides a simple health check endpoint
func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// metricsHandler returns the current metrics
func metricsHandler(metrics *metrics.MetricsCollector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		metrics.WriteMetrics(w)
	}
}

// metricsMiddleware collects metrics for each request
func metricsMiddleware(metrics *metrics.MetricsCollector) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			metrics.IncrementRequestCount()

			// Wrap the response writer to capture status code
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			// Call the next handler
			next.ServeHTTP(ww, r)

			// Record response metrics
			metrics.RecordResponseTime(r.URL.Path, ww.Status(), r.Method)
		})
	}
}
