#!/bin/bash

echo -e "Pattern is set to $PATTERN"
if [ "$PATTERN" == "all" ]
then

# register the microservice
read -d '' hservice <<EOF
{
  "sensor_url": "http://bluehorizon.network/microservices/hello",
  "sensor_name": "hello",
  "sensor_version": "1.0.0",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "app",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "MY_MS_VAR1": "myVar1Value"
      }
    }
  ]
}
EOF

echo -e "\n\n[D] hello ms service payload: $hservice"

echo "Registering hello ms service"

ERR=$(echo "$hservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/microservice/config" | jq -r '.error')
if [ "$ERR" != "null" ]; then
  echo -e "error occured: $ERR"
  exit 2
fi

# and then configure the workload variables
read -d '' hworkload <<EOF
{
  "workload_url": "http://bluehorizon.network/workloads/usehello",
  "workload_version": "1.0.0",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "User input variables",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "MY_VAR1": "inside"
      }
    }
  ]
}
EOF

echo -e "\n\n[D] use hello workload payload: $hworkload"

echo "Registering use hello workload"

RES=$(echo "$hworkload" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/workload/config" | jq -r '.')
if [ "$RES" == "" ]; then
  echo -e "error occured: $RES"
  exit 2
fi

elif [ "$PATTERN" == "susehello" ] || [ "$PATTERN" == "sall" ]
then

# Configure the usehello service variables.
read -d '' snsconfig <<EOF
{
  "url": "http://my.company.com/services/usehello2",
  "version": "1.0.0",
  "organization": "e2edev",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "User input variables",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "MY_VAR1": "e2edev"
      }
    }
  ]
}
EOF

echo -e "\n\n[D] usehello service config payload: $snsconfig"

echo "Registering usehello service config on node"

ERR=$(echo "$snsconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
if [ "$ERR" != "null" ]; then
  echo -e "error occured: $ERR"
  exit 2
fi

# Configure the hello service variables.
read -d '' snsconfig <<EOF
{
  "url": "http://my.company.com/services/hello2",
  "version": "1.0.0",
  "organization": "e2edev",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "User input variables",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "MY_S_VAR1": "e2edev"
      }
    }
  ]
}
EOF

echo -e "\n\n[D] hello service config payload: $snsconfig"

echo "Registering hello service config on node"

ERR=$(echo "$snsconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
if [ "$ERR" != "null" ]; then
  echo -e "error occured: $ERR"
  exit 2
fi

# Configure the cpu service variables.
read -d '' snsconfig <<EOF
{
  "url": "http://my.company.com/services/cpu2",
  "version": "1.0.0",
  "organization": "e2edev",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "User input variables",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "MY_CPU_VAR": "e2edev"
      }
    }
  ]
}
EOF

echo -e "\n\n[D] cpu service config payload: $snsconfig"

echo "Registering cpu service config on node"

ERR=$(echo "$snsconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
if [ "$ERR" != "null" ]; then
  echo -e "error occured: $ERR"
  exit 2
fi

fi