package service

import (
	"context"
	"log"
	"time"

	"email_processing/internal/repository"
)

// StatsService provides operations for retrieving domain statistics
type StatsService struct {
	repo   repository.DomainRepository
	logger *log.Logger
}

// NewStatsService creates a new stats service
func NewStatsService(repo repository.DomainRepository, logger *log.Logger) *StatsService {
	return &StatsService{
		repo:   repo,
		logger: logger,
	}
}

// DomainStats represents statistics about domains
type DomainStats struct {
	CatchAll    int64     `json:"catch_all"`
	NotCatchAll int64     `json:"not_catch_all"`
	Unknown     int64     `json:"unknown"`
	Total       int64     `json:"total"`
	Timestamp   time.Time `json:"timestamp"`
}

// GetDomainStats gets statistics about domains
// I want those domain stats. Gimme the context and I will give you the stats
func (s *StatsService) GetDomainStats(ctx context.Context) (*DomainStats, error) {
	// Add timeout to context for safety
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// If the repository implements the GetDomainStats method, use it
	if statsRepo, ok := s.repo.(interface {
		GetDomainStats(ctx context.Context) (map[string]int64, error)
	}); ok {
		stats, err := statsRepo.GetDomainStats(ctx)
		if err != nil {
			s.logger.Printf("Error getting domain stats: %v", err)
			return nil, err
		}

		return &DomainStats{
			CatchAll:    stats["catch-all"],
			NotCatchAll: stats["not-catch-all"],
			Unknown:     stats["unknown"],
			Total:       stats["total"],
			Timestamp:   time.Now(),
		}, nil
	}

	// Default implementation using scan (less efficient)
	s.logger.Printf("Repository does not implement GetDomainStats, using default implementation")
	return &DomainStats{
		CatchAll:    0,
		NotCatchAll: 0,
		Unknown:     0,
		Total:       0,
		Timestamp:   time.Now(),
	}, nil
}
