#!/bin/bash

EMAIL="foo@goo.com"

  echo "Calling node API"

curl -sS -H "Content-Type: application/json" "$ANAX_API/node" | jq -er '. | .account.id' > /dev/null

if [[ $? -eq 0 ]]; then
  read -d '' updatehzntoken <<EOF
{
  "account": {
    "id": "$USER"
  },
  "token": "$TOKEN"
}
EOF

  echo -e "\n[D] hzntoken payload: $updatehzntoken"

  echo "Setting device id and token into horizon API"

  echo "$updatehzntoken" | curl -sS -X PATCH -H "Content-Type: application/json" --data @- "$ANAX_API/node"

else

  read -d '' newhzndevice <<EOF
{
  "account": {
    "id": "$USER",
    "email": "$EMAIL"
  },
  "id": "$DEVICE_ID",
  "name": "$DEVICE_NAME",
  "token": "$TOKEN"
}
EOF

  echo -e "\n[D] hzndevice payload: $newhzndevice"

  echo "Updating horizon with out device id and token"

  echo "$newhzndevice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/node"
fi

read -d '' locationattribute <<EOF
{
  "id": "location",
  "short_type": "location",
  "label": "Registered Location Facts",
  "publishable": false,
  "mappings": {
    "lat": 41.921766,
    "lon": -73.894224,
    "location_accuracy_km": 0.5
  }
}
EOF

echo -e "\n\n[D] location payload: $locationattribute"

echo "Setting workload independent location attributes"

echo "$locationattribute" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/attribute"

read -d '' gpstestservice <<EOF
{
  "sensor_url": "https://bluehorizon.network/documentation/gpstest-device-api",
  "sensor_name": "gpstest",
  "attributes": [
    {
      "id": "compute",
      "short_type": "compute",
      "label": "Compute Resources",
      "publishable": true,
      "mappings": {
        "ram": 256,
        "cpus": 1
      }
    },
    {
      "id": "free form",
      "short_type": "mapped",
      "label": "Extra",
      "publishable": true,
      "mappings": {
        "foo": "goo",
        "new": "zoo"
      }
    }
  ]
}
EOF

echo -e "\n\n[D] gpstestservice payload: $gpstestservice"

echo "Registering gpstest service"

echo "$gpstestservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/microservice/config"

read -d '' location2service <<EOF
{
  "sensor_url": "https://bluehorizon.network/documentation/location2-device-api",
  "sensor_name": "location2",
  "attributes": [
    {
      "id": "compute",
      "short_type": "compute",
      "label": "Compute Resources",
      "publishable": true,
      "mappings": {
        "ram": 256,
        "cpus": 1
      }
    },
    {
      "id": "free form",
      "short_type": "mapped",
      "label": "Extra",
      "publishable": true,
      "mappings": {
        "foo": "goo",
        "new": "zoo"
      }
    }
  ]
}
EOF

echo -e "\n\n[D] location2service payload: $location2service"

echo "Registering location2 service"

echo "$location2service" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/microservice/config"

echo -e "\n\n[D] all registered attributes:\n"
curl -sS -H "Content-Type: application/json" "$ANAX_API/attribute" | jq -r '.'
