#!/bin/bash

# Wrapper for pattern_change.sh using the test framework
# Demonstrates pattern change testing with framework features

# Source test framework
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/test_config.sh"
source "${SCRIPT_DIR}/test_utils.sh"

# Test configuration
TEST_NAME="pattern_change"
TIMEOUT=$(get_timeout 300)

log_message INFO "Starting pattern change test"

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
initial_pattern=$(curl -sS "${ANAX_API}/node" | jq -r '.pattern // "none"')
initial_agreements=$(get_active_agreements | jq '. | length')
log_message INFO "Initial pattern: $initial_pattern"
log_message INFO "Initial agreements: $initial_agreements"
capture_metrics "${TEST_NAME}_start"

# Verify node is registered with a pattern
if [ "$initial_pattern" == "none" ] || [ "$initial_pattern" == "null" ]; then
    log_message ERROR "Node is not registered with a pattern"
    exit 1
fi

# Run the pattern change test
log_message INFO "Running pattern change test"
if [ "$TEST_RETRY_ENABLED" == "1" ]; then
    retry_command $TEST_MAX_RETRIES $TEST_RETRY_DELAY "${SCRIPT_DIR}/pattern_change.sh"
    result=$?
else
    "${SCRIPT_DIR}/pattern_change.sh"
    result=$?
fi

# Capture final state
log_message INFO "Capturing final state"
final_pattern=$(curl -sS "${ANAX_API}/node" | jq -r '.pattern // "none"')
final_agreements=$(get_active_agreements | jq '. | length')
log_message INFO "Final pattern: $final_pattern"
log_message INFO "Final agreements: $final_agreements"
capture_metrics "${TEST_NAME}_end"

# Report results
if [ $result -eq 0 ]; then
    log_message INFO "Pattern change test PASSED"
    
    # Verify pattern actually changed
    if [ "$initial_pattern" != "$final_pattern" ]; then
        log_message INFO "Pattern successfully changed: $initial_pattern -> $final_pattern"
    else
        log_message WARN "Pattern did not change (may be expected for test)"
    fi
    
    # Verify agreements reformed
    if [ "$final_agreements" -gt 0 ]; then
        log_message INFO "Agreements reformed after pattern change: $final_agreements"
    else
        log_message WARN "No agreements after pattern change"
    fi
else
    log_message ERROR "Pattern change test FAILED"
    
    # Collect diagnostic information
    log_message ERROR "Node configuration:"
    curl -sS "${ANAX_API}/node" | jq . || echo "Failed to get node config"
    
    log_message ERROR "Active agreements:"
    get_active_agreements | jq . || echo "Failed to get agreements"
    
    log_message ERROR "Anax status:"
    get_anax_status | jq . || echo "Failed to get status"
fi

exit $result
