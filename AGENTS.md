# AGENTS.md

This file provides guidance to agents when working with code in this repository.

> **Last Updated:** February 2026

## Project Overview

**anax** is the Horizon client system - the core agent software for Open Horizon edge computing platform. It enables autonomous management of containerized workloads on edge devices and clusters through agreement-based computing.

### Core Components

- **anax**: The main agent daemon that runs on edge devices/clusters
- **hzn CLI**: Command-line interface for managing the agent and interacting with the Exchange
- **Agreement Bot (agbot)**: Server-side component that negotiates agreements with edge nodes
- **CSS/ESS**: Cloud Sync Service and Edge Sync Service for model management and file distribution
- **Exchange Integration**: Communicates with the Exchange API for service discovery and agreement negotiation

### Technology Stack

- **Language**: Go 1.24+
- **Database**: BoltDB (edge), PostgreSQL (agbot)
- **Container Runtime**: Docker, Kubernetes
- **Architecture Support**: amd64, arm64, armhf, ppc64el, s390x, riscv64
- **Platforms**: Linux, macOS (CLI only)

### Architecture

anax uses an event-driven worker architecture:
- **Workers**: Independent components handling specific responsibilities (agreements, governance, API, containers, etc.)
- **Event System**: Message-based communication between workers
- **Policy Manager**: Manages deployment policies and constraints
- **Agreement Protocol**: Negotiates and manages service agreements between edge nodes and agbots

## Building and Running

### Prerequisites

- Go 1.24 or later
- Docker (for container builds)
- Make
- For linting: `go vet`, `golint`, `jshint`

### Build Commands

```bash
# Build all components (anax, hzn CLI, CSS, ESS)
make

# Build for specific architecture (cross-compilation)
export arch=arm64
export opsys=Linux
make

# Build with verbose output
make verbose=y

# Build with UPX compression
make USE_UPX=true
```

### Testing

```bash
# Run all checks (lint + unit tests + integration tests)
make check

# Unit tests only
make test

# Integration tests only
make test-integration

# Linting and static analysis
make lint

# Run tests with race detection
go test -race ./...

# Run coverage
make coverage
```

### Docker Images

```bash
# Build anax agent container
make anax-image

# Build agreement bot container
make agbot-image

# Build anax for Kubernetes
make anax-k8s-image

# Build file sync services (ESS/CSS)
make fss

# Push images to registry
make docker-push
```

### Package Building

```bash
# Debian packages
make debpkgs

# RPM packages
make rpmpkgs

# macOS package
make macpkg
```

## Development Conventions

### Code Organization

- **Main entry point**: `main.go` - initializes workers and event system
- **Workers**: Each major component is a worker (agreement, governance, API, container, etc.)
- **Persistence**: Database abstractions in `persistence/` and `agreementbot/persistence/`
- **API**: REST APIs in `api/` (agent) and `agreementbot/` (agbot)
- **Policy**: Policy management in `policy/`, `businesspolicy/`, `externalpolicy/`

### File and Module Organization Principles

1. **Keep Files Manageable**: Aim to keep individual source files under 1000 lines where practical
   - If a file grows beyond 1500 lines, consider splitting it into multiple files or subpackages
   - Large files are harder to navigate, review, and maintain

2. **Leverage Go's Package Structure**: 
   - Create new packages/directories when a set of related functionality becomes substantial
   - Each package should have a clear, single responsibility
   - Use internal packages for implementation details that shouldn't be exposed

3. **When to Create New Modules/Directories**:
   - When a feature or component has multiple related types and functions (>500 lines)
   - When functionality is logically distinct and could be tested independently
   - When code could potentially be reused by other parts of the system
   - When a file has grown too large and splitting within the same package isn't sufficient

4. **File Splitting Strategies**:
   - Split by functionality: `worker.go`, `worker_helpers.go`, `worker_handlers.go`
   - Split by type: `bolt_persistence.go`, `postgres_persistence.go`
   - Split by concern: `api.go`, `api_handlers.go`, `api_validation.go`
   - Keep related tests in corresponding `*_test.go` files

5. **Package Naming**:
   - Use short, descriptive package names (e.g., `policy`, not `policymanagement`)
   - Avoid generic names like `util`, `common`, `helpers` for new packages
   - Package name should describe what it provides, not what it contains

**Example of Good Module Organization:**
```
persistence/
  ├── persistence.go          # Interface definitions and common types
  ├── persistence_test.go     # Common persistence tests
  ├── bolt.go                 # BoltDB implementation (~800 lines)
  ├── bolt_helpers.go         # BoltDB helper functions (~400 lines)
  ├── postgres.go             # PostgreSQL implementation (~900 lines)
  └── postgres_helpers.go     # PostgreSQL helper functions (~500 lines)
```

### Source Maintainability Principles

**Avoid Hardcoded Duplication - Favor Centralized Configuration:**

1. **Single Source of Truth**: When the same values, constants, or structures are needed across multiple files or packages, create a centralized, configurable structure rather than duplicating hardcoded values.

2. **Configuration Over Hardcoding**: 
   - **Bad Practice**: Hardcoding values like timeouts, limits, or file extensions in multiple files
   - **Good Practice**: Define configuration parameters once with defaults, reference everywhere
   - Benefits: Single point of change, consistent behavior, user customization, easier testing

3. **Reusable Structures**:
   - Create shared types, constants, and helper functions in appropriate packages
   - Use configuration parameters for values that might need customization
   - Document the centralized structure and its usage in code comments

4. **Real-World Example**:
   ```go
   // Bad: Hardcoded in multiple files
   timeout := 30  // Repeated in 5 different workers
   
   // Good: Centralized in configuration
   type Config struct {
       WorkerTimeout int `json:"worker_timeout"`
   }
   
   // Usage with fallback to defaults
   timeout := config.WorkerTimeout
   if timeout == 0 {
       timeout = 30
   }
   ```

5. **When to Centralize**:
   - Values used in 3+ locations
   - Security-related constants (file extensions, timeouts, limits)
   - Business logic constants that might change
   - Environment-specific settings
   - Validation rules and constraints

6. **Maintainability Benefits**:
   - **Consistency**: Same behavior across all usages
   - **Flexibility**: Easy to customize per deployment
   - **Testability**: Single place to mock or override for tests
   - **Documentation**: Clear intent and purpose in one location
   - **Evolution**: Easy to extend or modify without hunting through codebase

### Code Style and Organization

**Indentation and Formatting:**
- **Use spaces, not tabs** for indentation in non-Go files
- **Standard indent size: 4 spaces** for non-Go files
- For Go files, follow `gofmt` conventions (which uses tabs)
- **Trim trailing whitespace** from all lines when editing files
- **End files with a single blank line** (newline at EOF)
- Ensure your editor is configured to:
  - Insert spaces when Tab key is pressed (for non-Go files)
  - Display tabs as 4 spaces for consistency
  - Automatically trim trailing whitespace on save
  - Ensure newline at end of file

**Function Ordering:**
- Prefer lexical (alphabetical) ordering of functions within a file where it makes sense
- Exceptions: Group related helper functions near their primary function when it improves comprehension
- Public functions (exported) should generally appear before private functions (unexported)
- Keep `init()` functions at the top of the file after package-level variables

**Lexical Ordering for Other Constructs:**
Apply alphabetical ordering where it makes sense for:
- **Variable declarations**: Group related variables, but within groups prefer alphabetical order
- **Struct fields**: Order fields alphabetically unless logical grouping (e.g., related fields, embedded types first) improves clarity
- **Constants**: Within const blocks, prefer alphabetical ordering
- **Interface methods**: Order methods alphabetically in interface definitions
- **Configuration parameters**: In configuration structs, order fields alphabetically for easier lookup
- **Import statements**: Go automatically formats imports, but group standard library, external, and internal imports

**When NOT to use lexical ordering:**
- When fields have dependencies or initialization order matters
- When grouping by functionality significantly improves comprehension
- When following established patterns in the existing codebase
- When struct field order affects memory layout for performance reasons

**Readability Principles:**
- Prioritize code legibility and readability without sacrificing technical depth or solution quality
- Use clear, descriptive variable and function names that convey intent
- Add comments for complex logic, but prefer self-documenting code
- Break down complex functions into smaller, well-named helper functions
- Use whitespace and formatting to visually separate logical blocks
- Avoid deeply nested code; prefer early returns and guard clauses

**Code Structure:**
- Keep functions focused on a single responsibility
- Limit function length to what fits on a screen when possible (aim for < 50 lines)
- Use consistent error handling patterns throughout the codebase
- Document exported functions, types, and constants with godoc-style comments
- Place package-level constants and variables at the top of files

**Example of Good Organization:**
```go
package example

// Package-level constants
const (
    DefaultTimeout = 30
    MaxRetries     = 3
)

// Package-level variables
var (
    globalCache map[string]interface{}
)

// init function
func init() {
    globalCache = make(map[string]interface{})
}

// Exported functions (alphabetically ordered)
func CreateAgreement(name string) error { ... }

func DeleteAgreement(id string) error { ... }

func GetAgreement(id string) (*Agreement, error) { ... }

func UpdateAgreement(id string, data []byte) error { ... }

// Unexported helper functions (alphabetically ordered)
func buildAgreementKey(id string) string { ... }

func validateAgreementData(data []byte) error { ... }
```

### Coding Standards

- **Formatting**: Use `make format` to run `gofmt` on all Go files
- **Linting**: Code must pass `make lint` (golint + go vet)
- **Error Handling**: Use glog for logging with appropriate verbosity levels
- **Testing**: Tag tests as `unit`, `integration`, or `ci` using build tags

### Pull Request Guidelines

1. **One commit per PR**: Squash all commits before submitting for review
2. **Commit message format**: "Issue xxxx - short description"
3. **Sign commits**: Use `git commit -s` to sign with DCO
4. **PR template**: Fill out the provided template completely
5. **Testing**: Ensure all tests pass before requesting review

### Internationalization (i18n)

**REST API Responses:**
- All REST API requests and responses MUST handle multilingual support
- Error messages, status messages, and user-facing text must be localizable
- Use Go's text/message package for translation
- Support language negotiation via Accept-Language headers where applicable
- Ensure consistent message formatting across different languages

**Logging Messages:**
- Logging messages are more permissive and can default to English
- Internal debug/trace logs do not require multilingual support
- Focus multilingual efforts on user-facing API responses and error messages
- Log messages should still be clear and descriptive for debugging purposes

**Supported Locales:**
- de, es, fr, it, ja, ko, pt_BR, zh_CN, zh_TW
- Update catalogs: `make i18n-catalog`
- Test with: `HZN_LANG=fr hzn version`

### Dependency Management

**Go Module Updates and Security:**

1. **Regular Dependency Checks**: 
   - Check for dependency updates during any ongoing source code work
   - Use `go list -m -u all` to check for available updates
   - Use `go mod tidy` to clean up unused dependencies

2. **Security Vulnerability Scanning**:
   - Run `go list -json -m all | nancy sleuth` or similar tools to check for CVEs
   - Use GitHub's Dependabot or similar automated scanning tools
   - Prioritize updates that address security vulnerabilities (CVEs)
   - Document CVE fixes in commit messages and pull requests

3. **Update Strategy - Prioritize Stability**:
   - **Critical Security Updates**: Apply immediately after testing
   - **Minor/Patch Updates**: Apply regularly, test thoroughly
   - **Major Version Updates**: Evaluate carefully, may require code changes
   - Always run full test suite after dependency updates: `go test ./...`
   - Always run race detector after updates: `go test -race ./...`
   - Verify builds succeed on all target platforms (amd64, arm64, armhf, ppc64el, s390x, riscv64)

4. **Testing After Updates**:
   ```bash
   # Update dependencies
   go get -u ./...
   go mod tidy
   go mod vendor
   
   # Verify build
   go build ./...
   
   # Run tests
   go test ./...
   go test -race ./...
   
   # Run coverage
   make coverage
   ```

5. **Dependency Update Guidelines**:
   - Never update dependencies without running the full test suite
   - Document breaking changes in dependency updates
   - If an update breaks the build, either fix the code or pin to the previous version
   - Use `go mod vendor` to ensure reproducible builds
   - Commit `go.mod` and `go.sum` changes together with any required code changes

6. **Handling Breaking Changes**:
   - Review changelogs and migration guides for major version updates
   - Create separate commits for dependency updates vs. code adaptations
   - Test with PostgreSQL and BoltDB if updating related dependencies
   - Verify container builds still work after updates

**Go Language Version Updates:**

1. **Regular Version Checks**:
   - Check for Go language updates during ongoing development work
   - Monitor Go release notes for security patches and improvements
   - Current project requirement: Go 1.24+

2. **Update Strategy - Prioritize Stability**:
   - **Security Patches**: Apply Go patch releases promptly after testing
   - **Minor Version Updates**: Evaluate and test thoroughly before upgrading
   - **Major Version Updates**: Plan carefully, may require code changes
   - **Never break the build**: Always verify builds succeed before committing version changes
   - Test on all target platforms (amd64, arm64, armhf, ppc64el, s390x, riscv64)

3. **Testing After Go Version Updates**:
   ```bash
   # Update go.mod
   go mod edit -go=1.25  # Example version
   go mod tidy
   
   # Verify build
   go build ./...
   
   # Run tests
   go test ./...
   go test -race ./...
   
   # Run coverage
   make coverage
   
   # Test container builds
   make anax-image
   ```

4. **Version Update Guidelines**:
   - Update `go.mod` file with new Go version
   - Update documentation (README.md, AGENTS.md) with new version requirement
   - Update CI/CD configurations (.travis.yml, GitHub Actions)
   - Update Dockerfiles if they specify Go version
   - Run full test suite including race detection
   - Verify all container builds succeed
   - Document any code changes required for the new version

5. **Backward Compatibility**:
   - Maintain compatibility with previous minor version when possible
   - Document minimum required Go version clearly
   - Test builds with both old and new versions during transition

### Configuration Management

**Configuration Implementation Requirements:**

1. **Dual Configuration Support**: All configuration parameters SHOULD support both:
   - Configuration file entries (e.g., `anax.config`)
   - Environment variables
   - Environment variables take precedence over file settings

2. **Configuration Layering**:
   - Default values defined in code (see `config/` package)
   - Configuration file values override defaults
   - Environment variables override both defaults and file values
   - This allows flexible deployment across different environments

3. **Adding New Configuration Parameters**:
   - Add field to appropriate `Config` struct in `config/` package
   - Add default value in initialization function
   - Document in configuration file with:
     - Description of the parameter
     - Default value
     - Environment variable name (if applicable)
     - Example usage
     - Appropriate section placement
   - Add validation if needed

4. **Configuration Best Practices**:
   - Use clear, descriptive parameter names
   - Provide sensible defaults for optional parameters
   - Document all parameters thoroughly
   - Validate configuration values early during startup
   - Use appropriate types (bool, int, string, etc.)
   - Group related configuration parameters together

5. **Path Configuration**:
   - Support both relative and absolute path specifications
   - Validate paths exist or can be created during startup
   - Document default paths clearly

6. **Linux Filesystem Hierarchy Standard (FHS) Compliance**:
   - On Linux systems, user-specific configuration may come from the user's home directory structure
   - The Linux Filesystem Hierarchy Standard (FHS), maintained by the LSB (Linux Standard Base) workgroup within the Linux Foundation, defines standard directory structures
   - **User Configuration Locations** (in order of precedence):
     - `~/.config/horizon/` - User-specific configuration files (XDG Base Directory Specification)
     - `~/.horizon/` - Alternative user-specific configuration location
     - `/etc/colonus/` or `/etc/horizon/` - System-wide configuration (default)
   - **FHS Standard Provisions**:
     - `/etc/` - System-wide configuration files
     - `/var/` - Variable data files (logs, databases, runtime state)
     - `/usr/local/` - Locally installed software and configuration
     - `~/.config/` - User-specific application configuration (XDG standard)
     - `~/.local/share/` - User-specific application data
   - **Implementation Considerations**:
     - Check user home directory configuration before falling back to system-wide defaults
     - Respect `XDG_CONFIG_HOME` environment variable if set (defaults to `~/.config`)
     - Respect `XDG_DATA_HOME` environment variable if set (defaults to `~/.local/share`)
     - Ensure proper file permissions when reading from user directories
     - Document configuration file search order in user-facing documentation
   - **Reference**: [Filesystem Hierarchy Standard](https://refspecs.linuxfoundation.org/fhs.shtml) maintained by the Linux Foundation

### Version Management

- Version is set dynamically at build time via `GO_BUILD_LDFLAGS`
- Version defined in `version/version.go`
- Format: `MAJOR.MINOR.PATCH[-BUILD_NUMBER]`
- Exchange version compatibility tracked in `version/version.go`

### Database Management

- **Edge DB**: BoltDB at `${DBPath}/anax.db`
- **Agbot DB**: PostgreSQL or BoltDB (configurable)
- **Migrations**: Handled automatically on startup
- **Cleanup**: DB removed on clean shutdown if configured

### Container Development

- **Dockerfiles**: Located in `anax-in-container/`, `anax-in-k8s/`
- **Base images**: Red Hat UBI (Universal Base Image)
- **Multi-arch**: Use `USE_DOCKER_BUILDX=true` for cross-platform builds
- **Image naming**: `openhorizon/{arch}_{component}:{version}`

### Security Considerations

1. **Authentication**: Token-based for API access
2. **Secrets**: Managed via secrets manager, stored encrypted
3. **Container isolation**: Proper namespace and resource limits
4. **TLS**: Required for Exchange and CSS/ESS communication
5. **Certificate Validation**: 
   - Never disable certificate validation in production
   - Support for certificate pinning where applicable
6. **Path Traversal Protection**: 
   - **CRITICAL**: All file operations MUST use path validation functions
   - Validate all file paths to prevent directory traversal attacks
   - Use absolute paths or properly sanitize relative paths
   - Path validation protects against:
     - Path traversal attacks (../, ../../, etc.)
     - Null byte injection (CWE-158)
     - Symlink attacks (CWE-61)
     - Access to files outside allowed directories
   - Implement allowlists for file extensions where appropriate
   - Test path validation logic thoroughly with security tests
7. **SSRF Protection**: 
   - Validate and sanitize URLs before making external requests
   - Implement allowlists for allowed domains/IP ranges where appropriate
   - Default to blocking access to private IP ranges unless explicitly needed

### Testing Practices

**CRITICAL: Test Coverage Requirements**

All source code changes MUST be accompanied by corresponding test updates:

1. **New Features**: Add comprehensive test cases covering:
   - Happy path scenarios
   - Edge cases and boundary conditions
   - Error handling paths
   - Concurrent access patterns (with race detection)

2. **Bug Fixes**: MANDATORY test requirements:
   - Add a test case that reproduces the bug BEFORE fixing it
   - Verify the test fails with the bug present
   - Verify the test passes after the fix
   - Document the bug scenario in test comments

3. **Security Fixes**: CRITICAL test requirements:
   - Add test cases demonstrating the vulnerability (safely)
   - Test both the attack vector and the mitigation
   - Include tests for bypass attempts
   - Document CVE or security issue reference if applicable
   - Examples: Path traversal tests, injection tests, authentication bypass tests

4. **Refactoring**: Ensure existing tests still pass and add tests for:
   - New code paths introduced
   - Changed behavior or interfaces
   - Performance characteristics if relevant

**Test Isolation Principle:**

All tests MUST be designed with proper isolation to ensure reliability and maintainability:

1. **Independent Test Execution**: Each test must be able to run independently without relying on:
   - Execution order of other tests
   - State left behind by previous tests
   - Shared global state that persists between tests

2. **Clean Test Environment**: 
   - Use `t.TempDir()` for temporary file operations (automatically cleaned up)
   - Create isolated database instances (in-memory or temporary BoltDB)
   - Reset or mock global state in `setUp` functions
   - Clean up resources in `defer` statements or `t.Cleanup()`

3. **Avoid Test Interdependencies**:
   - Never assume test execution order
   - Each test should set up its own required state
   - Don't share test data structures between test cases
   - Use table-driven tests with independent test cases

4. **Parallel Test Safety**:
   - Mark tests as parallel-safe with `t.Parallel()` when appropriate
   - Ensure parallel tests don't share mutable state
   - Use separate database instances for concurrent tests
   - Test with race detector: `go test -race`

5. **External Service Isolation**:
   - Use mock implementations for external dependencies (Exchange, CSS/ESS)
   - Provide test-specific configuration to avoid conflicts
   - Use unique database names or collections for integration tests
   - Clean up test data after integration tests complete

6. **Test Data Isolation**:
   - Generate unique test data for each test case (e.g., unique IDs, timestamps)
   - Avoid hardcoded test data that could conflict across tests
   - Use test-specific prefixes or namespaces for identifiers
   - Clean up test data immediately after test completion

7. **Configuration Isolation**:
   - Create test-specific configuration instances
   - Never modify global configuration in tests
   - Use configuration mocks or test doubles when needed
   - Reset configuration state in test cleanup

8. **Time and Randomness Isolation**:
   - Mock time-dependent functions for deterministic tests
   - Use fixed seeds for random number generators in tests
   - Avoid tests that depend on wall-clock time
   - Make time-sensitive tests configurable with timeouts

9. **Network and I/O Isolation**:
   - Mock network calls and external API interactions
   - Use in-memory implementations instead of real I/O when possible
   - Avoid tests that depend on external network availability
   - Use local test servers or mock servers for integration tests

10. **Example of Good Test Isolation**:
    ```go
    func TestAgreementCreation(t *testing.T) {
        t.Parallel() // Safe to run in parallel
        
        // Create isolated test environment
        tempDir := t.TempDir() // Auto-cleanup
        db := persistence.NewBoltDB(tempDir) // Isolated instance
        
        // Generate unique test data
        testID := fmt.Sprintf("test-%d", time.Now().UnixNano())
        
        // Clean up any resources
        t.Cleanup(func() {
            db.Close()
        })
        
        // Test logic with isolated state
        // ...
    }
    ```

**Test Cleanup and Environmental Responsibility:**

All tests MUST clean up after themselves and avoid leaving destructive changes to the environment:

1. **Complete Cleanup**: Every test must restore the environment to its original state
   - Delete temporary files and directories created during testing
   - Close all open file handles, network connections, and database connections
   - Remove test data from databases and storage systems
   - Restore modified configuration or global state
   - Use `defer` statements or `t.Cleanup()` to ensure cleanup happens even on test failure

2. **No Destructive Side Effects**: Tests must not:
   - Modify files outside the test's temporary directory
   - Delete or overwrite production data or configuration
   - Leave processes running after test completion
   - Consume system resources indefinitely (memory leaks, goroutine leaks)
   - Modify shared system state that affects other tests or processes

3. **Idempotent Tests**: Tests should be repeatable without manual cleanup
   - Running the same test multiple times should produce the same results
   - Tests should not depend on previous test runs being cleaned up
   - Use unique identifiers to avoid conflicts with concurrent test runs

4. **Resource Management**:
   - Always close resources in `defer` or `t.Cleanup()` blocks
   - Use context with timeout for operations that might hang
   - Monitor and clean up goroutines to prevent leaks
   - Release locks and semaphores properly

5. **Example of Proper Cleanup**:
   ```go
   func TestWithProperCleanup(t *testing.T) {
       // Create temporary directory (auto-cleanup)
       tempDir := t.TempDir()
       
       // Create test database connection
       db, err := openTestDB(tempDir)
       require.NoError(t, err)
       defer db.Close() // Ensure connection is closed
       
       // Register cleanup for test data
       t.Cleanup(func() {
           // Remove test data from database
           db.DeleteTestData(testID)
           // Stop any background workers
           stopTestWorkers()
       })
       
       // Test logic here
       // ...
   }
   ```

**When Creating or Updating Tests:**

- **Always verify test isolation**: Run the test multiple times in different orders
- **Test in parallel**: Use `go test -parallel=10` to expose isolation issues
- **Check for race conditions**: Always run `go test -race` on concurrent code
- **Verify cleanup**: Ensure no test artifacts remain after test completion
- **Check for resource leaks**: Monitor goroutines, file handles, and memory usage
- **Document dependencies**: Clearly document any external service requirements
- **Use subtests for variations**: Group related test cases with `t.Run()` for better organization

**Test File Conventions:**
- Unit tests in `*_test.go` files alongside source
- Integration tests require external services (Exchange, PostgreSQL, etc.)
- Race detection enabled for concurrency testing: `go test -race`
- Mock implementations available for testing
- Test databases use in-memory or temporary BoltDB instances

**Test Naming:**
- Use descriptive test names: `TestFunctionName_Scenario_ExpectedBehavior`
- Security tests: Prefix with vulnerability type (e.g., `TestAPI_PathTraversal_Blocked`)
- Bug fix tests: Reference issue number if available (e.g., `TestIssue123_NullPointerFix`)

**Test Documentation Requirements:**

All test functions MUST include comprehensive documentation above the function declaration:

1. **Documentation Structure**:
   - Start with a clear one-line summary of what the test validates
   - List specific test cases and scenarios covered
   - Explain security implications and CWE references where applicable
   - Include usage notes (e.g., "run with -race detector")
   - Explain why the test is critical for the system

2. **Documentation Format**:
   ```go
   // TestFunctionName_Scenario tests [brief description]:
   // - Specific test case 1
   // - Specific test case 2
   // - Edge case or boundary condition
   //
   // Additional context about security implications, CWE references,
   // or why this test is critical for production systems.
   //
   // Usage notes: Run with go test -race for concurrency tests.
   func TestFunctionName_Scenario(t *testing.T) {
       // Test implementation
   }
   ```

3. **Required Documentation Elements**:
   - **What**: Clear description of functionality being tested
   - **How**: List of specific test cases and scenarios
   - **Why**: Security implications, CWE references, or business criticality
   - **Usage**: Special requirements (race detector, external services, etc.)

4. **Security Test Documentation**:
   - MUST include CWE reference numbers (e.g., CWE-22, CWE-79, CWE-89)
   - MUST explain the attack vector being tested
   - MUST explain the mitigation being validated
   - MUST note if test demonstrates vulnerability safely

5. **Examples of Good Test Documentation**:
   ```go
   // TestAPI_PathValidation tests API path validation against traversal attacks:
   // - Parent directory traversal (../, ../../)
   // - Absolute paths outside allowed directories
   // - Null byte injection (CWE-158)
   //
   // This test ensures protection against CWE-22: Path Traversal attacks,
   // preventing unauthorized access to files outside allowed directories.
   // Critical for maintaining filesystem security boundaries.
   func TestAPI_PathValidation(t *testing.T) { ... }
   
   // TestWorker_ConcurrentAgreements tests concurrent agreement processing:
   // - 50 goroutines creating agreements simultaneously
   // - Verifies no race conditions in agreement logic
   // - Ensures consistent database state across concurrent operations
   //
   // Run with: go test -race
   // Critical for production environments with high agreement throughput.
   func TestWorker_ConcurrentAgreements(t *testing.T) { ... }
   ```

6. **Documentation Benefits**:
   - Makes test purpose immediately clear to reviewers
   - Helps maintainers understand security implications
   - Provides context for why tests exist
   - Documents attack vectors and mitigations
   - Serves as inline security documentation

7. **When to Update Documentation**:
   - When adding new test cases to existing tests
   - When fixing bugs that tests should have caught
   - When security vulnerabilities are discovered
   - When test behavior or scope changes

### Common Pitfalls

1. **Database Dependency**: Some tests require PostgreSQL or BoltDB to be available
2. **Path Configuration**: Relative paths resolved against configured base paths
3. **Race Conditions**: Always run race detector when modifying concurrent code
4. **Certificate Handling**: Proper certificate validation required for production
5. **Worker Communication**: Respect event-driven architecture for worker interactions
6. **Missing Tests**: Never commit code changes without corresponding test updates

### Monitoring and Logging

- **Logging**: Uses `glog` with verbosity levels (0-6)
- **Event log**: Structured events in database for API access
- **Metrics**: Exposed via API endpoints
- **Health checks**: `/status` endpoint for liveness/readiness

### Performance Tuning

Key configuration parameters for high-load scenarios:
- Worker pool sizes and queue depths
- Database connection pooling settings
- Agreement negotiation timeouts
- Container resource limits

## Key Workflows

### Agreement Lifecycle

1. Node registers with Exchange
2. Agbot discovers node via Exchange
3. Agbot proposes agreement based on policies
4. Node evaluates proposal against local policy
5. Agreement established, workload deployed
6. Continuous monitoring and governance
7. Agreement cancellation on policy violation or timeout

### Service Deployment

1. Service definition published to Exchange
2. Deployment policy created (pattern or business policy)
3. Node policy matches deployment requirements
4. Agreement negotiated and established
5. Container images pulled and verified
6. Service containers started with proper configuration
7. Health monitoring and automatic recovery

### Node Management

1. Node registration via `hzn register`
2. Policy updates via API or CLI
3. Service configuration via user input
4. Node management status tracking
5. Automatic upgrades via NMP (Node Management Policy)

## Important Notes for AI Agents

### Efficient Tool Usage and Batching Strategies

*


**Principles of Efficient Tool Usage:**

When working with code changes, especially large-scale modifications, agents should optimize tool usage to minimize context usage, reduce costs, and improve performance:

1. **Batch Similar Operations**: Group similar changes together in a single tool call when possible
2. **Use the Right Tool**: Choose tools that can handle multiple operations efficiently
3. **Minimize Round Trips**: Reduce the number of tool calls by combining operations
4. **Plan Before Executing**: Analyze the scope of work before starting to identify batching opportunities

**Batching Strategies for Large Changes:**

Batching is most effective for:
- Adding documentation to multiple functions in the same file
- Applying the same pattern across multiple files
- Making consistent formatting or style changes
- Adding similar test cases across multiple test files
- Updating configuration in multiple locations

**Tools That Support Batching:**

1. **apply_diff**: Can apply multiple search/replace blocks in a single call
   - Each block operates independently within the same file
   - Ideal for making multiple small, precise changes to one file
   - Example: Adding documentation to 5-10 functions in the same file
   - Each search/replace block should be separated by a blank line

2. **replace_regex**: Can apply multiple regex patterns in a single call
   - Supports multiple pattern/replacement pairs in one diff
   - Ideal for consistent pattern-based changes
   - Example: Updating import statements or renaming patterns across a file

**Batching Best Practices:**

1. **Group by File**: Batch all changes for a single file into one tool call
   - Good: One apply_diff call with 10 search/replace blocks for 10 functions
   - Bad: 10 separate apply_diff calls for the same file

2. **Limit Batch Size**: Keep batches manageable (5-15 operations per call)
   - Too small: Wastes tool calls and increases context usage
   - Too large: Harder to debug if one operation fails
   - Sweet spot: 8-12 related operations per batch

3. **Verify Before Batching**: Read the file first to ensure all targets exist
   - Use read_file to examine the file structure
   - Confirm all search strings will match exactly
   - Plan the batch based on actual file content

4. **Handle Failures Gracefully**: If a batch fails, break it into smaller batches
   - Identify which operation failed
   - Complete successful operations first
   - Retry failed operations individually or in smaller groups

**Example: Efficient Documentation Addition**

Instead of 10 separate tool calls:
```
# Inefficient: 10 separate apply_diff calls
<apply_diff> func1 </apply_diff>
<apply_diff> func2 </apply_diff>
...
<apply_diff> func10 </apply_diff>
```

Use one batched call:
```
# Efficient: 1 apply_diff call with 10 blocks
<apply_diff>
<file_path>/path/to/file.go</file_path>
<diff>
# Search: |||
func Function1() {
|||
# Replace with: |||
// Function1 does something important
func Function1() {
|||

# Search: |||
func Function2() {
|||
# Replace with: |||
// Function2 does something else
func Function2() {
|||

... (8 more blocks)
</diff>
</apply_diff>
```

**When NOT to Batch:**

Avoid batching when:
- Operations depend on each other (one must complete before the next)
- Changes span multiple files (use separate tool calls per file)
- Operations are unrelated or serve different purposes
- Debugging is needed (smaller operations are easier to troubleshoot)
- The batch would exceed 15-20 operations (too complex)

**Performance Optimization Tips:**

1. **Read Once, Write Once**: Read a file once, plan all changes, apply in one batch
2. **Use Appropriate Tools**: 
   - `apply_diff` for precise, literal replacements
   - `replace_regex` for pattern-based changes
   - `write_to_file` only for complete file rewrites
3. **Minimize File Reads**: Cache file content mentally during planning phase
4. **Parallel Planning**: While waiting for tool responses, plan the next batch
5. **Progressive Refinement**: Start with a small batch to verify approach, then scale up

### AI Agent Limitations

**DO NOT attempt to provide or calculate:**

1. **Timelines and Schedules**: Do not estimate project timelines, development schedules, or completion dates
2. **Performance Metrics**: Do not calculate or estimate:
   - Latency targets or measurements
   - Throughput rates or capacity
   - Hit rates or cache efficiency
   - Response times or processing speeds
   - Resource utilization percentages
3. **Risk Evaluations**: Do not assess or quantify:
   - Security risk levels or scores
   - Business impact assessments
   - Probability of failures or incidents
   - Cost-benefit analyses
4. **Quantitative Predictions**: Avoid making numerical predictions about system behavior, user adoption, or operational metrics

**Why these limitations exist:**
- AI agents cannot accurately perform these calculations without real-world data
- Estimates and predictions require domain expertise, historical data, and context that agents lack
- Providing inaccurate numbers creates false confidence and can lead to poor decisions
- These assessments require human judgment, stakeholder input, and organizational context

**What agents CAN do:**
- Identify areas where performance testing is needed
- Suggest monitoring and measurement approaches
- Recommend best practices for performance optimization
- Point to relevant documentation or tools for proper assessment
- Implement code changes based on clear, specific requirements

## Documentation Publishing

Documentation from this repository is automatically published to the Open Horizon website:

- **docs/** folder → https://open-horizon.github.io/docs/anax/docs/
- **agent-install/README.md** → https://open-horizon.github.io/docs/anax/docs/overview/

GitHub Actions automatically copy documentation on pushes to master branch. See `docs/AGENTS.md` for details.

## Related Projects

- **exchange-api**: Central service registry and agreement coordination
- **edge-sync-service**: Model and file distribution to edge nodes
- **horizon-deb-packager**: Debian package builder for multiple architectures
- **anax-ui**: Web-based management interface (deprecated)

## Troubleshooting

### Build Issues

- Ensure Go version is 1.24+
- Check `GOPATH` and `TMPGOPATH` settings
- For cross-compilation, verify `arch` and `opsys` variables
- Use `make verbose=y` for detailed build output

### Runtime Issues

- Check logs: `journalctl -u horizon.service -f`
- Increase log level: Set `ANAX_LOG_LEVEL=5`
- Verify Exchange connectivity: `hzn exchange status`
- Check agreement status: `hzn agreement list`

### Container Issues

- Verify Docker/Kubernetes is running
- Check image pull credentials
- Review container logs via API or CLI
- Ensure proper network configuration

## Additional Resources

- [API Documentation](docs/api.md)
- [Managed Workloads](docs/managed_workloads.md)
- [Deployment Policies](docs/deployment_policy.md)
- [Node Management](docs/node_management_overview.md)
- [Test Environment Setup](test/README.md)
