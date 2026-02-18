#!/bin/bash

# Example wrapper for verify_agreements.sh using the test framework
# This demonstrates how to adapt existing tests to use the new framework

# Source test framework
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PARENT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
source "${SCRIPT_DIR}/test_config.sh"
source "${SCRIPT_DIR}/test_utils.sh"

# Test configuration
TEST_NAME="verify_agreements"
TIMEOUT=$(get_timeout 300)

# Parse arguments
ORG_ID="${ORG_ID:-${DEVICE_ORG}}"
ADMIN_AUTH="${ADMIN_AUTH:-e2edevadmin:e2edevadminpw}"

log_message INFO "Starting agreement verification test"
log_message INFO "Organization: $ORG_ID"
log_message INFO "Timeout: ${TIMEOUT}s"

# Verify prerequisites
if ! is_anax_running; then
    log_message ERROR "Anax is not running"
    exit 1
fi

if ! is_exchange_accessible; then
    log_message ERROR "Exchange is not accessible"
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
capture_metrics "${TEST_NAME}_start"

# Run the actual test with retry logic
log_message INFO "Running agreement verification"
if [ "$TEST_RETRY_ENABLED" == "1" ]; then
    retry_command $TEST_MAX_RETRIES $TEST_RETRY_DELAY \
        "ORG_ID=$ORG_ID ADMIN_AUTH=$ADMIN_AUTH ${SCRIPT_DIR}/verify_agreements.sh"
    result=$?
else
    ORG_ID=$ORG_ID ADMIN_AUTH=$ADMIN_AUTH "${PARENT_DIR}/verify_agreements.sh"
    result=$?
fi

# Capture final state
log_message INFO "Capturing final state"
capture_metrics "${TEST_NAME}_end"

# Report results
if [ $result -eq 0 ]; then
    log_message INFO "Agreement verification PASSED"
    
    # Additional validation
    agreement_count=$(get_active_agreements | jq '. | length')
    log_message INFO "Active agreements: $agreement_count"
    
    if [ "$agreement_count" -eq 0 ]; then
        log_message WARN "No active agreements found"
    fi
else
    log_message ERROR "Agreement verification FAILED"
    
    # Collect diagnostic information
    log_message ERROR "Anax status:"
    get_anax_status | jq . || echo "Failed to get status"
    
    log_message ERROR "Active agreements:"
    get_active_agreements | jq . || echo "Failed to get agreements"
    
    log_message ERROR "Active services:"
    get_active_services | jq . || echo "Failed to get services"
fi

exit $result
