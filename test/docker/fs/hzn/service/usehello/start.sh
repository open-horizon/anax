#!/bin/sh

# Check env vars that we know should be set to verify that everything is working
verify() {
  if [ "$2" = "" ]
  then
    echo "Error: $1 should be set but is not."
    exit 2
  fi
}

# If the container is running in the Horizon environment, then the Horizon platform env vars should all be there.
# Otherwise, assume it is running outside Horizon and running in a non-Horizon environment.

if [ "$HZN_AGREEMENTID" != "" ]
then
  verify "HZN_RAM" "$HZN_RAM"
  verify "HZN_CPUS" "$HZN_CPUS"
  verify "HZN_ARCH" "$HZN_ARCH"
  verify "HZN_NODE_ID" "$HZN_NODE_ID"
  verify "HZN_ORGANIZATION" "$HZN_ORGANIZATION"
  # verify "HZN_HASH" "$HZN_HASH" - Delete
  verify "HZN_EXCHANGE_URL" "$HZN_EXCHANGE_URL"
  verify "HZN_ESS_API_PROTOCOL" "$HZN_ESS_API_PROTOCOL"
  verify "HZN_ESS_API_ADDRESS" "$HZN_ESS_API_ADDRESS"
  verify "HZN_ESS_API_PORT" "$HZN_ESS_API_PORT"
  verify "HZN_ESS_AUTH" "$HZN_ESS_AUTH"
  verify "HZN_ESS_CERT" "$HZN_ESS_CERT"
  echo "All Horizon platform env vars verified."

  echo "Service is running on node $HZN_NODE_ID in org $HZN_ORGANIZATION"

  if [ "${HZN_PATTERN}" = "" ]
  then
    echo "Service is running in policy mode"
  else
    echo "Service is running in pattern mode: ${HZN_PATTERN}"
  fi

  # Assuming the API address is a unix socket file. HZN_ESS_API_PROTOCOL should be "unix".
  ESS_SOCKET="${HZN_ESS_API_ADDRESS}"
  ESS_BASEURL="https://localhost/api/v1/objects/"

else
  echo "Running outside Horizon, skip Horizon platform env var checks."
fi

verify "MY_VAR1" "$MY_VAR1"
echo "All Agreement Service variables verified."

OBJECT_TYPE="model"
echo "Looking for file objects of type ${OBJECT_TYPE}"

# ${HZN_ESS_AUTH} is mounted to this container and contains a json file with the credentials for authenticating to the ESS.
# ${HZN_ESS_CERT} is mounted to this container and contains the client side SSL cert to talk to the ESS API.
# ESS_USER and ESS_PW are read here only when running inside Horizon (HZN_ESS_AUTH is set).
# ESS_SOCKET and ESS_BASEURL are set above when HZN_AGREEMENTID is set.
ESS_USER=""
ESS_PW=""
if [ "${HZN_ESS_AUTH}" != "" ]
then
  ESS_USER=$(jq -r ".id" < "${HZN_ESS_AUTH}")
  ESS_PW=$(jq -r ".token" < "${HZN_ESS_AUTH}")
fi

FAILCOUNT=0

# There should be 2 files loaded into the CSS for this node and the node should be able to get them quickly. If not,
# there is a problem and the service will terminate, causing it to get into a restart loop in docker. This should be detected
# by the test automation and terminate the test in error.

while :
do
  if [ "${ESS_SOCKET}" != "" ]
  then
    # Poll for all pending objects (not yet received) as well as any previously received objects.
    echo "Retrieving sync service objects."

    FILE_LOC="/e2edevuser/objects"
    mkdir -p "${FILE_LOC}"

    # For each object, write the data into the local file system using the object ID as the file name. Then mark the object
    # as received so that a subsequent poll doesn't see the object again.

    OBJS=$(curl -sL --unix-socket "${ESS_SOCKET}" -u "${ESS_USER}:${ESS_PW}" --cacert "${HZN_ESS_CERT}" "${ESS_BASEURL}${OBJECT_TYPE}")

    # Verify the response is a valid JSON array; a non-array response indicates an ESS error.
    if ! echo "${OBJS}" | jq -e 'if type == "array" then true else error end' > /dev/null 2>&1
    then
      echo "Error return from object poll (expected JSON array): ${OBJS}"
      exit 1
    fi

    echo "${OBJS}" | jq -c '.[]' | \
    while read -r i
    do
      del=$(echo "$i" | jq -r '.deleted')
      id=$(echo "$i" | jq -r '.objectID')
      if [ "$del" = "true" ]
      then
        echo "Acknowledging that Object $id is deleted"
        curl -sLX PUT --unix-socket "${ESS_SOCKET}" -u "${ESS_USER}:${ESS_PW}" --cacert "${HZN_ESS_CERT}" "${ESS_BASEURL}${OBJECT_TYPE}/${id}/deleted" > /dev/null
        rm -f "${FILE_LOC}/${id}"
      else
        curl -sL -o "${FILE_LOC}/${id}" --unix-socket "${ESS_SOCKET}" -u "${ESS_USER}:${ESS_PW}" --cacert "${HZN_ESS_CERT}" "${ESS_BASEURL}${OBJECT_TYPE}/${id}/data" > /dev/null
        curl -sLX PUT --unix-socket "${ESS_SOCKET}" -u "${ESS_USER}:${ESS_PW}" --cacert "${HZN_ESS_CERT}" "${ESS_BASEURL}${OBJECT_TYPE}/${id}/received" > /dev/null
        echo "Received object: ${id}"
      fi
    done

    # There should be 2 files in the file sync service for this node. If not, there is a problem, exit the workload to fail the test.
    COUNT=$(find "${FILE_LOC}" -maxdepth 1 -type f | wc -l | tr -d ' ')
    COUNT_TARGET="2"
    if [ "${COUNT}" != "${COUNT_TARGET}" ]
    then
      echo "Found ${COUNT} files from the sync service in ${FILE_LOC}, there should be ${COUNT_TARGET}."
      if [ "$FAILCOUNT" -gt "1" ]
      then
        exit 1
      fi
      sleep 2
      FAILCOUNT=$(( FAILCOUNT+1 ))
    else
      break
    fi
  fi
done

# Keep everything alive
while :
do
  echo "Service usehello running."
  if [ "$MY_VAR1" != "outside" ]
  then
    co=$(curl -sS "http://${HZN_ARCH}_helloservice:8000")
    echo "Hello service: $co"
    cpuo=$(curl -sS "http://${HZN_ARCH}_cpu:8347")
    echo "CPU Usage: $cpuo"
  fi

  if [ "${ESS_SOCKET}" != "" ]
  then
    echo "Calling ESS to poll for new objects"

    # Pick up any newly added objects or notifications of changed or deleted objects since our initial poll.
    OBJS=$(curl -sL --unix-socket "${ESS_SOCKET}" -u "${ESS_USER}:${ESS_PW}" --cacert "${HZN_ESS_CERT}" "${ESS_BASEURL}${OBJECT_TYPE}")

    echo "Full poll response: ${OBJS}"

    # Iterate over each returned object, it will be set into $i
    echo "${OBJS}" | jq -c '.[]' | \
    while read -r i
    do
      # work with each returned object in $i
      del=$(echo "$i" | jq -r '.deleted')
      id=$(echo "$i" | jq -r '.objectID')
      if [ "$del" = "true" ]
      then
        echo "Acknowledging that Object $id is deleted"
        curl -sLX PUT --unix-socket "${ESS_SOCKET}" -u "${ESS_USER}:${ESS_PW}" --cacert "${HZN_ESS_CERT}" "${ESS_BASEURL}${OBJECT_TYPE}/${id}/deleted" > /dev/null
        rm -f "${FILE_LOC}/${id}"
      else
        # Assume we got a new object
        curl -sL -o "${FILE_LOC}/${id}" --unix-socket "${ESS_SOCKET}" -u "${ESS_USER}:${ESS_PW}" --cacert "${HZN_ESS_CERT}" "${ESS_BASEURL}${OBJECT_TYPE}/${id}/data" > /dev/null
        curl -sLX PUT --unix-socket "${ESS_SOCKET}" -u "${ESS_USER}:${ESS_PW}" --cacert "${HZN_ESS_CERT}" "${ESS_BASEURL}${OBJECT_TYPE}/${id}/received" > /dev/null
        echo "Got a new object: ${id}"
      fi
    done
  fi

  sleep 10
done
