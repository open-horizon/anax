#!/bin/bash

EMAIL="foo@goo.com"

echo -e "\nBC setting is $BC"
echo -e "\nPATTERN setting is $PATTERN\n"

HA_DEVICE="false"
if [ "$HA" == "1" ]; then
  HA_DEVICE="true"
fi

echo "Calling node API"

read -d '' newhzndevice <<EOF
{
  "id": "$DEVICE_ID",
  "token": "$TOKEN",
  "name": "$DEVICE_NAME",
  "organization": "$ORG",
  "pattern": "$PATTERN",
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
# Set global attributes

read -d '' locationattribute <<EOF
{
  "type": "LocationAttributes",
  "label": "Registered Location Facts",
  "publishable": false,
  "host_only": false,
  "mappings": {
    "lat": 41.921766,
    "lon": -73.894224,
    "location_accuracy_km": 0.5
  }
}
EOF

echo -e "\n\n[D] location payload: $locationattribute"

echo "Setting workload independent location attributes"

ERR=$(echo "$locationattribute" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/attribute" | jq -r '.error')
if [ "$ERR" != "null" ]; then
  echo -e "error occured: $ERR"
  exit 2
fi

# =========================================================================
# Run some non-HA tests since we have HA setup right now. HA tests only
# run when testing services.

if [ "$HA" == "1" ]; then
    if [ "$PATTERN" = "sns" ] || [ "$PATTERN" = "spws" ]; then

        read -d '' pwsservice <<EOF
{
  "url": "https://bluehorizon.network/services/no-such-service",
  "organization": "e2edev",
  "version": "1.0.0",
  "attributes": [
    {
      "type": "ComputeAttributes",
      "label": "Compute Resources",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "ram": 256,
        "cpus": 1
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

    elif [ "$PATTERN" = "sgps" ] || [ "$PATTERN" = "sloc" ] || [ "$PATTERN" = "sall" ] || [ "$PATTERN" = "susehello" ] || [ "$PATTERN" = "cpu2msghub" ]; then
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
