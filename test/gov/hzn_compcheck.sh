#!/bin/bash

USERDEV_ADMIN_AUTH="userdev/userdevadmin:userdevadminpw"
export HZN_EXCHANGE_URL="${EXCH_APP_HOST}"

unset HZN_ORG_ID
unset HZN_EXCHANGE_NODE_AUTH
unset HZN_EXCHANGE_USER_AUTH

PREFIX="HZN policy compatibility test:"


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
CMD="hzn policy compatible -n userdev/an12345 -b userdev/bp_gpstest"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "Please specify the exchange credential with -u"

echo -e "\n${PREFIX} test with unauthorized user."
CMD="hzn policy compatible -u myorg/me:passwd -n userdev/an12345 -b userdev/bp_gpstest"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "invalid-credentials" "401"

echo -e "\n${PREFIX} test conflict."
CMD="hzn policy compatible -u $USERDEV_ADMIN_AUTH -n userdev/an12345 --node-pol /tmp/nodepol.json -b userdev/bp_gpstest"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "\-n and \-\-node-pol are mutually exclusive"

echo -e "\n${PREFIX} test conflict2."
CMD="hzn policy compatible -u $USERDEV_ADMIN_AUTH -n userdev/an12345 -b userdev/bp_gpstest --business-pol /tmp/businesspol.json"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "\-b and \-\-business-pol are mutually exclusive"

echo -e "\n${PREFIX} test input with node id and business policy id."
CMD="hzn policy compatible -u $USERDEV_ADMIN_AUTH -n userdev/an12345 -b userdev/bp_gpstest"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "true" ""

echo -e "\n${PREFIX} test input: wrong node id"
CMD="hzn policy compatible -u $USERDEV_ADMIN_AUTH -n userdev/an12345xxx -b userdev/bp_gpstest"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "No node policy found for this node"

echo -e "\n${PREFIX} test input: wrong business policy id"
CMD="hzn policy compatible -u $USERDEV_ADMIN_AUTH -n userdev/an12345 -b userdev/bp_gpstestxxx"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "No business policy found for this id"

echo -e "\n${PREFIX} test input: wrong org id"
CMD="hzn policy compatible -u $USERDEV_ADMIN_AUTH -n xxxuserdev/an12345 -b userdev/bp_gpstest"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "Error trying to query node policy for xxxuserdev/an12345" "403"

echo -e "\n${PREFIX} test input: node org and business org missing, they pick up org from the user cred."
CMD="hzn policy compatible -u $USERDEV_ADMIN_AUTH -n an12345 -b bp_gpstest"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "true" ""

echo -e "\n${PREFIX} test input: business policy id only. Use current node policy"
CMD="hzn policy compatible -u $USERDEV_ADMIN_AUTH -b bp_gpstest"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "Neither node id nor node policy is not specified. Getting node policy from the local node" "\"compatible\": true"

echo -e "\n${PREFIX} test input: node policy and business policy"
CMD="hzn policy compatible -u $USERDEV_ADMIN_AUTH --node-pol /tmp/node_policy.json --business-pol /tmp/business_policy.json"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "true" ""

echo -e "\n${PREFIX} test input: node policy, business policy and service policy. No user cred needed"
CMD="hzn policy compatible --node-pol /tmp/node_policy.json --business-pol /tmp/business_policy.json --service-pol /tmp/service_policy.json"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "true" ""

echo -e "\n${PREFIX} test input: node policy, business policy and service policy. not compatible"

CMD="hzn policy compatible -u $USERDEV_ADMIN_AUTH --node-pol /tmp/node_policy.json --business-pol /tmp/business_policy.json --service-pol /tmp/service_policy2.json"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "false" "Compatibility Error"

echo -e "\n${PREFIX} test input: mixed. node id, business policy. not compatible"
CMD="hzn policy compatible -u $USERDEV_ADMIN_AUTH  -n an12345 --business-pol /tmp/business_policy.json --service-pol /tmp/service_policy2.json"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "false" "Compatibility Error"

echo -e "\n${PREFIX} complete test\n"

