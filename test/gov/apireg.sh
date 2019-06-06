#!/bin/bash

EMAIL="foo@goo.com"

echo -e "\nBC setting is $BC"
echo -e "\nPATTERN setting is $PATTERN\n"

HA_DEVICE="false"
if [ "$HA" == "1" ]; then
  HA_DEVICE="true"
fi

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
  "pattern": "$pat",
  "ha": $HA_DEVICE
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

# ================================================================
# Set a node policy indicating the testing purpose of the node.

constraint2=""
if [ "$NONS" == "1" ]; then 
    constraint2="NONS == true"
else
    constraint2="NONS == false"
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

read -d '' newhznpolicy <<EOF
{
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
}
EOF



echo "Adding policy to the node using node/policy API"
RES=$(echo "$newhznpolicy" | curl -sS -X PUT -H "Content-Type: application/json" --data @- "$ANAX_API/node/policy")

if [ "$RES" == "" ]
then
  echo -e "$newhznpolicy \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "null" ]
then
    echo -e "$newhznpolicy \nresulted in incorrect response: $RES"

    echo -e "Wait for 30 seconds and try again"
    sleep 30
    RES=$(echo "$newhznpolicy" | curl -sS -X PUT -H "Content-Type: application/json" --data @- "$ANAX_API/node/policy")
    ERR=$(echo $RES | jq -r ".error")
    if [ "$ERR" != "null" ]
    then
        echo -e "$newhznpolicy \nsecond try resulted in incorrect response: $RES"
        exit 2
    else
        echo -e "found expected response in second try: $RES" 
    fi
else
  echo -e "found expected response: $RES"
fi



# =========================================================================
# Run some non-HA tests since we have HA setup right now. HA tests only
# run when testing services.

if [ "$HA" == "1" ]; then
    if [ "$PATTERN" = "sns" ] || [ "$PATTERN" = "spws" ]; then

        read -d '' pwsservice <<EOF
{
  "url": "https://bluehorizon.network/services/netspeed",
  "versionRange": "2.2.0",
  "organization": "IBM",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "User input variables",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "var1": "aString",
        "var2": 6,
        "var3": 12.3,
        "var4": ["abcdefg", "12345"],
        "var5": "override"
      }
    }
  ]
}
EOF

        echo "Testing non-HA service registration"

        echo -e "\n\n[D] service payload: $pwsservice"

        RES=$(echo "$pwsservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config")
        if [ "$RES" == "" ]
        then
            echo -e "$pwsservice \nresulted in empty response"
            exit 2
        fi

        ERR=$(echo $RES | jq -r ".error")
        SUB=${ERR:0:37}
        if [ "$SUB" != "services on an HA device must specify" ]
        then
            echo -e "$pwsservice \nresulted in incorrect response: $RES"
            exit 2
        else
            echo -e "found expected response: $RES"
        fi
 
    elif [ "$PATTERN" = "sgps" ] || [ "$PATTERN" = "sloc" ] || [ "$PATTERN" = "sall" ] || [ "$PATTERN" = "susehello" ] || [ "$PATTERN" = "cpu2msghub" ] || [ "$PATTERN" = "shelm" ]; then
        echo -e "Pattern $PATTERN is not supported with HA tests, only sns and spws are supported."
    fi
fi

# =======================================================================
# Setup services/workloads

echo -e "\nNo netspeed setting is $NONS"
if [ "$NONS" != "1" ]
then
  if [ "$HA" == "1" ]; then
    ./nsha_apireg.sh
  else
    ./ns_apireg.sh
  fi
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
  if [ "$HA" == "1" ]; then
    ./pwsha_apireg.sh
  else
    ./pws_apireg.sh
  fi
  if [ $? -ne 0 ]
  then
    exit 2
  fi
fi

echo -e "\nNo msghub setting is $NOMSGHUB"
if [ "$NOMSGHUB" != "1" ]
then
  if [ "$HA" == "1" ]; then
    exit 2
  else
    ./msghub_apireg.sh
  fi
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
