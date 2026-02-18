# Migration Guide: Using the Framework Directory

## Overview

The E2E test framework has been organized into a dedicated `framework/` directory for better clarity and maintainability. This guide explains the new structure and how to use it.

## Directory Structure

```
test/gov/
├── framework/                          # NEW: Framework directory
│   ├── README.md                       # Framework overview
│   ├── TEST_FRAMEWORK.md               # Comprehensive documentation
│   ├── README_TEST_FRAMEWORK.md        # Quick reference
│   ├── WRAPPER_INDEX.md                # Test wrapper index
│   ├── MIGRATION_GUIDE.md              # This file
│   │
│   ├── test_config.sh                  # Configuration
│   ├── test_utils.sh                   # Utilities
│   ├── test_framework.sh               # Execution engine
│   ├── gov-combined-new.sh             # Main orchestrator
│   │
│   └── *_wrapper.sh                    # Test wrappers (14 files)
│
└── *.sh                                # Original test scripts (unchanged)
```

## Key Changes

### 1. Framework Files Location

**Before:**
```bash
test/gov/test_config.sh
test/gov/test_utils.sh
test/gov/test_framework.sh
test/gov/gov-combined-new.sh
```

**After:**
```bash
test/gov/framework/test_config.sh
test/gov/framework/test_utils.sh
test/gov/framework/test_framework.sh
test/gov/framework/gov-combined-new.sh
```

### 2. Test Wrapper Files Location

**Before:**
```bash
test/gov/apitest_wrapper.sh
test/gov/sync_service_wrapper.sh
# ... etc
```

**After:**
```bash
test/gov/framework/apitest_wrapper.sh
test/gov/framework/sync_service_wrapper.sh
# ... etc
```

### 3. Original Test Scripts

**Unchanged:**
```bash
test/gov/apitest.sh
test/gov/sync_service.sh
test/gov/verify_agreements.sh
# ... all original test scripts remain in test/gov/
```

## Path References

### Framework Files (in framework/ directory)

Wrapper scripts source framework files from their own directory:

```bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PARENT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Source framework files from same directory
source "${SCRIPT_DIR}/test_config.sh"
source "${SCRIPT_DIR}/test_utils.sh"
```

### Test Scripts (in parent directory)

Wrapper scripts reference actual test scripts from parent directory:

```bash
# Run test script from parent directory
"${PARENT_DIR}/apitest.sh"
"${PARENT_DIR}/sync_service.sh"
```

## Usage

### Running Tests

**New command (from test/gov/):**
```bash
cd test/gov
./framework/gov-combined-new.sh
```

**With options:**
```bash
# Continue on failure
TEST_CONTINUE_ON_FAILURE=1 ./framework/gov-combined-new.sh

# Verbose output
TEST_VERBOSE=1 ./framework/gov-combined-new.sh

# Specific tests only
TEST_FILTER="api_tests,sync_service" ./framework/gov-combined-new.sh

# Generate JUnit XML
GENERATE_JUNIT_XML=1 ./framework/gov-combined-new.sh
```

### Running Individual Wrappers

```bash
cd test/gov
./framework/apitest_wrapper.sh
./framework/sync_service_wrapper.sh
```

## Benefits of New Structure

1. **Clear Separation**: Framework code is separate from test scripts
2. **Easy Navigation**: All framework files in one directory
3. **Better Organization**: Related files grouped together
4. **Maintainability**: Easier to update framework without affecting tests
5. **Documentation**: All docs in one place
6. **Backward Compatible**: Original test scripts unchanged

## Creating New Test Wrappers

When creating new test wrappers in the framework directory:

```bash
#!/bin/bash

# Source test framework
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PARENT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
source "${SCRIPT_DIR}/test_config.sh"
source "${SCRIPT_DIR}/test_utils.sh"

# Test configuration
TEST_NAME="my_new_test"

# Run test from parent directory
"${PARENT_DIR}/my_test_script.sh"
```

## Troubleshooting

### Script Not Found Errors

If you see errors like "script not found":

1. Verify you're in the correct directory: `pwd` should show `.../test/gov`
2. Check the script exists: `ls -la framework/`
3. Verify script is executable: `chmod +x framework/*.sh`

### Sourcing Errors

If framework files can't be sourced:

1. Check paths in wrapper scripts
2. Verify framework files exist: `ls -la framework/test_*.sh`
3. Check file permissions: `ls -la framework/`

### Test Script Not Found

If test scripts can't be found:

1. Verify test scripts are in parent directory: `ls -la *.sh`
2. Check PARENT_DIR is set correctly in wrapper
3. Use absolute paths if needed

## Migration Checklist

For existing test scripts:

- [ ] Original test scripts remain in `test/gov/` (no changes needed)
- [ ] Framework files moved to `test/gov/framework/`
- [ ] Test wrappers moved to `test/gov/framework/`
- [ ] All scripts are executable (`chmod +x framework/*.sh`)
- [ ] Path references updated in wrapper scripts
- [ ] Documentation moved to `test/gov/framework/`
- [ ] Test execution works from `test/gov/` directory

## Support

For questions or issues:

1. Review `framework/README.md` for overview
2. Check `framework/TEST_FRAMEWORK.md` for comprehensive docs
3. See `framework/README_TEST_FRAMEWORK.md` for quick reference
4. Consult `framework/WRAPPER_INDEX.md` for wrapper examples

## Version

Migration Guide Version: 1.0.0
Framework Version: 1.0.0
Last Updated: February 2026
