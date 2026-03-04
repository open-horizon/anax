#!/bin/bash

# Wrapper for hzn_nmp.sh using the test framework
# Tests node management policy (NMP) functionality

# Source test framework
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PARENT_DIR="$(pwd)"
source "${SCRIPT_DIR}/test_config.sh"
source "${SCRIPT_DIR}/test_utils.sh"

# Test configuration
TEST_NAME="hzn_nmp"

log_message INFO "Starting hzn NMP test"

# Verify prerequisites
if ! verify_prerequisites; then
    log_message ERROR "Prerequisites check failed"
    exit 1
fi

# Verify hzn CLI is available
if ! command -v hzn &> /dev/null; then
    log_message ERROR "hzn CLI not found"
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

# Verify node is registered
log_message INFO "Verifying node registration"
if ! hzn node list 2>&1 | grep -q "configured"; then
    log_message WARN "Node may not be properly registered"
fi

# Capture initial state
log_message INFO "Capturing initial NMP state"
capture_metrics "${TEST_NAME}_start"
hzn node management status > /tmp/${TEST_NAME}_status_before.json 2>&1

# Run the NMP test
log_message INFO "Running hzn NMP test"
if [ "$TEST_RETRY_ENABLED" == "1" ]; then
    retry_command "$TEST_MAX_RETRIES" "$TEST_RETRY_DELAY" "${PARENT_DIR}/hzn_nmp.sh"
    result=$?
else
    "${PARENT_DIR}/hzn_nmp.sh"
    result=$?
fi

# Capture final state
log_message INFO "Capturing final NMP state"
capture_metrics "${TEST_NAME}_end"
hzn node management status > /tmp/${TEST_NAME}_status_after.json 2>&1

# Verify node is still operational
log_message INFO "Verifying node is still operational"
if ! is_anax_running; then
    log_message ERROR "Anax stopped running during NMP test"
    result=1
fi

# Report results
if [ $result -eq 0 ]; then
    log_message INFO "HZN NMP test PASSED"
else
    log_message ERROR "HZN NMP test FAILED"
    
    # Collect diagnostic information
    log_message ERROR "Node status:"
    hzn node list 2>&1 || echo "Failed to get node status"
    
    log_message ERROR "Node management status:"
    hzn node management status 2>&1 || echo "Failed to get management status"
    
    log_message ERROR "Recent anax logs (last 50 lines):"
    if [ -f "/tmp/anax.log" ]; then
        tail -50 /tmp/anax.log
    else
        echo "Log file not found"
    fi
fi

# Cleanup temporary files
rm -f /tmp/${TEST_NAME}_status_before.json /tmp/${TEST_NAME}_status_after.json

exit $result
