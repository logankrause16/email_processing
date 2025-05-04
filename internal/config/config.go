package config

import (
	"encoding/json"
	"os"
	"strconv"
	"time"
)

// Lets hold all those fancy configs in one place. To use everything we need
// to pass them around like a hot potato. But the potato is a config, and I like
// sour cream and green onions on my potato. With probably too much cheese and butter.

// Config holds all configuration for the application
type Config struct {
	Server   ServerConfig   `json:"server"`
	MongoDB  MongoDBConfig  `json:"mongodb"`
	Metrics  MetricsConfig  `json:"metrics"`
	Business BusinessConfig `json:"business"`
}

// ServerConfig holds configuration for the HTTP server
type ServerConfig struct {
	Host            string        `json:"host"`
	Port            int           `json:"port"`
	ReadTimeout     time.Duration `json:"read_timeout"`
	WriteTimeout    time.Duration `json:"write_timeout"`
	IdleTimeout     time.Duration `json:"idle_timeout"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
}

// MongoDBConfig holds configuration for MongoDB
type MongoDBConfig struct {
	URI              string        `json:"uri"`
	Database         string        `json:"database"`
	Collection       string        `json:"collection"`
	ConnectTimeout   time.Duration `json:"connect_timeout"`
	OperationTimeout time.Duration `json:"operation_timeout"`
}

// MetricsConfig holds configuration for metrics collection
type MetricsConfig struct {
	Enabled            bool          `json:"enabled"`
	CollectionInterval time.Duration `json:"collection_interval"`
}

// BusinessConfig holds business logic configuration
// Defines # of delivered events required to consider a domain "catch-all"
// This is a business-specific configuration and can be adjusted as needed, default is 1,000
type BusinessConfig struct {
	DeliveredThreshold int64 `json:"delivered_threshold"`
}

// LoadConfig loads configuration from a file
func LoadConfig(filename string) (*Config, error) {
	// Default configuration
	config := &Config{
		Server: ServerConfig{
			Host:            "0.0.0.0",
			Port:            8080,
			ReadTimeout:     5 * time.Second,
			WriteTimeout:    10 * time.Second,
			IdleTimeout:     120 * time.Second,
			ShutdownTimeout: 10 * time.Second,
		},
		MongoDB: MongoDBConfig{
			URI:              "mongodb://localhost:27017",
			Database:         "catchall",
			Collection:       "domains",
			ConnectTimeout:   30 * time.Second,
			OperationTimeout: 5 * time.Second,
		},
		Metrics: MetricsConfig{
			Enabled:            true,
			CollectionInterval: 15 * time.Second,
		},
		Business: BusinessConfig{
			DeliveredThreshold: 1000,
		},
	}

	// If filename is provided, load from file
	if filename != "" {
		file, err := os.Open(filename)
		if err != nil {
			return nil, err
		}
		defer file.Close()

		decoder := json.NewDecoder(file)
		if err := decoder.Decode(config); err != nil {
			return nil, err
		}
	}

	// Override with environment variables if set
	if host := os.Getenv("SERVER_HOST"); host != "" {
		config.Server.Host = host
	}

	if port := os.Getenv("SERVER_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			config.Server.Port = p
		}
	}

	if mongoURI := os.Getenv("MONGODB_URI"); mongoURI != "" {
		config.MongoDB.URI = mongoURI
	}

	if mongoDatabase := os.Getenv("MONGODB_DATABASE"); mongoDatabase != "" {
		config.MongoDB.Database = mongoDatabase
	}

	if threshold := os.Getenv("DELIVERED_THRESHOLD"); threshold != "" {
		if t, err := strconv.ParseInt(threshold, 10, 64); err == nil {
			config.Business.DeliveredThreshold = t
		}
	}

	return config, nil
}
