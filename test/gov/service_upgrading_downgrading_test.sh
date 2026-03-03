#!/bin/bash

# Enable debug tracing when DEBUG=1 or RUNNER_DEBUG=1 (GitHub Actions debug mode).
if [ "${DEBUG:-0}" = "1" ] || [ "${RUNNER_DEBUG:-0}" = "1" ]; then
    set -x
fi

CPU_IMAGE_NAME="${DOCKER_CPU_INAME}"
CPU_IMAGE_TAG="${DOCKER_CPU_TAG}"

echo "Testing service upgrading"

ADMIN_AUTH="e2edev@somecomp.com/e2edevadmin:e2edevadminpw"
KEY_TEST_DIR="/tmp/keytest"
export HZN_EXCHANGE_URL="${EXCH_APP_HOST}"

# shellcheck source=test/gov/utils.sh
source ./gov/utils.sh

CPU_URL="https://bluehorizon.network/service-cpu"
CPU_ORG="e2edev@somecomp.com"

# Ensure cpu service is up and running
echo "Waiting for old version of cpu service to be running..."
if ! WaitForService $CPU_URL $CPU_ORG; then exit 1; fi

# Save current cpu version for later comparing
# shellcheck disable=SC2154  # svc_inst is set by WaitForService() in utils.sh
current_svc_version=$(echo "$svc_inst" | jq -r '.version')
old_cpu_version="${current_svc_version}"
echo "Running ${CPU_ORG} ${CPU_URL} version $current_svc_version"

# Deploy newer version of service
CPU_URL="https://bluehorizon.network/service-cpu"
CPU_ORG="e2edev@somecomp.com"
CPU_VERS_NEW="1.0.3"

# cpu service - needed by the e2edev@somecomp.com/netspeed service as a dependency.
cat <<EOF >$KEY_TEST_DIR/svc_cpu.json
{
  "label":"CPU service",
  "description":"CPU service",
  "public":true,
  "url":"$CPU_URL",
  "version":"$CPU_VERS_NEW",
  "arch":"${ARCH}",
  "sharable":"singleton",
  "matchHardware":{},
  "userInput":[
    {
      "name":"cpu_var1",
      "label":"",
      "type":"string"
    }
  ],
  "deployment":{
    "services":{
      "cpu":{
        "image":"${CPU_IMAGE_NAME}:${CPU_IMAGE_TAG}"
      }
    }
  },
  "deploymentSignature":""
}
EOF
echo -e "Register new version ($CPU_VERS_NEW) of e2edev@somecomp.com/cpu service:"
if ! hzn exchange service publish -I -O -u $ADMIN_AUTH -o e2edev@somecomp.com -f $KEY_TEST_DIR/svc_cpu.json -k $KEY_TEST_DIR/*private.key -K $KEY_TEST_DIR/*public.pem
then
    echo -e "hzn exchange service publish failed for e2edev@somecomp.com/cpu."
    exit 2
fi

# Stop agreements in order to start the service upgrading
hzn agreement list | jq ' .[] | .current_agreement_id' | sed 's/"//g' | while read -r word ; do hzn agreement cancel "$word" ; done

# Ensure service is upgrading
echo "Waiting for new cpu version ${CPU_VERS_NEW} to be started..."
if ! WaitForService $CPU_URL $CPU_ORG $CPU_VERS_NEW; then hzn eventlog list; exit 1; fi

# Check upgrading logs were produced
if ! hzn eventlog list | grep -q "Start upgrading service $CPU_ORG/$CPU_URL from version $old_cpu_version to version $CPU_VERS_NEW."; then
    echo -e "'Start upgrading service' logs has not been found"
    hzn eventlog list
    exit 2
fi
if ! hzn eventlog list | grep -q "Complete upgrading service $CPU_ORG/$CPU_URL from version $old_cpu_version to version $CPU_VERS_NEW."; then
    echo -e "'Complete upgrading service' logs has not been found"
    hzn eventlog list
    exit 2
fi

echo "Testing service downgrading"

# Deploy newer version of service with deployment error
CPU_URL="https://bluehorizon.network/service-cpu"
CPU_ORG="e2edev@somecomp.com"
CPU_VERS_ERR="1.0.4"
cat <<EOF >$KEY_TEST_DIR/svc_cpu.json
{
  "label":"CPU service",
  "description":"CPU service",
  "public":true,
  "url":"$CPU_URL",
  "version":"$CPU_VERS_ERR",
  "arch":"${ARCH}",
  "sharable":"singleton",
  "matchHardware":{},
  "userInput":[
    {
      "name":"cpu_var1",
      "label":"",
      "type":"string"
    }
  ],
  "deployment":{
    "services":{
      "cpu":{
        "image":"${CPU_IMAGE_NAME}:${CPU_IMAGE_TAG}",
        "binds":["/tmp:/hosttmp", ""]
      }
    }
  },
  "deploymentSignature":""
}
EOF
echo -e "Register new version ($CPU_VERS_ERR) of e2edev@somecomp.com/cpu service with an error in deployment:"
if ! hzn exchange service publish -I -O -u $ADMIN_AUTH -o e2edev@somecomp.com -f $KEY_TEST_DIR/svc_cpu.json -k $KEY_TEST_DIR/*private.key -K $KEY_TEST_DIR/*public.pem
then
    echo -e "hzn exchange service publish failed for e2edev@somecomp.com/cpu."
    exit 2
fi

# Stop agreements in order to start the service upgrading and downgrading because of error
hzn agreement list | jq ' .[] | .current_agreement_id' | sed 's/"//g' | while read -r word ; do hzn agreement cancel "$word" ; done

# Wait for the old version of the cpu service to stop
sleep 5

echo "Display agreement status, there should be none:"
hzn agreement list

# Ensure service is upgrading/downgrading
echo "Waiting for cpu service to be upgraded and downgraded because of an error..."
ERROR_SERVICE="true"
if ! WaitForService $CPU_URL $CPU_ORG $CPU_VERS_ERR $ERROR_SERVICE; then hzn eventlog list; exit 1; fi

if ! WaitForService $CPU_URL $CPU_ORG $CPU_VERS_NEW; then hzn eventlog list; exit 1; fi

# Check upgrading logs were produced
if ! hzn eventlog list | grep -q "Start upgrading service $CPU_ORG/$CPU_URL from version $CPU_VERS_NEW to version $CPU_VERS_ERR."; then
    echo -e "'Start upgrading service' logs has not been found"
    hzn eventlog list
    exit 2
fi

# Check downgrading logs were produced
if ! hzn eventlog list | grep -q "Start downgrading service $CPU_ORG/$CPU_URL version $CPU_VERS_ERR"; then
    echo -e "'Start downgrading service' logs has not been found"
    hzn eventlog list
    exit 2
fi

# Remove service with deployment error
if ! hzn exchange service remove -u "$ADMIN_AUTH" -o e2edev@somecomp.com -f "$CPU_ORG/bluehorizon.network-service-cpu_${CPU_VERS_ERR}_${ARCH}"
then
    echo -e "hzn exchange service remove failed for $CPU_ORG/cpu with deployment error"
    exit 2
fi
