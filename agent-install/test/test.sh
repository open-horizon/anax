#!/bin/bash

START=$(date +%s)
# common functions
. ./common.sh
# common assertions
. ./assertions.sh

TEST_RUNNING=0

AGENT_VERSION="$1"
PREV_AGENT_VERSION="$2"

# 1. Install w/ registrations skipped
testAgentInstallWoRegistration() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    removeVariableFromConfig "HZN_EXCHANGE_PATTERN" "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"
    
    .././agent-install.sh -s -i $AGENT_VERSION

    assertTrue "$MESSAGE_HZN_CLI_NOT_FOUND" "$HZN_CLI_INSTALLED"
    assertContains "Wrong expected node state" "$(queryNodeState)" "$NODE_STATE_UNCONFIGURED"

    # teardown
    removeConfig "$CONFIG_DEFAULT"
    removeCert "$CERT_DEFAULT"
    uninstallAgent
}

# 2. Install w/ registration but no pattern/policy specified
testAgentInstallWRegistration() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    removeVariableFromConfig "HZN_EXCHANGE_PATTERN" "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"

    .././agent-install.sh -i $AGENT_VERSION

    assertTrue "$MESSAGE_HZN_CLI_NOT_FOUND" "$HZN_CLI_INSTALLED"
    assertContains "Wrong expected node state" "$(queryNodeState)" "$NODE_STATE_CONFIGURED"

    # teardown
    removeConfig "$CONFIG_DEFAULT"
    removeCert "$CERT_DEFAULT"
    uninstallAgent
}

# 3. Install w/ pattern specified and no pode policy
testAgentInstallWPattern() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"

    .././agent-install.sh -i $AGENT_VERSION

    assertTrue "$MESSAGE_HZN_CLI_NOT_FOUND" "$HZN_CLI_INSTALLED"
    assertContains "Wrong expected node state" "$(queryNodeState)" "$NODE_STATE_CONFIGURED"
    assertNotNull "Workload is not executed" "$(waitForWorkloadUntilFinalized)"

    # teardown
    removeConfig "$CONFIG_DEFAULT"
    removeCert "$CERT_DEFAULT"
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

    .././agent-install.sh -n "$NODE_POLICY" -i "$AGENT_VERSION"

    assertTrue "$MESSAGE_HZN_CLI_NOT_FOUND" "$HZN_CLI_INSTALLED"
    assertContains "Wrong expected node state" "$(queryNodeState)" "$NODE_STATE_CONFIGURED"

    # teardown
    rm -f ${NODE_POLICY}
    removeConfig "$CONFIG_DEFAULT"
    removeCert "$CERT_DEFAULT"
    uninstallAgent
}

# 5. Install w/ both pattern and policy specified for the node
testAgentInstallWNodePolicyAndPattern() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    cp config/${NODE_POLICY} .
    prepareCert "$CERT_DEFAULT"

    CMD=$(.././agent-install.sh -n "$NODE_POLICY" -p "$PATTERN" -i $AGENT_VERSION); ret=$?
    
    assertFalse "Script successfully executed" "${ret}"
    
    # teardown
    rm -f ${NODE_POLICY}
    removeConfig "$CONFIG_DEFAULT"
    removeCert "$CERT_DEFAULT"
    uninstallAgent
}

# 6. Older version installed already (node registered), installing newer version.
testAgentInstalledOlderVersionInstallNewer() {
    if [ -z $PREV_AGENT_VERSION ]; then
        echo "Skipping agent-install upgrade test since no previous version provided."
    else
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"
    # installing previous version
    #preparePackages "$PREV_AGENT_VERSION" "."
    # install an older version
    .././agent-install.sh -i $PREV_AGENT_VERSION

    assertTrue "$MESSAGE_HZN_CLI_NOT_FOUND" "$HZN_CLI_INSTALLED"
    assertContains "Wrong expected node state" "$(queryNodeState)" "$NODE_STATE_CONFIGURED"
    assertEquals "Expected same agent version" "$PREV_AGENT_VERSION" "$(getAgentVersion)" 
    
    #removeEdgePackages "linux"
    # installing previous version
    #preparePackages "$AGENT_VERSION" "."

    echo "y" | .././agent-install.sh -i $AGENT_VERSION

    assertTrue "$MESSAGE_HZN_CLI_NOT_FOUND" "$HZN_CLI_INSTALLED"
    assertContains "Wrong expected node state" "$(queryNodeState)" "$NODE_STATE_CONFIGURED"
    assertEquals "Expected different agent version" "$PREV_AGENT_VERSION" "$(getAgentVersion)" 

    # clean up
    removeConfig "$CONFIG_DEFAULT"
    removeCert "$CERT_DEFAULT"
    #removeEdgePackages "linux"
    uninstallAgent
    fi
}

# 7. Validate any missing dependency packages are automatically installed
testAgentDependecies(){
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"

    removeAgentDependencies

    # install agent
    CMD=$(.././agent-install.sh -i $AGENT_VERSION); ret=$?
    
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
    uninstallAgent
}

# 8. Check that CLI autocomplete works for hzn CLI
testAgentCliAutocomplete() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"

    .././agent-install.sh -i $AGENT_VERSION

    assertTrue "$MESSAGE_HZN_CLI_NOT_FOUND" "$HZN_CLI_INSTALLED"
    assertContains "Wrong expected node state" "$(queryNodeState)" "$NODE_STATE_CONFIGURED"
    assertNotNull "Workload is not executed" "$(waitForWorkloadUntilFinalized)"
    assertNotEquals "Autocomplete file not found" 0 "$(findCliAutocomplete)"
    assertEquals "Autocomplete record not found" 0 "$(checkCliAutocomplete)"

    # teardown
    removeConfig "$CONFIG_DEFAULT"
    removeCert "$CERT_DEFAULT"
    uninstallAgent

}

# 9. Missing config file produces an error
testAgentMissingConfig() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"
    mv "$CONFIG_DEFAULT" "${CONFIG_DEFAULT}-error.cfg"

    CMD=$(.././agent-install.sh -i $AGENT_VERSION); ret=$?
    
    assertFalse "Script successfully executed" "${ret}"

    # teardown
    rm -f "${CONFIG_DEFAULT}-error.cfg"
    removeCert "$CERT_DEFAULT"
    uninstallAgent

}

# 10. Missing cert file produces an error
testAgentMissingCert() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"

    mv "$CERT_DEFAULT" "${CERT_DEFAULT}-error.crt"

    CMD=$(.././agent-install.sh -i $AGENT_VERSION); ret=$?
    
    assertFalse "Script successfully executed" "${ret}"

    # teardown
    mv -f "${CERT_DEFAULT}-error.crt" "$CERT_DEFAULT"
    removeCert "$CERT_DEFAULT"
    removeConfig "$CONFIG_DEFAULT"
    uninstallAgent
}

# 11. Package path option specified (use non-default path)
testAgentPackagePath() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"
    getOS
    getDistro
    getArch
    mkdir -p "custom-path"
    cp $AGENT_VERSION/* custom-path

    .././agent-install.sh -i "custom-path"

    assertTrue "$MESSAGE_HZN_CLI_NOT_FOUND" "$HZN_CLI_INSTALLED"
    assertContains "Wrong expected node state" "$(queryNodeState)" "$NODE_STATE_CONFIGURED"
    assertNotNull "Workload is not executed" "$(waitForWorkloadUntilFinalized)"

    # teardown
    removeCert "$CERT_DEFAULT"
    removeConfig "$CONFIG_DEFAULT"
    removeEdgePackages "custom-path"
    uninstallAgent
}

# 12. Missing default package path throws an error
testAgentMissingDefaultPath() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"

    CMD=$(.././agent-install.sh); ret=$?
    
    assertFalse "Script successfully executed" "${ret}"

    # teardown
    removeCert "$CERT_DEFAULT"
    removeConfig "$CONFIG_DEFAULT"
    uninstallAgent
}

# 13. Custom config file option specified
testAgentCustomConfig() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"
    mv "$CONFIG_DEFAULT" "${CONFIG_DEFAULT}-custom.cfg"

    CMD=$(.././agent-install.sh -k ${CONFIG_DEFAULT}-custom.cfg -i $AGENT_VERSION)

    assertTrue "$MESSAGE_HZN_CLI_NOT_FOUND" "$HZN_CLI_INSTALLED"
    assertContains "Wrong expected node state" "$(queryNodeState)" "$NODE_STATE_CONFIGURED"
    assertNotNull "Workload is not executed" "$(waitForWorkloadUntilFinalized)"

    # teardown
    rm -f "${CONFIG_DEFAULT}-custom.cfg"
    removeCert "$CERT_DEFAULT"
    uninstallAgent
}

# 14. Custom certificate file option specifed
testAgentCustomCert() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"
    mv "$CERT_DEFAULT" "${CERT_DEFAULT}-custom.crt"

    CMD=$(.././agent-install.sh -c ${CERT_DEFAULT}-custom.crt -i $AGENT_VERSION)

    assertTrue "$MESSAGE_HZN_CLI_NOT_FOUND" "$HZN_CLI_INSTALLED"
    assertContains "Wrong expected node state" "$(queryNodeState)" "$NODE_STATE_CONFIGURED"
    assertNotNull "Workload is not executed" "$(waitForWorkloadUntilFinalized)"

    # teardown
    mv "${CERT_DEFAULT}-custom.crt" "$CERT_DEFAULT"
    removeCert "$CERT_DEFAULT"
    removeConfig "$CONFIG_DEFAULT"
    uninstallAgent
}

# 15. Any missed required params in config file generate an error
testAgentMissedReqParams() {
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"
    # setup

    for VAR in HZN_EXCHANGE_URL HZN_FSS_CSSURL HZN_ORG_ID HZN_EXCHANGE_USER_AUTH;
    do
        prepareConfig "$CONFIG_DEFAULT"
        removeVariableFromConfig "$VAR" "$CONFIG_DEFAULT"
        CMD=$(.././agent-install.sh -i $AGENT_VERSION); ret=$?
        assertFalse "Script successfully executed" "${ret}"
    done;

    # teardown
    removeCert "$CERT_DEFAULT"
    removeConfig "$CONFIG_DEFAULT"
    uninstallAgent
}

# 16. Specify an invalid logging level
testAgentWrongLoggingLevel(){
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"

    CMD=$(.././agent-install.sh -l 999 -i $AGENT_VERSION); ret=$?
    assertFalse "Script successfully executed" "${ret}"

    # teardown
    removeCert "$CERT_DEFAULT"
    removeConfig "$CONFIG_DEFAULT"
    uninstallAgent
}

# 17. Validate environment variables override the config params
testAgentEnvVarsOverrideConfig() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"
    
    export HZN_EXCHANGE_URL="HZN_EXCHANGE_URL-TEST-ENV"
    export HZN_FSS_CSSURL="HZN_FSS_CSSURL-TEST-ENV"
    export HZN_ORG_ID="HZN_ORG_ID-TEST-ENV"
    export HZN_EXCHANGE_USER_AUTH="HZN_EXCHANGE_USER_AUTH-TEST-ENV:test"
    export HZN_EXCHANGE_PATTERN="HZN_EXCHANGE_PATTERN-TEST-ENV"

    CMD=$(.././agent-install.sh -b -i $AGENT_VERSION)
    CLEAN=${CMD//[$'\t\r\n']}
    assertContains "HZN_EXCHANGE_URL env variable hasn't overridden the HZN_EXCHANGE_URL config value" "${CLEAN}" "HZN_EXCHANGE_URL: ${HZN_EXCHANGE_URL}"
    assertContains "HZN_FSS_CSSURL env variable hasn't overridden the HZN_FSS_CSSURL config value" "${CLEAN}" "HZN_FSS_CSSURL: ${HZN_FSS_CSSURL}"
    assertContains "HZN_ORG_ID env variable hasn't overridden the HZN_ORG_ID config value" "${CLEAN}" "HZN_ORG_ID: ${HZN_ORG_ID}"
  
    # teardown
    removeCert "$CERT_DEFAULT"
    removeConfig "$CONFIG_DEFAULT"
    uninstallAgent
}

# 18. Command line arguments override env variables
testAgentCLArgsOverrideEnvVars() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"

    export HZN_EXCHANGE_URL="HZN_EXCHANGE_URL-TEST-ENV"
    export HZN_FSS_CSSURL="HZN_FSS_CSSURL-TEST-ENV"
    export HZN_ORG_ID="HZN_ORG_ID-TEST-ENV"
    export HZN_EXCHANGE_USER_AUTH="HZN_EXCHANGE_USER_AUTH-TEST-ENV:test"
    export HZN_EXCHANGE_PATTERN="HZN_EXCHANGE_PATTERN-TEST-ENV"
    export CERTIFICATE="CERTIFICATE-TEST-ENV"
    CERTIFICATE_CL="CERT-TEST-CL"

    CMD=$(.././agent-install.sh -b -p "HZN_EXCHANGE_PATTERN-TEST-CL" -c "$CERTIFICATE_CL" -i $AGENT_VERSION)
    CLEAN=${CMD//[$'\t\r\n']}

    assertContains "AGENT_CERT_FILE Hasn't overridden the default value of agent-install.crt" "${CLEAN}" "AGENT_CERT_FILE: ${CERTIFICATE_CL}"

    # teardown
    unset HZN_EXCHANGE_URL
    unset HZN_FSS_CSSURL
    unset HZN_ORG_ID
    unset HZN_EXCHANGE_USER_AUTH
    unset HZN_EXCHANGE_PATTERN
    unset CERTIFICATE
    removeCert "$CERT_DEFAULT"
    removeConfig "$CONFIG_DEFAULT"
    uninstallAgent
}

# 19. Error checking with env variables & no config file
testAgentEnvVarsNoConfig() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"
    
    mv "$CONFIG_DEFAULT" "${CONFIG_DEFAULT}-test"
    export HZN_EXCHANGE_URL="HZN_EXCHANGE_URL-TEST-ENV"
    export HZN_FSS_CSSURL="HZN_FSS_CSSURL-TEST-ENV"
    export HZN_ORG_ID="HZN_ORG_ID-TEST-ENV"
    export HZN_EXCHANGE_USER_AUTH="HZN_EXCHANGE_USER_AUTH-TEST-ENV"
    export HZN_EXCHANGE_PATTERN="HZN_EXCHANGE_PATTERN-TEST-ENV"

    unset HZN_EXCHANGE_URL
    CMD=$(.././agent-install.sh -i $AGENT_VERSION); ret=$?
    assertFalse "Script successfully executed" "${ret}"
    export HZN_EXCHANGE_URL="HZN_EXCHANGE_URL-TEST-ENV"
    
    unset HZN_FSS_CSSURL
    CMD=$(.././agent-install.sh -i $AGENT_VERSION); ret=$?
    assertFalse "Script successfully executed" "${ret}"
    export HZN_FSS_CSSURL="HZN_FSS_CSSURL-TEST-ENV"
    
    unset HZN_ORG_ID
    CMD=$(.././agent-install.sh -i $AGENT_VERSION); ret=$?
    assertFalse "Script successfully executed" "${ret}"
    export HZN_ORG_ID="HZN_ORG_ID-TEST-ENV"

    unset HZN_EXCHANGE_USER_AUTH
    CMD=$(.././agent-install.sh -i $AGENT_VERSION); ret=$?
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
    uninstallAgent
}

# 20. All variables can be passed with env vars & no config file
testAgentAllVarsWithEnvNoConfig() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"
    echo " config file is: ($cat $CONFIG_DEFAULT)"
    eval export $(cat "$CONFIG_DEFAULT")
    rm -f "$CONFIG_DEFAULT"
    echo "Config Params are: ExchURL($HZN_EXCHANGE_URL) ORG ID ($HZN_ORG_ID) USER_AUTH ($HZN_EXCHANGE_USER_AUTH) EXCHANGE_PATTERN ($HZN_EXCHANGE_PATTERN) CSS ($HZN_FSS_CSSURL)"

    CMD=$(.././agent-install.sh -i $AGENT_VERSION)
    CLEAN=${CMD//[$'\t\r\n']}

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
    uninstallAgent
}

# 21. All variables can be passed with command line flags specifying files
testAgentAllVarsWithCli() {
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"
   
    .././agent-install.sh -c "$CERT_DEFAULT" -k "$CONFIG_DEFAULT" -i "$AGENT_VERSION"

    assertTrue "$MESSAGE_HZN_CLI_NOT_FOUND" "$HZN_CLI_INSTALLED"
    assertContains "Wrong expected node state" "$(queryNodeState)" "$NODE_STATE_CONFIGURED"
    assertNotNull "Workload is not executed" "$(waitForWorkloadUntilFinalized)"

    # teardown
    removeCert "$CERT_DEFAULT"
    removeConfig "$CONFIG_DEFAULT"
    uninstallAgent
}

# 22. Agent install will change envs to the new environment
testAgentSwitchEnv() {
    if ! checkSecondaryConfig ; then
    echo "Test AgentSwitchEnv skipped due to missing /config/switch files"
    else
    # setup
    prepareConfig "$CONFIG_DEFAULT"
    showConfig "$CONFIG_DEFAULT"
    prepareCert "$CERT_DEFAULT"

    CURRENT_ORG=$(grep "^HZN_ORG_ID=" "$CONFIG_DEFAULT" | cut -d'=' -f2)
    CURRENT_EXCHANGE=$(grep "^HZN_EXCHANGE_URL=" "$CONFIG_DEFAULT" | cut -d'=' -f2)
    CURRENT_MMS=$(grep "^HZN_FSS_CSSURL=" "$CONFIG_DEFAULT" | cut -d'=' -f2)

    echo "=========================================="
    echo "Current org: ${CURRENT_ORG}"
    echo "Current exch: ${CURRENT_EXCHANGE}"
    echo "Current mms: ${CURRENT_MMS}"
    echo "=========================================="

    .././agent-install.sh -i $AGENT_VERSION

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

    .././agent-install.sh -i $AGENT_VERSION << EOF
y
EOF

    assertTrue "$MESSAGE_HZN_CLI_NOT_FOUND" "$HZN_CLI_INSTALLED"
    assertContains "Wrong expected node state" "$(queryNodeState)" "$NODE_STATE_CONFIGURED"
    assertContains "Wrong organization" "$(queryOrganization)" "$CURRENT_ORG"
    assertContains "Wrong Exchange URL" "$(queryExchangeURL)" "$CURRENT_EXCHANGE"
    assertContains "Wrong MMS URL" "$(queryMMSURL)" "$CURRENT_MMS"
    
    # teardown
    removeConfig "$CONFIG_DEFAULT"
    removeCert "$CERT_DEFAULT"
    uninstallAgent
    fi
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
    if [ $LOCAL_PACKAGE_DIR ]; then
        AGENT_VERSION=$LOCAL_PACKAGE_DIR
        PREV_AGENT_VERSION=
        echo "Using local package directory '$LOCAL_PACKAGE_DIR', you must have the correct horizon and horizon-cli packages in this directory for the appropriate architecture."
    elif [ -z $LOCAL_PACKAGE_DIR ] && [ -z $AGENT_VERSION ]; then
    echo "No version specified and no local package specified with -i. Specify one of these two parameters to start the test."
    exit 1
    elif [ -z $PREV_AGENT_VERSION ]; then
    getEdgePackages "${AGENT_VERSION}" "${AGENT_VERSION}"
    else
    # get packages for a current version
    getEdgePackages "${AGENT_VERSION}" "${AGENT_VERSION}"
    # get packages for an older version
    getEdgePackages "${PREV_AGENT_VERSION}" "${PREV_AGENT_VERSION}"
    fi
}

oneTimeTearDown() {
    echo "oneTimeTearDown()"
    if [ -z $LOCAL_PACKAGE_DIR ]; then
        # delete downloaded packages
        removeEdgePackages "${PREV_AGENT_VERSION}"
        removeEdgePackages "${AGENT_VERSION}"
    fi
    # removing dependencies
    cleanupDependencies
    END=$(date +%s)
    ELAPSED=$(( END - START ))
    echo "Script ran for $(( ${ELAPSED} / 3600 ))h $(( (${ELAPSED} / 60) % 60 ))m $(( ${ELAPSED} % 60 ))s"
}

function help() {
     cat << EndOfMessage
$(basename "$0") <options> -- testing agent-install.sh

Input Parameters:
    Param 1: Package Version: (required) This is required if option -i is not specified. Example usage for internal repo (artifactory): '2.27.0-95'
    Param 2: Downlevel Package Version: (optional) If not specified test 6 will be skipped and 'agent-install.sh' ability to upgrade agent versions will not be tested.

Parameters:
    -i    local installation packages location (if not specified, you must specify a package version above). This will only work with local directories.
Example: ./$(basename "$0") -i <pacakge_dir_name>

EndOfMessage
}

while getopts "i:h" opt; do
	case $opt in
		i) LOCAL_PACKAGE_DIR="$OPTARG"
        ;;
		h) help; exit 0
		;;
        \?) echo "Invalid option: -$OPTARG"; help; exit 1
		;;
		:) echo "Option -$OPTARG requires an argument"; help; exit 1
		;;
	esac
done
if [ -z ${1+x} ] && [ -z $LOCAL_PACKAGE_DIR ] ; then echo "A package version to download or a local package is required to be specified, exiting..."; exit 1; fi
if [ -z $ARTIFACTORY_APIKEY ] || [ -z $ARTIFACTORY_EMAIL ] && [ -z $LOCAL_PACKAGE_DIR ] ; then echo "Both variables ARTIFACTORY_APIKEY and ARTIFACTORY_EMAIL must be set to run, exiting..."; exit 1; fi

checkConfig
prepareDependencies

shift $#
. "${SHUNIT_PATH}/shunit2/shunit2" | tee agent-install-test-results.log
