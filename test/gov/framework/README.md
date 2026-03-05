# E2E Test Framework

This directory contains the comprehensive test framework for anax E2E testing.

## Directory Structure

```
framework/
├── README.md                           # This file
├── TEST_FRAMEWORK.md                   # Comprehensive framework documentation
├── README_TEST_FRAMEWORK.md            # Quick reference guide
├── WRAPPER_INDEX.md                    # Test wrapper index and templates
│
├── test_config.sh                      # Centralized configuration (40+ variables)
├── test_utils.sh                       # Utility functions (wait, retry, assertions)
├── test_framework.sh                   # Test execution engine
├── gov-combined-new.sh                 # Main test orchestrator
│
└── *_wrapper.sh                        # 14 test wrapper scripts
    ├── apitest_wrapper.sh
    ├── sync_service_wrapper.sh
    ├── verify_agreements_wrapper.sh
    ├── del_loop_wrapper.sh
    ├── service_configstate_wrapper.sh
    ├── hzn_reg_wrapper.sh
    ├── pattern_change_wrapper.sh
    ├── policy_change_wrapper.sh
    ├── service_upgrade_wrapper.sh
    ├── service_secrets_wrapper.sh
    ├── hzn_compcheck_wrapper.sh
    ├── verify_surfaced_error_wrapper.sh
    ├── ha_test_wrapper.sh
    └── agbot_apitest_wrapper.sh
```

## Quick Start

### Run All Tests

```bash
cd /path/to/anax/test/gov
./framework/gov-combined-new.sh
```

### Run with Continue-on-Failure

```bash
TEST_CONTINUE_ON_FAILURE=1 ./framework/gov-combined-new.sh
```

### Run Specific Tests

```bash
TEST_FILTER="api_tests,sync_service" ./framework/gov-combined-new.sh
```

### Run with Verbose Output

```bash
TEST_VERBOSE=1 ./framework/gov-combined-new.sh
```

### Generate JUnit XML for CI/CD

```bash
GENERATE_JUNIT_XML=1 ./framework/gov-combined-new.sh
```

## Key Features

✅ **Continue-on-Failure Mode**: Tests run to completion, collecting all results
✅ **Result Aggregation**: Complete statistics with pass/fail reporting
✅ **Test Isolation**: Optional isolated environments per test
✅ **Detailed Reporting**: Comprehensive failure reports with diagnostics
✅ **Wait Utilities**: Intelligent wait conditions replace fixed sleeps
✅ **Retry Logic**: Automatic retry with exponential backoff
✅ **Assertions**: Proper validation functions
✅ **Metrics Capture**: System state capture for debugging
✅ **JUnit XML**: CI/CD integration ready
✅ **Flexible Configuration**: 40+ environment variables
✅ **Backward Compatible**: Works with existing test scripts

## Configuration

All configuration is centralized in `test_config.sh`. Key variables:

- `TEST_CONTINUE_ON_FAILURE`: Continue running tests after failures (default: 0)
- `TEST_VERBOSE`: Enable verbose output (default: 0)
- `TEST_FILTER`: Comma-separated list of tests to run (default: all)
- `TEST_ISOLATION_ENABLED`: Enable test isolation (default: 0)
- `TEST_RETRY_ENABLED`: Enable automatic retry on failure (default: 1)
- `GENERATE_JUNIT_XML`: Generate JUnit XML report (default: 0)

See `test_config.sh` for complete list of configuration options.

## Documentation

- **TEST_FRAMEWORK.md**: Comprehensive framework documentation (15 KB)
  - Architecture and design
  - Configuration reference
  - API documentation
  - Best practices
  - Troubleshooting guide

- **README_TEST_FRAMEWORK.md**: Quick reference guide (6 KB)
  - Getting started
  - Common use cases
  - Configuration examples
  - Integration guide

- **WRAPPER_INDEX.md**: Test wrapper index and templates
  - Complete list of test wrappers
  - Template for creating new wrappers
  - Best practices for wrapper development

## Creating New Test Wrappers

1. Copy an existing wrapper as a template
2. Update the test name and configuration
3. Implement test-specific logic
4. Add proper error handling and cleanup
5. Document the test purpose and requirements
6. Make the script executable: `chmod +x new_wrapper.sh`

See `WRAPPER_INDEX.md` for detailed templates and examples.

## Integration with Existing Tests

The framework is designed to work with existing test scripts in the parent directory (`../`). Wrapper scripts:

1. Source the framework files from this directory
2. Reference actual test scripts from the parent directory using `${PARENT_DIR}`
3. Add framework features (retry, metrics, reporting)
4. Maintain backward compatibility

## Test Execution Flow

```
gov-combined-new.sh (orchestrator)
    ↓
test_framework.sh (execution engine)
    ↓
*_wrapper.sh (test wrappers)
    ↓
../*.sh (actual test scripts in parent directory)
```

## Troubleshooting

### Tests Not Running

- Verify all scripts are executable: `chmod +x framework/*.sh`
- Check that test scripts exist in parent directory: `ls -la ../`
- Review configuration in `test_config.sh`

### Tests Failing

- Enable verbose mode: `TEST_VERBOSE=1`
- Check test logs in `/tmp/test_results/`
- Review metrics captured during test execution
- Verify prerequisites are met (anax running, Exchange accessible)

### Framework Issues

- Check framework logs: `cat /tmp/test_results/framework.log`
- Verify sourcing of framework files succeeds
- Ensure all dependencies are installed (jq, curl, docker/podman)

## Support

For questions or issues:

1. Review the comprehensive documentation in `TEST_FRAMEWORK.md`
2. Check the quick reference in `README_TEST_FRAMEWORK.md`
3. Examine existing wrapper implementations for examples
4. Consult the wrapper index in `WRAPPER_INDEX.md`

## Contributing

When adding new tests or modifying the framework:

1. Follow existing patterns and conventions
2. Add comprehensive documentation
3. Include error handling and cleanup
4. Test with both success and failure scenarios
5. Update relevant documentation files
6. Ensure backward compatibility

## Version

Framework Version: 1.0.0
Last Updated: February 2026
