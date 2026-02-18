#!/bin/bash

# Wrapper for del_loop.sh using the test framework
# Demonstrates agreement deletion test with framework features

# Source test framework
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/test_config.sh"
source "${SCRIPT_DIR}/test_utils.sh"

# Test configuration
TEST_NAME="agreement_deletion"
TIMEOUT=$(get_timeout 300)

log_message INFO "Starting agreement deletion test"

# Verify prerequisites
if ! verify_prerequisites; then
    log_message ERROR "Prerequisites check failed"
    exit 1
fi

if ! is_anax_running; then
    log_message ERROR "Anax is not running"
    exit 1
fi

# Capture initial state
log_message INFO "Capturing initial state"
initial_agreements=$(get_active_agreements | jq '. | length')
log_message INFO "Initial active agreements: $initial_agreements"
capture_metrics "${TEST_NAME}_start"

# Verify we have agreements to delete
if [ "$initial_agreements" -eq 0 ]; then
    log_message WARN "No active agreements to delete"
    # Wait for agreements to form
    if ! wait_for_agreement "" 120; then
        log_message ERROR "No agreements formed, cannot test deletion"
        exit 1
    fi
    initial_agreements=$(get_active_agreements | jq '. | length')
    log_message INFO "Agreements formed: $initial_agreements"
fi

# Run the deletion test
log_message INFO "Running agreement deletion test"
if [ "$TEST_RETRY_ENABLED" == "1" ]; then
    retry_command $TEST_MAX_RETRIES $TEST_RETRY_DELAY "${SCRIPT_DIR}/del_loop.sh"
    result=$?
else
    "${SCRIPT_DIR}/del_loop.sh"
    result=$?
fi

# Capture final state
log_message INFO "Capturing final state"
capture_metrics "${TEST_NAME}_end"

# Verify agreements were deleted
if [ $result -eq 0 ]; then
    log_message INFO "Agreement deletion test completed"
    
    # Wait for agreements to be archived
    if wait_for_condition \
        "Agreements to be archived" \
        "[ \$(curl -sS ${ANAX_API}/agreement | jq '[.[] | select(.archived==false)] | length') -eq 0 ]" \
        60 \
        5; then
        log_message INFO "All agreements successfully archived"
    else
        log_message WARN "Some agreements may not be archived yet"
    fi
    
    # Wait for agreements to reform if NOLOOP is not set
    if [ "$NOLOOP" != "1" ]; then
        log_message INFO "Waiting for agreements to reform"
        if wait_for_agreement "" 120; then
            final_agreements=$(get_active_agreements | jq '. | length')
            log_message INFO "Agreements reformed: $final_agreements"
        else
            log_message WARN "Agreements did not reform within timeout"
        fi
    fi
else
    log_message ERROR "Agreement deletion test failed"
    
    # Collect diagnostic information
    log_message ERROR "Current agreements:"
    get_active_agreements | jq . || echo "Failed to get agreements"
    
    log_message ERROR "Anax status:"
    get_anax_status | jq . || echo "Failed to get status"
fi

exit $result
