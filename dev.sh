#!/usr/bin/env bash
#
# dev.sh — Development runner for NIMNear
#
# Features:
#   - Starts Docker services (Postgres, Redis, Kafka, Kafka UI)
#   - Waits for services to become healthy
#   - Runs database migrations (Up only)
#   - Ensures Kafka topics exist
#   - Starts the Go API with hot-reload (air)
#   - Starts the Next.js frontend on 0.0.0.0
#
# Usage:
#   ./dev.sh          — Full startup (infra + migrations + API + frontend)
#   ./dev.sh server   — Hot-reload API + frontend only (skip infra)
#   ./dev.sh infra    — Start infra only (no apps)
#   ./dev.sh migrate  — Run migrations only
#   ./dev.sh down     — Stop all Docker services
#   ./dev.sh logs     — Tail Docker service logs
#   ./dev.sh clean    — Stop infra, remove volumes, clean build artifacts
#
set -euo pipefail

# ─── Configuration ────────────────────────────────────────────────────────────
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_ROOT="$PROJECT_ROOT/backend"
FRONTEND_ROOT="$PROJECT_ROOT/frontend/web"
COMPOSE_FILE="$BACKEND_ROOT/deployments/docker-compose.yml"
MIGRATION_DIR="$BACKEND_ROOT/internal/infrastructure/postgres/migrations"
DB_USER="nimnear"
DB_PASSWORD="nimnear"
DB_NAME="nimnear"
DB_CONTAINER="nimnear-postgres"
export DB_USER DB_PASSWORD DB_NAME

# Host PostgreSQL often occupies 5432. Map Docker Postgres to a free port and
# tell the Go API to use the same host port.
if [[ -z "${DB_PORT:-}" ]]; then
    if ss -H -tln 2>/dev/null | awk '{print $4}' | grep -qE '(^|:)5432$'; then
        DB_PORT=55432
    else
        DB_PORT=5432
    fi
fi
export DB_PORT

# Build env — workaround for macOS sandbox permissions on /var/folders
export GOTMPDIR="$BACKEND_ROOT/tmp"
export GOCACHE="$BACKEND_ROOT/tmp/go-cache"
export TMPDIR="$BACKEND_ROOT/tmp"
export CGO_ENABLED=0

# Kafka env
export KAFKA_ENABLED=true

# Browser and Nimiq Pay WebView development origins. The frontend derives
# its API hostname from the URL used to open the app, so keep the same hosts here.
NIMNEAR_DEV_LAN_IP="${NIMNEAR_DEV_LAN_IP:-$(hostname -I 2>/dev/null | awk '{print $1}')}"
export NIMNEAR_DEV_LAN_IP
export CORS_ALLOWED_ORIGINS="${CORS_ALLOWED_ORIGINS:-http://localhost:3000,http://127.0.0.1:3000,http://${NIMNEAR_DEV_LAN_IP}:3000,http://${NIMNEAR_DEV_LAN_IP}:5173}"
export NIMNEAR_AUTH_NETWORK="${NIMNEAR_AUTH_NETWORK:-test-albatross}"
export NIMNEAR_AUTH_ENVIRONMENT="${NIMNEAR_AUTH_ENVIRONMENT:-testnet}"
export NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK="${NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK:-$NIMNEAR_AUTH_NETWORK}"
export NIMNEAR_AUTH_DOMAIN="${NIMNEAR_AUTH_DOMAIN:-nimnear.local}"
export NIMNEAR_API_URL="${NIMNEAR_API_URL:-http://localhost:8080}"

# Local development uses TestAlbatross. The public endpoint at
# https://rpc.nimiqwatch.com is MainAlbatross and must not be paired with
# NIMNEAR_NIMIQ_NETWORK=test-albatross. The testnet sibling below was
# live-verified as TestAlbatross (network id 5). It is rate-limited and is
# not production RPC infrastructure. Payment config must be supplied as a
# set because NIMNEAR_NIMIQ_RPC_URL is created through that existing bundle.
export NIMNEAR_NIMIQ_NETWORK="${NIMNEAR_NIMIQ_NETWORK:-test-albatross}"
export NIMNEAR_NIMIQ_RPC_URL="${NIMNEAR_NIMIQ_RPC_URL:-https://rpc.testnet.nimiqwatch.com}"
export NIMNEAR_MERCHANT_ADDRESS="${NIMNEAR_MERCHANT_ADDRESS:-NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604}"

API_PID=""
WEB_PID=""

# ─── Colors ───────────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

log_info()  { echo -e "${CYAN}[INFO]${NC}  $*"; }
log_ok()    { echo -e "${GREEN}[OK]${NC}    $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }
log_step()  { echo -e "\n${BOLD}━━━ $* ━━━${NC}"; }

# ─── Helpers ──────────────────────────────────────────────────────────────────

ensure_dirs() {
    mkdir -p "$BACKEND_ROOT/tmp"
    mkdir -p "$BACKEND_ROOT/bin"
}

check_docker() {
    if ! docker info &>/dev/null; then
        log_warn "Docker is not running. Attempting to start Docker Desktop..."
        open -a Docker 2>/dev/null || true
        local retries=30
        for i in $(seq 1 $retries); do
            docker info &>/dev/null && break
            printf "  waiting for Docker daemon... (%d/%d)\r" "$i" "$retries"
            sleep 2
        done
        echo ""
        if ! docker info &>/dev/null; then
            log_error "Docker daemon failed to start. Please start Docker manually."
            exit 1
        fi
        log_ok "Docker is ready"
    fi
}

install_air() {
    if ! command -v air &>/dev/null; then
        log_info "Installing air (hot-reload tool)..."
        GOBIN="$BACKEND_ROOT/bin" go install github.com/air-verse/air@latest 2>/dev/null \
            || go install github.com/air-verse/air@latest 2>/dev/null
        if [[ -f "$BACKEND_ROOT/bin/air" ]]; then
            export PATH="$BACKEND_ROOT/bin:$PATH"
        fi
    fi

    if ! command -v air &>/dev/null && [[ -f "$BACKEND_ROOT/bin/air" ]]; then
        export PATH="$BACKEND_ROOT/bin:$PATH"
    fi

    if ! command -v air &>/dev/null; then
        log_error "Failed to install air. Install manually: go install github.com/air-verse/air@latest"
        exit 1
    fi
    log_ok "air is available: $(which air)"
}

ensure_frontend() {
    if [[ ! -f "$FRONTEND_ROOT/package.json" ]]; then
        log_error "Frontend not found at $FRONTEND_ROOT"
        exit 1
    fi
    if ! command -v npm >/dev/null 2>&1; then
        log_error "npm is required to start the frontend."
        exit 1
    fi
    if [[ ! -d "$FRONTEND_ROOT/node_modules" ]]; then
        log_info "Installing frontend dependencies..."
        (cd "$FRONTEND_ROOT" && npm install)
        log_ok "Frontend dependencies installed"
    fi
}

stop_apps() {
    if [[ -n "${WEB_PID:-}" ]] && kill -0 "$WEB_PID" 2>/dev/null; then
        pkill -P "$WEB_PID" 2>/dev/null || true
        kill "$WEB_PID" 2>/dev/null || true
    fi
    if [[ -n "${API_PID:-}" ]] && kill -0 "$API_PID" 2>/dev/null; then
        pkill -P "$API_PID" 2>/dev/null || true
        kill "$API_PID" 2>/dev/null || true
    fi
    lsof -ti:3000 2>/dev/null | xargs kill -9 2>/dev/null || true
    lsof -ti:8080 2>/dev/null | xargs kill -9 2>/dev/null || true
}

# ─── Infrastructure ───────────────────────────────────────────────────────────

start_infra() {
    log_step "Starting infrastructure"
    check_docker

    log_info "Starting Docker Compose services..."
    log_info "Postgres host port: ${DB_PORT}"
    docker compose -f "$COMPOSE_FILE" up -d

    log_info "Waiting for services to become healthy..."
    local services=("nimnear-postgres" "nimnear-redis" "nimnear-kafka")
    for svc in "${services[@]}"; do
        local retries=30
        for i in $(seq 1 $retries); do
            local health
            health=$(docker inspect --format='{{.State.Health.Status}}' "$svc" 2>/dev/null || echo "missing")
            if [[ "$health" == "healthy" ]]; then
                log_ok "$svc is healthy"
                break
            fi
            if [[ $i -eq $retries ]]; then
                log_warn "$svc did not become healthy (status: $health)"
            fi
            sleep 2
        done
    done

    echo ""
    docker compose -f "$COMPOSE_FILE" ps --format "table {{.Name}}\t{{.Status}}\t{{.Ports}}"
    echo ""
}

stop_infra() {
    log_step "Stopping infrastructure"
    check_docker
    docker compose -f "$COMPOSE_FILE" down
    log_ok "All services stopped"
}

# ─── Migrations ───────────────────────────────────────────────────────────────

run_migrations() {
    log_step "Running database migrations"

    local count=0
    for f in "$MIGRATION_DIR"/0*.sql; do
        [[ -f "$f" ]] || continue
        local fname
        fname=$(basename "$f")
        # Extract only the UP part (between "-- +goose Up" and "-- +goose Down")
        local sql
        sql=$(sed -n '/^-- +goose Up$/,/^-- +goose Down$/p' "$f" | sed '1d;$d')
        if [[ -n "$sql" ]]; then
            echo "$sql" | docker exec -i "$DB_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" -q 2>/dev/null \
                && log_ok "  $fname" \
                || log_warn "  $fname (may already exist)"
            count=$((count + 1))
        fi
    done

    log_ok "Processed $count migration files"
}

# ─── Apps ─────────────────────────────────────────────────────────────────────

start_server() {
    log_step "Starting API and frontend"

    stop_apps
    sleep 1

    install_air
    ensure_frontend

    log_info "Watching for file changes..."
    log_info "Frontend will be available at ${BOLD}http://localhost:3000${NC}"
    log_info "API will be available at ${BOLD}http://localhost:8080${NC}"
    if [[ -n "$NIMNEAR_DEV_LAN_IP" ]]; then
        log_info "LAN frontend: ${BOLD}http://${NIMNEAR_DEV_LAN_IP}:3000${NC}"
        log_info "LAN API:      ${BOLD}http://${NIMNEAR_DEV_LAN_IP}:8080${NC}"
    fi
    log_info "Kafka UI available at ${BOLD}http://localhost:8090${NC}"
    log_info "Prometheus metrics at ${BOLD}http://localhost:8080/metrics${NC}"
    echo ""

    export PATH="$BACKEND_ROOT/bin:$PATH"
    (
        cd "$BACKEND_ROOT"
        exec air -c .air.toml
    ) &
    API_PID=$!

    (
        cd "$FRONTEND_ROOT"
        exec npm run dev -- --hostname 0.0.0.0 --port 3000
    ) &
    WEB_PID=$!

    trap 'echo ""; log_info "Stopping API and frontend..."; stop_apps; exit 0' INT TERM
    wait "$API_PID" "$WEB_PID"
}

# ─── Commands ─────────────────────────────────────────────────────────────────

cmd_full() {
    ensure_dirs
    start_infra
    run_migrations
    start_server
}

cmd_server() {
    ensure_dirs
    start_server
}

cmd_infra() {
    ensure_dirs
    start_infra
    run_migrations
    echo ""
    log_ok "Infrastructure is ready. Run ${BOLD}./dev.sh server${NC} to start the API and frontend."
}

cmd_migrate() {
    run_migrations
}

cmd_down() {
    stop_apps
    stop_infra
}

cmd_logs() {
    docker compose -f "$COMPOSE_FILE" logs -f --tail=50
}

cmd_clean() {
    log_step "Cleaning up"
    stop_apps
    check_docker
    docker compose -f "$COMPOSE_FILE" down -v 2>/dev/null || true
    rm -rf "$BACKEND_ROOT/tmp" "$BACKEND_ROOT/bin/server" "$BACKEND_ROOT/.tmp"
    log_ok "Cleaned: Docker volumes, tmp/, bin/server"
}

cmd_help() {
    echo -e "${BOLD}NIMNear development runner${NC}"
    echo ""
    echo "Usage: ./dev.sh [command]"
    echo ""
    echo "Commands:"
    echo -e "  ${GREEN}(default)${NC}   Full startup: infra + migrations + API + frontend"
    echo -e "  ${GREEN}server${NC}      Start API + frontend only (infra must be running)"
    echo -e "  ${GREEN}infra${NC}       Start infrastructure only (Postgres, Redis, Kafka)"
    echo -e "  ${GREEN}migrate${NC}     Run database migrations"
    echo -e "  ${GREEN}down${NC}        Stop apps and all Docker services"
    echo -e "  ${GREEN}logs${NC}        Tail Docker service logs"
    echo -e "  ${GREEN}clean${NC}       Stop infra, remove volumes, clean build artifacts"
    echo -e "  ${GREEN}help${NC}        Show this help message"
    echo ""
    echo "Environment:"
    echo "  KAFKA_ENABLED=true   (default: true)"
    echo "  DB_PORT=${DB_PORT}  (Docker Postgres host port)"
    echo "  DB_DSN=postgres://nimnear:nimnear@localhost:${DB_PORT}/nimnear?sslmode=disable"
    echo "  NIMNEAR_DEV_LAN_IP=${NIMNEAR_DEV_LAN_IP:-}"
    echo "  NIMNEAR_AUTH_NETWORK=${NIMNEAR_AUTH_NETWORK}"
    echo "  NIMNEAR_NIMIQ_NETWORK=${NIMNEAR_NIMIQ_NETWORK}"
    echo "  NIMNEAR_NIMIQ_RPC_URL=${NIMNEAR_NIMIQ_RPC_URL}"
    echo ""
    echo "Endpoints (when running):"
    echo "  Frontend:   http://localhost:3000"
    echo "  API:        http://localhost:8080"
    echo "  Health:     http://localhost:8080/health/ready"
    echo "  Metrics:    http://localhost:8080/metrics"
    echo "  Kafka UI:   http://localhost:8090"
}

# ─── Main ─────────────────────────────────────────────────────────────────────

case "${1:-}" in
    server)  cmd_server  ;;
    infra)   cmd_infra   ;;
    migrate) cmd_migrate ;;
    down)    cmd_down    ;;
    logs)    cmd_logs    ;;
    clean)   cmd_clean   ;;
    help|-h|--help) cmd_help ;;
    *)       cmd_full    ;;
esac
