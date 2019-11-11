#!/bin/bash

E2EDEV_ADMIN_AUTH="e2edev@somecomp.com/e2edevadmin:e2edevadminpw"
USERDEV_ADMIN_AUTH="userdev/userdevadmin:userdevadminpw"

PREFIX="Agbot API Test:"


echo ""
echo -e "${PREFIX} start test"

COMP_RESULT=""

# check the the result to see if it matches the expected http code and error
function results {
  rc="${1: -3}"
  output="${1::-3}"

  echo "$1" | jq -r '.'

  if [ "$rc" == "200" ]; then
    COMP_RESULT=$output
  fi

  # check http code
  if [ "$rc" != $2 ]
  then
    echo -e "Error: $(echo "$output" | jq -r '.')\n"
    exit 2
  fi

  # check if error text
  if [ ! -z "$3" ]; then
    res=$(echo "$1" | grep "$3")
    if [ $? -ne 0 ]; then
      echo -e "Error: the response should have contained \"$3\", but not. "
      exit 2
    fi
  fi

  #statements
  echo -e "Result expected."
}

# check the good result to see if the compatible and reason are correct.
function check_comp_results {
  if [ -z "$COMP_RESULT" ]; then
    echo "No result to compare."
    exit 2
  fi

  comp=$(echo $COMP_RESULT | jq -r ".compatible")
  reason=$(echo $COMP_RESULT | jq -r ".reason")

  if [ "$comp" != "$1" ]; then
    echo "Expexted compatible be $1 but got $comp."
    return 2
  fi

  if [ ! -z "$2" ]; then
    res=$(echo "$reason" | grep "$2")
    if [ $? -ne 0 ]; then
      echo -e "Error: the reason should have contained \"$2\", but not. "
      exit 2
    fi
  fi

  echo "Compatibility result expected."
}

function run_and_check {
  comp_input="$1"
  echo "$comp_input" | jq -r '.'
  CMD="curl -sLX GET -w %{http_code} --cacert ${CERT_FILE} -u ${USERDEV_ADMIN_AUTH} --data @- ${AGBOT_SAPI_URL}/policycompatible"
  echo "$CMD"
  RES=$(echo "$comp_input" | curl -sLX GET -w %{http_code} --cacert ${CERT_FILE} -u ${USERDEV_ADMIN_AUTH} --data @- ${AGBOT_SAPI_URL}/policycompatible)
  results "$RES" "$2" "$3"
}

# get the cert file 
if [ "${EXCH_APP_HOST}" = "http://exchange-api:8080/v1" ]; then
  CERT_FILE="/home/agbotuser/keys/agbotapi.crt"
else
  # agbot is remote
  CERT_FILE="/certs/agbotapi.crt"
fi
echo -e "${PREFIX} the cert file name is $CERT_FILE"
echo -e "${PREFIX} the agbot secure api url is $AGBOT_SAPI_URL"

echo -e "\n${PREFIX} test /policycompatible with unauthorized user."
CMD="curl -sLX GET -w %{http_code} --cacert ${CERT_FILE} -u myorg/me:passwd ${AGBOT_SAPI_URL}/policycompatible"
echo "$CMD"
RES=$($CMD)
results "$RES" "401" "Failed to authenticate"

echo -e "\n${PREFIX} test /policycompatible without cert"
CMD="curl -LX GET -w %{http_code} -u myorg/me:passwd ${AGBOT_SAPI_URL}/policycompatible"
echo "$CMD"
RES=$($CMD 2>&1)
echo "$RES" | grep "SSL certificate problem"
if [ $? -ne 0 ]; then
  echo -e "${PREFIX} the output should contain 'CRLfile: none', but not\n"
  exit 2
else
  echo -e "Result expected\n"
fi

echo -e "\n${PREFIX} test /policycompatible without input."
CMD="curl -sLX GET -w %{http_code} --cacert ${CERT_FILE} -u ${E2EDEV_ADMIN_AUTH} ${AGBOT_SAPI_URL}/policycompatible"
echo "$CMD"
RES=$($CMD)
results "$RES" "400" "No input found"

echo -e "\n${PREFIX} test /policycompatible. Input: node id and business policy id."
read -d '' comp_input <<EOF
{
  "node_id": "userdev/an12345",
  "business_policy_id": "userdev/bp_gpstest"
}
EOF
run_and_check "$comp_input" "200" ""
check_comp_results "true" ""


echo -e "\n${PREFIX} test /policycompatible. Input: wrong node id"
read -d '' comp_input <<EOF
{
  "node_id": "userdev/an12345xxx",
  "business_policy_id": "userdev/bp_gpstest"
}
EOF
run_and_check "$comp_input" "400" "No node policy found"


echo -e "\n${PREFIX} test /policycompatible. Input: wrong business policy id"
read -d '' comp_input <<EOF
{
  "node_id": "userdev/an12345",
  "business_policy_id": "userdev/bp_gpstestxxx"
}
EOF
run_and_check "$comp_input" "400" "No business policy found"

echo -e "\n${PREFIX} test /policycompatible. Input: wrong org id"
read -d '' comp_input <<EOF
{
  "node_id": "userdevxxx/an12345",
  "business_policy_id": "userdev/bp_gpstest"
}
EOF
run_and_check "$comp_input" "500" "READ_OTHER_ORGS"

echo -e "\n${PREFIX} test /policycompatible. Input: no node org specifiled"
read -d '' comp_input <<EOF
{
  "node_id": "an12345",
  "business_policy_id": "userdev/bp_gpstest"
}
EOF
run_and_check "$comp_input" "400" "Organization is not specified"

echo -e "\n${PREFIX} test /policycompatible. Input: no business policy org specifiled"
read -d '' comp_input <<EOF
{
  "node_id": "userdev/an12345",
  "business_policy_id": "bp_gpstest"
}
EOF
run_and_check "$comp_input" "400" "Organization is not specified "

echo -e "\n${PREFIX} test /policycompatible. Input: node policy and business policy"
read -d '' node_policy <<EOF
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

read -d '' business_policy <<EOF
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

read -d '' comp_input <<EOF
{
  "node_policy":      $node_policy,
  "business_policy":  $business_policy
}
EOF
run_and_check "$comp_input" "200" ""
check_comp_results "true" ""

echo -e "\n${PREFIX} test /policycompatible. Input: node policy, business policy and service policy"
read -d '' service_policy <<EOF
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

read -d '' comp_input <<EOF
{
  "node_policy":      $node_policy,
  "business_policy":  $business_policy,
  "service_policy":   $service_policy
}
EOF
run_and_check "$comp_input" "200" ""

echo -e "\n${PREFIX} test /policycompatible. Input: node policy, business policy and service policy. not compatible"
read -d '' service_policy <<EOF
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

read -d '' comp_input <<EOF
{
  "node_policy":      $node_policy,
  "business_policy":  $business_policy,
  "service_policy":   $service_policy
}
EOF
run_and_check "$comp_input" "200" ""
check_comp_results "false" "Compatibility Error"

echo -e "\n${PREFIX} test /policycompatible. Input: Mixed. node id, business policy. not compatible"
read -d '' comp_input <<EOF
{
  "node_id": "userdev/an12345",
  "business_policy": {
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
        "name": "gpsvar",
        "value": "gpsval"
      }
    ],
    "constraints": [
      "purpose == network-testing1"
    ]
  }
}

EOF
run_and_check "$comp_input" "200" ""
check_comp_results "false" "Compatibility Error"

echo -e "\n${PREFIX} complete test\n"

