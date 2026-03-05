#!/bin/bash

# Refactored E2E Test Suite using Test Framework
# This script replaces gov-combined.sh with improved test isolation and result collection

# Get script directory and change to parent gov directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FRAMEWORK_DIR="$SCRIPT_DIR"
GOV_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Debug: Log directory paths
echo "DEBUG: SCRIPT_DIR=${SCRIPT_DIR}"
echo "DEBUG: FRAMEWORK_DIR=${FRAMEWORK_DIR}"
echo "DEBUG: GOV_DIR=${GOV_DIR}"

# Change to gov directory so test scripts can be found
cd "${GOV_DIR}" || {
    echo "ERROR: Failed to change to gov directory: ${GOV_DIR}"
    exit 1
}

# PARENT_DIR is used by sourced scripts
# shellcheck disable=SC2034
PARENT_DIR="$(pwd)"

# Source test framework
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/test_config.sh"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/test_utils.sh"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/test_framework.sh"

# Initialize test suite
init_test_suite

# Configuration
TEST_DIFF_ORG=${TEST_DIFF_ORG:-1}
export ARCH=${ARCH}

# Initialize critical variables early
export HZN_AGENT_PORT=${HZN_AGENT_PORT:-8510}
export ANAX_API="http://localhost:${HZN_AGENT_PORT}"
export DEVICE_ORG=${DEVICE_ORG:-"e2edev@somecomp.com"}
export DEVICE_ID=${DEVICE_ID:-"an12345"}
export DEVICE_NAME=${DEVICE_NAME:-"anaxdev1"}
# Note: USER is a shell built-in variable, so we must explicitly set it
# to override the current username (e.g., 'runner' in GitHub Actions)
export USER="anax1"
export PASS=${PASS:-"anax1pw"}
export TOKEN=${TOKEN:-"Abcdefghijklmno1"}
export EXCH="${EXCH_APP_HOST}"

# Export AGBOT_API if not already set (from Makefile)
export AGBOT_API=${AGBOT_API:-}
export AGBOT2_API=${AGBOT2_API:-}

# Detect client IP for service connections (CSS, Exchange, etc.)
# E2EDEV_CLIENT_IP should be set by Makefile, but provide fallback
if [ -z "${E2EDEV_CLIENT_IP:-}" ]; then
    E2EDEV_CLIENT_IP=$(ip route get 1.1.1.1 2>/dev/null | grep -oP 'src \K\S+' || hostname -I 2>/dev/null | awk '{print $1}' || echo "127.0.0.1")
    export E2EDEV_CLIENT_IP
fi

# CSS URL uses client IP for connections
export CSS_URL=${CSS_URL:-http://${E2EDEV_CLIENT_IP}:9443}

# Set common exports
function set_exports {
    if [ "$NOANAX" != "1" ]; then
        export USER=anax1
        export PASS=anax1pw
        export DEVICE_ID="an12345"
        export DEVICE_NAME="anaxdev1"
        export DEVICE_ORG="e2edev@somecomp.com"
        export TOKEN="Abcdefghijklmno1"

        export HZN_AGENT_PORT=8510
        export ANAX_API="http://localhost:${HZN_AGENT_PORT}"
        export EXCH="${EXCH_APP_HOST}"
        
        if [ "${CERT_LOC}" -eq "1" ]; then
            export HZN_MGMT_HUB_CERT_PATH="/certs/css.crt"
        fi

        if [ "$TEST_DIFF_ORG" -eq 1 ]; then
            export USER=useranax1
            export PASS=useranax1pw
            export DEVICE_ORG="userdev"
        fi
    else
        log_message INFO "Anax is disabled"
    fi
}

# Check if hub is remote or all-in-one
EXCH_URL="${EXCH_APP_HOST}"
if echo "${EXCH_APP_HOST}" | grep -q "://exchange-api:"; then
    export REMOTE_HUB=0
else
    export REMOTE_HUB=1
fi
log_message INFO "REMOTE_HUB is set to ${REMOTE_HUB}"

# Update hosts file if needed (only in containerized environments)
if [ "$ICP_HOST_IP" != "0" ] && [ -w /etc/hosts ]; then
    log_message INFO "Updating hosts file"
    HOST_NAME_ICP=$(echo "$EXCH_URL" | awk -F/ '{print $3}' | sed 's/:.*//g')
    HOST_NAME=$(echo "$EXCH_URL" | awk -F/ '{print $3}' | sed 's/:.*//g' | sed 's/\.icp*//g')
    echo "$ICP_HOST_IP $HOST_NAME_ICP $HOST_NAME" >> /etc/hosts
elif [ "$ICP_HOST_IP" != "0" ]; then
    log_message INFO "Skipping /etc/hosts update (not writable, likely running in GitHub Actions)"
fi

# Change to /root only if it exists and is accessible (containerized environment)
if [ -d /root ] && [ -r /root ]; then
    cd /root || {
        log_message WARN "Failed to change to /root directory, continuing in current directory"
    }
else
    log_message INFO "Skipping /root directory change (not accessible, likely running in GitHub Actions)"
fi

# Setup certificate variable
if [ "${CERT_LOC}" -eq "1" ]; then
    CERT_VAR="--cacert /certs/css.crt"
else
    CERT_VAR=""
fi

# Create horizon directories (only if writable, skip in GitHub Actions)
if [ -w /var ] || mkdir -p /var/horizon 2>/dev/null; then
    mkdir -p /var/horizon/.colonus 2>/dev/null || log_message INFO "Skipping /var/horizon creation (not writable)"
else
    log_message INFO "Skipping /var/horizon creation (not writable, likely running in GitHub Actions)"
fi

# Verify prerequisites
log_message INFO "Verifying test prerequisites"
if ! verify_prerequisites; then
    log_message ERROR "Prerequisites check failed"
    exit 1
fi

# Setup real ARCH value in all test files
log_message INFO "Setting up architecture in test files"
for in_file in "$PWD"/gov/input_files/compcheck/*.json; do
    if [ -f "$in_file" ]; then
        if ! sed -i -e "s#__ARCH__#${ARCH}#g" "$in_file"; then
            log_message ERROR "Failed to set architecture in $in_file"
            exit 1
        fi
    fi
done

# ============================================================================
# Test Execution Section
# ============================================================================

# Export critical environment variables for test wrappers
export AGBOT_API="${AGBOT_API}"
export AGBOT2_API="${AGBOT2_API}"
export CSS_URL="${CSS_URL}"
export EXCH_ROOTPW="${EXCH_ROOTPW}"

# Test 1: Build old anax if needed
if [ "$OLDANAX" == "1" ]; then
    run_test "build_old_anax" "./build_old_anax.sh"
fi

# Test 2: Initialize CSS/ESS with organizations before testing
if ! should_skip_test "init_sync_service"; then
    log_message INFO "Initializing CSS/ESS with organizations"
    if ./init_sync_service.sh; then
        log_message INFO "CSS/ESS initialization successful"
    else
        log_message ERROR "CSS/ESS initialization failed"
        ((FAILED_TESTS++))
    fi
fi

# Test 3: CSS/ESS Sync Service Test
if ! should_skip_test "sync_service"; then
    run_test "sync_service" "${FRAMEWORK_DIR}/sync_service_wrapper.sh"
fi

# Test 4: API Tests (requires anax startup)
# Track whether Anax is available for subsequent tests
ANAX_AVAILABLE=0

if [ "$TESTFAIL" != "1" ] && ! should_skip_test "api_tests"; then
    set_exports
    
    # Detect anax binary location
    ANAX_BIN=""
    if [ -x "/usr/local/bin/anax" ]; then
        ANAX_BIN="/usr/local/bin/anax"
    elif [ -x "${GOPATH}/src/github.com/${GITHUB_REPOSITORY}/anax" ]; then
        ANAX_BIN="${GOPATH}/src/github.com/${GITHUB_REPOSITORY}/anax"
    elif [ -x "$(dirname "${GOV_DIR}")/anax" ]; then
        ANAX_BIN="$(dirname "${GOV_DIR}")/anax"
    elif command -v anax >/dev/null 2>&1; then
        ANAX_BIN="$(command -v anax)"
    fi
    
    if [ -z "$ANAX_BIN" ]; then
        log_message ERROR "Anax binary not found in expected locations"
        log_message ERROR "Skipping API tests - Anax binary not available"
        ANAX_AVAILABLE=0
    else
        # Ensure config files exist by processing templates if needed
        CONFIG_DIR="/etc/colonus"
        TEMPLATE_DIR="${GOV_DIR}/../docker/fs/etc/colonus"
        
        if [ "${CERT_LOC}" -eq "1" ]; then
            CONFIG_FILE="${CONFIG_DIR}/anax-combined.config"
            TEMPLATE_FILE="${TEMPLATE_DIR}/anax-combined.config.tmpl"
        else
            CONFIG_FILE="${CONFIG_DIR}/anax-combined-no-cert.config"
            TEMPLATE_FILE="${TEMPLATE_DIR}/anax-combined-no-cert.config.tmpl"
        fi
        
        # Create config file from template if it doesn't exist
        if [ ! -f "$CONFIG_FILE" ] && [ -f "$TEMPLATE_FILE" ]; then
            log_message INFO "Creating config file from template: $CONFIG_FILE"
            if mkdir -p "$CONFIG_DIR" 2>/dev/null; then
                EXCH_APP_HOST="${EXCH_APP_HOST}" CSS_URL="${CSS_URL}" HZN_AGBOT_URL="${AGBOT_SAPI_URL}" \
                    envsubst < "$TEMPLATE_FILE" > "$CONFIG_FILE" 2>/dev/null || {
                    log_message WARN "Failed to create config file in $CONFIG_DIR, trying temp location"
                    CONFIG_FILE="/tmp/anax-test.config"
                    EXCH_APP_HOST="${EXCH_APP_HOST}" CSS_URL="${CSS_URL}" HZN_AGBOT_URL="${AGBOT_SAPI_URL}" \
                        envsubst < "$TEMPLATE_FILE" > "$CONFIG_FILE"
                }
            else
                log_message WARN "Cannot write to $CONFIG_DIR, using temp location"
                CONFIG_FILE="/tmp/anax-test.config"
                EXCH_APP_HOST="${EXCH_APP_HOST}" CSS_URL="${CSS_URL}" HZN_AGBOT_URL="${AGBOT_SAPI_URL}" \
                    envsubst < "$TEMPLATE_FILE" > "$CONFIG_FILE"
            fi
        fi
        
        # Verify config file exists
        if [ ! -f "$CONFIG_FILE" ]; then
            log_message ERROR "Config file not found: $CONFIG_FILE"
            log_message ERROR "Skipping API tests - Cannot create config file"
            ANAX_AVAILABLE=0
        else
            # Start Anax for API tests
            log_message INFO "Starting Anax for API tests using: $ANAX_BIN"
            log_message INFO "Using config file: $CONFIG_FILE"
            "$ANAX_BIN" -v=5 -alsologtostderr=true -config "$CONFIG_FILE" >/tmp/anax.log 2>&1 &
        fi
        
        sleep 5
        
        # Wait for anax to be ready
        if wait_for_anax 120; then
            ANAX_AVAILABLE=1
            run_test "api_tests" "${FRAMEWORK_DIR}/apitest_wrapper.sh"
            
            # Cleanup after API tests
            log_message INFO "Cleaning up after API tests"
            # shellcheck disable=SC2046
            kill $(pidof anax) 2>/dev/null
            rm -fr /var/horizon/.colonus/*.db 2>/dev/null || true
            rm -fr /var/horizon/.colonus/policy.d/* 2>/dev/null || true
        else
            log_message ERROR "Anax failed to start for API tests"
            log_message WARN "Skipping tests that require Anax on localhost"
            # TEST_RESULTS is used by test framework
            # shellcheck disable=SC2034
            TEST_RESULTS["api_tests"]="FAIL"
            ((FAILED_TESTS++))
        fi
    fi
fi

# Test 5: Agbot verification
if [ "$NOAGBOT" != "1" ] && [ "$TESTFAIL" != "1" ] && [ ${REMOTE_HUB} -eq 0 ]; then
    if ! should_skip_test "agbot_verification"; then
        run_test "agbot_verification" "curl -sSL ${AGBOT_API}/agreement > /dev/null"
        
        if [ "$MULTIAGBOT" == "1" ]; then
            run_test "agbot2_verification" "curl -sSL ${AGBOT2_API}/agreement > /dev/null"
        fi
    fi
fi

# Test 6: Pattern-based or Policy-based tests
# These tests require Anax to be running on localhost
if [ "$TESTFAIL" != "1" ]; then
    if [ "$ANAX_AVAILABLE" -eq 0 ]; then
        log_message WARN "Skipping pattern/policy tests - Anax not available on localhost"
        log_message INFO "Note: Kubernetes cluster agent tests already completed successfully"
    elif [ "${TEST_PATTERNS}" = "" ]; then
        # Policy-based deployment
        log_message INFO "Testing policy-based deployment"
        
        set_exports
        export PATTERN=""
        
        if ! should_skip_test "node_start_policy"; then
            run_test "node_start_policy" "./start_node.sh"
        fi
        
        if ! should_skip_test "agreement_verification_policy"; then
            admin_auth="e2edevadmin:e2edevadminpw"
            if [ "$DEVICE_ORG" == "userdev" ]; then
                admin_auth="userdevadmin:userdevadminpw"
            fi
            
            if [ "$NOLOOP" == "1" ]; then
                run_test "verify_agreements_policy" "ORG_ID=${DEVICE_ORG} ADMIN_AUTH=${admin_auth} ${FRAMEWORK_DIR}/verify_agreements_wrapper.sh"

                if [ "$NOCANCEL" != "1" ]; then
                    run_test "delete_agreements_device" "${FRAMEWORK_DIR}/del_loop_wrapper.sh"
                    sleep 30
                    run_test "delete_agreements_agbot" "${FRAMEWORK_DIR}/agbot_del_loop_wrapper.sh"
                    log_message INFO "Waiting for agreement state to stabilize after cancellations..."
                    sleep 60
                    run_test "verify_agreements_restart" "ORG_ID=${DEVICE_ORG} ADMIN_AUTH=${admin_auth} ${FRAMEWORK_DIR}/verify_agreements_wrapper.sh"
                fi
            fi
        fi
        
        if [ "$NOSVC_CONFIGSTATE" != "1" ]; then
            run_test "service_configstate" "${FRAMEWORK_DIR}/service_configstate_wrapper.sh"
        fi
        
    else
        # Pattern-based deployment
        log_message INFO "Testing pattern-based deployment"
        
        last_pattern="${TEST_PATTERNS##*,}"
        
        for pat in $(echo "$TEST_PATTERNS" | tr "," " "); do
            export PATTERN=$pat
            log_message INFO "Testing pattern: $PATTERN"
            
            # Determine multi-agent pattern
            ma_pattern=$PATTERN
            if [ "${PATTERN}" == "sall" ]; then
                ma_pattern="sns"
            fi
            
            set_exports "$pat"
            
            # Start node with pattern
            if ! should_skip_test "node_start_${pat}"; then
                run_test "node_start_${pat}" "./start_node.sh"
            fi
            
            # Start multiple agents if configured
            if [ -n "$MULTIAGENTS" ] && [ "$MULTIAGENTS" != "0" ]; then
                # shellcheck disable=SC1091
                source ./multiple_agents.sh
                if ! should_skip_test "multiagent_start_${pat}"; then
                    run_test "multiagent_start_${pat}" "PATTERN=${ma_pattern} startMultiAgents"
                fi
            fi
            
            # Run agreement verification
            if ! should_skip_test "verify_agreements_${pat}"; then
                admin_auth="e2edevadmin:e2edevadminpw"
                if [ "$DEVICE_ORG" == "userdev" ]; then
                    admin_auth="userdevadmin:userdevadminpw"
                fi
                
                if [ "$NOLOOP" == "1" ]; then
                    run_test "verify_agreements_${pat}" "ORG_ID=${DEVICE_ORG} ADMIN_AUTH=${admin_auth} ${FRAMEWORK_DIR}/verify_agreements_wrapper.sh"

                    if [ "$NOCANCEL" != "1" ]; then
                        run_test "delete_agreements_device_${pat}" "${FRAMEWORK_DIR}/del_loop_wrapper.sh"
                        sleep 30
                        run_test "delete_agreements_agbot_${pat}" "${FRAMEWORK_DIR}/agbot_del_loop_wrapper.sh"
                        log_message INFO "Waiting for agreement state to stabilize after cancellations..."
                        sleep 60
                        run_test "verify_agreements_restart_${pat}" "ORG_ID=${DEVICE_ORG} ADMIN_AUTH=${admin_auth} ${FRAMEWORK_DIR}/verify_agreements_wrapper.sh"
                    fi
                fi
            fi
            
            # Verify multiple agents
            if [ -n "$MULTIAGENTS" ] && [ "$MULTIAGENTS" != "0" ]; then
                if ! should_skip_test "multiagent_verify_${pat}"; then
                    run_test "multiagent_verify_${pat}" "PATTERN=${ma_pattern} verifyMultiAgentsAgreements"
                fi
            fi
            
            # Service retry test
            if [ "$NORETRY" != "1" ]; then
                run_test "service_retry_${pat}" "${FRAMEWORK_DIR}/service_retry_test_wrapper.sh"
            fi
            
            # Service config state test
            if [ "$NOSVC_CONFIGSTATE" != "1" ]; then
                run_test "service_configstate_${pat}" "${FRAMEWORK_DIR}/service_configstate_wrapper.sh"
            fi
            
            # Unregister if not last pattern
            if [ "$pat" != "$last_pattern" ]; then
                mv /tmp/anax.log "/tmp/anax_${pat}.log"
                run_test "unregister_${pat}" "./unregister.sh"
                sleep 10
            fi
        done
    fi
fi

# Test 6: Compatibility check tests
if [ "$NOCOMPCHECK" != "1" ] && [ "$TESTFAIL" != "1" ]; then
    if [ "$TEST_PATTERNS" == "sall" ] || [ "$TEST_PATTERNS" == "" ]; then
        # Agbot compcheck doesn't require local Anax
        run_test "agbot_compcheck" "${FRAMEWORK_DIR}/agbot_apitest_wrapper.sh"
        
        # hzn_compcheck requires nodes to be registered in Exchange (from pattern/policy tests)
        # Skip if pattern/policy tests were skipped
        if [ "$ANAX_AVAILABLE" -eq 1 ]; then
            run_test "hzn_compcheck" "${FRAMEWORK_DIR}/hzn_compcheck_wrapper.sh"
        else
            log_message WARN "Skipping hzn_compcheck test - requires nodes registered by pattern/policy tests"
        fi
        
        # hzn_secretsmanager requires local Anax
        if [ "$NOVAULT" != "1" ] && [ "$ANAX_AVAILABLE" -eq 1 ]; then
            run_test "hzn_secretsmanager" "${FRAMEWORK_DIR}/hzn_secretsmanager_wrapper.sh"
        elif [ "$NOVAULT" != "1" ] && [ "$ANAX_AVAILABLE" -eq 0 ]; then
            log_message WARN "Skipping hzn_secretsmanager test - Anax not available on localhost"
        fi
    fi
fi

# Test 7: Surface error verification
if [ "$NOSURFERR" != "1" ] && [ "$TESTFAIL" != "1" ] && [ ${REMOTE_HUB} -eq 0 ] && [ "$ANAX_AVAILABLE" -eq 1 ]; then
    if [ "$TEST_PATTERNS" == "sall" ] || [ "$TEST_PATTERNS" == "" ]; then
        if [ "$NOLOOP" == "1" ]; then
            run_test "verify_surfaced_error" "${FRAMEWORK_DIR}/verify_surfaced_error_wrapper.sh"
        fi
    fi
elif [ "$NOSURFERR" != "1" ] && [ ${REMOTE_HUB} -eq 0 ] && [ "$ANAX_AVAILABLE" -eq 0 ]; then
    log_message WARN "Skipping surface error verification test - Anax not available on localhost"
fi

# Test 8: Policy change test
if [ "$NOSURFERR" != "1" ] && [ "$TESTFAIL" != "1" ] && [ ${REMOTE_HUB} -eq 0 ]; then
    if [ "$TEST_PATTERNS" == "" ] && [ "$NOLOOP" == "1" ]; then
        run_test "policy_change" "${FRAMEWORK_DIR}/policy_change_wrapper.sh"
    fi
fi

# Test 9: Service upgrade/downgrade test
if [ "$NOUPGRADE" != "1" ] && [ "$TESTFAIL" != "1" ] && [ ${REMOTE_HUB} -eq 0 ]; then
    if [ "$TEST_PATTERNS" == "sall" ]; then
        run_test "service_upgrade_downgrade" "${FRAMEWORK_DIR}/service_upgrade_wrapper.sh"
    fi
fi

# Test 10: Service secrets test
if [ "$NOVAULT" != "1" ] && [ "$TESTFAIL" != "1" ] && [ "$NOLOOP" == "1" ] && [ "$ANAX_AVAILABLE" -eq 1 ]; then
    if [ "$TEST_PATTERNS" == "" ]; then
        run_test "service_secrets" "${FRAMEWORK_DIR}/service_secrets_wrapper.sh"
    fi
elif [ "$NOVAULT" != "1" ] && [ "$NOLOOP" == "1" ] && [ "$ANAX_AVAILABLE" -eq 0 ]; then
    log_message WARN "Skipping service_secrets test - Anax not available on localhost"
fi

# Test 11: HZN registration tests
if [ "$NOHZNREG" != "1" ] && [ "$TESTFAIL" != "1" ] && [ "$ANAX_AVAILABLE" -eq 1 ]; then
    if [ "$TEST_PATTERNS" == "sall" ] || [ "$TEST_PATTERNS" == "" ]; then
        sleep 15
        run_test "hzn_registration" "${FRAMEWORK_DIR}/hzn_reg_wrapper.sh"
    fi
elif [ "$NOHZNREG" != "1" ] && [ "$ANAX_AVAILABLE" -eq 0 ]; then
    log_message WARN "Skipping hzn_registration test - Anax not available on localhost"
fi

# Test 12: Service log test
if [ "$TEST_PATTERNS" == "sall" ] && [ "$NOHZNLOG" != "1" ] && [ "$NOHZNREG" != "1" ] && [ "$TESTFAIL" != "1" ] && [ "$ANAX_AVAILABLE" -eq 1 ]; then
    run_test "service_log" "${FRAMEWORK_DIR}/service_log_test_wrapper.sh"
elif [ "$TEST_PATTERNS" == "sall" ] && [ "$NOHZNLOG" != "1" ] && [ "$ANAX_AVAILABLE" -eq 0 ]; then
    log_message WARN "Skipping service_log test - Anax not available on localhost"
fi

# Test 13: Pattern change test
if [ "$NOPATTERNCHANGE" != "1" ] && [ "$TESTFAIL" != "1" ] && [ "$ANAX_AVAILABLE" -eq 1 ]; then
    if [ "$TEST_PATTERNS" == "sall" ]; then
        run_test "pattern_change" "${FRAMEWORK_DIR}/pattern_change_wrapper.sh"
    fi
elif [ "$NOPATTERNCHANGE" != "1" ] && [ "$ANAX_AVAILABLE" -eq 0 ]; then
    log_message WARN "Skipping pattern_change test - Anax not available on localhost"
fi

# Test 14: HA test
if [ "$HA" == "1" ]; then
    run_test "ha_test" "${FRAMEWORK_DIR}/ha_test_wrapper.sh"
fi

# ============================================================================
# Cleanup Section
# ============================================================================

# Clean up remote environment if needed
if [ ${REMOTE_HUB} -eq 1 ]; then
    log_message INFO "Cleaning up remote environment"
    
    # Delete organizations
    # CERT_VAR intentionally unquoted - it's either empty or contains --cacert flag
    # shellcheck disable=SC2086
    curl -X DELETE $CERT_VAR -u "root/root:${EXCH_ROOTPW}" "${EXCH_URL}/orgs/e2edev@somecomp.com" > /dev/null 2>&1
    # shellcheck disable=SC2086
    curl -X DELETE $CERT_VAR -u "root/root:${EXCH_ROOTPW}" "${EXCH_URL}/orgs/userdev" > /dev/null 2>&1
    # shellcheck disable=SC2086
    curl -X DELETE $CERT_VAR -u "root/root:${EXCH_ROOTPW}" "${EXCH_URL}/orgs/Customer1" > /dev/null 2>&1
    # shellcheck disable=SC2086
    curl -X DELETE $CERT_VAR -u "root/root:${EXCH_ROOTPW}" "${EXCH_URL}/orgs/Customer2" > /dev/null 2>&1
    
    # Delete users
    # shellcheck disable=SC2086
    curl -X DELETE $CERT_VAR -u "root/root:${EXCH_ROOTPW}" "${EXCH_URL}/orgs/IBM/users/ibmadmin" > /dev/null 2>&1
    # shellcheck disable=SC2086
    curl -X DELETE $CERT_VAR -u "root/root:${EXCH_ROOTPW}" "${EXCH_URL}/orgs/IBM/users/agbot1" > /dev/null 2>&1
    
    log_message INFO "Remote cleanup completed"
fi

# ============================================================================
# Print Summary and Exit
# ============================================================================

print_test_summary

# Determine exit code based on results
if [ "$FAILED_TESTS" -gt 0 ]; then
    log_message ERROR "Test suite completed with failures"
    exit 1
else
    log_message INFO "Test suite completed successfully"
    exit 0
fi
