package service

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/logankrause16/email_processing/internal/domain"
	"github.com/logankrause16/email_processing/internal/repository"
)

func TestDomainService_GetDomainStatus(t *testing.T) {
	// Setup
	logger := log.New(os.Stdout, "TEST: ", log.LstdFlags)
	repo := repository.NewInMemoryDomainRepository()
	service := NewDomainService(repo, logger)
	ctx := context.Background()

	// Test cases
	tests := []struct {
		name           string
		domainName     string
		deliveredCount int64
		bouncedCount   int64
		expectedStatus domain.DomainStatus
	}{
		{
			name:           "Unknown domain with no events",
			domainName:     "unknown.example.com",
			deliveredCount: 0,
			bouncedCount:   0,
			expectedStatus: domain.StatusUnknown,
		},
		{
			name:           "Catch-all domain with many delivered events",
			domainName:     "catchall.example.com",
			deliveredCount: 1500,
			bouncedCount:   0,
			expectedStatus: domain.StatusCatchAll,
		},
		{
			name:           "Not catch-all domain with bounced events",
			domainName:     "notcatchall.example.com",
			deliveredCount: 2000,
			bouncedCount:   5,
			expectedStatus: domain.StatusNotCatchAll,
		},
		{
			name:           "Not catch-all domain with bounced events but few delivered",
			domainName:     "notcatchall2.example.com",
			deliveredCount: 500,
			bouncedCount:   1,
			expectedStatus: domain.StatusNotCatchAll,
		},
		{
			name:           "Domain just below threshold",
			domainName:     "borderline.example.com",
			deliveredCount: 999,
			bouncedCount:   0,
			expectedStatus: domain.StatusUnknown,
		},
		{
			name:           "Domain exactly at threshold",
			domainName:     "borderline2.example.com",
			deliveredCount: 1000,
			bouncedCount:   0,
			expectedStatus: domain.StatusCatchAll,
		},
	}

	// Run tests
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup domain with initial counts
			for i := int64(0); i < tc.deliveredCount; i++ {
				err := repo.IncrementEventCount(ctx, tc.domainName, domain.EventDelivered)
				if err != nil {
					t.Fatalf("Failed to increment delivered count: %v", err)
				}
			}

			for i := int64(0); i < tc.bouncedCount; i++ {
				err := repo.IncrementEventCount(ctx, tc.domainName, domain.EventBounced)
				if err != nil {
					t.Fatalf("Failed to increment bounced count: %v", err)
				}
			}

			// Get domain status
			result, err := service.GetDomainStatus(ctx, tc.domainName)
			if err != nil {
				t.Fatalf("Failed to get domain status: %v", err)
			}

			// Check result
			if result.Status != tc.expectedStatus {
				t.Errorf("Expected status %s, got %s", tc.expectedStatus, result.Status)
			}

			if result.Domain != tc.domainName {
				t.Errorf("Expected domain name %s, got %s", tc.domainName, result.Domain)
			}
		})
	}
}

func TestDomainService_RecordEvent(t *testing.T) {
	// Setup
	logger := log.New(os.Stdout, "TEST: ", log.LstdFlags)
	repo := repository.NewInMemoryDomainRepository()
	service := NewDomainService(repo, logger)
	ctx := context.Background()

	// Test cases
	tests := []struct {
		name              string
		domainName        string
		events            []domain.EventType
		expectedStatus    domain.DomainStatus
		expectedDelivered int64
		expectedBounced   int64
	}{
		{
			name:              "Record delivered events only",
			domainName:        "delivered.example.com",
			events:            []domain.EventType{domain.EventDelivered, domain.EventDelivered, domain.EventDelivered},
			expectedStatus:    domain.StatusUnknown,
			expectedDelivered: 3,
			expectedBounced:   0,
		},
		{
			name:              "Record both delivered and bounced events",
			domainName:        "mixed.example.com",
			events:            []domain.EventType{domain.EventDelivered, domain.EventDelivered, domain.EventBounced},
			expectedStatus:    domain.StatusNotCatchAll,
			expectedDelivered: 2,
			expectedBounced:   1,
		},
		{
			name:              "Record enough delivered events to be catch-all",
			domainName:        "many.example.com",
			events:            makeEvents(domain.EventDelivered, 1000),
			expectedStatus:    domain.StatusCatchAll,
			expectedDelivered: 1000,
			expectedBounced:   0,
		},
		{
			name:              "Record enough delivered events to be catch-all then a bounce",
			domainName:        "manythenbounce.example.com",
			events:            append(makeEvents(domain.EventDelivered, 1000), domain.EventBounced),
			expectedStatus:    domain.StatusNotCatchAll,
			expectedDelivered: 1000,
			expectedBounced:   1,
		},
	}

	// Run tests
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Record events
			for _, eventType := range tc.events {
				err := service.RecordEvent(ctx, tc.domainName, eventType)
				if err != nil {
					t.Fatalf("Failed to record event: %v", err)
				}
			}

			// Get domain info to verify counts
			domainInfo, err := repo.GetDomainInfo(ctx, tc.domainName)
			if err != nil {
				t.Fatalf("Failed to get domain info: %v", err)
			}

			// Check counts
			if domainInfo.DeliveredCount != tc.expectedDelivered {
				t.Errorf("Expected delivered count %d, got %d", tc.expectedDelivered, domainInfo.DeliveredCount)
			}

			if domainInfo.BouncedCount != tc.expectedBounced {
				t.Errorf("Expected bounced count %d, got %d", tc.expectedBounced, domainInfo.BouncedCount)
			}

			// Check status
			if domainInfo.Status != tc.expectedStatus {
				t.Errorf("Expected status %s, got %s", tc.expectedStatus, domainInfo.Status)
			}
		})
	}
}

// Helper function to create many events of the same type
func makeEvents(eventType domain.EventType, count int64) []domain.EventType {
	events := make([]domain.EventType, count)
	for i := int64(0); i < count; i++ {
		events[i] = eventType
	}
	return events
}
