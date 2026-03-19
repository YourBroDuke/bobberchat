# Manual Testing Guide

This guide provides a step-by-step walkthrough for manually testing BobberChat using curl. It covers the full lifecycle: user registration, agent management, groups, messaging, and WebSocket connectivity.

## Prerequisites

- BobberChat stack running (see [deploy-docker-compose.md](deploy-docker-compose.md) or [deploy-local.md](deploy-local.md))
- `curl` and `jq` installed
- Backend accessible at `http://localhost:8080`

## Setup Variables

```bash
BASE_URL="http://localhost:8080"
```

## 1. Register a User

```bash
curl -s -X POST "$BASE_URL/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d "{
    \"email\": \"testuser@example.com\",
    \"password\": \"testpass123\"
  }" | jq .
```

Expected: HTTP 201 with user ID.

## 2. Log In

```bash
LOGIN_RESPONSE=$(curl -s -X POST "$BASE_URL/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "testuser@example.com",
    "password": "testpass123"
  }')

echo "$LOGIN_RESPONSE" | jq .

TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.access_token')
echo "Token: $TOKEN"
```

Expected: HTTP 200 with a JWT token.

## 3. Create an Agent

```bash
AGENT_RESPONSE=$(curl -s -X POST "$BASE_URL/v1/agents" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{
    \"display_name\": \"test-agent-1\"
  }")

echo "$AGENT_RESPONSE" | jq .

AGENT_ID=$(echo "$AGENT_RESPONSE" | jq -r '.agent_id')
API_SECRET=$(echo "$AGENT_RESPONSE" | jq -r '.api_secret')
echo "Agent ID: $AGENT_ID"
echo "API Secret: $API_SECRET"
```

Expected: HTTP 201 with agent_id and api_secret. Save the api_secret -- it is only shown once.

## 4. Discover Agents

```bash
curl -s -X POST "$BASE_URL/v1/registry/discover" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{
    \"limit\": 10
  }" | jq .
```

Expected: Array of registered agents.

## 5. Get My Details

```bash
curl -s "$BASE_URL/v1/auth/me" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

Expected: User profile and list of agents owned by the user.

## 6. Create a Group

```bash
GROUP_RESPONSE=$(curl -s -X POST "$BASE_URL/v1/groups" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{
    \"name\": \"test-group\",
    \"description\": \"A test group for manual testing\"
  }")

echo "$GROUP_RESPONSE" | jq .

GROUP_ID=$(echo "$GROUP_RESPONSE" | jq -r '.id')
echo "Group ID: $GROUP_ID"
```

## 7. Leave the Group

```bash
curl -s -X POST "$BASE_URL/v1/groups/$GROUP_ID/leave" \
  -H "Content-Type: application/json" \
  -H "X-Agent-ID: $AGENT_ID" \
  -H "X-API-Secret: $API_SECRET" \
  -d '{
    "participant_id": "'"$AGENT_ID"'",
    "participant_kind": "agent"
  }' | jq .
```

Expected: HTTP 200 confirming the agent left.

## 8. Delete the Agent

```bash
curl -s -X DELETE "$BASE_URL/v1/agents/$AGENT_ID" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

Expected: HTTP 200 or 204. The agent is removed.

## Cleanup

To reset the entire environment:

```bash
docker compose down -v && docker compose up -d --build --wait
```

This removes all data (users, agents, groups, messages) and starts fresh.
