#!/bin/bash

EXCH_URL="http://${EXCH_APP_HOST:-172.17.0.1}:8080/v1"

# test microservice

RES=$(curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic e2edev/e2edevadmin:e2edevadminpw" -d '{"label":"Test for x86_64","description":"Test microservice","public":false,"specRef":"https://bluehorizon.network/microservices/test","version":"1.0.0","arch":"amd64","sharable":"none","downloadUrl":"","matchHardware":{},"userInput":[{"name":"test","label":"","type":"string"},{"name":"testdefault","label":"","type":"string","defaultValue":"default"}],"workloads":[]}' "${EXCH_URL}/orgs/e2edev/microservices" | jq -r '.')
if [ "$RES" == "" ]
then
  echo -e "creation of test microservice resulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".code")
if [ "$ERR" != "ok" ]
then
  echo -e "creation of test microservice resulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# verify that all userInputs are provided, test is missing
read -d '' testservice <<EOF
{
  "sensor_url": "https://bluehorizon.network/microservices/test",
  "sensor_name": "test",
  "sensor_version": "1.0.0",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "Extra",
      "publishable": true,
      "host_only": false,
      "mappings": {
      }
    }
  ]
}
EOF

echo -e "\n\n[D] test service payload: $testservice"

echo "Registering test service"

RES=$(echo "$testservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/microservice/config")
if [ "$RES" == "" ]
then
  echo -e "$testservice \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "variable test is missing from mappings" ]
then
  echo -e "$testservice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# verify that all userInputs are provided, test is still missing
read -d '' testservice <<EOF
{
  "sensor_url": "https://bluehorizon.network/microservices/test",
  "sensor_name": "test",
  "sensor_version": "1.0.0",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "Extra",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "testDefault":"override"
      }
    }
  ]
}
EOF

echo -e "\n\n[D] test service payload: $testservice"

echo "Registering test service"

RES=$(echo "$testservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/microservice/config")
if [ "$RES" == "" ]
then
  echo -e "$testservice \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "variable test is missing from mappings" ]
then
  echo -e "$testservice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# verify that all userInputs are provided, test is still missing
read -d '' testservice <<EOF
{
  "sensor_url": "https://bluehorizon.network/microservices/test",
  "sensor_name": "test",
  "sensor_version": "1.0.0",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "Extra",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "undefined":"unknown"
      }
    }
  ]
}
EOF

echo -e "\n\n[D] test service payload: $testservice"

echo "Registering test service"

RES=$(echo "$testservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/microservice/config")
if [ "$RES" == "" ]
then
  echo -e "$testservice \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "variable test is missing from mappings" ]
then
  echo -e "$testservice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# verify that all userInputs are provided, test is still missing
read -d '' testservice <<EOF
{
  "sensor_url": "https://bluehorizon.network/microservices/test",
  "sensor_name": "test",
  "sensor_version": "1.0.0",
  "attributes": []
}
EOF

echo -e "\n\n[D] test service payload: $testservice"

echo "Registering test service"

RES=$(echo "$testservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/microservice/config")
if [ "$RES" == "" ]
then
  echo -e "$testservice \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "variable test is missing from mappings" ]
then
  echo -e "$testservice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi
