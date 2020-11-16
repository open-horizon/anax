
echo "Testing service upgrading"

IBM_ADMIN_AUTH="IBM/ibmadmin:ibmadminpw"
KEY_TEST_DIR="/tmp/keytest"
export HZN_EXCHANGE_URL="${EXCH_APP_HOST}"

# $1 - Service url to wait for
# $2 - Service org to wait for
# $3 - Service version to wait for (optional)
function WaitForService() {
  current_svc_version=""
  TIMEOUT=0
  while [[ $TIMEOUT -le 25 ]]
  do
    svc_inst=$(curl -s $ANAX_API/service | jq -r ".instances.active[] | select (.ref_url == \"$1\") | select (.organization == \"$2\")")
    if [ $? -ne 0 ]; then
        echo -e "${PREFIX} failed to get $1 service instace. ${svc_inst}"
        exit 2
    fi
    current_svc_version=$(echo "$svc_inst" | jq -r '.version')

    if [ "$current_svc_version" == "" ] || ([ "$3" != "" ] && [ "$current_svc_version" != "$3" ]); then
        sleep 5s
        ((TIMEOUT++))
    else
        break
    fi

    if [[ $TIMEOUT == 26 ]]; then echo -e "Timeout for waiting to $1 service to start"; exit 2; fi
  done
}

CPU_URL="https://bluehorizon.network/service-cpu"
CPU_ORG="IBM"

# Ensure cpu service is up and running
echo "Waiting for old version of cpu service to be running..."
WaitForService $CPU_URL $CPU_ORG
if [ $? -ne 0 ]; then exit $?; fi

# Save current cpu version for later comparing
old_cpu_version="$current_svc_version"

# Deploy newer version of service
CPU_URL="https://bluehorizon.network/service-cpu"
CPU_ORG="IBM"
CPU_VERS_NEW="1.2.5"

# cpu service - needed by the hzn dev tests and the location top level service as a 3rd level dependency.
cat <<EOF >$KEY_TEST_DIR/svc_cpu.json
{
  "label":"CPU service",
  "description":"CPU service",
  "public":true,
  "url":"$CPU_URL",
  "version":"$CPU_VERS_NEW",
  "arch":"amd64",
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
        "image":"openhorizon/example_ms_x86_cpu:1.2.2",
        "binds":["/tmp:/hosttmp"]
      }
    }
  },
  "deploymentSignature":""
}
EOF
echo -e "Register new version ($CPU_VERS_NEW) of IBM/cpu service:"
hzn exchange service publish -I -O -u $IBM_ADMIN_AUTH -o IBM -f $KEY_TEST_DIR/svc_cpu.json -k $KEY_TEST_DIR/*private.key -K $KEY_TEST_DIR/*public.pem
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for IBM/cpu."
    exit 2
fi

# Stop agreements in order to start the service upgrading
hzn agreement list | jq ' .[] | .current_agreement_id' | sed 's/"//g' | while read word ; do hzn agreement cancel $word ; done

# Ensure service is upgrading
echo "Waiting for new cpu version to be started..."
WaitForService $CPU_URL $CPU_ORG $CPU_VERS_NEW
if [ $? -ne 0 ]; then exit $?; fi

# Check upgrading logs were produced
ret=$(hzn eventlog list | grep "Start upgrading service $CPU_ORG/$CPU_URL from version $old_cpu_version to version $CPU_VERS_NEW.")
if [ $? -ne 0 ]; then
    echo -e "'Start upgrading service' logs has not been found"
    exit 2
fi
ret=$(hzn eventlog list | grep "Complete upgrading service $CPU_ORG/$CPU_URL from version $old_cpu_version to version $CPU_VERS_NEW.")
if [ $? -ne 0 ]; then
    echo -e "'Complete upgrading service' logs has not been found"
    exit 2
fi

echo "Testing service downgrading"

# Deploy newer version of service with deployment error
CPU_URL="https://bluehorizon.network/service-cpu"
CPU_ORG="IBM"
CPU_VERS_ERR="1.2.6"
cat <<EOF >$KEY_TEST_DIR/svc_cpu.json
{
  "label":"CPU service",
  "description":"CPU service",
  "public":true,
  "url":"$CPU_URL",
  "version":"$CPU_VERS_ERR",
  "arch":"amd64",
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
        "image":"openhorizon/example_ms_x86_cpu:1.2.2",
        "binds":["/tmp:/hosttmp", ""]
      }
    }
  },
  "deploymentSignature":""
}
EOF
echo -e "Register new version ($CPU_VERS_ERR) of IBM/cpu service with an error in deployment:"
hzn exchange service publish -I -O -u $IBM_ADMIN_AUTH -o IBM -f $KEY_TEST_DIR/svc_cpu.json -k $KEY_TEST_DIR/*private.key -K $KEY_TEST_DIR/*public.pem
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for IBM/cpu."
    exit 2
fi

# Stop agreements in order to start the service upgrading and downgrading because of error
hzn agreement list | jq ' .[] | .current_agreement_id' | sed 's/"//g' | while read word ; do hzn agreement cancel $word ; done

# Wait 10 seconds to let the old version of cpu service to be stopped
sleep 10

# Ensure service is upgrading/downgrading
echo "Waiting for cpu service to be upgraded and downgraded because od an error..."
WaitForService $CPU_URL $CPU_ORG $CPU_VERS_NEW
if [ $? -ne 0 ]; then exit $?; fi

# Check upgrading logs were produced
ret=$(hzn eventlog list | grep "Start upgrading service $CPU_ORG/$CPU_URL from version $CPU_VERS_NEW to version $CPU_VERS_ERR.")
if [ $? -ne 0 ]; then
    echo -e "'Start upgrading service' logs has not been found"
    exit 2
fi

# Check downgrading logs were produced
ret=$(hzn eventlog list | grep "Start downgrading service $CPU_ORG/$CPU_URL version $CPU_VERS_ERR")
if [ $? -ne 0 ]; then
    echo -e "'Start downgrading service' logs has not been found"
    exit 2
fi

# Remove service with deployment error
hzn exchange service remove -u $IBM_ADMIN_AUTH -o IBM -f $CPU_ORG/bluehorizon.network-service-cpu_1.2.6_amd64
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service remove failed for $CPU_ORG/cpu with deployment error"
    exit 2
fi
