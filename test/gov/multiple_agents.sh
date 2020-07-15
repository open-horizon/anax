#!/bin/bash
COUNTER=0
if [ ${MUL_AGENTS} -lt 5 ]; then
  NUM_AGENTS=${MUL_AGENTS}
else
  NUM_AGENTS=4
fi
while [ ${COUNTER} -lt ${NUM_AGENTS} ]; do
  echo ${COUNTER}
  AGENT_PORT=$((8512 + ${COUNTER}))
  DEVICE_NUM=$((6 + ${COUNTER}))
  echo ${AGENT_PORT}
  echo ${DEVICE_NUM}
  cat /certs/css.crt > /tmp/css.crt
  echo "HZN_EXCHANGE_URL=${EXCH_APP_HOST}" > /tmp/horizon
  echo "HZN_FSS_CSSURL=${CSS_URL}" >> /tmp/horizon
  echo "HZN_DEVICE_ID=anaxdevice${DEVICE_NUM}" >> /tmp/horizon
  echo "HZN_MGMT_HUB_CERT_PATH=/tmp/css.crt" >> /tmp/horizon
  echo "HZN_AGENT_PORT=${AGENT_PORT}" >> /tmp/horizon
  echo "E2E_NETWORK=e2edev_test_network" >> /tmp/horizon
  #export HC_DOCKER_TAG=${DOCKER_EXCH_TAG};
  export zzHC_DONT_PULL=1;
  export HZN_ORG_ID="e2edev@somecomp.com";
  export HZN_EXCHANGE_URL=${EXCH_APP_HOST};
  export HZN_EXCHANGE_USER_AUTH="e2edev@somecomp.com/e2edevadmin:e2edevadminpw";
  export HZN_FSS_CSSURL=${CSS_URL};
  export HZN_AIC="/tmp/anax-in-container/horizon-container";
  export HORIZON_NUM=${DEVICE_NUM};
  #export HC_DOCKER_TAG=2.26.0
  ${HZN_AIC} start  ${HORIZON_NUM} /tmp/horizon
  sleep 10
  docker cp /tmp/exports_list horizon$HORIZON_NUM:/tmp/exports_list
  docker cp /root/. horizon$HORIZON_NUM:/root/.
  sleep 10
  docker exec horizon$HORIZON_NUM /bin/bash -c /root/run_agent.sh
  let COUNTER=COUNTER+1
done
