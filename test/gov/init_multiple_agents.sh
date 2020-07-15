#!/bin/bash
cat /certs/css.crt > /tmp/css.crt
echo "HZN_EXCHANGE_URL=${EXCH_APP_HOST}" > /tmp/horizon
echo "HZN_FSS_CSSURL=${CSS_URL}" >> /tmp/horizon
echo "HZN_DEVICE_ID=anaxdevice6" >> /tmp/horizon
echo "HZN_MGMT_HUB_CERT_PATH=/tmp/css.crt" >> /tmp/horizon
echo "HZN_AGENT_PORT=8512" >> /tmp/horizon
echo "E2E_NETWORK=e2edev_test_network" >> /tmp/horizon
#export HC_DOCKER_TAG=${DOCKER_EXCH_TAG};
export zzHC_DONT_PULL=1;
export HZN_ORG_ID="e2edev@somecomp.com";
export HZN_EXCHANGE_URL=${EXCH_APP_HOST};
export HZN_EXCHANGE_USER_AUTH="e2edev@somecomp.com/e2edevadmin:e2edevadminpw";
export HZN_FSS_CSSURL=${CSS_URL};
export HZN_AIC="/tmp/anax-in-container/horizon-container";
export HORIZON_NUM=6;
#export HC_DOCKER_TAG=2.26.0
${HZN_AIC} start  ${HORIZON_NUM} /tmp/horizon
sleep 10
docker cp /tmp/exports_list horizon$HORIZON_NUM:/tmp/exports_list
docker cp /root/. horizon$HORIZON_NUM:/root/.
