package service

import (
	"context"
	"log"
	"time"

	"email_processing/internal/domain"
	"email_processing/internal/repository"
)

// DomainService provides operations for managing domains
type DomainService struct {
	repo            repository.DomainRepository
	thresholdConfig domain.ThresholdConfig
	logger          *log.Logger
}

// NewDomainService creates a new domain service
func NewDomainService(repo repository.DomainRepository, logger *log.Logger) *DomainService {
	return &DomainService{
		repo:            repo,
		thresholdConfig: domain.NewThresholdConfig(),
		logger:          logger,
	}
}

// GetDomainStatus retrieves the status of a domain
func (s *DomainService) GetDomainStatus(ctx context.Context, domainName string) (*domain.DomainStatusResponse, error) {
	// Add timeout to context for safety
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Get domain info
	domainInfo, err := s.repo.GetDomainInfo(ctx, domainName)
	if err != nil {
		s.logger.Printf("Error getting domain info for %s: %v", domainName, err)
		return nil, err
	}

	// Check and potentially update status based on latest counts
	status := s.determineDomainStatus(domainInfo)

	// Update status if it has changed
	if status != domainInfo.Status {
		if err := s.repo.UpdateDomainStatus(ctx, domainName, status); err != nil {
			s.logger.Printf("Error updating domain status for %s: %v", domainName, err)
			// Continue with the correct status even if update fails
		}
	}

	return &domain.DomainStatusResponse{
		Domain: domainName,
		Status: status,
	}, nil
}

// RecordEvent records an event for a domain
func (s *DomainService) RecordEvent(ctx context.Context, domainName string, eventType domain.EventType) error {
	// Add timeout to context for safety
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Increment the event count
	if err := s.repo.IncrementEventCount(ctx, domainName, eventType); err != nil {
		s.logger.Printf("Error incrementing event count for %s: %v", domainName, err)
		return err
	}

	// Get the updated domain info to determine the new status
	domainInfo, err := s.repo.GetDomainInfo(ctx, domainName)
	if err != nil {
		s.logger.Printf("Error getting domain info after event for %s: %v", domainName, err)
		return err
	}

	// Determine and update status if needed
	status := s.determineDomainStatus(domainInfo)
	if status != domainInfo.Status {
		if err := s.repo.UpdateDomainStatus(ctx, domainName, status); err != nil {
			s.logger.Printf("Error updating domain status after event for %s: %v", domainName, err)
			return err
		}
	}

	return nil
}

// determineDomainStatus determines the status of a domain based on its event counts
func (s *DomainService) determineDomainStatus(info *domain.DomainInfo) domain.DomainStatus {
	// If there's at least one bounce, the domain is not catch-all
	if info.BouncedCount > 0 {
		return domain.StatusNotCatchAll
	}

	// If delivered count is above threshold, domain is catch-all
	if info.DeliveredCount >= s.thresholdConfig.DeliveredThreshold {
		return domain.StatusCatchAll
	}

	// Otherwise, there's not enough information
	return domain.StatusUnknown
}
