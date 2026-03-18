#!/usr/bin/env bash

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"

PASS=0
FAIL=0
TOTAL=31

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

print_step "1/31 Health check"
request "GET" "/v1/health"
assert_step "Health check" 200 "$LAST_STATUS" '"status":"ok"'

print_step "2/31 Register user"
request "POST" "/v1/auth/register" "{\"email\":\"test@example.com\",\"password\":\"testpass123\"}"
assert_step "Register user" 201 "$LAST_STATUS" '"email":"test@example.com"'

print_step "-- Verify email (extract token from console email logs)"
sleep 1
VERIFY_TOKEN="$(docker compose logs bobberd 2>/dev/null | grep -o 'token=[a-f0-9]*' | tail -1 | cut -d= -f2 || true)"
if [ -n "$VERIFY_TOKEN" ]; then
  request "POST" "/v1/auth/verify-email" "{\"token\":\"$VERIFY_TOKEN\"}"
  if [ "$LAST_STATUS" -eq 200 ]; then
    echo "  OK: Email verified (HTTP $LAST_STATUS)"
  else
    echo "  WARN: Email verification failed (HTTP $LAST_STATUS) — login may fail"
  fi
else
  echo "  WARN: Could not extract verification token from logs — login may fail"
fi

print_step "3/31 Login"
request "POST" "/v1/auth/login" "{\"email\":\"test@example.com\",\"password\":\"testpass123\"}"
TOKEN="$(echo "$LAST_BODY" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4 || true)"
if [ "$LAST_STATUS" -eq 200 ] && [ -n "$TOKEN" ]; then
  echo "  PASS: Login (HTTP $LAST_STATUS)"
  PASS=$((PASS + 1))
else
  echo "  FAIL: Login (expected 200 with access_token, got HTTP $LAST_STATUS)"
  FAIL=$((FAIL + 1))
fi

print_step "4/31 Create agent"
request "POST" "/v1/agents" "{\"display_name\":\"test-agent\",\"capabilities\":[\"test\"]}" "$TOKEN"
AGENT_ID="$(echo "$LAST_BODY" | grep -o '"agent_id":"[^"]*"' | cut -d'"' -f4 || true)"
API_SECRET="$(echo "$LAST_BODY" | grep -o '"api_secret":"[^"]*"' | cut -d'"' -f4 || true)"
if [ "$LAST_STATUS" -eq 201 ] && [ -n "$AGENT_ID" ] && [ -n "$API_SECRET" ] && [[ "$LAST_BODY" == *'"status":"REGISTERED"'* ]]; then
  echo "  PASS: Create agent (HTTP $LAST_STATUS)"
  PASS=$((PASS + 1))
else
  echo "  FAIL: Create agent (expected 201 with agent_id, api_secret, and status, got HTTP $LAST_STATUS)"
  FAIL=$((FAIL + 1))
fi

print_step "5/31 Get agent"
request "GET" "/v1/agents/$AGENT_ID" "" "$TOKEN"
assert_step "Get agent" 200 "$LAST_STATUS" '"display_name":"test-agent"'

print_step "6/31 List agents"
request "GET" "/v1/registry/agents" "" "$TOKEN"
assert_step "List agents" 200 "$LAST_STATUS" '"agents"'

print_step "7/31 Discover agents"
request "POST" "/v1/registry/discover" "{\"capability\":\"test\",\"status\":[\"REGISTERED\"],\"limit\":10}" "$TOKEN"
assert_step "Discover" 200 "$LAST_STATUS" '"agents"'

print_step "8/31 Create group"
request "POST" "/v1/groups" "{\"name\":\"test-group\",\"description\":\"e2e test\",\"visibility\":\"private\"}" "$TOKEN"
GROUP_ID="$(echo "$LAST_BODY" | grep -o '"id":"[^"]*"' | cut -d'"' -f4 || true)"
if [ "$LAST_STATUS" -eq 201 ] && [ -n "$GROUP_ID" ] && [[ "$LAST_BODY" == *'"name":"test-group"'* ]]; then
  echo "  PASS: Create group (HTTP $LAST_STATUS)"
  PASS=$((PASS + 1))
else
  echo "  FAIL: Create group (expected 201 with group id and name, got HTTP $LAST_STATUS)"
  FAIL=$((FAIL + 1))
fi

print_step "9/31 List groups"
request "GET" "/v1/groups" "" "$TOKEN"
assert_step "List groups" 200 "$LAST_STATUS" '"groups"'

print_step "10/31 Join group"
request "POST" "/v1/groups/$GROUP_ID/join" "{}" "$TOKEN"
assert_status "Join group" 200 "$LAST_STATUS"

print_step "11/31 Create topic"
request "POST" "/v1/groups/$GROUP_ID/topics" "{\"subject\":\"e2e test topic\"}" "$TOKEN"
assert_step "Create topic" 201 "$LAST_STATUS" '"subject":"e2e test topic"'

print_step "12/31 List topics"
request "GET" "/v1/groups/$GROUP_ID/topics" "" "$TOKEN"
assert_step "List topics" 200 "$LAST_STATUS" '"topics"'

print_step "13/31 Get pending approvals"
request "GET" "/v1/approvals/pending" "" "$TOKEN"
assert_step "Get pending approvals" 200 "$LAST_STATUS" '"approvals"'

print_step "14/31 Metrics"
request "GET" "/v1/metrics"
assert_status "Metrics" 200 "$LAST_STATUS"

# NEGATIVE TEST CASES

print_step "15/31 Register duplicate email"
request "POST" "/v1/auth/register" "{\"email\":\"test@example.com\",\"password\":\"different123\"}"
assert_status "Register duplicate email" 400 "$LAST_STATUS"

print_step "16/31 Login wrong password"
request "POST" "/v1/auth/login" "{\"email\":\"test@example.com\",\"password\":\"wrongpass123\"}"
assert_status "Login wrong password" 401 "$LAST_STATUS"

print_step "17/31 Login non-existent user"
request "POST" "/v1/auth/login" "{\"email\":\"nonexistent@example.com\",\"password\":\"testpass123\"}"
assert_status "Login non-existent user" 401 "$LAST_STATUS"

print_step "18/31 Create agent no auth"
request "POST" "/v1/agents" "{\"display_name\":\"test-agent\",\"capabilities\":[\"test\"]}"
assert_status "Create agent no auth" 401 "$LAST_STATUS"

print_step "19/31 Get agent not found"
request "GET" "/v1/agents/00000000-0000-0000-0000-000000000000" "" "$TOKEN"
assert_status "Get agent not found" 404 "$LAST_STATUS"

print_step "20/31 List agents no auth"
request "GET" "/v1/registry/agents"
assert_status "List agents no auth" 401 "$LAST_STATUS"

print_step "21/31 Create group no auth"
request "POST" "/v1/groups" "{\"name\":\"test-group\",\"description\":\"e2e test\",\"visibility\":\"private\"}"
assert_status "Create group no auth" 401 "$LAST_STATUS"

print_step "22/31 Get pending approvals no auth"
request "GET" "/v1/approvals/pending"
assert_status "Get pending approvals no auth" 401 "$LAST_STATUS"

print_step "23/31 Adapter ingest unknown adapter"
request "POST" "/v1/adapter/unknown/ingest" "{\"test\":true}" "$TOKEN"
assert_status "Adapter ingest unknown" 404 "$LAST_STATUS"

print_step "24/31 Adapter list no auth"
request "GET" "/v1/adapter"
assert_status "Adapter list no auth" 401 "$LAST_STATUS"

print_step "25/31 Adapter ingest empty body"
body_file="$(mktemp)"
LAST_STATUS="$(curl -s -o "$body_file" -w "%{http_code}" -X POST "$BASE_URL/v1/adapter/mcp/ingest" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d "")"
LAST_BODY="$(<"$body_file")"
rm -f "$body_file"
assert_status "Adapter ingest empty body" 400 "$LAST_STATUS"

print_step "26/31 Messages missing trace_id"
request "GET" "/v1/messages" "" "$TOKEN"
assert_status "Messages missing trace_id" 400 "$LAST_STATUS"

print_step "27/31 Messages no auth"
request "GET" "/v1/messages"
assert_status "Messages no auth" 401 "$LAST_STATUS"

print_step "28/31 Discover no auth"
request "POST" "/v1/registry/discover" "{\"capability\":\"test\",\"status\":[\"REGISTERED\"],\"limit\":10}"
assert_status "Discover no auth" 401 "$LAST_STATUS"

print_step "29/31 Adapter list with auth"
request "GET" "/v1/adapter" "" "$TOKEN"
assert_status "Adapter list with auth" 200 "$LAST_STATUS"

# CLEANUP (moved to end)

print_step "30/31 Leave group"
request "POST" "/v1/groups/$GROUP_ID/leave" "{}" "$TOKEN"
assert_status "Leave group" 200 "$LAST_STATUS"

print_step "31/31 Delete agent"
request "DELETE" "/v1/agents/$AGENT_ID" "" "$TOKEN"
assert_status "Delete agent" 200 "$LAST_STATUS"

echo
echo "$PASS/$TOTAL tests passed"

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
