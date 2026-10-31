#!/usr/bin/env bash
# scripts/migrate.sh
# ------------------------------------------------------------
# Convenience wrapper around golang-migrate for the local
# Docker‑Compose PostgreSQL instance.
#
# It builds a connection string from environment variables,
# mounts the migration files into the official migrate Docker
# image and forwards any arguments to the migrate CLI.
#
# Example:
#   DB_PASSWORD=secret ./scripts/migrate.sh up
# ------------------------------------------------------------

set -euo pipefail

# ------------------------------------------------------------------
# Resolve paths
# ------------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MIGRATIONS_DIR="${PROJECT_ROOT}/backend/internal/db/migrations"

# ------------------------------------------------------------------
# Default DB connection parameters (can be overridden by env vars)
# ------------------------------------------------------------------
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-postgres}"
DB_PASSWORD="${DB_PASSWORD:-postgres}"
DB_NAME="${DB_NAME:-finsight}"
DB_SSLMODE="${DB_SSLMODE:-disable}"

# ------------------------------------------------------------------
# Usage helper
# ------------------------------------------------------------------
usage() {
  cat <<EOF
Usage: $(basename "$0") <command> [options]

Commands (passed directly to golang-migrate):
  up               Apply all pending up migrations (default)
  down             Apply all down migrations
  goto <N>          Migrate to version N
  force <N>        Force set version without running migrations
  version          Print current version
  drop             Drop everything (dangerous)

Options:
  -h, --help       Show this help message

Environment variables (override defaults):
  DB_HOST          PostgreSQL host (default: $DB_HOST)
  DB_PORT          PostgreSQL port (default: $DB_PORT)
  DB_USER          PostgreSQL user (default: $DB_USER)
  DB_PASSWORD      PostgreSQL password (default: $DB_PASSWORD)
  DB_NAME          PostgreSQL database name (default: $DB_NAME)
  DB_SSLMODE       SSL mode (default: $DB_SSLMODE)
  DATABASE_URL     Full connection URL (overrides the DB_* vars)

Example:
  DB_PASSWORD=secret $(basename "$0") up
EOF
  exit 1
}

# ------------------------------------------------------------------
# Parse help flag
# ------------------------------------------------------------------
if [[ "${1-}" == "-h" ]] || [[ "${1-}" == "--help" ]]; then
  usage
fi

# ------------------------------------------------------------------
# Determine command (default to "up")
# ------------------------------------------------------------------
COMMAND="${1:-up}"
shift || true   # Remove the command from the argument list, keep the rest for migrate

# ------------------------------------------------------------------
# Build DATABASE_URL if not supplied explicitly
# ------------------------------------------------------------------
if [[ -z "${DATABASE_URL-}" ]]; then
  # URL‑encode password in case it contains special characters
  ENCODED_PASSWORD="$(printf '%s' "$DB_PASSWORD" | python3 -c 'import urllib.parse, sys; print(urllib.parse.quote_plus(sys.stdin.read()))')"
  DATABASE_URL="postgres://${DB_USER}:${ENCODED_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSLMODE}"
fi

# ------------------------------------------------------------------
# Validate migrations directory
# ------------------------------------------------------------------
if [[ ! -d "$MIGRATIONS_DIR" ]]; then
  echo "Error: migrations directory not found at $MIGRATIONS_DIR" >&2
  exit 1
fi

# ------------------------------------------------------------------
# Run migrate using the official Docker image
# ------------------------------------------------------------------
run_migrate() {
  docker run --rm \
    -v "${MIGRATIONS_DIR}":/migrations \
    -e "DATABASE_URL=${DATABASE_URL}" \
    migrate/migrate \
    -path /migrations -database "${DATABASE_URL}" "$@"
}

# ------------------------------------------------------------------
# Execute
# ------------------------------------------------------------------
echo "Running migrations: ${COMMAND} $*"
run_migrate "${COMMAND}" "$@"
echo "Migrations completed successfully."

exit 0