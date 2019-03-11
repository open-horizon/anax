#!/bin/bash

echo -e "Registering services and patterns for msghub"

EXCH_URL="http://${EXCH_APP_HOST:-172.17.0.1}:8080/v1"
IBM_ADMIN_AUTH="IBM/ibmadmin:ibmadminpw"
E2EDEV_ADMIN_AUTH="e2edev@somecomp.com/e2edevadmin:e2edevadminpw"
MH_SAMPLES_PATH="/root/examples/edge/msghub"


# This is IBM Message Hub you are sending data to
if [ -z "$MSGHUB_BROKER_URL" ] || [ -z "$MSGHUB_API_KEY" ] ; then 
    echo -e "MSGHUB_BROKER_URL or MSGHUB_API_KEY is not defined. Cannot perform msghub testing."
    exit 2
fi

KEY_TEST_DIR="/tmp/keytest"
mkdir -p $KEY_TEST_DIR

cd $KEY_TEST_DIR
ls *.key &> /dev/null
if [ $? -eq 0 ]
then
    echo -e "Using existing key"
else
  echo -e "Generate new signing keys:"
  hzn key create -l 4096 e2edev@somecomp.com e2edev@gmail.com
  if [ $? -ne 0 ]
  then
    echo -e "hzn key create failed."
    exit 2
  fi
fi

echo -e "Copy public key into anax folder:"
cp $KEY_TEST_DIR/*public.pem /root/.colonus/. &> /dev/null


# clone the open-horizon/examples repository, remove old code first
echo "-e clone the open-horizon/examples to get the poc templates."
cd /root
rm -Rf examples
git clone https://github.com/open-horizon/examples.git
if [ $? -ne 0 ]
then
    echo -e "Failed to clone open-horizon/examples."
    exit 2
fi

## setup env vars
export HZN_EXCHANGE_URL="http://${EXCH_APP_HOST:-172.17.0.1}:8080/v1"

export HZN_ORG_ID=IBM
export MYDOMAIN="bluehorizon.network"

export ARCH=amd64   # arch of your edge node: amd64, or arm for Raspberry Pi, or arm64 for TX2
export CPU2MSGHUB_NAME=cpu2msghub   # the name of the service, used in the docker image path and in the service url
export CPU2MSGHUB_VERSION=1.2.2   # the service version, and also used as the tag for the docker image. Must be in OSGI version format.

# dependencies
export CPU_NAME=cpu   # the name of the service, used in the docker image path and in the service url
export CPU_VERSION=1.2.2   # the service version, and also used as the tag for the docker image. Must be in OSGI version format.
export GPS_NAME=gps   # the name of the service, used in the docker image path and in the service url
export GPS_VERSION=2.0.3   # the service version, and also used as the tag for the docker image. Must be in OSGI version format.

export DOCKER_HUB_ID=openhorizon   

echo -e "Register $CPU2MSGHUB_NAME service $CPU_VERSION"
hzn exchange service publish -I -u $IBM_ADMIN_AUTH -o IBM -f "$MH_SAMPLES_PATH/cpu2msghub/horizon/service.definition.json" -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for $CPU2MSGHUB_NAME."
    unset HZN_EXCHANGE_URL
    exit 2
fi

echo -e "Register cpu2msghub pattern $VERS:"
hzn exchange pattern publish -u $E2EDEV_ADMIN_AUTH -o e2edev@somecomp.com  -f "$MH_SAMPLES_PATH/cpu2msghub/horizon/pattern/cpu2msghub.json" -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn exchange pattern publish failed for cpu2msghub."
    unset HZN_EXCHANGE_URL
    exit 2
fi

unset HZN_EXCHANGE_URL

