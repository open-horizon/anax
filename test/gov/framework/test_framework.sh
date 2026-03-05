#!/bin/bash

# Test Framework for E2E Testing
# Provides test result collection, isolation, and reporting capabilities

# Source configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/test_config.sh"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/test_utils.sh"

# Initialize test results tracking
declare -A TEST_RESULTS
declare -A TEST_OUTPUTS
declare -A TEST_DURATIONS
declare -a TEST_ORDER
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
SKIPPED_TESTS=0

# Test suite start time
TEST_SUITE_START_TIME=$(date +%s)

# Initialize test suite
init_test_suite() {
    echo "========================================="
    echo "Initializing Test Suite"
    echo "========================================="
    echo "Configuration:"
    echo "  Continue on Failure: ${TEST_CONTINUE_ON_FAILURE}"
    echo "  Parallel Execution: ${TEST_PARALLEL_EXECUTION}"
    echo "  Isolated Environment: ${TEST_ISOLATED_ENV}"
    echo "  Cleanup on Failure: ${TEST_CLEANUP_ON_FAILURE}"
    echo "  Verbose Mode: ${TEST_VERBOSE}"
    echo "  Timeout Multiplier: ${TEST_TIMEOUT_MULTIPLIER}"
    echo "========================================="
    echo ""
    
    # Create test results directory
    TEST_RESULTS_DIR="${TEST_RESULTS_DIR:-/tmp/e2etest_results_$$}"
    mkdir -p "$TEST_RESULTS_DIR"
    export TEST_RESULTS_DIR
    
    # Setup trap for cleanup
    trap cleanup_test_suite EXIT INT TERM
}

# Run a test and capture result
run_test() {
    local test_name="$1"
    local test_script="$2"
    local test_args="${3:-}"
    
    echo ""
    echo "========================================="
    echo "Running: $test_name"
    echo "Script: $test_script"
    if [ -n "$test_args" ]; then
        echo "Args: $test_args"
    fi
    echo "========================================="
    
    TEST_ORDER+=("$test_name")
    ((TOTAL_TESTS++))
    
    # Create test-specific log file
    local test_log="${TEST_RESULTS_DIR}/${test_name}.log"
    
    # Record start time
    local start_time
    start_time=$(date +%s)
    
    # Run test and capture output
    local exit_code
    if [ "$TEST_VERBOSE" == "1" ]; then
        # Show output in real-time
        if [ -n "$test_args" ]; then
            eval "$test_script $test_args" 2>&1 | tee "$test_log"
        else
            eval "$test_script" 2>&1 | tee "$test_log"
        fi
        exit_code=${PIPESTATUS[0]}
    else
        # Capture output to log file
        if [ -n "$test_args" ]; then
            eval "$test_script $test_args" > "$test_log" 2>&1
        else
            eval "$test_script" > "$test_log" 2>&1
        fi
        exit_code=$?
    fi
    
    # Record end time and duration
    local end_time
    end_time=$(date +%s)
    local duration=$((end_time - start_time))
    TEST_DURATIONS["$test_name"]=$duration
    
    # Store test output
    TEST_OUTPUTS["$test_name"]="$test_log"
    
    # Process result
    if [ "$exit_code" -eq 0 ]; then
        TEST_RESULTS["$test_name"]="PASS"
        ((PASSED_TESTS++))
        echo "✓ PASSED: $test_name (${duration}s)"
    else
        TEST_RESULTS["$test_name"]="FAIL"
        ((FAILED_TESTS++))
        echo "✗ FAILED: $test_name (exit code: $exit_code, ${duration}s)"
        
        # Show last 20 lines of output on failure
        if [ "$TEST_VERBOSE" != "1" ]; then
            echo "Last 20 lines of output:"
            tail -20 "$test_log" | sed 's/^/  /'
        fi
        
        # Generate detailed failure report
        report_test_failure "$test_name" "$exit_code" "$test_log"
        
        # Exit immediately if not continuing on failure
        if [ "$TEST_CONTINUE_ON_FAILURE" != "1" ]; then
            echo ""
            echo "Stopping test suite due to failure (TEST_CONTINUE_ON_FAILURE=0)"
            print_test_summary
            exit 1
        fi
    fi
    
    echo "========================================="
    
    return "$exit_code"
}

# Run a test with setup and teardown
run_isolated_test() {
    local test_name="$1"
    local test_script="$2"
    local test_args="${3:-}"
    
    if [ "$TEST_ISOLATED_ENV" != "1" ]; then
        # Run without isolation
        run_test "$test_name" "$test_script" "$test_args"
        return $?
    fi
    
    echo "Setting up isolated environment for: $test_name"
    test_setup "$test_name"
    local setup_result=$?
    
    if [ $setup_result -ne 0 ]; then
        echo "✗ Setup failed for: $test_name"
        TEST_RESULTS["$test_name"]="SKIP"
        ((SKIPPED_TESTS++))
        return 1
    fi
    
    # Run the actual test
    run_test "$test_name" "$test_script" "$test_args"
    local test_result=$?
    
    # Always run teardown
    echo "Tearing down environment for: $test_name"
    test_teardown "$test_name"
    
    return $test_result
}

# Setup test environment
test_setup() {
    local test_name="$1"
    
    # Create test-specific environment
    local test_env_dir="${TEST_RESULTS_DIR}/env_${test_name}"
    mkdir -p "$test_env_dir"
    
    export TEST_ENV_DIR="$test_env_dir"
    export TEST_NAME="$test_name"
    
    # Additional setup can be added here
    return 0
}

# Teardown test environment
test_teardown() {
    local test_name="$1"
    
    if [ "$TEST_CLEANUP_ON_FAILURE" == "1" ] || [ "${TEST_RESULTS[$test_name]}" == "PASS" ]; then
        # Clean up test environment
        if [ -d "$TEST_ENV_DIR" ]; then
            rm -rf "$TEST_ENV_DIR"
        fi
    else
        echo "Preserving test environment for debugging: $TEST_ENV_DIR"
    fi
    
    unset TEST_ENV_DIR
    unset TEST_NAME
    
    return 0
}

# Generate detailed failure report
report_test_failure() {
    local test_name="$1"
    local exit_code="$2"
    local test_log="$3"
    
    local report_file="${TEST_RESULTS_DIR}/failure_${test_name}.txt"
    
    cat > "$report_file" <<EOF
Test Failure Report
===================
Test Name: $test_name
Exit Code: $exit_code
Timestamp: $(date -Iseconds)
Duration: ${TEST_DURATIONS[$test_name]}s

Environment Variables:
- PATTERN: ${PATTERN:-<not set>}
- DEVICE_ORG: ${DEVICE_ORG:-<not set>}
- ANAX_API: ${ANAX_API:-<not set>}
- EXCH_APP_HOST: ${EXCH_APP_HOST:-<not set>}
- TEST_CONTINUE_ON_FAILURE: ${TEST_CONTINUE_ON_FAILURE}

Test Output (last 100 lines):
$(tail -100 "$test_log" 2>&1)

EOF

    # Try to get anax status if available
    if [ -n "${ANAX_API:-}" ]; then
        cat >> "$report_file" <<EOF

Anax Status:
$(curl -sS "$ANAX_API/status" 2>&1 || echo "Failed to get status")

Active Agreements:
$(curl -sS "$ANAX_API/agreement" 2>&1 || echo "Failed to get agreements")

Active Services:
$(curl -sS "$ANAX_API/service" 2>&1 || echo "Failed to get services")

EOF
    fi
    
    # Include recent anax logs if available
    if [ -f "/tmp/anax.log" ]; then
        cat >> "$report_file" <<EOF

Recent Anax Logs (last 100 lines):
$(tail -100 /tmp/anax.log 2>&1)
EOF
    fi
    
    echo "Detailed failure report saved to: $report_file"
}

# Print test summary
print_test_summary() {
    local suite_end_time
    suite_end_time=$(date +%s)
    local suite_duration=$((suite_end_time - TEST_SUITE_START_TIME))
    
    echo ""
    echo "========================================="
    echo "TEST SUITE SUMMARY"
    echo "========================================="
    echo "Total Duration: ${suite_duration}s"
    echo "Total Tests: $TOTAL_TESTS"
    echo "Passed: $PASSED_TESTS"
    echo "Failed: $FAILED_TESTS"
    echo "Skipped: $SKIPPED_TESTS"
    echo ""
    
    if [ $TOTAL_TESTS -gt 0 ]; then
        local pass_rate=$((PASSED_TESTS * 100 / TOTAL_TESTS))
        echo "Pass Rate: ${pass_rate}%"
        echo ""
    fi
    
    echo "Detailed Results:"
    echo "----------------------------------------"
    printf "%-40s %-8s %-10s\n" "Test Name" "Status" "Duration"
    echo "----------------------------------------"
    
    for test in "${TEST_ORDER[@]}"; do
        local status="${TEST_RESULTS[$test]}"
        local duration="${TEST_DURATIONS[$test]:-0}s"
        
        # Color code status
        local status_display="$status"
        if [ "$status" == "PASS" ]; then
            status_display="✓ PASS"
        elif [ "$status" == "FAIL" ]; then
            status_display="✗ FAIL"
        elif [ "$status" == "SKIP" ]; then
            status_display="⊘ SKIP"
        fi
        
        printf "%-40s %-8s %-10s\n" "$test" "$status_display" "$duration"
        
        # Show log file location for failed tests
        if [ "$status" == "FAIL" ]; then
            echo "  Log: ${TEST_OUTPUTS[$test]}"
            echo "  Report: ${TEST_RESULTS_DIR}/failure_${test}.txt"
        fi
    done
    
    echo "----------------------------------------"
    echo ""
    echo "Test results directory: $TEST_RESULTS_DIR"
    echo "========================================="
    
    # Generate JUnit XML report if requested
    if [ "${GENERATE_JUNIT_XML:-0}" == "1" ]; then
        generate_junit_xml
    fi
}

# Generate JUnit XML report
generate_junit_xml() {
    local xml_file="${TEST_RESULTS_DIR}/junit.xml"
    local suite_duration=$(($(date +%s) - TEST_SUITE_START_TIME))
    
    cat > "$xml_file" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
  <testsuite name="E2E Test Suite" tests="$TOTAL_TESTS" failures="$FAILED_TESTS" skipped="$SKIPPED_TESTS" time="$suite_duration">
EOF
    
    for test in "${TEST_ORDER[@]}"; do
        local status="${TEST_RESULTS[$test]}"
        local duration="${TEST_DURATIONS[$test]:-0}"
        local test_log="${TEST_OUTPUTS[$test]}"
        
        cat >> "$xml_file" <<EOF
    <testcase name="$test" time="$duration">
EOF
        
        if [ "$status" == "FAIL" ]; then
            cat >> "$xml_file" <<EOF
      <failure message="Test failed">
$(cat "$test_log" | sed 's/&/\&amp;/g; s/</\&lt;/g; s/>/\&gt;/g; s/"/\&quot;/g')
      </failure>
EOF
        elif [ "$status" == "SKIP" ]; then
            cat >> "$xml_file" <<EOF
      <skipped message="Test skipped"/>
EOF
        fi
        
        cat >> "$xml_file" <<EOF
    </testcase>
EOF
    done
    
    cat >> "$xml_file" <<EOF
  </testsuite>
</testsuites>
EOF
    
    echo "JUnit XML report generated: $xml_file"
}

# Cleanup function
cleanup_test_suite() {
    # This function is called on exit
    if [ "${TEST_SUITE_INITIALIZED:-0}" == "1" ]; then
        echo ""
        echo "Cleaning up test suite..."
    fi
}

# Mark suite as initialized
TEST_SUITE_INITIALIZED=1

# Export functions for use in test scripts
export -f run_test
export -f run_isolated_test
export -f test_setup
export -f test_teardown
export -f report_test_failure
export -f print_test_summary
