#!/bin/bash

# Wrapper for service_secrets_test.sh using the test framework
# Demonstrates service secrets testing with framework features

# Source test framework
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PARENT_DIR="$(pwd)"
source "${SCRIPT_DIR}/test_config.sh"
source "${SCRIPT_DIR}/test_utils.sh"

# Test configuration
TEST_NAME="service_secrets"
TIMEOUT=$(get_timeout 300)

log_message INFO "Starting service secrets test"

# Verify prerequisites
if ! verify_prerequisites; then
    log_message ERROR "Prerequisites check failed"
    exit 1
fi

if ! is_anax_running; then
    log_message ERROR "Anax is not running"
    exit 1
fi

# Check if vault is available (if required)
if [ "${NOVAULT:-0}" == "1" ]; then
    log_message INFO "Vault tests are disabled, skipping"
    exit 0
fi

# Wait for anax to be ready
log_message INFO "Waiting for anax to be ready"
if ! wait_for_anax 60; then
    log_message ERROR "Anax is not ready"
    exit 1
fi

# Capture initial state
log_message INFO "Capturing initial state"
initial_services=$(get_active_services | jq '. | length')
log_message INFO "Initial active services: $initial_services"
capture_metrics "${TEST_NAME}_start"

# Run the service secrets test
log_message INFO "Running service secrets test"
if [ "$TEST_RETRY_ENABLED" == "1" ]; then
    retry_command $TEST_MAX_RETRIES $TEST_RETRY_DELAY "${PARENT_DIR}/service_secrets_test.sh"
    result=$?
else
    "${PARENT_DIR}/service_secrets_test.sh"
    result=$?
fi

# Capture final state
log_message INFO "Capturing final state"
final_services=$(get_active_services | jq '. | length')
log_message INFO "Final active services: $final_services"
capture_metrics "${TEST_NAME}_end"

# Report results
if [ $result -eq 0 ]; then
    log_message INFO "Service secrets test PASSED"
    
    # Verify services are still running
    if [ "$final_services" -gt 0 ]; then
        log_message INFO "Services running after secrets test: $final_services"
    else
        log_message WARN "No services running after test"
    fi
else
    log_message ERROR "Service secrets test FAILED"
    
    # Collect diagnostic information
    log_message ERROR "Active services:"
    get_active_services | jq . || echo "Failed to get services"
    
    log_message ERROR "Active agreements:"
    get_active_agreements | jq . || echo "Failed to get agreements"
    
    log_message ERROR "Anax status:"
    get_anax_status | jq . || echo "Failed to get status"
    
    # Check vault status if available
    if command -v vault > /dev/null 2>&1; then
        log_message ERROR "Vault status:"
        vault status || echo "Failed to get vault status"
    fi
fi

exit $result
