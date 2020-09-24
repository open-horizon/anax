
echo "Testing node error surfacing to exchange"

IBM_ADMIN_AUTH="IBM/ibmadmin:ibmadminpw"
KEY_TEST_DIR="/tmp/keytest"
export HZN_EXCHANGE_URL="${EXCH_APP_HOST}"

echo "Waiting to ensure that all surfaced errors from previous tests are resolved before proceeding..."
NUM_ERRS=1
TIMEOUT=0
while [[ $NUM_ERRS -ge 1 ]] && [[ $TIMEOUT -le 25 ]]
do
  ERRS=$(hzn eventlog surface)
  NUM_ERRS=$(echo ${ERRS} | jq -r '. | length')
  sleep 5s
  ((TIMEOUT++))
  if [[ $TIMEOUT == 26 ]]; then echo -e "surface errors failed to resolve"; exit 2; fi
done

echo -e "All surfaced errors resolved, test can proceed."

# cpu service - needed by the hzn dev tests and the location top level service as a 3rd level dependency.
VERS="1.2.2"
cat <<EOF >$KEY_TEST_DIR/svc_cpu.json
{
  "label":"CPU service",
  "description":"CPU service",
  "public":true,
  "url":"https://bluehorizon.network/service-cpu",
  "version":"$VERS",
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
        "binds":["/tmp:/hosttmp",""]
      }
    }
  },
  "deploymentSignature":""
}
EOF
echo -e "Re-register IBM/cpu $VERS service with deployment error:"
hzn exchange service publish -I -O -u $IBM_ADMIN_AUTH -o IBM -f $KEY_TEST_DIR/svc_cpu.json -k $KEY_TEST_DIR/*private.key -K $KEY_TEST_DIR/*public.pem
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for IBM/cpu."
    exit 2
fi

hzn agreement list | jq ' .[] | .current_agreement_id' | sed 's/"//g' | while read word ; do hzn agreement cancel $word ; done

echo "Waiting on error to surface"
NUM_ERRS=0
TIMEOUT=0
while [[ $NUM_ERRS -le 0 ]] && [[ $TIMEOUT -le 15 ]]
do
  ERRS=$(hzn eventlog surface)
  NUM_ERRS=$(echo ${ERRS} | jq -r '. | length')
  sleep 5s
  ((TIMEOUT++))
  if [[ $TIMEOUT == 16 ]]; then echo -e "surface error failed to appear"; exit 2; fi
done

echo -e "Found surfaced error $ERRS"

# cpu service - needed by the hzn dev tests and the location top level service as a 3rd level dependency.
VERS="1.2.4"
cat <<EOF >$KEY_TEST_DIR/svc_cpu.json
{
  "label":"CPU service",
  "description":"CPU service",
  "public":true,
  "url":"https://bluehorizon.network/service-cpu",
  "version":"$VERS",
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
echo -e "Re-register IBM/cpu $VERS service without deployment error:"
hzn exchange service publish -I -O -u $IBM_ADMIN_AUTH -o IBM -f $KEY_TEST_DIR/svc_cpu.json -k $KEY_TEST_DIR/*private.key -K $KEY_TEST_DIR/*public.pem
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for IBM/cpu."
    exit 2
fi

hzn agreement list | jq ' .[] | .current_agreement_id' | sed 's/"//g' | while read word ; do hzn agreement cancel $word ; done

echo "Waiting on the surfaced error to be resolved"
NUM_ERRS=1
TIMEOUT=0
while [[ $NUM_ERRS -ge 1 ]] && [[ $TIMEOUT -le 25 ]]
do
  ERRS=$(hzn eventlog surface)
  NUM_ERRS=$(echo ${ERRS} | jq -r '. | length')
  sleep 5s
  ((TIMEOUT++))
  if [[ $TIMEOUT == 26 ]]; then echo -e "surface error failed to resolve"; exit 2; fi
done

echo -e "All surfaced errors resolved"
