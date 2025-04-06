package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/logankrause16/email_processing/internal/api"
	"github.com/logankrause16/email_processing/internal/config"
	"github.com/logankrause16/email_processing/internal/eventprocessor"
	"github.com/logankrause16/email_processing/internal/repository"
	"github.com/logankrause16/email_processing/internal/service"
	"github.com/logankrause16/email_processing/pkg/metrics"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "", "Path to configuration file")
	workers := flag.Int("workers", runtime.NumCPU(), "Number of event processing workers")
	enableProcessor := flag.Bool("processor", false, "Enable event processor")
	useMongoDb := flag.Bool("mongodb", false, "Use MongoDB instead of in-memory storage")
	enableCache := flag.Bool("cache", true, "Enable repository caching")
	cacheTTL := flag.Duration("cache-ttl", 5*time.Minute, "Cache TTL duration")
	flag.Parse()

	// Initialize logger
	logger := log.New(os.Stdout, "CATCH-ALL: ", log.LstdFlags)
	logger.Println("Starting catch-all domain service...")

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		logger.Fatalf("Failed to load configuration: %v", err)
	}

	// Log configuration
	logger.Printf("Server configuration: %+v", cfg.Server)
	logger.Printf("MongoDB configuration: %+v", cfg.MongoDB)
	logger.Printf("Business configuration: delivered threshold = %d", cfg.Business.DeliveredThreshold)
	logger.Printf("Worker count: %d", *workers)
	logger.Printf("Using MongoDB: %v", *useMongoDb)
	logger.Printf("Cache enabled: %v, Cache TTL: %v", *enableCache, *cacheTTL)

	// Initialize metrics collector
	metricsCollector := metrics.NewMetricsCollector()

	// Create repository context
	repoCtx := context.Background()

	// Determine which repository type to use
	var repoType repository.RepositoryType
	if *useMongoDb {
		repoType = repository.RepositoryTypeMongoDB
	} else {
		repoType = repository.RepositoryTypeInMemory
	}

	// Set repository options
	repoOptions := repository.RepositoryOptions{
		EnableCaching: *enableCache,
		CacheTTL:      *cacheTTL,
	}

	// Initialize repository
	repo, err := repository.NewDomainRepository(
		repoType,
		repoCtx,
		cfg.MongoDB.URI,
		cfg.MongoDB.Database,
		logger,
		repoOptions,
	)
	if err != nil {
		logger.Fatalf("Failed to initialize repository: %v", err)
	}

	// Initialize domain service with business configuration
	domainService := service.NewDomainService(repo, logger)

	// Initialize stats service
	statsService := service.NewStatsService(repo, logger)

	// Initialize event processor if enabled
	var processor eventprocessor.Processor
	if *enableProcessor {
		// Check if repository supports batch operations
		batchRepo, ok := repo.(repository.BatchDomainRepository)
		if ok && *useMongoDb {
			logger.Println("Using batch event processor")
			processor = eventprocessor.NewBatchEventProcessor(
				domainService,
				batchRepo,
				metricsCollector,
				logger,
				*workers,
			)
		} else {
			logger.Println("Using standard event processor")
			processor = eventprocessor.NewEventProcessor(
				domainService,
				metricsCollector,
				logger,
				*workers,
			)
		}

		processor.Start()
	}

	// Initialize HTTP server and routes
	router := api.NewRouter(domainService, statsService, metricsCollector, logger)

	// Configure server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start server in a goroutine
	go func() {
		logger.Printf("Server listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Server error: %v", err)
		}
	}()

	// Setup graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Println("Shutting down server...")

	// Stop event processor if running
	if processor != nil {
		logger.Println("Stopping event processor...")
		processor.Stop()
	}

	// Print cache stats if available
	if *enableCache {
		if cachedRepo, ok := repo.(*repository.CachedDomainRepository); ok {
			hits, misses := cachedRepo.GetCacheStats()
			hitRatio := float64(hits) / float64(hits+misses) * 100
			if hits+misses > 0 {
				logger.Printf("Cache performance: %d hits, %d misses (%.1f%% hit ratio)",
					hits, misses, hitRatio)
			}
		}
	}

	// Create context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatalf("Server forced to shutdown: %v", err)
	}

	logger.Println("Server gracefully stopped")
}
