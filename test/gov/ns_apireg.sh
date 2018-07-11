#!/bin/bash

echo -e "\nBC setting is $BC"

if [ "$BC" != "1" ]
then

echo -e "Pattern is set to $PATTERN"
if [ "$PATTERN" == "" ]
then

# Configure the netspeed service variables, at an older version level just to be sure
# that the runtime will still pick them up for the newer version that is installed in the exchange.
read -d '' snsconfig <<EOF
{
  "url": "https://bluehorizon.network/services/netspeed",
  "version": "2.2.0",
  "organization": "IBM",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "User input variables",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "var1": "aString",
        "var2": 5,
        "var3": 10.2,
        "var4": ["abc", "123"],
        "var5": "override"
      }
    }
  ]
}
EOF

echo -e "\n\n[D] netspeed service config payload: $snsconfig"

echo "Registering netspeed service config on node"

ERR=$(echo "$snsconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
if [ "$ERR" != "null" ]; then
  echo -e "error occured: $ERR"
  exit 2
fi

read -d '' networkservice <<EOF
{
  "url": "https://bluehorizon.network/services/network",
  "version": "1.0.0",
  "organization": "IBM",
  "attributes": []
}
EOF

echo -e "\n\n[D] networkservice payload: $networkservice"

echo "Registering network service"

ERR=$(echo "$networkservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
if [ "$ERR" != "null" ]; then
  echo -e "error occured: $ERR"
  exit 2
fi

read -d '' networkservice <<EOF
{
  "url": "https://bluehorizon.network/services/network2",
  "version": "1.0.0",
  "organization": "IBM",
  "attributes": []
}
EOF

echo -e "\n\n[D] networkservice payload: $networkservice"

echo "Registering network service"

ERR=$(echo "$networkservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
if [ "$ERR" != "null" ]; then
  if [ "${ERR:0:22}" != "Duplicate registration" ]; then
    echo -e "error occured: $ERR"
    exit 2
  fi
fi

elif [ "$PATTERN" == "ns" ] || [ "$PATTERN" == "all" ] || [ "$PATTERN" == "ns-keytest" ]; then

# and then configure the test workload variables, at an older version level
read -d '' workloadconfig <<EOF
{
  "workload_url": "https://bluehorizon.network/workloads/netspeed",
  "workload_version": "2.2.0",
  "organization": "IBM",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "User input variables",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "var1": "aString",
        "var2": 5,
        "var3": 10.2,
        "var4": ["abc", "123"],
        "var5": "override"
      }
    }
  ]
}
EOF

echo -e "\n\n[D] split ns workload config payload: $workloadconfig"

echo "Registering split ns workload config"

RES=$(echo "$workloadconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/workload/config" | jq -r '.')
if [ "$RES" == "" ]; then
  echo -e "error occured: $RES"
  exit 2
fi

elif [ "$PATTERN" == "sns" ] || [ "$PATTERN" == "sall" ]
then

# Configure the netspeed service variables, at an older version level just to be sure
# that the runtime will still pick them up for the newer version that is installed in the exchange.
read -d '' snsconfig <<EOF
{
  "url": "https://bluehorizon.network/services/netspeed",
  "version": "2.2.0",
  "organization": "IBM",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "User input variables",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "var1": "aString",
        "var2": 5,
        "var3": 10.2,
        "var4": ["abc", "123"],
        "var5": "override"
      }
    }
  ]
}
EOF

echo -e "\n\n[D] netspeed service config payload: $snsconfig"

echo "Registering netspeed service config on node"

ERR=$(echo "$snsconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
if [ "$ERR" != "null" ]; then
  echo -e "error occured: $ERR"
  exit 2
fi

fi

else

read -d '' splitnsservice <<EOF
{
  "sensor_url": "https://bluehorizon.network/microservices/network",
  "sensor_org": "IBM",
  "sensor_name": "network",
  "sensor_version": "1.0.0",
  "attributes": [
    {
      "type": "MeteringAttributes",
      "label": "Metering Policy",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "tokens": 2,
        "perTimeUnit": "hour",
        "notificationInterval": 3600
      }
    },
    {
      "type": "AgreementProtocolAttributes",
      "label": "Agreement Protocols",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "protocols": [
          {
            "Citizen Scientist": [
              {
                "name": "privatebc",
                "organization": "e2edev"
              },
              {
                "name": "bluehorizon",
                "organization": "e2edev"
              }
            ]
          },
          {
            "Basic": []
          }
        ]
      }
    }
  ]
}
EOF

echo -e "\n\n[D] split netspeed network microservice payload: $splitnsservice"

echo "Registering split netspeed network microservice"

ERR=$(echo "$splitnsservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/microservice/config" | jq -r '.error')
if [ "$ERR" != "null" ]; then
  echo -e "error occured: $ERR"
  exit 2
fi

# register the second microservice
read -d '' splitnsservice <<EOF
{
  "sensor_url": "https://bluehorizon.network/microservices/network2",
  "sensor_org": "IBM",
  "sensor_name": "network",
  "sensor_version": "1.5.0",
  "attributes": [
    {
      "type": "MeteringAttributes",
      "label": "Metering Policy",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "tokens": 3,
        "perTimeUnit": "hour",
        "notificationInterval": 1800
      }
    },
    {
      "type": "AgreementProtocolAttributes",
      "label": "Agreement Protocols",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "protocols": [
          {
            "Citizen Scientist": [
              {
                "name": "privatebc",
                "organization": "e2edev"
              },
              {
                "name": "bluehorizon",
                "organization": "e2edev"
              }
            ]
          },
          {
            "Basic": []
          }
        ]
      }
    }
  ]
}
EOF

echo -e "\n\n[D] split netspeed network2 microservice payload: $splitnsservice"

echo "Registering split netspeed network2 microservice"

ERR=$(echo "$splitnsservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/microservice/config" | jq -r '.error')
if [ "$ERR" != "null" ]; then
  echo -e "error occured: $ERR"
  exit 2
fi

# and then configure the test workload variables, at an older version level
read -d '' workloadconfig <<EOF
{
  "workload_url": "https://bluehorizon.network/workloads/netspeed",
  "workload_version": "2.2.0",
  "organization": "IBM",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "User input variables",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "var1": "aString",
        "var2": 5,
        "var3": 10.2,
        "var4": ["abc", "123"],
        "var5": "override"
      }
    }
  ]
}
EOF

echo -e "\n\n[D] split ns workload config payload: $workloadconfig"

echo "Registering split ns workload config"

ERR=$(echo "$workloadconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/workload/config" | jq -r '.error')
if [ "$ERR" != "null" ]; then
  echo -e "error occured: $ERR"
  exit 2
fi

fi