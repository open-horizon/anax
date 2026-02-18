#!/bin/bash

# Wrapper for hzn_secretsmanager.sh using the test framework
# Tests secrets manager functionality with hzn CLI

# Source test framework
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PARENT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
source "${SCRIPT_DIR}/test_config.sh"
source "${SCRIPT_DIR}/test_utils.sh"

# Test configuration
TEST_NAME="hzn_secretsmanager"
TIMEOUT=$(get_timeout $SERVICE_TIMEOUT)

log_message INFO "Starting hzn secrets manager test"

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

# Verify vault is accessible if required
if [ "$NOVAULT" != "1" ]; then
    log_message INFO "Verifying vault accessibility"
    if ! curl -sS "${VAULT_ADDR:-http://localhost:8200}/v1/sys/health" > /dev/null 2>&1; then
        log_message WARN "Vault may not be accessible"
    fi
fi

# Capture initial state
log_message INFO "Capturing initial state"
capture_metrics "${TEST_NAME}_start"

# Run the secrets manager test
log_message INFO "Running hzn secrets manager test"
if [ "$TEST_RETRY_ENABLED" == "1" ]; then
    retry_command $TEST_MAX_RETRIES $TEST_RETRY_DELAY "${PARENT_DIR}/hzn_secretsmanager.sh"
    result=$?
else
    "${PARENT_DIR}/hzn_secretsmanager.sh"
    result=$?
fi

# Capture final state
log_message INFO "Capturing final state"
capture_metrics "${TEST_NAME}_end"

# Report results
if [ $result -eq 0 ]; then
    log_message INFO "HZN secrets manager test PASSED"
else
    log_message ERROR "HZN secrets manager test FAILED"
    
    # Collect diagnostic information
    log_message ERROR "Node status:"
    hzn node list 2>&1 || echo "Failed to get node status"
    
    log_message ERROR "Recent anax logs (last 50 lines):"
    if [ -f "/tmp/anax.log" ]; then
        tail -50 /tmp/anax.log
    else
        echo "Log file not found"
    fi
fi

exit $result
