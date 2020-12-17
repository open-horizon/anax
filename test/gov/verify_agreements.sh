#!/bin/bash

# include node status verification function
source ./check_node_status.sh

PREFIX="Verifying agreements:"

echo -e "${PREFIX} starting"

# This function returns a json array of agreements that have been formed, or an error code.
# MONITOR_AGS holds the agreement list.
function verifyAgreements {
        # Wait until there are agreements
        TARGET_NUM_AG=1
        if [ "${HZN_REG_TEST}" != "1" ]; then
            if [ "${PATTERN}" == "sall" ]; then
                TARGET_NUM_AG=6
            elif [ "${PATTERN}" == "" ]; then
                TARGET_NUM_AG=5
            fi
        fi

        if [ "${EXCH_APP_HOST}" = "http://exchange-api:8080/v1" ]; then
          TIMEOUT_MUL=1
        else
          TIMEOUT_MUL=3
        fi

        # Look for agreements to appear.
        AG_LOOP_CNT=0
        while [ $AG_LOOP_CNT -le  $(expr $(expr 48 + 12 \* $TARGET_NUM_AG) \* $TIMEOUT_MUL) ]
        do
                echo -e "${PREFIX} waiting for ${TARGET_NUM_AG} agreement(s)"

                AGS=$(curl -sSL $ANAX_API/agreement | jq -r '.agreements.active')
                NUM_AGS=$(echo ${AGS} | jq -r '. | length')
                if [ "${TARGET_NUM_AG}" == "${NUM_AGS}" ]; then
                        # Make an array of agreement ids that we should be tracking
                        MONITOR_AGS=$(echo ${AGS} | jq -r '[.[].current_agreement_id]')
                        echo -e "${PREFIX} found ${TARGET_NUM_AG} agreement(s): ${MONITOR_AGS}"
                        return 0
                fi
                let AG_LOOP_CNT+=1
                sleep 5
        done

        echo -e "Timed out, ${NUM_AGS} agreements formed but we were looking for ${TARGET_NUM_AG}"
        exit 1
}

# Check if an agreement still exists. Returns non-zero if it doesnt.
# $1 - all current agreements
# $2 - the agreement id to check.
#
function agreementExists {
        STILL_EXIST=$(echo $1 | jq -r '.[] | select (.current_agreement_id == "'$2'")')
        if [ "${STILL_EXIST}" == "" ]; then
                echo -e "${PREFIX} agreement $2 no longer exists"
                AAGS=$(curl -sSL $ANAX_API/agreement | jq -r '.agreements.archived')
                echo -e "${PREFIX} agreement archive contains: ${AAGS}"
                exit 1
        fi
}


# Wait until the agreement(s) get to a specific lifecycle state, and make sure the agreements dont change during this time.
# $1 - the timestamped field name that should be non-zero
#
function agreementsReached {
        NUM_AGS=$(echo ${MONITOR_AGS} | jq -r '. | length')
        while :
        do
                echo -e "${PREFIX} waiting for agreement(s) to have $1 set"

                AGS=$(curl -sSL $ANAX_API/agreement | jq -r '.agreements.active')
                NOT_YET=0
                for (( ix=0; ix<$NUM_AGS; ix++ ))
                do
                        AG=$(echo ${MONITOR_AGS} | jq -r '.['${ix}']')
                        agreementExists "${AGS}" "${AG}"

                        STATE_TS=$(echo ${AGS} | jq -r '.[] | select (.current_agreement_id == "'${AG}'") | .'$1'')
                        # STATE_TS=$(echo ${THIS_AG} | jq -r '.$1')

                        if [ "${STATE_TS}" == "0" ]; then
                                NOT_YET=1
                                break
                        else
                                echo -e "${PREFIX} agreement ${AG} executing"
                        fi
                done
                if [ "${NOT_YET}" == "1" ]; then
                        sleep 10
                else
                        return 0
                fi
        done
}

# Keep an eye on the agreements to make sure they dont go away. This function
# assumes that MONITOR_AGS has been previously set.
function monitorAgreements {
    NUM_AGS=$(echo ${MONITOR_AGS} | jq -r '. | length')
	NOT_YET=0
        while :
        do
                AGS=$(curl -sSL $ANAX_API/agreement | jq -r '.agreements.active')
                for (( ix=0; ix<$NUM_AGS; ix++ ))
                do
                        AG=$(echo ${MONITOR_AGS} | jq -r '.['${ix}']')
                        agreementExists "${AGS}" "${AG}"
                done
                echo -e "${PREFIX} agreements still present"

                if [ "${NOLOOP}" == "1" ]; then
                        if [ "${NOT_YET}" == "1" ]; then
                                return 0
                        else
                                NOT_YET=1
                                echo -e "Sleeping for 30s giving agreements time to fail before checking again"
                                sleep 30
                        fi
                else
                        sleep 120
                fi
        done
}

# Do the location specific verification. This function has specific knowledge of the
# way in which the location service (agreement service) should be running.
# $1 - the GET /service object for the location service
#
function handleLocation {

        REFURL=$(echo $1 | jq -r '.ref_url')

        # Make sure we dont get called twice
        if [ "${LOC_CPU_NETNAME}" != "" ]; then
                echo -e "${PREFIX} ${REFURL} should have 1 service, but encountered another"
                exit 2
        fi

        # Validate that it has 1 container and should have 3 networks.
        CONT_NUM=$(echo $1 | jq -r '.containers | length')
        if [ "${CONT_NUM}" != "1" ]; then
                echo -e "${PREFIX} ${REFURL} should have 1 container, but there are ${CONT_NUM}"
                exit 2
        fi

        # Grab the map of networks. There should be 3, each is a key in the map.
        NETS=$(echo $1 | jq -r '.containers[0].NetworkSettings.Networks')
        NUM_NETS=$(echo ${NETS} | jq -r '. | length')
        if [ "${NUM_NETS}" != "3" ]; then
                echo -e "${PREFIX} ${REFURL} should have 3 networks, but there are ${NUM_NETS}"
                exit 2
        fi

        # Grab the network name keys as a json array so we can iterate them. One of the networks should be
        # the same as a known agreement id. The other 2 are; the GPS container that is specific to the location
        # service and the cpu container which is just there to create a third level of dependency.
        NET_KEYS=$(echo ${NETS} | jq -r '. | keys')

        # The network topology is complex. The agreement service depends on an agreement-less service (locgps), which itself
        # has a dependency (cpu) that is the same dependency that the agreement service has.
        if [ "$(echo ${NET_KEYS} | jq -r 'contains(["services-locgps"])')" == "false" ]; then
                echo -e "${PREFIX} the location service is not in the dependent network for locgps, location has: ${NET_KEYS}"
                exit 2
        elif [ "$(echo ${NET_KEYS} | jq -r 'contains(["service-cpu"])')" == "false" ]; then
                echo -e "${PREFIX} the location service is not in the dependent network for cpu, location has: ${NET_KEYS}"
                exit 2
        fi

        # Grab the network name of the cpu service so that we can check it against the network of the cpu
        # service to make sure they match.
        for (( lix=0; lix<$NUM_NETS; lix++ ))
        do
                NET_NAME=$(echo ${NET_KEYS} | jq -r '.['$lix']')
                if [[ ${NET_NAME} = *"services-cpu"* ]]; then
                        LOC_CPU_NETNAME="${NET_NAME}"
                        break
                fi
        done

        # CPU_NET_NAME might not be filled in yet. We will do this same check when we verify the CPU service.
        if [ "${CPU_NET_NAME}" != "" ] && [ "${CPU_NET_NAME}" != "${LOC_CPU_NETNAME}" ]; then
                echo -e "${PREFIX} location's cpu service network is different from the network of the CPU service itself"
                exit 2
        elif [ "${CPU_NET_NAME}" != "" ] && [ "${CPU_NET_NAME}" == "${LOC_CPU_NETNAME}" ]; then
                echo -e "${PREFIX} location's cpu service network is the same as the network of the CPU service itself"
        fi
}


# Do the cpu specific verification. This function has specific knowledge of the
# way in which the cpu service (dependent service) should be running.
# $1 - the GET /service object for the cpu service
#
function handleCPU {

        REFURL=$(echo $1 | jq -r '.ref_url')

        # Validate that it has 1 container.
        CONT_NUM=$(echo $1 | jq -r '.containers | length')
        if [ "${CONT_NUM}" != "1" ]; then
                echo -e "${PREFIX} ${REFURL} should have 1 container, but there are ${CONT_NUM}"
                exit 2
        fi

        # Grab the map of networks. There should be 1.
        NETS=$(echo $1 | jq -r '.containers[0].NetworkSettings.Networks')
        NUM_NETS=$(echo ${NETS} | jq -r '. | length')
        if [ "${NUM_NETS}" != "1" ]; then
                echo -e "${PREFIX} ${REFURL} should have 1 network, but there are ${NUM_NETS}"
                exit 2
        fi

        # Grab the network name keys as a json array so we can iterate them. There sould be only 1.
        NET_KEYS=$(echo ${NETS} | jq -r '. | keys')

        # There is another cpu service which is from e2edev@somecomp.com2edev org. CPU_NET_NAME should be the one from IBM org.
        netname=$(echo ${NET_KEYS} | jq -r '.[0]')
        echo $netname | grep IBM
        if [ $? -eq 0 ]; then
            CPU_NET_NAME=$netname
        fi

        # LOC_CPU_NETNAME might not be filled in yet. We will do this same check when we verify the CPU service.
        if [ "${LOC_CPU_NETNAME}" != "" ] && [ "${CPU_NET_NAME}" != "${LOC_CPU_NETNAME}" ]; then
                echo -e "${PREFIX} location's cpu service network is different from the network of the CPU service itself"
                exit 2
        elif [ "${LOC_CPU_NETNAME}" != "" ] && [ "${CPU_NET_NAME}" == "${LOC_CPU_NETNAME}" ]; then
                echo -e "${PREFIX} location's cpu service network is the same as the network of the CPU service itself"
        fi

}

# Verify the service instances that should be running
function verifyServices {
        ALLSERV=$(curl -sSL $ANAX_API/service | jq -r '.instances.active')
        NUMSERV=$(echo ${ALLSERV} | jq -r '. | length')

        echo -e "There are ${NUMSERV} active services running"

        for (( ix=0; ix<$NUMSERV; ix++ ))
        do
                INST=$(echo ${ALLSERV} | jq -r '.['$ix']')
                REFURL=$(echo ${INST} | jq -r '.ref_url')
                # echo -e "${PREFIX} working on service ${ix}, ${REFURL}: ${INST}"
                echo -e "${PREFIX} working on service ${ix}, ${REFURL}"

                if [ "${REFURL}" == "https://bluehorizon.network/services/location" ]; then
                        handleLocation "${INST}"
                elif [ "${REFURL}" == "https://bluehorizon.network/service-cpu" ]; then
                        handleCPU "${INST}"
                fi
        done
}

function handleMsghubDataVerification {

    echo -e "${PREFIX} veriying data for $2."
    # get needed env from the Env of docker container
    cpu2msghub_env=$(docker inspect $1 | jq -r '.[0].Config.Env')
    MSGHUB_BROKER_URL=$(echo  $cpu2msghub_env |jq '.' |grep MSGHUB_BROKER_URL | grep -o '=.*"' | sed 's/["=]//g')
    MSGHUB_API_KEY=$(echo  $cpu2msghub_env |jq '.' |grep MSGHUB_API_KEY | grep -o '=.*"' | sed 's/["=]//g')
    HZN_ORGANIZATION=$(echo  $cpu2msghub_env |jq '.' |grep HZN_ORGANIZATION | grep -o '=.*"' | sed 's/["=]//g')
    HZN_PATTERN=$(echo  $cpu2msghub_env |jq '.' |grep HZN_PATTERN | grep -o '=.*"' | sed 's/["=]//g')

    # the script will exit after receiving first data. timeout after 1 minute.
    timeout --preserve-status 1m kafkacat -C -c 1 -q -o end -f "%t/%p/%o/%k: %s\n" -b $MSGHUB_BROKER_URL -X "api.version.request=true" -X "security.protocol=sasl_ssl" -X "sasl.mechanisms=PLAIN" -X "sasl.username=${MSGHUB_API_KEY:0:16}" -X "sasl.password=${MSGHUB_API_KEY:16}" -t "$HZN_ORGANIZATION.$HZN_PATTERN"
    if [ $? -eq 0 ]; then
        echo -e "${PREFIX} data verification for $2 service successful."
    else
        echo -e "${PREFIX} error: No data received from $2 service."
        exit 2
    fi
}

# Data verification
function verifyData {
    ALLSERV=$(curl -sSL $ANAX_API/service | jq -r '.instances.active')
    NUMSERV=$(echo ${ALLSERV} | jq -r '. | length')

    for (( ix=0; ix<$NUMSERV; ix++ ))
    do
        INST=$(echo ${ALLSERV} | jq -r '.['$ix']')
        REFURL=$(echo ${INST} | jq -r '.ref_url')

        if [ "${REFURL}" == "https://bluehorizon.network/service-cpu2msghub" ]; then
            id=$(echo "${INST}" | jq -r '.containers[0].Id')
            handleMsghubDataVerification "${id}" "${REFURL}"
        fi
    done
}

# =====================================================================================================
# Main body of the script. Wait until all agreements are started.
verifyAgreements

# Wait until all agreements are executing.
agreementsReached agreement_execution_start_time

# Check to make sure service are running correctly.
verifyServices

# Verify exchange node status
checkNodeStatus true
if [ $? != 0 ]; then
    echo "Node status verification failed"
    exit 1
fi

# Do data verification
if [ "${PATTERN}" == "sall" ]; then
    verifyData
fi

# Monitor agreements to make sure they stay in place. This function does not
# return unless it finds an error.
monitorAgreements
