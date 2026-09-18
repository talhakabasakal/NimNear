#!/usr/bin/env bash
#
# migrate.sh - Database migration helper
#
# Canonical migrations use goose against the Nimnear DSN.
# Default DSN is the local Nimnear development database, never MasterFabric.
#
# Usage:
#   ./scripts/migrate.sh up
#   ./scripts/migrate.sh down
#   ./scripts/migrate.sh status
#   ./scripts/migrate.sh create NAME
#
# Override the target with DB_DSN. Destructive down requires an explicit DSN
# when you are not targeting the local development database.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MIGRATION_DIR="$PROJECT_ROOT/internal/infrastructure/postgres/migrations"
DEFAULT_DSN="postgres://nimnear:nimnear@localhost:5432/nimnear?sslmode=disable"
DB_DSN="${DB_DSN:-$DEFAULT_DSN}"

if echo "$DB_DSN" | grep -qi 'masterfabric'; then
    echo "Refusing to run Nimnear migrations against a MasterFabric DSN. Set DB_DSN to a Nimnear database." >&2
    exit 1
fi

if ! command -v goose >/dev/null 2>&1; then
    echo "goose is required. Install with: go install github.com/pressly/goose/v3/cmd/goose@latest" >&2
    exit 1
fi

cd "$PROJECT_ROOT"

case "${1:-}" in
    up)
        echo "Running goose up against the configured Nimnear DSN..."
        goose -dir "$MIGRATION_DIR" postgres "$DB_DSN" up
        ;;
    down)
        echo "Rolling back last goose migration..."
        goose -dir "$MIGRATION_DIR" postgres "$DB_DSN" down
        ;;
    status)
        goose -dir "$MIGRATION_DIR" postgres "$DB_DSN" status
        ;;
    create)
        if [[ -z "${2:-}" ]]; then
            echo "Usage: $0 create MIGRATION_NAME" >&2
            exit 1
        fi
        name="${2}"
        if [[ ! "$name" =~ ^[a-zA-Z0-9_]+$ ]]; then
            echo "Migration name may only contain letters, numbers, and underscores" >&2
            exit 1
        fi
        goose -dir "$MIGRATION_DIR" create "$name" sql
        ;;
    *)
        echo "Usage: $0 {up|down|status|create NAME}"
        exit 1
        ;;
esac
