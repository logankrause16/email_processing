package repository

import (
	"context"
	"log"
	"sync"
	"time"

	"email_processing/internal/domain"
)

// CachedDomainRepository is a caching wrapper around another DomainRepository
type CachedDomainRepository struct {
	repo            DomainRepository
	cache           map[string]*cachedItem
	mutex           sync.RWMutex
	ttl             time.Duration
	cleanupInterval time.Duration
	logger          *log.Logger
	stats           CacheStats
}

// CacheStats tracks cache performance metrics
type CacheStats struct {
	Hits   uint64
	Misses uint64
	mutex  sync.RWMutex
}

type cachedItem struct {
	info      *domain.DomainInfo
	expiresAt time.Time
}

// NewCachedDomainRepository creates a new cached repository wrapper
func NewCachedDomainRepository(repo DomainRepository, ttl time.Duration, logger *log.Logger) *CachedDomainRepository {
	cr := &CachedDomainRepository{
		repo:            repo,
		cache:           make(map[string]*cachedItem),
		ttl:             ttl,
		cleanupInterval: 5 * time.Minute, // Clean expired items every 5 minutes
		logger:          logger,
	}

	// Start background cleanup routine
	go cr.startCleanup()

	return cr
}

// GetDomainInfo retrieves information about a domain, using cache when available
func (c *CachedDomainRepository) GetDomainInfo(ctx context.Context, domainName string) (*domain.DomainInfo, error) {
	// Try to get from cache first
	c.mutex.RLock()
	item, found := c.cache[domainName]
	c.mutex.RUnlock()

	// If found in cache and not expired, return it
	if found && time.Now().Before(item.expiresAt) {
		c.incrementHits()
		return item.info, nil
	}

	// Not in cache or expired, get from underlying repository
	c.incrementMisses()
	info, err := c.repo.GetDomainInfo(ctx, domainName)
	if err != nil {
		return nil, err
	}

	// Cache the result
	c.mutex.Lock()
	c.cache[domainName] = &cachedItem{
		info:      info,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mutex.Unlock()

	return info, nil
}

// IncrementEventCount increments the count and invalidates cache
func (c *CachedDomainRepository) IncrementEventCount(ctx context.Context, domainName string, eventType domain.EventType) error {
	// Increment count in the underlying repository
	err := c.repo.IncrementEventCount(ctx, domainName, eventType)
	if err != nil {
		return err
	}

	// Invalidate cache for this domain
	c.mutex.Lock()
	delete(c.cache, domainName)
	c.mutex.Unlock()

	return nil
}

// UpdateDomainStatus updates the status and invalidates cache
func (c *CachedDomainRepository) UpdateDomainStatus(ctx context.Context, domainName string, status domain.DomainStatus) error {
	// Update status in the underlying repository
	err := c.repo.UpdateDomainStatus(ctx, domainName, status)
	if err != nil {
		return err
	}

	// Invalidate cache for this domain
	c.mutex.Lock()
	delete(c.cache, domainName)
	c.mutex.Unlock()

	return nil
}

// GetCacheStats returns current cache statistics
func (c *CachedDomainRepository) GetCacheStats() (hits, misses uint64) {
	c.stats.mutex.RLock()
	defer c.stats.mutex.RUnlock()
	return c.stats.Hits, c.stats.Misses
}

// startCleanup periodically removes expired items from cache
// Checks for an item every 5 minutes
// This is a goroutine that runs in the background, cleaning up expired items
func (c *CachedDomainRepository) startCleanup() {
	ticker := time.NewTicker(c.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

// cleanup removes expired items from cache
// This method is called periodically by the cleanup goroutine
// It checks the cache for expired items and removes them
// It also logs the number of items removed if a logger is provided
func (c *CachedDomainRepository) cleanup() {
	now := time.Now()
	var count int

	c.mutex.Lock()
	for domain, item := range c.cache {
		if now.After(item.expiresAt) {
			delete(c.cache, domain)
			count++
		}
	}
	c.mutex.Unlock()

	if count > 0 && c.logger != nil {
		c.logger.Printf("Cache cleanup: removed %d expired items", count)
	}
}

// incrementHits increments the cache hit counter
// Making sure we acquire the lock before modifying the hits counter
// This is a thread-safe operation
func (c *CachedDomainRepository) incrementHits() {
	c.stats.mutex.Lock()
	c.stats.Hits++
	c.stats.mutex.Unlock()
}

// incrementMisses increments the cache miss counter
// Making sure we acquire the lock before modifying the misses counter
// This is a thread-safe operation
func (c *CachedDomainRepository) incrementMisses() {
	c.stats.mutex.Lock()
	c.stats.Misses++
	c.stats.mutex.Unlock()
}

// If the wrapped repository supports batch operations, pass them through
// These methods implement the BatchDomainRepository interface

// IncrementEventCountBatch implements batch increment if supported by underlying repo
func (c *CachedDomainRepository) IncrementEventCountBatch(ctx context.Context, events []EventBatch) error {
	// Check if underlying repo supports batch operations
	if batchRepo, ok := c.repo.(BatchDomainRepository); ok {
		// Execute batch operation
		err := batchRepo.IncrementEventCountBatch(ctx, events)
		if err != nil {
			return err
		}

		// Invalidate cache for all affected domains
		c.mutex.Lock()
		for _, event := range events {
			delete(c.cache, event.DomainName)
		}
		c.mutex.Unlock()

		return nil
	}

	// Fall back to individual operations if batch not supported
	for _, event := range events {
		for i := 0; i < event.Count; i++ {
			if err := c.IncrementEventCount(ctx, event.DomainName, event.EventType); err != nil {
				return err
			}
		}
	}

	return nil
}

// UpdateStatusBatch implements batch status updates if supported by underlying repo
func (c *CachedDomainRepository) UpdateStatusBatch(ctx context.Context, updates map[string]domain.DomainStatus) error {
	// Check if underlying repo supports batch operations
	if batchRepo, ok := c.repo.(BatchDomainRepository); ok {
		// Execute batch operation
		err := batchRepo.UpdateStatusBatch(ctx, updates)
		if err != nil {
			return err
		}

		// Invalidate cache for all affected domains
		c.mutex.Lock()
		for domainName := range updates {
			delete(c.cache, domainName)
		}
		c.mutex.Unlock()

		return nil
	}

	// Fall back to individual operations if batch not supported
	for domainName, status := range updates {
		if err := c.UpdateDomainStatus(ctx, domainName, status); err != nil {
			return err
		}
	}

	return nil
}

// GetDomainsStatusBatch implements batch get if supported by underlying repo
func (c *CachedDomainRepository) GetDomainsStatusBatch(ctx context.Context, domainNames []string) (map[string]*domain.DomainInfo, error) {
	// Initialize result map and list of domains to fetch
	result := make(map[string]*domain.DomainInfo)
	var domainsToFetch []string

	// Check cache first for each domain
	c.mutex.RLock()
	now := time.Now()
	for _, name := range domainNames {
		item, found := c.cache[name]
		if found && now.Before(item.expiresAt) {
			// Cache hit
			c.incrementHits()
			result[name] = item.info
		} else {
			// Cache miss
			c.incrementMisses()
			domainsToFetch = append(domainsToFetch, name)
		}
	}
	c.mutex.RUnlock()

	// If all domains were in cache, return early
	if len(domainsToFetch) == 0 {
		return result, nil
	}

	// Check if underlying repo supports batch operations
	if batchRepo, ok := c.repo.(BatchDomainRepository); ok {
		// Fetch missing domains in batch
		fetchedDomains, err := batchRepo.GetDomainsStatusBatch(ctx, domainsToFetch)
		if err != nil {
			return nil, err
		}

		// Update cache with fetched domains
		c.mutex.Lock()
		for name, info := range fetchedDomains {
			result[name] = info
			c.cache[name] = &cachedItem{
				info:      info,
				expiresAt: now.Add(c.ttl),
			}
		}
		c.mutex.Unlock()

		return result, nil
	}

	// Fall back to individual gets if batch not supported
	for _, name := range domainsToFetch {
		info, err := c.repo.GetDomainInfo(ctx, name)
		if err != nil {
			return nil, err
		}

		result[name] = info

		// Cache the result
		c.mutex.Lock()
		c.cache[name] = &cachedItem{
			info:      info,
			expiresAt: now.Add(c.ttl),
		}
		c.mutex.Unlock()
	}

	return result, nil
}
