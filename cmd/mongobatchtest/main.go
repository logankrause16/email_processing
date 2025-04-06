package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/logankrause16/email_processing/internal/repository"
)

func main() {
	// Parse command line flags
	mongoURI := flag.String("uri", "mongodb://localhost:27017", "MongoDB connection URI")
	dbName := flag.String("db", "catchall", "MongoDB database name")
	batchSize := flag.Int("batch-size", 1000, "Number of operations per batch")
	numDomains := flag.Int("domains", 100, "Number of unique domains to test")
	testMode := flag.String("mode", "both", "Test mode: individual, batch, or both")
	flag.Parse()

	// Initialize logger
	logger := log.New(os.Stdout, "MONGO-TEST: ", log.LstdFlags)
	logger.Printf("Starting MongoDB batch operation test with %d domains", *numDomains)

	// Initialize MongoDB repository
	ctx := context.Background()
	logger.Printf("Connecting to MongoDB at %s, database %s", *mongoURI, *dbName)

	repo, err := repository.NewMongoDomainRepository(ctx, *mongoURI, *dbName)
	if err != nil {
		logger.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	// Get batch repository
	batchRepo, ok := repository.AsBatchRepository(repo)
	if !ok {
		logger.Fatalf("Repository does not support batch operations")
	}

	// Generate test domains
	domains := generateTestDomains(*numDomains)
	logger.Printf("Generated %d test domains", len(domains))

	// Run individual operations test
	if *testMode == "individual" || *testMode == "both" {
		logger.Println("Running individual operations test...")

		start := time.Now()
		count := runIndividualTest(ctx, repo, domains)
		duration := time.Since(start)

		logger.Printf("Individual operations: %d operations in %v (%.2f ops/sec)",
			count, duration, float64(count)/duration.Seconds())
	}

	// Run batch operations test
	if *testMode == "batch" || *testMode == "both" {
		logger.Printf("Running batch operations test with batch size %d...", *batchSize)

		start := time.Now()
		count := runBatchTest(ctx, batchRepo, domains, *batchSize)
		duration := time.Since(start)

		logger.Printf("Batch operations: %d operations in %v (%.2f ops/sec)",
			count, duration, float64(count)/duration.Seconds())
	}

	// Clean up test data
	logger.Println("Cleaning up test data...")
	cleanupTestData(ctx, repo, domains)

	logger.Println("Test completed")
}

// generateTestDomains generates unique domain names for testing
func generateTestDomains(count int) []string {
	domains := make([]string, count)
	for i := 0; i < count; i++ {
		domains[i] = fmt.Sprintf("test-domain-%d.example.com", i)
	}
	return domains
}

// runIndividualTest runs individual increment operations
func runIndividualTest(ctx context.Context, repo repository.DomainRepository, domains []string) int {
	totalCount := 0
	for _, domain := range domains {
		// Increment delivered count
		for i := 0; i < 10; i++ {
			err := repo.IncrementEventCount(ctx, domain, domain.EventDelivered)
			if err != nil {
				log.Printf("Error incrementing delivered count for %s: %v", domain, err)
				continue
			}
			totalCount++
		}

		// Increment bounced count for some domains
		if rand.Intn(100) < 20 { // 20% chance of having bounces
			err := repo.IncrementEventCount(ctx, domain, domain.EventBounced)
			if err != nil {
				log.Printf("Error incrementing bounced count for %s: %v", domain, err)
				continue
			}
			totalCount++
		}
	}
	return totalCount
}

// runBatchTest runs batch increment operations
func runBatchTest(ctx context.Context, repo repository.BatchDomainRepository, domains []string, batchSize int) int {
	totalCount := 0
	batch := make([]repository.EventBatch, 0, batchSize)

	// Function to process a batch
	processBatch := func() {
		if len(batch) == 0 {
			return
		}

		err := repo.IncrementEventCountBatch(ctx, batch)
		if err != nil {
			log.Printf("Error processing batch: %v", err)
		} else {
			totalCount += len(batch)
		}

		// Clear batch
		batch = batch[:0]
	}

	// Build and process batches
	for _, domain := range domains {
		// Add delivered events
		for i := 0; i < 10; i++ {
			batch = append(batch, repository.EventBatch{
				DomainName: domain,
				EventType:  domain.EventDelivered,
				Count:      1,
			})

			// Process batch if full
			if len(batch) >= batchSize {
				processBatch()
			}
		}

		// Add bounced event for some domains
		if rand.Intn(100) < 20 { // 20% chance of having bounces
			batch = append(batch, repository.EventBatch{
				DomainName: domain,
				EventType:  domain.EventBounced,
				Count:      1,
			})

			// Process batch if full
			if len(batch) >= batchSize {
				processBatch()
			}
		}
	}

	// Process any remaining items
	processBatch()

	return totalCount
}

// cleanupTestData removes test data from the database
func cleanupTestData(ctx context.Context, repo repository.DomainRepository, domains []string) {
	// For this test, we're not actually deleting the data
	// In a real application, you might want to clean up test data
	log.Println("Note: Test data is left in the database for inspection")
}
