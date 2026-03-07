#!/usr/bin/env bash
# =============================================================================
# Start all Sentinel-Proxy services locally
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
CONFIG_FILE="${PROJECT_ROOT}/config/sentinel.yaml"
LOG_FILE="/tmp/sentinel-proxy.log"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'
BOLD='\033[1m'

info() { echo -e "${CYAN}[INFO]${NC} $1"; }
success() { echo -e "${GREEN}[OK]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; }

cleanup() {
    info "Stopping services..."
    if [ -n "${BACKEND_PID:-}" ]; then
        kill "$BACKEND_PID" 2>/dev/null || true
        wait "$BACKEND_PID" 2>/dev/null || true
    fi
    if [ -n "${PROXY_PID:-}" ]; then
        kill "$PROXY_PID" 2>/dev/null || true
        wait "$PROXY_PID" 2>/dev/null || true
    fi
    success "All services stopped"
}

trap cleanup EXIT INT TERM

# Check Redis
check_redis() {
    if command -v redis-cli &> /dev/null; then
        if redis-cli ping > /dev/null 2>&1; then
            success "Redis is running"
            return 0
        fi
    fi
    warn "Redis is not running. System will operate in fail-open mode."
    warn "To start Redis: docker run -d -p 6379:6379 redis:7-alpine"
    return 0  # Don't fail - fail-open design
}

# Build
build() {
    info "Building services..."
    cd "${PROJECT_ROOT}"

    info "Building backend..."
    go build -o bin/backend ./cmd/backend/
    success "Backend built"

    info "Building proxy..."
    go build -o bin/proxy ./cmd/proxy/
    success "Proxy built"
}

# Start
start() {
    info "Starting backend gRPC server..."
    "${PROJECT_ROOT}/bin/backend" -config "${CONFIG_FILE}" &
    BACKEND_PID=$!
    sleep 1

    if kill -0 "$BACKEND_PID" 2>/dev/null; then
        success "Backend started (PID: ${BACKEND_PID})"
    else
        error "Backend failed to start"
        exit 1
    fi

    info "Starting proxy HTTP server..."
    "${PROJECT_ROOT}/bin/proxy" -config "${CONFIG_FILE}" > "$LOG_FILE" 2>&1 &
    PROXY_PID=$!
    sleep 1

    if kill -0 "$PROXY_PID" 2>/dev/null; then
        success "Proxy started (PID: ${PROXY_PID})"
    else
        error "Proxy failed to start"
        exit 1
    fi
}

main() {
    echo -e "${BOLD}${CYAN}"
    echo "╔═══════════════════════════════════════════════════════════════╗"
    echo "║         🛡️  SENTINEL-PROXY - Local Development              ║"
    echo "╚═══════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"

    check_redis
    build

    # Clean slate for demo
    if command -v redis-cli &>/dev/null && redis-cli ping &>/dev/null; then
        redis-cli FLUSHDB > /dev/null 2>&1 || true
        info "Redis flushed for clean demo"
    fi
    > "$LOG_FILE"  # truncate log file

    start

    echo ""
    echo -e "${BOLD}${GREEN}Services Running:${NC}"
    echo -e "  Proxy HTTP:    http://localhost:8080"
    echo -e "  Backend gRPC:  localhost:9090"
    echo -e "  Metrics:       http://localhost:9100/metrics"
    echo -e "  Health:        http://localhost:8080/health"
    echo ""
    echo -e "${BOLD}${CYAN}Endpoints:${NC}"
    echo -e "  GET /data              - Standard data endpoint"
    echo -e "  GET /export            - Export endpoint (elevated limits)"
    echo -e "  GET /simulate/normal   - Simulate normal user"
    echo -e "  GET /simulate/attack   - Simulate attacker"
    echo -e "  GET /simulate/export   - Simulate legitimate export"
    echo -e "  GET /health            - Health check"
    echo ""
    echo -e "${BOLD}${CYAN}Live Logs:${NC}"
    echo -e "  Proxy log:  ${LOG_FILE}"
    echo -e "  Log watch:  ./scripts/logwatch.sh"
    echo ""
    echo -e "${YELLOW}Press Ctrl+C to stop all services${NC}"
    echo ""

    # Wait for both processes
    wait
}

main "$@"
