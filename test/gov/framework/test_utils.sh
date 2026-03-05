#!/bin/bash

# Test Utilities
# Helper functions for E2E test suite

# Source configuration if not already loaded
if [ -z "${TEST_CONTINUE_ON_FAILURE:-}" ]; then
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    # shellcheck disable=SC1091
    source "${SCRIPT_DIR}/test_config.sh"
fi

# Wait for a condition to be true
# Usage: wait_for_condition "description" "command" [timeout] [interval]
wait_for_condition() {
    local description="$1"
    local condition_cmd="$2"
    local timeout="${3:-$DEFAULT_WAIT_TIMEOUT}"
    local interval="${4:-$DEFAULT_POLL_INTERVAL}"
    
    # Apply timeout multiplier
    timeout=$(get_timeout "$timeout")
    
    log_message INFO "Waiting for: $description (timeout: ${timeout}s)"
    
    local start_time
    start_time=$(date +%s)
    local elapsed=0
    
    while [ "$elapsed" -lt "$timeout" ]; do
        # Execute condition command
        if eval "$condition_cmd" > /dev/null 2>&1; then
            log_message INFO "Condition met: $description (after ${elapsed}s)"
            return 0
        fi
        
        sleep "$interval"
        elapsed=$(($(date +%s) - start_time))
        
        # Show progress every 30 seconds
        if [ $((elapsed % 30)) -eq 0 ] && [ $elapsed -gt 0 ]; then
            log_message INFO "Still waiting for: $description (${elapsed}s elapsed)"
        fi
    done
    
    log_message ERROR "Timeout waiting for: $description (${timeout}s)"
    return 1
}

# Wait for anax to be ready
wait_for_anax() {
    local timeout="${1:-120}"
    
    wait_for_condition \
        "Anax to be ready" \
        "curl -sS ${ANAX_API}/status | jq -r '.geth' | grep -q 'not configured'" \
        "$timeout" \
        5
}

# Wait for agreement to be formed
wait_for_agreement() {
    local pattern="${1:-}"
    local timeout="${2:-$AGREEMENT_TIMEOUT}"
    
    if [ -z "$pattern" ]; then
        # Wait for any agreement
        wait_for_condition \
            "Agreement to be formed" \
            "curl -sS ${ANAX_API}/agreement | jq -r '.[].current_agreement_id' | grep -q ." \
            "$timeout" \
            5
    else
        # Wait for specific pattern agreement
        wait_for_condition \
            "Agreement for pattern $pattern" \
            "curl -sS ${ANAX_API}/agreement | jq -r '.[] | select(.pattern==\"$pattern\") | .current_agreement_id' | grep -q ." \
            "$timeout" \
            5
    fi
}

# Wait for service to be running
wait_for_service() {
    local service_url="$1"
    local timeout="${2:-$SERVICE_TIMEOUT}"
    
    wait_for_condition \
        "Service $service_url to be running" \
        "curl -sS ${ANAX_API}/service | jq -r '.instances.active[] | select(.ref_url==\"$service_url\") | .instance_id' | grep -q ." \
        "$timeout" \
        5
}

# Wait for agreement count
wait_for_agreement_count() {
    local expected_count="$1"
    local timeout="${2:-$AGREEMENT_TIMEOUT}"
    
    wait_for_condition \
        "$expected_count agreements to be formed" \
        "[ \$(curl -sS ${ANAX_API}/agreement | jq '. | length') -eq $expected_count ]" \
        "$timeout" \
        5
}

# Wait for service count
wait_for_service_count() {
    local expected_count="$1"
    local timeout="${2:-$SERVICE_TIMEOUT}"
    
    wait_for_condition \
        "$expected_count services to be running" \
        "[ \$(curl -sS ${ANAX_API}/service | jq '.instances.active | length') -eq $expected_count ]" \
        "$timeout" \
        5
}

# Retry a command with exponential backoff
retry_command() {
    local max_attempts="${1:-3}"
    local delay="${2:-5}"
    shift 2
    local command="$*"
    
    local attempt=1
    while [ "$attempt" -le "$max_attempts" ]; do
        log_message INFO "Attempt $attempt/$max_attempts: $command"
        
        if eval "$command"; then
            log_message INFO "Command succeeded on attempt $attempt"
            return 0
        fi
        
        if [ "$attempt" -lt "$max_attempts" ]; then
            local wait_time=$((delay * attempt))
            log_message WARN "Command failed, retrying in ${wait_time}s..."
            sleep $wait_time
        fi
        
        ((attempt++))
    done
    
    log_message ERROR "Command failed after $max_attempts attempts"
    return 1
}

# Check if anax is running
is_anax_running() {
    if [ -z "${ANAX_API:-}" ]; then
        return 1
    fi
    
    curl -sS "${ANAX_API}/status" > /dev/null 2>&1
    return $?
}

# Check if exchange is accessible
is_exchange_accessible() {
    if [ -z "${EXCH_APP_HOST:-}" ]; then
        return 1
    fi
    
    curl -sS "${EXCH_APP_HOST}/v1/admin/version" > /dev/null 2>&1
    return $?
}

# Get anax status
get_anax_status() {
    if ! is_anax_running; then
        echo "Anax is not running"
        return 1
    fi
    
    curl -sS "${ANAX_API}/status" | jq .
}

# Get active agreements
get_active_agreements() {
    if ! is_anax_running; then
        echo "[]"
        return 1
    fi
    
    curl -sS "${ANAX_API}/agreement" | jq '[.[] | select(.archived==false)]'
}

# Get active services
get_active_services() {
    if ! is_anax_running; then
        echo "[]"
        return 1
    fi
    
    curl -sS "${ANAX_API}/service" | jq '.instances.active // []'
}

# Cancel all agreements
cancel_all_agreements() {
    log_message INFO "Cancelling all agreements"
    
    local agreements
    agreements=$(get_active_agreements | jq -r '.[].current_agreement_id')
    
    if [ -z "$agreements" ]; then
        log_message INFO "No active agreements to cancel"
        return 0
    fi
    
    for ag_id in $agreements; do
        log_message INFO "Cancelling agreement: $ag_id"
        curl -sS -X DELETE "${ANAX_API}/agreement/${ag_id}" > /dev/null 2>&1
    done
    
    # Wait for agreements to be archived
    wait_for_condition \
        "All agreements to be cancelled" \
        "[ \$(curl -sS ${ANAX_API}/agreement | jq '[.[] | select(.archived==false)] | length') -eq 0 ]" \
        60 \
        2
}

# Unregister node
unregister_node() {
    log_message INFO "Unregistering node"
    
    if ! is_anax_running; then
        log_message WARN "Anax is not running, cannot unregister"
        return 1
    fi
    
    # Cancel all agreements first
    cancel_all_agreements
    
    # Unregister
    curl -sS -X DELETE "${ANAX_API}/node" > /dev/null 2>&1
    
    # Wait for node to be unregistered
    wait_for_condition \
        "Node to be unregistered" \
        "curl -sS ${ANAX_API}/node | jq -r '.configstate.state' | grep -q 'unconfigured'" \
        30 \
        2
}

# Register node with pattern
register_node_pattern() {
    local pattern="$1"
    local node_id="${2:-testnode}"
    local node_token="${3:-testtoken}"
    
    log_message INFO "Registering node with pattern: $pattern"
    
    local reg_data
    reg_data=$(cat <<EOF
{
    "id": "$node_id",
    "token": "$node_token",
    "organization": "${DEVICE_ORG}",
    "pattern": "$pattern"
}
EOF
)
    
    curl -sS -X POST "${ANAX_API}/node" \
        -H "Content-Type: application/json" \
        -d "$reg_data" > /dev/null 2>&1
    
    # Wait for registration to complete
    wait_for_condition \
        "Node registration to complete" \
        "curl -sS ${ANAX_API}/node | jq -r '.configstate.state' | grep -q 'configured'" \
        30 \
        2
}

# Register node with policy
register_node_policy() {
    local node_id="${1:-testnode}"
    local node_token="${2:-testtoken}"
    local node_policy="${3:-}"
    
    log_message INFO "Registering node with policy"
    
    local reg_data
    reg_data=$(cat <<EOF
{
    "id": "$node_id",
    "token": "$node_token",
    "organization": "${DEVICE_ORG}"
}
EOF
)
    
    curl -sS -X POST "${ANAX_API}/node" \
        -H "Content-Type: application/json" \
        -d "$reg_data" > /dev/null 2>&1
    
    # Set node policy if provided
    if [ -n "$node_policy" ]; then
        curl -sS -X PUT "${ANAX_API}/node/policy" \
            -H "Content-Type: application/json" \
            -d "$node_policy" > /dev/null 2>&1
    fi
    
    # Wait for registration to complete
    wait_for_condition \
        "Node registration to complete" \
        "curl -sS ${ANAX_API}/node | jq -r '.configstate.state' | grep -q 'configured'" \
        30 \
        2
}

# Clean up Docker containers
cleanup_docker_containers() {
    local pattern="${1:-horizon}"
    
    log_message INFO "Cleaning up Docker containers matching: $pattern"
    
    local containers
    containers=$(docker ps -a --filter "name=$pattern" -q)
    
    if [ -n "$containers" ]; then
        # shellcheck disable=SC2086
        docker rm -f $containers > /dev/null 2>&1
        log_message INFO "Removed $(echo "$containers" | wc -w) containers"
    else
        log_message INFO "No containers to clean up"
    fi
}

# Clean up Docker networks
cleanup_docker_networks() {
    local pattern="${1:-horizon}"
    
    log_message INFO "Cleaning up Docker networks matching: $pattern"
    
    local networks
    networks=$(docker network ls --filter "name=$pattern" -q)
    
    if [ -n "$networks" ]; then
        # shellcheck disable=SC2086
        docker network rm $networks > /dev/null 2>&1
        log_message INFO "Removed $(echo "$networks" | wc -w) networks"
    else
        log_message INFO "No networks to clean up"
    fi
}

# Verify test prerequisites
verify_prerequisites() {
    local missing_prereqs=()
    
    # Check required commands
    for cmd in curl jq docker; do
        if ! command -v $cmd > /dev/null 2>&1; then
            missing_prereqs+=("$cmd")
        fi
    done
    
    # Check required environment variables
    for var in ANAX_API EXCH_APP_HOST DEVICE_ORG; do
        if [ -z "${!var:-}" ]; then
            missing_prereqs+=("$var (environment variable)")
        fi
    done
    
    if [ ${#missing_prereqs[@]} -gt 0 ]; then
        log_message ERROR "Missing prerequisites:"
        for prereq in "${missing_prereqs[@]}"; do
            log_message ERROR "  - $prereq"
        done
        return 1
    fi
    
    log_message INFO "All prerequisites verified"
    return 0
}

# Assert condition
assert() {
    local condition="$1"
    local message="${2:-Assertion failed}"
    
    if ! eval "$condition"; then
        log_message ERROR "ASSERTION FAILED: $message"
        log_message ERROR "Condition: $condition"
        return 1
    fi
    
    return 0
}

# Assert equals
assert_equals() {
    local expected="$1"
    local actual="$2"
    local message="${3:-Values not equal}"
    
    if [ "$expected" != "$actual" ]; then
        log_message ERROR "ASSERTION FAILED: $message"
        log_message ERROR "Expected: $expected"
        log_message ERROR "Actual: $actual"
        return 1
    fi
    
    return 0
}

# Assert not empty
assert_not_empty() {
    local value="$1"
    local message="${2:-Value is empty}"
    
    if [ -z "$value" ]; then
        log_message ERROR "ASSERTION FAILED: $message"
        return 1
    fi
    
    return 0
}

# Capture test metrics
capture_metrics() {
    local test_name="$1"
    local metrics_file="${TEST_RESULTS_DIR}/metrics_${test_name}.json"
    
    local metrics
    metrics=$(cat <<EOF
{
    "timestamp": "$(date -Iseconds)",
    "test_name": "$test_name",
    "anax_status": $(get_anax_status 2>/dev/null || echo "null"),
    "active_agreements": $(get_active_agreements 2>/dev/null || echo "[]"),
    "active_services": $(get_active_services 2>/dev/null || echo "[]"),
    "docker_containers": $(docker ps --format json 2>/dev/null | jq -s . || echo "[]"),
    "system_info": {
        "load_average": "$(uptime | awk -F'load average:' '{print $2}' | xargs)",
        "memory_usage": "$(free -m | awk 'NR==2{printf "%.2f%%", $3*100/$2}')",
        "disk_usage": "$(df -h / | awk 'NR==2{print $5}')"
    }
}
EOF
)
    
    echo "$metrics" > "$metrics_file"
    log_message DEBUG "Metrics captured to: $metrics_file"
}

# Export all functions
export -f wait_for_condition
export -f wait_for_anax
export -f wait_for_agreement
export -f wait_for_service
export -f wait_for_agreement_count
export -f wait_for_service_count
export -f retry_command
export -f is_anax_running
export -f is_exchange_accessible
export -f get_anax_status
export -f get_active_agreements
export -f get_active_services
export -f cancel_all_agreements
export -f unregister_node
export -f register_node_pattern
export -f register_node_policy
export -f cleanup_docker_containers
export -f cleanup_docker_networks
export -f verify_prerequisites
export -f assert
export -f assert_equals
export -f assert_not_empty
export -f capture_metrics
