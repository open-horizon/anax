#!/bin/bash

# Wrapper for service_upgrading_downgrading_test.sh using the test framework
# Demonstrates service upgrade/downgrade testing with framework features

# Source test framework
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PARENT_DIR="$(pwd)"
source "${SCRIPT_DIR}/test_config.sh"
source "${SCRIPT_DIR}/test_utils.sh"

# Test configuration
TEST_NAME="service_upgrade_downgrade"

log_message INFO "Starting service upgrade/downgrade test"

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
log_message INFO "Capturing initial state"
initial_services=$(get_active_services | jq '. | length')
log_message INFO "Initial active services: $initial_services"

# Get initial service versions
initial_versions=$(get_active_services | jq -r '.[].ref_url' | sort)
log_message INFO "Initial service versions:"
echo "$initial_versions" | while read -r svc; do
    log_message INFO "  - $svc"
done

capture_metrics "${TEST_NAME}_start"

# Verify we have services to upgrade/downgrade
if [ "$initial_services" -eq 0 ]; then
    log_message ERROR "No services running, cannot test upgrade/downgrade"
    exit 1
fi

# Run the service upgrade/downgrade test
log_message INFO "Running service upgrade/downgrade test"
if [ "$TEST_RETRY_ENABLED" == "1" ]; then
    retry_command "$TEST_MAX_RETRIES" "$TEST_RETRY_DELAY" "${PARENT_DIR}/service_upgrading_downgrading_test.sh"
    result=$?
else
    "${PARENT_DIR}/service_upgrading_downgrading_test.sh"
    result=$?
fi

# Capture final state
log_message INFO "Capturing final state"
final_services=$(get_active_services | jq '. | length')
log_message INFO "Final active services: $final_services"

# Get final service versions
final_versions=$(get_active_services | jq -r '.[].ref_url' | sort)
log_message INFO "Final service versions:"
echo "$final_versions" | while read -r svc; do
    log_message INFO "  - $svc"
done

capture_metrics "${TEST_NAME}_end"

# Report results
if [ $result -eq 0 ]; then
    log_message INFO "Service upgrade/downgrade test PASSED"
    
    # Verify services are still running
    if [ "$final_services" -gt 0 ]; then
        log_message INFO "Services still running after upgrade/downgrade: $final_services"
    else
        log_message WARN "No services running after test"
    fi
    
    # Check for any services in error state
    error_services=$(get_active_services | jq '[.[] | select(.execution_failure_code != 0)] | length')
    if [ "$error_services" -gt 0 ]; then
        log_message WARN "Found $error_services services in error state"
    fi
else
    log_message ERROR "Service upgrade/downgrade test FAILED"
    
    # Collect diagnostic information
    log_message ERROR "Active services:"
    get_active_services | jq . || echo "Failed to get services"
    
    log_message ERROR "Active agreements:"
    get_active_agreements | jq . || echo "Failed to get agreements"
    
    log_message ERROR "Anax status:"
    get_anax_status | jq . || echo "Failed to get status"
    
    log_message ERROR "Docker containers:"
    docker ps -a --filter "name=horizon" || echo "Failed to list containers"
fi

exit $result
