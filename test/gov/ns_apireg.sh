#!/bin/bash

source ./utils.sh

echo -e "\nBC setting is $BC"

if [ "$BC" != "1" ]
then

    echo -e "Pattern is set to $PATTERN"
    if [ "$PATTERN" == "" ]
    then

        # Configure the netspeed service variables, at an older version level just to be sure
        # that the runtime will still pick them up for the newer version that is installed in the exchange.
        # To test the services from different orgs with same url, we have setup 2 netspeed services.
        # IBM/netspeed depends on: IBM/nework, IBN/network2, IBM/cpu
        # e2edev@somecomp.com/netspeed depends on: e2edev@somecomp.com/network, e2edev@somecomp.com/network2, IBM/cpu e2edev@somecomp.com/cpu

        read -d '' snsconfig <<EOF
[
  {
    "serviceOrgid": "IBM",
    "serviceUrl": "https://bluehorizon.network/services/netspeed",
    "serviceArch": "${ARCH}",
    "serviceVersionRange": "[2.2.0,INFINITY)",
    "inputs": [
      {
        "name": "var1",
        "value": "aString"
      },
      {
        "name": "var2",
        "value": 5
      },
      {
        "name": "var3",
        "value": 22.2
      }
    ]
  },
  {
    "serviceOrgid": "e2edev@somecomp.com",
    "serviceUrl": "https://bluehorizon.network/services/netspeed",
    "serviceArch": "${ARCH}",
    "serviceVersionRange": "[2.2.0,INFINITY)",
    "inputs": [
      {
        "name": "var1",
        "value": "node_String"
      },
      {
        "name": "var2",
        "value": 20
      },
      {
        "name": "var3",
        "value": 23.2
      }
    ]
  },
  {
    "serviceOrgid": "IBM",
    "serviceUrl": "https://bluehorizon.network/service-cpu",
    "serviceArch": "${ARCH}",
    "serviceVersionRange": "[1.0.0,INFINITY)",
    "inputs": [
      {
        "name": "cpu_var1",
        "value": "ibmnodevar1"
      }
    ]
  },
  {
    "serviceOrgid": "e2edev@somecomp.com",
    "serviceUrl": "https://bluehorizon.network/service-cpu",
    "serviceArch": "${ARCH}",
    "inputs": [
      {
        "name": "cpu_var1",
        "value": "e2edevnodevar1"
      }
    ]
  }
]
EOF
        echo -e "\n\n[D] user input for netspeed service: $snsconfig"
        echo "Registering user input for netspeed service"
        RES=$(echo "$snsconfig" | curl -sS -X PATCH -w %{http_code} -H "Content-Type: application/json" --data @- "$ANAX_API/node/userinput")
        check_api_result "201" "$RES"

    elif [ "$PATTERN" == "sns" ] || [ "$PATTERN" == "sall" ]
    then

        # Configure the netspeed service variables, at an older version level just to be sure
        # that the runtime will still pick them up for the newer version that is installed in the exchange.

        read -d '' snsconfig <<EOF
[
  {
    "serviceOrgid": "IBM",
    "serviceUrl": "https://bluehorizon.network/services/netspeed",
    "serviceArch": "${ARCH}",
    "serviceVersionRange": "[2.2.0,INFINITY)",
    "inputs": [
      {
        "name": "var1",
        "value": "node_String"
      },
      {
        "name": "var2",
        "value": 20
      },
      {
        "name": "var3",
        "value": 20.2
      },
      {
        "name": "var4",
        "value": ["nodeabcd"]
      }
    ]
  },
  {
    "serviceOrgid": "e2edev@somecomp.com",
    "serviceUrl": "https://bluehorizon.network/services/netspeed",
    "serviceArch": "${ARCH}",
    "serviceVersionRange": "[2.2.0,INFINITY)",
    "inputs": [
      {
        "name": "var1",
        "value": "node_String"
      },
      {
        "name": "var2",
        "value": 21
      },
      {
        "name": "var3",
        "value": 21.2
      },
      {
        "name": "var4",
        "value": ["nodeabcd"]
      }
    ]
  },
  {
    "serviceOrgid": "IBM",
    "serviceUrl": "https://bluehorizon.network/service-cpu",
    "serviceArch": "${ARCH}",
    "serviceVersionRange": "[1.0.0,INFINITY)",
    "inputs": [
      {
        "name": "cpu_var1",
        "value": "ibmnodevar1"
      }
    ]
  },
  {
    "serviceOrgid": "e2edev@somecomp.com",
    "serviceUrl": "https://bluehorizon.network/service-cpu",
    "serviceArch": "${ARCH}",
    "inputs": [
      {
        "name": "cpu_var1",
        "value": "e2edevnodevar1"
      }
    ]
  }
]
EOF

        echo -e "\n\n[D] user input for netspeed service: $snsconfig"
        echo "Registering user input for netspeed service"
        RES=$(echo "$snsconfig" | curl -sS -w %{http_code} -X PATCH -H "Content-Type: application/json" --data @- "$ANAX_API/node/userinput")
        check_api_result "201" "$RES"

    fi
fi
