package repository

import (
	"context"
	"log"
	"time"

	"github.com/logankrause16/email_processing/internal/domain"
)

// DomainRepository defines the interface for domain data storage
type DomainRepository interface {
	// GetDomainInfo retrieves information about a domain
	GetDomainInfo(ctx context.Context, domainName string) (*domain.DomainInfo, error)

	// IncrementEventCount increments the count for a specific event type
	IncrementEventCount(ctx context.Context, domainName string, eventType domain.EventType) error

	// UpdateDomainStatus updates the status of a domain
	UpdateDomainStatus(ctx context.Context, domainName string, status domain.DomainStatus) error
}

// BatchDomainRepository defines additional batch operations for repositories that support it
type BatchDomainRepository interface {
	// IncrementEventCountBatch increments event counts for multiple domains at once
	IncrementEventCountBatch(ctx context.Context, events []EventBatch) error

	// UpdateStatusBatch updates status for multiple domains at once
	UpdateStatusBatch(ctx context.Context, updates map[string]domain.DomainStatus) error

	// GetDomainsStatusBatch retrieves information about multiple domains at once
	GetDomainsStatusBatch(ctx context.Context, domainNames []string) (map[string]*domain.DomainInfo, error)
}

// EventBatch represents a batch of events to be processed
type EventBatch struct {
	DomainName string
	EventType  domain.EventType
	Count      int
}

// RepositoryType defines the type of repository to use
type RepositoryType string

const (
	// RepositoryTypeInMemory uses an in-memory storage
	RepositoryTypeInMemory RepositoryType = "memory"

	// RepositoryTypeMongoDB uses MongoDB for storage
	RepositoryTypeMongoDB RepositoryType = "mongodb"
)

// RepositoryOptions defines options for creating a repository
type RepositoryOptions struct {
	// EnableCaching enables caching layer
	EnableCaching bool

	// CacheTTL defines how long items should stay in cache
	CacheTTL time.Duration
}

// DefaultRepositoryOptions returns default repository options
func DefaultRepositoryOptions() RepositoryOptions {
	return RepositoryOptions{
		EnableCaching: true,
		CacheTTL:      5 * time.Minute, // 5 minute cache TTL by default
	}
}

// NewDomainRepository creates a new domain repository
func NewDomainRepository(
	repoType RepositoryType,
	ctx context.Context,
	mongoURI string,
	dbName string,
	logger *log.Logger,
	options ...RepositoryOptions,
) (DomainRepository, error) {
	// Apply options with defaults
	opts := DefaultRepositoryOptions()
	if len(options) > 0 {
		opts = options[0]
	}

	var baseRepo DomainRepository
	var err error

	// Create base repository
	switch repoType {
	case RepositoryTypeMongoDB:
		logger.Println("Using MongoDB repository")
		baseRepo, err = NewMongoDomainRepository(ctx, mongoURI, dbName)
		if err != nil {
			return nil, err
		}
	default:
		logger.Println("Using in-memory repository")
		baseRepo = NewInMemoryDomainRepository()
	}

	// Add caching if enabled
	if opts.EnableCaching {
		logger.Printf("Adding cache layer with TTL %v", opts.CacheTTL)
		baseRepo = NewCachedDomainRepository(baseRepo, opts.CacheTTL, logger)
	}

	return baseRepo, nil
}

// AsBatchRepository tries to convert a DomainRepository to a BatchDomainRepository
// Returns the BatchDomainRepository and a boolean indicating success
func AsBatchRepository(repo DomainRepository) (BatchDomainRepository, bool) {
	// Try to convert to BatchDomainRepository
	batchRepo, ok := repo.(BatchDomainRepository)
	return batchRepo, ok
}
