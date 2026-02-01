#!/bin/bash

# Test script for the updated proxy creation API

TOKEN="8f683f3b0c2291e05221b66ed1859aa3d4fc891a66a0de2a45bdb1ce49d882d15"
BASE_URL="http://localhost:8021"

echo "Testing proxy creation with the updated API..."
echo ""

# First, login to get a session cookie
echo "Step 1: Logging in..."
COOKIE_JAR=$(mktemp)
curl -s -c "$COOKIE_JAR" -X POST "$BASE_URL/api/login" \
  -H "Content-Type: application/json" \
  -d "{\"password\": \"$TOKEN\"}" > /dev/null

echo "Step 2: Creating a test proxy..."
RESPONSE=$(curl -s -b "$COOKIE_JAR" -X POST "$BASE_URL/api/caddy/create" \
  -H "Content-Type: application/json" \
  -d '{
    "hostname": "tailrelay-dev.koi-great.ts.net",
    "port": 8083,
    "target": "http://whoami-test:80",
    "tls": true,
    "trusted_proxies": true,
    "enabled": true
  }')

echo "Response:"
echo "$RESPONSE" | jq . 2>/dev/null || echo "$RESPONSE"
echo ""

# Check logs for any errors
echo "Step 3: Checking container logs for errors..."
docker logs tailrelay-test 2>&1 | grep -E "(ERROR|Failed|error)" | tail -5

rm -f "$COOKIE_JAR"
