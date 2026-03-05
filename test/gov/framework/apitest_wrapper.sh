#!/bin/bash

# Wrapper for apitest.sh using the test framework
# Demonstrates API testing with framework features

# Source test framework
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PARENT_DIR="$(pwd)"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/test_config.sh"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/test_utils.sh"

# Test configuration
TEST_NAME="api_tests"

log_message INFO "Starting API tests"

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

# Wait for anax to be fully ready
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

# Capture initial state
log_message INFO "Capturing initial API state"
capture_metrics "${TEST_NAME}_start"

# Test basic API endpoints
log_message INFO "Testing basic API endpoints"
endpoints=(
    "/status"
    "/node"
    "/agreement"
    "/service"
    "/attribute"
)

for endpoint in "${endpoints[@]}"; do
    log_message DEBUG "Testing endpoint: $endpoint"
    if ! curl -sS "${ANAX_API}${endpoint}" > /dev/null 2>&1; then
        log_message WARN "Endpoint $endpoint not accessible"
    fi
done

# Run the actual API test suite
log_message INFO "Running comprehensive API test suite"
if [ "$TEST_RETRY_ENABLED" == "1" ]; then
    retry_command "$TEST_MAX_RETRIES" "$TEST_RETRY_DELAY" "${PARENT_DIR}/apitest.sh"
    result=$?
else
    "${PARENT_DIR}/apitest.sh"
    result=$?
fi

# Capture final state
log_message INFO "Capturing final API state"
capture_metrics "${TEST_NAME}_end"

# Verify API is still responsive after tests
log_message INFO "Verifying API is still responsive"
if ! curl -sS "${ANAX_API}/status" > /dev/null 2>&1; then
    log_message ERROR "API became unresponsive after tests"
    result=1
fi

# Report results
if [ $result -eq 0 ]; then
    log_message INFO "API tests PASSED"
    
    # Verify no unexpected side effects
    status=$(get_anax_status)
    if [ -z "$status" ]; then
        log_message WARN "Unable to get anax status after tests"
    else
        log_message INFO "Anax status after tests: OK"
    fi
else
    log_message ERROR "API tests FAILED"
    
    # Collect diagnostic information
    log_message ERROR "Anax status:"
    get_anax_status | jq . || echo "Failed to get status"
    
    log_message ERROR "Recent anax logs (last 50 lines):"
    if [ -f "/tmp/anax.log" ]; then
        tail -50 /tmp/anax.log
    else
        echo "Log file not found"
    fi
fi

exit $result
