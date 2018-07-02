#!/bin/bash

# ==================================================================
# Begin testing workload config API

EXCH_URL="http://${EXCH_APP_HOST:-172.17.0.1}:8080/v1"

# empty workload URL
read -d '' workloadconfig <<EOF
{
  "workload_url": "",
  "workload_version": "1.0.0",
  "attributes": []
}
EOF

echo -e "\n\n[D] workload config payload: $workloadconfig"

echo "Setting workload config with bad URL"

RES=$(echo "$workloadconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost/workload/config")
if [ "$RES" == "" ]
then
  echo -e "$workloadconfig \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "not specified" ]
then
  echo -e "$workloadconfig \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# invalid version string
read -d '' workloadconfig <<EOF
{
  "workload_url": "https://bluehorizon.network/workloads/testwl",
  "workload_version": "a",
  "attributes": []
}
EOF

echo -e "\n\n[D] workload config payload: $workloadconfig"

echo "Setting workload config with invalid version"

RES=$(echo "$workloadconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost/workload/config")
if [ "$RES" == "" ]
then
  echo -e "$workloadconfig \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "workload_version a is not a valid version string or expression" ]
then
  echo -e "$workloadconfig \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# invalid attributes section
read -d '' workloadconfig <<EOF
{
  "workload_url": "https://bluehorizon.network/workloads/testwl",
  "workload_version": "1.2.3",
  "attributes": {}
}
EOF

echo -e "\n\n[D] workload config payload: $workloadconfig"

echo "Setting workload config with invalid variables"

RES=$(echo "$workloadconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost/workload/config")
if [ "$RES" == "" ]
then
  echo -e "$workloadconfig \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "Input body could not be demarshalled, error: json: cannot unmarshal object into Go struct field WorkloadConfig.attributes of type []api.Attribute" ]
then
  echo -e "$workloadconfig \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# unknown workload
read -d '' workloadconfig <<EOF
{
  "workload_url": "https://bluehorizon.network/workloads/testwlX",
  "workload_version": "1.2.3",
  "attributes": []
}
EOF

echo -e "\n\n[D] workload config payload: $workloadconfig"

echo "Setting workload config with unknown workload"

RES=$(echo "$workloadconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost/workload/config")
if [ "$RES" == "" ]
then
  echo -e "$workloadconfig \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "unable to find the workload definition using https://bluehorizon.network/workloads/testwlX e2edev [1.2.3,INFINITY) amd64 in the exchange." ]
then
  echo -e "$workloadconfig \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# ==============================================================================
# context testcases here. Checking input config request against workload definition

# setup fake workloads to use - testwl
# it has userInput variables:
#  var1 string
#  var2 int
#  var3 float
#  var4 list of strings
#  var5 string - default foo

echo -e "\nSetting up test workload for context tests"

read -d '' workload <<EOF
{
  "label":"test",
  "description":"test workload",
  "public":false,
  "workloadUrl":"https://bluehorizon.network/workloads/testwl",
  "version":"1.0.0",
  "arch":"amd64",
  "downloadUrl":"",
  "apiSpec":[],
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
  "workloads":[]
}
EOF

WLRES=$(echo "$workload" | curl -sS -X POST -H "Content-Type: application/json" -H "Accept: application/json" -H "Authorization:Basic e2edev/e2edevadmin:e2edevadminpw" --data @- "${EXCH_URL}/orgs/e2edev/workloads")
echo -e "Registered testwl: $WLRES"
MSG=$(echo $WLRES | jq -r ".msg")
if [ "$MSG" != "workload 'e2edev/bluehorizon.network-workloads-testwl_1.0.0_amd64' created" ]
then
  echo -e "Register testwl resulted in incorrect response: $WLRES"
  exit 2
else
  echo -e "found expected response: $MSG"
fi

# wrong variable type (number) in the variables section
read -d '' workloadconfig <<EOF
{
  "workload_url": "https://bluehorizon.network/workloads/testwl",
  "workload_version": "1.0.0",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "app",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "var1": 5
      }
    }
  ]
}
EOF

echo -e "\n\n[D] workload config payload: $workloadconfig"

echo "Setting workload config with wrong variable type"

RES=$(echo "$workloadconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost/workload/config")
if [ "$RES" == "" ]
then
  echo -e "$workloadconfig \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "WorkloadConfig variable var1 is type json.Number, expecting string" ]
then
  echo -e "$workloadconfig \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# wrong variable type (array of string) in the variables section
read -d '' workloadconfig <<EOF
{
  "workload_url": "https://bluehorizon.network/workloads/testwl",
  "workload_version": "1.0.0",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "app",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "var1":["a"]
      }
    }
  ]
}
EOF

echo -e "\n\n[D] workload config payload: $workloadconfig"

echo "Setting workload config with wrong variable type"

RES=$(echo "$workloadconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost/workload/config")
if [ "$RES" == "" ]
then
  echo -e "$workloadconfig \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "WorkloadConfig variable var1 is type []interface {}, expecting string" ]
then
  echo -e "$workloadconfig \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# wrong variable type (array of string) in the variables section
read -d '' workloadconfig <<EOF
{
  "workload_url": "https://bluehorizon.network/workloads/testwl",
  "workload_version": "1.0.0",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "app",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "var1":{"a":"b"}
      }
    }
  ]
}
EOF

echo -e "\n\n[D] workload config payload: $workloadconfig"

echo "Setting workload config with wrong variable type"

RES=$(echo "$workloadconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost/workload/config")
if [ "$RES" == "" ]
then
  echo -e "$workloadconfig \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "WorkloadConfig variable var1 is type map[string]interface {}, is an unexpected type." ]
then
  echo -e "$workloadconfig \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# wrong variable type (string) in the variables section for an int
read -d '' workloadconfig <<EOF
{
  "workload_url": "https://bluehorizon.network/workloads/testwl",
  "workload_version": "1.0.0",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "app",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "var1":"a","var2":"abc"
      }
    }
  ]
}
EOF

echo -e "\n\n[D] workload config payload: $workloadconfig"

echo "Setting workload config with wrong variable type"

RES=$(echo "$workloadconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost/workload/config")
if [ "$RES" == "" ]
then
  echo -e "$workloadconfig \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "WorkloadConfig variable var2 is type string, expecting int" ]
then
  echo -e "$workloadconfig \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# wrong variable type (string) in the variables section for a float
read -d '' workloadconfig <<EOF
{
  "workload_url": "https://bluehorizon.network/workloads/testwl",
  "workload_version": "1.0.0",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "app",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "var1":"a","var2":5,"var3":"a"
      }
    }
  ]
}
EOF

echo -e "\n\n[D] workload config payload: $workloadconfig"

echo "Setting workload config with wrong variable type"

RES=$(echo "$workloadconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost/workload/config")
if [ "$RES" == "" ]
then
  echo -e "$workloadconfig \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "WorkloadConfig variable var3 is type string, expecting float" ]
then
  echo -e "$workloadconfig \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# wrong variable type (string) in the variables section for a float
read -d '' workloadconfig <<EOF
{
  "workload_url": "https://bluehorizon.network/workloads/testwl",
  "workload_version": "1.0.0",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "app",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "var1":"a","var2":5,"var3":10.2,"var4":"a"
      }
    }
  ]
}
EOF

echo -e "\n\n[D] workload config payload: $workloadconfig"

echo "Setting workload config with wrong variable type"

RES=$(echo "$workloadconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost/workload/config")
if [ "$RES" == "" ]
then
  echo -e "$workloadconfig \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "WorkloadConfig variable var4 is type string, expecting list of strings" ]
then
  echo -e "$workloadconfig \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# wrong variable type (float) in the variables section for a int
read -d '' workloadconfig <<EOF
{
  "workload_url": "https://bluehorizon.network/workloads/testwl",
  "workload_version": "1.0.0",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "app",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "var1":"a","var2":10.2
      }
    }
  ]
}
EOF

echo -e "\n\n[D] workload config payload: $workloadconfig"

echo "Setting workload config with wrong variable type"

RES=$(echo "$workloadconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost/workload/config")
if [ "$RES" == "" ]
then
  echo -e "$workloadconfig \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "WorkloadConfig variable var2 is type float, expecting int" ]
then
  echo -e "$workloadconfig \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# wrong variable type (int) in the variables section for a list of string
read -d '' workloadconfig <<EOF
{
  "workload_url": "https://bluehorizon.network/workloads/testwl",
  "workload_version": "1.0.0",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "app",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "var1":"a","var2":5,"var3":10.2,"var4":5
      }
    }
  ]
}
EOF

echo -e "\n\n[D] workload config payload: $workloadconfig"

echo "Setting workload config with incorrect variable types"

RES=$(echo "$workloadconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost/workload/config")
if [ "$RES" == "" ]
then
  echo -e "$workloadconfig \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "WorkloadConfig variable var4 is type json.Number, expecting list of strings" ]
then
  echo -e "$workloadconfig \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# wrong variable type (array numbers) in the variables section for a list of string
read -d '' workloadconfig <<EOF
{
  "workload_url": "https://bluehorizon.network/workloads/testwl",
  "workload_version": "1.0.0",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "app",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "var1":"a","var2":5,"var3":10.2,"var4":[5]
      }
    }
  ]
}
EOF

echo -e "\n\n[D] workload config payload: $workloadconfig"

echo "Setting workload config with incorrect variable types"

RES=$(echo "$workloadconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost/workload/config")
if [ "$RES" == "" ]
then
  echo -e "$workloadconfig \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "WorkloadConfig variable var4 is type []interface {}, expecting []string" ]
then
  echo -e "$workloadconfig \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# unknown variable name in the variables section
read -d '' workloadconfig <<EOF
{
  "workload_url": "https://bluehorizon.network/workloads/testwl",
  "workload_version": "1.0.0",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "app",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "var1":"a","var2":5,"var3":10.2,"var4":["abc"],"varX":"oops"
      }
    }
  ]
}
EOF

echo -e "\n\n[D] workload config payload: $workloadconfig"

echo "Setting workload config with unknown variable name"

RES=$(echo "$workloadconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost/workload/config")
if [ "$RES" == "" ]
then
  echo -e "$workloadconfig \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "unable to find the workload config variable varX in workload definition https://bluehorizon.network/workloads/testwl e2edev [1.0.0,INFINITY) amd64" ]
then
  echo -e "$workloadconfig \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# ==============================================================================
# context testcases here. Checking userInput against API input

# missing variable in the variables section
read -d '' workloadconfig <<EOF
{
  "workload_url": "https://bluehorizon.network/workloads/testwl",
  "workload_version": "1.0.0",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "app",
      "publishable": false,
      "host_only": false,
      "mappings": {}
    }
  ]
}
EOF

echo -e "\n\n[D] workload config payload: $workloadconfig"

echo "Setting workload config with missing variable"

RES=$(echo "$workloadconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost/workload/config")
if [ "$RES" == "" ]
then
  echo -e "$workloadconfig \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "WorkloadConfig does not set var1, which has no default value" ]
then
  echo -e "$workloadconfig \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# another missing variable in the variables section
read -d '' workloadconfig <<EOF
{
  "workload_url": "https://bluehorizon.network/workloads/testwl",
  "workload_version": "1.0.0",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "app",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "var1":"a"
      }
    }
  ]
}
EOF

echo -e "\n\n[D] workload config payload: $workloadconfig"

echo "Setting workload config with missing variable"

RES=$(echo "$workloadconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost/workload/config")
if [ "$RES" == "" ]
then
  echo -e "$workloadconfig \nresulted in empty response"
  exit 2
fi

ERR=$(echo $RES | jq -r ".error")
if [ "$ERR" != "WorkloadConfig does not set var2, which has no default value" ]
then
  echo -e "$workloadconfig \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi


# ========================================================================================
# successfully write the config
read -d '' workloadconfig <<EOF
{
  "workload_url": "https://bluehorizon.network/workloads/testwl",
  "workload_version": "1.0.0",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "app",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "var1":"a","var2":5,"var3":10.2,"var4":["abc"]
      }
    }
  ]
}
EOF

echo -e "\n\n[D] workload config payload: $workloadconfig"

echo "Setting workload config with correct variables"

RES=$(echo "$workloadconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost/workload/config")
if [ "$RES" == "" ]
then
  echo -e "resulted in unexpected empty response: $RES"
  exit 2
else
  echo -e "Workload config created."
fi

# try to config again, get an error
read -d '' workloadconfig <<EOF
{
  "workload_url": "https://bluehorizon.network/workloads/testwl",
  "workload_version": "1.0.0",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "app",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "var1":"a","var2":5,"var3":10.2,"var4":["abc"]
      }
    }
  ]
}
EOF

echo -e "\n\n[D] workload config payload: $workloadconfig"

echo "Setting workload config again, in error"

RES=$(echo "$workloadconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost/workload/config")
if [ "$RES" == "" ]
then
  echo -e "$workloadconfig \nresulted in empty response"
  exit 2
fi

if [ "$RES" != "workloadconfig already exists" ]
then
  echo -e "$workloadconfig \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# dump out the workload config record
echo -e "\nDumping contents of configured workload with GET API."
RES=$(curl -sS "http://localhost/workload/config" | jq -r '.')
if [ "$RES" == "" ]
then
  echo -e "Get resulted in unexpected empty response"
  exit 2
else
  echo -e "Configured testwl:\n $RES"
fi

# Now delete the workload config record
read -d '' workloadconfig <<EOF
{
  "workload_url": "https://bluehorizon.network/workloads/testwl",
  "organization": "e2edev",
  "workload_version": "1.0.0"
}
EOF

echo -e "\n\n[D] workload config delete payload: $workloadconfig"

echo "Deleting workload config"

RES=$(echo "$workloadconfig" | curl -sS -X DELETE -H "Content-Type: application/json" --data @- "http://localhost/workload/config")
if [ "$RES" != "" ]
then
  echo -e "resulted in unexpected non-empty response: $RES"
  exit 2
else
  echo -e "Workload config deleted."
fi

# ========================================================================================
# dump out the workload config record, should be gone.

echo -e "\nDumping contents of configured workload with GET API."
RES=$(curl -sS "http://localhost/workload/config" | jq -r '.config')
if [ "$RES" == "[]" ]
then
  echo -e "Get resulted in expected empty response"
else
  echo -e "Get resulted in unexpected response:\n $RES"
  exit 2
fi

# Now delete the workload config record again
read -d '' workloadconfig <<EOF
{
  "workload_url": "https://bluehorizon.network/workloads/testwl",
  "organization": "e2edev",
  "workload_version": "1.0.0"
}
EOF

echo -e "\n\n[D] workload config delete payload: $workloadconfig"

echo "Deleting workload config, should be error"

RES=$(echo "$workloadconfig" | curl -sS -X DELETE -H "Content-Type: application/json" --data @- "http://localhost/workload/config")
if [ "$RES" == "" ]
then
  echo -e "resulted in unexpected empty response"
  exit 2
fi

ERR=$(echo "$RES" | jq -r '.error')
if [ "$ERR" != "WorkloadConfig not found" ]
then
  echo -e "resulted in unexpected response: $RES"
  exit 2
else
  echo -e "Workload config not found as expected."
fi
