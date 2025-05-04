# Email Processing Service
A high-performance, scalable service for detecting catch-all email domains by analyzing email delivery events.

# Overview
This service processes and analyzes email delivery events at scale to determine whether a domain is a "catch-all" domain (one that accepts all incoming email). The determination is made based on specific business rules:

A domain is considered a catch-all when it receives more than 1000 delivered emails and has no bounced emails
A domain is considered not a catch-all when it receives at least 1 bounced email
A domain is considered unknown when it has fewer than 1000 delivered emails and no bounces
## Architecture
The system is built using Go's clean architecture principles with clear separation of concerns:

## Core Components
Domain Layer (internal/domain)
Defines core business models and rules
Contains domain entities and value objects
Defines business-specific constants and thresholds
Repository Layer (internal/repository)
Abstracts data storage using the repository pattern
Supports both MongoDB and in-memory implementations
Includes optional caching with background cache cleanup
Service Layer (internal/service)
Implements core business logic
Applies rules for determining domain status
Coordinates between repository and API layers
API Layer (internal/api)
Exposes RESTful API endpoints
Handles HTTP requests and responses
Implements routing using the Chi router
Event Processing (internal/eventprocessor)
Processes events using worker pools
Supports batch processing for higher throughput
Manages event lifecycle with recycling
Metrics Collection (pkg/metrics)
Collects performance metrics
Tracks event counts and response times
Provides statistics for monitoring
Technical Highlights
Concurrency: Uses Go's goroutines and channels for parallelism
Caching: Implements TTL-based caching with background cleanup
Batch Processing: Processes events in batches for efficiency
MongoDB Integration: Optimized for high-throughput operations
Graceful Shutdown: Proper resource cleanup on termination
API Endpoints
The service exposes these main endpoints:

```
`PUT /events/<domain_name>/delivered - Records a delivered email event`
`PUT /events/<domain_name>/bounced - Records a bounced email event`
`GET /domains/<domain_name> - Gets the status of a domain (catch-all/not-catch-all/unknown)`
`GET /domains/stats - Gets statistics about domains in the system`
`GET /metrics - Gets performance metrics of the service`
```

## Configuration
The service can be configured in multiple ways:

Default Configuration: Sensible defaults for development
Configuration File: JSON file with custom settings
Environment Variables: Override settings via environment
Command-line Flags: Runtime options for flexibility
Configuration Options
Server: Host, port, timeouts
MongoDB: Connection URI, database name, timeouts
Metrics: Collection interval, enabled flag
Business: Delivered threshold for catch-all detection
Performance Optimizations
The service includes several optimizations for high-throughput processing:

### MongoDB Optimizations:
Atomic operations for counters
Efficient indexing strategy
Bulk operations for batch processing
Connection pooling for resource efficiency
Memory Optimizations:
Object pooling to reduce allocations
TTL-based caching for hot data
Batch processing to reduce overhead
Background cleanup to manage memory
Concurrency Optimizations:
Worker pools for parallel processing
Non-blocking operations where possible
Thread-safe data structures
Context-based timeout management
Running the Service
Prerequisites
Go 1.21+
MongoDB (optional, for production use)
Starting the Service
bash
# Run with in-memory storage (for development)
go run cmd/server/main.go

# Run with MongoDB
go run cmd/server/main.go -mongodb

# Run with all optimizations
go run cmd/server/main.go -mongodb -cache -processor -workers=16
Command-line Options
-config=<path>: Path to configuration file
-mongodb: Use MongoDB for storage (instead of in-memory)
-cache: Enable in-memory caching layer
-cache-ttl=<duration>: Set cache TTL (default: 5m)
-processor: Enable background event processor
-workers=<count>: Set worker count (default: CPU count)
Development
Project Structure

```
email_processing/
├── cmd/                      # Entry points
│   ├── server/               # Main application
│   ├── loadtest/             # Load testing tool
│   └── mongobatchtest/       # MongoDB batch testing
├── internal/                 # Private application code
│   ├── api/                  # HTTP API handlers
│   ├── config/               # Configuration management
│   ├── domain/               # Domain models
│   ├── eventprocessor/       # Event processing
│   ├── repository/           # Data access layer
│   └── service/              # Business logic
├── pkg/                      # Public packages
│   ├── eventpool/            # Event generation
│   └── metrics/              # Metrics collection
└── scripts/                  # Utility scripts
```

Testing
bash
# Run all tests
go test ./...

# Run performance tests
go run cmd/loadtest/main.go -events=10000 -concurrency=8

# Run MongoDB batch tests
go run cmd/mongobatchtest/main.go
Design Decisions
This project demonstrates several key Go patterns and best practices:

Interface-based design for flexibility and testability
Repository pattern for data storage abstraction
Dependency injection for loose coupling
Context usage for timeout and cancellation
Graceful shutdown for proper resource cleanup
Worker pool pattern for concurrent processing
Decorator pattern for adding caching functionality
Options pattern for flexible configuration
Scalability
The service is designed to scale to handle high traffic:

Horizontal scaling through stateless design
MongoDB sharding support for data distribution
Efficient use of resources through batch processing
Caching to reduce database load
Configurable worker counts for CPU utilization
License
This project is proprietary and confidential.

