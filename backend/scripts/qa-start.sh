#!/usr/bin/env bash
# qa-start.sh — Spin up local QA environment and print browser injection snippet
#
# Usage:
#   ./scripts/qa-start.sh                   # Interactive: starts backend, creates user
#   ./scripts/qa-start.sh --user-only       # Skip backend start, just create a new user
#   ./scripts/qa-start.sh --ttl 2h          # Custom token TTL (default: 8h)
#   ./scripts/qa-start.sh --name "Jane" --email "jane@toqui-test.local"
#
# Prerequisites:
#   - Docker running (for postgres + firestore emulator)
#   - gcloud auth application-default login (for GCP Secret Manager)
#   - Run from toqui-backend repo root

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Defaults
USER_ONLY=false
TTL="8h"
USER_NAME="QA User"
USER_EMAIL="qa-$(date +%s)@toqui-test.local"
BACKEND_PORT=8090
FRONTEND_PORT=8081

# Parse args
while [[ $# -gt 0 ]]; do
  case "$1" in
    --user-only) USER_ONLY=true; shift ;;
    --ttl) TTL="$2"; shift 2 ;;
    --name) USER_NAME="$2"; shift 2 ;;
    --email) USER_EMAIL="$2"; shift 2 ;;
    *) echo "Unknown arg: $1"; exit 1 ;;
  esac
done

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

log_step() { echo -e "${CYAN}▶ $1${NC}"; }
log_ok()   { echo -e "${GREEN}✓ $1${NC}"; }
log_warn() { echo -e "${YELLOW}⚠ $1${NC}"; }
log_err()  { echo -e "${RED}✗ $1${NC}"; }

echo -e "${BOLD}═══════════════════════════════════════${NC}"
echo -e "${BOLD}       Toqui Local QA Setup             ${NC}"
echo -e "${BOLD}═══════════════════════════════════════${NC}"
echo ""

cd "$REPO_ROOT"

# ─── 1. Check Docker containers ─────────────────────────────────────────────
log_step "Checking Docker infra..."

POSTGRES_UP=$(docker ps --filter "name=toqui-backend-postgres-1" --filter "status=running" --format "{{.Names}}" 2>/dev/null || true)
FIRESTORE_UP=$(docker ps --filter "name=toqui-backend-firestore-1" --filter "status=running" --format "{{.Names}}" 2>/dev/null || true)

if [[ -z "$POSTGRES_UP" || -z "$FIRESTORE_UP" ]]; then
  log_warn "Docker containers not running. Starting..."
  make docker-up
  sleep 3
  make migrate-up
  log_ok "Docker infra ready"
else
  log_ok "Docker containers running (postgres + firestore)"
fi

# ─── 2. Start backend (unless --user-only) ───────────────────────────────────
if [[ "$USER_ONLY" == false ]]; then
  HEALTH=$(curl -sf "http://localhost:${BACKEND_PORT}/healthz" 2>/dev/null || true)
  if echo "$HEALTH" | grep -q '"status":"ok"' 2>/dev/null; then
    log_ok "Backend already running on :${BACKEND_PORT}"
  else
    log_step "Starting backend on :${BACKEND_PORT}..."
    log_warn "Run this in a separate terminal:"
    echo ""
    echo -e "  ${BOLD}cd $(pwd) && make run${NC}"
    echo ""
    echo "Waiting for backend to be ready (Ctrl+C to skip)..."
    for i in $(seq 1 30); do
      HEALTH=$(curl -sf "http://localhost:${BACKEND_PORT}/healthz" 2>/dev/null || true)
      if echo "$HEALTH" | grep -q '"status":"ok"' 2>/dev/null; then
        log_ok "Backend ready"
        break
      fi
      if [[ $i -eq 30 ]]; then
        log_warn "Backend not responding after 30s. Continuing anyway..."
        log_warn "Make sure to start it: cd $(pwd) && make run"
      fi
      sleep 1
    done
  fi
fi

# ─── 3. Check frontend ────────────────────────────────────────────────────────
FRONTEND_UP=$(curl -sf "http://localhost:${FRONTEND_PORT}" 2>/dev/null || true)
if [[ -n "$FRONTEND_UP" ]]; then
  log_ok "Frontend already running on :${FRONTEND_PORT}"
else
  log_warn "Frontend not running on :${FRONTEND_PORT}"
  log_warn "Start it in another terminal:"
  echo ""
  TOQUI_PATH="$(cd "$REPO_ROOT/../toqui" 2>/dev/null && pwd || echo '../toqui')"
  echo -e "  ${BOLD}cd $TOQUI_PATH && EXPO_PUBLIC_API_URL=http://localhost:8090 pnpm web${NC}"
  echo ""
fi

# ─── 4. Create test user ─────────────────────────────────────────────────────
log_step "Creating test user: ${USER_NAME} <${USER_EMAIL}>"

OUTPUT=$(go run ./cmd/testctl create-user \
  --name "$USER_NAME" \
  --email "$USER_EMAIL" \
  --ttl "$TTL" 2>&1)

USER_ID=$(echo "$OUTPUT" | grep -o '"user_id":"[^"]*"' | cut -d'"' -f4)
TOKEN=$(echo "$OUTPUT" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)

if [[ -z "$TOKEN" || -z "$USER_ID" ]]; then
  log_err "Failed to create test user. Output:"
  echo "$OUTPUT"
  exit 1
fi

log_ok "Created user: ${USER_ID}"
log_ok "Token TTL: ${TTL}"

# ─── 5. Print browser injection snippet ──────────────────────────────────────
echo ""
echo -e "${BOLD}═══════════════════════════════════════${NC}"
echo -e "${BOLD}   Browser Console Injection Snippet    ${NC}"
echo -e "${BOLD}═══════════════════════════════════════${NC}"
echo ""
echo -e "Open ${CYAN}http://localhost:${FRONTEND_PORT}${NC} → DevTools Console → paste:"
echo ""
echo -e "${YELLOW}// ─── Toqui QA Token Injection ───"
echo "localStorage.clear();"
echo "localStorage.setItem('toqui_access_token', '${TOKEN}');"
echo "// NOTE: do NOT set toqui_refresh_token — any value triggers a backend"
echo "// refresh call that will wipe all auth state on failure."
echo "localStorage.setItem('toqui_user', JSON.stringify({"
echo "  id: '${USER_ID}',"
echo "  email: '${USER_EMAIL}',"
echo "  name: '${USER_NAME}',"
echo "  tier: 'free'"
echo "}));"
echo "localStorage.setItem('toqui_age_verified', 'true');"
echo "localStorage.setItem('toqui_age_synced', 'true');"
echo -e "window.location.reload();${NC}"
echo ""

# ─── 6. Print grpcurl reference ──────────────────────────────────────────────
echo -e "${BOLD}═══════════════════════════════════════${NC}"
echo -e "${BOLD}       grpcurl Quick Reference          ${NC}"
echo -e "${BOLD}═══════════════════════════════════════${NC}"
echo ""
echo "# List all services"
echo "grpcurl -plaintext localhost:${BACKEND_PORT} list"
echo ""
echo "# Create a trip"
echo "grpcurl -plaintext \\"
echo "  -H \"Authorization: Bearer ${TOKEN}\" \\"
echo "  -d '{\"title\": \"My QA Trip\", \"start_date\": \"2025-10-01\", \"end_date\": \"2025-10-07\"}' \\"
echo "  localhost:${BACKEND_PORT} toqui.v1.TripService/CreateTrip"
echo ""
echo "# Send a chat message (streaming)"
echo "grpcurl -plaintext \\"
echo "  -H \"Authorization: Bearer ${TOKEN}\" \\"
echo "  -d '{\"trip_id\": \"<trip-id>\", \"content\": \"Plan day 1 in Tokyo\"}' \\"
echo "  localhost:${BACKEND_PORT} toqui.v1.ChatService/SendMessage"
echo ""
echo "# Cleanup this user when done"
echo "go run ./cmd/testctl cleanup-user --user-id ${USER_ID}"
echo ""
echo -e "${GREEN}${BOLD}QA session ready! Token expires in ${TTL}.${NC}"
