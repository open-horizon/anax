#!/bin/bash

# =================================================================
# Run tests on the Configstate API

# transition from configuring to configuring
echo "Testing Configstate API"

read -d '' newhzndevice <<EOF
{
  "state": "configuring"
}
EOF

echo "Testing for noop state change in configstate API"
RES=$(echo "$newhzndevice" | curl -sS -X PUT -H "Content-Type: application/json" --data @- "$ANAX_API/node/configstate")

if [ "$RES" == "" ]
then
  echo -e "$newhzndevice \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "null" ]
then
  echo -e "$newhzndevice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# ====================================================================================
# transition from configuring to configured

echo "Testing Configstate API"

read -d '' newhzndevice <<EOF
{
  "state": "configured"
}
EOF

echo "Testing for transition to configured in configstate API"
RES=$(echo "$newhzndevice" | curl -sS -X PUT -H "Content-Type: application/json" --data @- "$ANAX_API/node/configstate")

if [ "$RES" == "" ]
then
  echo -e "$newhzndevice \nresulted in empty response"
  exit 2
fi

# The gps pattern doesnt require MS and workload config to have been done, likewise for the sgps service. We expect no error
# for this test in that case. For all other patterns, we expect an error.
if [ "$PATTERN" == "gps" ] || [ "$PATTERN" == "sgps" ] || [ "$PATTERN" == "" ]
then

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "null" ]
then
  echo -e "$newhzndevice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# check for the error when running everything else
else

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" == "null" ]
then
  echo -e "$newhzndevice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

fi

# ====================================================================================
# transition from configured to configuring
echo "Testing Configstate API"

read -d '' newhzndevice <<EOF
{
  "state": "configuring"
}
EOF

echo "Testing for transition to configuring in configstate API"
RES=$(echo "$newhzndevice" | curl -sS -X PUT -H "Content-Type: application/json" --data @- "$ANAX_API/node/configstate")

if [ "$RES" == "" ]
then
  echo -e "$newhzndevice \nresulted in empty response"
  exit 2
fi

# The gps pattern doesnt require MS and workload config to have been done, likewise for the sgps service. We expect no error
# for this test in that case. For all other patterns, we expect an error.
if [ "$PATTERN" == "gps" ] || [ "$PATTERN" == "sgps" ] || [ "$PATTERN" == "" ]
then

ERR=$(echo $RES | jq -r ".error")
if [ "${ERR:0:62}" != "Transition from 'configured' to 'configuring' is not supported" ]
then
  echo -e "$newhzndevice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

else

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "null" ]
then
  echo -e "$newhzndevice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

fi
