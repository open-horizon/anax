#!/bin/bash

USERDEV_ADMIN_AUTH="userdev/userdevadmin:userdevadminpw"
export HZN_EXCHANGE_URL="${EXCH_APP_HOST}"

unset HZN_ORG_ID
unset HZN_EXCHANGE_NODE_AUTH
unset HZN_EXCHANGE_USER_AUTH

export ARCH=${ARCH}

PREFIX="HZN deployment compatibility test:"


echo ""
echo -e "${PREFIX} start test"

if [ "${NOVAULT}" != "1" ]; then
  service_location="/root/input_files/compcheck/service_location_secrets.json"
  bp_location="/root/input_files/compcheck/business_pol_location_secrets.json"
  pattern_sloc="/root/input_files/compcheck/pattern_sloc_secrets.json"
else
  service_location="/root/input_files/compcheck/service_location.json"
  bp_location="/root/input_files/compcheck/business_pol_location.json"
  pattern_sloc="/root/input_files/compcheck/pattern_sloc.json"
fi 


# check the the result to see if it matches the expected http code and error
function results {

  echo "$1"

  # check error text
  if [ ! -z "$2" ]; then
    res=$(echo "$1" | grep "$2")
    if [ $? -ne 0 ]; then
      echo -e "Error: the response should have contained \"$2\", but did not. "
      exit 2
    fi
  fi

  # check error code
  if [ ! -z "$3" ]; then
    res=$(echo "$1" | grep "$3")
    if [ $? -ne 0 ]; then
      echo -e "Error: the response should have contained \"$3\", but did not. "
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
    echo "Expected compatible be $2 but got $comp."
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

for subcmd in "policy" "userinput" "secretbinding" "all"
do
  echo -e "\n${PREFIX} test without user cred."
  CMD="hzn deploycheck $subcmd -n userdev/an12345 -b userdev/bp_gpstest"
  echo "$CMD"
  RES=$($CMD 2>&1)
  results "$RES" "Please specify the Exchange credential with -u"

  echo -e "\n${PREFIX} test with unauthorized user."
  CMD="hzn deploycheck $subcmd -u myorg/me:passwd -n userdev/an12345 -b userdev/bp_gpstest"
  echo "$CMD"
  RES=$($CMD 2>&1)
  results "$RES" "invalid-credentials" "401"

  echo -e "\n${PREFIX} test input: wrong node id"
  CMD="hzn deploycheck $subcmd -u $USERDEV_ADMIN_AUTH -n userdev/an12345xxx -b userdev/bp_gpstest"
  echo "$CMD"
  RES=$($CMD 2>&1)
  results "$RES" "Error getting node"

  echo -e "\n${PREFIX} test input: wrong org id"
  CMD="hzn deploycheck $subcmd -u $USERDEV_ADMIN_AUTH -n xxxuserdev/an12345 -b userdev/bp_gpstest"
  echo "$CMD"
  RES=$($CMD 2>&1)
  results "$RES" "Error getting node xxxuserdev/an12345 from the Exchange"
done

PREFIX="HZN policy compatibility test:"

echo -e "\n${PREFIX} test conflict."
CMD="hzn deploycheck policy -u $USERDEV_ADMIN_AUTH -n userdev/an12345 --node-pol input_files/node_policy.json -b userdev/bp_gpstest"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "\-n and \-\-node-pol are mutually exclusive"

echo -e "\n${PREFIX} test conflict2."
CMD="hzn deploycheck policy -u $USERDEV_ADMIN_AUTH -n userdev/an12345 -b userdev/bp_gpstest -B input_files/business_pol_gpstest.json"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "\-b and \-B are mutually exclusive"

echo -e "\n${PREFIX} test input with node id and business policy id."
CMD="hzn deploycheck policy -u $USERDEV_ADMIN_AUTH -n userdev/an12345 -b userdev/bp_gpstest"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "true" ""

echo -e "\n${PREFIX} test input: wrong business policy id"
CMD="hzn deploycheck policy -u $USERDEV_ADMIN_AUTH -n userdev/an12345 -b userdev/bp_gpstestxxx"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "Deployment policy not found for"

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
CMD="hzn deploycheck policy -u $USERDEV_ADMIN_AUTH --node-pol input_files/compcheck/node_policy.json -B input_files/compcheck/business_pol_gpstest.json"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "true" ""

echo -e "\n${PREFIX} test input: node policy, business policy, service policy and service"
CMD="hzn deploycheck policy -u $USERDEV_ADMIN_AUTH --node-pol input_files/compcheck/node_policy.json -B input_files/compcheck/business_pol_gpstest.json --service-pol input_files/compcheck/service_policy.json --service input_files/compcheck/service_gpstest.json"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "true" ""

echo -e "\n${PREFIX} test input: node policy, business policy and service policy. Incompatible. Type mismatch"
CMD="hzn deploycheck policy -u $USERDEV_ADMIN_AUTH -t cluster --node-pol input_files/compcheck/node_policy.json -B input_files/compcheck/business_pol_gpstest.json --service-pol input_files/compcheck/service_policy.json"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "false" "Service does not have cluster deployment configuration for node type 'cluster'"

echo -e "\n${PREFIX} test input: node policy, business policy and service policy. wrong arch"
CMD="hzn deploycheck policy -u $USERDEV_ADMIN_AUTH  -a arm64 --node-pol input_files/compcheck/node_policy.json -B input_files/compcheck/business_pol_gpstest.json --service-pol input_files/compcheck/service_policy.json"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "false" "Service with 'arch' arm64 cannot be found in the deployment policy"

echo -e "\n${PREFIX} test input: node id, business policy. wrong arch"
CMD="hzn deploycheck policy -u $USERDEV_ADMIN_AUTH -n an12345 -B input_files/compcheck/business_pol_gpstest2.json"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "false" "Service with 'arch' ${ARCH} cannot be found in the deployment policy"

echo -e "\n${PREFIX} test input: node policy, business policy and service policy. not compatible"
CMD="hzn deploycheck policy -u $USERDEV_ADMIN_AUTH --node-pol input_files/compcheck/node_policy.json -B input_files/compcheck/business_pol_gpstest.json --service-pol input_files/compcheck/service_policy2.json"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "false" "Compatibility Error"

echo -e "\n${PREFIX} test input: mixed. node id, business policy. not compatible"
CMD="hzn deploycheck policy -u $USERDEV_ADMIN_AUTH  -n an12345 -B input_files/compcheck/business_pol_gpstest.json --service-pol input_files/compcheck/service_policy2.json"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "false" "Compatibility Error"

# bp_location has 2 services. one is compatible, the other one is not with input_files/compcheck/node_policy2.json
echo -e "\n${PREFIX} test input: mixed. node pol, business policy id. compatible, multible output."
CMD="hzn deploycheck policy -u $USERDEV_ADMIN_AUTH  -b userdev/bp_location --node-pol input_files/compcheck/node_policy2.json -c"
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
echo $RES | jq ".reason.\"e2edev@somecomp.com/bluehorizon.network-services-location_2.0.6_${ARCH}\"" | grep -q Incompatible
if [ $? -ne 0 ]; then
  echo "Service bluehorizon.network-services-location_2.0.6_${ARCH} should be incompatible but not."
  exit 2
fi
echo $RES | jq ".reason.\"e2edev@somecomp.com/bluehorizon.network-services-location_2.0.7_${ARCH}\"" | grep -q Incompatible
if [ $? -eq 0 ]; then
  echo "Service bluehorizon.network-services-location_2.0.7_${ARCH} should be compatible but not."
  exit 2
fi
echo "Compatibility result expected."

PREFIX="HZN userinput compatibility test:"

echo -e "\n${PREFIX} test conflict."
CMD="hzn deploycheck userinput -u $USERDEV_ADMIN_AUTH -n userdev/an12345 --node-ui input_files/node_ui.json -b userdev/bp_gpstest"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "\-n and \-\-node-ui are mutually exclusive"

echo -e "\n${PREFIX} test conflict2."
CMD="hzn deploycheck userinput -u $USERDEV_ADMIN_AUTH -n userdev/an12345 -b userdev/bp_gpstest -B input_files/business_pol_gpstest.json"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "\-b and \-B are mutually exclusive"

echo -e "\n${PREFIX} test conflict3."
CMD="hzn deploycheck userinput -u $USERDEV_ADMIN_AUTH -n userdev/an12345 -p e2edev@somecomp.com/sloc -P input_files/pattern_sloc.json"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "\-p and \-P are mutually exclusive"

echo -e "\n${PREFIX} test input with node id and business policy id."
CMD="hzn deploycheck userinput -u $USERDEV_ADMIN_AUTH -n userdev/an12345 -b userdev/bp_gpstest"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "true" ""

echo -e "\n${PREFIX} test input with node id and pattern id."
CMD="hzn deploycheck userinput -u $USERDEV_ADMIN_AUTH -n userdev/an12345 -p e2edev@somecomp.com/sloc"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "true" ""

echo -e "\n${PREFIX} test input: wrong business policy id"
CMD="hzn deploycheck userinput -u $USERDEV_ADMIN_AUTH -n userdev/an12345 -b userdev/bp_gpstestxxx"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "Deployment policy not found for"

echo -e "\n${PREFIX} test input: wrong pattern id"
CMD="hzn deploycheck userinput -u $USERDEV_ADMIN_AUTH -n userdev/an12345 -p e2edev@somecomp.com/slocxxx"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "Pattern not found for e2edev@somecomp.com/slocxxx"

echo -e "\n${PREFIX} test input: node org and business org missing, they pick up org from the user cred."
CMD="hzn deploycheck userinput -u $USERDEV_ADMIN_AUTH -n an12345 -b bp_gpstest"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "true" ""

echo -e "\n${PREFIX} test input: business policy id only. Use current node user input"
CMD="hzn deploycheck userinput -u $USERDEV_ADMIN_AUTH -b bp_netspeed"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "Neither node id nor node user input file is specified. Getting node user input from the local node" "\"compatible\": true"

echo -e "\n${PREFIX} test input: pattern id only. Use current node user input"
CMD="hzn deploycheck userinput -u $USERDEV_ADMIN_AUTH -p e2edev@somecomp.com/sloc"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "Neither node id nor node user input file is specified. Getting node user input from the local node" "\"compatible\": true"

echo -e "\n${PREFIX} test input: node user input and business policy"
CMD="hzn deploycheck userinput -u $USERDEV_ADMIN_AUTH --node-ui input_files/compcheck/node_ui.json -B $bp_location"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "true" ""

echo -e "\n${PREFIX} test input: node user input and pattern"
CMD="hzn deploycheck userinput -u $USERDEV_ADMIN_AUTH --node-ui input_files/compcheck/node_ui.json -P $pattern_sloc"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "true" ""

echo -e "\n${PREFIX} test input: node user input, business policy and service."
CMD="hzn deploycheck userinput -u $USERDEV_ADMIN_AUTH --node-ui input_files/compcheck/node_ui.json -B $bp_location --service $service_location"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "true" ""

echo -e "\n${PREFIX} test input: node user input, pattern and services."
CMD="hzn deploycheck userinput -u $USERDEV_ADMIN_AUTH --node-ui input_files/compcheck/node_ui.json -P $pattern_sloc --service $service_location --service input_files/compcheck/service_locgps.json"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "true" ""

echo -e "\n${PREFIX} test input: node policy, business policy and service policy. wrong arch"
CMD="hzn deploycheck userinput -u $USERDEV_ADMIN_AUTH -a arm64 --node-ui input_files/compcheck/node_ui.json -B $bp_location --service $service_location"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "No service versions with architecture arm64 specified in the deployment policy or pattern"

echo -e "\n${PREFIX} test input: node user input, pattern and services. wrong arch"
CMD="hzn deploycheck userinput -u $USERDEV_ADMIN_AUTH -a arm64 --node-ui input_files/compcheck/node_ui.json -P $pattern_sloc --service $service_location --service input_files/compcheck/service_locgps.json"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "No service versions with architecture arm64 specified in the deployment policy or pattern"

echo -e "\n${PREFIX} test input: node id, business policy. wrong arch"
CMD="hzn deploycheck userinput -u $USERDEV_ADMIN_AUTH -n an12345 -B input_files/compcheck/business_pol_gpstest2.json"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "No service versions with architecture ${ARCH} specified in the deployment policy or pattern"

echo -e "\n${PREFIX} test input: node user input, business policy and service. not compatible"
CMD="hzn deploycheck userinput -u $USERDEV_ADMIN_AUTH --node-ui input_files/compcheck/node_ui2.json -B $bp_location --service $service_location"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "false" "User Input Incompatible: Service definition not found in the input"
check_comp_results "$RES" "false" "User Input Incompatible: Failed to verify user input for dependent service"

echo -e "\n${PREFIX} test input: node user input, pattern and service. not compatible"
CMD="hzn deploycheck userinput -u $USERDEV_ADMIN_AUTH --node-ui input_files/compcheck/node_ui2.json -P $pattern_sloc --service $service_location"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "false" "User Input Incompatible: Service definition not found in the input"
check_comp_results "$RES" "false" "User Input Incompatible: Failed to verify user input for dependent service"

PREFIX="HZN all compatibility test:"

echo -e "\n${PREFIX} test conflict."
CMD="hzn deploycheck all -u $USERDEV_ADMIN_AUTH -n userdev/an12345 --node-ui input_files/node_ui.json -b userdev/bp_gpstest"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "\-n and \-\-node-ui are mutually exclusive"

echo -e "\n${PREFIX} test conflict2."
CMD="hzn deploycheck all -u $USERDEV_ADMIN_AUTH -n userdev/an12345 -b userdev/bp_gpstest -B input_files/business_pol_gpstest.json"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "\-b and \-B are mutually exclusive"

echo -e "\n${PREFIX} test conflict3."
CMD="hzn deploycheck all -u $USERDEV_ADMIN_AUTH -n userdev/an12345 -p e2edev@somecomp.com/sloc -P input_files/pattern_sloc.json"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "\-p and \-P are mutually exclusive"

echo -e "\n${PREFIX} test conflict4."
CMD="hzn deploycheck all -u $USERDEV_ADMIN_AUTH -n userdev/an12345 --node-pol input_files/node_policy.json -b userdev/bp_gpstest"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "\-n and \-\-node-pol are mutually exclusive"

echo -e "\n${PREFIX} test input with node id and business policy id."
CMD="hzn deploycheck all -u $USERDEV_ADMIN_AUTH -n userdev/an12345 -b userdev/bp_gpstest"
echo "$CMD"
RES=$($CMD 2>&1)
results "$RES" "true" ""

echo -e "\n${PREFIX} test input with node id and pattern id."
CMD="hzn deploycheck all -u $USERDEV_ADMIN_AUTH -n userdev/an12345 -p e2edev@somecomp.com/sloc"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "true" ""

echo -e "\n${PREFIX} test input: node user input, pattern and services."
CMD="hzn deploycheck all -u $USERDEV_ADMIN_AUTH --node-ui input_files/compcheck/node_ui.json -P $pattern_sloc --service $service_location --service input_files/compcheck/service_locgps.json --node-pol input_files/compcheck/node_policy.json -O userdev"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "true" ""

echo -e "\n${PREFIX} test input: node policy node user input, business policy, service, and service policy. Compatible"
CMD="hzn deploycheck all -u $USERDEV_ADMIN_AUTH --node-ui input_files/compcheck/node_ui.json --node-pol input_files/compcheck/node_policy.json --service-pol input_files/compcheck/service_policy.json -B $bp_location --service $service_location -O userdev"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "true" ""

echo -e "\n${PREFIX} test input: node policy node user input, business policy, service, and service policy. Incompatible"
CMD="hzn deploycheck all -u $USERDEV_ADMIN_AUTH --node-ui input_files/compcheck/node_ui2.json --node-pol input_files/compcheck/node_policy.json --service-pol input_files/compcheck/service_policy.json -B $bp_location --service $service_location -O userdev"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "false" "User Input Incompatible"

echo -e "\n${PREFIX} test input: node policy node user input, business policy, service, and service policy. One compatible, one not"
CMD="hzn deploycheck all -u $USERDEV_ADMIN_AUTH --node-ui input_files/compcheck/node_ui.json --node-pol input_files/compcheck/node_policy2.json --service-pol input_files/compcheck/service_policy.json -B $bp_location --service $service_location -O userdev -c"
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "true" ""
c=$(echo $RES | jq '.compatible')
if [ "$c" != "true" ]; then
  echo "It should return compatible but not."
  exit 2
fi
l=$(echo $RES | jq 'del(..|.general?) |.reason | length')
if [ "$l" != "2" ]; then
  echo "It should return 2 service result but got $l."
  exit 2
fi
echo $RES | jq ".reason.\"e2edev@somecomp.com/bluehorizon.network-services-location_2.0.6_${ARCH}\"" | grep -q Incompatible
if [ $? -ne 0 ]; then
  echo "Service bluehorizon.network-services-location_2.0.6_${ARCH} should be incompatible but not."
  exit 2
fi
echo $RES | jq ".reason.\"e2edev@somecomp.com/bluehorizon.network-services-location_2.0.7_${ARCH}\"" | grep -q Incompatible
if [ $? -eq 0 ]; then
  echo "Service bluehorizon.network-services-location_2.0.7_${ARCH} should be compatible but not."
  exit 2
fi
echo "Compatibility result expected."

echo -e "\n${PREFIX} test input: node policy node user input, business policy, service, and service policy. Incompatible"
CMD="hzn deploycheck all -u $USERDEV_ADMIN_AUTH --node-ui input_files/compcheck/node_ui.json --node-pol input_files/compcheck/node_policy.json --service-pol input_files/compcheck/service_policy2.json -B $bp_location --service $service_location "
echo "$CMD"
RES=$($CMD 2>&1)
check_comp_results "$RES" "false" "Policy Incompatible"

echo -e "\n${PREFIX} test service type checking. Compatible"
CMD="hzn deploycheck all -u $USERDEV_ADMIN_AUTH -p e2edev@somecomp.com/sall -c"
echo "$CMD"
RES=$($CMD 2>&1 | grep -v 'Neither node id')
check_comp_results "$RES" "true" "Service does not have deployment configuration for node type 'device'"

echo -e "\n${PREFIX} test service type checking, pattern. Incompatible"
CMD="hzn deploycheck all -u $USERDEV_ADMIN_AUTH -p e2edev@somecomp.com/sk8s"
echo "$CMD"
RES=$($CMD 2>&1 | grep -v 'Neither node id')
check_comp_results "$RES" "false" "Service does not have deployment configuration for node type 'device'"

echo -e "\n${PREFIX} test service type checking, business policy. Incompatible"
CMD="hzn deploycheck all -u $USERDEV_ADMIN_AUTH -b bp_k8s"
echo "$CMD"
RES=$($CMD 2>&1 | grep -v 'Neither node id')
check_comp_results "$RES" "false" "Service does not have deployment configuration for node type 'device'"

# secret binding tests
if [ "$NOVAULT" != "1" ]; then
  export HZN_AGBOT_URL=${AGBOT_SAPI_URL}

  echo -e "\n${PREFIX} test secret binding with business policy and service input. Compatible"
  CMD="hzn deploycheck secretbinding -u $USERDEV_ADMIN_AUTH -B input_files/compcheck/business_pol_location_secrets.json --service input_files/compcheck/service_location_secrets.json"
  echo "$CMD"
  RES=$($CMD 2>&1 | grep -v 'Getting the node information from the local node')
  check_comp_results "$RES" "true"

  echo -e "\n${PREFIX} test secret binding with pattern and service input. Compatible"
  CMD="hzn deploycheck secretbinding -u $USERDEV_ADMIN_AUTH -P input_files/compcheck/pattern_sloc_secrets.json --service input_files/compcheck/service_location_secrets.json --service input_files/compcheck/service_locgps.json"
  echo "$CMD"
  RES=$($CMD 2>&1 | grep -v 'Getting the node information from the local node')
  check_comp_results "$RES" "true"
fi

echo -e "\n${PREFIX} complete test\n"
