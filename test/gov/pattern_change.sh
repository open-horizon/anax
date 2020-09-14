#!/bin/bash

export HZN_EXCHANGE_URL="${EXCH_APP_HOST}"
E2EDEV_ADMIN_AUTH="e2edev@somecomp.com/e2edevadmin:e2edevadminpw"
USERDEV_ADMIN_AUTH="userdev/userdevadmin:userdevadminpw"
PREFIX="Pattern Change Test:"

unset HZN_ORG_ID

echo ""
echo -e "${PREFIX} Start node pattern change test"

cat <<EOF > /tmp/userinput_for_sns.json
[
  {
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
    ],
    "serviceArch": "amd64",
    "serviceOrgid": "IBM",
    "serviceUrl": "https://bluehorizon.network/services/netspeed",
    "serviceVersionRange": "[2.2.0,INFINITY)"
  },
  {
    "inputs": [
      {
        "name": "cpu_var1",
        "value": "ibmnodevar1"
      }
    ],
    "serviceArch": "amd64",
    "serviceOrgid": "IBM",
    "serviceUrl": "https://bluehorizon.network/service-cpu",
    "serviceVersionRange": "[0.0.0,INFINITY)"
  }
]
EOF

cat <<EOF > /tmp/userinput_for_sall.json
{
  "userInput": [
    {
      "serviceOrgid": "IBM",
      "serviceUrl": "https://bluehorizon.network/services/netspeed",
      "serviceArch": "amd64",
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
          "value": ["node_String1", "node_String2", "node_String3"]
        }
      ]
    },
    {
      "serviceOrgid": "e2edev@somecomp.com",
      "serviceUrl": "https://bluehorizon.network/services/netspeed",
      "serviceArch": "amd64",
      "serviceVersionRange": "2.2.0",
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
          "value": []
        }
      ]
    },
    {
      "serviceOrgid": "IBM",
      "serviceUrl": "https://bluehorizon.network/service-cpu",
      "serviceArch": "amd64",
      "serviceVersionRange": "[0.0.0,INFINITY)",
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
      "serviceArch": "amd64",
      "serviceVersionRange": "1.0.0",
      "inputs": [
        {
          "name": "cpu_var1",
          "value": "e2edevnodevar1"
        }
      ]
    },
    {
      "serviceOrgid": "e2edev@somecomp.com",
      "serviceUrl": "https://bluehorizon.network/services/locgps",
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
        },
        {
          "name": "test",
          "value": "testValue"
        },
        {
          "name": "extra",
          "value": "extraValue"
        }
      ]
    },
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
    },
    {
      "serviceOrgid": "e2edev@somecomp.com",
      "serviceUrl": "https://bluehorizon.network/services/weather",
      "serviceArch": "amd64",
      "serviceVersionRange": "1.5.0",
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
        },
        {
          "name": "HZN_WUGNAME",
          "value": "e2edev mocked pws"
        },
        {
          "name": "HZN_PWS_MODEL",
          "value": "LaCrosse WS2317"
        },
        {
          "name": "MTN_PWS_MODEL",
          "value": "LaCrosse WS2317"
        },
        {
          "name": "HZN_PWS_ST_TYPE",
          "value": "WS23xx"
        },
        {
          "name": "MTN_PWS_ST_TYPE",
          "value": "WS23xx"
        }
      ]
    },
    {
      "serviceOrgid": "e2edev@somecomp.com",
      "serviceUrl": "my.company.com.services.usehello2",
      "serviceArch": "amd64",
      "serviceVersionRange": "[0.0.0,INFINITY)",
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
      "serviceArch": "amd64",
      "serviceVersionRange": "[0.0.0,INFINITY)",
      "inputs": [
        {
          "name": "MY_S_VAR1",
          "value": "e2edev"
        }
      ]
    },
    {
      "serviceOrgid": "e2edev@somecomp.com",
      "serviceUrl": "my.company.com.services.cpu2",
      "serviceArch": "amd64",
      "serviceVersionRange": "[0.0.0,INFINITY)",
      "inputs": [
        {
          "name": "MY_CPU_VAR",
          "value": "e2edev"
        }
      ]
    }
  ]
}
EOF

function results {
  if [ "$(echo "$1" | jq -r '.code')" != "ok" ]
  then
    echo -e "Error: $(echo "$1" | jq -r '.msg')"
    exit 2
  fi
}

# make sure agreements are up and running
function verify_agreements {
  HZN_REG_TEST=1 ./verify_agreements.sh
  if [ $? -ne 0 ]; then
    echo -e "${PREFIX} Failed to verify agreement."
    exit 1
  fi
}

# check if current node pattern is the same as the given pattern
function checkNodePattern {
    pattern=$1

    echo "Checking if device has the new pattern name $pattern."
    ret=$(hzn node list |jq '.pattern')
    if [ $? -ne 0 ]; then
        echo -e "${PREFIX} Error: failed getting node. $ret"
        exit 1
    elif [ $ret != "\"$pattern\"" ]; then
        echo -e "${PREFIX} Error: the node pattern has not changed. $ret"
        exit 1
    else
        echo -e "${PREFIX} The node pattern has changed to $ret"
    fi
}

if [ ${CERT_LOC} -eq "1" ]; then
  CERT_VAR="--cacert /certs/css.crt"
else
  CERT_VAR=""
fi

# get the node org, it can be userdev or e2edev@somecomp.com
ret=$(hzn node list |jq '.organization')
if [ $? -ne 0 ]; then
    echo -e "${PREFIX} Failed getting node. $ret"
    exit 1
fi

if [ $ret == '"userdev"' ]; then
    org="userdev"
    auth=${USERDEV_ADMIN_AUTH}
else
    org="e2edev@somecomp.com"
    auth=${E2EDEV_ADMIN_AUTH}
fi

# change the exchange node pattern to sns
echo -e "${PREFIX} Change the userinput for node in ${org}"
ret=$(hzn userinput add -f /tmp/userinput_for_sns.json)
if [ $? -ne 0 ]; then
    echo -e "${PREFIX} Failed changing the user input for local node. $ret"
    exit 1
fi

echo -e "${PREFIX} change node pattern on the exchange to sns"
RES=$(curl -sLX PATCH $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json'  -u $auth  -d  '{"pattern":"e2edev@somecomp.com/sns"}' "${HZN_EXCHANGE_URL}/orgs/$org/nodes/an12345")
results "$RES"

echo "Sleeping 90 seconds..."
sleep 90

checkNodePattern "e2edev@somecomp.com/sns"
verify_agreements


# now change the pattern to sall, this will fail because there is not enough user input
echo -e "${PREFIX} change node pattern back on the exchange to sall"
RES=$(curl -sLX PATCH $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json'  -u $auth  -d  '{"pattern":"e2edev@somecomp.com/sall"}' "${HZN_EXCHANGE_URL}/orgs/$org/nodes/an12345")
results "$RES"

echo "Sleeping 30 seconds..."
sleep 30

ret=$(hzn eventlog list | grep 'Error validating new node pattern e2edev@somecomp.com/sall')
if [ $? -ne 0 ]; then
    echo -e "${PREFIX} New pattern verifcation should have failed, but it did not"
    exit 1
fi

# make sure the node still use the old pattern
ret=$(hzn node list |jq '.pattern')
if [ $? -ne 0 ]; then
    echo -e "${PREFIX} Error: failed getting node. $ret"
    exit 1
elif [ $ret != "\"e2edev@somecomp.com/sns\"" ]; then
    echo -e "${PREFIX} Error: the node pattern should stays the same but got changed to  $ret"
    exit 1
else
    echo -e "${PREFIX} The node pattern stays the same: $ret"
fi

# now assign correct user input for pattern sall
echo -e "${PREFIX} Change the userinput for node"
ret=$(hzn exchange node update -u $auth -o $org an12345 -f /tmp/userinput_for_sall.json)
if [ $? -ne 0 ]; then
    echo -e "${PREFIX} Failed changing the user input for local node. $ret"
    exit 1
fi

echo "Sleeping 60 seconds..."
sleep 60

# the pattern should have change on local node
checkNodePattern "e2edev@somecomp.com/sall"
./verify_agreements.sh
if [ $? -ne 0 ]; then
  echo -e "${PREFIX} Failed to verify agreement."
  exit 1
fi

echo -e "${PREFIX} Complete node pattern change test"
