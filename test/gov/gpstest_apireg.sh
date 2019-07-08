#!/bin/bash

echo -e "\nBC setting is $BC"

if [ "$BC" != "1" ]
then

echo -e "Pattern is set to $PATTERN"

  # add user input with /node/userinput api
  read -d '' nodeui <<EOF
[
    {
      "serviceOrgid": "IBM",
      "serviceUrl": "https://bluehorizon.network/service-gps",
      "serviceArch": "amd64",
      "serviceVersionRange": "2.0.3",
      "inputs": [
        {
          "name": "HZN_LAT",
          "value": 41.921766
        },
        {
          "name": "HZN_LON",
          "value": -73.894224
        },
        {
          "name": "HZN_LOCATION_ACCURACY_KM",
          "value": 0.5
        },
        {
          "name": "HZN_USE_GPS",
          "value": false
        }
      ]
    }
]

EOF
  echo "Adding service configurarion for service-gps with /node/userinput api..."
  RES=$(echo "$nodeui" | curl -sS -X PATCH -w "%{http_code}" -H "Content-Type: application/json" --data @- "$ANAX_API/node/userinput")
  if [ "$RES" == "" ]
  then
    echo -e "$newhznpolicy \nresulted in empty response"
    exit 2
  fi

  ERR=$(echo $RES | jq -r '.' | tail -1)
  if [ "$ERR" != "201" ]
  then
    echo -e "$nodeui \nresulted in incorrect response: $RES"
  else
    echo -e "found expected response: $RES"
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
                "organization": "e2edev@somecomp.com"
              },
              {
                "name": "bluehorizon",
                "organization": "e2edev@somecomp.com"
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
