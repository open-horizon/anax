# E2E Test Framework Documentation

## Overview

The E2E Test Framework provides improved test isolation, result collection, and failure handling for the anax test suite. It replaces the monolithic `gov-combined.sh` with a modular, configurable framework that supports:

- **Continue-on-failure**: Run all tests even if some fail
- **Result aggregation**: Collect and report results from all tests
- **Test isolation**: Optional isolated environments per test
- **Detailed reporting**: Comprehensive failure reports with diagnostics
- **Flexible configuration**: Environment-based configuration
- **Retry logic**: Automatic retry of flaky tests
- **Parallel execution**: Run independent tests in parallel (optional)

## Quick Start

### Basic Usage

```bash
# Run all tests with default configuration
cd test/gov
./gov-combined-new.sh

# Run with continue-on-failure enabled
TEST_CONTINUE_ON_FAILURE=1 ./gov-combined-new.sh

# Run with verbose output
TEST_VERBOSE=1 ./gov-combined-new.sh

# Run specific tests only
TEST_FILTER="api_tests,sync_service" ./gov-combined-new.sh

# Skip specific tests
TEST_SKIP="ha_test,upgrade_test" ./gov-combined-new.sh
```

### Configuration

The framework is configured via environment variables. See `test_config.sh` for all available options.

Key configuration variables:

```bash
# Continue running tests after failures (default: 1)
export TEST_CONTINUE_ON_FAILURE=1

# Show verbose output (default: 0)
export TEST_VERBOSE=0

# Use isolated test environments (default: 0)
export TEST_ISOLATED_ENV=0

# Clean up on failure (default: 0 - preserve for debugging)
export TEST_CLEANUP_ON_FAILURE=0

# Timeout multiplier for slow environments (default: 1)
export TEST_TIMEOUT_MULTIPLIER=2

# Generate JUnit XML report (default: 0)
export GENERATE_JUNIT_XML=1
```

## Architecture

### Components

1. **test_config.sh**: Centralized configuration management
2. **test_utils.sh**: Utility functions (wait conditions, assertions, etc.)
3. **test_framework.sh**: Core framework (test execution, result collection)
4. **gov-combined-new.sh**: Main test orchestrator (refactored)

### Test Execution Flow

```
1. init_test_suite()
   ├── Load configuration
   ├── Create results directory
   └── Setup cleanup handlers

2. run_test() or run_isolated_test()
   ├── Execute test script
   ├── Capture output and exit code
   ├── Record duration
   ├── Generate failure report (if failed)
   └── Continue or stop based on configuration

3. print_test_summary()
   ├── Display results table
   ├── Show pass/fail statistics
   ├── List failure reports
   └── Generate JUnit XML (optional)
```

## Writing Tests

### Using the Framework in New Tests

```bash
#!/bin/bash

# Source the framework
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/test_config.sh"
source "${SCRIPT_DIR}/test_utils.sh"

# Your test logic
log_message INFO "Starting my test"

# Wait for a condition
if ! wait_for_condition "Anax to be ready" "is_anax_running" 120 5; then
    log_message ERROR "Anax not ready"
    exit 1
fi

# Make assertions
assert_not_empty "$ANAX_API" "ANAX_API must be set"
assert "[ $(get_active_agreements | jq '. | length') -gt 0 ]" "No agreements found"

# Capture metrics
capture_metrics "my_test"

log_message INFO "Test completed successfully"
exit 0
```

### Wrapping Existing Tests

Create a wrapper script that adds framework features to existing tests:

```bash
#!/bin/bash

# Source framework
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/test_config.sh"
source "${SCRIPT_DIR}/test_utils.sh"

# Verify prerequisites
verify_prerequisites || exit 1

# Wait for dependencies
wait_for_anax 60 || exit 1

# Capture initial state
capture_metrics "test_start"

# Run existing test with retry
if [ "$TEST_RETRY_ENABLED" == "1" ]; then
    retry_command $TEST_MAX_RETRIES $TEST_RETRY_DELAY "./existing_test.sh"
    result=$?
else
    ./existing_test.sh
    result=$?
fi

# Capture final state
capture_metrics "test_end"

exit $result
```

## Utility Functions

### Wait Functions

```bash
# Wait for a generic condition
wait_for_condition "description" "command" [timeout] [interval]

# Wait for anax to be ready
wait_for_anax [timeout]

# Wait for agreement formation
wait_for_agreement [pattern] [timeout]

# Wait for service to start
wait_for_service "service_url" [timeout]

# Wait for specific agreement count
wait_for_agreement_count 3 [timeout]

# Wait for specific service count
wait_for_service_count 2 [timeout]
```

### Retry Functions

```bash
# Retry a command with exponential backoff
retry_command max_attempts delay "command"

# Example
retry_command 3 5 "curl -sS ${ANAX_API}/status"
```

### Status Functions

```bash
# Check if anax is running
is_anax_running

# Check if exchange is accessible
is_exchange_accessible

# Get anax status
get_anax_status

# Get active agreements
get_active_agreements

# Get active services
get_active_services
```

### Management Functions

```bash
# Cancel all agreements
cancel_all_agreements

# Unregister node
unregister_node

# Register with pattern
register_node_pattern "pattern_name" "node_id" "token"

# Register with policy
register_node_policy "node_id" "token" "policy_json"
```

### Cleanup Functions

```bash
# Clean up Docker containers
cleanup_docker_containers [pattern]

# Clean up Docker networks
cleanup_docker_networks [pattern]
```

### Assertion Functions

```bash
# Assert a condition is true
assert "[ -f /tmp/test.txt ]" "File should exist"

# Assert values are equal
assert_equals "expected" "actual" "Values should match"

# Assert value is not empty
assert_not_empty "$VAR" "Variable should be set"
```

### Logging Functions

```bash
# Log messages at different levels
log_message DEBUG "Debug information"
log_message INFO "Informational message"
log_message WARN "Warning message"
log_message ERROR "Error message"
```

## Configuration Reference

### Test Execution Control

| Variable | Default | Description |
|----------|---------|-------------|
| `TEST_CONTINUE_ON_FAILURE` | 1 | Continue running tests after failure |
| `TEST_PARALLEL_EXECUTION` | 0 | Run independent tests in parallel |
| `TEST_ISOLATED_ENV` | 0 | Use isolated test environments |
| `TEST_CLEANUP_ON_FAILURE` | 0 | Clean up environment on failure |
| `TEST_VERBOSE` | 0 | Show verbose output in real-time |
| `TEST_TIMEOUT_MULTIPLIER` | 1 | Multiplier for timeout values |
| `GENERATE_JUNIT_XML` | 0 | Generate JUnit XML report |

### Timeout Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `DEFAULT_WAIT_TIMEOUT` | 300 | Default timeout for wait conditions (seconds) |
| `DEFAULT_POLL_INTERVAL` | 5 | Default polling interval (seconds) |
| `AGREEMENT_TIMEOUT` | 48 | Agreement formation timeout (seconds) |
| `SERVICE_TIMEOUT` | 120 | Service startup timeout (seconds) |
| `API_TIMEOUT` | 30 | API response timeout (seconds) |

### Test Retry Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `TEST_RETRY_ENABLED` | 0 | Enable test retry on failure |
| `TEST_MAX_RETRIES` | 2 | Maximum number of retries |
| `TEST_RETRY_DELAY` | 10 | Delay between retries (seconds) |

### Test Selection

| Variable | Default | Description |
|----------|---------|-------------|
| `TEST_FILTER` | "" | Run only specific tests (comma-separated) |
| `TEST_SKIP` | "" | Skip specific tests (comma-separated) |
| `TEST_TAGS` | "" | Run tests with specific tags |

### Logging Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `TEST_LOG_LEVEL` | INFO | Log level (DEBUG, INFO, WARN, ERROR) |
| `KEEP_SUCCESS_LOGS` | 1 | Keep logs after successful tests |
| `MAX_LOG_SIZE` | 10485760 | Maximum log file size (bytes) |

### Cleanup Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `CLEANUP_CONTAINERS` | 1 | Clean up Docker containers after tests |
| `CLEANUP_NETWORKS` | 1 | Clean up Docker networks after tests |
| `CLEANUP_TEMP_FILES` | 1 | Clean up temporary files after tests |

## Test Results

### Results Directory Structure

```
/tmp/e2etest_results_<pid>/
├── test_name.log                    # Test output
├── failure_test_name.txt            # Detailed failure report
├── metrics_test_name_start.json    # Initial metrics
├── metrics_test_name_end.json      # Final metrics
├── env_test_name/                   # Test-specific environment (if isolated)
└── junit.xml                        # JUnit XML report (if enabled)
```

### Failure Reports

Failure reports include:

- Test name and exit code
- Timestamp and duration
- Environment variables
- Test output (last 100 lines)
- Anax status and agreements
- Active services
- Recent anax logs

### JUnit XML Reports

Enable JUnit XML generation for CI/CD integration:

```bash
GENERATE_JUNIT_XML=1 ./gov-combined-new.sh
```

The report is saved to `${TEST_RESULTS_DIR}/junit.xml`.

## Migration Guide

### Migrating from gov-combined.sh

1. **Update test invocation**:
   ```bash
   # Old
   ./gov-combined.sh
   
   # New
   ./gov-combined-new.sh
   ```

2. **Enable continue-on-failure**:
   ```bash
   TEST_CONTINUE_ON_FAILURE=1 ./gov-combined-new.sh
   ```

3. **Review test results**:
   ```bash
   # Results are in /tmp/e2etest_results_<pid>/
   ls -la /tmp/e2etest_results_*/
   ```

### Adapting Individual Tests

1. **Add framework source**:
   ```bash
   source "${SCRIPT_DIR}/test_config.sh"
   source "${SCRIPT_DIR}/test_utils.sh"
   ```

2. **Replace sleep with wait_for_condition**:
   ```bash
   # Old
   sleep 60
   
   # New
   wait_for_anax 60
   ```

3. **Add logging**:
   ```bash
   # Old
   echo "Starting test"
   
   # New
   log_message INFO "Starting test"
   ```

4. **Add assertions**:
   ```bash
   # Old
   if [ -z "$VAR" ]; then exit 1; fi
   
   # New
   assert_not_empty "$VAR" "Variable must be set"
   ```

## Best Practices

### Test Design

1. **Make tests independent**: Each test should be able to run standalone
2. **Use wait functions**: Don't use fixed sleep times
3. **Add proper logging**: Use log_message for all important events
4. **Capture metrics**: Use capture_metrics for debugging
5. **Clean up resources**: Always clean up in test teardown

### Error Handling

1. **Use assertions**: Prefer assertions over manual checks
2. **Provide context**: Include descriptive error messages
3. **Capture state**: Use capture_metrics on failure
4. **Exit with proper codes**: 0 for success, non-zero for failure

### Performance

1. **Use appropriate timeouts**: Adjust based on environment
2. **Enable parallel execution**: For independent tests
3. **Use retry logic**: For flaky tests
4. **Monitor resource usage**: Check metrics for bottlenecks

## Troubleshooting

### Tests Fail Immediately

Check if `TEST_CONTINUE_ON_FAILURE` is set:
```bash
TEST_CONTINUE_ON_FAILURE=1 ./gov-combined-new.sh
```

### Tests Timeout

Increase timeout multiplier:
```bash
TEST_TIMEOUT_MULTIPLIER=2 ./gov-combined-new.sh
```

### Need More Debug Information

Enable verbose mode and debug logging:
```bash
TEST_VERBOSE=1 TEST_LOG_LEVEL=DEBUG ./gov-combined-new.sh
```

### Test Environment Issues

Preserve environment on failure:
```bash
TEST_CLEANUP_ON_FAILURE=0 ./gov-combined-new.sh
```

### Finding Failure Details

Check the failure report:
```bash
cat /tmp/e2etest_results_*/failure_test_name.txt
```

## Examples

### Example 1: Run All Tests with Continue-on-Failure

```bash
#!/bin/bash
export TEST_CONTINUE_ON_FAILURE=1
export TEST_VERBOSE=1
export GENERATE_JUNIT_XML=1

./gov-combined-new.sh

# Check results
echo "Test results in: $TEST_RESULTS_DIR"
cat $TEST_RESULTS_DIR/junit.xml
```

### Example 2: Run Specific Tests Only

```bash
#!/bin/bash
export TEST_FILTER="api_tests,sync_service,agbot_verification"
export TEST_VERBOSE=1

./gov-combined-new.sh
```

### Example 3: Run Tests with Retry

```bash
#!/bin/bash
export TEST_RETRY_ENABLED=1
export TEST_MAX_RETRIES=3
export TEST_RETRY_DELAY=10

./gov-combined-new.sh
```

### Example 4: Run Tests in Isolated Environments

```bash
#!/bin/bash
export TEST_ISOLATED_ENV=1
export TEST_CLEANUP_ON_FAILURE=0

./gov-combined-new.sh
```

### Example 5: Custom Test Script

```bash
#!/bin/bash

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/test_config.sh"
source "${SCRIPT_DIR}/test_utils.sh"
source "${SCRIPT_DIR}/test_framework.sh"

# Initialize
init_test_suite

# Run tests
run_test "my_test_1" "./test1.sh"
run_test "my_test_2" "./test2.sh" "arg1 arg2"
run_isolated_test "my_test_3" "./test3.sh"

# Print summary
print_test_summary

# Exit with appropriate code
[ $FAILED_TESTS -eq 0 ] && exit 0 || exit 1
```

## Contributing

When adding new tests or modifying the framework:

1. Follow existing patterns and conventions
2. Add appropriate logging and error handling
3. Update documentation for new features
4. Test with both continue-on-failure enabled and disabled
5. Verify JUnit XML generation works correctly

## Support

For issues or questions:

1. Check the troubleshooting section
2. Review test logs in `$TEST_RESULTS_DIR`
3. Enable verbose mode for more details
4. Check failure reports for diagnostic information
