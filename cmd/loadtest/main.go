package main

import (
	"context"
	"flag"
	"log"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/logankrause16/email_processing/internal/domain"
	"github.com/logankrause16/email_processing/internal/repository"
	"github.com/logankrause16/email_processing/pkg/eventpool"
)

func main() {
	// Parse command line flags
	mongoURI := flag.String("mongodb-uri", "mongodb://localhost:27017", "MongoDB connection URI")
	dbName := flag.String("db-name", "catchall", "MongoDB database name")
	numEvents := flag.Int("events", 100000, "Total number of events to process")
	concurrency := flag.Int("concurrency", 10, "Number of concurrent workers")
	batchSize := flag.Int("batch-size", 100, "Number of events per batch")
	useCache := flag.Bool("cache", true, "Enable repository caching")
	useBatch := flag.Bool("batch", true, "Use batch processing")
	verbose := flag.Bool("verbose", false, "Verbose output")
	flag.Parse()

	// Initialize logger
	logger := log.New(os.Stdout, "LOADTEST: ", log.LstdFlags)
	logger.Printf("Starting load test with %d events, %d workers, batch size %d", *numEvents, *concurrency, *batchSize)
	logger.Printf("Cache enabled: %v, Batch processing: %v", *useCache, *useBatch)

	// Set up options
	opts := repository.DefaultRepositoryOptions()
	opts.EnableCaching = *useCache
	opts.CacheTTL = 1 * time.Minute

	// Initialize repository
	ctx := context.Background()
	repo, err := repository.NewDomainRepository(
		repository.RepositoryTypeMongoDB,
		ctx,
		*mongoURI,
		*dbName,
		logger,
		opts,
	)
	if err != nil {
		logger.Fatalf("Failed to create repository: %v", err)
	}

	// Check if we can use batch operations
	var batchRepo repository.BatchDomainRepository
	canBatch := false
	if *useBatch {
		var ok bool
		batchRepo, ok = repository.AsBatchRepository(repo)
		if !ok {
			logger.Println("Repository does not support batch operations, falling back to individual operations")
		} else {
			canBatch = true
			logger.Println("Using batch repository operations")
		}
	}

	// Initialize event generator
	eventPool := eventpool.SpawnEventPool()
	defer eventPool.Close()

	// Statistics
	var totalProcessed int64
	var totalDelivered int64
	var totalBounced int64
	var totalDomains sync.Map
	startTime := time.Now()

	// Create worker pool
	var wg sync.WaitGroup
	eventsToProcess := make(chan *eventpool.Event, *batchSize*2)

	// Launch producer
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(eventsToProcess)

		for i := 0; i < *numEvents; i++ {
			event := eventPool.GetEvent()
			eventsToProcess <- event
		}
	}()

	// Display progress periodically
	if *verbose {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					processed := atomic.LoadInt64(&totalProcessed)
					elapsed := time.Since(startTime).Seconds()
					eps := float64(processed) / elapsed
					domainCount := countDomains(&totalDomains)

					logger.Printf("Progress: %d/%d events (%.1f%%), %.1f events/sec, %d domains",
						processed, *numEvents, float64(processed)*100/float64(*numEvents),
						eps, domainCount)

					if processed >= int64(*numEvents) {
						return
					}
				}
			}
		}()
	}

	// Launch workers
	if canBatch {
		// Using batch processing
		for i := 0; i < *concurrency; i++ {
			wg.Add(1)
			go batchWorker(i, *batchSize, &wg, eventsToProcess, batchRepo, logger,
				&totalProcessed, &totalDelivered, &totalBounced, &totalDomains, *verbose)
		}
	} else {
		// Using individual processing
		for i := 0; i < *concurrency; i++ {
			wg.Add(1)
			go individualWorker(i, &wg, eventsToProcess, repo, eventPool, logger,
				&totalProcessed, &totalDelivered, &totalBounced, &totalDomains, *verbose)
		}
	}

	// Wait for all workers to finish
	wg.Wait()

	// Print results
	elapsed := time.Since(startTime)
	domainCount := countDomains(&totalDomains)
	logger.Printf("Test completed in %v", elapsed)
	logger.Printf("Processed %d events (%.1f events/sec)",
		totalProcessed, float64(totalProcessed)/elapsed.Seconds())
	logger.Printf("Events: %d delivered, %d bounced",
		totalDelivered, totalBounced)
	logger.Printf("Unique domains: %d", domainCount)

	// If using cache, print cache stats
	if *useCache {
		if cachedRepo, ok := repo.(*repository.CachedDomainRepository); ok {
			hits, misses := cachedRepo.GetCacheStats()
			hitRatio := float64(hits) / float64(hits+misses) * 100
			logger.Printf("Cache performance: %d hits, %d misses (%.1f%% hit ratio)",
				hits, misses, hitRatio)
		}
	}

	// Print domain stats from repository
	if statsRepo, ok := repo.(interface {
		GetDomainStats(ctx context.Context) (map[string]int64, error)
	}); ok {
		stats, err := statsRepo.GetDomainStats(ctx)
		if err != nil {
			logger.Printf("Error getting domain stats: %v", err)
		} else {
			logger.Printf("Domain stats: %d catch-all, %d not-catch-all, %d unknown, %d total",
				stats["catch-all"], stats["not-catch-all"], stats["unknown"], stats["total"])
		}
	}
}

// individualWorker processes events one at a time
func individualWorker(
	id int,
	wg *sync.WaitGroup,
	events <-chan *eventpool.Event,
	repo repository.DomainRepository,
	eventPool eventpool.EventPool,
	logger *log.Logger,
	totalProcessed *int64,
	totalDelivered *int64,
	totalBounced *int64,
	totalDomains *sync.Map,
	verbose bool,
) {
	defer wg.Done()
	if verbose {
		logger.Printf("Worker %d started", id)
	}

	for event := range events {
		// Convert event type
		var eventType domain.EventType
		if event.Type == eventpool.TypeDelivered {
			eventType = domain.EventDelivered
			atomic.AddInt64(totalDelivered, 1)
		} else {
			eventType = domain.EventBounced
			atomic.AddInt64(totalBounced, 1)
		}

		// Process event
		err := repo.IncrementEventCount(context.Background(), event.Domain, eventType)
		if err != nil {
			logger.Printf("Worker %d error: %v", id, err)
		}

		// Track domain
		totalDomains.Store(event.Domain, true)

		// Recycle event
		eventPool.RecycleEvent(event)

		// Update counter
		atomic.AddInt64(totalProcessed, 1)
	}

	if verbose {
		logger.Printf("Worker %d finished", id)
	}
}

// batchWorker processes events in batches
func batchWorker(
	id int,
	batchSize int,
	wg *sync.WaitGroup,
	events <-chan *eventpool.Event,
	repo repository.BatchDomainRepository,
	logger *log.Logger,
	totalProcessed *int64,
	totalDelivered *int64,
	totalBounced *int64,
	totalDomains *sync.Map,
	verbose bool,
) {
	defer wg.Done()
	if verbose {
		logger.Printf("Batch worker %d started with batch size %d", id, batchSize)
	}

	batch := make([]repository.EventBatch, 0, batchSize)
	eventMap := make(map[string]map[domain.EventType]int)
	recycleQueue := make([]*eventpool.Event, 0, batchSize)

	processEvents := func() {
		if len(eventMap) == 0 {
			return
		}

		// Convert map to batch
		batch = batch[:0] // Clear batch slice but reuse capacity
		for domain, events := range eventMap {
			for eventType, count := range events {
				batch = append(batch, repository.EventBatch{
					DomainName: domain,
					EventType:  eventType,
					Count:      count,
				})
			}
		}

		// Process batch
		err := repo.IncrementEventCountBatch(context.Background(), batch)
		if err != nil {
			logger.Printf("Batch worker %d error: %v", id, err)
		}

		// Clear map for next batch
		for domain := range eventMap {
			delete(eventMap, domain)
		}
	}

	timeout := time.NewTimer(100 * time.Millisecond)

	for {
		select {
		case event, ok := <-events:
			if !ok {
				// Channel closed, process remaining events
				processEvents()

				// Recycle remaining events
				for _, e := range recycleQueue {
					e.RecycleEvent(e)
				}

				if verbose {
					logger.Printf("Batch worker %d finished", id)
				}
				return
			}

			// Convert event type
			var eventType domain.EventType
			if event.Type == eventpool.TypeDelivered {
				eventType = domain.EventDelivered
				atomic.AddInt64(totalDelivered, 1)
			} else {
				eventType = domain.EventBounced
				atomic.AddInt64(totalBounced, 1)
			}

			// Add to batch
			if eventMap[event.Domain] == nil {
				eventMap[event.Domain] = make(map[domain.EventType]int)
			}
			eventMap[event.Domain][eventType]++

			// Track domain
			totalDomains.Store(event.Domain, true)

			// Queue event for recycling
			recycleQueue = append(recycleQueue, event)

			// Update counter
			atomic.AddInt64(totalProcessed, 1)

			// Process if batch is full
			if len(eventMap) >= batchSize {
				processEvents()

				// Recycle events
				for _, e := range recycleQueue {
					e.RecycleEvent(e)
				}
				recycleQueue = recycleQueue[:0] // Clear recycle queue

				// Reset timeout
				if !timeout.Stop() {
					<-timeout.C
				}
				timeout.Reset(100 * time.Millisecond)
			}

		case <-timeout.C:
			// Process partial batch on timeout
			if len(eventMap) > 0 {
				processEvents()

				// Recycle events
				for _, e := range recycleQueue {
					e.RecycleEvent(e)
				}
				recycleQueue = recycleQueue[:0] // Clear recycle queue
			}

			timeout.Reset(100 * time.Millisecond)
		}
	}
}

// countDomains counts unique domains in the map
func countDomains(domains *sync.Map) int {
	count := 0
	domains.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

// randomBool returns true with the specified probability
func randomBool(probability float64) bool {
	return rand.Float64() < probability
}
