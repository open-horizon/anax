#!/bin/bash

# Wrapper for hzn_reg.sh using the test framework
# Demonstrates hzn registration/unregistration testing with framework features

# Source test framework
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/test_config.sh"
source "${SCRIPT_DIR}/test_utils.sh"

# Test configuration
TEST_NAME="hzn_registration"
TIMEOUT=$(get_timeout 300)

log_message INFO "Starting hzn registration/unregistration test"

# Verify prerequisites
if ! verify_prerequisites; then
    log_message ERROR "Prerequisites check failed"
    exit 1
fi

# Check if hzn command is available
if ! command -v hzn > /dev/null 2>&1; then
    log_message ERROR "hzn command not found"
    exit 1
fi

# Verify exchange is accessible
if ! is_exchange_accessible; then
    log_message ERROR "Exchange is not accessible"
    exit 1
fi

# Capture initial state
log_message INFO "Capturing initial state"
if is_anax_running; then
    initial_state=$(curl -sS "${ANAX_API}/node" | jq -r '.configstate.state')
    log_message INFO "Initial node state: $initial_state"
else
    log_message WARN "Anax is not running initially"
    initial_state="unknown"
fi
capture_metrics "${TEST_NAME}_start"

# Run the hzn registration test
log_message INFO "Running hzn registration/unregistration test"
if [ "$TEST_RETRY_ENABLED" == "1" ]; then
    retry_command $TEST_MAX_RETRIES $TEST_RETRY_DELAY "${SCRIPT_DIR}/hzn_reg.sh"
    result=$?
else
    "${SCRIPT_DIR}/hzn_reg.sh"
    result=$?
fi

# Capture final state
log_message INFO "Capturing final state"
if is_anax_running; then
    final_state=$(curl -sS "${ANAX_API}/node" | jq -r '.configstate.state')
    log_message INFO "Final node state: $final_state"
else
    log_message WARN "Anax is not running after test"
    final_state="unknown"
fi
capture_metrics "${TEST_NAME}_end"

# Report results
if [ $result -eq 0 ]; then
    log_message INFO "hzn registration/unregistration test PASSED"
    
    # Verify hzn command still works
    if hzn version > /dev/null 2>&1; then
        log_message INFO "hzn command is functional"
    else
        log_message WARN "hzn command may have issues"
    fi
else
    log_message ERROR "hzn registration/unregistration test FAILED"
    
    # Collect diagnostic information
    log_message ERROR "hzn version:"
    hzn version || echo "Failed to get hzn version"
    
    log_message ERROR "hzn node list:"
    hzn node list || echo "Failed to list node"
    
    if is_anax_running; then
        log_message ERROR "Anax status:"
        get_anax_status | jq . || echo "Failed to get status"
        
        log_message ERROR "Node configuration:"
        curl -sS "${ANAX_API}/node" | jq . || echo "Failed to get node config"
    fi
fi

exit $result
