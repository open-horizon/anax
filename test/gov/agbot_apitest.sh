#!/bin/bash

E2EDEV_ADMIN_AUTH="e2edev@somecomp.com/e2edevadmin:e2edevadminpw"
USERDEV_ADMIN_AUTH="userdev/userdevadmin:userdevadminpw"

PREFIX="Agbot API Test:"


echo ""
echo -e "${PREFIX} Start testing compatibility"

COMP_RESULT=""

bp_location=$(</root/input_files/compcheck/business_pol_location.json)
node_policy=$(</root/input_files/compcheck/node_policy.json)
service_policy=`cat /root/input_files/compcheck/service_policy.json`
node_ui=`cat /root/input_files/compcheck/node_ui.json`
pattern_sloc=`cat /root/input_files/compcheck/pattern_sloc.json`
service_location=`cat /root/input_files/compcheck/service_location.json`
service_locgps=`cat /root/input_files/compcheck/service_locgps.json`

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
    echo "Expected compatible be $1 but got $comp."
    exit 2
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
  local api="$1"
  local comp_input="$2"
  echo "$comp_input" | jq -r '.'
  CMD="curl -LX GET -w %{http_code} --cacert ${CERT_FILE} -u ${USERDEV_ADMIN_AUTH} --data @- ${AGBOT_SAPI_URL}/${api}"
  echo "$CMD"
  RES=$(echo "$comp_input" | curl -sLX GET -w %{http_code} --cacert ${CERT_FILE} -u ${USERDEV_ADMIN_AUTH} --data @- ${AGBOT_SAPI_URL}/${api})
  results "$RES" "$3" "$4"
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

for api in "policycompatible" "userinputcompatible" "deploycompatible"
do
  echo ${api}

  echo -e "\n${PREFIX} test /${api} with unauthorized user."
  CMD="curl -sLX GET -w %{http_code} --cacert ${CERT_FILE} -u myorg/me:passwd ${AGBOT_SAPI_URL}/${api}"
  echo "$CMD"
  RES=$($CMD)
  results "$RES" "401" "Failed to authenticate"

  echo -e "\n${PREFIX} test /${api} without cert"
  CMD="curl -LX GET -w %{http_code} -u myorg/me:passwd ${AGBOT_SAPI_URL}/${api}"
  echo "$CMD"
  RES=$($CMD 2>&1)
  echo "$RES" | grep "SSL certificate problem"
  if [ $? -ne 0 ]; then
    echo -e "${PREFIX} the output should contain 'CRLfile: none', but not\n"
    exit 2
  else
    echo -e "Result expected\n"
  fi

  echo -e "\n${PREFIX} test /${api} without input."
  CMD="curl -sLX GET -w %{http_code} --cacert ${CERT_FILE} -u ${E2EDEV_ADMIN_AUTH} ${AGBOT_SAPI_URL}/${api}"
  echo "$CMD"
  RES=$($CMD)
  results "$RES" "400" "No input found"

  echo -e "\n${PREFIX} test /${api}. Input: node id and business policy id."
  read -d '' comp_input<<EOF
  {
    "node_id": "userdev/an12345",
    "business_policy_id": "userdev/bp_gpstest"
  }
EOF
  run_and_check "$api" "$comp_input" "200" ""
  check_comp_results "true" ""


  echo -e "\n${PREFIX} test /${api}. Input: wrong node id"
  read -d '' comp_input <<EOF
  {
    "node_id": "userdev/an12345xxx",
    "business_policy_id": "userdev/bp_gpstest"
  }
EOF
  run_and_check "$api" "$comp_input" "500" "Error getting node"


  echo -e "\n${PREFIX} test /${api}. Input: wrong business policy id"
  read -d '' comp_input <<EOF
  {
    "node_id": "userdev/an12345",
    "business_policy_id": "userdev/bp_gpstestxxx"
  }
EOF
  run_and_check "$api" "$comp_input" "400" "No business policy found"

  echo -e "\n${PREFIX} test /${api}. Input: wrong org id"
  read -d '' comp_input <<EOF
  {
    "node_id": "userdevxxx/an12345",
    "business_policy_id": "userdev/bp_gpstest"
  }
EOF
  run_and_check "$api" "$comp_input" "500" "device userdevxxx/an12345 not in GET response map"

  echo -e "\n${PREFIX} test /${api}. Input: no node org specifiled"
  read -d '' comp_input <<EOF
  {
    "node_id": "an12345",
    "business_policy_id": "userdev/bp_gpstest"
  }
EOF
  run_and_check "$api" "$comp_input" "400" "Organization is not specified"

  echo -e "\n${PREFIX} test /${api}. Input: no business policy org specifiled"
  read -d '' comp_input <<EOF
  {
   "node_id": "userdev/an12345",
    "business_policy_id": "bp_gpstest"
  }
EOF
  run_and_check "$api" "$comp_input" "400" "Organization is not specified "
done

echo -e "\n${PREFIX} test /policycompatible. Input: node policy and business policy"
read -d '' comp_input <<EOF
{
  "node_policy":      $node_policy,
  "business_policy":  $bp_location
}
EOF
run_and_check "policycompatible" "$comp_input" "200" ""
check_comp_results "true" ""

echo -e "\n${PREFIX} test /policycompatible. Input: node policy, business policy and service policy"
read -d '' comp_input <<EOF
{
  "node_policy":      $node_policy,
  "business_policy":  $bp_location,
  "service_policy":   $service_policy
}
EOF
run_and_check "policycompatible" "$comp_input" "200" ""

echo -e "\n${PREFIX} test /policycompatible. Input: node policy, business policy and service policy. not compatible"
read -d '' service_policy_bad <<EOF
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
  "business_policy":  $bp_location,
  "service_policy":   $service_policy_bad
}
EOF
run_and_check "policycompatible" "$comp_input" "200" ""
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
run_and_check "policycompatible" "$comp_input" "200" ""
check_comp_results "false" "Compatibility Error"

echo -e "\n${PREFIX} test /userinputcompatible. Input: node userinput, business policy. compatible"
read -d '' comp_input <<EOF
{
  "node_user_input":  $node_ui,
  "business_policy":  $bp_location
}
EOF
run_and_check "userinputcompatible" "$comp_input" "200" ""
check_comp_results "true" "Compatible"

echo -e "\n${PREFIX} test /userinputcompatible. Input: node userinput, business policy. not compatible"
read -d '' node_ui_bad <<EOF
[
  {
    "serviceOrgid": "e2edev@somecomp.com",
    "serviceUrl": "https://bluehorizon.network/services/locgps",
    "serviceArch": "amd64",
    "serviceVersionRange": "2.0.3",
    "inputs": [
      {
        "name": "xxxHZN_LAT",
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
  }
]
EOF
read -d '' comp_input <<EOF
{
  "node_user_input":  $node_ui_bad,
  "business_policy":  $bp_location
}
EOF
run_and_check "userinputcompatible" "$comp_input" "200" ""
check_comp_results "false" "User Input Incompatible"
check_comp_results "false" "A required user input value is missing for variable HZN_LAT"

echo -e "\n${PREFIX} test /deploycompatible. Input: node policy, node userinput, business policy. compatible"
read -d '' comp_input <<EOF
{
  "node_policy":      $node_policy,
  "node_user_input":  $node_ui,
  "business_policy":  $bp_location
}
EOF
run_and_check "deploycompatible" "$comp_input" "200" ""
check_comp_results "true" "Compatible"

echo -e "\n${PREFIX} test /deploycompatible. Input: node policy, node userinput, business policy. not compatible"
read -d '' comp_input <<EOF
{
  "node_policy":      $node_policy,
  "node_user_input":  $node_ui_bad,
  "business_policy":  $bp_location
}
EOF
run_and_check "deploycompatible" "$comp_input" "200" ""
check_comp_results "false" "User Input Incompatible"

echo -e "\n${PREFIX} test /deploycompatible. Input: node policy, node userinput, business policy. Result: version 2.0.6 policy not compatible, version 2.0.7 user input not compatible."
read -d '' node_pol1 <<EOF
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
    "NONS == false || NOLOC == false || NOPWS == false || NOHELLO == false || NOGPS == false",
    "openhorizon.service.version != 2.0.6"
  ]
}
EOF
read -d '' comp_input <<EOF
{
  "node_policy":      $node_pol1,
  "node_user_input":  $node_ui_bad,
  "business_policy":  $bp_location
}
EOF
run_and_check "deploycompatible" "$comp_input" "200" ""
check_comp_results "false" "Policy Incompatible"
check_comp_results "false" "User Input Incompatible"

echo -e "\n${PREFIX} test /deploycompatible. Input: patten id, node user input. Result: compatible."
read -d '' comp_input <<EOF
{
  "node_user_input":  $node_ui,
  "pattern_id":       "e2edev@somecomp.com/sloc"
}
EOF
run_and_check "deploycompatible" "$comp_input" "200" ""
check_comp_results "true" "Compatible"

echo -e "\n${PREFIX} test /deploycompatible. Input: patten id, node user input. Result: not compatible."
read -d '' comp_input <<EOF
{
  "node_user_input":  $node_ui_bad,
  "pattern_id":       "e2edev@somecomp.com/sloc"
}
EOF
run_and_check "deploycompatible" "$comp_input" "200" ""
check_comp_results "false" "User Input Incompatible"

echo -e "\n${PREFIX} test /deploycompatible. Input: node id, pattern id. Result: compatible."
read -d '' comp_input <<EOF
{
  "node_id":          "userdev/an12345",
  "pattern_id":       "e2edev@somecomp.com/sall"
}
EOF
run_and_check "deploycompatible" "$comp_input" "200" ""
check_comp_results "true" "Compatible"

echo -e "\n${PREFIX} test /deploycompatible. Input: patten, node user input. Result: compatible."
read -d '' comp_input <<EOF
{
  "node_user_input":  $node_ui,
  "pattern":          $pattern_sloc
}
EOF
run_and_check "deploycompatible" "$comp_input" "200" ""
check_comp_results "true" "Compatible"

echo -e "\n${PREFIX} test /deploycompatible. Input: patten, node user input, service. Result: compatible."
read -d '' comp_input <<EOF
{
  "node_user_input":  $node_ui,
  "pattern":          $pattern_sloc,
  "service":          [$service_location, $service_locgps]
}
EOF
run_and_check "deploycompatible" "$comp_input" "200" ""
check_comp_results "true" "Compatible"


echo -e "\n${PREFIX} test /deploycompatible. Input: patten, node user input, service, node arch. Result: not compatible."
read -d '' comp_input <<EOF
{
  "node_user_input":  $node_ui,
  "pattern":          $pattern_sloc,
  "service":          [$service_location],
  "node_arch":        "amd64"
 }
EOF
run_and_check "deploycompatible" "$comp_input" "200" ""
check_comp_results "false" "User Input Incompatible"


echo -e "\n${PREFIX} complete test\n"
