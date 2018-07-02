#!/bin/bash

echo -e "Pattern is set to $PATTERN"
if [ "$PATTERN" == "" ]
then

read -d '' networkservice <<EOF
{
  "url": "https://bluehorizon.network/services/network",
  "version": "1.0.0",
  "organization": "IBM",
  "attributes": [
    {
      "type": "HAAttributes",
      "label": "HA Partner",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "partnerID": ["$PARTNERID"]
      }
    }
  ]
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
  "attributes": [
    {
      "type": "HAAttributes",
      "label": "HA Partner",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "partnerID": ["$PARTNERID"]
      }
    }
  ]
}
EOF

echo -e "\n\n[D] networkservice payload: $networkservice"

echo "Registering network service"

ERR=$(echo "$networkservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
if [ "$ERR" != "null" ]; then
  echo -e "error occured: $ERR"
  exit 2
fi

# using a pattern, no policy. Only service patterns are supported.
elif [ "$PATTERN" = "sns" ]; then

read -d '' netspeedservice <<EOF
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
    },
    {
      "type": "HAAttributes",
      "label": "HA Partner",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "partnerID": ["$PARTNERID"]
      }
    }
  ]
}
EOF

echo -e "\n\n[D] netspeedservice payload: $netspeedservice"

echo "Registering netspeed service"

ERR=$(echo "$netspeedservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
if [ "$ERR" != "null" ]; then
  echo -e "error occured: $ERR"
  exit 2
fi

read -d '' networkservice <<EOF
{
  "url": "https://bluehorizon.network/services/network",
  "version": "1.0.0",
  "organization": "IBM",
  "attributes": [
    {
      "type": "HAAttributes",
      "label": "HA Partner",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "partnerID": ["$PARTNERID"]
      }
    }
  ]
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
  "attributes": [
    {
      "type": "HAAttributes",
      "label": "HA Partner",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "partnerID": ["$PARTNERID"]
      }
    }
  ]
}
EOF

echo -e "\n\n[D] networkservice payload: $networkservice"

echo "Registering network service"

ERR=$(echo "$networkservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
if [ "$ERR" != "null" ]; then
  echo -e "error occured: $ERR"
  exit 2
fi

else
  echo "Non netspeed pattern, nothing to do."
fi