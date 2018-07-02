#!/bin/bash

echo -e "\nBC setting is $BC"

if [ "$BC" != "1" ]
then

echo -e "Pattern is set to $PATTERN"
if [ "$PATTERN" == "" ]
then

# and then configure by service API to opt into the gps service.
read -d '' slocservice <<EOF
{
  "url": "https://bluehorizon.network/services/gps",
  "name": "gps",
  "versionRange": "2.0.3",
  "attributes": []
}
EOF

echo -e "\n\n[D] service based gps service payload: $slocservice"

echo "Registering service based gps service"

ERR=$(echo "$slocservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
if [ "$ERR" != "null" ]; then
    echo -e "error occured: $ERR"
    exit 2
fi

fi

# blockchain is in use
else

read -d '' splitgpsservice <<EOF
{
  "sensor_url": "https://bluehorizon.network/microservices/gps",
  "sensor_name": "gps",
  "sensor_version": "2.0.3",
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
    },
    {
      "type": "PropertyAttributes",
      "label": "Property",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "iame2edev": true
      }
    }
  ]
}
EOF

echo -e "\n\n[D] split gps service payload with BC: $splitgpsservice"

echo "Registering split gps service with BC"

ERR=$(echo "$splitgpsservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/microservice/config" | jq -r '.error')
if [ "$ERR" != "null" ]; then
  echo -e "error occured: $ERR"
  exit 2
fi

fi