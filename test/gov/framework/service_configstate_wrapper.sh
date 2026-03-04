#!/bin/bash

# Wrapper for service_configstate_test.sh using the test framework
# Demonstrates service configuration state testing with framework features

# Source test framework
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PARENT_DIR="$(pwd)"
source "${SCRIPT_DIR}/test_config.sh"
source "${SCRIPT_DIR}/test_utils.sh"

# Test configuration
TEST_NAME="service_configstate"

log_message INFO "Starting service configuration state test"

# Verify prerequisites
if ! verify_prerequisites; then
    log_message ERROR "Prerequisites check failed"
    exit 1
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
log_message INFO "Capturing initial service state"
initial_services=$(get_active_services | jq '. | length')
log_message INFO "Initial active services: $initial_services"
capture_metrics "${TEST_NAME}_start"

# Verify we have services running
if [ "$initial_services" -eq 0 ]; then
    log_message WARN "No active services found"
    # Wait for services to start
    if ! wait_for_service_count 1 120; then
        log_message ERROR "No services started, cannot test configuration state"
        exit 1
    fi
    initial_services=$(get_active_services | jq '. | length')
    log_message INFO "Services started: $initial_services"
fi

# Run the service config state test
log_message INFO "Running service configuration state test"
if [ "$TEST_RETRY_ENABLED" == "1" ]; then
    retry_command "$TEST_MAX_RETRIES" "$TEST_RETRY_DELAY" "${PARENT_DIR}/service_configstate_test.sh"
    result=$?
else
    "${PARENT_DIR}/service_configstate_test.sh"
    result=$?
fi

# Capture final state
log_message INFO "Capturing final service state"
capture_metrics "${TEST_NAME}_end"

# Verify services are still running
final_services=$(get_active_services | jq '. | length')
log_message INFO "Final active services: $final_services"

if [ $result -eq 0 ]; then
    log_message INFO "Service configuration state test PASSED"
    
    # Verify service state consistency
    if [ "$final_services" -lt "$initial_services" ]; then
        log_message WARN "Service count decreased from $initial_services to $final_services"
    else
        log_message INFO "Service count stable or increased: $initial_services -> $final_services"
    fi
    
    # Check for any services in error state
    error_services=$(get_active_services | jq '[.[] | select(.execution_failure_code != 0)] | length')
    if [ "$error_services" -gt 0 ]; then
        log_message WARN "Found $error_services services in error state"
    fi
else
    log_message ERROR "Service configuration state test FAILED"
    
    # Collect diagnostic information
    log_message ERROR "Active services:"
    get_active_services | jq . || echo "Failed to get services"
    
    log_message ERROR "Service details:"
    curl -sS "${ANAX_API}/service" | jq . || echo "Failed to get service details"
    
    log_message ERROR "Anax status:"
    get_anax_status | jq . || echo "Failed to get status"
    
    # Check for container issues
    log_message ERROR "Docker containers:"
    docker ps -a --filter "name=horizon" || echo "Failed to list containers"
fi

exit $result
