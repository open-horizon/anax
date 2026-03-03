#!/bin/bash

# Wrapper for policy_change.sh using the test framework
# Demonstrates policy change testing with framework features

# Source test framework
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PARENT_DIR="$(pwd)"
source "${SCRIPT_DIR}/test_config.sh"
source "${SCRIPT_DIR}/test_utils.sh"

# Test configuration
TEST_NAME="policy_change"
TIMEOUT=$(get_timeout 300)

log_message INFO "Starting policy change test"

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

# Get initial node policy
initial_policy=$(curl -sS "${ANAX_API}/node/policy" 2>/dev/null)
if [ -n "$initial_policy" ]; then
    log_message INFO "Initial node policy exists"
else
    log_message WARN "No initial node policy found"
fi

capture_metrics "${TEST_NAME}_start"

# Run the policy change test
log_message INFO "Running policy change test"
if [ "$TEST_RETRY_ENABLED" == "1" ]; then
    retry_command $TEST_MAX_RETRIES $TEST_RETRY_DELAY "${PARENT_DIR}/policy_change.sh"
    result=$?
else
    "${PARENT_DIR}/policy_change.sh"
    result=$?
fi

# Capture final state
log_message INFO "Capturing final state"
final_agreements=$(get_active_agreements | jq '. | length')
log_message INFO "Final agreements: $final_agreements"

# Get final node policy
final_policy=$(curl -sS "${ANAX_API}/node/policy" 2>/dev/null)
capture_metrics "${TEST_NAME}_end"

# Report results
if [ $result -eq 0 ]; then
    log_message INFO "Policy change test PASSED"
    
    # Verify agreements reformed
    if [ "$final_agreements" -gt 0 ]; then
        log_message INFO "Agreements reformed after policy change: $final_agreements"
    else
        log_message WARN "No agreements after policy change"
    fi
    
    # Check if policy actually changed
    if [ "$initial_policy" != "$final_policy" ]; then
        log_message INFO "Node policy was modified during test"
    fi
else
    log_message ERROR "Policy change test FAILED"
    
    # Collect diagnostic information
    log_message ERROR "Node policy:"
    curl -sS "${ANAX_API}/node/policy" | jq . || echo "Failed to get node policy"
    
    log_message ERROR "Active agreements:"
    get_active_agreements | jq . || echo "Failed to get agreements"
    
    log_message ERROR "Anax status:"
    get_anax_status | jq . || echo "Failed to get status"
    
    log_message ERROR "Node configuration:"
    curl -sS "${ANAX_API}/node" | jq . || echo "Failed to get node config"
fi

exit $result
