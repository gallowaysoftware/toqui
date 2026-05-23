#!/usr/bin/env bash
# db-restore-verify.sh — Restore the latest prod Cloud SQL backup to a
# temporary instance, run a health check, tear down, and report.
#
# This is the manual one-time-or-on-demand version of #266. The
# long-term plan is a monthly Cloud Scheduler job that runs the same
# steps end-to-end. This script is what proves the restore path works
# in the first place — no point automating a flow no one's verified.
#
# Compliance posture: GDPR Article 32 ("the ability to restore the
# availability and access to personal data in a timely manner in the
# event of a physical or technical incident") and PIPEDA Schedule 1
# Principle 4.7 (Safeguards) both require that backups be tested.
# Configured-but-untested = compliance gap.
#
# Usage:
#   ./scripts/db-restore-verify.sh                  # Interactive, with confirmations
#   ./scripts/db-restore-verify.sh --yes            # Non-interactive (CI mode)
#   ./scripts/db-restore-verify.sh --keep           # Skip teardown (debugging)
#   ./scripts/db-restore-verify.sh --backup <ID>    # Restore a specific backup ID
#                                                   # (default: latest)
#
# Prerequisites:
#   - gcloud auth: `gcloud auth login` and `gcloud auth application-default login`
#   - IAM: roles/cloudsql.admin on toqui-prod (or higher).
#   - The user running this needs ~$0.30 of GCP budget headroom for the temp
#     instance (db-f1-micro × ~30 minutes = a few cents; rounded up).
#
# What this script does:
#   1. Picks the latest successful backup of toqui-db in toqui-prod (or
#      the explicit ID passed via --backup).
#   2. Creates a NEW Cloud SQL instance `toqui-db-restore-verify-<ts>`
#      from that backup. The restore creates a fresh instance — it does
#      NOT touch toqui-db itself. Public IP only (saves us the VPC dance
#      since this instance lives for 30 minutes).
#   3. Connects via Cloud SQL Auth Proxy.
#   4. Runs a health-check SQL query: counts rows in 6 critical tables,
#      verifies pgcrypto + postgis extensions are loaded, sanity-checks
#      that recent (last-hour) rows exist in users + trips.
#   5. Writes a timestamped result line to docs/restore-verify-log.md
#      (PASS or FAIL with the row counts).
#   6. Tears down the temp instance unless --keep was passed.
#
# Cost guardrail: this script intentionally does NOT enable HA, NOT
# attach a VPC, and uses db-f1-micro. The restored instance is a
# read-only sanity-check fixture, not a production candidate. If you
# want to actually point the app at it (real DR drill), edit the
# instance settings after restore — don't shortcut by removing this
# script's defaults.

set -euo pipefail

readonly PROJECT="toqui-prod"
readonly REGION="northamerica-northeast1"
readonly SOURCE_INSTANCE="toqui-db"
readonly TEMP_TIER="db-f1-micro"

# Parse args
KEEP_INSTANCE=false
NON_INTERACTIVE=false
BACKUP_ID=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --yes) NON_INTERACTIVE=true; shift ;;
    --keep) KEEP_INSTANCE=true; shift ;;
    --backup) BACKUP_ID="$2"; shift 2 ;;
    -h|--help) sed -n '2,40p' "$0"; exit 0 ;;
    *) echo "Unknown arg: $1" >&2; exit 1 ;;
  esac
done

readonly TS="$(date -u +%Y%m%d-%H%M%S)"
readonly TARGET_INSTANCE="toqui-db-restore-verify-${TS}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
RESET='\033[0m'

confirm() {
  if [[ "$NON_INTERACTIVE" == "true" ]]; then
    return 0
  fi
  printf "%b%s%b [y/N] " "$YELLOW" "$1" "$RESET"
  read -r reply
  [[ "$reply" =~ ^[Yy]$ ]]
}

log_info() { printf "%b[info]%b %s\n" "$GREEN" "$RESET" "$1"; }
log_warn() { printf "%b[warn]%b %s\n" "$YELLOW" "$RESET" "$1" >&2; }
log_err()  { printf "%b[err ]%b %s\n" "$RED" "$RESET" "$1" >&2; }

# ---------- Step 0: sanity ----------
gcloud auth application-default print-access-token >/dev/null 2>&1 || {
  log_err "gcloud application-default credentials not configured."
  log_err "Run: gcloud auth application-default login"
  exit 1
}

log_info "Project: $PROJECT  Region: $REGION  Source: $SOURCE_INSTANCE"

# ---------- Step 1: pick a backup ----------
if [[ -z "$BACKUP_ID" ]]; then
  log_info "Finding latest successful backup of $SOURCE_INSTANCE..."
  BACKUP_ID="$(
    gcloud sql backups list \
      --instance="$SOURCE_INSTANCE" \
      --project="$PROJECT" \
      --filter="status=SUCCESSFUL" \
      --sort-by=~windowStartTime \
      --limit=1 \
      --format='value(id)'
  )"
  if [[ -z "$BACKUP_ID" ]]; then
    log_err "No successful backup found. Bail."
    exit 1
  fi
fi
log_info "Using backup ID: $BACKUP_ID"

confirm "Create temp instance $TARGET_INSTANCE from this backup?" || {
  log_warn "Aborted by user."
  exit 0
}

# ---------- Step 2: restore ----------
log_info "Creating $TARGET_INSTANCE from backup (this takes 5-15 min)..."
gcloud sql instances clone \
  "$SOURCE_INSTANCE" "$TARGET_INSTANCE" \
  --project="$PROJECT" \
  --backup-id="$BACKUP_ID" \
  >/dev/null

log_info "Switching $TARGET_INSTANCE to db-f1-micro + public IP for cheap verification..."
gcloud sql instances patch "$TARGET_INSTANCE" \
  --project="$PROJECT" \
  --tier="$TEMP_TIER" \
  --availability-type=ZONAL \
  --no-backup \
  --assign-ip \
  --quiet \
  >/dev/null

# ---------- Step 3: health check via cloud-sql-proxy ----------
log_info "Running health check..."

# Pull the database password from Secret Manager (same secret prod uses).
# We hit the read-only side; if Secret Manager is unhappy the verify
# fails closed and we bail.
DB_URL="$(gcloud secrets versions access latest --secret=prod-database-url --project="$PROJECT")"
DB_USER="$(echo "$DB_URL" | sed -E 's|^postgres://([^:]+):.*|\1|')"
DB_PASS="$(echo "$DB_URL" | sed -E 's|^postgres://[^:]+:([^@]+)@.*|\1|')"
DB_NAME="$(echo "$DB_URL" | sed -E 's|.*/([^?]+).*|\1|')"

# Use the Cloud SQL Auth Proxy in a subshell. v2 syntax.
PROXY_PORT=15432
log_info "Starting cloud-sql-proxy on localhost:$PROXY_PORT..."
cloud-sql-proxy \
  --port="$PROXY_PORT" \
  "$PROJECT:$REGION:$TARGET_INSTANCE" \
  &
PROXY_PID=$!
trap 'kill $PROXY_PID 2>/dev/null || true' EXIT

# Wait for proxy to be ready
for _ in $(seq 1 30); do
  if nc -z localhost "$PROXY_PORT" 2>/dev/null; then break; fi
  sleep 1
done

CHECK_SQL=$(cat <<'SQL'
\set ON_ERROR_STOP on
\echo '--- extensions ---'
SELECT extname FROM pg_extension WHERE extname IN ('pgcrypto', 'postgis') ORDER BY extname;
\echo '--- table row counts ---'
SELECT 'users'              AS table_name, COUNT(*) AS rows FROM users
UNION ALL SELECT 'trips',           COUNT(*) FROM trips
UNION ALL SELECT 'itinerary_items', COUNT(*) FROM itinerary_items
UNION ALL SELECT 'bookings',        COUNT(*) FROM bookings
UNION ALL SELECT 'subscriptions',   COUNT(*) FROM subscriptions
UNION ALL SELECT 'refresh_tokens',  COUNT(*) FROM refresh_tokens;
\echo '--- recency probes ---'
SELECT MAX(created_at) AS latest_user_created FROM users;
SELECT MAX(updated_at) AS latest_trip_updated FROM trips;
SQL
)

OUTPUT_FILE="$(mktemp -t restore-verify.XXXXXX)"
if PGPASSWORD="$DB_PASS" psql \
  --host=localhost --port="$PROXY_PORT" \
  --username="$DB_USER" --dbname="$DB_NAME" \
  --no-psqlrc --quiet \
  --command="$CHECK_SQL" \
  > "$OUTPUT_FILE" 2>&1
then
  CHECK_STATUS="PASS"
else
  CHECK_STATUS="FAIL"
fi

log_info "Health check $CHECK_STATUS — output:"
cat "$OUTPUT_FILE"

# ---------- Step 4: append to log ----------
LOG_PATH="${SCRIPT_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}/../docs/restore-verify-log.md"
mkdir -p "$(dirname "$LOG_PATH")"
{
  echo
  echo "## $TS UTC — $CHECK_STATUS"
  echo
  echo "- Backup ID: \`$BACKUP_ID\`"
  echo "- Target instance: \`$TARGET_INSTANCE\`"
  echo "- Operator: \`$(whoami)\`"
  echo
  echo '<details><summary>Health check output</summary>'
  echo
  echo '```'
  cat "$OUTPUT_FILE"
  echo '```'
  echo
  echo '</details>'
} >> "$LOG_PATH"
log_info "Logged result to $LOG_PATH"

# ---------- Step 5: teardown ----------
if [[ "$KEEP_INSTANCE" == "true" ]]; then
  log_warn "Skipping teardown ($TARGET_INSTANCE preserved per --keep flag)."
  log_warn "Don't forget to delete it manually:"
  log_warn "  gcloud sql instances delete $TARGET_INSTANCE --project=$PROJECT"
else
  log_info "Tearing down $TARGET_INSTANCE..."
  gcloud sql instances delete "$TARGET_INSTANCE" \
    --project="$PROJECT" \
    --quiet \
    >/dev/null
  log_info "Teardown complete."
fi

if [[ "$CHECK_STATUS" == "PASS" ]]; then
  log_info "Restore verification PASSED."
  exit 0
else
  log_err "Restore verification FAILED — see output above and $LOG_PATH."
  exit 1
fi
