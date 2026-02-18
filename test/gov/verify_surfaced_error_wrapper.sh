#!/bin/bash

# Wrapper for verify_surfaced_error.sh using the test framework
# Demonstrates surfaced error verification testing with framework features

# Source test framework
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/test_config.sh"
source "${SCRIPT_DIR}/test_utils.sh"

# Test configuration
TEST_NAME="verify_surfaced_error"
TIMEOUT=$(get_timeout 300)

log_message INFO "Starting surfaced error verification test"

# Verify prerequisites
if ! verify_prerequisites; then
    log_message ERROR "Prerequisites check failed"
    exit 1
fi

if ! is_anax_running; then
    log_message ERROR "Anax is not running"
    exit 1
fi

# Wait for anax to be ready
log_message INFO "Waiting for anax to be ready"
if ! wait_for_anax 60; then
    log_message ERROR "Anax is not ready"
    exit 1
fi

# Capture initial state
log_message INFO "Capturing initial state"
initial_agreements=$(get_active_agreements | jq '. | length')
log_message INFO "Initial agreements: $initial_agreements"
capture_metrics "${TEST_NAME}_start"

# Run the surfaced error verification test
log_message INFO "Running surfaced error verification test"
if [ "$TEST_RETRY_ENABLED" == "1" ]; then
    retry_command $TEST_MAX_RETRIES $TEST_RETRY_DELAY "${SCRIPT_DIR}/verify_surfaced_error.sh"
    result=$?
else
    "${SCRIPT_DIR}/verify_surfaced_error.sh"
    result=$?
fi

# Capture final state
log_message INFO "Capturing final state"
final_agreements=$(get_active_agreements | jq '. | length')
log_message INFO "Final agreements: $final_agreements"
capture_metrics "${TEST_NAME}_end"

# Report results
if [ $result -eq 0 ]; then
    log_message INFO "Surfaced error verification test PASSED"
    
    # Check event log for surfaced errors
    if is_anax_running; then
        error_count=$(curl -sS "${ANAX_API}/eventlog" | jq '[.[] | select(.severity=="error")] | length' 2>/dev/null || echo "0")
        log_message INFO "Error events in log: $error_count"
    fi
else
    log_message ERROR "Surfaced error verification test FAILED"
    
    # Collect diagnostic information
    log_message ERROR "Event log (last 20 entries):"
    curl -sS "${ANAX_API}/eventlog" | jq '.[-20:]' || echo "Failed to get event log"
    
    log_message ERROR "Active agreements:"
    get_active_agreements | jq . || echo "Failed to get agreements"
    
    log_message ERROR "Anax status:"
    get_anax_status | jq . || echo "Failed to get status"
fi

exit $result
