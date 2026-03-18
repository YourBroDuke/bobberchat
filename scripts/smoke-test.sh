#!/bin/bash
# smoke-test.sh — Verify all API endpoints on a live environment.
#
# Usage:
#   ./scripts/smoke-test.sh                          # defaults to https://staging.bobbers.cc
#   ./scripts/smoke-test.sh https://api.bobbers.cc   # run against production
#   ./scripts/smoke-test.sh http://localhost:8080     # run against local
#
# Exit code: 0 if all checks pass, 1 if any fail.
# Dependencies: curl, python3
set -e

BASE_URL="${1:-https://staging.bobbers.cc}"
PASS=0
FAIL=0
TOTAL=0
TMPDIR="${TMPDIR:-/tmp}"

check() {
  TOTAL=$((TOTAL+1))
  local desc="$1" expected_status="$2" method="$3" path="$4" data="$5"
  shift 5
  local cmd=(curl -sk -o "${TMPDIR}/smoke_body" -w '%{http_code}' -X "$method" "${BASE_URL}${path}")
  [ -n "$data" ] && cmd+=(-H "Content-Type: application/json" -d "$data")
  while [ $# -gt 0 ]; do cmd+=("$1"); shift; done

  local status=$("${cmd[@]}" 2>/dev/null)
  local body=$(cat "${TMPDIR}/smoke_body" 2>/dev/null)

  if [ "$status" = "$expected_status" ]; then
    PASS=$((PASS+1))
    echo "  ✅ [$status] $desc"
  else
    FAIL=$((FAIL+1))
    echo "  ❌ [$status] $desc (expected $expected_status)"
    echo "     Body: $(echo "$body" | head -c 200)"
  fi
}

jp() { python3 -c "import json,sys; print(json.load(sys.stdin)$1)" < "${TMPDIR}/smoke_body" 2>/dev/null; }

echo "═══════════════════════════════════════════"
echo "  SMOKE TEST: $BASE_URL"
echo "═══════════════════════════════════════════"

TS=$(date +%s)
EMAIL="smoke-${TS}@test.cc"
PASSWORD="SmokePass123!"

echo ""
echo "▸ Health & Metrics"
check "GET /v1/health" "200" "GET" "/v1/health" ""
check "GET /v1/metrics" "200" "GET" "/v1/metrics" ""

echo ""
echo "▸ Auth"
check "POST /v1/auth/register" "201" "POST" "/v1/auth/register" "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}"
USER_ID=$(jp "['id']")

check "POST /v1/auth/login" "200" "POST" "/v1/auth/login" "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}"
TOKEN=$(jp "['access_token']")

echo ""
echo "▸ Agents"
check "POST /v1/agents" "201" "POST" "/v1/agents" "{\"display_name\":\"smoke-agent-${TS}\",\"capabilities\":[\"chat\"],\"version\":\"1.0\"}" -H "Authorization: Bearer $TOKEN"
AGENT_ID=$(jp "['agent_id']")
AGENT_SECRET=$(jp "['api_secret']")

check "GET /v1/agents/{id}" "200" "GET" "/v1/agents/$AGENT_ID" "" -H "Authorization: Bearer $TOKEN"
check "POST /v1/agents/{id}/rotate-secret" "200" "POST" "/v1/agents/$AGENT_ID/rotate-secret" "{}" -H "Authorization: Bearer $TOKEN"
AGENT_SECRET=$(jp "['api_secret']")

# Create an agent to delete
check "POST /v1/agents (for delete)" "201" "POST" "/v1/agents" "{\"display_name\":\"smoke-del-${TS}\",\"capabilities\":[],\"version\":\"1.0\"}" -H "Authorization: Bearer $TOKEN"
DEL_ID=$(jp "['agent_id']")
check "DELETE /v1/agents/{id}" "200" "DELETE" "/v1/agents/$DEL_ID" "" -H "Authorization: Bearer $TOKEN"

echo ""
echo "▸ Registry"
check "GET /v1/registry/agents" "200" "GET" "/v1/registry/agents" "" -H "Authorization: Bearer $TOKEN"
check "POST /v1/registry/discover" "200" "POST" "/v1/registry/discover" "{\"capability\":\"chat\"}" -H "X-Agent-ID: $AGENT_ID" -H "X-API-Secret: $AGENT_SECRET"

echo ""
echo "▸ Groups"
check "POST /v1/groups" "201" "POST" "/v1/groups" "{\"name\":\"smoke-group-${TS}\",\"description\":\"test\",\"visibility\":\"public\"}" -H "Authorization: Bearer $TOKEN"
GROUP_ID=$(jp "['id']")
check "GET /v1/groups" "200" "GET" "/v1/groups" "" -H "Authorization: Bearer $TOKEN"

echo ""
echo "▸ Group Join/Leave"
check "POST /v1/groups/{id}/join" "200" "POST" "/v1/groups/$GROUP_ID/join" "{}" -H "X-Agent-ID: $AGENT_ID" -H "X-API-Secret: $AGENT_SECRET"
check "POST /v1/groups/{id}/leave" "200" "POST" "/v1/groups/$GROUP_ID/leave" "{}" -H "X-Agent-ID: $AGENT_ID" -H "X-API-Secret: $AGENT_SECRET"

echo ""
echo "▸ Messages"
TRACE_ID=$(python3 -c "import uuid; print(uuid.uuid4())")
check "GET /v1/messages?trace_id" "200" "GET" "/v1/messages?trace_id=$TRACE_ID" "" -H "Authorization: Bearer $TOKEN"

echo ""
echo "▸ Approvals"
check "GET /v1/approvals/pending" "200" "GET" "/v1/approvals/pending" "" -H "Authorization: Bearer $TOKEN"

echo ""
echo "▸ Adapters"
check "GET /v1/adapter" "200" "GET" "/v1/adapter" "" -H "Authorization: Bearer $TOKEN"

echo ""
echo "▸ Negative Tests"
check "No auth → 401" "401" "GET" "/v1/groups" ""
check "Duplicate email → 400" "400" "POST" "/v1/auth/register" "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}"
check "Wrong password → 401" "401" "POST" "/v1/auth/login" "{\"email\":\"$EMAIL\",\"password\":\"wrong\"}"

echo ""
echo "═══════════════════════════════════════════"
echo "  RESULTS: $PASS passed, $FAIL failed, $TOTAL total"
echo "═══════════════════════════════════════════"

[ $FAIL -eq 0 ] && exit 0 || exit 1
