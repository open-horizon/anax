#!/bin/bash

echo -e "\nBC setting is $BC"

if [ "$BC" != "1" ]
then

echo -e "Pattern is set to $PATTERN"
if [ "$PATTERN" == "" ]
then

# and then configure by service API to opt into the node side services.
read -d '' slocservice <<EOF
{
  "url": "https://bluehorizon.network/services/locgps",
  "name": "gps",
  "versionRange": "2.0.3",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "Extra",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "test": "testValue",
        "extra": "extraValue"
      }
    }
  ]
}
EOF

echo -e "\n\n[D] service based loc gps service payload: $slocservice"

echo "Registering service based loc gps service"

ERR=$(echo "$slocservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
if [ "$ERR" != "null" ]; then
  echo -e "error occured: $ERR"
  exit 2
fi

read -d '' slocservice <<EOF
{
  "url": "https://internetofthings.ibmcloud.com/services/cpu",
  "name": "cpu",
  "organization": "IBM",
  "versionRange": "1.0.0",
  "attributes": []
}
EOF

echo -e "\n\n[D] service based cpu service payload: $slocservice"

echo "Registering service based cpu service"

ERR=$(echo "$slocservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
if [ "$ERR" != "null" ]; then
  echo -e "error occured: $ERR"
  exit 2
fi

read -d '' slocservice <<EOF
{
  "url": "https://bluehorizon.network/services/network2",
  "name": "gps",
  "organization": "IBM",
  "versionRange": "1.0.0",
  "attributes": []
}
EOF

echo -e "\n\n[D] service based network2 service payload: $slocservice"

echo "Registering service based network2 service"

ERR=$(echo "$slocservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
if [ "$ERR" != "null" ]; then
  if [ "${ERR:0:22}" != "Duplicate registration" ]; then
    echo -e "error occured: $ERR"
    exit 2
  fi
fi

# using pattern, must configure the gps container, by old microservice
elif [ "$PATTERN" == "all" ] || [ "$PATTERN" == "loc" ]; then

read -d '' splitgpsservice <<EOF
{
  "sensor_url": "https://bluehorizon.network/microservices/locgps",
  "sensor_name": "gps",
  "sensor_version": "2.0.3",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "Extra",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "test": "testValue",
        "extra": "extraValue"
      }
    }
  ]
}
EOF

echo -e "\n\n[D] split gps service payload: $splitgpsservice"

echo "Registering split gps loc service"

ERR=$(echo "$splitgpsservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/microservice/config" | jq -r '.error')
if [ "$ERR" != "null" ]; then
  echo -e "error occured: $ERR"
  exit 2
fi

elif [ "$PATTERN" == "sall" ] || [ "$PATTERN" == "sloc" ]; then

# and then configure by service API
read -d '' slocservice <<EOF
{
  "url": "https://bluehorizon.network/services/locgps",
  "name": "gps",
  "versionRange": "2.0.3",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "Extra",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "test": "testValue",
        "extra": "extraValue"
      }
    }
  ]
}
EOF

echo -e "\n\n[D] service based loc gps service payload: $slocservice"

echo "Registering service based loc gps service"

ERR=$(echo "$slocservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
if [ "$ERR" != "null" ]; then
  echo -e "error occured: $ERR"
  exit 2
fi

fi

# blockchain in use
else

read -d '' splitgpsservice <<EOF
{
  "sensor_url": "https://bluehorizon.network/microservices/locgps",
  "sensor_name": "gps",
  "sensor_version": "2.0.3",
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
      "type": "UserInputAttributes",
      "label": "Extra",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "test": "testValue",
        "extra": "extraValue"
      }
    },
    {
      "type": "agreementprotocol",
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

echo -e "\n\n[D] split gps service payload: $splitgpsservice"

echo "Registering split gps service"

ERR=$(echo "$splitgpsservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/microservice/config" | jq -r '.error')
if [ "$ERR" != "null" ]; then
  echo -e "error occured: $ERR"
  exit 2
fi

fi