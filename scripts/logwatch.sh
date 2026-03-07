#!/usr/bin/env bash
# =================================================================
#  Live log watcher — run this in a SIDE TERMINAL during the demo
# =================================================================
#  Shows a clean, color-coded feed of proxy decisions in real time.
#  The audience sees raw structured logs proving the system works.
# =================================================================

set -euo pipefail

R='\033[0;31m'  G='\033[0;32m'  Y='\033[1;33m'  C='\033[0;36m'
W='\033[1;37m'  DIM='\033[2m'   BOLD='\033[1m'   NC='\033[0m'

PROXY_LOG="${1:-/tmp/sentinel-proxy.log}"

clear
echo ""
echo -e "${BOLD}${W}  SENTINEL LIVE LOG FEED${NC}"
echo -e "${DIM}  ──────────────────────────────────────────${NC}"
echo -e "${DIM}  Watching: ${PROXY_LOG}${NC}"
echo -e "${DIM}  Press Ctrl+C to stop${NC}"
echo ""

# If the log file doesn't exist yet, wait for it
if [[ ! -f "$PROXY_LOG" ]]; then
    echo -e "${DIM}  Waiting for log file ...${NC}"
    while [[ ! -f "$PROXY_LOG" ]]; do sleep 0.5; done
    echo ""
fi

# Tail and format — highlight the important events
tail -f "$PROXY_LOG" 2>/dev/null | while IFS= read -r line; do
    # Skip debug-level noise
    if echo "$line" | grep -q '"level":"DEBUG"'; then
        continue
    fi

    # Extract key fields from JSON log
    msg=$(echo "$line" | grep -oP '"msg"\s*:\s*"\K[^"]+' 2>/dev/null || echo "")
    user=$(echo "$line" | grep -oP '"user_id"\s*:\s*"\K[^"]+' 2>/dev/null || echo "")
    total=$(echo "$line" | grep -oP '"total_bytes"\s*:\s*\K[0-9]+' 2>/dev/null || echo "")
    allowed=$(echo "$line" | grep -oP '"allowed"\s*:\s*\K[0-9]+' 2>/dev/null || echo "")
    reason=$(echo "$line" | grep -oP '"reason"\s*:\s*"\K[^"]+' 2>/dev/null || echo "")
    outcome=$(echo "$line" | grep -oP '"outcome"\s*:\s*"\K[^"]+' 2>/dev/null || echo "")
    grade=$(echo "$line" | grep -oP '"grade"\s*:\s*"\K[^"]+' 2>/dev/null || echo "")

    ts=$(date '+%H:%M:%S')

    case "$msg" in
        "STREAM HARD KILL")
            echo -e "  ${R}${BOLD}${ts}  KILL${NC}  ${R}${user}${NC}  ${DIM}${reason}${NC}  ${R}${total} bytes${NC} (limit: ${allowed})"
            ;;
        "SOFT BREACH detected")
            echo -e "  ${Y}${ts}  BREACH${NC}  ${Y}${user}${NC}  ${DIM}${reason}${NC}  ${Y}${total} bytes${NC}"
            ;;
        "STREAM THROTTLED"*)
            echo -e "  ${Y}${ts}  THROTTLE${NC}  ${Y}${user}${NC}  ${total} bytes"
            ;;
        "enforcement decision")
            echo -e "  ${Y}${ts}  GRADE${NC}  ${user}  → ${BOLD}${grade}${NC}"
            ;;
        "VIOLATION LOGGED"*)
            echo -e "  ${Y}${ts}  GRACE${NC}  ${user}  ${DIM}logged, continuing${NC}"
            ;;
        "request received")
            echo -e "  ${C}${ts}  →${NC}  ${C}${user}${NC}"
            ;;
        "threshold decision")
            if [[ -n "$allowed" ]]; then
                echo -e "  ${DIM}${ts}  EVAL${NC}  ${DIM}allowed=${allowed}  avg=${total:-?}${NC}"
            fi
            ;;
        "request completed")
            if [[ "$outcome" == "killed" ]]; then
                echo -e "  ${R}${ts}  ←${NC}  ${R}${user}${NC}  ${R}KILLED${NC}  ${total} bytes  ${DIM}${reason}${NC}"
            elif [[ "$outcome" == "complete" ]]; then
                echo -e "  ${G}${ts}  ←${NC}  ${G}${user}${NC}  ${G}OK${NC}  ${total} bytes"
            else
                echo -e "  ${DIM}${ts}  ←${NC}  ${user}  ${outcome}  ${total} bytes"
            fi
            ;;
        "bypass header detected"*)
            echo -e "  ${Y}${ts}  BYPASS${NC}  ${user}"
            ;;
        "sentinel-proxy starting"|"sentinel-proxy listening"*)
            echo -e "  ${G}${ts}  STARTUP${NC}  ${DIM}${msg}${NC}"
            ;;
        *)
            # Show other warnings/errors
            if echo "$line" | grep -q '"level":"ERROR"'; then
                echo -e "  ${R}${ts}  ERROR${NC}  ${DIM}${msg}${NC}"
            elif echo "$line" | grep -q '"level":"WARN"'; then
                echo -e "  ${Y}${ts}  WARN${NC}  ${DIM}${msg}${NC}"
            fi
            ;;
    esac
done
