#!/usr/bin/env bash

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
TENANT_ID="550e8400-e29b-41d4-a716-446655440000"

PASS=0
FAIL=0
TOTAL=16

LAST_STATUS=""
LAST_BODY=""

print_step() {
  local message="$1"
  echo
  echo "==> $message"
}

assert_step() {
  local name="$1"
  local expected_status="$2"
  local actual_status="$3"
  local required_substring="${4:-}"

  if [ "$actual_status" -ne "$expected_status" ]; then
    echo "  FAIL: $name (expected $expected_status, got $actual_status)"
    FAIL=$((FAIL + 1))
    return
  fi

  if [ -n "$required_substring" ] && [[ "$LAST_BODY" != *"$required_substring"* ]]; then
    echo "  FAIL: $name (HTTP $actual_status, missing '$required_substring')"
    FAIL=$((FAIL + 1))
    return
  fi

  echo "  PASS: $name (HTTP $actual_status)"
  PASS=$((PASS + 1))
}

assert_status() {
  local name="$1"
  local expected="$2"
  local actual="$3"
  if [ "$actual" -eq "$expected" ]; then
    echo "  PASS: $name (HTTP $actual)"
    PASS=$((PASS+1))
  else
    echo "  FAIL: $name (expected $expected, got $actual)"
    FAIL=$((FAIL+1))
  fi
}

request() {
  local method="$1"
  local path="$2"
  local body="${3:-}"
  local token="${4:-}"

  local body_file
  body_file="$(mktemp)"

  if [ -n "$token" ] && [ -n "$body" ]; then
    LAST_STATUS="$(curl -s -o "$body_file" -w "%{http_code}" -X "$method" "$BASE_URL$path" -H "Authorization: Bearer $token" -H "Content-Type: application/json" -d "$body")"
  elif [ -n "$token" ]; then
    LAST_STATUS="$(curl -s -o "$body_file" -w "%{http_code}" -X "$method" "$BASE_URL$path" -H "Authorization: Bearer $token")"
  elif [ -n "$body" ]; then
    LAST_STATUS="$(curl -s -o "$body_file" -w "%{http_code}" -X "$method" "$BASE_URL$path" -H "Content-Type: application/json" -d "$body")"
  else
    LAST_STATUS="$(curl -s -o "$body_file" -w "%{http_code}" -X "$method" "$BASE_URL$path")"
  fi

  LAST_BODY="$(<"$body_file")"
  rm -f "$body_file"
}

echo "Running BobberChat E2E API checks against: $BASE_URL"

print_step "1/16 Health check"
request "GET" "/v1/health"
assert_step "Health check" 200 "$LAST_STATUS" '"status":"ok"'

print_step "2/16 Register user"
request "POST" "/v1/auth/register" "{\"tenant_id\":\"$TENANT_ID\",\"email\":\"test@example.com\",\"password\":\"testpass123\"}"
assert_status "Register user" 201 "$LAST_STATUS"

print_step "3/16 Login"
request "POST" "/v1/auth/login" "{\"email\":\"test@example.com\",\"password\":\"testpass123\"}"
TOKEN="$(echo "$LAST_BODY" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4 || true)"
if [ "$LAST_STATUS" -eq 200 ] && [ -n "$TOKEN" ]; then
  echo "  PASS: Login (HTTP $LAST_STATUS)"
  PASS=$((PASS + 1))
else
  echo "  FAIL: Login (expected 200 with access_token, got HTTP $LAST_STATUS)"
  FAIL=$((FAIL + 1))
fi

print_step "4/16 Create agent"
request "POST" "/v1/agents" "{\"display_name\":\"test-agent\",\"capabilities\":[\"test\"],\"version\":\"1.0.0\"}" "$TOKEN"
AGENT_ID="$(echo "$LAST_BODY" | grep -o '"agent_id":"[^"]*"' | cut -d'"' -f4 || true)"
API_SECRET="$(echo "$LAST_BODY" | grep -o '"api_secret":"[^"]*"' | cut -d'"' -f4 || true)"
if [ "$LAST_STATUS" -eq 201 ] && [ -n "$AGENT_ID" ] && [ -n "$API_SECRET" ]; then
  echo "  PASS: Create agent (HTTP $LAST_STATUS)"
  PASS=$((PASS + 1))
else
  echo "  FAIL: Create agent (expected 201 with agent_id and api_secret, got HTTP $LAST_STATUS)"
  FAIL=$((FAIL + 1))
fi

print_step "5/16 Get agent"
request "GET" "/v1/agents/$AGENT_ID" "" "$TOKEN"
assert_status "Get agent" 200 "$LAST_STATUS"

print_step "6/16 List agents"
request "GET" "/v1/registry/agents" "" "$TOKEN"
assert_status "List agents" 200 "$LAST_STATUS"

print_step "7/16 Discover agents"
request "POST" "/v1/registry/discover" "{\"capability\":\"test\",\"status\":[\"REGISTERED\"],\"limit\":10}" "$TOKEN"
assert_status "Discover" 200 "$LAST_STATUS"

print_step "8/16 Create group"
request "POST" "/v1/groups" "{\"name\":\"test-group\",\"description\":\"e2e test\",\"visibility\":\"private\"}" "$TOKEN"
GROUP_ID="$(echo "$LAST_BODY" | grep -o '"id":"[^"]*"' | cut -d'"' -f4 || true)"
if [ "$LAST_STATUS" -eq 201 ] && [ -n "$GROUP_ID" ]; then
  echo "  PASS: Create group (HTTP $LAST_STATUS)"
  PASS=$((PASS + 1))
else
  echo "  FAIL: Create group (expected 201 with group id, got HTTP $LAST_STATUS)"
  FAIL=$((FAIL + 1))
fi

print_step "9/16 List groups"
request "GET" "/v1/groups" "" "$TOKEN"
assert_status "List groups" 200 "$LAST_STATUS"

print_step "10/16 Join group"
request "POST" "/v1/groups/$GROUP_ID/join" "{}" "$TOKEN"
assert_status "Join group" 200 "$LAST_STATUS"

print_step "11/16 Create topic"
request "POST" "/v1/groups/$GROUP_ID/topics" "{\"subject\":\"e2e test topic\"}" "$TOKEN"
assert_status "Create topic" 201 "$LAST_STATUS"

print_step "12/16 List topics"
request "GET" "/v1/groups/$GROUP_ID/topics" "" "$TOKEN"
assert_status "List topics" 200 "$LAST_STATUS"

print_step "13/16 Get pending approvals"
request "GET" "/v1/approvals/pending" "" "$TOKEN"
assert_status "Get pending approvals" 200 "$LAST_STATUS"

print_step "14/16 Metrics"
request "GET" "/v1/metrics"
assert_status "Metrics" 200 "$LAST_STATUS"

print_step "15/16 Leave group"
request "POST" "/v1/groups/$GROUP_ID/leave" "{}" "$TOKEN"
assert_status "Leave group" 200 "$LAST_STATUS"

print_step "16/16 Delete agent"
request "DELETE" "/v1/agents/$AGENT_ID" "" "$TOKEN"
assert_status "Delete agent" 200 "$LAST_STATUS"

echo
echo "$PASS/$TOTAL tests passed"

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
