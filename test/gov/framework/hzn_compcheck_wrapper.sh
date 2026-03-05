#!/bin/bash

# Wrapper for hzn_compcheck.sh using the test framework
# Demonstrates policy compatibility check testing with framework features

# Source test framework
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PARENT_DIR="$(pwd)"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/test_config.sh"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/test_utils.sh"

# Test configuration
TEST_NAME="hzn_compcheck"

log_message INFO "Starting hzn policy compatibility check test"

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
capture_metrics "${TEST_NAME}_start"

# Verify hzn command works
log_message INFO "Verifying hzn command functionality"
if ! hzn version > /dev/null 2>&1; then
    log_message ERROR "hzn command is not functional"
    exit 1
fi

# Run the compatibility check test
log_message INFO "Running hzn policy compatibility check test"
if [ "$TEST_RETRY_ENABLED" == "1" ]; then
    retry_command "$TEST_MAX_RETRIES" "$TEST_RETRY_DELAY" "${PARENT_DIR}/hzn_compcheck.sh"
    result=$?
else
    "${PARENT_DIR}/hzn_compcheck.sh"
    result=$?
fi

# Capture final state
log_message INFO "Capturing final state"
capture_metrics "${TEST_NAME}_end"

# Report results
if [ $result -eq 0 ]; then
    log_message INFO "hzn policy compatibility check test PASSED"
    
    # Verify hzn command still works
    if hzn version > /dev/null 2>&1; then
        log_message INFO "hzn command is still functional"
    else
        log_message WARN "hzn command may have issues after test"
    fi
else
    log_message ERROR "hzn policy compatibility check test FAILED"
    
    # Collect diagnostic information
    log_message ERROR "hzn version:"
    hzn version || echo "Failed to get hzn version"
    
    log_message ERROR "Exchange connectivity:"
    curl -sS "${EXCH_APP_HOST}/v1/admin/version" || echo "Failed to connect to Exchange"
    
    if is_anax_running; then
        log_message ERROR "Anax status:"
        get_anax_status | jq . || echo "Failed to get status"
    fi
fi

exit $result
