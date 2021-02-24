
echo "Testing node error surfacing to exchange"

ADMIN_AUTH="e2edev@somecomp.com/e2edevadmin:e2edevadminpw"
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

# e2edev@somecomp.com/cpu service - needed by e2edev@somecomp.com/netspeed service.
VERS="1.0.1"
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
        "image":"openhorizon/amd64_cpu:1.2.2",
        "binds":["/tmp:/hosttmp",""]
      }
    }
  },
  "deploymentSignature":""
}
EOF
echo -e "Re-register e2edev@somecomp.com/cpu $VERS service with a deployment error:"
hzn exchange service publish -I -O -u $ADMIN_AUTH -o e2edev@somecomp.com -f $KEY_TEST_DIR/svc_cpu.json -k $KEY_TEST_DIR/*private.key -K $KEY_TEST_DIR/*public.pem
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for e2edev@somecomp.com/cpu."
    exit 2
fi

hzn agreement list | jq ' .[] | .current_agreement_id' | sed 's/"//g' | while read word ; do hzn agreement cancel $word ; done

echo "Waiting on error to surface"
NUM_ERRS=0
TIMEOUT=0
while [[ $NUM_ERRS -le 0 ]] && [[ $TIMEOUT -le 80 ]]
do
  ERRS=$(hzn eventlog surface)
  NUM_ERRS=$(echo ${ERRS} | jq -r '. | length')
  sleep 1s
  ((TIMEOUT++))
  if [[ $TIMEOUT == 80 ]]; then echo -e "surface error failed to appear"; hzn eventlog list; docker ps -a; docker network ls; exit 2; fi
done

echo -e "Found surfaced error $ERRS"

# e2edev@somecomp.com/cpu service - needed by e2edev@somecomp.com/netspeed service.
VERS="1.0.2"
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
        "image":"openhorizon/amd64_cpu:1.2.2"
      }
    }
  },
  "deploymentSignature":""
}
EOF
echo -e "Re-register e2edev@somecomp.com/cpu $VERS service without a deployment error:"
hzn exchange service publish -I -O -u $ADMIN_AUTH -o e2edev@somecomp.com -f $KEY_TEST_DIR/svc_cpu.json -k $KEY_TEST_DIR/*private.key -K $KEY_TEST_DIR/*public.pem
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for e2edev@somecomp.com/cpu."
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
