#!/bin/bash

E2EDEV_ADMIN_AUTH="e2edev@somecomp.com/e2edevadmin:e2edevadminpw"
USERDEV_ADMIN_AUTH="userdev/userdevadmin:userdevadminpw"

PREFIX="Agbot API Test:"

echo ""
echo -e "${PREFIX} Start testing compatibility"

if [ -z ${AGBOT_SAPI_URL} ]; then
  echo -e "\n${PREFIX} Envvar AGBOT_SAPI_URL is empty. Skip test\n"
  exit 0
fi

# -------------------- deployment-check api tests ------------------------- #
COMP_RESULT=""

bp_location=$(</root/input_files/compcheck/business_pol_location.json)
node_policy=$(</root/input_files/compcheck/node_policy.json)
service_policy=`cat /root/input_files/compcheck/service_policy.json`
node_ui=`cat /root/input_files/compcheck/node_ui.json`
pattern_sloc=`cat /root/input_files/compcheck/pattern_sloc.json`
service_location=`cat /root/input_files/compcheck/service_location.json`
service_locgps=`cat /root/input_files/compcheck/service_locgps.json`

if [ "$NOVAULT" != "1" ]; then
  service_location=`cat /root/input_files/compcheck/service_location_secrets.json`
  service_location_secret_extra=`cat /root/input_files/compcheck/service_location_secrets_extra.json`
  bp_location=$(</root/input_files/compcheck/business_pol_location_secrets.json)
  pattern_sloc=`cat /root/input_files/compcheck/pattern_sloc_secrets.json`
fi

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

  # check if error text contains all of the test text snippets
  for (( i=3; i<=$#; i++))
  {
    eval TEST_ARG='$'$i
    if [ ! -z "$TEST_ARG" ]; then
      res=$(echo "$1" | grep "$TEST_ARG")
      if [ $? -ne 0 ]; then
        echo -e "Error: the response should have contained \"$TEST_ARG\", but did not. \n"
        exit 2
      fi
    fi
  }

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
      echo -e "Error: the reason should have contained \"$2\", but not. \n"
      exit 2
    fi
  fi

  echo "Compatibility result expected."
}

function run_and_check {
  local api="$1"
  local comp_input="$2"
  echo "$comp_input" | jq -r '.'
  CMD="curl -LX GET -w %{http_code} ${CERT_VAR} -u ${USERDEV_ADMIN_AUTH} --data @- ${AGBOT_SAPI_URL}/${api}"
  echo "$CMD"
  RES=$(echo "$comp_input" | curl -sLX GET -w %{http_code} ${CERT_VAR} -u ${USERDEV_ADMIN_AUTH} --data @- ${AGBOT_SAPI_URL}/${api})
  results "$RES" "$3" "$4"
}

# get the cert file
if [ ${CERT_LOC} -eq "1" ]; then
  CERT_VAR="--cacert /certs/agbotapi.crt"
else
  CERT_VAR=""
fi

echo -e "${PREFIX} the agbot secure api url is $AGBOT_SAPI_URL"

for api in "deploycheck/policycompatible" "deploycheck/userinputcompatible" "deploycheck/deploycompatible"
do
  echo ${api}

  echo -e "\n${PREFIX} test /${api} with unauthorized user."
  CMD="curl -sLX GET -w %{http_code} ${CERT_VAR} -u myorg/me:passwd ${AGBOT_SAPI_URL}/${api}"
  echo "$CMD"
  RES=$($CMD)
  results "$RES" "401" "Failed to authenticate"


  if [ -z "${AGBOT_SAPI_URL##https://*}" ]; then
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
  else
    echo -e "\n${PREFIX} Skip test without cert because of http protocol in envvar AGBOT_SAPI_URL: ${AGBOT_SAPI_URL}"
  fi	  

  echo -e "\n${PREFIX} test /${api} without input."
  CMD="curl -sLX GET -w %{http_code} ${CERT_VAR} -u ${E2EDEV_ADMIN_AUTH} ${AGBOT_SAPI_URL}/${api}"
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
  run_and_check "$api" "$comp_input" "400" "No deployment policy found"

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

echo -e "\n${PREFIX} test /deploycheck/policycompatible. Input: node policy and business policy"
read -d '' comp_input <<EOF
{
  "node_policy":      $node_policy,
  "business_policy":  $bp_location
}
EOF
run_and_check "deploycheck/policycompatible" "$comp_input" "200" ""
check_comp_results "true" ""

echo -e "\n${PREFIX} test /deploycheck/policycompatible. Input: node policy, business policy and service policy"
read -d '' comp_input <<EOF
{
  "node_policy":      $node_policy,
  "business_policy":  $bp_location,
  "service_policy":   $service_policy
}
EOF
run_and_check "deploycheck/policycompatible" "$comp_input" "200" ""

echo -e "\n${PREFIX} test /deploycheck/policycompatible. Input: node policy, business policy and service policy. not compatible"
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
run_and_check "deploycheck/policycompatible" "$comp_input" "200" ""
check_comp_results "false" "Compatibility Error"

echo -e "\n${PREFIX} test /deploycheck/policycompatible. Input: Mixed. node id, business policy. not compatible"
read -d '' comp_input <<EOF
{
  "node_id": "userdev/an12345",
  "business_policy": {
    "label": "business policy for gpstest",
    "description": "for gpstest",
    "service": {
      "name": "https://bluehorizon.network/services/gpstest",
      "org": "e2edev@somecomp.com",
      "arch": "${ARCH}",
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
run_and_check "deploycheck/policycompatible" "$comp_input" "200" ""
check_comp_results "false" "Compatibility Error"

echo -e "\n${PREFIX} test /deploycheck/userinputcompatible. Input: node userinput, business policy. compatible"
read -d '' comp_input <<EOF
{
  "node_user_input":  $node_ui,
  "business_policy":  $bp_location
}
EOF
run_and_check "deploycheck/userinputcompatible" "$comp_input" "200" ""
check_comp_results "true" "Compatible"

echo -e "\n${PREFIX} test /deploycheck/userinputcompatible. Input: node userinput, business policy. not compatible"
read -d '' node_ui_bad <<EOF
[
  {
    "serviceOrgid": "e2edev@somecomp.com",
    "serviceUrl": "https://bluehorizon.network/services/locgps",
    "serviceArch": "${ARCH}",
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
run_and_check "deploycheck/userinputcompatible" "$comp_input" "200" ""
check_comp_results "false" "User Input Incompatible"
check_comp_results "false" "A required user input value is missing for variable HZN_LAT"

echo -e "\n${PREFIX} test /deploycheck/deploycompatible. Input: node policy, node userinput, business policy. compatible"
read -d '' comp_input <<EOF
{
  "node_policy":      $node_policy,
  "node_user_input":  $node_ui,
  "business_policy":  $bp_location
}
EOF
run_and_check "deploycheck/deploycompatible" "$comp_input" "200" ""
check_comp_results "true" "Compatible"

echo -e "\n${PREFIX} test /deploycheck/deploycompatible. Input: node policy, node userinput, business policy. not compatible"
read -d '' comp_input <<EOF
{
  "node_policy":      $node_policy,
  "node_user_input":  $node_ui_bad,
  "business_policy":  $bp_location
}
EOF
run_and_check "deploycheck/deploycompatible" "$comp_input" "200" ""
check_comp_results "false" "User Input Incompatible"

echo -e "\n${PREFIX} test /deploycheck/deploycompatible. Input: node policy, node userinput, business policy. Result: version 2.0.6 policy not compatible, version 2.0.7 user input not compatible."
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
run_and_check "deploycheck/deploycompatible" "$comp_input" "200" ""
check_comp_results "false" "Policy Incompatible"
check_comp_results "false" "User Input Incompatible"

echo -e "\n${PREFIX} test /deploycompatible. Input: patten id, node user input. Result: compatible."
read -d '' comp_input <<EOF
{
  "node_user_input":  $node_ui,
  "pattern_id":       "e2edev@somecomp.com/sloc"
}
EOF
run_and_check "deploycheck/deploycompatible" "$comp_input" "200" ""
check_comp_results "true" "Compatible"

echo -e "\n${PREFIX} test /deploycheck/deploycompatible. Input: patten id, node user input. Result: not compatible."
read -d '' comp_input <<EOF
{
  "node_user_input":  $node_ui_bad,
  "pattern_id":       "e2edev@somecomp.com/sloc"
}
EOF
run_and_check "deploycheck/deploycompatible" "$comp_input" "200" ""
check_comp_results "false" "User Input Incompatible"

echo -e "\n${PREFIX} test /deploycheck/deploycompatible. Input: node id, pattern id. Result: compatible."
read -d '' comp_input <<EOF
{
  "node_id":          "userdev/an12345",
  "pattern_id":       "e2edev@somecomp.com/sall"
}
EOF
run_and_check "deploycheck/deploycompatible" "$comp_input" "200" ""
check_comp_results "true" "Compatible"

echo -e "\n${PREFIX} test /deploycheck/deploycompatible. Input: patten, node user input. Result: compatible."
read -d '' comp_input <<EOF
{
  "node_user_input":  $node_ui,
  "pattern":          $pattern_sloc
}
EOF
run_and_check "deploycheck/deploycompatible" "$comp_input" "200" ""
check_comp_results "true" "Compatible"

echo -e "\n${PREFIX} test /deploycheck/deploycompatible. Input: patten, node user input, service. Result: compatible."
read -d '' comp_input <<EOF
{
  "node_user_input":  $node_ui,
  "pattern":          $pattern_sloc,
  "service":          [$service_location, $service_locgps]
}
EOF
run_and_check "deploycheck/deploycompatible" "$comp_input" "200" ""
check_comp_results "true" "Compatible"


echo -e "\n${PREFIX} test /deploycheck/deploycompatible. Input: patten, node user input, service, node arch. Result: not compatible."
read -d '' comp_input <<EOF
{
  "node_user_input":  $node_ui,
  "pattern":          $pattern_sloc,
  "service":          [$service_location],
  "node_arch":        "${ARCH}"
 }
EOF
run_and_check "deploycheck/deploycompatible" "$comp_input" "200" ""
check_comp_results "false" "User Input Incompatible"

if [ "$NOVAULT" == "1" ]; then
  echo -e "\n${PREFIX} Skipping agbot API tests for secret binding and vault\n"
  exit 0
fi

# test secret binding in deploymen check
echo -e "\n${PREFIX} test /deploycheck/secretbindingcompatible. Input: patten with secret binding, service with secret. Result: compatible."
read -d '' comp_input <<EOF
{
  "pattern":          $pattern_sloc,
  "service":          [$service_location, $service_locgps]
}
EOF
run_and_check "deploycheck/secretbindingcompatible" "$comp_input" "200" ""
check_comp_results "true" "Compatible"

echo -e "\n${PREFIX} test /deploycheck/secretbindingcompatible. business policy with secret, service with secret. compatible"
read -d '' comp_input <<EOF
{
  "business_policy":  $bp_location,
  "service":          [$service_location]
}
EOF
run_and_check "deploycheck/secretbindingcompatible" "$comp_input" "200" ""
check_comp_results "true" "Compatible"


echo -e "\n${PREFIX} test /deploycheck/deploycompatible. Input: node policy, node userinput, business policy with secret, service with secret. Incompatible"
read -d '' comp_input <<EOF
{
  "node_policy":      $node_policy,
  "node_user_input":  $node_ui,
  "business_policy":  $bp_location,
  "service":          [$service_location_secret_extra]
}
EOF
run_and_check "deploycheck/deploycompatible" "$comp_input" "200" ""
check_comp_results "false" "Secret Binding Incompatible" "No secret binding found for"

# -------------------- secret api tests ------------------------- #
echo ""
echo -e "${PREFIX} Start testing for vault secrets API"

# Later export these from /root/init_vault
TEST_VAULT_SECRET_ORG="userdev"
TEST_VAULT_SECRET_NAME="secret"
TEST_VAULT_SECRET_VALUE="${TEST_VAULT_SECRET_NAME}"

read -d '' create_secret <<EOF
{
  \"key\":\"test\",
  \"value\":\"value\"
}
EOF

LIST_ORG_SECRET="org/${TEST_VAULT_SECRET_ORG}/secrets/${TEST_VAULT_SECRET_NAME}"
LIST_ORG_SECRETS="org/${TEST_VAULT_SECRET_ORG}/secrets"
CREATE_ORG_SECRETS="org/${TEST_VAULT_SECRET_ORG}/secrets/secret1"
DELETE_ORG_SECRETS="org/${TEST_VAULT_SECRET_ORG}/secrets/secret1"

echo -e "\n${PREFIX} test ${LIST_ORG_SECRET} LIST"
CMD="curl -sLX LIST -w %{http_code} ${CERT_VAR} -u ${USERDEV_ADMIN_AUTH} ${AGBOT_SAPI_URL}/${LIST_ORG_SECRET}"
echo "$CMD"
RES=$($CMD)
results "$RES" "200" "exists" "false"

echo -e "\n${PREFIX} test ${LIST_ORG_SECRETS} LIST"
CMD="curl -sLX LIST -w %{http_code} ${CERT_VAR} -u ${USERDEV_ADMIN_AUTH} ${AGBOT_SAPI_URL}/${LIST_ORG_SECRETS}"
echo "$CMD"
RES=$($CMD)
results "$RES" "200" "${TEST_VAULT_SECRET_NAME}"

echo -e "\n${PREFIX} test ${CREATE_ORG_SECRETS} POST"
CMD="curl -sLX POST -w %{http_code} ${CERT_VAR} -u ${USERDEV_ADMIN_AUTH} -d ${create_secret} ${AGBOT_SAPI_URL}/${CREATE_ORG_SECRETS}"
echo "$CMD"
RES=$(curl -sLX POST -w %{http_code} ${CERT_VAR} -u ${USERDEV_ADMIN_AUTH} -d "${create_secret}" ${AGBOT_SAPI_URL}/${CREATE_ORG_SECRETS})
results "$RES" "201" ""

echo -e "\n${PREFIX} test ${LIST_ORG_SECRET} LIST"
CMD="curl -sLX LIST -w %{http_code} ${CERT_VAR} -u ${USERDEV_ADMIN_AUTH} ${AGBOT_SAPI_URL}/${LIST_ORG_SECRET}1"
echo "$CMD"
RES=$($CMD)
results "$RES" "200" "exists" "true"

echo -e "\n${PREFIX} test ${LIST_ORG_SECRET} LIST"
CMD="curl -sLX LIST -w %{http_code} ${CERT_VAR} -u ${USERDEV_ADMIN_AUTH} ${AGBOT_SAPI_URL}/${LIST_ORG_SECRET}_wrong"
echo "$CMD"
RES=$($CMD)
results "$RES" "200" "false"

echo -e "\n${PREFIX} test ${LIST_ORG_SECRET} GET with invalid credentials"
CMD="curl -sLX GET -w %{http_code} ${CERT_VAR} -u ${USERDEV_ADMIN_AUTH}_wrong ${AGBOT_SAPI_URL}/${LIST_ORG_SECRET}"
echo "$CMD"
RES=$($CMD)
results "$RES" "401" "Failed to authenticate"

echo -e "\n${PREFIX} test ${LIST_ORG_SECRET} LIST with invalid secret and secret org"
CMD="curl -sLX LIST -w %{http_code} ${CERT_VAR} -u ${USERDEV_ADMIN_AUTH} ${AGBOT_SAPI_URL}/${LIST_ORG_SECRET}_wrong"
echo "$CMD"
RES=$($CMD)
results "$RES" "200" "exists" "false"

echo -e "\n${PREFIX} test ${LIST_ORG_SECRET} LIST with invalid org"
CMD="curl -sLX LIST -w %{http_code} ${CERT_VAR} -u ${USERDEV_ADMIN_AUTH} ${AGBOT_SAPI_URL}/org/${TEST_VAULT_SECRET_ORG}_wrong/secrets"
echo "$CMD"
RES=$($CMD)
results "$RES" "403" ""

echo -e "\n${PREFIX} test ${DELETE_ORG_SECRETS} DELETE with valid secret and secret org"
CMD="curl -sLX DELETE -w %{http_code} ${CERT_VAR} -u ${USERDEV_ADMIN_AUTH} ${AGBOT_SAPI_URL}/${DELETE_ORG_SECRETS}"
echo "$CMD"
RES=$($CMD)
results "$RES" "204" ""

echo -e "\n${PREFIX} test ${LIST_ORG_SECRET} LIST with deleted secret"
CMD="curl -sLX LIST -w %{http_code} ${CERT_VAR} -u ${USERDEV_ADMIN_AUTH} ${AGBOT_SAPI_URL}/${LIST_ORG_SECRET}1"
echo "$CMD"
RES=$($CMD)
results "$RES" "200" "exists" "false"

# skip if not local e2edev test
if [ "${EXCH_APP_HOST}" == "http://exchange-api:8081/v1" ]; then
  # Check agbot <-> vault health status using AGBOT_API
  echo -e "\n${PREFIX} Check agbot-vault health status"
  CMD="curl -sLX GET -w %{http_code} ${AGBOT_API}/health"
  echo "$CMD"
  RES=$($CMD)
  results "$RES" "200" "lastVaultInteraction"
fi

echo -e "\n${PREFIX} complete test\n"
