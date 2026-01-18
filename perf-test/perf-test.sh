#!/usr/bin/env bash
set -euo pipefail

# === CONFIGURATION ===========================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
RESULTS_DIR="$SCRIPT_DIR/test-results"
CONFIG_DIR="${CONFIG_DIR:-./config-perf-test}"
KONG_ADMIN_URL="http://localhost:8001"
KONG_HEALTH_CHECK_MAX_RETRIES=60
KONG_HEALTH_CHECK_INTERVAL=1
CLEANUP_ENABLED=${1:-true}  # Default to cleanup, pass 'false' or '--no-cleanup' to skip

# Load .env if it exists
if [[ -f "$SCRIPT_DIR/.env" ]]; then
    set -o allexport
    source "$SCRIPT_DIR/.env"
    set +o allexport
fi

mkdir -p "$RESULTS_DIR"
# =============================================================

log() {
    echo "[$(date '+%H:%M:%S')] $*"
}

log_section() {
    echo
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "$1"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
}

cleanup() {
    if [[ "$CLEANUP_ENABLED" == "false" ]] || [[ "$CLEANUP_ENABLED" == "--no-cleanup" ]]; then
        log "Skipping docker-compose cleanup for verification..."
        log "Kong services still running. To cleanup later, run:"
        log "  cd $SCRIPT_DIR && docker-compose down"
    else
        log "Cleaning up docker-compose..."
        docker-compose -f "$SCRIPT_DIR/docker-compose.yaml" down 2>/dev/null || true
        log "✓ Cleanup complete"
    fi
}

startup_kong() {
    log "Starting Kong with docker-compose..."
    
    # Start Kong services
    docker-compose -f "$SCRIPT_DIR/docker-compose.yaml" up -d
    
    log "✓ Kong docker-compose started"
}

wait_for_kong() {
    log "Waiting for Kong control plane to be ready..."
    local retry_count=0
    
    while [[ $retry_count -lt $KONG_HEALTH_CHECK_MAX_RETRIES ]]; do
        # Check Kong is responding with version info (comprehensive health check)
        local version=$(curl -sf -H "kong-admin-token: password" "$KONG_ADMIN_URL" 2>/dev/null | grep -o '"version":"[^"]*"' | cut -d'"' -f4 || echo "")
        
        if [[ -n "$version" ]]; then
            log "✓ Kong control plane is ready! (version: $version)"
            return 0
        fi
        
        retry_count=$((retry_count + 1))
        if [[ $((retry_count % 10)) -eq 0 ]]; then
            log "  Waiting for Kong... ($retry_count/$KONG_HEALTH_CHECK_MAX_RETRIES)"
        fi
        sleep "$KONG_HEALTH_CHECK_INTERVAL"
    done
    
    log "❌ Kong failed to become fully ready after ${KONG_HEALTH_CHECK_MAX_RETRIES}s"
    exit 1
}

publish_results() {
    local result_file="$1"
    local elapsed_real="$2"
    local elapsed_user="$3"
    local elapsed_sys="$4"
    
    # Create summary report
    local report_file="${result_file%.txt}_summary.txt"
    
    {
        echo "=========================================="
        echo "kwot Performance Test Results"
        echo "=========================================="
        echo ""
        echo "Execution Time:"
        echo "  Real:   $elapsed_real"
        echo "  User:   $elapsed_user"
        echo "  System: $elapsed_sys"
        echo ""
        echo "Configuration:"
        echo "  Directory: $CONFIG_DIR"
        echo "  Kong setup: docker-compose (${KONG_DOCKER_IMAGE:-kong/kong-gateway:3.6.1.7-rhel})"
        echo "  Timestamp: $(date)"
        echo ""
        echo "Details:"
        echo "  Full output: $(basename "$result_file")"
        echo ""
    } > "$report_file"
    
    log "✓ Summary published to: $(basename "$report_file")"
}

run_test() {
    log "Running kwot apply test"
    log "Config directory: $CONFIG_DIR"
    
    # Validate config directory exists
    if [[ ! -d "$CONFIG_DIR" ]]; then
        log "❌ Config directory not found: $CONFIG_DIR"
        exit 1
    fi
    
    # Determine kwot binary path
    local kwot_bin="${PROJECT_ROOT}/bin/kwot"
    if [[ ! -f "$kwot_bin" ]]; then
        kwot_bin=$(which kwot)
    fi
    
    # Create result file
    local timestamp=$(date +%Y%m%d_%H%M%S)
    local outfile="$RESULTS_DIR/kwot_test_${timestamp}.txt"
    local timefile="$RESULTS_DIR/kwot_test_${timestamp}_time.txt"
    
    log "⏱️  Timing started..."
    log "Executing: CONFIG_DIR=$CONFIG_DIR $kwot_bin apply"
    
    # Run command with timing output to separate file
    {
        time CONFIG_DIR="$CONFIG_DIR" "$kwot_bin" apply
    } > "$outfile" 2> "$timefile"
    
    # Read timing file
    local timing_content=$(cat "$timefile")
    
    # Extract timing information (compatible with both GNU and BSD grep)
    local elapsed_real=$(echo "$timing_content" | grep 'real' | awk '{print $2}')
    local elapsed_user=$(echo "$timing_content" | grep 'user' | awk '{print $2}')
    local elapsed_sys=$(echo "$timing_content" | grep 'sys' | awk '{print $2}')
    
    log "Test completed"
    log "  Real time:   $elapsed_real"
    log "  User time:   $elapsed_user"
    log "  System time: $elapsed_sys"
    
    # Append timing to output file
    echo "" >> "$outfile"
    echo "========================================" >> "$outfile"
    echo "Execution Timing:" >> "$outfile"
    echo "========================================" >> "$outfile"
    echo "$timing_content" >> "$outfile"
    
    # Publish results
    publish_results "$outfile" "$elapsed_real" "$elapsed_user" "$elapsed_sys"
}

# === MAIN ====================================================

log_section "kwot Performance Test with Kong"

# === VALIDATE SETUP ==========================================

log_section "Validating Setup"

if ! command -v kwot &> /dev/null && [[ ! -f "$PROJECT_ROOT/bin/kwot" ]]; then
    log "❌ kwot not found in PATH or at $PROJECT_ROOT/bin/kwot"
    exit 1
fi

if ! command -v docker-compose &> /dev/null; then
    log "❌ docker-compose not found in PATH"
    exit 1
fi

if ! command -v curl &> /dev/null; then
    log "❌ curl not found in PATH"
    exit 1
fi

if [[ ! -f "$SCRIPT_DIR/docker-compose.yaml" ]]; then
    log "❌ docker-compose.yaml not found in $SCRIPT_DIR"
    exit 1
fi

log "✓ kwot found: $(which kwot)"
log "✓ docker-compose found: $(which docker-compose)"
log "✓ docker-compose.yaml found"
log "✓ Results directory: $RESULTS_DIR"

# === SETUP KONG ===============================================

log_section "Setting Up Kong"

# Set trap to cleanup on exit
trap cleanup EXIT

startup_kong
wait_for_kong

# === RUN TEST ================================================

log_section "Running Test"
run_test

# === RESULTS =================================================

log_section "Test Complete"
log "✓ Results saved to: $RESULTS_DIR/"
echo ""
ls -lh "$RESULTS_DIR"/*.txt 2>/dev/null | tail -2
