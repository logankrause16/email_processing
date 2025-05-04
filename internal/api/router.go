package api

import (
	"log"
	"net/http"

	"github.com/logankrause16/email_processing/internal/service"
	"github.com/logankrause16/email_processing/pkg/metrics"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter sets up the HTTP routes for the API
// Defining the following routes (I thought the health and metrics would be useful :D ):
// - GET /healthz: Health check endpoint
// - GET /metrics: Metrics endpoint
// - GET /domains/{domainName}: Get domain status
// - GET /domains/stats: Get domain stats
// - PUT /events/{domainName}/delivered: Record delivered event
// - PUT /events/{domainName}/bounced: Record bounced event
func NewRouter(
	domainService *service.DomainService,
	statsService *service.StatsService,
	metricsCollector *metrics.MetricsCollector,
	logger *log.Logger,
) http.Handler {
	// We use chi router for its simplicity and performance
	// It is a lightweight and idiomatic router for Go - Kinda like FastAPI for Python (but not as fast as FastAPI, so don't get too excited)
	r := chi.NewRouter()

	// Middleware - these are functions that wrap around the request and response
	// They can be used for logging, authentication, metrics, etc. but this is a simple example
	// If we were to go into production, we would want to add authentication and authorization
	// but for now, we will just log the requests and responses
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// I'm particularly fond of the metrics middleware, this was a great idea from Claude that I shamelessly stole
	// and will be using it in all my projects from now on
	r.Use(metricsMiddleware(metricsCollector))

	// Handler initialization
	domainHandler := NewDomainHandler(domainService, logger)
	statsHandler := NewStatsHandler(statsService, logger)

	// Routes for health check and metrics
	r.Get("/healthz", healthCheckHandler)
	r.Get("/metrics", metricsHandler(metricsCollector))

	// Domain routes - these are the main routes for domain status and stats
	r.Route("/domains", func(r chi.Router) {
		r.Get("/{domainName}", domainHandler.GetDomainStatus)
		r.Get("/stats", statsHandler.GetDomainStats)
	})

	// Event routes - these are for recording events related to domains
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
// Lets write the metrics to the response writer
func metricsHandler(metrics *metrics.MetricsCollector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		metrics.WriteMetrics(w)
	}
}

// metricsMiddleware collects metrics for each request
// We want to track the number of requests, response times, and status codes
// We take the metrics collector as an argument and return a middleware function
// that takes an http.Handler and returns an http.Handler
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
