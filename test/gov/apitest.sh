#!/bin/bash

EMAIL="foo@goo.com"

TESTFAIL="0"

ORG="e2edev@somecomp.com"

# =================================================================
# Run error tests on the node API

# missing org
echo "Testing node API"

read -d '' newhzndevice <<EOF
{
  "id": "$DEVICE_ID",
  "name": "$DEVICE_NAME",
  "token": "$TOKEN"
}
EOF

echo "Testing for missing organization in node API"
RES=$(echo "$newhzndevice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/node")

if [ "$RES" == "" ]
then
  echo -e "$newhzndevice \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "null and must not be" ]
then
  echo -e "$newhzndevice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# undefined org
read -d '' newhzndevice <<EOF
{
  "id": "$DEVICE_ID",
  "name": "$DEVICE_NAME",
  "token": "$TOKEN",
  "organization": "fred"
}
EOF

echo "Testing for undefined organization in node API"
RES=$(echo "$newhzndevice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/node")

if [ "$RES" == "" ]
then
  echo -e "$newhzndevice \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "${ERR:0:39}" != "organization fred not found in exchange" ]
then
  echo -e "$newhzndevice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# undefined pattern
read -d '' newhzndevice <<EOF
{
  "id": "$DEVICE_ID",
  "name": "$DEVICE_NAME",
  "token": "$TOKEN",
  "organization": "$ORG",
  "pattern": "fred"
}
EOF

echo "Testing for undefined pattern in node API"
RES=$(echo "$newhzndevice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/node")

if [ "$RES" == "" ]
then
  echo -e "$newhzndevice \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "${ERR:0:54}" != "pattern e2edev@somecomp.com/fred not found in exchange" ]
then
  echo -e "$newhzndevice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# =================================================================
# Run error test on the Configstate API

# node not registered yet
echo "Testing Configstate API"

read -d '' newhzndevice <<EOF
{
  "state": "configuring"
}
EOF

echo "Testing for not registered device in configstate API"
RES=$(echo "$newhzndevice" | curl -sS -X PUT -H "Content-Type: application/json" --data @- "$ANAX_API/node/configstate")

if [ "$RES" == "" ]
then
  echo -e "$newhzndevice \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "${ERR:0:34}" != "Exchange registration not recorded" ]
then
  echo -e "$newhzndevice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# =================================================================
# Run error tests on the node/policy API

# Missing input on PUT and POST
# First try POST
echo "Testing for missing input on node/policy API"
RES=$(curl -sS -X POST -H "Content-Type: application/json" "$ANAX_API/node/policy")

if [ "$RES" == "" ]
then
  echo -e "Missing input in node/policy test resulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "${ERR:0:36}" != "Input body could not be deserialized" ]
then
  echo -e "Missing input in node/policy test resulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# Then try PUT
echo "Testing for missing input on node/policy API"
RES=$(curl -sS -X PUT -H "Content-Type: application/json" "$ANAX_API/node/policy")

if [ "$RES" == "" ]
then
  echo -e "Missing input in node/policy test resulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "${ERR:0:36}" != "Input body could not be deserialized" ]
then
  echo -e "Missing input in node/policy test resulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# Incorrect input (not demarshallable) on POST
read -d '' newhznpolicy <<EOF
{
  "properties": [{name":"prop1"}],
  "constraints": ""
}
EOF

echo "Testing for not demarshallable input on node/policy API"
RES=$(echo "$newhznpolicy" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/node/policy")

if [ "$RES" == "" ]
then
  echo -e "$newhznpolicy \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "${ERR:0:36}" != "Input body could not be deserialized" ]
then
  echo -e "$newhznpolicy \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# Incorrect input (wrong field types) on PUT
read -d '' newhznpolicy <<EOF
{
  "properties": 11,
  "constraints": 0
}
EOF

echo "Testing for incorrect input field types on node/policy API"
RES=$(echo "$newhznpolicy" | curl -sS -X PUT -H "Content-Type: application/json" --data @- "$ANAX_API/node/policy")

if [ "$RES" == "" ]
then
  echo -e "$newhznpolicy \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "${ERR:0:36}" != "Input body could not be deserialized" ]
then
  echo -e "$newhznpolicy \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# Delete when nothing to delete
echo "Testing for delete when node is node defined on node/policy API"
RES=$(curl -sS -X DELETE -H "Content-Type: application/json" "$ANAX_API/node/policy")

if [ "$RES" == "" ]
then
  echo -e "Testing for delete when node is not defined."
  exit 2
fi

err=$(echo $RES | jq '.error')
if [ $err != "Exchange registration not recorded. Complete account and node registration with an exchange and then record node registration using this API's /node path." ]
then
  echo -e "Testing for delete when node not defined resulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# =====================================================================================
# Register a test device so that subsequent tests can run

echo "Calling node API"

curl -sS -H "Content-Type: application/json" "$ANAX_API/node" | jq -er '. | .account.id' > /dev/null

if [[ $? -eq 0 ]]; then
  read -d '' updatehzntoken <<EOF
{
  "token": "$TOKEN",
  "id": "$DEVICE_ID",
  "organization": "$ORG",
  "pattern": "$PATTERN"
}
EOF

  echo -e "\n[D] hzntoken payload: $updatehzntoken"

  echo "Setting device id and token into horizon API"

  RES=$(echo "$updatehzntoken" | curl -sS -X PATCH -H "Content-Type: application/json" --data @- "$ANAX_API/node")

else

  read -d '' newhzndevice <<EOF
{
  "id": "$DEVICE_ID",
  "name": "$DEVICE_NAME",
  "token": "$TOKEN",
  "organization": "$ORG",
  "pattern": "$PATTERN"
}
EOF

  echo -e "\n[D] hzndevice payload: $newhzndevice"

  echo "Updating horizon with device id and token"

  RES=$(echo "$newhzndevice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/node")

fi

echo -e "Response:\n$RES"
PAT=$(echo $RES | jq -r '.pattern')
if [ "$PAT" != "$PATTERN" ]
then
  echo -e "$newhzndevice \nresulted in incorrect response, wrong pattern: $RES"
  exit 2
fi

O=$(echo $RES | jq -r '.organization')
if [ "$O" != "$ORG" ]
then
  echo -e "$newhzndevice \nresulted in incorrect response, wrong organization: $RES"
  exit 2
fi

read -d '' locationattribute <<EOF
{
  "type": "LocationAttributes",
  "label": "Registered Location Facts",
  "publishable": false,
  "host_only": false,
  "mappings": {
    "lat": 41.921766,
    "lon": -73.894224
  }
}
EOF

echo -e "\n\n[D] location payload: $locationattribute"

echo "Setting workload independent location attributes"

RES=$(echo "$locationattribute" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/attribute")
echo -e "Response:\n$RES"
LAT=$(echo $RES | jq -r '.mappings["lat"]')
if [ "$LAT" != "41.921766" ]
then
  echo -e "$newhzndevice \nresulted in incorrect response, wrong lat: $RES"
  exit 2
fi

# =================================================================
# run HA tests - device is non-HA, cant use HA config

SERVICE_MODEL="service"
APITESTURL="https://bluehorizon.network/"$SERVICE_MODEL"s/no-such-service"

read -d '' service <<EOF
{
  "url": "$APITESTURL",
  "versionRange": "1.0.0",
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
    },
    {
      "type": "HAAttributes",
      "label": "HA Partner",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "partnerID": ["an54321"]
      }
    }
  ]
}
EOF

#fi

echo -e "\n\n[D] payload for HA test: $service"

echo "Registering service for HA test"

RES=$(echo "$service" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/$SERVICE_MODEL/config")
if [ "$RES" == "" ]
then
  echo -e "$service \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
SUB=${ERR:0:24}
if [ "$SUB" != "HA partner not permitted" ]
then
  echo -e "$service \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# =================================================================
# run attribute specific tests

# Set env vars depending on whether we're running services or not.
export SERVICE_MODE="service"
export SERVICE_URL="url"
export SERVICE_ORG="organization"
export SERVICE_NAME="name"
export SERVICE_VERSION="version"

./metering_apitest.sh
if [ $? -ne 0 ]
then
  echo -e "Metering tests failed"
  TESTFAIL="1"
  exit 2
else
  echo -e "Metering tests SUCCESSFUL"
fi

./agp_apitest.sh
if [ $? -ne 0 ]
then
  echo -e "Agreementprotocol tests failed"
  TESTFAIL="1"
  exit 2
else
  echo -e "Agreementprotocol tests SUCCESSFUL"
fi

  ./service_apitest.sh
if [ $? -ne 0 ]
then
  echo -e "Service config tests failed"
  TESTFAIL="1"
  exit 2
else
  echo -e "Service config tests SUCCESSFUL"
fi

./cs_apitest.sh
if [ $? -ne 0 ]
then
  echo -e "Configstate API tests failed"
  TESTFAIL="1"
  exit 2
else
  echo -e "Configstate API tests SUCCESSFUL"
fi

# =================================================================

echo -e "\n\n[D] all registered attributes:\n"
curl -sS -H "Content-Type: application/json" "$ANAX_API/attribute" | jq -r '.'

# =================================================================

if [ "$TESTFAIL" != "0" ]
then
  echo -e "Test failures occurred"
  exit 1
else
  echo -e "All tests passed"
fi
