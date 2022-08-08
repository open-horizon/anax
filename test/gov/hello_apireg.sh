#!/bin/bash

source ./utils.sh

echo -e "Pattern is set to $PATTERN"

if [ "$PATTERN" == "susehello" ] || [ "$PATTERN" == "sall" ]
then

  read -d '' helloconfig <<EOF
[
  {
    "serviceOrgid": "e2edev@somecomp.com",
    "serviceUrl": "my.company.com.services.usehello2",
    "serviceVersionRange": "[1.0.0,INFINITY)",
    "inputs": [
      {
        "name": "MY_VAR1",
        "value": "e2edev"
      }
    ]
  },
  {
    "serviceOrgid": "e2edev@somecomp.com",
    "serviceUrl": "my.company.com.services.hello2",
    "serviceVersionRange": "[1.0.0,INFINITY)",
    "inputs": [
      {
        "name": "MY_S_VAR1",
        "value": "node_String"
      }
    ]
  },
  {
    "serviceOrgid": "e2edev@somecomp.com",
    "serviceUrl": "my.company.com.services.cpu2",
    "serviceVersionRange": "[1.0.0,INFINITY)",
    "inputs": [
      {
        "name": "MY_CPU_VAR",
        "value": "e2edev"
      }
    ]
  }
]
EOF

  echo -e "\n\n[D] user input for usehello service: $helloconfig"

  echo "Registering user input for usehello service"

  RES=$(echo "$helloconfig" | curl -sS -w %{http_code} -X PATCH -H "Content-Type: application/json" --data @- "$ANAX_API/node/userinput")
  check_api_result "201" "$RES"

fi