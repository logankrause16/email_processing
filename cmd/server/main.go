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

	"email_processing/internal/api"
	"email_processing/internal/config"
	"email_processing/internal/eventprocessor"
	"email_processing/internal/repository"
	"email_processing/internal/service"
	"email_processing/pkg/metrics"
)

/*

Disclaimer: This application is a work of fiction. Any resemblance to actual persons, living or dead, or actual events is purely coincidental.

Seriously though, this app was created with the help of Claude AI (3.7 Sonnet). Claude was used as a pair programmer
to help with the implementation of a few key components such as Cacheing (Just more of a rubber ducky really. Asking for a skeleton base to work with).
The code was written by me, but Claude provided suggestions and guidance along the way.
Specifically for refamiliarizing myself with Go and the libraries used in this application as well as the concurrent mutex pattern to
assist me with a thread-safe cache implementation. If this disqualifies me from furthering my candidacy for the Mailgun position, I understand.
And let me say that I am very grateful for the opportunity, and this project has had me flex my Go, mutex and cacheing muscles quite a bit!

This was not written by AI lol Or was it

*/

func main() {
	// Parse command line flags with... flag lol
	// These flags are used to configure the server and its behavior
	// The flag package is a simple way to parse command line arguments
	// and is part of the Go standard library
	configPath := flag.String("config", "", "Path to configuration file")
	workers := flag.Int("workers", runtime.NumCPU(), "Number of event processing workers")
	enableProcessor := flag.Bool("processor", false, "Enable event processor")
	useMongoDb := flag.Bool("mongodb", false, "Use MongoDB instead of in-memory storage")
	enableCache := flag.Bool("cache", true, "Enable repository caching")
	cacheTTL := flag.Duration("cache-ttl", 5*time.Minute, "Cache TTL duration")
	flag.Parse()

	// Initialize logger - This is a simple logger that writes to standard output
	// It is used to log messages to the console
	logger := log.New(os.Stdout, "CATCH-ALL: ", log.LstdFlags)
	logger.Println("Starting catch-all domain service...")

	// Load configuration - Dependency Injection goodness begins!
	// We send it with *configPath vs configPath because we want the value of the pointer, not the pointer itself
	// When we use flag, it creates a pointer to the value we pass in
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

	// Determine which repository type to use - if the user has specified MongoDB, we use that
	// Otherwise, we use in-memory storage
	// This is a simple way to switch between different storage backends
	// without changing the code in the rest of the application
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

	// Initialize repository - As of now, we only support MongoDB and in-memory storage
	// This is where we create the repository based on the type specified
	// and pass it the context, mongoURI and dbName
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
	// This is where we create the domain service and pass it the repository
	// and the logger. The domain service is responsible for the business logic
	// and interacts with the repository to perform CRUD operations
	// and other operations on the data
	domainService := service.NewDomainService(repo, logger)

	// Initialize stats service
	// This is where we create the stats service and pass it the repository
	// and the logger. The stats service is responsible for collecting
	// and reporting statistics about the system. It interacts with the repository
	// to perform CRUD operations and other operations on the data
	// and provides an API for the domain service to access the statistics
	// and report them to the user
	statsService := service.NewStatsService(repo, logger)

	// Initialize event processor if enabled
	var processor eventprocessor.Processor
	if *enableProcessor {
		// Check if repository supports batch operations
		batchRepo, ok := repo.(repository.BatchDomainRepository)
		if ok && *useMongoDb {
			logger.Println("Using batch event processor")
			// If the repository supports batch operations, we use the batch event processor
			// This is more efficient for large numbers of events. Sends along the batch repo
			// to the event processor so it can use it to perform batch operations
			processor = eventprocessor.NewBatchEventProcessor(
				domainService,
				batchRepo,
				metricsCollector,
				logger,
				*workers,
			)
		} else {
			logger.Println("Using standard event processor")
			// If the repository does not support batch operations, we use the standard event processor
			// This is less efficient for large numbers of events, but works with any repository
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
	// This is where we create the HTTP server and set up the routes
	router := api.NewRouter(domainService, statsService, metricsCollector, logger)

	// Configure server
	// Lets take the server configuration from the config file and serve this bad boi up
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start server in a goroutine
	// But we want to run this in a goroutine so we can handle shutdown gracefully
	// This allows us to run the server in the background and continue
	go func() {
		logger.Printf("Server listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Server error: %v", err)
		}
	}()

	// Setup graceful shutdown
	// Look at that quit channel! I forgot Go could handle this so elegantly
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Println("Shutting down server...")

	// Stop event processor if running
	// Stahp it! Stahp it right now!
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
