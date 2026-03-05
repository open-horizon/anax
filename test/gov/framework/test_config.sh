#!/bin/bash

# Test Configuration
# Centralized configuration for E2E test suite behavior

# Test Execution Control
# ----------------------

# Continue running tests even if one fails (1=continue, 0=stop on first failure)
export TEST_CONTINUE_ON_FAILURE=${TEST_CONTINUE_ON_FAILURE:-1}

# Run independent tests in parallel (1=parallel, 0=sequential)
export TEST_PARALLEL_EXECUTION=${TEST_PARALLEL_EXECUTION:-0}

# Use isolated test environments (1=isolated, 0=shared)
export TEST_ISOLATED_ENV=${TEST_ISOLATED_ENV:-0}

# Clean up test environment on failure (1=cleanup, 0=preserve for debugging)
export TEST_CLEANUP_ON_FAILURE=${TEST_CLEANUP_ON_FAILURE:-0}

# Show verbose test output in real-time (1=verbose, 0=quiet)
export TEST_VERBOSE=${TEST_VERBOSE:-0}

# Multiplier for timeout values (useful for slower environments)
export TEST_TIMEOUT_MULTIPLIER=${TEST_TIMEOUT_MULTIPLIER:-1}

# Generate JUnit XML report (1=generate, 0=skip)
export GENERATE_JUNIT_XML=${GENERATE_JUNIT_XML:-0}

# Test Results Directory
# ----------------------
export TEST_RESULTS_DIR=${TEST_RESULTS_DIR:-/tmp/e2etest_results_$$}

# Timeout Configuration
# ---------------------

# Default timeout for waiting on conditions (seconds)
export DEFAULT_WAIT_TIMEOUT=${DEFAULT_WAIT_TIMEOUT:-300}

# Default polling interval for condition checks (seconds)
export DEFAULT_POLL_INTERVAL=${DEFAULT_POLL_INTERVAL:-5}

# Agreement formation timeout (seconds)
export AGREEMENT_TIMEOUT=${AGREEMENT_TIMEOUT:-$((48 * TEST_TIMEOUT_MULTIPLIER))}

# Service startup timeout (seconds)
export SERVICE_TIMEOUT=${SERVICE_TIMEOUT:-$((120 * TEST_TIMEOUT_MULTIPLIER))}

# API response timeout (seconds)
export API_TIMEOUT=${API_TIMEOUT:-$((30 * TEST_TIMEOUT_MULTIPLIER))}

# Test Retry Configuration
# ------------------------

# Enable test retry on failure (1=retry, 0=no retry)
export TEST_RETRY_ENABLED=${TEST_RETRY_ENABLED:-0}

# Maximum number of retries per test
export TEST_MAX_RETRIES=${TEST_MAX_RETRIES:-2}

# Delay between retries (seconds)
export TEST_RETRY_DELAY=${TEST_RETRY_DELAY:-10}

# Logging Configuration
# ---------------------

# Log level for test framework (DEBUG, INFO, WARN, ERROR)
export TEST_LOG_LEVEL=${TEST_LOG_LEVEL:-INFO}

# Keep test logs after successful tests (1=keep, 0=delete)
export KEEP_SUCCESS_LOGS=${KEEP_SUCCESS_LOGS:-1}

# Maximum log file size before rotation (bytes)
export MAX_LOG_SIZE=${MAX_LOG_SIZE:-10485760}  # 10MB

# Test Selection
# --------------

# Run only specific tests (comma-separated list, empty=all)
export TEST_FILTER=${TEST_FILTER:-}

# Skip specific tests (comma-separated list)
export TEST_SKIP=${TEST_SKIP:-}

# Test tags to run (comma-separated, empty=all)
export TEST_TAGS=${TEST_TAGS:-}

# Resource Limits
# ---------------

# Maximum parallel test jobs
export MAX_PARALLEL_JOBS=${MAX_PARALLEL_JOBS:-4}

# Memory limit per test (MB, 0=unlimited)
export TEST_MEMORY_LIMIT=${TEST_MEMORY_LIMIT:-0}

# CPU limit per test (cores, 0=unlimited)
export TEST_CPU_LIMIT=${TEST_CPU_LIMIT:-0}

# Cleanup Configuration
# --------------------

# Clean up Docker containers after tests (1=cleanup, 0=preserve)
export CLEANUP_CONTAINERS=${CLEANUP_CONTAINERS:-1}

# Clean up Docker networks after tests (1=cleanup, 0=preserve)
export CLEANUP_NETWORKS=${CLEANUP_NETWORKS:-1}

# Clean up temporary files after tests (1=cleanup, 0=preserve)
export CLEANUP_TEMP_FILES=${CLEANUP_TEMP_FILES:-1}

# Notification Configuration
# --------------------------

# Send notifications on test completion (1=send, 0=skip)
export SEND_NOTIFICATIONS=${SEND_NOTIFICATIONS:-0}

# Notification webhook URL (optional)
export NOTIFICATION_WEBHOOK_URL=${NOTIFICATION_WEBHOOK_URL:-}

# Notification email address (optional)
export NOTIFICATION_EMAIL=${NOTIFICATION_EMAIL:-}

# Debug Configuration
# -------------------

# Enable debug mode (1=debug, 0=normal)
export TEST_DEBUG=${TEST_DEBUG:-0}

# Pause on test failure for debugging (1=pause, 0=continue)
export PAUSE_ON_FAILURE=${PAUSE_ON_FAILURE:-0}

# Save core dumps on crashes (1=save, 0=skip)
export SAVE_CORE_DUMPS=${SAVE_CORE_DUMPS:-0}

# Compatibility with Existing Tests
# ----------------------------------

# These variables maintain compatibility with existing test scripts
# They can be overridden by environment variables

# NOLOOP: Run tests only once (1=once, 0=loop)
export NOLOOP=${NOLOOP:-1}

# NOCANCEL: Skip cancellation tests (1=skip, 0=run)
export NOCANCEL=${NOCANCEL:-0}

# NOAGBOT: Skip agbot tests (1=skip, 0=run)
export NOAGBOT=${NOAGBOT:-0}

# NOANAX: Skip anax tests (1=skip, 0=run)
export NOANAX=${NOANAX:-0}

# NOKUBE: Skip Kubernetes tests (1=skip, 0=run)
export NOKUBE=${NOKUBE:-0}

# NOVAULT: Skip vault tests (1=skip, 0=run)
export NOVAULT=${NOVAULT:-0}

# NOCOMPCHECK: Skip compatibility check tests (1=skip, 0=run)
export NOCOMPCHECK=${NOCOMPCHECK:-0}

# NOSURFERR: Skip surface error tests (1=skip, 0=run)
export NOSURFERR=${NOSURFERR:-0}

# NOUPGRADE: Skip upgrade tests (1=skip, 0=run)
export NOUPGRADE=${NOUPGRADE:-0}

# NOHZNREG: Skip hzn registration tests (1=skip, 0=run)
export NOHZNREG=${NOHZNREG:-0}

# NOHZNLOG: Skip hzn log tests (1=skip, 0=run)
export NOHZNLOG=${NOHZNLOG:-0}

# NOPATTERNCHANGE: Skip pattern change tests (1=skip, 0=run)
export NOPATTERNCHANGE=${NOPATTERNCHANGE:-0}

# NORETRY: Skip retry tests (1=skip, 0=run)
export NORETRY=${NORETRY:-0}

# NOSVC_CONFIGSTATE: Skip service config state tests (1=skip, 0=run)
export NOSVC_CONFIGSTATE=${NOSVC_CONFIGSTATE:-0}

# NOAGENTAUTO: Skip agent auto upgrade tests (1=skip, 0=run)
export NOAGENTAUTO=${NOAGENTAUTO:-0}

# Helper Functions
# ----------------

# Check if a test should be skipped based on filters
should_skip_test() {
    local test_name="$1"
    
    # Check TEST_SKIP list
    if [ -n "$TEST_SKIP" ]; then
        if echo "$TEST_SKIP" | grep -q "$test_name"; then
            return 0  # Skip
        fi
    fi
    
    # Check TEST_FILTER list (if set, only run tests in the list)
    if [ -n "$TEST_FILTER" ]; then
        if ! echo "$TEST_FILTER" | grep -q "$test_name"; then
            return 0  # Skip
        fi
    fi
    
    return 1  # Don't skip
}

# Get timeout value with multiplier applied
get_timeout() {
    local base_timeout="$1"
    echo $((base_timeout * TEST_TIMEOUT_MULTIPLIER))
}

# Log message based on log level
log_message() {
    local level="$1"
    shift
    local message="$*"
    
    local level_priority=0
    case "$level" in
        DEBUG) level_priority=0 ;;
        INFO)  level_priority=1 ;;
        WARN)  level_priority=2 ;;
        ERROR) level_priority=3 ;;
    esac
    
    local config_priority=1
    case "$TEST_LOG_LEVEL" in
        DEBUG) config_priority=0 ;;
        INFO)  config_priority=1 ;;
        WARN)  config_priority=2 ;;
        ERROR) config_priority=3 ;;
    esac
    
    if [ $level_priority -ge $config_priority ]; then
        echo "[$(date -Iseconds)] [$level] $message"
    fi
}

# Export helper functions
export -f should_skip_test
export -f get_timeout
export -f log_message

# Print configuration summary if requested
if [ "${PRINT_TEST_CONFIG:-0}" == "1" ]; then
    echo "Test Configuration:"
    echo "  TEST_CONTINUE_ON_FAILURE: $TEST_CONTINUE_ON_FAILURE"
    echo "  TEST_PARALLEL_EXECUTION: $TEST_PARALLEL_EXECUTION"
    echo "  TEST_ISOLATED_ENV: $TEST_ISOLATED_ENV"
    echo "  TEST_CLEANUP_ON_FAILURE: $TEST_CLEANUP_ON_FAILURE"
    echo "  TEST_VERBOSE: $TEST_VERBOSE"
    echo "  TEST_TIMEOUT_MULTIPLIER: $TEST_TIMEOUT_MULTIPLIER"
    echo "  TEST_RESULTS_DIR: $TEST_RESULTS_DIR"
    echo "  TEST_LOG_LEVEL: $TEST_LOG_LEVEL"
fi
