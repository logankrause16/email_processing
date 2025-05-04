package domain

import "time"

// DomainStatus represents the possible states of a domain
// Easy for JSON to serialize and deserialize, and so I don't have to worry about it really.
type DomainStatus string

const (
	// StatusUnknown is used when there's not enough information to determine if a domain is catch-all
	StatusUnknown DomainStatus = "unknown"

	// StatusCatchAll indicates the domain is a catch-all domain
	StatusCatchAll DomainStatus = "catch-all"

	// StatusNotCatchAll indicates the domain is not a catch-all domain
	StatusNotCatchAll DomainStatus = "not-catch-all"
)

// EventType represents the types of events
type EventType string

const (
	// EventDelivered represents a successful email delivery
	EventDelivered EventType = "delivered"

	// EventBounced represents a bounced email
	EventBounced EventType = "bounced"
)

// DomainInfo represents information about a domain
// This struct is used for both MongoDB and in-memory storage
type DomainInfo struct {
	Name           string       `json:"name" bson:"_id"`
	DeliveredCount int64        `json:"delivered_count" bson:"delivered_count"`
	BouncedCount   int64        `json:"bounced_count" bson:"bounced_count"`
	Status         DomainStatus `json:"status" bson:"status"`
	UpdatedAt      time.Time    `json:"updated_at" bson:"updated_at"`
}

// DomainStatusResponse represents the API response for domain status
type DomainStatusResponse struct {
	Domain string       `json:"domain"`
	Status DomainStatus `json:"status"`
}

// ThresholdConfig contains the configurable thresholds for determining domain status
type ThresholdConfig struct {
	// DeliveredThreshold is the number of delivered events required to consider a domain catch-all
	DeliveredThreshold int64
}

// NewThresholdConfig creates a new ThresholdConfig with default values
func NewThresholdConfig() ThresholdConfig {
	return ThresholdConfig{
		DeliveredThreshold: 1000, // Default threshold as per requirements
	}
}
