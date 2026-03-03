#!/bin/bash

# Wrapper for service_retry_test.sh using the test framework
# Tests service retry logic and recovery

# Source test framework
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PARENT_DIR="$(pwd)"
source "${SCRIPT_DIR}/test_config.sh"
source "${SCRIPT_DIR}/test_utils.sh"

# Test configuration
TEST_NAME="service_retry_test"
TIMEOUT=$(get_timeout $SERVICE_TIMEOUT)

log_message INFO "Starting service retry test"

# Verify prerequisites
if ! verify_prerequisites; then
    log_message ERROR "Prerequisites check failed"
    exit 1
fi

# Verify anax is running
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

# Verify initial agreements exist
log_message INFO "Verifying initial agreements"
if ! wait_for_agreements 1 120; then
    log_message WARN "No agreements found, test may not work correctly"
fi

# Capture initial state
log_message INFO "Capturing initial state"
capture_metrics "${TEST_NAME}_start"

# Run the service retry test
log_message INFO "Running service retry test"
if [ "$TEST_RETRY_ENABLED" == "1" ]; then
    retry_command $TEST_MAX_RETRIES $TEST_RETRY_DELAY "${PARENT_DIR}/service_retry_test.sh"
    result=$?
else
    "${PARENT_DIR}/service_retry_test.sh"
    result=$?
fi

# Capture final state
log_message INFO "Capturing final state"
capture_metrics "${TEST_NAME}_end"

# Verify anax is still running after test
log_message INFO "Verifying anax is still running"
if ! is_anax_running; then
    log_message ERROR "Anax stopped running during test"
    result=1
fi

# Report results
if [ $result -eq 0 ]; then
    log_message INFO "Service retry test PASSED"
    
    # Verify agreements recovered
    if wait_for_agreements 1 60; then
        log_message INFO "Agreements recovered successfully"
    else
        log_message WARN "Agreements did not recover as expected"
    fi
else
    log_message ERROR "Service retry test FAILED"
    
    # Collect diagnostic information
    log_message ERROR "Agreement status:"
    curl -sS "${ANAX_API}/agreement" | jq . || echo "Failed to get agreements"
    
    log_message ERROR "Service status:"
    curl -sS "${ANAX_API}/service" | jq . || echo "Failed to get services"
    
    log_message ERROR "Recent anax logs (last 50 lines):"
    if [ -f "/tmp/anax.log" ]; then
        tail -50 /tmp/anax.log
    else
        echo "Log file not found"
    fi
fi

exit $result
