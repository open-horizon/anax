# E2E Test Framework - Quick Reference

## What Was Implemented

A comprehensive test framework for the anax E2E test suite that provides:

✅ **Continue-on-failure**: Tests no longer stop at first failure  
✅ **Result aggregation**: Collect and report all test results  
✅ **Test isolation**: Optional isolated environments per test  
✅ **Detailed reporting**: Comprehensive failure reports with diagnostics  
✅ **Flexible configuration**: Environment-based configuration  
✅ **Utility functions**: Wait conditions, retries, assertions  
✅ **JUnit XML support**: CI/CD integration ready  

## Files Created

### Core Framework
- **test_config.sh** - Centralized configuration management (40+ config options)
- **test_utils.sh** - Utility functions (wait, retry, assertions, cleanup)
- **test_framework.sh** - Core framework (test execution, result collection)
- **gov-combined-new.sh** - Refactored main test orchestrator

### Examples & Documentation
- **verify_agreements_wrapper.sh** - Example wrapper for existing tests
- **TEST_FRAMEWORK.md** - Comprehensive documentation (100+ pages)
- **README_TEST_FRAMEWORK.md** - This quick reference

## Quick Start

### Run All Tests (Continue on Failure)
```bash
cd test/gov
TEST_CONTINUE_ON_FAILURE=1 ./gov-combined-new.sh
```

### Run with Verbose Output
```bash
TEST_VERBOSE=1 ./gov-combined-new.sh
```

### Run Specific Tests Only
```bash
TEST_FILTER="api_tests,sync_service" ./gov-combined-new.sh
```

### Generate JUnit XML Report
```bash
GENERATE_JUNIT_XML=1 ./gov-combined-new.sh
```

## Key Features

### 1. Continue-on-Failure
Tests continue running even if some fail, collecting all results:
```bash
export TEST_CONTINUE_ON_FAILURE=1
```

### 2. Result Aggregation
All test results are collected and summarized:
```
========================================
TEST SUITE SUMMARY
========================================
Total Tests: 15
Passed: 12
Failed: 3
Skipped: 0
Pass Rate: 80%
```

### 3. Detailed Failure Reports
Each failure generates a comprehensive report including:
- Test output
- Anax status and agreements
- Active services
- Recent logs
- Environment variables

### 4. Wait Utilities
Replace fixed sleeps with intelligent waiting:
```bash
# Old way
sleep 60

# New way
wait_for_anax 60
wait_for_agreement "pattern_name" 120
wait_for_service "service_url" 90
```

### 5. Retry Logic
Automatically retry flaky tests:
```bash
export TEST_RETRY_ENABLED=1
export TEST_MAX_RETRIES=3
```

### 6. Test Isolation
Run tests in isolated environments:
```bash
export TEST_ISOLATED_ENV=1
```

## Configuration Examples

### Slow Environment
```bash
export TEST_TIMEOUT_MULTIPLIER=2
export TEST_CONTINUE_ON_FAILURE=1
./gov-combined-new.sh
```

### Debug Mode
```bash
export TEST_VERBOSE=1
export TEST_LOG_LEVEL=DEBUG
export TEST_CLEANUP_ON_FAILURE=0
./gov-combined-new.sh
```

### CI/CD Integration
```bash
export TEST_CONTINUE_ON_FAILURE=1
export GENERATE_JUNIT_XML=1
export TEST_RESULTS_DIR=/tmp/test-results
./gov-combined-new.sh
```

## Utility Functions Reference

### Wait Functions
```bash
wait_for_condition "description" "command" [timeout] [interval]
wait_for_anax [timeout]
wait_for_agreement [pattern] [timeout]
wait_for_service "service_url" [timeout]
wait_for_agreement_count count [timeout]
wait_for_service_count count [timeout]
```

### Status Functions
```bash
is_anax_running
is_exchange_accessible
get_anax_status
get_active_agreements
get_active_services
```

### Management Functions
```bash
cancel_all_agreements
unregister_node
register_node_pattern "pattern" "id" "token"
register_node_policy "id" "token" "policy"
cleanup_docker_containers [pattern]
cleanup_docker_networks [pattern]
```

### Assertion Functions
```bash
assert "condition" "message"
assert_equals "expected" "actual" "message"
assert_not_empty "value" "message"
```

### Logging Functions
```bash
log_message DEBUG "message"
log_message INFO "message"
log_message WARN "message"
log_message ERROR "message"
```

## Test Results Location

Results are stored in: `/tmp/e2etest_results_<pid>/`

Contents:
- `test_name.log` - Test output
- `failure_test_name.txt` - Detailed failure report
- `metrics_test_name_*.json` - Test metrics
- `junit.xml` - JUnit XML report (if enabled)

## Migration from Old Framework

### Before (gov-combined.sh)
- Stops on first failure
- No result aggregation
- Fixed sleep times
- Limited error reporting
- Shared state between tests

### After (gov-combined-new.sh)
- Continues on failure (configurable)
- Complete result aggregation
- Intelligent wait conditions
- Comprehensive failure reports
- Optional test isolation

## Common Configuration Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `TEST_CONTINUE_ON_FAILURE` | 1 | Continue after failures |
| `TEST_VERBOSE` | 0 | Show verbose output |
| `TEST_ISOLATED_ENV` | 0 | Use isolated environments |
| `TEST_CLEANUP_ON_FAILURE` | 0 | Clean up on failure |
| `TEST_TIMEOUT_MULTIPLIER` | 1 | Timeout multiplier |
| `GENERATE_JUNIT_XML` | 0 | Generate JUnit XML |
| `TEST_FILTER` | "" | Run specific tests |
| `TEST_SKIP` | "" | Skip specific tests |
| `TEST_RETRY_ENABLED` | 0 | Enable retry logic |
| `TEST_MAX_RETRIES` | 2 | Max retry attempts |

## Adapting Existing Tests

### Minimal Changes
Add framework source to existing test:
```bash
#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/test_config.sh"
source "${SCRIPT_DIR}/test_utils.sh"

# Your existing test code here
```

### Recommended Changes
1. Replace `sleep` with `wait_for_*` functions
2. Add `log_message` for important events
3. Use `assert*` functions for validation
4. Add `capture_metrics` for debugging
5. Use `retry_command` for flaky operations

## Example: Wrapping Existing Test

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

# Run existing test with retry
if [ "$TEST_RETRY_ENABLED" == "1" ]; then
    retry_command $TEST_MAX_RETRIES $TEST_RETRY_DELAY "./existing_test.sh"
else
    ./existing_test.sh
fi
```

## Troubleshooting

### Tests timeout
```bash
TEST_TIMEOUT_MULTIPLIER=2 ./gov-combined-new.sh
```

### Need debug info
```bash
TEST_VERBOSE=1 TEST_LOG_LEVEL=DEBUG ./gov-combined-new.sh
```

### Preserve failed environment
```bash
TEST_CLEANUP_ON_FAILURE=0 ./gov-combined-new.sh
```

### Check failure details
```bash
cat /tmp/e2etest_results_*/failure_test_name.txt
```

## Next Steps

1. **Test the framework**: Run with existing tests
2. **Adapt tests**: Add framework features to individual tests
3. **Configure CI/CD**: Use JUnit XML reports
4. **Monitor results**: Review failure reports
5. **Optimize**: Adjust timeouts and retry logic

## Documentation

- **TEST_FRAMEWORK.md** - Comprehensive documentation
- **test_config.sh** - All configuration options
- **test_utils.sh** - All utility functions
- **test_framework.sh** - Core framework implementation

## Benefits

✅ **Faster debugging**: Comprehensive failure reports  
✅ **Better CI/CD**: JUnit XML integration  
✅ **More reliable**: Retry logic and wait conditions  
✅ **Easier maintenance**: Centralized configuration  
✅ **Better visibility**: Complete test results  
✅ **Flexible execution**: Run all, some, or specific tests  

## Support

For detailed information, see **TEST_FRAMEWORK.md**.

For configuration options, see **test_config.sh**.

For utility functions, see **test_utils.sh**.
