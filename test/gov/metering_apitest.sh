#!/bin/bash

# Enable debug tracing when DEBUG=1 or RUNNER_DEBUG=1 (GitHub Actions debug mode).
if [ "${DEBUG:-0}" = "1" ] || [ "${RUNNER_DEBUG:-0}" = "1" ]; then
    set -x
fi

# ==================================================================
# Begin testing metering properties

# missing perTimeUnit field
cat > /tmp/netspeedservice.tmp <<EOF
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
netspeedservice=$(cat /tmp/netspeedservice.tmp)

echo -e "\n\n[D] netspeedservice payload: $netspeedservice"

echo "Registering netspeed service"

RES=$(echo "$netspeedservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/${SERVICE_MODE}/config")
if [ "$RES" = "" ]
then
  echo -e "$netspeedservice \nresulted in empty response"
  exit 2
fi

ERR=$(echo "$RES" | jq -r ".error")
if [ "$ERR" != "missing key" ]
then
  echo -e "$netspeedservice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# missing token field
cat > /tmp/netspeedservice.tmp <<EOF
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
netspeedservice=$(cat /tmp/netspeedservice.tmp)

echo -e "\n\n[D] netspeedservice payload: $netspeedservice"

echo "Registering netspeed service"

RES=$(echo "$netspeedservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/${SERVICE_MODE}/config")
if [ "$RES" = "" ]
then
  echo -e "$netspeedservice \nresulted in empty response"
  exit 2
fi

ERR=$(echo "$RES" | jq -r ".error")
if [ "$ERR" != "missing key" ]
then
  echo -e "$netspeedservice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# missing tokens and perTimeUnit fields
cat > /tmp/netspeedservice.tmp <<EOF
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
netspeedservice=$(cat /tmp/netspeedservice.tmp)

echo -e "\n\n[D] netspeedservice payload: $netspeedservice"

echo "Registering netspeed service"

RES=$(echo "$netspeedservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/${SERVICE_MODE}/config")
if [ "$RES" = "" ]
then
  echo -e "$netspeedservice \nresulted in empty response"
  exit 2
fi

ERR=$(echo "$RES" | jq -r ".error")
if [ "$ERR" != "missing tokens and perTimeUnit keys" ]
then
  echo -e "$netspeedservice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# tokens must be non-zero
cat > /tmp/netspeedservice.tmp <<EOF
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
netspeedservice=$(cat /tmp/netspeedservice.tmp)

echo -e "\n\n[D] netspeedservice payload: $netspeedservice"

echo "Registering netspeed service"

RES=$(echo "$netspeedservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/${SERVICE_MODE}/config")
if [ "$RES" = "" ]
then
  echo -e "$netspeedservice \nresulted in empty response"
  exit 2
fi

ERR=$(echo "$RES" | jq -r ".error")
if [ "$ERR" != "must be non-zero" ]
then
  echo -e "$netspeedservice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# perTimeUnit must be non-blank
cat > /tmp/netspeedservice.tmp <<EOF
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
netspeedservice=$(cat /tmp/netspeedservice.tmp)

echo -e "\n\n[D] netspeedservice payload: $netspeedservice"

echo "Registering netspeed service"

RES=$(echo "$netspeedservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/${SERVICE_MODE}/config")
if [ "$RES" = "" ]
then
  echo -e "$netspeedservice \nresulted in empty response"
  exit 2
fi

ERR=$(echo "$RES" | jq -r ".error")
if [ "$ERR" != "must be non-empty" ]
then
  echo -e "$netspeedservice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# tokens must be a valid number
cat > /tmp/netspeedservice.tmp <<EOF
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
netspeedservice=$(cat /tmp/netspeedservice.tmp)

echo -e "\n\n[D] netspeedservice payload: $netspeedservice"

echo "Registering netspeed service"

RES=$(echo "$netspeedservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/${SERVICE_MODE}/config")
if [ "$RES" = "" ]
then
  echo -e "$netspeedservice \nresulted in empty response"
  exit 2
fi

ERR=$(echo "$RES" | jq -r ".error")
if [ "$ERR" != "expected integer" ]
then
  echo -e "$netspeedservice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# perTimeUnit must be a string
cat > /tmp/netspeedservice.tmp <<EOF
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
netspeedservice=$(cat /tmp/netspeedservice.tmp)

echo -e "\n\n[D] netspeedservice payload: $netspeedservice"

echo "Registering netspeed service"

RES=$(echo "$netspeedservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/${SERVICE_MODE}/config")
if [ "$RES" = "" ]
then
  echo -e "$netspeedservice \nresulted in empty response"
  exit 2
fi

ERR=$(echo "$RES" | jq -r ".error")
if [ "$ERR" != "expected string" ]
then
  echo -e "$netspeedservice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# notification interval must be a number
cat > /tmp/netspeedservice.tmp <<EOF
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
netspeedservice=$(cat /tmp/netspeedservice.tmp)

echo -e "\n\n[D] netspeedservice payload: $netspeedservice"

echo "Registering netspeed service"

RES=$(echo "$netspeedservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/${SERVICE_MODE}/config")
if [ "$RES" = "" ]
then
  echo -e "$netspeedservice \nresulted in empty response"
  exit 2
fi

ERR=$(echo "$RES" | jq -r ".error")
if [ "$ERR" != "expected integer" ]
then
  echo -e "$netspeedservice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# notificationInterval cannot be specified without tokens perTimeUnit
cat > /tmp/netspeedservice.tmp <<EOF
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
netspeedservice=$(cat /tmp/netspeedservice.tmp)

echo -e "\n\n[D] netspeedservice payload: $netspeedservice"

echo "Registering netspeed service"

RES=$(echo "$netspeedservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/${SERVICE_MODE}/config")
if [ "$RES" = "" ]
then
  echo -e "$netspeedservice \nresulted in empty response"
  exit 2
fi

ERR=$(echo "$RES" | jq -r ".error")
if [ "$ERR" != "cannot be non-zero without tokens and perTimeUnit" ]
then
  echo -e "$netspeedservice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi
