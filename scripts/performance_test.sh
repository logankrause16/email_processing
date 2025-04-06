#!/bin/bash

# Performance test script for catch-all domain service
# This script tests different configurations and load levels

# Default parameters
BASE_URL="http://localhost:8080"
DURATION=60
CONCURRENT=10
EVENTS_PER_SEC=1000
TOTAL_EVENTS=10000
USE_MONGODB=false
USE_CACHING=true
USE_BATCHING=true

# Function to print usage
usage() {
  echo "Usage: $0 [options]"
  echo "Options:"
  echo "  -h, --help                 Show this help message"
  echo "  -u, --url URL              Base URL (default: $BASE_URL)"
  echo "  -d, --duration SECONDS     Test duration in seconds (default: $DURATION)"
  echo "  -c, --concurrent NUM       Concurrent connections (default: $CONCURRENT)"
  echo "  -r, --rate NUM             Events per second (default: $EVENTS_PER_SEC)"
  echo "  -e, --events NUM           Total events to process (default: $TOTAL_EVENTS)"
  echo "  -m, --mongodb              Use MongoDB (default: $USE_MONGODB)"
  echo "  --no-cache                 Disable caching (default: caching enabled)"
  echo "  --no-batch                 Disable batch processing (default: batching enabled)"
  echo "  --all-tests                Run all test configurations"
  exit 1
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)
      usage
      ;;
    -u|--url)
      BASE_URL="$2"
      shift 2
      ;;
    -d|--duration)
      DURATION="$2"
      shift 2
      ;;
    -c|--concurrent)
      CONCURRENT="$2"
      shift 2
      ;;
    -r|--rate)
      EVENTS_PER_SEC="$2"
      shift 2
      ;;
    -e|--events)
      TOTAL_EVENTS="$2"
      shift 2
      ;;
    -m|--mongodb)
      USE_MONGODB=true
      shift
      ;;
    --no-cache)
      USE_CACHING=false
      shift
      ;;
    --no-batch)
      USE_BATCHING=false
      shift
      ;;
    --all-tests)
      RUN_ALL_TESTS=true
      shift
      ;;
    *)
      echo "Unknown option: $1"
      usage
      ;;
  esac
done

# Function to run a loadtest with specific parameters
run_test() {
  local test_name="$1"
  local use_mongodb="$2"
  local use_cache="$3"
  local use_batch="$4"
  local total_events="$5"
  local concurrent="$6"
  
  echo "============================================================"
  echo "Running test: $test_name"
  echo "MongoDB: $use_mongodb, Cache: $use_cache, Batch: $use_batch"
  echo "Events: $total_events, Concurrent: $concurrent"
  echo "============================================================"
  
  # Build command options
  local mongodb_flag=""
  if [ "$use_mongodb" = "true" ]; then
    mongodb_flag="-mongodb"
  fi
  
  local cache_flag="--cache"
  if [ "$use_cache" = "false" ]; then
    cache_flag="-cache=false"
  fi
  
  local batch_flag="--batch"
  if [ "$use_batch" = "false" ]; then
    batch_flag="-batch=false"
  fi
  
  # Run the loadtest command
  go run cmd/loadtest/main.go $mongodb_flag $cache_flag $batch_flag -events="$total_events" -concurrency="$concurrent" -verbose
  
  echo ""
  echo "Test completed: $test_name"
  echo ""
  
  # Wait a bit between tests
  sleep 2
}

# Check if running all test configurations
if [ "$RUN_ALL_TESTS" = "true" ]; then
  # Test with in-memory storage, with and without optimizations
  run_test "In-memory basic" "false" "false" "false" "10000" "4"
  run_test "In-memory with cache" "false" "true" "false" "10000" "4"
  run_test "In-memory with batch" "false" "false" "true" "10000" "4"
  run_test "In-memory fully optimized" "false" "true" "true" "10000" "4"
  
  # Test with MongoDB, with and without optimizations
  run_test "MongoDB basic" "true" "false" "false" "10000" "4"
  run_test "MongoDB with cache" "true" "true" "false" "10000" "4"
  run_test "MongoDB with batch" "true" "false" "true" "10000" "4"
  run_test "MongoDB fully optimized" "true" "true" "true" "10000" "4"
  
  # Test scalability with increasing load
  run_test "Scalability test 10k" "true" "true" "true" "10000" "4"
  run_test "Scalability test 50k" "true" "true" "true" "50000" "8"
  run_test "Scalability test 100k" "true" "true" "true" "100000" "16"
else
  # Run a single test with specified parameters
  test_name="Custom test"
  if [ "$USE_MONGODB" = "true" ]; then
    test_name="${test_name} with MongoDB"
  else
    test_name="${test_name} with in-memory"
  fi
  
  if [ "$USE_CACHING" = "true" ]; then
    test_name="${test_name}, cache enabled"
  else
    test_name="${test_name}, cache disabled"
  fi
  
  if [ "$USE_BATCHING" = "true" ]; then
    test_name="${test_name}, batch enabled"
  else
    test_name="${test_name}, batch disabled"
  fi
  
  run_test "$test_name" "$USE_MONGODB" "$USE_CACHING" "$USE_BATCHING" "$TOTAL_EVENTS" "$CONCURRENT"
fi

echo "All tests completed!"