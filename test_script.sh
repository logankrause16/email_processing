#!/bin/bash

# Test a catch-all domain (>1000 delivered, no bounces)
echo "Testing catch-all domain detection..."
for i in {1..1001}; do
  curl -s -X PUT http://localhost:8081/events/catchall-test.com/delivered > /dev/null
  if [ $((i % 100)) -eq 0 ]; then
    echo "Sent $i delivered events"
  fi
done
echo "Status for catchall-test.com:"
curl -s http://localhost:8081/domains/catchall-test.com | jq .

# Test non-catch-all domain (with bounces)
echo -e "\nTesting non-catch-all domain detection..."
for i in {1..500}; do
  curl -s -X PUT http://localhost:8081/events/non-catchall-test.com/delivered > /dev/null
done
echo "Sent 500 delivered events"
curl -s -X PUT http://localhost:8081/events/non-catchall-test.com/bounced > /dev/null
echo "Sent 1 bounced event"
echo "Status for non-catchall-test.com:"
curl -s http://localhost:8081/domains/non-catchall-test.com | jq .

# Test unknown domain (<1000 delivered, no bounces)
echo -e "\nTesting unknown domain status..."
for i in {1..500}; do
  curl -s -X PUT http://localhost:8081/events/unknown-test.com/delivered > /dev/null
done
echo "Sent 500 delivered events"
echo "Status for unknown-test.com:"
curl -s http://localhost:8081/domains/unknown-test.com | jq .
