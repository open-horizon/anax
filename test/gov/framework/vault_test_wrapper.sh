#!/bin/bash

# Wrapper for vault_test.sh using the test framework
# Tests Vault integration functionality

# Source test framework
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PARENT_DIR="$(pwd)"
source "${SCRIPT_DIR}/test_config.sh"
source "${SCRIPT_DIR}/test_utils.sh"

# Test configuration
TEST_NAME="vault_test"
TIMEOUT=$(get_timeout $SERVICE_TIMEOUT)

log_message INFO "Starting Vault integration test"

# Verify prerequisites
if ! verify_prerequisites; then
    log_message ERROR "Prerequisites check failed"
    exit 1
fi

# Check if vault tests are disabled
if [ "$NOVAULT" == "1" ]; then
    log_message WARN "Vault tests disabled, skipping"
    exit 0
fi

# Verify vault is accessible
log_message INFO "Verifying Vault accessibility"
VAULT_ADDR=${VAULT_ADDR:-"http://localhost:8200"}
if ! curl -sS "${VAULT_ADDR}/v1/sys/health" > /dev/null 2>&1; then
    log_message ERROR "Vault is not accessible at ${VAULT_ADDR}"
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

# Capture initial state
log_message INFO "Capturing initial Vault state"
capture_metrics "${TEST_NAME}_start"

# Run the Vault test
log_message INFO "Running Vault integration test"
if [ "$TEST_RETRY_ENABLED" == "1" ]; then
    retry_command $TEST_MAX_RETRIES $TEST_RETRY_DELAY "${PARENT_DIR}/vault_test.sh"
    result=$?
else
    "${PARENT_DIR}/vault_test.sh"
    result=$?
fi

# Capture final state
log_message INFO "Capturing final Vault state"
capture_metrics "${TEST_NAME}_end"

# Verify Vault is still accessible after test
log_message INFO "Verifying Vault is still accessible"
if ! curl -sS "${VAULT_ADDR}/v1/sys/health" > /dev/null 2>&1; then
    log_message ERROR "Vault became inaccessible after test"
    result=1
fi

# Verify anax is still running
log_message INFO "Verifying anax is still running"
if ! is_anax_running; then
    log_message ERROR "Anax stopped running during Vault test"
    result=1
fi

# Report results
if [ $result -eq 0 ]; then
    log_message INFO "Vault integration test PASSED"
else
    log_message ERROR "Vault integration test FAILED"
    
    # Collect diagnostic information
    log_message ERROR "Vault health:"
    curl -sS "${VAULT_ADDR}/v1/sys/health" | jq . || echo "Failed to get Vault health"
    
    log_message ERROR "Node status:"
    curl -sS "${ANAX_API}/node" | jq . || echo "Failed to get node status"
    
    log_message ERROR "Recent anax logs (last 50 lines):"
    if [ -f "/tmp/anax.log" ]; then
        tail -50 /tmp/anax.log
    else
        echo "Log file not found"
    fi
fi

exit $result
