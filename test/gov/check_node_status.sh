#!/bin/bash

# Check node status. The inputs are:
# $1 - check for non-empty running services
# $2 - expected number of running services
# $3 - the node id to check (an12345 by default)
# $4 - the node's orgId (could be set also by ORG_ID, userdev by default)
# $5 - user creds in org/user:userpw format (could be set also by ADMIN_AUTH, userdev admin by default)
# $6 - serviceUrl to check
function checkNodeStatus {
    local nonEmptyRunningSvcCheckEnabled="$1"
    local nodeId="${3:-an12345}"
    local svcUrl="$6"

    local org="${4:-userdev}"
    if [ "$4" == "" ] && [ "$ORG_ID" != "" ]; then
      org="$ORG_ID"
    fi

    local creds="${5:-userdev/userdevadmin:userdevadminpw}"
    if [ "$5" == "" ] && [ "$ADMIN_AUTH" != "" ]; then
      creds="$org/$ADMIN_AUTH"
    fi

    echo -e "Checking node status in the exchange..."

    NST=$(curl -sSL $CERT_VAR --header 'Accept: application/json' -u ${creds} "${EXCH_APP_HOST}/orgs/${org}/nodes/${nodeId}/status" | jq -r '.')

    # check if we got expected response
    respContains=$(echo $NST | grep "services")
    if [ "${respContains}" == "" ]; then
        echo -e "\nERROR: Unexpected node status response:"
        echo -e "$NST"
        return 1
    fi

    if [ $nonEmptyRunningSvcCheckEnabled == "true" ]; then
        # check if there is any renning services
        runningService=$(echo $NST | jq -r ".runningServices")
        if [ "${runningService}" == "" ] || [ "${runningService}" == "|" ]; then
            echo -e "\nERROR: No services are running on the node"
            return 1
        fi
    fi

    if [ "${svcUrl}" != "" ]; then
        # check if we got expected service running on node
        runningService=$(echo $NST | jq -r ".runningServices" | grep $svcUrl)
        if [ "${runningService}" == "" ]; then
            echo -e "\nERROR: Expected service '${svcUrl}' is not running on the node"
            return 1
        fi
    fi

    local runningCount=0
    local agrCount=0
    local svcCount=0
    while IFS=$"\n" read -r c; do
        ((svcCount++))
        state=$(echo "$c" | jq -r '.containerStatus[0].state')
        if [ "$state" == "running" ] ;then
            ((runningCount++))
        fi

        agreementId=$(echo "$c" | jq -r '.agreementId')
        if [ "$agreementId" != "" ] ;then
            ((agrCount++))
        fi
    done < <(echo "$NST" | jq -c '.services[]')

    if [ "${2}" != "" ]; then
        if [ $svcCount != $2 ]; then
            echo -e "\nERROR: Expected ${2} running services, but got ${svcCount}"
            return 1
        fi
    fi

    echo "Found ${runningCount} running services"

    return 0
}