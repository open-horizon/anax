#!/bin/bash

# ==================================================================
# Begin testing service config API

EXCH_URL="http://${EXCH_APP_HOST:-172.17.0.1}:8080/v1"

# empty service URL
read -d '' snsconfig <<EOF
{
  "url": "",
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

echo -e "\n\n[D] service config payload: $snsconfig"

echo "Registering service config with empty URL"

RES=$(echo "$snsconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config")
if [ "$RES" == "" ]
then
  echo -e "$snsconfig \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "not specified" ]
then
  echo -e "$snsconfig \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# invalid version string
read -d '' snsconfig <<EOF
{
  "url": "https://bluehorizon.network/services/netspeed",
  "versionRange": "a",
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

echo "Registering netspeed service config with empty URL"

RES=$(echo "$snsconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config")
if [ "$RES" == "" ]
then
  echo -e "$snsconfig \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "${ERR:0:58}" != "versionRange a cannot be converted to a version expression" ]
then
  echo -e "$snsconfig \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# invalid attributes section
read -d '' snsconfig <<EOF
{
  "url": "https://bluehorizon.network/services/netspeed",
  "versionRange": "1.2.3",
  "organization": "IBM",
  "attributes": {}
}
EOF

echo -e "\n\n[D] netspeed service config payload: $snsconfig"

echo "Registering netspeed service config with invalid variables"

RES=$(echo "$snsconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config")
if [ "$RES" == "" ]
then
  echo -e "$snsconfig \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "${ERR:0:61}" != "Input body couldn't be deserialized to service/config object:" ]
then
  echo -e "$snsconfig \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# unknown service
read -d '' snsconfig <<EOF
{
  "url": "https://bluehorizon.network/services/testServiceX",
  "versionRange": "1.2.3",
  "organization": "IBM",
  "attributes": []
}
EOF

echo -e "\n\n[D] testServiceX service config payload: $snsconfig"

echo "Registering testServiceX service config with invalid variables"

RES=$(echo "$snsconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config")
if [ "$RES" == "" ]
then
  echo -e "$snsconfig \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "${ERR:0:37}" != "Unable to find the service definition" ]
then
  echo -e "$snsconfig \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# ==============================================================================
# context testcases here. Checking input config request against service definition

# setup fake service to use - testservice
# it has userInput variables:
#  var1 string
#  var2 int
#  var3 float
#  var4 list of strings
#  var5 string - default foo

echo -e "\nSetting up test service for context tests"

read -d '' service <<EOF
{
  "label":"test",
  "description":"test service",
  "public":false,
  "url":"https://bluehorizon.network/services/testservice",
  "version":"1.0.0",
  "arch":"amd64",
  "sharable":"multiple",
  "matchHardware":{},
  "userInput":[
    {
      "name":"var1",
      "label":"",
      "type":"string"
    },
    {
      "name":"var2",
      "label":"",
      "type":"int"
    },
    {
      "name":"var3",
      "label":"",
      "type":"float"
    },
    {
      "name":"var4",
      "label":"",
      "type":"list of strings"
    },
    {
      "name":"var5",
      "label":"",
      "type":"string",
      "defaultValue":"foo"
    }
  ],
  "deployment":"",
  "deploymentSignature":""
}
EOF

WLRES=$(echo "$service" | curl -sS -X POST -H "Content-Type: application/json" -H "Accept: application/json" -H "Authorization:Basic e2edev@somecomp.com/e2edevadmin:e2edevadminpw" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/services")
echo -e "Registered testwl: $WLRES"
MSG=$(echo $WLRES | jq -r ".msg")
if [ "$MSG" != "service 'e2edev@somecomp.com/bluehorizon.network-services-testservice_1.0.0_amd64' created" ]
then
  echo -e "Register testservice resulted in incorrect response: $WLRES"
  exit 2
else
  echo -e "found expected response: $MSG"
fi

# wrong variable type (number) in the variables section
read -d '' snsconfig <<EOF
{
  "url": "https://bluehorizon.network/services/testservice",
  "version": "1.0.0",
  "organization": "e2edev@somecomp.com",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "User input variables",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "var1": 5
      }
    }
  ]
}
EOF

echo -e "\n\n[D] testservice service config payload: $snsconfig"

echo "Registering testservice service config with wrong variable type int for string"

RES=$(echo "$snsconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config")
if [ "$RES" == "" ]
then
  echo -e "$snsconfig \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "variable var1 for service e2edev@somecomp.com/https://bluehorizon.network/services/testservice is type json.Number, expecting string." ]
then
  echo -e "$snsconfig \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# wrong variable type (array of string) in the variables section
read -d '' snsconfig <<EOF
{
  "url": "https://bluehorizon.network/services/testservice",
  "version": "1.0.0",
  "organization": "e2edev@somecomp.com",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "User input variables",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "var1":["a"]
      }
    }
  ]
}
EOF

echo -e "\n\n[D] testservice service config payload: $snsconfig"

echo "Registering testservice service config with wrong variable type array of string for string"

RES=$(echo "$snsconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config")
if [ "$RES" == "" ]
then
  echo -e "$snsconfig \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "variable var1 for service e2edev@somecomp.com/https://bluehorizon.network/services/testservice is type []interface {}, expecting string." ]
then
  echo -e "$snsconfig \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# wrong variable type object in the variables section
read -d '' snsconfig <<EOF
{
  "url": "https://bluehorizon.network/services/testservice",
  "version": "1.0.0",
  "organization": "e2edev@somecomp.com",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "User input variables",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "var1":{"a":"b"}
      }
    }
  ]
}
EOF

echo -e "\n\n[D] testservice service config payload: $snsconfig"

echo "Registering testservice service config with wrong variable type object for string"

RES=$(echo "$snsconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config")
if [ "$RES" == "" ]
then
  echo -e "$snsconfig \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "variable var1 for service e2edev@somecomp.com/https://bluehorizon.network/services/testservice is type map[string]interface {}, is an unexpected type." ]
then
  echo -e "$snsconfig \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# wrong variable type (string) in the variables section for an int
read -d '' snsconfig <<EOF
{
  "url": "https://bluehorizon.network/services/testservice",
  "version": "1.0.0",
  "organization": "e2edev@somecomp.com",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "User input variables",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "var2":"a"
      }
    }
  ]
}
EOF

echo -e "\n\n[D] testservice service config payload: $snsconfig"

echo "Registering testservice service config with wrong variable type string for int"

RES=$(echo "$snsconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config")
if [ "$RES" == "" ]
then
  echo -e "$snsconfig \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "variable var2 for service e2edev@somecomp.com/https://bluehorizon.network/services/testservice is type string, expecting int." ]
then
  echo -e "$snsconfig \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# wrong variable type (string) in the variables section for a float
read -d '' snsconfig <<EOF
{
  "url": "https://bluehorizon.network/services/testservice",
  "version": "1.0.0",
  "organization": "e2edev@somecomp.com",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "User input variables",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "var3":"a"
      }
    }
  ]
}
EOF

echo -e "\n\n[D] testservice service config payload: $snsconfig"

echo "Registering testservice service config with wrong variable type string for float"

RES=$(echo "$snsconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config")
if [ "$RES" == "" ]
then
  echo -e "$snsconfig \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "variable var3 for service e2edev@somecomp.com/https://bluehorizon.network/services/testservice is type string, expecting float." ]
then
  echo -e "$snsconfig \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# wrong variable type (string) in the variables section for list of strings
read -d '' snsconfig <<EOF
{
  "url": "https://bluehorizon.network/services/testservice",
  "version": "1.0.0",
  "organization": "e2edev@somecomp.com",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "User input variables",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "var4":"a"
      }
    }
  ]
}
EOF

echo -e "\n\n[D] testservice service config payload: $snsconfig"

echo "Registering testservice service config with wrong variable type string for list of strings"

RES=$(echo "$snsconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config")
if [ "$RES" == "" ]
then
  echo -e "$snsconfig \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "variable var4 for service e2edev@somecomp.com/https://bluehorizon.network/services/testservice is type string, expecting list of strings." ]
then
  echo -e "$snsconfig \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# wrong variable type (float) in the variables section for a int
read -d '' snsconfig <<EOF
{
  "url": "https://bluehorizon.network/services/testservice",
  "version": "1.0.0",
  "organization": "e2edev@somecomp.com",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "User input variables",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "var2":10.2
      }
    }
  ]
}
EOF

echo -e "\n\n[D] testservice service config payload: $snsconfig"

echo "Registering testservice service config with wrong variable type float for int"

RES=$(echo "$snsconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config")
if [ "$RES" == "" ]
then
  echo -e "$snsconfig \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "variable var2 for service e2edev@somecomp.com/https://bluehorizon.network/services/testservice is type float, expecting int." ]
then
  echo -e "$snsconfig \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# wrong variable type (int) in the variables section for a list of string
read -d '' snsconfig <<EOF
{
  "url": "https://bluehorizon.network/services/testservice",
  "version": "1.0.0",
  "organization": "e2edev@somecomp.com",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "User input variables",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "var4":5
      }
    }
  ]
}
EOF

echo -e "\n\n[D] testservice service config payload: $snsconfig"

echo "Registering testservice service config with wrong variable type int for list of string"

RES=$(echo "$snsconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config")
if [ "$RES" == "" ]
then
  echo -e "$snsconfig \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "variable var4 for service e2edev@somecomp.com/https://bluehorizon.network/services/testservice is type json.Number, expecting list of strings." ]
then
  echo -e "$snsconfig \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# wrong variable type (array numbers) in the variables section for a list of string
read -d '' snsconfig <<EOF
{
  "url": "https://bluehorizon.network/services/testservice",
  "version": "1.0.0",
  "organization": "e2edev@somecomp.com",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "User input variables",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "var4":[5]
      }
    }
  ]
}
EOF

echo -e "\n\n[D] testservice service config payload: $snsconfig"

echo "Registering testservice service config with wrong variable type []int for list of string"

RES=$(echo "$snsconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config")
if [ "$RES" == "" ]
then
  echo -e "$snsconfig \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "variable var4 for service e2edev@somecomp.com/https://bluehorizon.network/services/testservice is type []interface {}, expecting []string." ]
then
  echo -e "$snsconfig \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# ==============================================================================
# context testcases here. Checking userInput against API input

if [ "$PATTERN" != "" ]
then 
  # missing variable in the variables section
  read -d '' snsconfig <<EOF
{
  "url": "https://bluehorizon.network/services/testservice",
  "version": "1.0.0",
  "organization": "e2edev@somecomp.com",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "User input variables",
      "publishable": false,
      "host_only": false,
      "mappings": {}
    }
  ]
}
EOF

  echo -e "\n\n[D] testservice service config payload: $snsconfig"

  echo "Registering testservice service config with missing variable"

  RES=$(echo "$snsconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config")
  if [ "$RES" == "" ]
  then
    echo -e "$snsconfig \nresulted in empty response"
    exit 2
  fi

  ERR=$(echo $RES | jq -r ".error")
  if [ "$ERR" != "variable var1 for service e2edev@somecomp.com/https://bluehorizon.network/services/testservice is missing from mappings." ]
  then
    echo -e "$snsconfig \nresulted in incorrect response: $RES"
    exit 2
  else
   echo -e "found expected response: $RES"
  fi


 # another missing variable in the variables section
 read -d '' snsconfig <<EOF
{
  "url": "https://bluehorizon.network/services/testservice",
  "version": "1.0.0",
  "organization": "e2edev@somecomp.com",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "User input variables",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "var1":"a"
      }
    }
  ]
}
EOF

  echo -e "\n\n[D] testservice service config payload: $snsconfig"

  echo "Registering testservice service config with another missing variable"

  RES=$(echo "$snsconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config")
  if [ "$RES" == "" ]
  then
    echo -e "$snsconfig \nresulted in empty response"
    exit 2
  fi

  ERR=$(echo $RES | jq -r ".error")
  if [ "$ERR" != "variable var2 for service e2edev@somecomp.com/https://bluehorizon.network/services/testservice is missing from mappings." ]
  then
    echo -e "$snsconfig \nresulted in incorrect response: $RES"
    exit 2
  else
    echo -e "found expected response: $RES"
  fi
fi

# Configure the testservice service variables, at an older version level just to be sure
# that the runtime will still pick them up for the newer version that is installed in the exchange.
# The configstate tests that come after these service tests depend on the following to work correctly.
read -d '' snsconfig <<EOF
{
  "url": "https://bluehorizon.network/services/testservice",
  "version": "0.5.0",
  "organization": "e2edev@somecomp.com",
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

echo -e "\n\n[D] testservice service config payload: $snsconfig"

echo "Registering testservice service config on node"

ERR=$(echo "$snsconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
if [ "$ERR" != "null" ]; then
  echo -e "error occured: $ERR"
  exit 2
fi

# try to config again, get an error
read -d '' snsconfig <<EOF
{
  "url": "https://bluehorizon.network/services/testservice",
  "version": "2.2.0",
  "organization": "e2edev@somecomp.com",
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

echo -e "\n\n[D] testservice service config payload: $snsconfig"

echo "Registering testservice service config on node again"

RES=$(echo "$snsconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config")
if [ "$RES" == "" ]
then
  echo -e "$snsconfig \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "${ERR:0:95}" != "Duplicate registration for e2edev@somecomp.com/https://bluehorizon.network/services/testservice" ]
then
  echo -e "$snsconfig \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi
