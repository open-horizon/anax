#!/bin/bash

# Enable debug tracing when DEBUG=1 or RUNNER_DEBUG=1 (GitHub Actions debug mode).
if [ "${DEBUG:-0}" = "1" ] || [ "${RUNNER_DEBUG:-0}" = "1" ]; then
    set -x
fi

# Base directory for test resources (test/ directory, one level up from this script).
E2EDEV_ROOT="$(pwd)"

PREFIX="Multiple agents:"

startMultiAgents() {
  echo -e "${PREFIX} Starting agents"

  # get main agent's user input and save it to a file to be used by mult-agent
  UIFILE="/tmp/agent_userinput.json"
  if ! ui=$(hzn userinput list); then
    echo -e "${PREFIX} Failed to get user input from the main agent. $ui"
    exit 1
  fi
  
  echo -e "${PREFIX} userinput is: $ui"
  echo "$ui" > $UIFILE

  # set css certs for the agent container
  if [ "${CERT_LOC}" -eq 1 ]; then
    mkdir -p /tmp/hzndev
    cat /certs/css.crt > /tmp/hzndev/css.crt
  fi

  counter=0
  while [ "${counter}" -lt "${MULTIAGENTS}" ]; do
    agent_port=$((8512 + counter))
    device_num=$((6 + counter))

    # set config for the agent container
    mkdir -p /tmp/hzndev
    configfile="/tmp/hzndev/horizon.multi_agents"
    {
      echo -e "HZN_EXCHANGE_URL=${EXCH_APP_HOST}"
      echo -e "HZN_FSS_CSSURL=${CSS_URL}"
      echo -e "HZN_AGBOT_URL=${AGBOT_SAPI_URL}"
      echo -e "HZN_DEVICE_ID=anaxdevice${device_num}"
      echo -e "HZN_NODE_ID=anaxdevice${device_num}"
      echo -e "HZN_AGENT_PORT=${agent_port}"
    } > "$configfile"
    if [ "${CERT_LOC}" -eq 1 ]; then
      echo "HZN_MGMT_HUB_CERT_PATH=/tmp/hzndev/css.crt" >> "$configfile"
    fi

    # start agent container
    echo "${PREFIX} Start agent container horizon${horizon_num} ..."
    export HC_DONT_PULL=1;
    export HC_DOCKER_TAG=testing
    horizon_num=${device_num};
    if ! /tmp/anax-in-container/horizon-container start ${horizon_num} $configfile; then
      echo -e "${PREFIX} Failed to start agent horizon${horizon_num}."
      exit 1
    fi

    # connect the hzn_horizonnet network to the container so that it
    # can use the local exchange-api and css-api through this network
    if ! docker network connect "${DOCKER_TEST_NETWORK}" "horizon${horizon_num}"; then
      echo -e "${PREFIX} Failed to connect agent container horizon${horizon_num} to network ${DOCKER_TEST_NETWORK}."
      exit 1
    fi
    sleep 10

    # copy the userinput file to agent container
    if ! docker cp $UIFILE horizon${horizon_num}:$UIFILE; then
      echo -e "${PREFIX} Failed to copy file $UIFILE to agent container horizon${horizon_num}."
      exit 1
    fi

    # register the agent
    ha_group_option=""
    if [ "$HA" = "1" ]; then
      ha_group_option="--ha-group group2"
    fi

    regcmd="hzn register -f $UIFILE -p $PATTERN -o e2edev@somecomp.com -u e2edev@somecomp.com/e2edevadmin:e2edevadminpw $ha_group_option"
    # shellcheck disable=SC2086
    if ! ret=$(docker exec -e "HORIZON_URL=http://localhost:${agent_port}" "horizon${horizon_num}" $regcmd); then
      echo "${PREFIX} Registration failed for anaxdevice${device_num}: $ret"
      return 1
    fi
    echo "$ret"

    (( counter=counter+1 ))
  done
}

verifyMultiAgentsAgreements() {
  echo -e "${PREFIX} Verifying agreements"

  counter=0
  while [ "${counter}" -lt "${MULTIAGENTS}" ]; do
    agent_port=$((8512 + counter))
    device_num=$((6 + counter))

    echo "${PREFIX} Verify agreement for agent container horizon${device_num} ..."
 
    # copy the test scripts over to agent container
    docker cp "${E2EDEV_ROOT}"/gov/verify_agreements.sh horizon${device_num}:/tmp/.
    docker cp "${E2EDEV_ROOT}"/gov/check_node_status.sh horizon${device_num}:/tmp/.

    docker exec -e "ANAX_API=http://localhost:${agent_port}" \
        -e "EXCH_APP_HOST=${EXCH_APP_HOST}" \
        -e ORG_ID=e2edev@somecomp.com \
        -e "PATTERN=${PATTERN}" \
        -e ADMIN_AUTH=e2edevadmin:e2edevadminpw \
        -e "NODEID=anaxdevice${device_num}" \
        -e "NOLOOP=${NOLOOP}" \
        "horizon${device_num}" /tmp/verify_agreements.sh

    (( counter=counter+1 ))
  done
}

stopMultiAgents() {
  echo -e "${PREFIX} Stopping agents"

  counter=0
  while [ "${counter}" -lt "${MULTIAGENTS}" ]; do
    agent_port=$((8512 + counter))
    device_num=$((6 + counter))

    echo "${PREFIX} Delete agent container horizon${device_num} ..."
    ret=$(docker exec -e HORIZON_URL=http://localhost:${agent_port} horizon${device_num} hzn unregister -f -r)
    echo "$ret"
    /tmp/anax-in-container/horizon-container stop ${device_num}

    (( counter=counter+1 ))
  done
}

