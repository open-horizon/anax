#!/bin/bash

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
function get_container_id {
    timeout=$1
    # get the instance id of the specified service with quotes removed
 	inst=$(curl -s $ANAX_API/service | jq -r --arg SVC_URL "$SVC_URL" --arg SVC_ORG "$SVC_ORG" '.instances.active[] | select (.ref_url==$SVC_URL and .organization==$SVC_ORG)')
    while [ $timeout -gt 0 ] && [[ $inst == "" ]]; do
        let timeout=$timeout-1
        echo "Waiting for netspeed service to start."
        sleep 5s
        inst=$(curl -s $ANAX_API/service | jq -r --arg SVC_URL "$SVC_URL" --arg SVC_ORG "$SVC_ORG" '.instances.active[] | select (.ref_url==$SVC_URL and .organization==$SVC_ORG)')
    done 
    if [ $? -ne 0 ]; then
        echo -e "${PREFIX} failed to get $SVC_ORG/$SVC_URL service instace."
        exit -1
    fi
    inst_id=$(echo "$inst" | jq '.instance_id')
    inst_id="${inst_id%\"}"
    inst_id="${inst_id#\"}"

    # get the cpu service container for the main agent
    container=$(docker ps |grep $inst_id)
    if [ $? -ne 0 ]; then
        echo -e "${PREFIX} cannot not find $SVC_ORG/$SVC_URL container."
        exit -1
    fi

    docker_id=$(echo "$container" | cut -d ' ' -f1)
    if [ $? -ne 0 ]; then
        echo -e "${PREFIX} failed to get the $SVC_ORG/$SVC_URL container id."
        exit -1
    fi

	CONTAINER_ID=${docker_id}
}

# first parameter is secret name
# second parameter is secret key
# third parameter is secret detail
# fourth parameter is the number of 10 second intervals to wait for the secret to update. 0 for no wait.
function check_container_secret {
    # get the contents of the secret file
    secret_file_content=$(docker exec $CONTAINER_ID sh -c "cat /open-horizon-secrets/$1")
    if [ $? -ne 0 ]; then 
        echo -e "${PREFIX} failed to find secret file in container $CONTAINER_ID for service $SVC_ORG/$SVC_URL for service secret $1."
        exit -1
    fi

    secret_key=$(echo $secret_file_content | jq -r '.key')
    if [ $? -ne 0 ]; then 
        echo -e "${PREFIX} failed to find secret key in secret file $1 in container $CONTAINER_ID for service $SVC_ORG/$SVC_URL."
        exit -1
    fi

    timeout=$4
    while [[ $secret_key != $2  ]] && [ $timeout -gt 0 ]; do
        echo -e "${PREFIX} waiting for secret $1 to be updated."
        let timeout=$timeout-1
        sleep 10s

        secret_file_content=$(docker exec $CONTAINER_ID sh -c "cat /open-horizon-secrets/$1")
        if [ $? -ne 0 ]; then 
            echo -e "${PREFIX} failed to find secret file in container $CONTAINER_ID for service $SVC_ORG/$SVC_URL for service secret $1."
            exit -1
        fi
        export secret_key=$(echo $secret_file_content | jq -r '.key')
        if [ $? -ne 0 ]; then 
            echo -e "${PREFIX} failed to find secret key in secret file $1 in container $CONTAINER_ID for service $SVC_ORG/$SVC_URL."
            exit -1
        fi
    done

    if [[ $secret_key != $2 ]]; then 
        echo -e "${PREFIX} expected secret $1 in container $CONTAINER_ID for service $SVC_ORG/$SVC_URL to have key \"$2\". Found key \"$secret_key\"."
        exit -1
    fi

    secret_value=$(echo $secret_file_content | jq -r '.value')
    if [ $? -ne 0 ]; then 
        echo -e "${PREFIX} failed to find secret value in secret file $1 in container $CONTAINER_ID for service $SVC_ORG/$SVC_URL."
        exit -1
    fi
    if [[ $secret_value != $3 ]]; then 
        echo -e "${PREFIX} expected secret $1 in container $CONTAINER_ID for service $SVC_ORG/$SVC_URL to have value \"$3\". Found value \"$secret_value\"."
        exit -1
    fi
    echo $secret_value
}

# first parameter is secret name
# second parameter is secret key
# third parameter is secret detail
# fourth is user auth
# fifth is org
function update_secret {
    hzn secretsmanager secret add "$1" --secretKey="$2" --secretDetail="$3" -u "$4" -o "$5" -O
    if [ $? -ne 0 ]; then
        echo -e "${PREFIX} failed to update service secret $1."
        exit -1
    fi
}

# collect the node's current policy and pattern to return to after verifying secret cleanup
function unregister_node {
    # get the node's current pattern
    CURRENT_NODE_INFO=$(hzn node list)
    if [ $? -ne 0 ]; then 
        echo -e "${PREFIX} 'hzn node list' returned non-zero exit code."
        exit -1
    fi
    CURRENT_PATTERN=$(echo $CURRENT_NODE_INFO | jq -r '.pattern')
    if [[ $CURRENT_PATTERN != "" ]]; then
        REREG_PATTERN="-p $CURRENT_PATTERN"
    fi
    REREG_ORG=$(echo $CURRENT_NODE_INFO | jq -r '.organization')
    REREG_AUTH=$E2EDEV_ADMIN_AUTH
    if [[ $REREG_ORG == "userdev" ]]; then
        REREG_AUTH=$USERDEV_ADMIN_AUTH
    fi

    REREG_POLICY=$(hzn policy list)
    if [ $? -ne 0 ]; then 
        echo -e "${PREFIX} failed to find the node's current policy."
        exit -1
    fi

    #get the userinput the node is registered with
    USER_INPUTS=$(hzn userinput list)
    if [ $? -ne 0 ]; then 
        echo -e "${PREFIX} failed to find the node's current userinputs."
        exit -1
    fi

    hzn unregister -f
    if [ $? -ne 0 ]; then 
        echo -e "${PREFIX} failed to unregister the node."
        exit -1
    fi
}

# reregister the node with the saved node policy and pattern info
function reregister_node {
    echo $USER_INPUTS > ./userinput.json
    echo $REREG_POLICY | hzn register -n "an12345" -u $REREG_AUTH -o $REREG_ORG $REREG_PATTERN --policy /dev/stdin -f ./userinput.json
    if [ $? -ne 0 ]; then 
        echo -e "${PREFIX} failed to reregister the node."
        exit -1
    fi
    rm ./userinput.json
}

# first arg is service name
# second arg is service org
# third arg is service version
function suspend_service {
    echo -e "Suspending service $2/$1:$3"
    hzn service configstate suspend $2 $1 $3 -f
    if [ $? -ne 0 ]; then 
        echo -e "${PREFIX} failed to suspend service $2/$1:$3"
        exit -1
    fi
}

# first arg is service name
# second arg is service org
# third arg is service version
function resume_service {
    echo -e "Suspending service $2/$1:$3"
    hzn service configstate resume $2 $1 $3
    if [ $? -ne 0 ]; then 
        echo -e "${PREFIX} failed to resume service $2/$1:$3"
        exit -1
    fi
}

function get_auth_for_tests {
    CURRENT_NODE_INFO=$(hzn node list)
    if [ $? -ne 0 ]; then 
        echo -e "${PREFIX} 'hzn node list' returned non-zero exit code."
        exit -1
    fi

    export USE_ORG=$(echo $CURRENT_NODE_INFO | jq -r '.organization')
    export USE_AUTH=$E2EDEV_ADMIN_AUTH
    if [[ $USE_ORG == "userdev" ]]; then
        export USE_AUTH=$USERDEV_ADMIN_AUTH
    fi
}

# Set the auth and org for what the node is registered for
get_auth_for_tests

# Check that the secrets are initially mounted for the top-level netspeed service 
SVC_URL="https://bluehorizon.network/services/netspeed"
SVC_ORG="e2edev@somecomp.com"
get_container_id 6

check_container_secret "sec1" $INITIAL_SECRET_KEY1 $INITIAL_SECRET_DETAIL1 0

check_container_secret "sec2" $INITIAL_SECRET_KEY2 $INITIAL_SECRET_DETAIL2 0

# Check that the secrets are initially mounted for the dependent singleton IBM/service-cpu service 
SVC_URL="https://bluehorizon.network/service-cpu"
SVC_ORG="IBM"
get_container_id 0

check_container_secret "secret-dep1" $INITIAL_SECRET_KEY1 $INITIAL_SECRET_DETAIL1 0

# Update netspeed-secret1 and check it updates in both containers
update_secret "netspeed-secret1" "test1" "updatedSecret1" $USE_AUTH $USE_ORG
check_container_secret "secret-dep1" "test1" "updatedSecret1" 24

SVC_URL="https://bluehorizon.network/services/netspeed"
SVC_ORG="e2edev@somecomp.com"
get_container_id 0
check_container_secret "sec1" "test1" "updatedSecret1" 0

# Suspend the location service (location has service-cpu as a dependent)
suspend_service "https://bluehorizon.network/services/location" "e2edev@somecomp.com" 

ag_num=$(hzn agreement list | jq '. | length')
timeout=6

while [ $ag_num -gt 4 ] && [ $timeout -gt 0 ]; do
    echo "Waiting for the location agreement to be cancelled."
    sleep 5s
    let ag_num=$(hzn agreement list | jq '. | length')
    let timeout=$timeout-1
done 
if [ $ag_num -gt 4 ]; then
    echo "Timed out waiting for the location agreement to be removed."
    exit -1
fi 

# Check that the secret for the shared singleton is not removed
SVC_URL="https://bluehorizon.network/service-cpu"
SVC_ORG="IBM"
get_container_id 0
check_container_secret "secret-dep1" "test1" "updatedSecret1" 0

# Resume the location service and reset the changed secret
resume_service "https://bluehorizon.network/services/location" "e2edev@somecomp.com" 
update_secret "netspeed-secret1" $INITIAL_SECRET_KEY1 $INITIAL_SECRET_KEY2 $USE_AUTH $USE_ORG

# Unregister the node and verify all secret files are removed
unregister_node

timeout=6

while [ "$(ls -A /root/tmp)" ] && [ $timeout -gt 0 ]; do
    echo "Waiting for all secret files in /var/run/horizon/secrets to be removed."
    let timeout=$timeout-1
    sleep 5s
done
if [ "$(ls -A /root/tmp)" ]; then
    echo "Timed out waiting for the secret files to be removed from the agent filesystem."
    exit -1
fi 

# Return the node to it's previous registered state
reregister_node