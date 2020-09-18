#!/bin/bash

# Common assertions

MESSAGE_HZN_CLI_NOT_FOUND="hzn CLI command not found"
MESSAGE_HZN_CLI_FOUND="hzn CLI command found"

HZN_CLI_INSTALLED="command -v hzn >/dev/null 2>&1;"

DOCKER_INSTALLED="command -v docker >/dev/null 2>&1;"
JQ_INSTALLED="command -v jq >/dev/null 2>&1;"
CURL_INSTALLED="command -v curl >/dev/null 2>&1;"

MESSAGE_DOCKER_NOT_FOUND="docker not found"
MESSAGE_JQ_NOT_FOUND="jq not found"
MESSAGE_CURL_NOT_FOUND="curl not found"

WORKLOAD_EXECUTION_TIMEOUT=60

isCLIInstalled() {
    if command -v hzn >/dev/null 2>&1; then
        return 0
    else
        return 1
    fi

}

queryNodeState(){
    hzn node list | jq -r .configstate.state
}

queryOrganization(){
    hzn node list | jq -r .organization
}

queryExchangeURL(){
    hzn node list | jq -r .configuration.exchange_api
}

queryMMSURL(){
    hzn node list | jq -r .configuration.mms_api
}

waitForWorkloadUntilFinalized(){

    if command -v hzn >/dev/null 2>&1; then
        check_start=`date +%s`

        while [ -z "$(hzn agreement list | jq -r .[].agreement_execution_start_time)" ] ; do
        check_current=`date +%s`
            if (( check_current - check_start > WORKLOAD_EXECUTION_TIMEOUT )); then
                return 1
            fi
            sleep 5
        done

        hzn agreement list | jq -r .[].agreement_execution_start_time
    fi

}

# gets agent version
getAgentVersion() {
    hzn version | grep "^Horizon Agent" | sed 's/^.*: //' | cut -d'-' -f1
}

findCliAutocomplete() {
    find / -name "hzn_bash_autocomplete.sh" | wc -l
}

checkCliAutocomplete() {
    if grep -q "^source $(find / -name "hzn_bash_autocomplete.sh")" ~/.${SHELL##*/}rc ; then
        echo 0
    else
        echo 1
    fi
}
