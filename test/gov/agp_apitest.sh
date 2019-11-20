#!/bin/bash

# ==================================================================
# Begin testing global agreement protocol attributes

# missing protocol definition
read -d '' agreementprotocolattribute <<EOF
{
  "type": "AgreementProtocolAttributes",
  "label": "Agreement Protocols",
  "publishable": true,
  "host_only": false,
  "mappings": {}
}
EOF

echo -e "\n\n[D] agreement protocol payload: $agreementprotocolattribute"

echo "Setting workload independent agreement protocol attribute"

RES=$(echo "$agreementprotocolattribute" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/attribute")
if [ "$RES" == "" ]
then
  echo -e "$agreementprotocolattribute \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "missing key" ]
then
  echo -e "$agreementprotocolattribute \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# no protocols specified
read -d '' agreementprotocolattribute <<EOF
{
  "type": "AgreementProtocolAttributes",
  "label": "Agreement Protocols",
  "publishable": true,
  "host_only": false,
  "mappings": {
    "protocols": []
  }
}
EOF

echo -e "\n\n[D] agreement protocol payload: $agreementprotocolattribute"

echo "Setting workload independent agreement protocol attribute"

RES=$(echo "$agreementprotocolattribute" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/attribute")
if [ "$RES" == "" ]
then
  echo -e "$agreementprotocolattribute \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "array value is empty" ]
then
  echo -e "$agreementprotocolattribute \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# protocol is not a number
read -d '' agreementprotocolattribute <<EOF
{
  "type": "AgreementProtocolAttributes",
  "label": "Agreement Protocols",
  "publishable": true,
  "host_only": false,
  "mappings": {
    "protocols": 5
  }
}
EOF

echo -e "\n\n[D] agreement protocol payload: $agreementprotocolattribute"

echo "Setting workload independent agreement protocol attribute"

RES=$(echo "$agreementprotocolattribute" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/attribute")
if [ "$RES" == "" ]
then
  echo -e "$agreementprotocolattribute \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "expected []interface{} received json.Number" ]
then
  echo -e "$agreementprotocolattribute \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# protocol is not an array of numbers
read -d '' agreementprotocolattribute <<EOF
{
  "type": "AgreementProtocolAttributes",
  "label": "Agreement Protocols",
  "publishable": true,
  "host_only": false,
  "mappings": {
    "protocols": [5]
  }
}
EOF

echo -e "\n\n[D] agreement protocol payload: $agreementprotocolattribute"

echo "Setting workload independent agreement protocol attribute"

RES=$(echo "$agreementprotocolattribute" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/attribute")
if [ "$RES" == "" ]
then
  echo -e "$agreementprotocolattribute \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "array value is not a map[string]interface{}, it is json.Number" ]
then
  echo -e "$agreementprotocolattribute \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# fred is not one of the supported protocol names
read -d '' agreementprotocolattribute <<EOF
{
  "type": "AgreementProtocolAttributes",
  "label": "Agreement Protocols",
  "publishable": true,
  "host_only": false,
  "mappings": {
    "protocols": [
      {
        "fred": []
      }
    ]
  }
}
EOF

echo -e "\n\n[D] agreement protocol payload: $agreementprotocolattribute"

echo "Setting workload independent agreement protocol attribute"

RES=$(echo "$agreementprotocolattribute" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/attribute")
if [ "$RES" == "" ]
then
  echo -e "$agreementprotocolattribute \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "protocol name fred is not supported" ]
then
  echo -e "$agreementprotocolattribute \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# blockchain not specified correctly
read -d '' agreementprotocolattribute <<EOF
{
  "type": "AgreementProtocolAttributes",
  "label": "Agreement Protocols",
  "publishable": true,
  "host_only": false,
  "mappings": {
    "protocols": [
      {
        "Basic": 5
      }
    ]
  }
}
EOF

echo -e "\n\n[D] agreement protocol payload: $agreementprotocolattribute"

echo "Setting workload independent agreement protocol attribute"

RES=$(echo "$agreementprotocolattribute" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/attribute")
if [ "$RES" == "" ]
then
  echo -e "$agreementprotocolattribute \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "blockchain value is not []interface{}, it is json.Number" ]
then
  echo -e "$agreementprotocolattribute \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# blockchain array not specified correctly
read -d '' agreementprotocolattribute <<EOF
{
  "type": "AgreementProtocolAttributes",
  "label": "Agreement Protocols",
  "publishable": true,
  "host_only": false,
  "mappings": {
    "protocols": [
      {
        "Basic": [5]
      }
    ]
  }
}
EOF

echo -e "\n\n[D] agreement protocol payload: $agreementprotocolattribute"

echo "Setting workload independent agreement protocol attribute"

RES=$(echo "$agreementprotocolattribute" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/attribute")
if [ "$RES" == "" ]
then
  echo -e "$agreementprotocolattribute \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "blockchain array element is not map[string]interface{}, it is json.Number" ]
then
  echo -e "$agreementprotocolattribute \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# blockchain type is not a number
read -d '' agreementprotocolattribute <<EOF
{
  "type": "AgreementProtocolAttributes",
  "label": "Agreement Protocols",
  "publishable": true,
  "host_only": false,
  "mappings": {
    "protocols": [
      {
        "Basic": [
          {
            "type": 5,
            "name": "hl1"
          }
        ]
      }
    ]
  }
}
EOF

echo -e "\n\n[D] agreement protocol payload: $agreementprotocolattribute"

echo "Setting workload independent agreement protocol attribute"

RES=$(echo "$agreementprotocolattribute" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/attribute")
if [ "$RES" == "" ]
then
  echo -e "$agreementprotocolattribute \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "blockchain type is not string, it is json.Number" ]
then
  echo -e "$agreementprotocolattribute \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# blockchain name is not a number
read -d '' agreementprotocolattribute <<EOF
{
  "type": "AgreementProtocolAttributes",
  "label": "Agreement Protocols",
  "publishable": true,
  "host_only": false,
  "mappings": {
    "protocols": [
      {
        "Basic": [
          {
            "type": "hyperledger",
            "name": 5
          }
        ]
      }
    ]
  }
}
EOF

echo -e "\n\n[D] agreement protocol payload: $agreementprotocolattribute"

echo "Setting workload independent agreement protocol attribute"

RES=$(echo "$agreementprotocolattribute" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/attribute")
if [ "$RES" == "" ]
then
  echo -e "$agreementprotocolattribute \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "blockchain name is not string, it is json.Number" ]
then
  echo -e "$agreementprotocolattribute \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# blockchain type is not one of the supported types for Basic protocol
read -d '' agreementprotocolattribute <<EOF
{
  "type": "AgreementProtocolAttributes",
  "label": "Agreement Protocols",
  "publishable": true,
  "host_only": false,
  "mappings": {
    "protocols": [
      {
        "Basic": [
          {
            "type": "hyperledger",
            "name": "hl1"
          }
        ]
      }
    ]
  }
}
EOF

echo -e "\n\n[D] agreement protocol payload: $agreementprotocolattribute"

echo "Setting workload independent agreement protocol attribute"

RES=$(echo "$agreementprotocolattribute" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/attribute")
if [ "$RES" == "" ]
then
  echo -e "$agreementprotocolattribute \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "blockchain type hyperledger is not supported for protocol Basic" ]
then
  echo -e "$agreementprotocolattribute \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# ==================================================================
# Now testing service specific agreement protocol attributes

# missing protocol specification
read -d '' netspeedservice <<EOF
{
  "${SERVICE_URL}": "https://bluehorizon.network/${SERVICE_MODE}s/network",
  "${SERVICE_ORG}": "IBM",
  "${SERVICE_NAME}": "network",
  "${SERVICE_VERSION}": "1.0.0",
  "attributes": [
    {
      "type": "AgreementProtocolAttributes",
      "label": "Agreement Protocols",
      "publishable": true,
      "host_only": false,
      "mappings": {}
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

# empty protocol array
read -d '' netspeedservice <<EOF
{
  "${SERVICE_URL}": "https://bluehorizon.network/${SERVICE_MODE}s/network",
  "${SERVICE_ORG}": "IBM",
  "${SERVICE_NAME}": "network",
  "${SERVICE_VERSION}": "1.0.0",
  "attributes": [
    {
      "type": "AgreementProtocolAttributes",
      "label": "Agreement Protocols",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "protocols": []
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
if [ "$ERR" != "array value is empty" ]
then
  echo -e "$netspeedservice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# invalid value type for protocol
read -d '' netspeedservice <<EOF
{
  "${SERVICE_URL}": "https://bluehorizon.network/${SERVICE_MODE}s/network",
  "${SERVICE_ORG}": "IBM",
  "${SERVICE_NAME}": "network",
  "${SERVICE_VERSION}": "1.0.0",
  "attributes": [
    {
      "type": "AgreementProtocolAttributes",
      "label": "Agreement Protocols",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "protocols": 5
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
if [ "$ERR" != "expected []interface{} received json.Number" ]
then
  echo -e "$netspeedservice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# invalid value type for protocol
read -d '' netspeedservice <<EOF
{
  "${SERVICE_URL}": "https://bluehorizon.network/${SERVICE_MODE}s/network",
  "${SERVICE_ORG}": "IBM",
  "${SERVICE_NAME}": "network",
  "${SERVICE_VERSION}": "1.0.0",
  "attributes": [
    {
      "type": "AgreementProtocolAttributes",
      "label": "Agreement Protocols",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "protocols": [5]
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
if [ "$ERR" != "array value is not a map[string]interface{}, it is json.Number" ]
then
  echo -e "$netspeedservice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# fred is not a known protocol
read -d '' netspeedservice <<EOF
{
  "${SERVICE_URL}": "https://bluehorizon.network/${SERVICE_MODE}s/network",
  "${SERVICE_ORG}": "IBM",
  "${SERVICE_NAME}": "network",
  "${SERVICE_VERSION}": "1.0.0",
  "attributes": [
    {
      "type": "AgreementProtocolAttributes",
      "label": "Agreement Protocols",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "protocols": [
          {
            "fred": []
          }
        ]
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
if [ "$ERR" != "protocol name fred is not supported" ]
then
  echo -e "$netspeedservice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# invalid value type for protocol
read -d '' netspeedservice <<EOF
{
  "${SERVICE_URL}": "https://bluehorizon.network/${SERVICE_MODE}s/network",
  "${SERVICE_ORG}": "IBM",
  "${SERVICE_NAME}": "network",
  "${SERVICE_VERSION}": "1.0.0",
  "attributes": [
    {
      "type": "AgreementProtocolAttributes",
      "label": "Agreement Protocols",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "protocols": [
          {
            "Basic": 5
          }
        ]
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
if [ "$ERR" != "blockchain value is not []interface{}, it is json.Number" ]
then
  echo -e "$netspeedservice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# invalid value type for protocol
read -d '' netspeedservice <<EOF
{
  "${SERVICE_URL}": "https://bluehorizon.network/${SERVICE_MODE}s/network",
  "${SERVICE_ORG}": "IBM",
  "${SERVICE_NAME}": "network",
  "${SERVICE_VERSION}": "1.0.0",
  "attributes": [
    {
      "type": "AgreementProtocolAttributes",
      "label": "Agreement Protocols",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "protocols": [
          {
            "Basic": [5]
          }
        ]
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
if [ "$ERR" != "blockchain array element is not map[string]interface{}, it is json.Number" ]
then
  echo -e "$netspeedservice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# invalid value type for blockchain type
read -d '' netspeedservice <<EOF
{
  "${SERVICE_URL}": "https://bluehorizon.network/${SERVICE_MODE}s/network",
  "${SERVICE_ORG}": "IBM",
  "${SERVICE_NAME}": "network",
  "${SERVICE_VERSION}": "1.0.0",
  "attributes": [
    {
      "type": "AgreementProtocolAttributes",
      "label": "Agreement Protocols",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "protocols": [
          {
            "Basic": [
              {
                "type": 5,
                "name": "hl1"
              }
            ]
          }
        ]
      }
    }
  ]
}
EOF

echo -e "\n\n[D] agreement protocol payload: $netspeedservice"

echo "Setting workload independent agreement protocol attribute"

RES=$(echo "$netspeedservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/${SERVICE_MODE}/config")
if [ "$RES" == "" ]
then
  echo -e "$netspeedservice \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "blockchain type is not string, it is json.Number" ]
then
  echo -e "$netspeedservice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# invalid value type for blockchain name
read -d '' netspeedservice <<EOF
{
  "${SERVICE_URL}": "https://bluehorizon.network/${SERVICE_MODE}s/network",
  "${SERVICE_ORG}": "IBM",
  "${SERVICE_NAME}": "network",
  "${SERVICE_VERSION}": "1.0.0",
  "attributes": [
    {
      "type": "AgreementProtocolAttributes",
      "label": "Agreement Protocols",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "protocols": [
          {
            "Basic": [
              {
                "type": "hyperledger",
                "name": 5
              }
            ]
          }
        ]
      }
    }
  ]
}
EOF

echo -e "\n\n[D] agreement protocol payload: $netspeedservice"

echo "Setting workload independent agreement protocol attribute"

RES=$(echo "$netspeedservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/${SERVICE_MODE}/config")
if [ "$RES" == "" ]
then
  echo -e "$netspeedservice \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "blockchain name is not string, it is json.Number" ]
then
  echo -e "$netspeedservice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# invalid blockchain type for known protocol
read -d '' netspeedservice <<EOF
{
  "${SERVICE_URL}": "https://bluehorizon.network/${SERVICE_MODE}s/network",
  "${SERVICE_ORG}": "IBM",
  "${SERVICE_NAME}": "network",
  "${SERVICE_VERSION}": "1.0.0",
  "attributes": [
    {
      "type": "AgreementProtocolAttributes",
      "label": "Agreement Protocols",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "protocols": [
          {
            "Basic": [
              {
                "type": "hyperledger",
                "name": "hl1"
              }
            ]
          }
        ]
      }
    }
  ]
}
EOF

echo -e "\n\n[D] agreement protocol payload: $netspeedservice"

echo "Setting workload independent agreement protocol attribute"

RES=$(echo "$netspeedservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/${SERVICE_MODE}/config")
if [ "$RES" == "" ]
then
  echo -e "$netspeedservice \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "blockchain type hyperledger is not supported for protocol Basic" ]
then
  echo -e "$netspeedservice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# ==========================================================================================
# Now testing valid service specific agreement protocol attributes that will never be used.

if [ "$PATTERN" == "" ]
then

read -d '' netspeedservice <<EOF
{
  "${SERVICE_URL}": "https://bluehorizon.network/${SERVICE_MODE}s/no-such-service",
  "${SERVICE_NAME}": "no-such",
  "${SERVICE_VERSION}": "1.0.0",
  "attributes": [
    {
      "type": "AgreementProtocolAttributes",
      "label": "Agreement Protocols",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "protocols": [
          {
            "Basic": []
          }
        ]
      }
    }
  ]
}
EOF

echo -e "\n\n[D] agreement protocol payload: $netspeedservice"

echo "Setting workload independent agreement protocol attribute"

RES=$(echo "$netspeedservice" | curl -sS -o /dev/null -w "%{http_code}" -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/${SERVICE_MODE}/config")
if [ "$RES" != "201" ]
then
  echo -e "$netspeedservice \nresulted in $RES response"
  exit 2
else
  echo -e "found expected response: success"
fi

else

# When patterns are in use, the device side cannot set any policy, there should be an error
read -d '' netspeedservice <<EOF
{
  "${SERVICE_URL}": "https://bluehorizon.network/${SERVICE_MODE}s/no-such-service",
  "${SERVICE_NAME}": "no-such",
  "${SERVICE_VERSION}": "1.0.0",
  "attributes": [
    {
      "type": "AgreementProtocolAttributes",
      "label": "Agreement Protocols",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "protocols": [
          {
            "Basic": []
          }
        ]
      }
    }
  ]
}
EOF

echo -e "\n\n[D] agreement protocol payload: $netspeedservice"

echo "Setting workload independent agreement protocol attribute"

RES=$(echo "$netspeedservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/${SERVICE_MODE}/config")
if [ "$RES" == "" ]
then
  echo -e "$netspeedservice \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "${ERR:0:25}" != "device is using a pattern" ]
then
  echo -e "$netspeedservice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

fi
