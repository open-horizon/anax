#!/bin/bash

START=$(date +%s)
# common functions
. ./common.sh
# common assertions
. ./assertions.sh

TEST_RUNNING=0

if [ -z ${1+x} ] || [ -z ${2+x} ] ; then echo "All two params (the current version and previous version) required to be set, exiting..."; exit 1; fi

checkConfig

AGENT_VERSION="$1"
PREV_AGENT_VERSION="$2"

prepareDependencies

# 1. Install w/ registrations skipped
testAgentInstallWoRegistration() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    removeVariableFromConfig "HZN_EXCHANGE_PATTERN" "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"
    # prepare packages
    preparePackages "$AGENT_VERSION" "."
    
    .././agent-install.sh -s

    assertTrue "$MESSAGE_HZN_CLI_NOT_FOUND" "$HZN_CLI_INSTALLED"
    assertContains "Wrong expected node state" "$(queryNodeState)" "$NODE_STATE_UNCONFIGURED"

    # teardown
    removeConfig "$CONFIG_DEFAULT"
    removeCert "$CERT_DEFAULT"
    removeEdgePackages "linux"
    uninstallAgent
}

# 2. Install w/ registration but no pattern/policy specified
testAgentInstallWRegistration() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    removeVariableFromConfig "HZN_EXCHANGE_PATTERN" "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"
    # prepare packages
    preparePackages "$AGENT_VERSION" "."

    .././agent-install.sh

    assertTrue "$MESSAGE_HZN_CLI_NOT_FOUND" "$HZN_CLI_INSTALLED"
    assertContains "Wrong expected node state" "$(queryNodeState)" "$NODE_STATE_CONFIGURED"

    # teardown
    removeConfig "$CONFIG_DEFAULT"
    removeCert "$CERT_DEFAULT"
    removeEdgePackages "linux"
    uninstallAgent
}

# 3. Install w/ pattern specified and no pode policy
testAgentInstallWPattern() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"
    # prepare packages
    preparePackages "$AGENT_VERSION" "."

    .././agent-install.sh

    assertTrue "$MESSAGE_HZN_CLI_NOT_FOUND" "$HZN_CLI_INSTALLED"
    assertContains "Wrong expected node state" "$(queryNodeState)" "$NODE_STATE_CONFIGURED"
    assertNotNull "Workload is not executed" "$(waitForWorkloadUntilFinalized)"

    # teardown
    removeConfig "$CONFIG_DEFAULT"
    removeCert "$CERT_DEFAULT"
    removeEdgePackages "linux"
    uninstallAgent
}

# 4. Install w/ policy specified and no pattern
testAgentInstallWNodePolicy() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    removeVariableFromConfig "HZN_EXCHANGE_PATTERN" "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    cp config/${NODE_POLICY} .
    prepareCert "$CERT_DEFAULT"
    # prepare packages
    preparePackages "$AGENT_VERSION" "."

    .././agent-install.sh -n "$NODE_POLICY"

    assertTrue "$MESSAGE_HZN_CLI_NOT_FOUND" "$HZN_CLI_INSTALLED"
    assertContains "Wrong expected node state" "$(queryNodeState)" "$NODE_STATE_CONFIGURED"
    assertNotNull "Workload is not executed" "$(waitForWorkloadUntilFinalized)"

    # teardown
    rm -f ${NODE_POLICY}
    removeConfig "$CONFIG_DEFAULT"
    removeCert "$CERT_DEFAULT"
    removeEdgePackages "linux"
    uninstallAgent
}

# 5. Install w/ both pattern and policy specified for the node
testAgentInstallWNodePolicyAndPattern() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    cp config/${NODE_POLICY} .
    prepareCert "$CERT_DEFAULT"
    # prepare packages
    preparePackages "$AGENT_VERSION" "."

    CMD=$(.././agent-install.sh -n "$NODE_POLICY" -p "$PATTERN"); ret=$?
    
    assertFalse "Script successfully executed" "${ret}"
    
    # teardown
    rm -f ${NODE_POLICY}
    removeConfig "$CONFIG_DEFAULT"
    removeCert "$CERT_DEFAULT"
    removeEdgePackages "linux"
    uninstallAgent
}

# 6. Older version installed already (node registered), installing newer version.
testAgentInstalledOlderVersionInstallNewer() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"
    # installing previous version
    preparePackages "$PREV_AGENT_VERSION" "."
    # install an older version
    .././agent-install.sh

    assertTrue "$MESSAGE_HZN_CLI_NOT_FOUND" "$HZN_CLI_INSTALLED"
    assertContains "Wrong expected node state" "$(queryNodeState)" "$NODE_STATE_CONFIGURED"
    assertNotNull "Workload is not executed" "$(waitForWorkloadUntilFinalized)"
    assertEquals "Expected different agent version" "$PREV_AGENT_VERSION" "$(getAgentVersion)" 
    
    removeEdgePackages "linux"
    # installing previous version
    preparePackages "$AGENT_VERSION" "."

    echo "y" | .././agent-install.sh

    assertTrue "$MESSAGE_HZN_CLI_NOT_FOUND" "$HZN_CLI_INSTALLED"
    assertContains "Wrong expected node state" "$(queryNodeState)" "$NODE_STATE_CONFIGURED"
    assertNotNull "Workload is not executed" "$(waitForWorkloadUntilFinalized)"
    assertEquals "Expected different agent version" "$AGENT_VERSION" "$(getAgentVersion)" 

    # clean up
    removeConfig "$CONFIG_DEFAULT"
    removeCert "$CERT_DEFAULT"
    removeEdgePackages "linux"
    uninstallAgent
}

# 7. Newer version installed, installing older version but selecting no to prompts
testAgentInstalledNewerVersionInstallOlderNo() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"
    # installing previous version
    preparePackages "$AGENT_VERSION" "."
    # install an older version
    .././agent-install.sh

    assertTrue "$MESSAGE_HZN_CLI_NOT_FOUND" "$HZN_CLI_INSTALLED"
    assertContains "Wrong expected node state" "$(queryNodeState)" "$NODE_STATE_CONFIGURED"
    assertNotNull "Workload is not executed" "$(waitForWorkloadUntilFinalized)"

    removeEdgePackages "linux"
    # installing previous version
    preparePackages "$PREV_AGENT_VERSION" "."

    # install an older version
    printf 'n' | .././agent-install.sh

    assertTrue "$MESSAGE_HZN_CLI_NOT_FOUND" "$HZN_CLI_INSTALLED"
    assertContains "Wrong expected node state" "$(queryNodeState)" "$NODE_STATE_CONFIGURED"
    assertNotNull "Workload is not executed" "$(waitForWorkloadUntilFinalized)"
    assertEquals "Expected different agent version" "$AGENT_VERSION" "$(getAgentVersion)"

    # clean up
    removeConfig "$CONFIG_DEFAULT"
    removeCert "$CERT_DEFAULT"
    removeEdgePackages "linux"
    uninstallAgent
}

# 8. Newer version installed, installing older version and yes to the prompt
testAgentInstalledNewerVersionInstallOlder() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"
    # installing current version
    preparePackages "$AGENT_VERSION" "."
    # install a current version
    .././agent-install.sh

    assertTrue "$MESSAGE_HZN_CLI_NOT_FOUND" "$HZN_CLI_INSTALLED"
    assertContains "Wrong expected node state" "$(queryNodeState)" "$NODE_STATE_CONFIGURED"
    assertNotNull "Workload is not executed" "$(waitForWorkloadUntilFinalized)"

    removeEdgePackages "linux"
    # installing previous version
    preparePackages "$PREV_AGENT_VERSION" "."
    
    # install an older version
    .././agent-install.sh << EOF
y
y
EOF

    assertTrue "$MESSAGE_HZN_CLI_NOT_FOUND" "$HZN_CLI_INSTALLED"
    assertContains "Wrong expected node state" "$(queryNodeState)" "$NODE_STATE_CONFIGURED"
    assertNotNull "Workload is not executed" "$(waitForWorkloadUntilFinalized)"
    assertEquals "Expected different agent version" "$PREV_AGENT_VERSION" "$(getAgentVersion)"

    # clean up
    removeConfig "$CONFIG_DEFAULT"
    removeCert "$CERT_DEFAULT"
    removeEdgePackages "linux"
    uninstallAgent
}


# 9. Validate any missing dependency packages are automatically installed
testAgentDependecies(){
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"
    # installing previous version
    preparePackages "$AGENT_VERSION" "."

    removeAgentDependencies

    # install agent
    CMD=$(.././agent-install.sh); ret=$?
    
    assertTrue "Script failed" "${ret}"
    assertTrue "$MESSAGE_HZN_CLI_NOT_FOUND" "$HZN_CLI_INSTALLED"
    assertContains "Wrong expected node state" "$(queryNodeState)" "$NODE_STATE_CONFIGURED"
    assertNotNull "Workload is not executed" "$(waitForWorkloadUntilFinalized)"
    
    assertTrue "$MESSAGE_DOCKER_NOT_FOUND" "$DOCKER_INSTALLED"
    assertTrue "$MESSAGE_JQ_NOT_FOUND" "$JQ_INSTALLED"
    assertTrue "$MESSAGE_CURL_NOT_FOUND" "$CURL_INSTALLED"
    
    # teardown
    installAgentDependencies
    removeConfig "$CONFIG_DEFAULT"
    removeCert "$CERT_DEFAULT"
    removeEdgePackages "linux"
    uninstallAgent
}

# 10. Check that CLI autocomplete works for hzn CLI
testAgentCliAutocomplete() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"
    # prepare packages
    preparePackages "$AGENT_VERSION" "."

    .././agent-install.sh

    assertTrue "$MESSAGE_HZN_CLI_NOT_FOUND" "$HZN_CLI_INSTALLED"
    assertContains "Wrong expected node state" "$(queryNodeState)" "$NODE_STATE_CONFIGURED"
    assertNotNull "Workload is not executed" "$(waitForWorkloadUntilFinalized)"
    assertNotEquals "Autocomplete file not found" 0 "$(findCliAutocomplete)"
    assertEquals "Autocomplete record not found" 0 "$(checkCliAutocomplete)"

    # teardown
    removeConfig "$CONFIG_DEFAULT"
    removeCert "$CERT_DEFAULT"
    removeEdgePackages "linux"
    uninstallAgent

}

# 11. Missing config file produces an error
testAgentMissingConfig() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"
    # prepare packages
    preparePackages "$AGENT_VERSION" "."
    mv "$CONFIG_DEFAULT" "${CONFIG_DEFAULT}-error.cfg"

    CMD=$(.././agent-install.sh); ret=$?
    
    assertFalse "Script successfully executed" "${ret}"

    # teardown
    rm -f "${CONFIG_DEFAULT}-error.cfg"
    removeCert "$CERT_DEFAULT"
    removeEdgePackages "linux"
    uninstallAgent

}

# 12. Missing cert file produces an error
testAgentMissingCert() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"
    # prepare packages
    preparePackages "$AGENT_VERSION" "."
    mv "$CERT_DEFAULT" "${CERT_DEFAULT}-error.crt"

    CMD=$(.././agent-install.sh); ret=$?
    
    assertFalse "Script successfully executed" "${ret}"

    # teardown
    mv -f "${CERT_DEFAULT}-error.crt" "$CERT_DEFAULT"
    removeCert "$CERT_DEFAULT"
    removeConfig "$CONFIG_DEFAULT"
    removeEdgePackages "linux"
    uninstallAgent
}

# 13. Package path option specified (use non-default path)
testAgentPackagePath() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"
    getOS
    getDistro
    getArch
    mkdir "custom-path"
    preparePackages "$AGENT_VERSION" "."
    cp ${OS}/${DISTRO}/${CODENAME}/${ARCH}/* custom-path

    .././agent-install.sh -i "custom-path"

    assertTrue "$MESSAGE_HZN_CLI_NOT_FOUND" "$HZN_CLI_INSTALLED"
    assertContains "Wrong expected node state" "$(queryNodeState)" "$NODE_STATE_CONFIGURED"
    assertNotNull "Workload is not executed" "$(waitForWorkloadUntilFinalized)"

    # teardown
    removeCert "$CERT_DEFAULT"
    removeConfig "$CONFIG_DEFAULT"
    removeEdgePackages "custom-path"
    removeEdgePackages "linux"
    uninstallAgent
}

# 14. Missing default package path throws an error
testAgentMissingDefaultPath() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"
    preparePackages "$AGENT_VERSION" "."
    mv "linux" "linux-error"

    CMD=$(.././agent-install.sh); ret=$?
    
    assertFalse "Script successfully executed" "${ret}"

    # teardown
    removeCert "$CERT_DEFAULT"
    removeConfig "$CONFIG_DEFAULT"
    removeEdgePackages "linux-error"
    uninstallAgent
}

# 15. Custom config file option specified
testAgentCustomConfig() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"
    preparePackages "$AGENT_VERSION" "."
    mv "$CONFIG_DEFAULT" "${CONFIG_DEFAULT}-custom.cfg"

    CMD=$(.././agent-install.sh -k ${CONFIG_DEFAULT}-custom.cfg)

    assertContains "Custom config is not used" "$CMD" "Configuration file: ${CONFIG_DEFAULT}-custom.cfg"
    assertTrue "$MESSAGE_HZN_CLI_NOT_FOUND" "$HZN_CLI_INSTALLED"
    assertContains "Wrong expected node state" "$(queryNodeState)" "$NODE_STATE_CONFIGURED"
    assertNotNull "Workload is not executed" "$(waitForWorkloadUntilFinalized)"

    # teardown
    rm -f "${CONFIG_DEFAULT}-custom.cfg"
    removeCert "$CERT_DEFAULT"
    removeEdgePackages "linux"
    uninstallAgent
}

# 16. Custom certificate file option specifed
testAgentCustomCert() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"
    preparePackages "$AGENT_VERSION" "."
    mv "$CERT_DEFAULT" "${CERT_DEFAULT}-custom.crt"

    CMD=$(.././agent-install.sh -c ${CERT_DEFAULT}-custom.crt)

    assertContains "Custom certificate is not used" "$CMD" "Certification file: ${CERT_DEFAULT}-custom.crt"
    assertTrue "$MESSAGE_HZN_CLI_NOT_FOUND" "$HZN_CLI_INSTALLED"
    assertContains "Wrong expected node state" "$(queryNodeState)" "$NODE_STATE_CONFIGURED"
    assertNotNull "Workload is not executed" "$(waitForWorkloadUntilFinalized)"

    # teardown
    mv "${CERT_DEFAULT}-custom.crt" "$CERT_DEFAULT"
    removeCert "$CERT_DEFAULT"
    removeConfig "$CONFIG_DEFAULT"
    removeEdgePackages "linux"
    uninstallAgent
}

# 17. Any missed required params in config file generate an error
testAgentMissedReqParams() {
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"
    # setup
    preparePackages "$AGENT_VERSION" "."

    for VAR in HZN_EXCHANGE_URL HZN_FSS_CSSURL HZN_ORG_ID HZN_EXCHANGE_USER_AUTH;
    do
        prepareConfig "$CONFIG_DEFAULT"
        removeVariableFromConfig "$VAR" "$CONFIG_DEFAULT"
        CMD=$(.././agent-install.sh); ret=$?
        assertFalse "Script successfully executed" "${ret}"
    done;

    # teardown
    removeCert "$CERT_DEFAULT"
    removeConfig "$CONFIG_DEFAULT"
    removeEdgePackages "linux"
    uninstallAgent
}

# 18. Specify an invalid logging level
testAgentWrongLoggingLevel(){
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"
    preparePackages "$AGENT_VERSION" "."

    CMD=$(.././agent-install.sh -l 999); ret=$?
    assertFalse "Script successfully executed" "${ret}"

    # teardown
    removeCert "$CERT_DEFAULT"
    removeConfig "$CONFIG_DEFAULT"
    removeEdgePackages "linux"
    uninstallAgent
}

# 19. Validate environment variables override the config params
testAgentEnvVarsOverrideConfig() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"
    preparePackages "$AGENT_VERSION" "."
    
    export HZN_EXCHANGE_URL="HZN_EXCHANGE_URL-TEST-ENV"
    export HZN_FSS_CSSURL="HZN_FSS_CSSURL-TEST-ENV"
    export HZN_ORG_ID="HZN_ORG_ID-TEST-ENV"
    export HZN_EXCHANGE_USER_AUTH="HZN_EXCHANGE_USER_AUTH-TEST-ENV"
    export HZN_EXCHANGE_PATTERN="HZN_EXCHANGE_PATTERN-TEST-ENV"

    CMD=$(timeout $AGENT_INSTALL_TIMEOUT .././agent-install.sh)
    CLEAN=${CMD//[$'\t\r\n']}
    assertContains "HZN_EXCHANGE_URL env variable hasn't overridden the HZN_EXCHANGE_URL config value" "${CLEAN}" "HZN_EXCHANGE_URL is ${HZN_EXCHANGE_URL}"
    assertContains "HZN_FSS_CSSURL env variable hasn't overridden the HZN_FSS_CSSURL config value" "${CLEAN}" "HZN_FSS_CSSURL is ${HZN_FSS_CSSURL}"
    assertContains "HZN_ORG_ID env variable hasn't overridden the HZN_ORG_ID config value" "${CLEAN}" "HZN_ORG_ID is ${HZN_ORG_ID}"
  
    # teardown
    removeCert "$CERT_DEFAULT"
    removeConfig "$CONFIG_DEFAULT"
    removeEdgePackages "linux"
    uninstallAgent
}

# 20. Command line arguments override env variables
testAgentCLArgsOverrideEnvVars() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"
    preparePackages "$AGENT_VERSION" "."

    export HZN_EXCHANGE_URL="HZN_EXCHANGE_URL-TEST-ENV"
    export HZN_FSS_CSSURL="HZN_FSS_CSSURL-TEST-ENV"
    export HZN_ORG_ID="HZN_ORG_ID-TEST-ENV"
    export HZN_EXCHANGE_USER_AUTH="HZN_EXCHANGE_USER_AUTH-TEST-ENV"
    export HZN_EXCHANGE_PATTERN="HZN_EXCHANGE_PATTERN-TEST-ENV"
    export CERTIFICATE="CERTIFICATE-TEST-ENV"
    CERTIFICATE_CL="CERT-TEST-CL"

    CMD=$(timeout $AGENT_INSTALL_TIMEOUT .././agent-install.sh -p "HZN_EXCHANGE_PATTERN-TEST-CL" -c "$CERTIFICATE_CL")
    CLEAN=${CMD//[$'\t\r\n']}
    echo "${CLEAN}"

    assertContains "HZN_EXCHANGE_PATTERN env variable hasn't overridden the HZN_EXCHANGE_PATTERN config value" "${CLEAN}" "CERTIFICATE is ${CERTIFICATE_CL}"

    # teardown
    unset HZN_EXCHANGE_URL
    unset HZN_FSS_CSSURL
    unset HZN_ORG_ID
    unset HZN_EXCHANGE_USER_AUTH
    unset HZN_EXCHANGE_PATTERN
    unset CERTIFICATE
    removeCert "$CERT_DEFAULT"
    removeConfig "$CONFIG_DEFAULT"
    removeEdgePackages "linux"
    uninstallAgent
}

# 21. Error checking with env variables & no config file
testAgentEnvVarsNoConfig() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"
    preparePackages "$AGENT_VERSION" "."
    
    mv "$CONFIG_DEFAULT" "${CONFIG_DEFAULT}-test"
    export HZN_EXCHANGE_URL="HZN_EXCHANGE_URL-TEST-ENV"
    export HZN_FSS_CSSURL="HZN_FSS_CSSURL-TEST-ENV"
    export HZN_ORG_ID="HZN_ORG_ID-TEST-ENV"
    export HZN_EXCHANGE_USER_AUTH="HZN_EXCHANGE_USER_AUTH-TEST-ENV"
    export HZN_EXCHANGE_PATTERN="HZN_EXCHANGE_PATTERN-TEST-ENV"

    unset HZN_EXCHANGE_URL
    CMD=$(.././agent-install.sh); ret=$?
    assertFalse "Script successfully executed" "${ret}"
    export HZN_EXCHANGE_URL="HZN_EXCHANGE_URL-TEST-ENV"
    
    unset HZN_FSS_CSSURL
    CMD=$(.././agent-install.sh); ret=$?
    assertFalse "Script successfully executed" "${ret}"
    export HZN_FSS_CSSURL="HZN_FSS_CSSURL-TEST-ENV"
    
    unset HZN_ORG_ID
    CMD=$(.././agent-install.sh); ret=$?
    assertFalse "Script successfully executed" "${ret}"
    export HZN_ORG_ID="HZN_ORG_ID-TEST-ENV"

    unset HZN_EXCHANGE_USER_AUTH
    CMD=$(.././agent-install.sh); ret=$?
    assertFalse "Script successfully executed" "${ret}"
    export HZN_EXCHANGE_USER_AUTH="HZN_EXCHANGE_USER_AUTH-TEST-ENV"

    # teardown
    unset HZN_EXCHANGE_URL
    unset HZN_FSS_CSSURL
    unset HZN_ORG_ID
    unset HZN_EXCHANGE_USER_AUTH
    unset HZN_EXCHANGE_PATTERN
    rm -f "${CONFIG_DEFAULT}-test"
    removeCert "$CERT_DEFAULT"
    removeEdgePackages "linux"
    uninstallAgent
}

# 22. All variables can be passed with env vars & no config file
testAgentAllVarsWithEnvNoConfig() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"
    preparePackages "$AGENT_VERSION" "."
    eval export $(cat "$CONFIG_DEFAULT")
    rm -f "$CONFIG_DEFAULT"

    CMD=$(timeout $AGENT_INSTALL_TIMEOUT .././agent-install.sh)
    CLEAN=${CMD//[$'\t\r\n']}

    echo "${CLEAN}"

    assertNotContains "The agent install script exit detected..." "${CLEAN}" "exiting"
    assertTrue "$MESSAGE_HZN_CLI_NOT_FOUND" "$HZN_CLI_INSTALLED"
    assertContains "Wrong expected node state" "$(queryNodeState)" "$NODE_STATE_CONFIGURED"
    assertNotNull "Workload is not executed" "$(waitForWorkloadUntilFinalized)"
    
    # teardown
    unset HZN_EXCHANGE_URL
    unset HZN_FSS_CSSURL
    unset HZN_ORG_ID
    unset HZN_EXCHANGE_USER_AUTH
    unset HZN_EXCHANGE_PATTERN
    removeCert "$CERT_DEFAULT"
    removeEdgePackages "linux"
    uninstallAgent
}

# 23. All variables can be passed with command line flags specifying files
testAgentAllVarsWithCli() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"
    preparePackages "$AGENT_VERSION" "."
   
    .././agent-install.sh -c "$CERT_DEFAULT" -k "$CONFIG_DEFAULT"

    assertTrue "$MESSAGE_HZN_CLI_NOT_FOUND" "$HZN_CLI_INSTALLED"
    assertContains "Wrong expected node state" "$(queryNodeState)" "$NODE_STATE_CONFIGURED"
    assertNotNull "Workload is not executed" "$(waitForWorkloadUntilFinalized)"

    # teardown
    removeCert "$CERT_DEFAULT"
    removeConfig "$CONFIG_DEFAULT"
    removeEdgePackages "linux"
    uninstallAgent
}

# 24. Agent install will change envs to the new environment
testAgentSwitchEnv() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"
    preparePackages "$AGENT_VERSION" "."

    CURRENT_ORG=$(grep "^HZN_ORG_ID=" "$CONFIG_DEFAULT" | cut -d'=' -f2)
    CURRENT_EXCHANGE=$(grep "^HZN_EXCHANGE_URL=" "$CONFIG_DEFAULT" | cut -d'=' -f2)
    CURRENT_MMS=$(grep "^HZN_FSS_CSSURL=" "$CONFIG_DEFAULT" | cut -d'=' -f2)

    echo "=========================================="
    echo "Current org: ${CURRENT_ORG}"
    echo "Current exch: ${CURRENT_EXCHANGE}"
    echo "Current mms: ${CURRENT_MMS}"
    echo "=========================================="

    .././agent-install.sh

    assertTrue "$MESSAGE_HZN_CLI_NOT_FOUND" "$HZN_CLI_INSTALLED"
    assertContains "Wrong expected node state" "$(queryNodeState)" "$NODE_STATE_CONFIGURED"
    assertNotNull "Workload is not executed" "$(waitForWorkloadUntilFinalized)"
    assertContains "Wrong Organization" "$(queryOrganization)" "$CURRENT_ORG"
    assertContains "Wrong Exchange URL" "$(queryExchangeURL)" "$CURRENT_EXCHANGE"
    assertContains "Wrong MMS URL" "$(queryMMSURL)" "$CURRENT_MMS"

    removeConfig "$CONFIG_DEFAULT"
    removeCert "$CERT_DEFAULT"
    prepareConfig "switch/${CONFIG_DEFAULT}"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "switch/${CERT_DEFAULT}"
    CURRENT_ORG=$(grep "^HZN_ORG_ID=" "$CONFIG_DEFAULT" | cut -d'=' -f2)
    CURRENT_EXCHANGE=$(grep "^HZN_EXCHANGE_URL=" "$CONFIG_DEFAULT" | cut -d'=' -f2)
    CURRENT_MMS=$(grep "^HZN_FSS_CSSURL=" "$CONFIG_DEFAULT" | cut -d'=' -f2)
    
    echo "=========================================="
    echo "Current org: ${CURRENT_ORG}"
    echo "Current exch: ${CURRENT_EXCHANGE}"
    echo "Current mms: ${CURRENT_MMS}"
    echo "=========================================="

    .././agent-install.sh << EOF
y
EOF

    assertTrue "$MESSAGE_HZN_CLI_NOT_FOUND" "$HZN_CLI_INSTALLED"
    assertContains "Wrong expected node state" "$(queryNodeState)" "$NODE_STATE_CONFIGURED"
    assertNotNull "Workload is not executed" "$(waitForWorkloadUntilFinalized)"
    assertContains "Wrong organization" "$(queryOrganization)" "$CURRENT_ORG"
    assertContains "Wrong Exchange URL" "$(queryExchangeURL)" "$CURRENT_EXCHANGE"
    assertContains "Wrong MMS URL" "$(queryMMSURL)" "$CURRENT_MMS"
    
    # teardown
    removeConfig "$CONFIG_DEFAULT"
    removeCert "$CERT_DEFAULT"
    removeEdgePackages "linux"
    uninstallAgent
}

setUp() {
    TEST_RUNNING=$(( TEST_RUNNING + 1 ))
    echo "setUp()"
    echo "************************************"
    echo "starting test ${TEST_RUNNING}...."
    startCount
}

tearDown() {
    echo "tearDown()"

    echo "finished test ${TEST_RUNNING}..."
    echo "************************************"
    stopCount
}

oneTimeSetUp() {
    echo "onetimeSetup()"

    # get packages for a current version
    getEdgePackages "${AGENT_VERSION}" "${AGENT_VERSION}"
    # get packages for an older version
    getEdgePackages "${PREV_AGENT_VERSION}" "${PREV_AGENT_VERSION}"

}

oneTimeTearDown() {
    echo "oneTimeTearDown()"

    # delete downloaded packages
    removeEdgePackages "${PREV_AGENT_VERSION}"
    removeEdgePackages "${AGENT_VERSION}"
    # removing dependencies
    cleanupDependencies
    END=$(date +%s)
    ELAPSED=$(( END - START ))
    echo "Script ran for $(( ${ELAPSED} / 3600 ))h $(( (${ELAPSED} / 60) % 60 ))m $(( ${ELAPSED} % 60 ))s"
}

shift $#
. "${SHUNIT_PATH}/shunit2/shunit2"
