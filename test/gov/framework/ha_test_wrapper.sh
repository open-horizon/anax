#!/bin/bash

# Wrapper for ha_test.sh using the test framework
# Demonstrates high availability testing with framework features

# Source test framework
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PARENT_DIR="$(pwd)"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/test_config.sh"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/test_utils.sh"

# Test configuration
TEST_NAME="ha_test"

log_message INFO "Starting high availability (HA) test"

# Verify prerequisites
if ! verify_prerequisites; then
    log_message ERROR "Prerequisites check failed"
    exit 1
fi

# Check if HA test is enabled
if [ "${HA:-0}" != "1" ]; then
    log_message INFO "HA test is disabled, skipping"
    exit 0
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
initial_services=$(get_active_services | jq '. | length')
log_message INFO "Initial agreements: $initial_agreements"
log_message INFO "Initial services: $initial_services"
capture_metrics "${TEST_NAME}_start"

# Verify we have agreements and services for HA testing
if [ "$initial_agreements" -eq 0 ] || [ "$initial_services" -eq 0 ]; then
    log_message ERROR "Need active agreements and services for HA testing"
    exit 1
fi

# Run the HA test
log_message INFO "Running high availability test"
if [ "$TEST_RETRY_ENABLED" == "1" ]; then
    retry_command "$TEST_MAX_RETRIES" "$TEST_RETRY_DELAY" "${PARENT_DIR}/ha_test.sh"
    result=$?
else
    "${PARENT_DIR}/ha_test.sh"
    result=$?
fi

# Capture final state
log_message INFO "Capturing final state"
final_agreements=$(get_active_agreements | jq '. | length')
final_services=$(get_active_services | jq '. | length')
log_message INFO "Final agreements: $final_agreements"
log_message INFO "Final services: $final_services"
capture_metrics "${TEST_NAME}_end"

# Report results
if [ $result -eq 0 ]; then
    log_message INFO "High availability test PASSED"
    
    # Verify system recovered
    if [ "$final_agreements" -gt 0 ] && [ "$final_services" -gt 0 ]; then
        log_message INFO "System recovered successfully after HA test"
    else
        log_message WARN "System may not have fully recovered"
    fi
else
    log_message ERROR "High availability test FAILED"
    
    # Collect diagnostic information
    log_message ERROR "Active agreements:"
    get_active_agreements | jq . || echo "Failed to get agreements"
    
    log_message ERROR "Active services:"
    get_active_services | jq . || echo "Failed to get services"
    
    log_message ERROR "Anax status:"
    get_anax_status | jq . || echo "Failed to get status"
    
    log_message ERROR "Docker containers:"
    docker ps -a --filter "name=horizon" || echo "Failed to list containers"
fi

exit $result
