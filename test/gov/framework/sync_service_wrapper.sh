#!/bin/bash

# Wrapper for sync_service_test.sh using the test framework
# Demonstrates CSS/ESS sync service testing with framework features

# Source test framework
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FRAMEWORK_DIR="$SCRIPT_DIR"
GOV_DIR="$(dirname "$FRAMEWORK_DIR")"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/test_config.sh"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/test_framework.sh"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/test_utils.sh"

# Test configuration
TEST_NAME="sync_service"

log_message INFO "Starting sync service (CSS/ESS) test"

# Verify prerequisites
if ! verify_prerequisites; then
    log_message ERROR "Prerequisites check failed"
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

# Check if CSS/ESS is accessible
log_message INFO "Checking CSS/ESS accessibility"
if [ -n "${CSS_URL:-}" ]; then
    if curl -sS "${CSS_URL}/api/v1/health" > /dev/null 2>&1; then
        log_message INFO "CSS is accessible at ${CSS_URL}"
    else
        log_message WARN "CSS may not be accessible at ${CSS_URL}"
    fi
fi

# Set required environment variables for hzn mms commands
# Note: Use explicit username instead of $USER to avoid conflict with shell's USER variable
EXCH_USER="${USER:-anax1}"
if [ "$TEST_DIFF_ORG" = "1" ]; then
    EXCH_USER="useranax1"
fi

export HZN_ORG_ID="${DEVICE_ORG}"
export HZN_EXCHANGE_USER_AUTH="${EXCH_USER}:${PASS}"
export HZN_EXCHANGE_URL="${EXCH_APP_HOST}"
export HZN_FSS_CSSURL="${CSS_URL}"

# Change to test directory before running test
cd "$GOV_DIR" || {
    log_message ERROR "Failed to change to test directory: $GOV_DIR"
    exit 1
}

# Run the sync service test
log_message INFO "Running sync service test"
if [ "$TEST_RETRY_ENABLED" = "1" ]; then
    retry_command "$TEST_MAX_RETRIES" "$TEST_RETRY_DELAY" "./sync_service_test.sh"
    result=$?
else
    ./sync_service_test.sh
    result=$?
fi

# Capture final state
log_message INFO "Capturing final state"
capture_metrics "${TEST_NAME}_end"

# Report results
if [ $result -eq 0 ]; then
    log_message INFO "Sync service test PASSED"
else
    log_message ERROR "Sync service test FAILED"
    
    # Collect diagnostic information
    if [ -n "${CSS_URL:-}" ]; then
        log_message ERROR "CSS health check:"
        curl -sS "${CSS_URL}/api/v1/health" || echo "Failed to get CSS health"
    fi
    
    log_message ERROR "Exchange status:"
    if [ -n "${HZN_EXCHANGE_USER_AUTH:-}" ]; then
        curl -sS -u "${HZN_EXCHANGE_USER_AUTH}" "${EXCH_APP_HOST}/admin/version" 2>&1 || echo "Failed to get Exchange version"
    else
        curl -sS "${EXCH_APP_HOST}/admin/version" 2>&1 || echo "Failed to get Exchange version (no auth)"
    fi
fi

exit $result
