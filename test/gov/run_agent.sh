source /tmp/exports_list
EX_IP_GATEWAY=$(docker inspect exchange-api | jq -r '.[].NetworkSettings.Networks.e2edev_test_network.Gateway')
EX_HOST_PORT=$(docker inspect exchange-api | jq -r '.[].NetworkSettings.Ports."8080/tcp"[].HostPort')
export HZN_EXCHANGE_URL="http://$EX_IP_GATEWAY:$EX_HOST_PORT/v1"
export EXCH_APP_HOST="http://$EX_IP_GATEWAY:$EX_HOST_PORT/v1"
AGENT_PORT=$(docker port "$(cat /proc/self/cgroup | grep "docker" | sed s/\\//\\n/g | tail -1)" | awk -F'/' '{print $1}')
echo ${AGENT_PORT}
DEVICE_NUM=$((${AGENT_PORT} - 8512 + 6))
echo ${DEVICE_NUM}
export USER=anax1
export PASS=anax1pw
export DEVICE_ID="anaxdevice${DEVICE_NUM}"
export DEVICE_NAME="anaxdevice${DEVICE_NUM}"
export DEVICE_ORG="e2edev@somecomp.com"
export TOKEN="abcdefg"
export HA_DEVICE="false"
echo ${DEVICE_NAME}
export HZN_AGENT_PORT=${AGENT_PORT}
export ANAX_API="http://localhost:${HZN_AGENT_PORT}"
export EXCH=$HZN_EXCHANGE_URL

export PATTERN="sall"
export NOK8S=1
export constraint2="NOK8S == true"
export PARTNERID="$DEVICE_ORG/anaxdevice${DEVICE_NUM}"
export HZN_MGMT_HUB_CERT_PATH="/tmp/css.crt"

./apireg.sh > agent_debug.log
