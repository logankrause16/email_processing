package repository

import (
	"context"
	"sync"
	"time"

	"email_processing/internal/domain"
)

// InMemoryDomainRepository is an in-memory implementation of DomainRepository
type InMemoryDomainRepository struct {
	domains map[string]*domain.DomainInfo
	mutex   sync.RWMutex
}

// NewInMemoryDomainRepository creates a new in-memory domain repository
func NewInMemoryDomainRepository() *InMemoryDomainRepository {
	return &InMemoryDomainRepository{
		domains: make(map[string]*domain.DomainInfo),
	}
}

// GetDomainInfo retrieves information about a domain
func (r *InMemoryDomainRepository) GetDomainInfo(ctx context.Context, domainName string) (*domain.DomainInfo, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	info, exists := r.domains[domainName]
	if !exists {
		// If the domain doesn't exist, create a new entry with default values
		return &domain.DomainInfo{
			Name:           domainName,
			DeliveredCount: 0,
			BouncedCount:   0,
			Status:         domain.StatusUnknown,
			UpdatedAt:      time.Now(),
		}, nil
	}

	return info, nil
}

// IncrementEventCount increments the count for a specific event type
func (r *InMemoryDomainRepository) IncrementEventCount(ctx context.Context, domainName string, eventType domain.EventType) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Get or create domain info
	info, exists := r.domains[domainName]
	if !exists {
		info = &domain.DomainInfo{
			Name:           domainName,
			DeliveredCount: 0,
			BouncedCount:   0,
			Status:         domain.StatusUnknown,
			UpdatedAt:      time.Now(),
		}
		r.domains[domainName] = info
	}

	// Update the appropriate counter
	switch eventType {
	case domain.EventDelivered:
		info.DeliveredCount++
	case domain.EventBounced:
		info.BouncedCount++
	}

	info.UpdatedAt = time.Now()
	return nil
}

// UpdateDomainStatus updates the status of a domain
func (r *InMemoryDomainRepository) UpdateDomainStatus(ctx context.Context, domainName string, status domain.DomainStatus) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Get or create domain info
	info, exists := r.domains[domainName]
	if !exists {
		info = &domain.DomainInfo{
			Name:           domainName,
			DeliveredCount: 0,
			BouncedCount:   0,
			Status:         status,
			UpdatedAt:      time.Now(),
		}
		r.domains[domainName] = info
	} else {
		info.Status = status
		info.UpdatedAt = time.Now()
	}

	return nil
}

// GetDomainStats returns statistics about the domains
func (r *InMemoryDomainRepository) GetDomainStats(ctx context.Context) (map[string]int64, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	stats := map[string]int64{
		"catch-all":     0,
		"not-catch-all": 0,
		"unknown":       0,
		"total":         int64(len(r.domains)),
	}

	// Count domains by status
	for _, info := range r.domains {
		switch info.Status {
		case domain.StatusCatchAll:
			stats["catch-all"]++
		case domain.StatusNotCatchAll:
			stats["not-catch-all"]++
		case domain.StatusUnknown:
			stats["unknown"]++
		}
	}

	return stats, nil
}

// IncrementEventCountBatch increments the counts for multiple domains in a batch
func (r *InMemoryDomainRepository) IncrementEventCountBatch(ctx context.Context, events []EventBatch) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for _, event := range events {
		// Get or create domain info
		info, exists := r.domains[event.DomainName]
		if !exists {
			info = &domain.DomainInfo{
				Name:           event.DomainName,
				DeliveredCount: 0,
				BouncedCount:   0,
				Status:         domain.StatusUnknown,
				UpdatedAt:      time.Now(),
			}
			r.domains[event.DomainName] = info
		}

		// Update the appropriate counter
		switch event.EventType {
		case domain.EventDelivered:
			info.DeliveredCount += int64(event.Count)
		case domain.EventBounced:
			info.BouncedCount += int64(event.Count)
		}

		info.UpdatedAt = time.Now()
	}

	return nil
}

// UpdateStatusBatch updates the status for multiple domains in a batch
func (r *InMemoryDomainRepository) UpdateStatusBatch(ctx context.Context, updates map[string]domain.DomainStatus) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	now := time.Now()

	for domainName, status := range updates {
		// Get or create domain info
		info, exists := r.domains[domainName]
		if !exists {
			info = &domain.DomainInfo{
				Name:           domainName,
				DeliveredCount: 0,
				BouncedCount:   0,
				Status:         status,
				UpdatedAt:      now,
			}
			r.domains[domainName] = info
		} else {
			info.Status = status
			info.UpdatedAt = now
		}
	}

	return nil
}

// GetDomainsStatusBatch retrieves information about multiple domains in a batch
func (r *InMemoryDomainRepository) GetDomainsStatusBatch(ctx context.Context, domainNames []string) (map[string]*domain.DomainInfo, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	result := make(map[string]*domain.DomainInfo, len(domainNames))
	now := time.Now()

	for _, name := range domainNames {
		info, exists := r.domains[name]
		if !exists {
			// If the domain doesn't exist, create a new entry with default values
			result[name] = &domain.DomainInfo{
				Name:           name,
				DeliveredCount: 0,
				BouncedCount:   0,
				Status:         domain.StatusUnknown,
				UpdatedAt:      now,
			}
		} else {
			// Make a copy to avoid concurrent modification
			infoCopy := *info
			result[name] = &infoCopy
		}
	}

	return result, nil
}
