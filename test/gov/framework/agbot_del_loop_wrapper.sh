#!/bin/bash

# Wrapper for agbot_del_loop.sh using the test framework
# Tests agreement bot deletion loop functionality

# Source test framework
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PARENT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
source "${SCRIPT_DIR}/test_config.sh"
source "${SCRIPT_DIR}/test_utils.sh"

# Test configuration
TEST_NAME="agbot_del_loop"
TIMEOUT=$(get_timeout $AGBOT_TIMEOUT)

log_message INFO "Starting agbot deletion loop test"

# Verify prerequisites
if ! verify_prerequisites; then
    log_message ERROR "Prerequisites check failed"
    exit 1
fi

# Verify agbot is accessible
if [ "$NOAGBOT" == "1" ]; then
    log_message WARN "Agbot tests disabled, skipping"
    exit 0
fi

log_message INFO "Verifying agbot accessibility"
if ! curl -sS "${AGBOT_API}/status" > /dev/null 2>&1; then
    log_message ERROR "Agbot is not accessible"
    exit 1
fi

# Verify agreements exist before deletion
log_message INFO "Checking for existing agreements"
agreement_count=$(curl -sS "${AGBOT_API}/agreement" 2>/dev/null | jq '. | length' 2>/dev/null || echo "0")
log_message INFO "Found $agreement_count agreements before deletion"

if [ "$agreement_count" == "0" ]; then
    log_message WARN "No agreements found, deletion test may not be meaningful"
fi

# Capture initial state
log_message INFO "Capturing initial agbot state"
capture_metrics "${TEST_NAME}_start"

# Run the agbot deletion loop test
log_message INFO "Running agbot deletion loop test"
if [ "$TEST_RETRY_ENABLED" == "1" ]; then
    retry_command $TEST_MAX_RETRIES $TEST_RETRY_DELAY "${PARENT_DIR}/agbot_del_loop.sh"
    result=$?
else
    "${PARENT_DIR}/agbot_del_loop.sh"
    result=$?
fi

# Capture final state
log_message INFO "Capturing final agbot state"
capture_metrics "${TEST_NAME}_end"

# Verify agbot is still accessible after deletion
log_message INFO "Verifying agbot is still accessible"
if ! curl -sS "${AGBOT_API}/status" > /dev/null 2>&1; then
    log_message ERROR "Agbot became inaccessible after deletion loop"
    result=1
fi

# Check agreement count after deletion
agreement_count_after=$(curl -sS "${AGBOT_API}/agreement" 2>/dev/null | jq '. | length' 2>/dev/null || echo "0")
log_message INFO "Found $agreement_count_after agreements after deletion"

# Report results
if [ $result -eq 0 ]; then
    log_message INFO "Agbot deletion loop test PASSED"
    log_message INFO "Agreements deleted: $((agreement_count - agreement_count_after))"
else
    log_message ERROR "Agbot deletion loop test FAILED"
    
    # Collect diagnostic information
    log_message ERROR "Agbot status:"
    curl -sS "${AGBOT_API}/status" | jq . || echo "Failed to get agbot status"
    
    log_message ERROR "Remaining agreements:"
    curl -sS "${AGBOT_API}/agreement" | jq . || echo "Failed to get agreements"
fi

exit $result
