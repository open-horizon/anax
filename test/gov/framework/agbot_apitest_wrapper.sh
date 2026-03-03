#!/bin/bash

# Wrapper for agbot_apitest.sh using the test framework
# Demonstrates agbot API testing with framework features

# Source test framework
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PARENT_DIR="$(pwd)"
source "${SCRIPT_DIR}/test_config.sh"
source "${SCRIPT_DIR}/test_utils.sh"

# Test configuration
TEST_NAME="agbot_apitest"
TIMEOUT=$(get_timeout $API_TIMEOUT)

log_message INFO "Starting agbot API test"

# Verify prerequisites
if ! verify_prerequisites; then
    log_message ERROR "Prerequisites check failed"
    exit 1
fi

# Check if agbot is disabled
if [ "${NOAGBOT:-0}" == "1" ]; then
    log_message INFO "Agbot tests are disabled, skipping"
    exit 0
fi

# Verify agbot is accessible
if [ -z "${AGBOT_API:-}" ]; then
    log_message ERROR "AGBOT_API not set"
    exit 1
fi

log_message INFO "Verifying agbot accessibility at ${AGBOT_API}"
if ! curl -sS "${AGBOT_API}/agreement" > /dev/null 2>&1; then
    log_message ERROR "Agbot is not accessible"
    exit 1
fi

# Capture initial state
log_message INFO "Capturing initial agbot state"
capture_metrics "${TEST_NAME}_start"

# Get initial agbot agreements
initial_agbot_agreements=$(curl -sS "${AGBOT_API}/agreement" | jq '. | length' 2>/dev/null || echo "0")
log_message INFO "Initial agbot agreements: $initial_agbot_agreements"

# Run the agbot API test
log_message INFO "Running agbot API test"
if [ "$TEST_RETRY_ENABLED" == "1" ]; then
    retry_command $TEST_MAX_RETRIES $TEST_RETRY_DELAY "${PARENT_DIR}/agbot_apitest.sh"
    result=$?
else
    "${PARENT_DIR}/agbot_apitest.sh"
    result=$?
fi

# Capture final state
log_message INFO "Capturing final agbot state"
capture_metrics "${TEST_NAME}_end"

# Get final agbot agreements
final_agbot_agreements=$(curl -sS "${AGBOT_API}/agreement" | jq '. | length' 2>/dev/null || echo "0")
log_message INFO "Final agbot agreements: $final_agbot_agreements"

# Report results
if [ $result -eq 0 ]; then
    log_message INFO "Agbot API test PASSED"
    
    # Verify agbot is still responsive
    if curl -sS "${AGBOT_API}/status" > /dev/null 2>&1; then
        log_message INFO "Agbot is still responsive"
    else
        log_message WARN "Agbot may have issues after test"
    fi
else
    log_message ERROR "Agbot API test FAILED"
    
    # Collect diagnostic information
    log_message ERROR "Agbot status:"
    curl -sS "${AGBOT_API}/status" | jq . || echo "Failed to get agbot status"
    
    log_message ERROR "Agbot agreements:"
    curl -sS "${AGBOT_API}/agreement" | jq . || echo "Failed to get agbot agreements"
    
    log_message ERROR "Agbot patterns:"
    curl -sS "${AGBOT_API}/pattern" | jq . || echo "Failed to get agbot patterns"
fi

exit $result
