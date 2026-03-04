#!/bin/bash

# Wrapper for metering_apitest.sh using the test framework
# Tests metering API functionality

# Source test framework
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PARENT_DIR="$(pwd)"
source "${SCRIPT_DIR}/test_config.sh"
source "${SCRIPT_DIR}/test_utils.sh"

# Test configuration
TEST_NAME="metering_apitest"

log_message INFO "Starting metering API test"

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

# Verify API is responding
log_message INFO "Verifying API responsiveness"
if ! retry_command 3 5 "curl -sS ${ANAX_API}/status > /dev/null"; then
    log_message ERROR "API is not responding"
    exit 1
fi

# Verify agreements exist for metering
log_message INFO "Verifying agreements exist"
if ! wait_for_agreements 1 120; then
    log_message WARN "No agreements found, metering test may not work correctly"
fi

# Capture initial state
log_message INFO "Capturing initial metering state"
capture_metrics "${TEST_NAME}_start"

# Run the metering API test
log_message INFO "Running metering API test"
if [ "$TEST_RETRY_ENABLED" == "1" ]; then
    retry_command "$TEST_MAX_RETRIES" "$TEST_RETRY_DELAY" "${PARENT_DIR}/metering_apitest.sh"
    result=$?
else
    "${PARENT_DIR}/metering_apitest.sh"
    result=$?
fi

# Capture final state
log_message INFO "Capturing final metering state"
capture_metrics "${TEST_NAME}_end"

# Verify API is still responsive after tests
log_message INFO "Verifying API is still responsive"
if ! curl -sS "${ANAX_API}/status" > /dev/null 2>&1; then
    log_message ERROR "API became unresponsive after metering tests"
    result=1
fi

# Report results
if [ $result -eq 0 ]; then
    log_message INFO "Metering API test PASSED"
else
    log_message ERROR "Metering API test FAILED"
    
    # Collect diagnostic information
    log_message ERROR "Agreement status:"
    curl -sS "${ANAX_API}/agreement" | jq . || echo "Failed to get agreements"
    
    log_message ERROR "Metering status:"
    curl -sS "${ANAX_API}/metering" | jq . || echo "Failed to get metering data"
    
    log_message ERROR "Recent anax logs (last 50 lines):"
    if [ -f "/tmp/anax.log" ]; then
        tail -50 /tmp/anax.log
    else
        echo "Log file not found"
    fi
fi

exit $result
