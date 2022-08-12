#!/bin/bash

echo -e "\nBC setting is $BC"
echo -e "\nPATTERN setting is $PATTERN\n"

if [ "$HA" == "1" ]; then
    if [ "$PATTERN" = "sgps" ] || [ "$PATTERN" = "sloc" ] || [ "$PATTERN" = "sall" ] || [ "$PATTERN" = "susehello" ] || [ "$PATTERN" = "shelm" ]; then
        echo -e "Pattern $PATTERN is not supported with HA tests, only sns and spws are supported."
        exit 2
    fi
fi

EMAIL="foo@goo.com"

echo "Calling node API"

pat=$PATTERN
if [[ "$PATTERN" != "" ]]; then
    pat="e2edev@somecomp.com/$PATTERN"
fi


read -d '' newhzndevice <<EOF
{
  "id": "$DEVICE_ID",
  "token": "$TOKEN",
  "name": "$DEVICE_NAME",
  "organization": "$DEVICE_ORG",
  "pattern": "$pat"
}
EOF

while :
do
  echo -e "\n[D] hzndevice payload: $newhzndevice"

  echo "Updating horizon with id and token"

  ERR=$(echo "$newhzndevice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/node" | jq -r '.error')
  echo -e "the error $ERR"
  if [ "$ERR" == "null" ]
  then
    break
  fi

  if [ "${ERR:0:19}" == "Node is restarting," ]
  then
    sleep 5
  else
    echo -e "error occured: $ERR"
    exit 2
  fi

done

echo "Waiting for node to sync up with exchange"
sleep 30

# ================================================================
# Set a node policy indicating the testing purpose of the node.

constraint2=""
if [ "$NONS" == "1" ]; then 
    constraint2="NONS==true"
else
    constraint2="NONS==false"
fi
if [ "$NOGPS" == "1" ]; then 
    constraint2="$constraint2 || NOGPS == true"
else
    constraint2="$constraint2 || NOGPS == false"
fi
if [ "$NOLOC" == "1" ]; then 
    constraint2="$constraint2 || NOLOC == true"
else
    constraint2="$constraint2 || NOLOC == false"
fi
if [ "$NOPWS" == "1" ]; then 
    constraint2="$constraint2 || NOPWS == true"
else
    constraint2="$constraint2 || NOPWS == false"
fi
if [ "$NOHELLO" == "1" ]; then 
    constraint2="$constraint2 || NOHELLO == true"
else
    constraint2="$constraint2 || NOHELLO == false"
fi
if [ "$NOK8S" == "1" ]; then 
    constraint2="$constraint2 || NOK8S == true"
else
    constraint2="$constraint2 || NOK8S == false"
fi

constraint3=""
if [ "$NOAGENTAUTO" == "1" ]; then 
    constraint3="NOAGENTAUTO==true"
else
    constraint3="NOAGENTAUTO==false"
fi

read -d '' newhznpolicy <<EOF
{
  "deployment": {
    "properties": [
      {
        "name":"purpose","value":"network-testing"
      },
      {
        "name":"group","value":"bluenode"
      }
    ],
    "constraints": [
      "iame2edev == true",
      "$constraint2"
    ]
  },
  "management": {
    "properties": [
      {
        "name":"purpose","value":"nmp-testing"
      },
      {
        "name":"group","value":"bluenode"
      }
    ],
    "constraints": [
      "iame2edev == true",
      "$constraint3"
    ]
  }
}
EOF


echo "Adding policy to the node using node/policy API"
RES=$(echo "$newhznpolicy" | curl -sS -X PUT -w "%{http_code}" -H "Content-Type: application/json" --data @- "$ANAX_API/node/policy")

if [ "$RES" == "" ]
then
  echo -e "$newhznpolicy \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r '.' | tail -1)
if [ "$ERR" != "201" ]
then
    echo -e "$newhznpolicy \nresulted in incorrect response: $RES"

    echo -e "Wait for 30 seconds and try again"
    sleep 30
    RES=$(echo "$newhznpolicy" | curl -sS -X PUT -H "Content-Type: application/json" --data @- "$ANAX_API/node/policy")
    ERR=$(echo $RES | jq -r '.' | tail -1)
    if [ "$ERR" != "201" ]
    then
        echo -e "$newhznpolicy \nsecond try resulted in incorrect response: $RES"
        exit 2
    else
        echo -e "found expected response in second try: $RES" 
    fi
else
  echo -e "found expected response: $RES"
fi


# =======================================================================
# Setup some services/workloads
echo -e "\nNo netspeed setting is $NONS"
if [ "$NONS" != "1" ]
then
  ./ns_apireg.sh
  if [ $? -ne 0 ]
  then
    exit 2
  fi
fi

echo -e "\nNo location setting is $NOLOC"
if [ "$NOLOC" != "1" ]
then
    ./loc2_apireg.sh
    if [ $? -ne 0 ]
    then
        exit 2
    fi
fi

echo -e "\nNo gpstest setting is $NOGPS"
if [ "$NOGPS" != "1" ]
then
    ./gpstest_apireg.sh
    if [ $? -ne 0 ]
    then
        exit 2
   fi
fi

echo -e "\nNo pws setting is $NOPWS"
if [ "$NOPWS" != "1" ]
then
  ./pws_apireg.sh
  if [ $? -ne 0 ]
  then
    exit 2
  fi
fi

./hello_apireg.sh
if [ $? -ne 0 ]
then
  exit 2
fi

echo -e "\nCompleting node registration"
./cs_apireg.sh

if [ $? -ne 0 ]
then
  echo -e "Error setting up to run workloads"
  TESTFAIL="1"
  exit 2
else
  echo -e "Workload setup SUCCESSFUL"
fi

echo -e "\n\n[D] all registered attributes:\n"
curl -sS -H "Content-Type: application/json" "$ANAX_API/attribute" | jq -r '.'
