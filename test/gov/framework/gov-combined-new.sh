#!/bin/bash

# Refactored E2E Test Suite using Test Framework
# This script replaces gov-combined.sh with improved test isolation and result collection

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PARENT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Source test framework
source "${SCRIPT_DIR}/test_config.sh"
source "${SCRIPT_DIR}/test_utils.sh"
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
export USER=${USER:-"anax1"}
export PASS=${PASS:-"anax1pw"}
export TOKEN=${TOKEN:-"Abcdefghijklmno1"}
export EXCH="${EXCH_APP_HOST}"

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
        
        if [ ${CERT_LOC} -eq "1" ]; then
            export HZN_MGMT_HUB_CERT_PATH="/certs/css.crt"
        fi

        if [[ $TEST_DIFF_ORG -eq 1 ]]; then
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
if [[ ${EXCH_APP_HOST} == *"://exchange-api:"* ]]; then
    export REMOTE_HUB=0
else
    export REMOTE_HUB=1
fi
log_message INFO "REMOTE_HUB is set to ${REMOTE_HUB}"

# Update hosts file if needed
if [ "$ICP_HOST_IP" != "0" ]; then
    log_message INFO "Updating hosts file"
    HOST_NAME_ICP=$(echo $EXCH_URL | awk -F/ '{print $3}' | sed 's/:.*//g')
    HOST_NAME=$(echo $EXCH_URL | awk -F/ '{print $3}' | sed 's/:.*//g' | sed 's/\.icp*//g')
    echo "$ICP_HOST_IP $HOST_NAME_ICP $HOST_NAME" >> /etc/hosts
fi

cd /root

# Setup certificate variable
if [ ${CERT_LOC} -eq "1" ]; then
    CERT_VAR="--cacert /certs/css.crt"
else
    CERT_VAR=""
fi

# Create horizon directories
mkdir -p /var/horizon
mkdir -p /var/horizon/.colonus

# Verify prerequisites
log_message INFO "Verifying test prerequisites"
if ! verify_prerequisites; then
    log_message ERROR "Prerequisites check failed"
    exit 1
fi

# Setup real ARCH value in all test files
log_message INFO "Setting up architecture in test files"
for in_file in /root/input_files/compcheck/*.json; do
    sed -i -e "s#__ARCH__#${ARCH}#g" "$in_file"
    if [ $? -ne 0 ]; then
        log_message ERROR "Failed to set architecture in $in_file"
        exit 1
    fi
done

# ============================================================================
# Test Execution Section
# ============================================================================

# Test 1: Build old anax if needed
if [ "$OLDANAX" == "1" ]; then
    run_test "build_old_anax" "./build_old_anax.sh"
fi

# Test 2: CSS/ESS Sync Service Test
if ! should_skip_test "sync_service"; then
    run_test "sync_service" "./sync_service_test.sh"
fi

# Test 3: API Tests (requires anax startup)
if [ "$TESTFAIL" != "1" ] && ! should_skip_test "api_tests"; then
    set_exports
    
    # Start Anax for API tests
    log_message INFO "Starting Anax for API tests"
    if [ ${CERT_LOC} -eq "1" ]; then
        /usr/local/bin/anax -v=5 -alsologtostderr=true -config /etc/colonus/anax-combined.config >/tmp/anax.log 2>&1 &
    else
        /usr/local/bin/anax -v=5 -alsologtostderr=true -config /etc/colonus/anax-combined-no-cert.config >/tmp/anax.log 2>&1 &
    fi
    
    sleep 5
    
    # Wait for anax to be ready
    if wait_for_anax 120; then
        run_test "api_tests" "./apitest.sh"
        
        # Cleanup after API tests
        log_message INFO "Cleaning up after API tests"
        kill $(pidof anax) 2>/dev/null
        rm -fr /root/.colonus/*.db
        rm -fr /root/.colonus/policy.d/*
    else
        log_message ERROR "Anax failed to start for API tests"
        TEST_RESULTS["api_tests"]="FAIL"
        ((FAILED_TESTS++))
    fi
fi

# Test 4: Agbot verification
if [ "$NOAGBOT" != "1" ] && [ "$TESTFAIL" != "1" ] && [ ${REMOTE_HUB} -eq 0 ]; then
    if ! should_skip_test "agbot_verification"; then
        run_test "agbot_verification" "bash -c 'curl -sSL ${AGBOT_API}/agreement > /dev/null'"
        
        if [ "$MULTIAGBOT" == "1" ]; then
            run_test "agbot2_verification" "bash -c 'curl -sSL ${AGBOT2_API}/agreement > /dev/null'"
        fi
    fi
fi

# Test 5: Pattern-based or Policy-based tests
if [ "$TESTFAIL" != "1" ]; then
    if [[ "${TEST_PATTERNS}" == "" ]]; then
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
                run_test "verify_agreements_policy" "bash -c 'ORG_ID=${DEVICE_ORG} ADMIN_AUTH=${admin_auth} ./verify_agreements.sh'"
                
                if [ "$NOCANCEL" != "1" ]; then
                    run_test "delete_agreements_device" "./del_loop.sh"
                    sleep 30
                    run_test "delete_agreements_agbot" "./agbot_del_loop.sh"
                    run_test "verify_agreements_restart" "bash -c 'ORG_ID=${DEVICE_ORG} ADMIN_AUTH=${admin_auth} ./verify_agreements.sh'"
                fi
            fi
        fi
        
        if [ "$NOSVC_CONFIGSTATE" != "1" ]; then
            run_test "service_configstate" "./service_configstate_test.sh"
        fi
        
    else
        # Pattern-based deployment
        log_message INFO "Testing pattern-based deployment"
        
        last_pattern=$(echo $TEST_PATTERNS | sed -e 's/^.*,//')
        
        for pat in $(echo $TEST_PATTERNS | tr "," " "); do
            export PATTERN=$pat
            log_message INFO "Testing pattern: $PATTERN"
            
            # Determine multi-agent pattern
            ma_pattern=$PATTERN
            if [ "${PATTERN}" == "sall" ]; then
                ma_pattern="sns"
            fi
            
            set_exports $pat
            
            # Start node with pattern
            if ! should_skip_test "node_start_${pat}"; then
                run_test "node_start_${pat}" "./start_node.sh"
            fi
            
            # Start multiple agents if configured
            if [ -n "$MULTIAGENTS" ] && [ "$MULTIAGENTS" != "0" ]; then
                source ./multiple_agents.sh
                if ! should_skip_test "multiagent_start_${pat}"; then
                    run_test "multiagent_start_${pat}" "bash -c 'PATTERN=${ma_pattern} startMultiAgents'"
                fi
            fi
            
            # Run agreement verification
            if ! should_skip_test "verify_agreements_${pat}"; then
                admin_auth="e2edevadmin:e2edevadminpw"
                if [ "$DEVICE_ORG" == "userdev" ]; then
                    admin_auth="userdevadmin:userdevadminpw"
                fi
                
                if [ "$NOLOOP" == "1" ]; then
                    run_test "verify_agreements_${pat}" "bash -c 'ORG_ID=${DEVICE_ORG} ADMIN_AUTH=${admin_auth} ./verify_agreements.sh'"
                    
                    if [ "$NOCANCEL" != "1" ]; then
                        run_test "delete_agreements_device_${pat}" "./del_loop.sh"
                        sleep 30
                        run_test "delete_agreements_agbot_${pat}" "./agbot_del_loop.sh"
                        run_test "verify_agreements_restart_${pat}" "bash -c 'ORG_ID=${DEVICE_ORG} ADMIN_AUTH=${admin_auth} ./verify_agreements.sh'"
                    fi
                fi
            fi
            
            # Verify multiple agents
            if [ -n "$MULTIAGENTS" ] && [ "$MULTIAGENTS" != "0" ]; then
                if ! should_skip_test "multiagent_verify_${pat}"; then
                    run_test "multiagent_verify_${pat}" "bash -c 'PATTERN=${ma_pattern} verifyMultiAgentsAgreements'"
                fi
            fi
            
            # Service retry test
            if [ "$NORETRY" != "1" ]; then
                run_test "service_retry_${pat}" "./service_retry_test.sh"
            fi
            
            # Service config state test
            if [ "$NOSVC_CONFIGSTATE" != "1" ]; then
                run_test "service_configstate_${pat}" "./service_configstate_test.sh"
            fi
            
            # Unregister if not last pattern
            if [ "$pat" != "$last_pattern" ]; then
                mv /tmp/anax.log /tmp/anax_$pat.log
                run_test "unregister_${pat}" "./unregister.sh"
                sleep 10
            fi
        done
    fi
fi

# Test 6: Compatibility check tests
if [ "$NOCOMPCHECK" != "1" ] && [ "$TESTFAIL" != "1" ]; then
    if [ "$TEST_PATTERNS" == "sall" ] || [ "$TEST_PATTERNS" == "" ]; then
        run_test "agbot_compcheck" "./agbot_apitest.sh"
        run_test "hzn_compcheck" "./hzn_compcheck.sh"
        
        if [ "$NOVAULT" != "1" ]; then
            run_test "hzn_secretsmanager" "./hzn_secretsmanager.sh"
        fi
    fi
fi

# Test 7: Surface error verification
if [ "$NOSURFERR" != "1" ] && [ "$TESTFAIL" != "1" ] && [ ${REMOTE_HUB} -eq 0 ]; then
    if [ "$TEST_PATTERNS" == "sall" ] || [ "$TEST_PATTERNS" == "" ]; then
        if [ "$NOLOOP" == "1" ]; then
            run_test "verify_surfaced_error" "./verify_surfaced_error.sh"
        fi
    fi
fi

# Test 8: Policy change test
if [ "$NOSURFERR" != "1" ] && [ "$TESTFAIL" != "1" ] && [ ${REMOTE_HUB} -eq 0 ]; then
    if [ "$TEST_PATTERNS" == "" ] && [ "$NOLOOP" == "1" ]; then
        run_test "policy_change" "./policy_change.sh"
    fi
fi

# Test 9: Service upgrade/downgrade test
if [ "$NOUPGRADE" != "1" ] && [ "$TESTFAIL" != "1" ] && [ ${REMOTE_HUB} -eq 0 ]; then
    if [ "$TEST_PATTERNS" == "sall" ]; then
        run_test "service_upgrade_downgrade" "./service_upgrading_downgrading_test.sh"
    fi
fi

# Test 10: Service secrets test
if [ "$NOVAULT" != "1" ] && [ "$TESTFAIL" != "1" ] && [ "$NOLOOP" == "1" ]; then
    if [ "$TEST_PATTERNS" == "" ]; then
        run_test "service_secrets" "./service_secrets_test.sh"
    fi
fi

# Test 11: HZN registration tests
if [ "$NOHZNREG" != "1" ] && [ "$TESTFAIL" != "1" ]; then
    if [ "$TEST_PATTERNS" == "sall" ] || [ "$TEST_PATTERNS" == "" ]; then
        sleep 15
        run_test "hzn_registration" "./hzn_reg.sh"
    fi
fi

# Test 12: Service log test
if [ "$TEST_PATTERNS" == "sall" ] && [ "$NOHZNLOG" != "1" ] && [ "$NOHZNREG" != "1" ] && [ "$TESTFAIL" != "1" ]; then
    run_test "service_log" "./service_log_test.sh"
fi

# Test 13: Pattern change test
if [ "$NOPATTERNCHANGE" != "1" ] && [ "$TESTFAIL" != "1" ]; then
    if [ "$TEST_PATTERNS" == "sall" ]; then
        run_test "pattern_change" "./pattern_change.sh"
    fi
fi

# Test 14: HA test
if [ "$HA" == "1" ]; then
    run_test "ha_test" "./ha_test.sh"
fi

# ============================================================================
# Cleanup Section
# ============================================================================

# Clean up remote environment if needed
if [ ${REMOTE_HUB} -eq 1 ]; then
    log_message INFO "Cleaning up remote environment"
    
    # Delete organizations
    curl -X DELETE $CERT_VAR -u "root/root:${EXCH_ROOTPW}" "${EXCH_URL}/orgs/e2edev@somecomp.com" > /dev/null 2>&1
    curl -X DELETE $CERT_VAR -u "root/root:${EXCH_ROOTPW}" "${EXCH_URL}/orgs/userdev" > /dev/null 2>&1
    curl -X DELETE $CERT_VAR -u "root/root:${EXCH_ROOTPW}" "${EXCH_URL}/orgs/Customer1" > /dev/null 2>&1
    curl -X DELETE $CERT_VAR -u "root/root:${EXCH_ROOTPW}" "${EXCH_URL}/orgs/Customer2" > /dev/null 2>&1
    
    # Delete users
    curl -X DELETE $CERT_VAR -u "root/root:${EXCH_ROOTPW}" "${EXCH_URL}/orgs/IBM/users/ibmadmin" > /dev/null 2>&1
    curl -X DELETE $CERT_VAR -u "root/root:${EXCH_ROOTPW}" "${EXCH_URL}/orgs/IBM/users/agbot1" > /dev/null 2>&1
    
    log_message INFO "Remote cleanup completed"
fi

# ============================================================================
# Print Summary and Exit
# ============================================================================

print_test_summary

# Determine exit code based on results
if [ $FAILED_TESTS -gt 0 ]; then
    log_message ERROR "Test suite completed with failures"
    exit 1
else
    log_message INFO "Test suite completed successfully"
    exit 0
fi
