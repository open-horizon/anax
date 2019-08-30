#!/bin/sh

# Check env vars that we know should be set to verify that everything is working
function verify {
    if [ "$2" == "" ]
    then
        echo -e "Error: $1 should be set but is not."
        exit 2
    fi
}

# If the container is running in the Horizon environment, then the Horizon platform env vars should all be there.
# Otherwise, assume it is running outside Horizon and running in a non-Horizon environment.

BASEURL=""
if [ "$HZN_AGREEMENTID" != "" ]
then
    verify "HZN_RAM" $HZN_RAM
    verify "HZN_CPUS" $HZN_CPUS
    verify "HZN_ARCH" $HZN_ARCH
    verify "HZN_DEVICE_ID" $HZN_DEVICE_ID
    verify "HZN_ORGANIZATION" $HZN_ORGANIZATION
#    verify "HZN_HASH" $HZN_HASH - Delete
    verify "HZN_EXCHANGE_URL" $HZN_EXCHANGE_URL
    verify "HZN_ESS_API_PROTOCOL" $HZN_ESS_API_PROTOCOL
    verify "HZN_ESS_API_ADDRESS" $HZN_ESS_API_ADDRESS
    verify "HZN_ESS_API_PORT" $HZN_ESS_API_PORT
    verify "HZN_ESS_AUTH" $HZN_ESS_AUTH
    verify "HZN_ESS_CERT" $HZN_ESS_CERT
    echo -e "All Horizon platform env vars verified."

    echo -e "Service is running on node $HZN_DEVICE_ID in org $HZN_ORGANIZATION"

    if [ "${HZN_PATTERN}" == "" ]
    then
        echo "Service is running in policy mode"
    else
        echo "Service is running in pattern mode: ${HZN_PATTERN}"
    fi

    # Assuming the API address is a unix socket file. HZN_ESS_API_PROTOCOL should be "unix".
    BASEURL='--unix-socket '${HZN_ESS_API_ADDRESS}' https://localhost/api/v1/objects/'

else
    echo -e "Running outside Horizon, skip Horizon platform env var checks."
fi

verify "MY_VAR1" $MY_VAR1
echo -e "All Agreement Service variables verified."

OBJECT_TYPE="model"
echo -e "Looking for file objects of type ${OBJECT_TYPE}"

# ${HZN_ESS_AUTH} is mounted to this container and contains a json file with the credentials for authenticating to the ESS.
USER=$(cat ${HZN_ESS_AUTH} | jq -r ".id")
PW=$(cat ${HZN_ESS_AUTH} | jq -r ".token")

# Passing basic auth creds in base64 encoded form (-u).
AUTH="-u ${USER}:${PW} "

# ${HZN_ESS_CERT} is mounted to this container and contains the client side SSL cert to talk to the ESS API.
CERT="--cacert ${HZN_ESS_CERT} "

FAILCOUNT=0

# There should be 2 files loaded into the CSS for this node and the node should be able to get them quickly. If not,
# there is a problem and the service will terminate, causing it to get into a restart loop in docker. This should be detected
# by the test automation and terminate the test in error.

while :
do
    if [ "$BASEURL" != "" ]
    then
        # First sync service call should pick up any objects received the last time we were started.
        echo -e "Retrieving sync service objects that have already been received."

        FILE_LOC="/root/objects"
        mkdir -p ${FILE_LOC}

        # For each object, write the data into the local file system using the object ID as the file name. Then mark the object
        # as received so that a subsequent poll doesn't see the object again.
        OBJS=$(curl -sL ${AUTH}${CERT}${BASEURL}${OBJECT_TYPE}?received=true)

        BADRES=$(echo ${OBJS} | jq -r '.[].objectID')
        if [ "${BADRES}" == "" ]
        then
            echo "Error Return from object poll: ${OBJS}"
            exit 1
        fi

        echo ${OBJS} | jq -c '.[]' | while read i; do

            del=$(echo $i | jq -r '.deleted')
            id=$(echo $i | jq -r '.objectID')
            if [ "$del" == "true" ]
            then
                echo "Acknowledging that Object $id is deleted"
                ACKDEL=$(curl -sLX PUT ${AUTH}${CERT}${BASEURL}${OBJECT_TYPE}/${id}/deleted)
                rm -f ${FILE_LOC}/${id}
            else
                DATA=$(curl -sL -o ${FILE_LOC}/${id} ${AUTH}${CERT}${BASEURL}${OBJECT_TYPE}/${id}/data)
                RCVD=$(curl -sLX PUT ${AUTH}${CERT}${BASEURL}${OBJECT_TYPE}/${id}/received)
                echo -e "Received object: ${id}"
            fi
        done

        # There should be 2 files in the file sync service for this node. If not, there is a problem, exit the workload to fail the test.
        COUNT=$(ls ${FILE_LOC} | wc -l)
        COUNT_TARGET="2"
        if [ "${COUNT}" != "${COUNT_TARGET}" ]
        then
            echo -e "Found ${COUNT} files from the sync service in ${FILE_LOC}, there should be ${COUNT_TARGET}."
            if [ "$FAILCOUNT" -gt "1" ]
            then
                exit 1
            fi
            sleep 2
            FAILCOUNT=$((FAILCOUNT+1))
        else
            break
        fi
    else
        break
    fi
done

# Keep everything alive
while :
do
    echo -e "Service usehello running."
    if [ "$MY_VAR1" != "outside" ]
    then
        co=$(curl -sS 'http://amd64_helloservice:8000')
        echo -e "Hello service: $co"
        cpuo=$(curl -sS 'http://amd64_cpu:8347')
        echo -e "CPU Usage: $cpuo"
    fi

    if [ "$BASEURL" != "" ]
    then
        echo -e "Calling ESS to poll for new objects"

        # Pick up any newly added objects or notifications of changed or deleted objects since our initial poll.
        OBJS=$(curl -sL ${AUTH}${CERT}${BASEURL}${OBJECT_TYPE})

        echo "Full poll response: ${OBJS}"

        # Iterate over each returned object, it will be set into $i
        echo ${OBJS} | jq -c '.[]' | while read i; do

            # work with each returned object in $i
            del=$(echo $i | jq -r '.deleted')
            id=$(echo $i | jq -r '.objectID')
            if [ "$del" == "true" ]
            then
                echo "Acknowledging that Object $id is deleted"
                ACKDEL=$(curl -sLX PUT ${AUTH}${CERT}${BASEURL}${OBJECT_TYPE}/${id}/deleted)
                rm -f ${FILE_LOC}/${id}
            else
                # Assume we got a new object
                DATA=$(curl -sL -o ${FILE_LOC}/${id} ${AUTH}${CERT}${BASEURL}${OBJECT_TYPE}/${id}/data)
                RCVD=$(curl -sLX PUT ${AUTH}${CERT}${BASEURL}${OBJECT_TYPE}/${id}/received)
                echo -e "Got a new object: ${id}"
            fi
        done
    fi

    sleep 10
done
