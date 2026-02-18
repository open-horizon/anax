#!/bin/bash

# Wrapper for agbot_upgrade_test.sh using the test framework
# Tests agreement bot upgrade functionality

# Source test framework
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PARENT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
source "${SCRIPT_DIR}/test_config.sh"
source "${SCRIPT_DIR}/test_utils.sh"

# Test configuration
TEST_NAME="agbot_upgrade_test"
TIMEOUT=$(get_timeout $AGBOT_TIMEOUT)

log_message INFO "Starting agbot upgrade test"

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

# Capture initial agbot state
log_message INFO "Capturing initial agbot state"
capture_metrics "${TEST_NAME}_start"
curl -sS "${AGBOT_API}/agreement" > /tmp/${TEST_NAME}_agreements_before.json 2>&1

# Run the agbot upgrade test
log_message INFO "Running agbot upgrade test"
if [ "$TEST_RETRY_ENABLED" == "1" ]; then
    retry_command $TEST_MAX_RETRIES $TEST_RETRY_DELAY "${PARENT_DIR}/agbot_upgrade_test.sh"
    result=$?
else
    "${PARENT_DIR}/agbot_upgrade_test.sh"
    result=$?
fi

# Capture final agbot state
log_message INFO "Capturing final agbot state"
capture_metrics "${TEST_NAME}_end"
curl -sS "${AGBOT_API}/agreement" > /tmp/${TEST_NAME}_agreements_after.json 2>&1

# Verify agbot is still accessible after upgrade
log_message INFO "Verifying agbot is still accessible"
if ! curl -sS "${AGBOT_API}/status" > /dev/null 2>&1; then
    log_message ERROR "Agbot became inaccessible after upgrade"
    result=1
fi

# Report results
if [ $result -eq 0 ]; then
    log_message INFO "Agbot upgrade test PASSED"
    
    # Compare agreement counts
    before_count=$(jq '. | length' /tmp/${TEST_NAME}_agreements_before.json 2>/dev/null || echo "0")
    after_count=$(jq '. | length' /tmp/${TEST_NAME}_agreements_after.json 2>/dev/null || echo "0")
    log_message INFO "Agreements before: $before_count, after: $after_count"
else
    log_message ERROR "Agbot upgrade test FAILED"
    
    # Collect diagnostic information
    log_message ERROR "Agbot status:"
    curl -sS "${AGBOT_API}/status" | jq . || echo "Failed to get agbot status"
    
    log_message ERROR "Agbot agreements:"
    curl -sS "${AGBOT_API}/agreement" | jq . || echo "Failed to get agreements"
fi

# Cleanup temporary files
rm -f /tmp/${TEST_NAME}_agreements_before.json /tmp/${TEST_NAME}_agreements_after.json

exit $result
