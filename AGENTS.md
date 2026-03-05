# AGENTS.md

This file provides guidance to agents when working with code in this repository.

> **Last Updated:** March 2026

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

### Documentation Guidelines

**Do Not Create Fix Documentation Files**

When fixing bugs or issues, agents should NOT create separate documentation files (e.g., `FIX_*.md`, `*_FIX.md`, `BUGFIX_*.md`) to document the changes. Instead:

1. **Use Git Commit Messages**: Document the fix in the commit message with clear description of the problem and solution
2. **Update Existing Documentation**: If the fix reveals a gap in documentation, update the relevant existing documentation files
3. **Add Code Comments**: Add comments in the code explaining why the fix was necessary if it's not obvious
4. **Update AGENTS.md**: If the fix reveals a pattern that agents should follow, add it to this file in the appropriate section

**Why This Policy Exists:**
- Fix documentation files accumulate as technical debt
- Information becomes stale and outdated
- Creates maintenance burden
- Proper commit messages and code comments are more maintainable
- Existing documentation should be kept up-to-date instead

**Example of What NOT to Do:**
```bash
# DON'T create files like:
test/gov/framework/SYNC_SERVICE_AUTH_FIX.md
test/gov/FIX_USER_VARIABLE_CONFLICT.md
BUGFIX_AUTHENTICATION.md
MAKEFILE_OPTIMIZATION.md
PERFORMANCE_IMPROVEMENTS.md
```

**This also applies to summary documents:**
- Do NOT create summary documents for changes (e.g., `SUMMARY.md`, `CHANGES_SUMMARY.md`)
- Do NOT create explanation documents for optimizations (e.g., `OPTIMIZATION_GUIDE.md`)
- Changes should be self-documenting through clear code, comments, and commit messages
- If broader documentation is needed, update existing documentation files (README.md, AGENTS.md, etc.)
```

**Example of What TO Do:**
```bash
# DO write clear commit messages:
git commit -m "Fix sync service authentication in GitHub Actions

The hzn CLI was failing with 401 errors because:
1. Missing HZN_EXCHANGE_URL environment variable
2. Shell's USER variable conflicted with test framework's USER variable

Fixed by adding missing env vars and using EXCH_USER local variable."
```

### Change Impact Analysis

**Always Check the Logical Call Stack for Side Effects**

When making changes to code, configuration, or infrastructure, agents must trace the logical call stack to identify all affected components and potential side effects.

**Why This Matters:**
- A change in one location often has ripple effects throughout the system
- Configuration changes can affect multiple consumers
- Missing related changes leads to incomplete fixes and new bugs
- Understanding the full impact prevents partial solutions

**Analysis Process:**

1. **Identify All Consumers**: Find all code/configs that reference or depend on what you're changing
   - Use `search_files` to find all references
   - Check configuration templates that might use the value
   - Look for environment variables that propagate the setting
   - Consider both direct and indirect dependencies

2. **Trace the Data Flow**: Follow how values flow through the system
   - Where is the value set initially?
   - How is it transformed or passed along?
   - What components consume it?
   - Are there multiple paths to the same destination?

3. **Check for Patterns**: Look for similar code that might need the same fix
   - If fixing one config template, check all related templates
   - If fixing one test script, check similar test scripts
   - If fixing one API endpoint, check related endpoints

4. **Verify Assumptions**: Question your understanding of the system
   - Does this value mean what I think it means?
   - Are there edge cases I haven't considered?
   - What happens in different deployment scenarios?

**Real-World Example from This Codebase:**

When fixing the `/root/.colonus` permission issue:
- **Initial Discovery**: Found hardcoded path in `anax-combined-no-cert.config.tmpl`
- **Call Stack Analysis**: 
  - Searched for all `/root/` references in test directory
  - Found 6 different config templates using the same path
  - Found orchestrator script that generates configs from templates
  - Identified that `ANAX_DB_PATH` needed to be set before template expansion
- **Complete Fix**: Updated all 6 templates + orchestrator script
- **Result**: Comprehensive solution instead of partial fix

**Common Pitfalls to Avoid:**

1. **Fixing Only the Immediate Problem**: 
   - Bad: Fix one config file where error occurred
   - Good: Find and fix all config files with the same issue

2. **Not Checking Template Consumers**:
   - Bad: Update a template but not the code that uses it
   - Good: Update template AND ensure variables are exported

3. **Ignoring Similar Patterns**:
   - Bad: Fix `anax-combined.config.tmpl` but miss `anax-combined2.config.tmpl`
   - Good: Search for pattern and fix all instances

4. **Assuming Single Use**:
   - Bad: Assume a config is only used in one scenario
   - Good: Check all test modes and deployment scenarios

**Tools for Impact Analysis:**

```bash
# Find all references to a value
search_files with regex pattern

# Check git history for related changes
git log --all --grep="keyword"

# Find files that import/use a module
grep -r "import.*module" .

# Check for similar patterns
find . -name "*.tmpl" -o -name "*.config"
```

**When to Escalate:**

If impact analysis reveals:
- Changes affecting critical production paths
- Modifications to security-sensitive code
- Breaking changes to public APIs
- Complex interdependencies you don't fully understand

Then: Document findings and ask for human review before proceeding.

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

### Shell Script File Permissions

**CRITICAL: Executable Permissions for Shell Scripts**

When creating shell script files (`.sh` extension), agents MUST set executable permissions:

```bash
# After creating a shell script file, make it executable
chmod +x script_name.sh
```

**Why This Matters:**
- Shell scripts need execute permissions to run directly
- Without execute permissions, scripts must be invoked as `bash script.sh` instead of `./script.sh`
- Test frameworks and automation expect scripts to be directly executable
- Prevents "Permission denied" errors when running scripts

**Implementation:**
- Use `chmod +x` immediately after creating any `.sh` file
- This applies to test scripts, wrappers, utilities, and any executable shell files
- Verify permissions with `ls -la` to confirm execute bit is set

**Example:**
```bash
# Create script
cat > test_script.sh << 'EOF'
#!/bin/bash
echo "Hello"
EOF

# REQUIRED: Make it executable
chmod +x test_script.sh

# Now it can be run directly
./test_script.sh
```

### Shell Script Best Practices

**CRITICAL: Shell Scripting Guidelines for Robustness and Portability**

When writing or modifying shell scripts, follow these best practices to ensure reliability, security, and maintainability:

#### 1. Variable Quoting and Word Splitting

**Always quote variables** to prevent word splitting and glob expansion:

```bash
# BAD - Unquoted variables can cause issues
echo $VARIABLE | awk '{print $1}'
curl -sS $API_URL/endpoint
for item in $LIST; do

# GOOD - Quoted variables are safe
echo "$VARIABLE" | awk '{print $1}'
curl -sS "$API_URL/endpoint"
for item in "$LIST"; do
```

**Exceptions where unquoted variables are acceptable:**
- Variable assignments: `TIMEOUT=$(get_timeout $DEFAULT_TIMEOUT)`
- Intentional word splitting: `docker network rm $networks` (when `$networks` contains space-separated list)
- Inside `[[ ]]` constructs (but prefer `[ ]` for portability)

#### 2. Command Execution with Environment Variables

When passing environment variables to commands, use `eval` for proper execution:

```bash
# BAD - Will fail with "ORG_ID=value: command not found"
$script_name ORG_ID=value ./command.sh

# GOOD - Use eval for proper environment variable handling
eval "$script_name ORG_ID=value ./command.sh"
```

**Why this matters:**
- Shell tries to execute `ORG_ID=value` as a command without `eval`
- `eval` ensures environment variables are set in the command's context
- Critical for test frameworks and wrapper scripts

#### 3. POSIX Compatibility

**Prefer POSIX-compliant syntax** over bash-specific features for better portability:

```bash
# BAD - Bash-specific [[ ]] syntax
if [[ $VAR == "value" ]]; then
if [[ $STRING == *"substring"* ]]; then

# GOOD - POSIX-compliant [ ] syntax
if [ "$VAR" = "value" ]; then
if echo "$STRING" | grep -q "substring"; then

# BAD - Bash-specific == operator
if [ "$VAR" == "value" ]; then

# GOOD - POSIX = operator
if [ "$VAR" = "value" ]; then
```

**Why this matters:**
- Scripts may run in different shells (sh, bash, dash)
- POSIX compliance ensures broader compatibility
- Reduces unexpected behavior across environments

#### 4. Error Handling for Directory Changes

**Always handle cd failures** to prevent commands running in wrong directory:

```bash
# BAD - No error handling
cd /some/directory
rm -rf *  # Dangerous if cd failed!

# GOOD - Error handling with fallback
cd /some/directory || {
    echo "Error: Failed to change to /some/directory"
    exit 1
}

# GOOD - Error handling with warning
cd /some/directory || {
    echo "Warning: Failed to change directory, continuing in current directory"
}
```

#### 5. Command Substitution Safety

**Quote command substitutions** to preserve output integrity:

```bash
# BAD - Unquoted command substitution
result=$(curl -sS $API_URL)
for file in $(ls *.txt); do

# GOOD - Quoted command substitution
result="$(curl -sS "$API_URL")"
while IFS= read -r file; do
    # process "$file"
done < <(find . -name "*.txt")
```

#### 6. Curl Command Safety

**Always quote URLs** in curl commands:

```bash
# BAD - Unquoted URL
curl -sS $ANAX_API/status

# GOOD - Quoted URL
curl -sS "$ANAX_API/status"
```

#### 7. Test Condition Best Practices

**Use proper quoting in test conditions:**

```bash
# BAD - Unquoted variables in tests
if [ $COUNT -eq 0 ]; then
if [ -z $VARIABLE ]; then

# GOOD - Quoted variables in tests
if [ "$COUNT" -eq 0 ]; then
if [ -z "$VARIABLE" ]; then
```

#### 8. Common Pitfalls to Avoid

**Avoid these common mistakes:**

1. **Unquoted variables in echo/command substitution**
   ```bash
   # BAD
   HOST=$(echo $URL | awk '{print $1}')
   
   # GOOD
   HOST=$(echo "$URL" | awk '{print $1}')
   ```

2. **Missing error handling for critical operations**
   ```bash
   # BAD
   source config.sh
   cd /important/directory
   
   # GOOD
   source config.sh || { echo "Failed to load config"; exit 1; }
   cd /important/directory || { echo "Failed to cd"; exit 1; }
   ```

3. **Bash-specific features without shebang**
   ```bash
   # If using bash features, declare it
   #!/bin/bash
   
   # Otherwise, stick to POSIX sh
   #!/bin/sh
   ```

#### 9. Testing Shell Scripts

**CRITICAL REQUIREMENT: shellcheck is MANDATORY for ALL shell script changes**

**Zero Tolerance Policy:**
- ALL shell script modifications MUST pass shellcheck with exit code 0
- NO exceptions - shellcheck failures MUST be fixed before committing
- NO warnings allowed - all shellcheck warnings MUST be addressed
- NO bypassing shellcheck with disable directives unless absolutely necessary and documented
- Code reviews WILL reject any shell script changes without shellcheck validation

**Why This Is Non-Negotiable:**
- Prevents common shell scripting errors that cause production failures
- Catches security vulnerabilities (command injection, path traversal, etc.)
- Ensures POSIX compliance and portability across different shells
- Detects quoting issues that lead to word splitting and glob expansion
- Identifies unsafe practices before they reach production

**MANDATORY Validation Process:**

```bash
# STEP 1: Run shellcheck on ALL modified shell scripts
shellcheck script.sh

# STEP 2: Verify exit code is 0 (no errors or warnings)
echo $?  # Must be 0

# STEP 3: Fix ALL issues reported by shellcheck
# - Address errors immediately
# - Address warnings immediately
# - Document any necessary disable directives with clear justification

# STEP 4: Re-run shellcheck until clean
shellcheck script.sh && echo "PASS: Ready to commit"
```

**Enforcement:**
- CI/CD pipelines SHOULD include shellcheck validation
- Pre-commit hooks SHOULD run shellcheck automatically
- Code reviewers MUST verify shellcheck was run
- Pull requests with shellcheck failures WILL be rejected

**Additional Validation Steps:**

```bash
# Syntax check (catches basic syntax errors)
bash -n script.sh

# Test with different shells (verify POSIX compliance)
sh script.sh
bash script.sh

# Verify executable permissions (required for .sh files)
ls -la script.sh
chmod +x script.sh  # If not executable
```

**When to Run Shellcheck:**
- **ALWAYS** after creating a new shell script
- **ALWAYS** after modifying any existing shell script
- **ALWAYS** before committing changes
- **ALWAYS** during code review
- **RECOMMENDED** in pre-commit hooks
- **RECOMMENDED** in CI/CD pipelines

**Installing Shellcheck:**
```bash
# Debian/Ubuntu
apt-get install shellcheck

# macOS
brew install shellcheck

# Fedora/RHEL
dnf install shellcheck

# Or use online: https://www.shellcheck.net/
```

**Common Shellcheck Issues and Fixes:**

1. **Unquoted Variables (SC2086)**
   ```bash
   # BAD
   curl $URL
   
   # GOOD
   curl "$URL"
   ```

2. **Useless Cat (SC2002)**
   ```bash
   # BAD
   cat file.txt | grep pattern
   
   # GOOD
   grep pattern file.txt
   ```

3. **Unquoted Command Substitution (SC2046)**
   ```bash
   # BAD
   for file in $(ls *.txt); do
   
   # GOOD
   for file in *.txt; do
   ```

4. **Missing Error Handling (SC2164)**
   ```bash
   # BAD
   cd /some/directory
   
   # GOOD
   cd /some/directory || exit 1
   ```

**Acceptable Disable Directives:**

Only use shellcheck disable directives when absolutely necessary:

```bash
# Acceptable: Intentional word splitting
# shellcheck disable=SC2086
docker network rm $networks

# Document WHY the directive is needed
# In this case, $networks contains space-separated list that needs splitting
```

**Unacceptable Reasons to Disable:**
- "It's too much work to fix"
- "The script works fine without it"
- "I don't understand the warning"
- "It's just a warning, not an error"

**Bottom Line:**
If you modify a shell script and don't run shellcheck, your changes WILL be rejected. No exceptions.

#### 10. Security Considerations

**Prevent command injection and path traversal:**

```bash
# BAD - Potential command injection
eval $USER_INPUT

# GOOD - Validate and sanitize input
if [[ "$USER_INPUT" =~ ^[a-zA-Z0-9_-]+$ ]]; then
    eval "$USER_INPUT"
else
    echo "Invalid input"
    exit 1
fi

# BAD - Unquoted variables in sensitive operations
rm -rf $DIRECTORY/*

# GOOD - Quoted and validated
if [ -d "$DIRECTORY" ] && [ -n "$DIRECTORY" ]; then
    rm -rf "${DIRECTORY:?}"/*
fi
```

**Key Takeaways:**
- Quote variables unless you have a specific reason not to
- Use POSIX-compliant syntax for portability
- Handle errors explicitly, especially for cd and source
- Use `eval` when passing environment variables to commands
- Test scripts thoroughly before committing
- Validate and sanitize user input

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

**Test-First Development Methodology**

This project follows a test-first development approach:

1. **Tests Define Expected Behavior**: Write tests that describe the correct, desired behavior of the system
   - Tests should reflect what the code SHOULD do, not what it currently does
   - Never modify test expectations to match existing functional deficiencies
   - If a test fails due to incorrect implementation, fix the implementation, not the test

2. **Functional Improvements Over Test Adjustments**: When tests reveal issues:
   - **Correct Approach**: Improve the implementation to make the test pass
   - **Incorrect Approach**: Change the test to match the broken behavior
   - Exception: Only adjust tests if the original test expectations were genuinely incorrect

3. **Test-Driven Bug Fixes**:
   - Write a failing test that demonstrates the bug
   - Verify the test fails with current code
   - Fix the implementation to make the test pass
   - Never adjust the test to accept the buggy behavior

4. **Example Scenario**:
   ```
   BAD:  Test expects validation to reject invalid input
         → Implementation doesn't validate
         → Change test to accept invalid input ❌
   
   GOOD: Test expects validation to reject invalid input
         → Implementation doesn't validate
         → Add validation to implementation ✓
   ```

5. **When to Adjust Tests**:
   - Original test expectations were based on misunderstanding requirements
   - Requirements have legitimately changed
   - Test was testing implementation details rather than behavior
   - Never adjust tests simply because implementation is difficult to fix

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
7. **Test Execution Without Service Checks**: Running tests without verifying services are available leads to long timeout waits. Always check service availability before running dependent tests.
8. **Network Binding Confusion**: Services bind to `0.0.0.0` to listen on all interfaces, but clients must connect to specific IPs (never `0.0.0.0`). Use separate variables for binding (`E2EDEV_HOST_IP`) and connections (`E2EDEV_CLIENT_IP`).
9. **Working Directory Assumptions**: Test scripts may assume they run from specific directories. Always use proper `cd` with error handling and relative paths based on known directory variables.
10. **Long Timeout Waits**: Not detecting failures early in test loops wastes time. Use connection checks (`nc -z`) to fail fast when services aren't listening, rather than waiting for full timeout periods.

### E2E Test Infrastructure

**Infrastructure Issues Are In-Scope**

E2E (end-to-end) test infrastructure is part of the test scaffolding managed by this project. Infrastructure issues that affect test reliability, performance, or correctness are in-scope and should be fixed, not worked around.

**Scope of Test Infrastructure:**

Test infrastructure includes all components that support test execution:
- Test orchestration and framework scripts
- Docker container builds and network management
- Management hub components (Exchange, Agbot, CSS/ESS, databases)
- Test services and their dependencies
- CI/CD pipeline configuration and caching strategies

**Guiding Principles:**

1. **Fix Root Causes, Don't Work Around**:
   - Infrastructure problems should be resolved at their source
   - Workarounds mask issues and accumulate technical debt
   - Proper fixes improve reliability for all developers

2. **Fail Fast with Clear Diagnostics**:
   - Detect infrastructure problems early rather than waiting for timeouts
   - Provide actionable error messages that explain what failed and why
   - Include context about what succeeded to aid debugging

3. **Idempotent and Isolated**:
   - Tests should work regardless of previous state
   - Clean up stale resources before starting
   - Use unique identifiers to avoid conflicts between concurrent tests
   - Don't assume a clean environment

4. **Performance Matters**:
   - Optimize build times through proper caching strategies
   - Minimize unnecessary rebuilds and resource recreation
   - Balance thoroughness with execution speed

5. **Maintainability Over Convenience**:
   - Infrastructure code should be clear and well-documented
   - Prefer explicit cleanup over implicit assumptions
   - Document why infrastructure decisions were made

**When to Fix vs. Work Around:**

**Fix the Infrastructure When:**
- The issue affects test reliability (flaky tests)
- The issue affects test performance (slow builds, long waits)
- The issue affects test correctness (false positives/negatives)
- The issue is reproducible and understood
- The fix improves the test framework for all users

**Work Around Only When:**
- The issue is in external dependencies beyond project control
- The fix would require major architectural changes
- The workaround is well-documented and maintainable
- The issue is rare and non-critical

**Common Infrastructure Problem Categories:**

1. **Resource Lifecycle Issues**:
   - Stale containers, networks, or volumes from previous runs
   - Improper cleanup on test failure or interruption
   - Resource conflicts between concurrent test executions

2. **Build and Cache Inefficiency**:
   - Unnecessary rebuilds defeating layer caching
   - Cache key mismatches preventing cache reuse
   - Missing or incorrect cache invalidation

3. **Service Readiness and Timing**:
   - Tests starting before services are ready
   - Missing health checks or readiness probes
   - Race conditions in service startup sequences

4. **Environment and Configuration**:
   - Missing or incorrect environment variable propagation
   - Configuration conflicts between test modes
   - Path assumptions that break in different environments

5. **Network and Connectivity**:
   - Network binding vs. connection confusion (0.0.0.0 vs. specific IPs)
   - Stale network resources causing connection failures
   - Port conflicts between services

**Infrastructure Maintenance Practices:**

- **Regular Review**: Monitor test execution times and failure rates
- **Dependency Updates**: Keep infrastructure dependencies current
- **Documentation**: Document infrastructure changes and decisions
- **Cleanup**: Remove obsolete infrastructure and workarounds
- **Validation**: Test infrastructure changes across different environments

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

The Open Horizon ecosystem consists of multiple interconnected projects. Understanding these relationships helps agents work effectively across the platform.

### Core Platform Components

- **[exchange-api](https://github.com/open-horizon/exchange-api)**: Central management hub and system state
  - System state management for all Open Horizon resources
  - Authentication, authorization, and identity management
  - Service, pattern, and policy registry
  - Node registration and lifecycle management
  - Agreement proposal and acceptance workflow
  - Used by: anax (this project), agbot, hzn CLI

- **[edge-sync-service](https://github.com/open-horizon/edge-sync-service)**: Model and file distribution (CSS/ESS)
  - Cloud Sync Service (CSS) for centralized model storage
  - Edge Sync Service (ESS) running on edge nodes
  - Automatic synchronization of ML models and configuration files
  - Used by: anax for model management and file distribution

- **[edge-utilities](https://github.com/open-horizon/edge-utilities)**: Shared utilities and logging
  - Common logging functions and configuration
  - Shared utilities for sync services, anax, hzn, and agbots
  - Consistent logging patterns across Open Horizon components
  - Used by: anax, edge-sync-service, agbot for logging and utilities

- **[OpenBao](https://github.com/openbao/openbao)**: Secrets management (fork of HashiCorp Vault)
  - Secure storage for credentials, API keys, and certificates
  - Open source secrets management solution
  - Used by: anax for secrets injection into services

- **[openbao-plugin-auth-openhorizon](https://github.com/open-horizon/openbao-plugin-auth-openhorizon)**: OpenBao authentication plugin
  - Custom authentication plugin for Open Horizon integration
  - Enables OpenBao to authenticate Open Horizon nodes and services
  - Used by: anax to authenticate with OpenBao for secrets access

### Development and Deployment Tools

- **[devops](https://github.com/open-horizon/devops)**: Development and operations tools
  - Docker Compose configurations for local development
  - Management hub deployment scripts
  - CI/CD pipeline examples
  - Used for: Setting up local test environments

### Documentation and Examples

- **[examples](https://github.com/open-horizon/examples)**: Sample services and patterns
  - Edge service examples (hello world, CPU usage, GPS, etc.)
  - Pattern and policy examples
  - Deployment tutorials
  - Used for: Learning and testing Open Horizon

- **[open-horizon.github.io](https://github.com/open-horizon/open-horizon.github.io)**: Official documentation
  - User guides and tutorials
  - API reference documentation
  - Architecture overviews
  - Note: Documentation from this repository (anax/docs/) is automatically published here

### Deprecated Projects

- **[anax-ui](https://github.com/open-horizon/anax-ui)**: Web-based management interface (deprecated)
  - Replaced by hzn CLI and direct API access
  - Maintained for historical reference only

### Cross-Project Dependencies

When working on anax, be aware of these key dependencies:

1. **Exchange API**: anax communicates with Exchange for all system state, authentication, service discovery, and agreement negotiation
2. **CSS/ESS**: anax uses sync services for model and file distribution
3. **edge-utilities**: anax uses shared logging and utility functions
4. **OpenBao**: anax integrates with OpenBao for secrets management via the Open Horizon auth plugin
5. **Examples**: Test services in anax/test/ are based on patterns from examples repository

### Finding Related Issues

When investigating issues that span multiple projects:
- Check exchange-api for authentication, authorization, service registration, or agreement issues
- Check edge-sync-service for model distribution or file sync issues
- Check edge-utilities for logging or shared utility issues
- Check openbao-plugin-auth-openhorizon for secrets management authentication issues
- Check devops for deployment and configuration issues

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

## GitHub Actions Workflow Best Practices

### Caching Strategy for CI/CD Pipelines

When implementing caching in GitHub Actions workflows, follow these principles to ensure cache effectiveness even during development and debugging phases:

#### The "Never Had a Happy Path" Problem

**Problem**: Traditional caching strategies only save cache on successful workflow runs. If tests consistently fail during development or debugging, the cache never gets populated, leading to:
- Repeated expensive image pulls on every run
- Slower iteration cycles when fixing issues
- Wasted network bandwidth
- Longer time to first successful run

**Solution**: Use `if: always()` conditions on cache save steps to ensure caching happens regardless of test outcomes.

#### Implementation Pattern

**1. Pre-Pull Strategy (Before Tests)**
```yaml
# Pull images if not in cache AND not already in Docker daemon
- name: Pull base image if not cached
  if: steps.cache-image.outputs.cache-hit != 'true'
  run: |
    if ! docker image inspect alpine:latest > /dev/null 2>&1; then
      echo "Pulling alpine:latest..."
      docker pull alpine:latest
    else
      echo "alpine:latest already present, skipping pull"
    fi
```

**Benefits:**
- Avoids redundant pulls if image already exists
- Ensures images are available before tests run
- Enables caching even if tests fail immediately

**2. Always-Save Strategy (After Tests)**
```yaml
# Save to cache whether tests pass or fail
- name: Save image to cache
  if: always() && steps.cache-image.outputs.cache-hit != 'true'
  continue-on-error: true
  run: |
    if docker image inspect alpine:latest > /dev/null 2>&1; then
      echo "Saving alpine:latest to cache..."
      docker save alpine:latest -o /tmp/alpine-image.tar
    else
      echo "alpine:latest not found, skipping cache save"
    fi
```

**Benefits:**
- Cache builds up even during debugging/fixing phase
- Subsequent runs benefit from cached images
- Faster iteration when fixing failing tests
- `continue-on-error: true` prevents blocking failure diagnostics

#### Cache Key Strategy

**Weekly Rotation for Mutable Tags:**
```yaml
key: alpine-image-${{ runner.os }}-latest-${{ steps.date.outputs.week }}
restore-keys: |
  alpine-image-${{ runner.os }}-latest-
```

**Benefits:**
- Balances freshness with cache efficiency
- Mutable tags (`:latest`, `:testing`) get refreshed weekly
- Restore-keys provide fallback to previous week's cache

**Stable Keys for Pinned Tags:**
```yaml
key: mongo-image-${{ runner.os }}-4.0.6
```

**Benefits:**
- Pinned versions never change, so cache key is constant
- Maximum cache hit rate for stable dependencies

#### Workflow Execution Flow

**First Run (No Cache, Tests Fail):**
1. Cache miss → Pull images (only if not present)
2. Tests run and fail
3. `always()` condition → Images still saved to cache ✓
4. Next run benefits from cached images

**Second Run (Cache Hit):**
1. Cache hit → Load images from cache (no pull)
2. Tests run (pass or fail)
3. Cache already populated, no save needed

**Result:** Cache builds incrementally even during development, reducing iteration time and network usage.

#### Best Practices Summary

1. **Use `if: always()` on save steps** - Ensures caching happens regardless of test outcome
2. **Check before pulling** - Avoid redundant pulls if image already exists in Docker daemon
3. **Use `continue-on-error: true`** - Prevents cache failures from blocking diagnostics
4. **Separate cache entries** - One cache entry per image for independent invalidation
5. **Weekly rotation for mutable tags** - Balances freshness with cache efficiency
6. **Stable keys for pinned versions** - Maximizes cache hits for unchanging dependencies
7. **Pre-pull before tests** - Ensures images available even if tests fail early

#### Go Build Cache Strategy

The Go build cache (`~/.cache/go-build`) also benefits from the always-save strategy:

```yaml
# Restore Go build cache
- name: Cache Go build cache
  id: cache-go-build
  uses: actions/cache@v4
  with:
    path: ~/.cache/go-build
    key: go-build-${{ runner.os }}-go1.24-${{ hashFiles('**/*.go') }}
    restore-keys: |
      go-build-${{ runner.os }}-go1.24-

# Build step happens here...

# Save Go build cache even if tests fail
- name: Save Go build cache
  if: always() && steps.cache-go-build.outputs.cache-hit != 'true'
  uses: actions/cache/save@v4
  with:
    path: ~/.cache/go-build
    key: ${{ steps.cache-go-build.outputs.cache-primary-key }}
```

**Benefits:**
- Compiled artifacts cached even when tests fail
- Faster builds on subsequent runs
- Reduced compilation time during debugging
- Works with `actions/cache/save` for explicit post-job saving

**Note:** The `setup-go` action's built-in cache (`cache: true`) handles Go module dependencies (`$GOPATH/pkg/mod`) automatically, but only saves on success. For build artifacts, use the explicit save pattern above.

#### Example: E2E Test Workflow

See [`.github/workflows/E2E-test.yml`](.github/workflows/E2E-test.yml) for a complete implementation that caches:
- Go build cache (`~/.cache/go-build`) - saved even on test failure
- Go module dependencies (`$GOPATH/pkg/mod`) - handled by setup-go action
- Base images (alpine:latest, registry.access.redhat.com/ubi9-minimal:latest)
- Management hub images (Exchange, Agbot, CSS, MongoDB, PostgreSQL, Vault)
- Service images built during tests


### Matrix Job Cache Key Strategy

When using matrix jobs in GitHub Actions, cache keys must be carefully designed to ensure proper cache sharing and reuse.

#### Shared vs. Per-Job Cache Keys

**Principle**: Cache keys should reflect whether the cached content is identical across matrix jobs or unique to each job.

**Shared Resources** (identical across all matrix jobs):
- Management hub images (Exchange, Agbot, CSS, databases)
- Base images (Alpine, UBI, etc.)
- External dependencies that don't vary by test type
- **Cache key pattern**: `resource-${{ runner.os }}-version-${{ date }}`
- **NO matrix variable in key**

**Per-Job Resources** (unique to each matrix job):
- Go build cache (different files compiled per test)
- Test-specific artifacts
- Job-specific build outputs
- **Cache key pattern**: `resource-${{ runner.os }}-${{ hashFiles() }}-${{ matrix.variable }}`
- **INCLUDE matrix variable in key**

**Why This Matters:**
- Shared resources with matrix-specific keys create separate caches that can't be reused
- Per-job resources without matrix-specific keys cause cache conflicts and corruption
- Mismatched save/restore keys result in 0% cache hit rate

**Example:**
```yaml
# Shared image - all jobs use same cache
key: agbot-image-${{ runner.os }}-e2edev-${{ steps.date.outputs.week }}

# Per-job artifact - each job has own cache
key: go-build-${{ runner.os }}-${{ hashFiles('**/*.go') }}-${{ matrix.tests }}
```

### Docker Build Caching in CI/CD

**Principle**: Docker layer caching is essential for fast CI/CD pipelines. Never disable it without good reason.

#### Avoid Cache-Defeating Patterns

**Anti-patterns that break caching:**
1. Using `--no-cache` flag in docker build commands
2. Running `docker rmi` before building (deletes cached layers)
3. Making `build` target depend on `clean` target
4. Unnecessary image deletion in build scripts

**Correct patterns:**
1. Let Docker use its layer cache naturally
2. Only use `--no-cache` when debugging cache-related issues
3. Separate `clean` targets from normal `build` targets
4. Use `docker build -t image:tag .` without additional flags

**When to use `--no-cache`:**
- Debugging suspected cache corruption
- Forcing fresh builds after base image updates
- One-off builds, not in CI/CD pipelines

**Performance impact:**
- With caching: Builds use cached layers, ~75% faster
- Without caching: Full rebuild every time, wastes time and resources

### Test Framework Wrapper Requirements

**Principle**: All tests must use framework wrappers to ensure consistent environment, error handling, and diagnostics.

#### Why Wrappers Are Mandatory

Test wrappers provide critical infrastructure:

1. **Environment Setup**: Export all required variables (CSS_URL, EXCH_APP_HOST, etc.)
2. **Service Verification**: Check prerequisites before running tests
3. **Error Handling**: Consistent error reporting and diagnostics
4. **Retry Logic**: Automatic retries for transient failures
5. **Metrics Collection**: Capture test timing and resource usage
6. **Cleanup**: Ensure proper cleanup even on failure

#### Direct Script Calls Are Incorrect

**Anti-pattern:**
```bash
run_test "api_tests" "./apitest.sh"  # Missing environment setup
```

**Correct pattern:**
```bash
run_test "api_tests" "${FRAMEWORK_DIR}/apitest_wrapper.sh"  # Full infrastructure
```

#### Common Issues from Direct Calls

- Tests fail with "variable not set" errors (CSS_URL, etc.)
- Inconsistent error messages across tests
- Missing diagnostic information on failures
- No retry capability for flaky tests
- Incomplete cleanup leaving test artifacts

#### Wrapper Naming Convention

- Test script: `test_name.sh` in `test/gov/`
- Wrapper: `test_name_wrapper.sh` in `test/gov/framework/`
- Orchestrator calls wrapper, wrapper calls test script
- Wrapper handles all framework integration

## Test Framework Architecture

The anax project uses a comprehensive test framework for E2E (end-to-end) testing located in `test/gov/framework/`.

### Framework Structure

- **Main Orchestrator**: `gov-combined-new.sh` - Coordinates all test execution
- **Test Wrappers**: `*_wrapper.sh` files - Wrap individual test scripts with framework utilities
- **Core Utilities**: 
  - `test_framework.sh` - Core framework functions (logging, test execution, result tracking)
  - `test_utils.sh` - Utility functions (waiting, service checks, cleanup)
  - `test_config.sh` - Environment configuration and variable setup
- **Test Scripts**: Individual test scripts in `test/gov/` directory
- **Migration Guide**: `MIGRATION_GUIDE.md` - Guide for migrating tests to the framework

### Test Orchestration Flow

The `gov-combined-new.sh` script orchestrates tests in this order:

1. **Environment Setup**: Initialize variables, detect configuration
2. **Service Registration**: Register services with Exchange
3. **API Tests**: Start Anax locally and run API tests
4. **Agbot Verification**: Verify agreement bot connectivity
5. **Pattern/Policy Tests**: Run pattern-based or policy-based deployment tests
6. **Compatibility Tests**: Run compatibility check tests
7. **Surface Error Tests**: Verify error surfacing mechanisms
8. **Policy Change Tests**: Test policy update scenarios
9. **Service Tests**: Upgrade/downgrade, secrets, configuration state tests
10. **HZN CLI Tests**: Test hzn command-line interface
11. **Kubernetes Tests**: Test Kubernetes cluster agent deployment

### Test Wrapper Pattern

Test wrappers provide consistent interface and error handling:

```bash
# Example wrapper structure
source "${FRAMEWORK_DIR}/test_framework.sh"

# Test-specific configuration
TEST_NAME="example_test"
TEST_DESCRIPTION="Tests example functionality"

# Run the actual test
run_test_script "${GOV_DIR}/example_test.sh"
```

### Test Configuration

Tests are configured via environment variables in `test/Makefile`:

- **Service URLs**: `EXCH_URL`, `CSS_URL`, `AGBOT_API`, etc.
- **Test Modes**: `REMOTE_HUB`, `CERT_LOC`, `NOLOOP`, `NOCANCEL`
- **Test Selection**: `TEST_PATTERNS`, `NOCOMPCHECK`, `NOVAULT`, etc.
- **Network Config**: `E2EDEV_HOST_IP`, `E2EDEV_CLIENT_IP`

### Adding New Tests

To add a new test to the framework:

1. Create test script in `test/gov/`
2. Create wrapper in `test/gov/framework/` following the pattern
3. Add test execution to `gov-combined-new.sh`
4. Update `WRAPPER_INDEX.md` with test documentation
5. Add any required environment variables to `test_config.sh`

## Test Execution Best Practices

### Conditional Test Execution

Tests should check for service availability before execution:

```bash
# Track service availability
ANAX_AVAILABLE=0

# Try to start service
if wait_for_anax 120; then
    ANAX_AVAILABLE=1
    run_test "api_tests" "./apitest.sh"
else
    log_message ERROR "Anax failed to start"
    log_message WARN "Skipping tests that require Anax"
fi

# Later tests check availability
if [ "$ANAX_AVAILABLE" -eq 1 ]; then
    run_test "dependent_test" "./test_requiring_anax.sh"
else
    log_message WARN "Skipping dependent_test - Anax not available"
fi
```

### Graceful Degradation

When services aren't available, tests should:

1. **Log clear messages** explaining what was skipped and why
2. **Continue with other tests** that don't require the unavailable service
3. **Provide context** about what did succeed (e.g., "Kubernetes tests passed")
4. **Fail fast** instead of waiting for timeouts

### Fast Failure Patterns

Avoid long waits when services aren't running:

```bash
# BAD: Waits full timeout even when connection fails immediately
for i in $(seq 1 100); do
    curl -sS http://localhost:8510/status
    sleep 5
done

# GOOD: Detect connection failure early
for i in $(seq 1 100); do
    if ! curl -sS http://localhost:8510/status 2>/dev/null; then
        if ! nc -z localhost 8510 2>/dev/null; then
            log_message ERROR "Service not listening on port 8510"
            return 1
        fi
    fi
    sleep 5
done
```

### Test Dependencies

Document which tests require which services:

- **Anax on localhost**: Pattern/policy tests, agreement verification, service tests
- **Kubernetes cluster**: Cluster agent tests, operator tests
- **Exchange**: All tests (required)
- **Agbot**: Agreement negotiation tests
- **CSS/ESS**: Sync service tests, model management tests
- **Vault**: Secrets manager tests

### Clear Diagnostic Messages

Always provide context in log messages:

```bash
# BAD: Unclear what failed
log_message ERROR "Test failed"

# GOOD: Clear context and next steps
log_message ERROR "Anax failed to start for API tests"
log_message WARN "Skipping tests that require Anax on localhost"
log_message INFO "Note: Kubernetes cluster agent tests already completed successfully"
```

## Network Configuration for Testing

### Service Binding vs. Client Connections

**Critical Distinction**: Services bind to network interfaces, but clients connect to specific IP addresses.

#### Service Binding

Services should bind to `0.0.0.0` to listen on all interfaces:

```bash
# Service binds to all interfaces
ANAX_LISTEN_IP=0.0.0.0
```

This allows the service to accept connections from:
- localhost (127.0.0.1)
- Host IP (e.g., 192.168.1.100)
- Container networks
- Kubernetes pod networks

#### Client Connections

Clients must connect to a specific IP address, **never** `0.0.0.0`:

```bash
# BAD: Cannot connect to 0.0.0.0
curl http://0.0.0.0:8080/status

# GOOD: Connect to specific IP
curl http://192.168.1.100:8080/status
curl http://127.0.0.1:8080/status
```

### IP Detection for Client Connections

Detect the correct IP for client connections:

```bash
# Detect host IP for client connections
if [ -z "$E2EDEV_CLIENT_IP" ]; then
    # Try to detect from default route
    E2EDEV_CLIENT_IP=$(ip route get 1.1.1.1 | grep -oP 'src \K\S+' 2>/dev/null)
    
    # Fallback to localhost if detection fails
    if [ -z "$E2EDEV_CLIENT_IP" ]; then
        E2EDEV_CLIENT_IP="127.0.0.1"
    fi
fi

# Use for client connections
EXCH_URL="http://${E2EDEV_CLIENT_IP}:8080/v1"
```

### Environment Variable Pattern

Separate binding and connection IPs:

```makefile
# Service binding (all interfaces)
E2EDEV_HOST_IP ?= 0.0.0.0

# Client connections (specific IP)
E2EDEV_CLIENT_IP ?= $(shell ip route get 1.1.1.1 | grep -oP 'src \K\S+' 2>/dev/null || echo "127.0.0.1")

# Service URLs use client IP
EXCH_URL = http://$(E2EDEV_CLIENT_IP):8080/v1
CSS_URL = http://$(E2EDEV_CLIENT_IP):9443
```

### Kubernetes Pod Connectivity

When services run in Kubernetes pods:

1. **Pod-to-Host**: Pods can connect to host services via host IP
2. **Host-to-Pod**: Host connects to pods via NodePort or port forwarding
3. **Pod-to-Pod**: Pods connect via Kubernetes service names

```bash
# Detect if running in Kubernetes
if kubectl get pods -n openhorizon 2>/dev/null; then
    # Get pod IP for connections
    POD_IP=$(kubectl get pod agent-pod -n openhorizon -o jsonpath='{.status.podIP}')
    ANAX_API="http://${POD_IP}:8510"
else
    # Use host IP for local Anax
    ANAX_API="http://${E2EDEV_CLIENT_IP}:8510"
fi
```

## Kubernetes Testing

### MicroK8s Test Environment

The E2E tests use MicroK8s for Kubernetes testing:

```bash
# Start MicroK8s
sudo -E microk8s.start

# Wait for ready
microk8s status --wait-ready

# Enable required addons
microk8s enable dns
microk8s enable helm3
```

### Agent Deployment Modes

Anax can run in two modes:

1. **Host Mode**: Anax runs directly on the host system
   - Used for: Pattern/policy tests, API tests, CLI tests
   - Connects to: localhost:8510

2. **Kubernetes Mode**: Anax runs in a Kubernetes pod
   - Used for: Cluster agent tests, operator tests
   - Connects to: Pod IP or NodePort

### Test Isolation Strategy

Tests are isolated by deployment mode:

```bash
# Kubernetes cluster agent tests
if [ "$REMOTE_HUB" -eq 1 ]; then
    # Deploy agent to Kubernetes
    kubectl apply -f deployment.yaml
    
    # Wait for pod ready
    kubectl wait --for=condition=ready pod/agent-pod --timeout=90s
    
    # Run cluster-specific tests
    run_test "cluster_agent" "./cluster_agent_test.sh"
fi

# Host-based tests (only if Anax available on host)
if [ "$ANAX_AVAILABLE" -eq 1 ]; then
    run_test "host_agent" "./host_agent_test.sh"
fi
```

### Pod Readiness Checks

Always wait for pods to be ready before testing:

```bash
# Wait for pod with timeout
kubectl wait --for=condition=ready pod/agent-pod \
    --namespace=openhorizon \
    --timeout=90s

# Verify pod is running
POD_STATUS=$(kubectl get pod agent-pod -n openhorizon -o jsonpath='{.status.phase}')
if [ "$POD_STATUS" != "Running" ]; then
    log_message ERROR "Pod not running: $POD_STATUS"
    exit 1
fi
```

### Network Connectivity in Kubernetes

Test connectivity to pods:

```bash
# Get pod IP
POD_IP=$(kubectl get pod agent-pod -n openhorizon -o jsonpath='{.status.podIP}')

# Test connectivity
if curl -sS "http://${POD_IP}:8510/status" > /dev/null; then
    log_message INFO "Agent pod is accessible"
else
    log_message ERROR "Cannot connect to agent pod"
fi
```

### Kubernetes Test Cleanup

Always clean up Kubernetes resources:

```bash
# Cleanup function
cleanup_kubernetes() {
    kubectl delete namespace openhorizon --ignore-not-found=true
    kubectl delete clusterrolebinding agent-cluster-rule --ignore-not-found=true
}

# Register cleanup
trap cleanup_kubernetes EXIT
```

## Test Timeout Management

### Dynamic Timeout Calculation

Calculate timeouts based on test parameters:

```bash
# Base timeout + per-item timeout
BASE_TIMEOUT=48
PER_ITEM_TIMEOUT=12
TIMEOUT_MULTIPLIER=${TIMEOUT_MUL:-1}

# Calculate total timeout
NUM_ITEMS=5
TOTAL_LOOPS=$(( (BASE_TIMEOUT + PER_ITEM_TIMEOUT * NUM_ITEMS) * TIMEOUT_MULTIPLIER ))
TIMEOUT_SECONDS=$(( TOTAL_LOOPS * 5 ))

log_message INFO "Timeout: ${TIMEOUT_SECONDS}s for ${NUM_ITEMS} items"
```

### Timeout Multipliers

Use multipliers for different environments:

```bash
# Fast local testing
TIMEOUT_MUL=1

# Slower CI environment
TIMEOUT_MUL=2

# Very slow or resource-constrained environment
TIMEOUT_MUL=3
```

### Early Exit on Failures

Detect failures early instead of waiting full timeout:

```bash
# Track consecutive failures
CONSECUTIVE_FAILURES=0
MAX_CONSECUTIVE_FAILURES=3

for i in $(seq 1 "$MAX_LOOPS"); do
    if curl -sS "$API_URL/status" > /dev/null 2>&1; then
        CONSECUTIVE_FAILURES=0
        # Success - continue waiting for condition
    else
        ((CONSECUTIVE_FAILURES++))
        if [ "$CONSECUTIVE_FAILURES" -ge "$MAX_CONSECUTIVE_FAILURES" ]; then
            log_message ERROR "Service unreachable after $CONSECUTIVE_FAILURES attempts"
            return 1
        fi
    fi
    sleep 5
done
```

### Connection vs. Timeout Failures

Distinguish between connection failures and timeouts:

```bash
# Check if service is listening
if ! nc -z localhost 8510 2>/dev/null; then
    log_message ERROR "Service not listening on port 8510"
    log_message ERROR "This is a connection failure, not a timeout"
    return 1
fi

# Service is listening, but not responding correctly
if ! curl -sS http://localhost:8510/status > /dev/null 2>&1; then
    log_message WARN "Service listening but not responding correctly"
    log_message WARN "Continuing to wait..."
fi
```

## Working Directory Management

### Framework vs. Test Directory

The test framework has two key directories:

- **Framework Directory**: `test/gov/framework/` - Contains framework scripts
- **Test Directory**: `test/gov/` - Contains actual test scripts

### Directory Navigation

Framework scripts must navigate to the test directory:

```bash
# In framework script (test/gov/framework/wrapper.sh)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FRAMEWORK_DIR="$SCRIPT_DIR"
GOV_DIR="$(dirname "$FRAMEWORK_DIR")"

# Change to test directory before running tests
cd "$GOV_DIR" || {
    log_message ERROR "Failed to change to test directory: $GOV_DIR"
    exit 1
}

# Now test scripts can use relative paths
./test_script.sh
```

### Proper cd Error Handling

Always handle directory change failures:

```bash
# BAD: No error handling
cd /some/directory
rm -rf *  # Dangerous if cd failed!

# GOOD: Error handling with exit
cd /some/directory || {
    echo "Error: Failed to change to /some/directory"
    exit 1
}

# GOOD: Error handling with return
cd /some/directory || {
    echo "Error: Failed to change directory"
    return 1
}
```

### Relative Path Handling

Use relative paths consistently:

```bash
# From test directory (test/gov/)
./test_script.sh                    # Current directory
./framework/wrapper.sh              # Subdirectory
../docker/fs/hzn/service/hello/     # Parent directory

# Avoid absolute paths when possible
# BAD: /home/user/anax/test/gov/test_script.sh
# GOOD: ./test_script.sh
```

### GOV_DIR Variable

Use `GOV_DIR` to track the test directory:

```bash
# Set in framework initialization
export GOV_DIR="${GOV_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"

# Use in test scripts
source "${GOV_DIR}/framework/test_utils.sh"
"${GOV_DIR}/test_script.sh"
```

## E2E Test Environment

### Test Makefile Configuration

The `test/Makefile` configures the E2E test environment:

```makefile
# Network configuration
E2EDEV_HOST_IP ?= 0.0.0.0
E2EDEV_CLIENT_IP ?= $(shell ip route get 1.1.1.1 | grep -oP 'src \K\S+' 2>/dev/null || echo "127.0.0.1")

# Service URLs
EXCH_URL = http://$(E2EDEV_CLIENT_IP):8080/v1
CSS_URL = http://$(E2EDEV_CLIENT_IP):9443
AGBOT_API = http://$(E2EDEV_CLIENT_IP):8046

# Test modes
REMOTE_HUB ?= 0          # 1 = Kubernetes mode, 0 = host mode
CERT_LOC ?= 0            # 1 = use certificates, 0 = no certificates
NOLOOP ?= 0              # 1 = skip loop tests
NOCANCEL ?= 0            # 1 = skip agreement cancellation tests
```

### Key Environment Variables

#### Test Mode Variables

- `REMOTE_HUB`: Controls deployment mode
  - `0` = Host mode (Anax on localhost)
  - `1` = Kubernetes mode (Anax in pod)

- `CERT_LOC`: Certificate configuration
  - `0` = No certificates (testing)
  - `1` = Use certificates (production-like)

- `NOLOOP`: Loop test control
  - `0` = Run loop tests (agreement verification, etc.)
  - `1` = Skip loop tests (faster testing)

- `NOCANCEL`: Agreement cancellation tests
  - `0` = Run cancellation tests
  - `1` = Skip cancellation tests

#### Test Selection Variables

- `TEST_PATTERNS`: Comma-separated list of patterns to test
  - Empty = Policy-based deployment
  - `"sall"` = All patterns
  - `"sns,sloc"` = Specific patterns

- `NOCOMPCHECK`: Skip compatibility check tests
- `NOVAULT`: Skip Vault/secrets manager tests
- `NOUPGRADE`: Skip service upgrade/downgrade tests
- `NOSVC_CONFIGSTATE`: Skip service configuration state tests
- `NORETRY`: Skip service retry tests
- `NOHZNREG`: Skip hzn registration tests

#### Service Configuration

- `EXCH_URL`: Exchange API URL
- `CSS_URL`: Cloud Sync Service URL
- `AGBOT_API`: Agreement Bot API URL
- `AGBOT2_API`: Second Agreement Bot API (for multi-agbot tests)
- `VAULT_URL`: Vault secrets manager URL

### Test Modes

#### Pattern-Based Testing

Tests services deployed via patterns:

```bash
export TEST_PATTERNS="sall"  # Test all patterns
export PATTERN="sns"         # Specific pattern

# Pattern tests include:
# - Service registration
# - Node registration with pattern
# - Agreement formation
# - Service execution
# - Agreement cancellation
```

#### Policy-Based Testing

Tests services deployed via policies:

```bash
export TEST_PATTERNS=""      # Empty = policy mode
export PATTERN=""            # No pattern

# Policy tests include:
# - Business policy creation
# - Node policy configuration
# - Policy-based agreement formation
# - Policy updates
# - Service deployment
```

### Test Skipping Strategies

Skip tests based on environment:

```bash
# Skip test if variable is set
if [ "$NOCOMPCHECK" != "1" ]; then
    run_test "compatibility_check" "./compcheck.sh"
fi

# Skip test if service not available
if [ "$ANAX_AVAILABLE" -eq 1 ]; then
    run_test "api_test" "./apitest.sh"
else
    log_message WARN "Skipping api_test - Anax not available"
fi

# Skip test if in wrong mode
if [ "$REMOTE_HUB" -eq 0 ]; then
    run_test "host_test" "./host_test.sh"
fi
```

### Environment Setup

Tests set up environment in this order:

1. **Load configuration**: Source `test_config.sh`
2. **Detect network**: Set `E2EDEV_CLIENT_IP`
3. **Start services**: Exchange, CSS, Agbot, Vault
4. **Register services**: Publish to Exchange
5. **Run tests**: Execute test suite
6. **Cleanup**: Stop services, remove data

This strategy ensures the cache builds up progressively, even when tests are failing during development, significantly reducing iteration time, compilation time, and network bandwidth usage.
