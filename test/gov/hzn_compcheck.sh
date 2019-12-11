#!/bin/bash

USERDEV_ADMIN_AUTH="userdev/userdevadmin:userdevadminpw"
export HZN_EXCHANGE_URL="${EXCH_APP_HOST}"

unset HZN_ORG_ID
unset HZN_EXCHANGE_NODE_AUTH
unset HZN_EXCHANGE_USER_AUTH

PREFIX="HZN deployment compatibility test:"


echo ""
echo -e "${PREFIX} start test"

# check the the result to see if it matches the expected http code and error
function results {
 
  echo "$1"

  # check error text
  if [ ! -z "$2" ]; then
    res=$(echo "$1" | grep "$2")
    if [ $? -ne 0 ]; then
      echo -e "Error: the response should have contained \"$2\", but not. "
      exit 2
    fi
  fi

  # check error code
  if [ ! -z "$3" ]; then
    res=$(echo "$1" | grep "$3")
    if [ $? -ne 0 ]; then
      echo -e "Error: the response should have contained \"$3\", but not. "
      exit 2
    fi
  fi

  echo -e "Result expected."
}

# check the good result to see if the compatible and reason are correct.
function check_comp_results {
  echo "$1"

  if [ -z "$1" ]; then
    echo "No result to compare."
    exit 2
  fi

  comp=$(echo $1 | jq -r ".compatible")
  reason=$(echo $1 | jq -r ".reason")

  if [ "$comp" != "$2" ]; then
    echo "Expexted compatible be $2 but got $comp."
    exit 2
  fi

  if [ ! -z "$3" ]; then
    res=$(echo "$reason" | grep "$3")
    if [ $? -ne 0 ]; then
      echo -e "Error: the reason should have contained \"$3\", but not. "
      exit 2
    fi
  fi

  echo "Compatibility result expected."
}

cat <<EOF > /tmp/node_policy.json
{
  "properties": [
    {
      "name": "purpose",
      "value": "network-testing"
    },
    {
      "name": "group",
      "value": "bluenode"
    }
  ],
  "constraints": [
    "iame2edev == true",
    "NONS==false || NOGPS == false || NOLOC == false || NOPWS == false || NOHELLO == false"
  ]
}
EOF

# has specific service version requirement
cat <<EOF > /tmp/node_policy2.json
{
  "properties": [
    {
      "name": "purpose",
      "value": "network-testing"
    },
    {
      "name": "group",
      "value": "bluenode"
    }
  ],
  "constraints": [
    "iame2edev == true",
    "NONS==false || NOGPS == false || NOLOC == false || NOPWS == false || NOHELLO == false",
    "openhorizon.service.version != 2.0.6"
  ]
}
EOF

cat <<EOF > /tmp/business_policy.json
{
    "label": "business policy for gpstest",
    "description": "for gpstest",
    "service": {
      "name": "https://bluehorizon.network/services/gpstest",
      "org": "e2edev@somecomp.com",
      "arch": "amd64",
      "serviceVersions": [
        {
          "version": "1.0.0",
          "priority": {},
          "upgradePolicy": {}
        }
      ],
      "nodeHealth": {
        "missing_heartbeat_interval": 1800,
        "check_agreement_status": 1800
      }
    },
    "properties": [
      {
        "name": "iame2edev",
        "value": "true"
      },
      {
        "name": "NOGPS",
        "value": false
      },
      {
        "name": "number",
        "value": 24
      },
      {
        "name": "gpsvar",
        "value": "gpsval"
      }
    ],
    "constraints": [
      "purpose == network-testing"
    ]
}
EOF

# different arch
cat <<EOF > /tmp/business_policy2.json
{
    "label": "business policy for gpstest",
    "description": "for gpstest",
    "service": {
      "name": "https://bluehorizon.network/services/gpstest",
      "org": "e2edev@somecomp.com",
      "arch": "arm64",
      "serviceVersions": [
        {
          "version": "1.0.0",
          "priority": {},
          "upgradePolicy": {}
        }
      ],
      "nodeHealth": {
        "missing_heartbeat_interval": 1800,
        "check_agreement_status": 1800
      }
    },
    "properties": [
      {
        "name": "iame2edev",
        "value": "true"
      },
      {
        "name": "NOGPS",
        "value": false
      },
      {
        "name": "number",
        "value": 24
      },
      {
        "name": "gpsvar",
        "value": "gpsval"
      }
    ],
    "constraints": [
      "purpose == network-testing"
    ]
}
EOF

cat <<EOF > /tmp/service_policy.json
{
  "properties": [
    {
      "name": "iame2edev_service",
      "value": "true"
    },
    {
      "name": "service_var2",
      "value": "this is gpstest service"
    }
  ],
  "constraints": [
    "group == bluenode"
  ]
}
EOF

# diffrent constraint
cat <<EOF > /tmp/service_policy2.json
{
  "properties": [
    {
      "name": "iame2edev_service",
      "value": "true"
    },
    {
      "name": "service_var2",
      "value": "this is gpstest service"
    }
  ],
  "constraints": [
    "group == greennode"
  ]
}
EOF

echo -e "\n${PREFIX} test without user cred."
CMD="hzn deploycheck policy -n userdev/an12345 -b userdev/bp_gpstest"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "Please specify the exchange credential with -u"

echo -e "\n${PREFIX} test with unauthorized user."
CMD="hzn deploycheck policy -u myorg/me:passwd -n userdev/an12345 -b userdev/bp_gpstest"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "invalid-credentials" "401"

echo -e "\n${PREFIX} test conflict."
CMD="hzn deploycheck policy -u $USERDEV_ADMIN_AUTH -n userdev/an12345 --node-pol /tmp/nodepol.json -b userdev/bp_gpstest"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "\-n and \-\-node-pol are mutually exclusive"

echo -e "\n${PREFIX} test conflict2."
CMD="hzn deploycheck policy -u $USERDEV_ADMIN_AUTH -n userdev/an12345 -b userdev/bp_gpstest -B /tmp/businesspol.json"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "\-b and \-B are mutually exclusive"

echo -e "\n${PREFIX} test input with node id and business policy id."
CMD="hzn deploycheck policy -u $USERDEV_ADMIN_AUTH -n userdev/an12345 -b userdev/bp_gpstest"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "true" ""

echo -e "\n${PREFIX} test input: wrong node id"
CMD="hzn deploycheck policy -u $USERDEV_ADMIN_AUTH -n userdev/an12345xxx -b userdev/bp_gpstest"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "Error getting node"

echo -e "\n${PREFIX} test input: wrong business policy id"
CMD="hzn deploycheck policy -u $USERDEV_ADMIN_AUTH -n userdev/an12345 -b userdev/bp_gpstestxxx"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "Business policy not found for"

echo -e "\n${PREFIX} test input: wrong org id"
CMD="hzn deploycheck policy -u $USERDEV_ADMIN_AUTH -n xxxuserdev/an12345 -b userdev/bp_gpstest"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "Error getting node xxxuserdev/an12345 from the exchange" "403"

echo -e "\n${PREFIX} test input: node org and business org missing, they pick up org from the user cred."
CMD="hzn deploycheck policy -u $USERDEV_ADMIN_AUTH -n an12345 -b bp_gpstest"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "true" ""

echo -e "\n${PREFIX} test input: business policy id only. Use current node policy"
CMD="hzn deploycheck policy -u $USERDEV_ADMIN_AUTH -b bp_netspeed"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "Neither node id nor node policy is specified. Getting node policy from the local node" "\"compatible\": true"

echo -e "\n${PREFIX} test input: node policy and business policy"
CMD="hzn deploycheck policy -u $USERDEV_ADMIN_AUTH --node-pol /tmp/node_policy.json --business-pol /tmp/business_policy.json"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "true" ""

echo -e "\n${PREFIX} test input: node policy, business policy and service policy. No user cred needed"
CMD="hzn deploycheck policy --node-pol /tmp/node_policy.json --business-pol /tmp/business_policy.json --service-pol /tmp/service_policy.json"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "true" ""

echo -e "\n${PREFIX} test input: node policy, business policy and service policy. wrong arch"
CMD="hzn deploycheck policy -a arm64 --node-pol /tmp/node_policy.json --business-pol /tmp/business_policy.json --service-pol /tmp/service_policy.json"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "false" "Service with 'arch' arm64 cannot be found in the business policy"

echo -e "\n${PREFIX} test input: node id, business policy. wrong arch"
CMD="hzn deploycheck policy -u $USERDEV_ADMIN_AUTH -n an12345 --business-pol /tmp/business_policy2.json"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "false" "Service with 'arch' amd64 cannot be found in the business policy"

echo -e "\n${PREFIX} test input: node policy, business policy and service policy. not compatible"
CMD="hzn deploycheck policy -u $USERDEV_ADMIN_AUTH --node-pol /tmp/node_policy.json --business-pol /tmp/business_policy.json --service-pol /tmp/service_policy2.json"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "false" "Compatibility Error"

echo -e "\n${PREFIX} test input: mixed. node id, business policy. not compatible"
CMD="hzn deploycheck policy -u $USERDEV_ADMIN_AUTH  -n an12345 --business-pol /tmp/business_policy.json --service-pol /tmp/service_policy2.json"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "false" "Compatibility Error"

# bp_location has 2 services. one is compatible, the other one is not with /tmp/node_policy2.json
echo -e "\n${PREFIX} test input: mixed. node pol, business policy id. compatible, multible output."
CMD="hzn deploycheck policy -u $USERDEV_ADMIN_AUTH  -b userdev/bp_location --node-pol /tmp/node_policy2.json -c"
echo "$CMD"
RES=$($CMD 2>&1)
c=$(echo $RES | jq '.compatible')
if [ "$c" != "true" ]; then 
  echo "It should return compatible but not."
  exit 2
fi
l=$(echo $RES | jq '.reason | length')
if [ "$l" != "2" ]; then 
  echo "It should return 2 service result but got $l."
  exit 2
fi
echo $RES | jq '.reason."e2edev@somecomp.com/bluehorizon.network-services-location_2.0.6_amd64"' | grep -q Incompatible 
if [ $? -ne 0 ]; then
  echo "Service bluehorizon.network-services-location_2.0.6_amd64 should be incompatible but not."
  exit 2
fi
echo $RES | jq '.reason."e2edev@somecomp.com/bluehorizon.network-services-location_2.0.7_amd64"' | grep -q Incompatible 
if [ $? -eq 0 ]; then
  echo "Service bluehorizon.network-services-location_2.0.7_amd64 should be compatible but not."
  exit 2
fi
echo "Compatibility result expected."

echo -e "\n${PREFIX} complete test\n"

