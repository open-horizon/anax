#!/bin/bash

PREFIX="Multiple agents:"

function startMultiAgents {
  echo -e "${PREFIX} Starting agents"

  # get main agent's user input and save it to a file to be used by mult-agent
  UIFILE="/tmp/agent_userinput.json"
  ui=$(hzn userinput list)
  if [ $? -ne 0 ]; then
    echo -e "${PREFIX} Failed to get user input from the main agent. $ui"
    exit 1
  fi 
  
  echo -e "${PREFIX} userinput is: $ui"
  echo "$ui" > $UIFILE

  EX_IP_GATEWAY=$(docker inspect exchange-api | jq -r '.[].NetworkSettings.Networks.e2edev_test_network.Gateway')
  CSS_IP_GATEWAY=$(docker inspect css-api | jq -r '.[].NetworkSettings.Networks.e2edev_test_network.Gateway')
  EX_HOST_PORT=$(docker inspect exchange-api | jq -r '.[].NetworkSettings.Ports."8080/tcp"[].HostPort')
  CSS_HOST_PORT=$(docker inspect css-api | jq -r '.[].NetworkSettings.Ports."9443/tcp"[].HostPort')

  # set css certs for the agent container
  cat /certs/css.crt > /tmp/css.crt

  counter=0
  while [ ${counter} -lt ${MUL_AGENTS} ]; do
    agent_port=$((8512 + ${counter}))
    device_num=$((6 + ${counter}))

    # set config for the agent container
    echo "HZN_EXCHANGE_URL=http://$EX_IP_GATEWAY:$EX_HOST_PORT/v1" > /tmp/horizon
    echo "HZN_FSS_CSSURL=${CSS_URL}" >> /tmp/horizon
    echo "HZN_DEVICE_ID=anaxdevice${device_num}" >> /tmp/horizon
    echo "HZN_MGMT_HUB_CERT_PATH=/tmp/css.crt" >> /tmp/horizon
    echo "HZN_AGENT_PORT=${agent_port}" >> /tmp/horizon
    echo "E2E_NETWORK=e2edev_test_network" >> /tmp/horizon

    # start agent container
    echo "${PREFIX} Start agent container horizon${horizon_num} ..."
    export HC_DONT_PULL=1;
    export HC_DOCKER_TAG=testing
    horizon_num=${device_num};
    /tmp/anax-in-container/horizon-container start ${horizon_num} /tmp/horizon
    sleep 10

    # copy the userinput file to agent container
    docker cp $UIFILE horizon${horizon_num}:$UIFILE

    # register the agent
    regcmd="hzn register -f $UIFILE -p $PATTERN -o e2edev@somecomp.com -u e2edev@somecomp.com/e2edevadmin:e2edevadminpw"
    ret=$(docker exec -e "HORIZON_URL=http://localhost:${agent_port}" horizon${horizon_num} $regcmd)
    if [ $? -ne 0 ]; then
      echo "${PREFIX} Registration failed for anaxdevice${device_num}: $ret"
      return 1
    fi
    echo "$ret"

    let counter=counter+1
  done
}

function verifyMultiAgentsAgreements {
  echo -e "${PREFIX} Verifying agreements"

  counter=0
  while [ ${counter} -lt ${MUL_AGENTS} ]; do
    agent_port=$((8512 + ${counter}))
    device_num=$((6 + ${counter}))

    echo "${PREFIX} Verify agreement for agent container horizon${device_num} ..."
 
    # copy the test scripts over to agent container
    docker cp /root/verify_agreements.sh horizon${device_num}:/root/.
    docker cp /root/check_node_status.sh horizon${device_num}:/root/.

    docker exec -e ANAX_API=http://localhost:${agent_port} \
        -e EXCH_APP_HOST=http://exchange-api:8080/v1 \
        -e ORG_ID=e2edev@somecomp.com \
        -e PATTERN=${PATTERN} \
        -e ADMIN_AUTH=e2edevadmin:e2edevadminpw \
        -e NODEID=anaxdevice${device_num} \
        -e NOLOOP=${NOLOOP} \
        horizon${device_num} /root/verify_agreements.sh

    let counter=counter+1
  done
}

function stopMultiAgents {
  echo -e "${PREFIX} Stopping agents"

  counter=0
  while [ ${counter} -lt ${MUL_AGENTS} ]; do
    agent_port=$((8512 + ${counter}))
    device_num=$((6 + ${counter}))

    echo "${PREFIX} Delete agent container horizon${device_num} ..."
    let horizon_num=$i+5
    let port_num=$i+8511
    ret=$(docker exec -e HORIZON_URL=http://localhost:${agent_port} horizon${device_num} hzn unregister -f -r)
    echo "$ret"
    /tmp/anax-in-container/horizon-container stop ${device_num}

    let counter=counter+1
  done
}

