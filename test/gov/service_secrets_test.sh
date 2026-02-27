#!/bin/bash

# Enable debug tracing when DEBUG=1 or RUNNER_DEBUG=1 (GitHub Actions debug mode).
if [ "${DEBUG:-0}" = "1" ] || [ "${RUNNER_DEBUG:-0}" = "1" ]; then
    set -x
fi

PREFIX="Service secrets test:"
# Check that the netspeed secrets are in the top level and the dependent service containers

ANAX_API=http://localhost:8510
USERDEV_ADMIN_AUTH="userdev/userdevadmin:userdevadminpw"
E2EDEV_ADMIN_AUTH="e2edev@somecomp.com/e2edevadmin:e2edevadminpw"

INITIAL_SECRET_KEY1="test"
INITIAL_SECRET_DETAIL1="netspeed-password"
INITIAL_SECRET_KEY2="test"
INITIAL_SECRET_DETAIL2="netspeed-other-password"

# first paramter is number of 5 sec intervals to wait for netspeed to start. 0 for no wait
# (sometimes netspeed hasn't started again after the agreement was cancelled for the previous test)
get_container_id() {
    timeout=$1
    # get the instance id of the specified service with quotes removed
    if ! inst=$(curl -s $ANAX_API/service | jq -r --arg SVC_URL "$SVC_URL" --arg SVC_ORG "$SVC_ORG" '.instances.active[] | select (.ref_url==$SVC_URL and .organization==$SVC_ORG and .containers[].State=="running")'); then
        echo -e "${PREFIX} failed to get $SVC_ORG/$SVC_URL service instance."
        exit 255
    fi
    while [ "$timeout" -gt 0 ] && [[ $inst = "" ]]; do
        (( timeout=timeout-1 ))
        echo "Waiting for netspeed service to start."
        sleep 5s
        inst=$(curl -s $ANAX_API/service | jq -r --arg SVC_URL "$SVC_URL" --arg SVC_ORG "$SVC_ORG" '.instances.active[] | select (.ref_url==$SVC_URL and .organization==$SVC_ORG and .containers[].State=="running")')
    done
    inst_id=$(echo "$inst" | jq '.instance_id')
    inst_id="${inst_id%\"}"
    inst_id="${inst_id#\"}"

    # get the cpu service container for the main agent
    if ! container=$(docker ps | grep "$inst_id"); then
        echo -e "${PREFIX} cannot not find $SVC_ORG/$SVC_URL container."
        exit 255
    fi

    if ! docker_id=$(echo "$container" | cut -d ' ' -f1); then
        echo -e "${PREFIX} failed to get the $SVC_ORG/$SVC_URL container id."
        exit 255
    fi

	CONTAINER_ID=${docker_id}
}

# first parameter is secret name
# second parameter is secret key
# third parameter is secret detail
# fourth parameter is the number of 10 second intervals to wait for the secret to update. 0 for no wait.
# fifth parameter (true) indicate it is value only format
check_container_secret() {
    # get the contents of the secret file
    if ! secret_file_content=$(docker exec "$CONTAINER_ID" sh -c "cat /open-horizon-secrets/$1"); then
        echo -e "${PREFIX} failed to find secret file in container $CONTAINER_ID for service $SVC_ORG/$SVC_URL for service secret $1."
        exit 255
    fi

    is_value_only=$5

    if ! $is_value_only; then
        echo -e "${PREFIX} secret $1 is NOT value only format."
        if ! secret_key=$(echo "$secret_file_content" | jq -r '.key'); then
            echo -e "${PREFIX} failed to find secret key in secret file $1 in container $CONTAINER_ID for service $SVC_ORG/$SVC_URL."
            exit 255
        fi

        timeout=$4
        while [[ $secret_key != "$2" ]] && [ "$timeout" -gt 0 ]; do
            echo -e "${PREFIX} waiting for secret $1 to be updated."
            (( timeout=timeout-1 ))
            sleep 10s

            if ! secret_file_content=$(docker exec "$CONTAINER_ID" sh -c "cat /open-horizon-secrets/$1"); then
                echo -e "${PREFIX} failed to find secret file in container $CONTAINER_ID for service $SVC_ORG/$SVC_URL for service secret $1."
                exit 255
            fi
            if ! secret_key=$(echo "$secret_file_content" | jq -r '.key'); then
                echo -e "${PREFIX} failed to find secret key in secret file $1 in container $CONTAINER_ID for service $SVC_ORG/$SVC_URL."
                exit 255
            fi
            export secret_key
        done

        if [[ $secret_key != "$2" ]]; then
            echo -e "${PREFIX} expected secret $1 in container $CONTAINER_ID for service $SVC_ORG/$SVC_URL to have key \"$2\". Found key \"$secret_key\"."
            exit 255
        fi

        if ! secret_value=$(echo "$secret_file_content" | jq -r '.value'); then
            echo -e "${PREFIX} failed to find secret value in secret file $1 in container $CONTAINER_ID for service $SVC_ORG/$SVC_URL."
            exit 255
        fi
    else
        echo -e "${PREFIX} secret $1 is value only format."
        secret_value=$secret_file_content
    fi
    if [[ $secret_value != "$3" ]]; then
        echo -e "${PREFIX} expected secret $1 in container $CONTAINER_ID for service $SVC_ORG/$SVC_URL to have value \"$3\". Found value \"$secret_value\"."
        exit 255
    fi
    echo "$secret_value"
}

# first parameter is secret name
# second parameter is secret key
# third parameter is secret detail
# fourth is user auth
# fifth is org
update_secret() {
    if ! hzn secretsmanager secret add "$1" --secretKey="$2" --secretDetail="$3" -u "$4" -o "$5" -O; then
        echo -e "${PREFIX} failed to update service secret $1."
        exit 255
    fi
}

# collect the node's current policy and pattern to return to after verifying secret cleanup
unregister_node() {
    # get the node's current pattern
    if ! CURRENT_NODE_INFO=$(hzn node list); then
        echo -e "${PREFIX} 'hzn node list' returned non-zero exit code."
        exit 255
    fi
    CURRENT_PATTERN=$(echo "$CURRENT_NODE_INFO" | jq -r '.pattern')
    if [[ $CURRENT_PATTERN != "" ]]; then
        REREG_PATTERN="-p $CURRENT_PATTERN"
    fi
    REREG_ORG=$(echo "$CURRENT_NODE_INFO" | jq -r '.organization')
    REREG_AUTH=$E2EDEV_ADMIN_AUTH
    if [[ $REREG_ORG = "userdev" ]]; then
        REREG_AUTH=$USERDEV_ADMIN_AUTH
    fi

    if ! REREG_POLICY=$(hzn policy list); then
        echo -e "${PREFIX} failed to find the node's current policy."
        exit 255
    fi

    #get the userinput the node is registered with
    if ! USER_INPUTS=$(hzn userinput list); then
        echo -e "${PREFIX} failed to find the node's current userinputs."
        exit 255
    fi

    if ! hzn unregister -f; then
        echo -e "${PREFIX} failed to unregister the node."
        exit 255
    fi
}

# reregister the node with the saved node policy and pattern info
reregister_node() {
    echo "$USER_INPUTS" > ./userinput.json
    # shellcheck disable=SC2086
    if ! echo "$REREG_POLICY" | hzn register -n "an12345" -u "$REREG_AUTH" -o "$REREG_ORG" $REREG_PATTERN --policy /dev/stdin -f ./userinput.json; then
        echo -e "${PREFIX} failed to reregister the node."
        exit 255
    fi
    rm ./userinput.json
}

# first arg is service name
# second arg is service org
# third arg is service version
suspend_service() {
    echo -e "Suspending service $2/$1:$3"
    if ! hzn service configstate suspend "$2" "$1" "$3" -f; then
        echo -e "${PREFIX} failed to suspend service $2/$1:$3"
        exit 255
    fi
}

# first arg is service name
# second arg is service org
# third arg is service version
resume_service() {
    echo -e "Resuming service $2/$1:$3"
    if ! hzn service configstate resume "$2" "$1" "$3"; then
        echo -e "${PREFIX} failed to resume service $2/$1:$3"
        exit 255
    fi
}

get_auth_for_tests() {
    if ! CURRENT_NODE_INFO=$(hzn node list); then
        echo -e "${PREFIX} 'hzn node list' returned non-zero exit code."
        exit 255
    fi

    USE_ORG=$(echo "$CURRENT_NODE_INFO" | jq -r '.organization')
    export USE_ORG
    export USE_AUTH=$E2EDEV_ADMIN_AUTH
    if [[ $USE_ORG = "userdev" ]]; then
        export USE_AUTH=$USERDEV_ADMIN_AUTH
    fi
}

# Set the auth and org for what the node is registered for
get_auth_for_tests

# Check that the secrets are initially mounted for the top-level netspeed service 
SVC_URL="https://bluehorizon.network/services/netspeed"
SVC_ORG="e2edev@somecomp.com"
get_container_id 150

check_container_secret "sec1" $INITIAL_SECRET_KEY1 $INITIAL_SECRET_DETAIL1 0 true

check_container_secret "sec2" $INITIAL_SECRET_KEY2 $INITIAL_SECRET_DETAIL2 0 false

# Check that the secrets are initially mounted for the dependent singleton IBM/service-cpu service 
SVC_URL="https://bluehorizon.network/service-cpu"
SVC_ORG="IBM"
get_container_id 0

check_container_secret "secret-dep1" $INITIAL_SECRET_KEY1 $INITIAL_SECRET_DETAIL1 0 false

# Update netspeed-secret1 and check it updates in both containers
update_secret "netspeed-secret1" "test1" "updatedSecret1" "$USE_AUTH" "$USE_ORG"
check_container_secret "secret-dep1" "test1" "updatedSecret1" 24 false

SVC_URL="https://bluehorizon.network/services/netspeed"
SVC_ORG="e2edev@somecomp.com"
get_container_id 0
check_container_secret "sec1" "test1" "updatedSecret1" 0 true

# Suspend the location service (location has service-cpu as a dependent)
suspend_service "https://bluehorizon.network/services/location" "e2edev@somecomp.com" 

ag_num=$(hzn agreement list | jq '. | length')
timeout=6

while [ "$ag_num" -gt 4 ] && [ "$timeout" -gt 0 ]; do
    echo "Waiting for the location agreement to be cancelled."
    sleep 5s
    (( ag_num=$(hzn agreement list | jq '. | length') ))
    (( timeout=timeout-1 ))
done 
if [ "$ag_num" -gt 4 ]; then
    echo "Timed out waiting for the location agreement to be removed."
    exit 255
fi 

# Check that the secret for the shared singleton is not removed
SVC_URL="https://bluehorizon.network/service-cpu"
SVC_ORG="IBM"
get_container_id 0
check_container_secret "secret-dep1" "test1" "updatedSecret1" 0 false

# Resume the location service and reset the changed secret
resume_service "https://bluehorizon.network/services/location" "e2edev@somecomp.com" 
update_secret "netspeed-secret1" "$INITIAL_SECRET_KEY1" "$INITIAL_SECRET_KEY2" "$USE_AUTH" "$USE_ORG"

# Unregister the node and verify all secret files are removed
unregister_node

timeout=6

while [ "$(ls -A /root/tmp)" ] && [ $timeout -gt 0 ]; do
    echo "Waiting for all secret files in /var/run/horizon/secrets to be removed."
    (( timeout=timeout-1 ))
    sleep 5s
done
if [ "$(ls -A /root/tmp)" ]; then
    echo "Timed out waiting for the secret files to be removed from the agent filesystem."
    exit 255
fi 

# Return the node to it's previous registered state
reregister_node