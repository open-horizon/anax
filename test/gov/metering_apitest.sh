#!/bin/bash

# ==================================================================
# Begin testing metering properties

# missing perTimeUnit field
read -d '' netspeedservice <<EOF
{
  "${SERVICE_URL}": "https://bluehorizon.network/${SERVICE_MODE}s/network",
  "${SERVICE_ORG}": "IBM",
  "${SERVICE_NAME}": "network",
  "${SERVICE_VERSION}": "1.0.0",
  "attributes": [
    {
      "type": "MeteringAttributes",
      "label": "Metering Policy",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "tokens": 3
      }
    }
  ]
}
EOF

echo -e "\n\n[D] netspeedservice payload: $netspeedservice"

echo "Registering netspeed service"

RES=$(echo "$netspeedservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/${SERVICE_MODE}/config")
if [ "$RES" == "" ]
then
  echo -e "$netspeedservice \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "missing key" ]
then
  echo -e "$netspeedservice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# missing token field
read -d '' netspeedservice <<EOF
{
  "${SERVICE_URL}": "https://bluehorizon.network/${SERVICE_MODE}s/network",
  "${SERVICE_ORG}": "IBM",
  "${SERVICE_NAME}": "network",
  "${SERVICE_VERSION}": "1.0.0",
  "attributes": [
    {
      "type": "MeteringAttributes",
      "label": "Metering Policy",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "perTimeUnit": "min"
      }
    }
  ]
}
EOF

echo -e "\n\n[D] netspeedservice payload: $netspeedservice"

echo "Registering netspeed service"

RES=$(echo "$netspeedservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/${SERVICE_MODE}/config")
if [ "$RES" == "" ]
then
  echo -e "$netspeedservice \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "missing key" ]
then
  echo -e "$netspeedservice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# missing tokens and perTimeUnit fields
read -d '' netspeedservice <<EOF
{
  "${SERVICE_URL}": "https://bluehorizon.network/${SERVICE_MODE}s/network",
  "${SERVICE_ORG}": "IBM",
  "${SERVICE_NAME}": "network",
  "${SERVICE_VERSION}": "1.0.0",
  "attributes": [
    {
      "type": "MeteringAttributes",
      "label": "Metering Policy",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "notificationInterval": 15
      }
    }
  ]
}
EOF

echo -e "\n\n[D] netspeedservice payload: $netspeedservice"

echo "Registering netspeed service"

RES=$(echo "$netspeedservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/${SERVICE_MODE}/config")
if [ "$RES" == "" ]
then
  echo -e "$netspeedservice \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "missing tokens and perTimeUnit keys" ]
then
  echo -e "$netspeedservice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# tokens must be non-zero
read -d '' netspeedservice <<EOF
{
  "${SERVICE_URL}": "https://bluehorizon.network/${SERVICE_MODE}s/network",
  "${SERVICE_ORG}": "IBM",
  "${SERVICE_NAME}": "network",
  "${SERVICE_VERSION}": "1.0.0",
  "attributes": [
    {
      "type": "MeteringAttributes",
      "label": "Metering Policy",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "tokens": 0,
        "perTimeUnit": "min"
      }
    }
  ]
}
EOF

echo -e "\n\n[D] netspeedservice payload: $netspeedservice"

echo "Registering netspeed service"

RES=$(echo "$netspeedservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/${SERVICE_MODE}/config")
if [ "$RES" == "" ]
then
  echo -e "$netspeedservice \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "must be non-zero" ]
then
  echo -e "$netspeedservice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# perTimeUnit must be non-blank
read -d '' netspeedservice <<EOF
{
  "${SERVICE_URL}": "https://bluehorizon.network/${SERVICE_MODE}s/network",
  "${SERVICE_ORG}": "IBM",
  "${SERVICE_NAME}": "network",
  "${SERVICE_VERSION}": "1.0.0",
  "attributes": [
    {
      "type": "MeteringAttributes",
      "label": "Metering Policy",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "tokens": 3,
        "perTimeUnit": ""
      }
    }
  ]
}
EOF

echo -e "\n\n[D] netspeedservice payload: $netspeedservice"

echo "Registering netspeed service"

RES=$(echo "$netspeedservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/${SERVICE_MODE}/config")
if [ "$RES" == "" ]
then
  echo -e "$netspeedservice \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "must be non-empty" ]
then
  echo -e "$netspeedservice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# tokens must be a valid number
read -d '' netspeedservice <<EOF
{
  "${SERVICE_URL}": "https://bluehorizon.network/${SERVICE_MODE}s/network",
  "${SERVICE_ORG}": "IBM",
  "${SERVICE_NAME}": "network",
  "${SERVICE_VERSION}": "1.0.0",
  "attributes": [
    {
      "type": "MeteringAttributes",
      "label": "Metering Policy",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "tokens": "abc",
        "perTimeUnit": "min",
        "notificationInterval": 15
      }
    }
  ]
}
EOF

echo -e "\n\n[D] netspeedservice payload: $netspeedservice"

echo "Registering netspeed service"

RES=$(echo "$netspeedservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/${SERVICE_MODE}/config")
if [ "$RES" == "" ]
then
  echo -e "$netspeedservice \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "expected integer" ]
then
  echo -e "$netspeedservice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# perTimeUnit must be a string
read -d '' netspeedservice <<EOF
{
  "${SERVICE_URL}": "https://bluehorizon.network/${SERVICE_MODE}s/network",
  "${SERVICE_ORG}": "IBM",
  "${SERVICE_NAME}": "network",
  "${SERVICE_VERSION}": "1.0.0",
  "attributes": [
    {
      "type": "MeteringAttributes",
      "label": "Metering Policy",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "tokens": 3,
        "perTimeUnit": 10,
        "notificationInterval": 15
      }
    }
  ]
}
EOF

echo -e "\n\n[D] netspeedservice payload: $netspeedservice"

echo "Registering netspeed service"

RES=$(echo "$netspeedservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/${SERVICE_MODE}/config")
if [ "$RES" == "" ]
then
  echo -e "$netspeedservice \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "expected string" ]
then
  echo -e "$netspeedservice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# notification interval must be a number
read -d '' netspeedservice <<EOF
{
  "${SERVICE_URL}": "https://bluehorizon.network/${SERVICE_MODE}s/network",
  "${SERVICE_ORG}": "IBM",
  "${SERVICE_NAME}": "network",
  "${SERVICE_VERSION}": "1.0.0",
  "attributes": [
    {
      "type": "MeteringAttributes",
      "label": "Metering Policy",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "tokens": 3,
        "perTimeUnit": "min",
        "notificationInterval": "abc"
      }
    }
  ]
}
EOF

echo -e "\n\n[D] netspeedservice payload: $netspeedservice"

echo "Registering netspeed service"

RES=$(echo "$netspeedservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/${SERVICE_MODE}/config")
if [ "$RES" == "" ]
then
  echo -e "$netspeedservice \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "expected integer" ]
then
  echo -e "$netspeedservice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# notificationInterval cannot be specified without tokens perTimeUnit
read -d '' netspeedservice <<EOF
{
  "${SERVICE_URL}": "https://bluehorizon.network/${SERVICE_MODE}s/network",
  "${SERVICE_ORG}": "IBM",
  "${SERVICE_NAME}": "network",
  "${SERVICE_VERSION}": "1.0.0",
  "attributes": [
    {
      "type": "MeteringAttributes",
      "label": "Metering Policy",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "tokens": 0,
        "perTimeUnit": "",
        "notificationInterval": 15
      }
    }
  ]
}
EOF

echo -e "\n\n[D] netspeedservice payload: $netspeedservice"

echo "Registering netspeed service"

RES=$(echo "$netspeedservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/${SERVICE_MODE}/config")
if [ "$RES" == "" ]
then
  echo -e "$netspeedservice \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "cannot be non-zero without tokens and perTimeUnit" ]
then
  echo -e "$netspeedservice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi
