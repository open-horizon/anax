#!/bin/bash

# =================================================================
# CounterParty Property tests

# not a valid expression
read -d '' counterpartypropertyattribute <<EOF
{
  "type": "CounterPartyPropertyAttributes",
  "label": "CounterParty Property",
  "publishable": true,
  "host_only": false,
  "mappings": {
    "expression": {
      "fred": [
        {"name":"rpiprop1", "op":"=", "value":"rpival1"},
        {"name":"rpiprop2", "op":"=", "value":"rpival2"}
      ]
    }
  }
}
EOF

echo -e "\n\n[D] counterparty property payload: $counterpartypropertyattribute"

echo "Setting workload independent counterparty property attribute"

RES=$(echo "$counterpartypropertyattribute" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost/attribute")
if [ "$RES" == "" ]
then
  echo -e "$counterpartypropertyattribute \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
SUB=${ERR:0:22}
if [ "$SUB" != "not a valid expression" ]
then
  echo -e "$counterpartypropertyattribute \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# expression spelled wrong
read -d '' counterpartypropertyattribute <<EOF
{
  "type": "CounterPartyPropertyAttributes",
  "label": "CounterParty Property",
  "publishable": true,
  "host_only": false,
  "mappings": {
    "express": {
      "and": [
        {"name":"rpiprop1", "op":"=", "value":"rpival1"},
        {"name":"rpiprop2", "op":"=", "value":"rpival2"}
      ]
    }
  }
}
EOF

echo -e "\n\n[D] counterparty property payload: $counterpartypropertyattribute"

echo "Setting workload independent counterparty property attribute"

RES=$(echo "$counterpartypropertyattribute" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost/attribute")
if [ "$RES" == "" ]
then
  echo -e "$counterpartypropertyattribute \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "missing key" ]
then
  echo -e "$counterpartypropertyattribute \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# property expression in not valid
read -d '' counterpartypropertyattribute <<EOF
{
  "type": "CounterPartyPropertyAttributes",
  "label": "CounterParty Property",
  "publishable": true,
  "host_only": false,
  "mappings": {
    "expression": {
      "and": true
    }
  }
}
EOF

echo -e "\n\n[D] counterparty property payload: $counterpartypropertyattribute"

echo "Setting workload independent counterparty property attribute"

RES=$(echo "$counterpartypropertyattribute" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost/attribute")
if [ "$RES" == "" ]
then
  echo -e "$counterpartypropertyattribute \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
SUB=${ERR:0:22}
if [ "$SUB" != "not a valid expression" ]
then
  echo -e "$counterpartypropertyattribute \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# invalid operator in an expression
read -d '' counterpartypropertyattribute <<EOF
{
  "type": "CounterPartyPropertyAttributes",
  "label": "CounterParty Property",
  "publishable": true,
  "host_only": false,
  "mappings": {
    "expression": {
      "or": [
        {"name":"rpiprop1", "op":"abc", "value":"rpival1"},
        {"name":"rpiprop2", "op":"=", "value":"rpival2"}
      ]
    }
  }
}
EOF

echo -e "\n\n[D] counterparty property payload: $counterpartypropertyattribute"

echo "Setting workload independent counterparty property attribute"

RES=$(echo "$counterpartypropertyattribute" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost/attribute")
if [ "$RES" == "" ]
then
  echo -e "$counterpartypropertyattribute \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
SUB=${ERR:0:22}
if [ "$SUB" != "not a valid expression" ]
then
  echo -e "$counterpartypropertyattribute \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi
