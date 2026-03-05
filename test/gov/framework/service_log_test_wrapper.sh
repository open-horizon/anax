#!/bin/bash

# Wrapper for service_log_test.sh using the test framework
# Tests service logging functionality

# Source test framework
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PARENT_DIR="$(pwd)"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/test_config.sh"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/test_utils.sh"

# Test configuration
TEST_NAME="service_log_test"

log_message INFO "Starting service log test"

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

# Verify services are running
log_message INFO "Verifying services are running"
if ! wait_for_service "helloworld" 120; then
    log_message WARN "Helloworld service not running, test may fail"
fi

# Capture initial state
log_message INFO "Capturing initial service state"
capture_metrics "${TEST_NAME}_start"

# Run the service log test
log_message INFO "Running service log test"
if [ "$TEST_RETRY_ENABLED" == "1" ]; then
    retry_command "$TEST_MAX_RETRIES" "$TEST_RETRY_DELAY" "${PARENT_DIR}/service_log_test.sh"
    result=$?
else
    "${PARENT_DIR}/service_log_test.sh"
    result=$?
fi

# Capture final state
log_message INFO "Capturing final service state"
capture_metrics "${TEST_NAME}_end"

# Report results
if [ $result -eq 0 ]; then
    log_message INFO "Service log test PASSED"
else
    log_message ERROR "Service log test FAILED"
    
    # Collect diagnostic information
    log_message ERROR "Service status:"
    curl -sS "${ANAX_API}/service" | jq . || echo "Failed to get service status"
    
    log_message ERROR "Recent anax logs (last 50 lines):"
    if [ -f "/tmp/anax.log" ]; then
        tail -50 /tmp/anax.log
    else
        echo "Log file not found"
    fi
fi

exit $result
