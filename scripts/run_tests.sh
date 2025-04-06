#!/bin/bash

# Script to run all tests for the catch-all domain service

# First, run unit tests
echo "Running unit tests..."
go test ./... -v

# MongoDB batch test
echo "Running MongoDB batch test..."
go run cmd/mongobatchtest/main.go --uri=mongodb://localhost:27017 --db=catchall_test --mode=both --domains=100 --batch-size=100

# Basic load test with in-memory storage
echo "Running basic load test with in-memory storage..."
go run cmd/loadtest/main.go -events=1000 -concurrency=4 -verbose

# Load test with MongoDB
echo "Running load test with MongoDB..."
go run cmd/loadtest/main.go -mongodb -cache -batch -events=10000 -concurrency=8 -verbose

echo "All tests completed!"