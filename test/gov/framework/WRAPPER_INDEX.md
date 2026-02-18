# Test Wrapper Index

This document provides an index of all test wrappers created for the E2E test framework.

## Overview

Test wrappers add framework features to existing test scripts, providing:
- Retry logic
- Wait conditions
- Metrics capture
- Detailed error reporting
- Consistent logging

## Available Test Wrappers

### Core API and Service Tests

1. **apitest_wrapper.sh**
   - Wraps: `apitest.sh`
   - Purpose: API endpoint testing with retry and diagnostics
   - Features: API responsiveness verification, endpoint testing

2. **sync_service_wrapper.sh**
   - Wraps: `sync_service_test.sh`
   - Purpose: CSS/ESS sync service testing
   - Features: CSS health checks, sync service validation

### Agreement and Service Management

3. **verify_agreements_wrapper.sh**
   - Wraps: `verify_agreements.sh`
   - Purpose: Agreement verification with retry logic
   - Features: Agreement count validation, state capture

4. **del_loop_wrapper.sh**
   - Wraps: `del_loop.sh`
   - Purpose: Agreement deletion testing
   - Features: Pre/post deletion state verification, reformation monitoring

5. **service_configstate_wrapper.sh**
   - Wraps: `service_configstate_test.sh`
   - Purpose: Service configuration state testing
   - Features: Service count monitoring, error state detection

### Registration and Configuration

6. **hzn_reg_wrapper.sh**
   - Wraps: `hzn_reg.sh`
   - Purpose: HZN registration/unregistration testing
   - Features: Node state tracking, hzn command validation

7. **pattern_change_wrapper.sh**
   - Wraps: `pattern_change.sh`
   - Purpose: Pattern change testing
   - Features: Pattern transition tracking, agreement reformation

8. **policy_change_wrapper.sh**
   - Wraps: `policy_change.sh`
   - Purpose: Policy change testing
   - Features: Policy modification tracking, agreement impact analysis

### Service Lifecycle

9. **service_upgrade_wrapper.sh**
   - Wraps: `service_upgrading_downgrading_test.sh`
   - Purpose: Service upgrade/downgrade testing
   - Features: Version tracking, service state monitoring

10. **service_secrets_wrapper.sh**
    - Wraps: `service_secrets_test.sh`
    - Purpose: Service secrets management testing
    - Features: Vault integration, secrets validation

### Compatibility and Validation

11. **hzn_compcheck_wrapper.sh**
    - Wraps: `hzn_compcheck.sh`
    - Purpose: Policy compatibility checking
    - Features: HZN command validation, exchange connectivity

12. **verify_surfaced_error_wrapper.sh**
    - Wraps: `verify_surfaced_error.sh`
    - Purpose: Error surfacing verification
    - Features: Event log analysis, error tracking

### Advanced Testing

13. **ha_test_wrapper.sh**
    - Wraps: `ha_test.sh`
    - Purpose: High availability testing
    - Features: System recovery validation, state consistency checks

14. **agbot_apitest_wrapper.sh**
    - Wraps: `agbot_apitest.sh`
    - Purpose: Agreement bot API testing
    - Features: Agbot responsiveness, agreement tracking

## Usage

### Basic Usage

```bash
# Run a wrapper directly
./verify_agreements_wrapper.sh

# Run with retry enabled
TEST_RETRY_ENABLED=1 ./verify_agreements_wrapper.sh

# Run with verbose output
TEST_VERBOSE=1 ./verify_agreements_wrapper.sh
```

### Integration with Test Framework

Wrappers are designed to be called from `gov-combined-new.sh`:

```bash
run_test "test_name" "./test_wrapper.sh"
```

### Configuration

All wrappers respect the test framework configuration:

- `TEST_RETRY_ENABLED` - Enable retry logic
- `TEST_MAX_RETRIES` - Maximum retry attempts
- `TEST_RETRY_DELAY` - Delay between retries
- `TEST_VERBOSE` - Verbose output
- `TEST_TIMEOUT_MULTIPLIER` - Timeout adjustment

## Common Features

All wrappers provide:

1. **Prerequisites Verification**
   - Check required tools and services
   - Verify environment variables

2. **State Capture**
   - Initial state metrics
   - Final state metrics
   - Diagnostic information on failure

3. **Wait Conditions**
   - Wait for anax readiness
   - Wait for service/agreement formation
   - Intelligent polling instead of fixed sleeps

4. **Error Reporting**
   - Detailed failure diagnostics
   - System state on failure
   - Log collection

5. **Retry Logic**
   - Configurable retry attempts
   - Exponential backoff
   - Success/failure tracking

## Creating New Wrappers

Template for creating a new wrapper:

```bash
#!/bin/bash

# Wrapper for <test_script> using the test framework

# Source test framework
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/test_config.sh"
source "${SCRIPT_DIR}/test_utils.sh"

# Test configuration
TEST_NAME="test_name"
TIMEOUT=$(get_timeout 300)

log_message INFO "Starting test"

# Verify prerequisites
if ! verify_prerequisites; then
    log_message ERROR "Prerequisites check failed"
    exit 1
fi

# Capture initial state
capture_metrics "${TEST_NAME}_start"

# Run test with retry
if [ "$TEST_RETRY_ENABLED" == "1" ]; then
    retry_command $TEST_MAX_RETRIES $TEST_RETRY_DELAY "./test_script.sh"
    result=$?
else
    ./test_script.sh
    result=$?
fi

# Capture final state
capture_metrics "${TEST_NAME}_end"

# Report results
if [ $result -eq 0 ]; then
    log_message INFO "Test PASSED"
else
    log_message ERROR "Test FAILED"
    # Collect diagnostics
fi

exit $result
```

## Benefits

Using wrappers provides:

- **Consistency**: All tests use same patterns
- **Reliability**: Retry logic reduces flakiness
- **Debugging**: Comprehensive diagnostics on failure
- **Monitoring**: Metrics capture for analysis
- **Flexibility**: Easy to enable/disable features

## See Also

- **TEST_FRAMEWORK.md** - Comprehensive framework documentation
- **README_TEST_FRAMEWORK.md** - Quick reference guide
- **test_config.sh** - Configuration options
- **test_utils.sh** - Utility functions
- **test_framework.sh** - Core framework implementation
