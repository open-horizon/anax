#!/bin/bash

TEST_DIFF_ORG=${TEST_DIFF_ORG:-1}

export ARCH=${ARCH}

function set_exports {
  if [ "$NOANAX" != "1" ]
  then
    export USER=anax1
    export PASS=anax1pw
    export DEVICE_ID="an12345"
    export DEVICE_NAME="anaxdev1"
    export DEVICE_ORG="e2edev@somecomp.com"
    export TOKEN="Abcdefghijklmno1"

    export HZN_AGENT_PORT=8510
    export ANAX_API="http://localhost:${HZN_AGENT_PORT}"
    export EXCH="${EXCH_APP_HOST}"
    if [ ${CERT_LOC} -eq "1" ]; then
      export HZN_MGMT_HUB_CERT_PATH="/certs/css.crt"
    fi

    if [[ $TEST_DIFF_ORG -eq 1 ]]; then
      export USER=useranax1
      export PASS=useranax1pw
      export DEVICE_ORG="userdev"
    fi
  else
    echo -e "Anax is disabled"
  fi
}

function run_delete_loops {
  # Start the deletion loop tests if they have not been disabled.
  echo -e "No loop setting is $NOLOOP"

  # get the admin auth for verify_agreements.sh
   local admin_auth="e2edevadmin:e2edevadminpw"
   if [ "$DEVICE_ORG" == "userdev" ]; then
     admin_auth="userdevadmin:userdevadminpw"
   fi

  if [ "$NOLOOP" != "1" ] && [ "$NOAGBOT" != "1" ]
  then
    echo "Starting deletion loop tests. Giving time for 1st agreement to complete."
    sleep 240

    echo "Starting device delete agreement script"
    ./del_loop.sh &

    # Give the device script time to get started and get into it's 10 min cycle. Wait 5 mins
    # and then start the agbot delete cycle, so that it is interleaved with the device cycle.
    sleep 300
    ./agbot_del_loop.sh &
  else
    echo -e "Deletion loop tests set to only run once."

    if [ "${PATTERN}" == "sall" ] || [ "${PATTERN}" == "sloc" ] || [ "${PATTERN}" == "sns" ] || [ "${PATTERN}" == "sgps" ] || [ "${PATTERN}" == "spws" ] || [ "${PATTERN}" == "susehello" ] || [ "${PATTERN}" == "shelm" ]; then
      echo -e "Starting service pattern verification scripts"
      if [ "$NOLOOP" == "1" ]; then
        ORG_ID=${DEVICE_ORG} ADMIN_AUTH=${admin_auth} ./verify_agreements.sh
        if [ $? -ne 0 ]; then echo "Verify agreement failure."; exit 1; fi
         echo -e "No cancellation setting is $NOCANCEL"
        if [ "$NOCANCEL" != "1" ]; then
          ./del_loop.sh
          if [ $? -ne 0 ]; then echo "Agreement deletion failure."; exit 1; fi
          echo -e "Sleeping for 30s between device and agbot agreement deletion"
          sleep 30
          ./agbot_del_loop.sh
          if [ $? -ne 0 ]; then echo "Agbot agreement deletion failure."; exit 1; fi
          ORG_ID=${DEVICE_ORG} ADMIN_AUTH=${admin_auth} ./verify_agreements.sh
          if [ $? -ne 0 ]; then echo "Agreement restart failure."; exit 1; fi
        else
          echo -e "Cancellation tests are disabled"
        fi
      else
        ORG_ID=${DEVICE_ORG} ADMIN_AUTH=${admin_auth} ./verify_agreements.sh &
      fi
    else
      echo -e "Verifying policy based workload deployment"
      echo -e "No cancellation setting is $NOCANCEL"
      if [ "$NOCANCEL" != "1" ]; then
        if [ "$NONS" == "1" ] || [ "$NOPWS" == "1" ] || [ "$NOLOC" == "1" ] || [ "$NOGPS" == "1" ] || [ "$NOHELLO" == "1" ] || [ "$NOK8S" == "1" ]; then
          echo "Skipping agreement verification"
          sleep 30
        else
          ORG_ID=${DEVICE_ORG} ADMIN_AUTH=${admin_auth} ./verify_agreements.sh
          if [ $? -ne 0 ]; then echo "Verify agreement failure."; exit 1; fi
        fi
        ./del_loop.sh
        if [ $? -ne 0 ]; then echo "Agreement deletion failure."; exit 1; fi
        echo -e "Sleeping for 30s between device and agbot agreement deletion"
        sleep 30
        ./agbot_del_loop.sh
        if [ $? -ne 0 ]; then echo "Agbot agreement deletion failure."; exit 1; fi
      else
        echo -e "Cancellation tests are disabled"
      fi
      if [ "$NONS" == "1" ] || [ "$NOPWS" == "1" ] || [ "$NOLOC" == "1" ] || [ "$NOGPS" == "1" ] || [ "$NOHELLO" == "1" ] || [ "$NOK8S" == "1" ]; then
        echo "Skipping agreement verification"
      else
        ORG_ID=${DEVICE_ORG} ADMIN_AUTH=${admin_auth} ./verify_agreements.sh
        if [ $? -ne 0 ]; then echo "Verify agreement failure."; exit 1; fi
      fi
    fi
  fi
}

EXCH_URL="${EXCH_APP_HOST}"

# the horizon var base for storing the keys. It is the default value for HZN_VAR_BASE.
mkdir -p /var/horizon
mkdir -p /var/horizon/.colonus

# update host file if needed
if [ "$ICP_HOST_IP" != "0" ]
then
  echo "Updating hosts file."
  HOST_NAME_ICP=`echo $EXCH_URL | awk -F/ '{print $3}' | sed 's/:.*//g'`
  HOST_NAME=`echo $EXCH_URL | awk -F/ '{print $3}' | sed 's/:.*//g' | sed 's/\.icp*//g'`
  echo "$ICP_HOST_IP $HOST_NAME_ICP $HOST_NAME"
  echo "$ICP_HOST_IP $HOST_NAME_ICP $HOST_NAME" >> /etc/hosts
fi

cd /root

# Build an old anax if we need it
if [ "$OLDANAX" == "1" ]; then
  ./build_old_anax.sh
  if [ $? -ne 0 ]; then
    exit -1
  fi
fi

#--cacert /certs/css.crt
if [ ${CERT_LOC} -eq "1" ]; then
  CERT_VAR="--cacert /certs/css.crt"
else
  CERT_VAR=""
fi

# Start the API Key tests if it has been set
#if [ ${API_KEY} != "0" ]; then
#  echo -e "Starting API Key test."
#  ./api_key.sh
#  if [ $? -ne 0 ]
#  then
#    echo -e "API Key test failure."
#    exit -1
#  fi
#fi

# test the CSS API
./sync_service_test.sh
if [ $? -ne 0 ]
then
  echo -e "Model management sync service test failure."
  exit -1
fi

# Setup to use the anax registration APIs
if [ "$TESTFAIL" != "1" ]
then
  export USER=anax1
  export PASS=anax1pw
  export DEVICE_ID="an12345"
  export DEVICE_NAME="an12345"
  export DEVICE_ORG="e2edev@somecomp.com"
  export HZN_AGENT_PORT=8510
  export ANAX_API="http://localhost:${HZN_AGENT_PORT}"
  export EXCH="${EXCH_APP_HOST}"
  export TOKEN="Abcdefghijklmno1"

  if [ ${CERT_LOC} -eq "1" ]; then
    export HZN_MGMT_HUB_CERT_PATH="/certs/css.crt"
  fi

  if [[ $TEST_DIFF_ORG -eq 1 ]]; then
    export USER=useranax1
    export PASS=useranax1pw
    export DEVICE_ORG="userdev"
  fi

  # Start Anax
  echo "Starting Anax1 for tests."
  if [ ${CERT_LOC} -eq "1" ]; then
    /usr/local/bin/anax -v=5 -alsologtostderr=true -config /etc/colonus/anax-combined.config >/tmp/anax.log 2>&1 &
  else
    /usr/local/bin/anax -v=5 -alsologtostderr=true -config /etc/colonus/anax-combined-no-cert.config >/tmp/anax.log 2>&1 &
  fi

  sleep 5

  TESTFAIL="0"
  echo "Running API tests"
  ./apitest.sh
  if [ $? -ne 0 ]
  then
    echo "API Test failure."
    TESTFAIL="1"
    exit 1
  else
    echo "API tests completed SUCCESSFULLY."

    echo "Killing anax and cleaning up."
    kill $(pidof anax)
    rm -fr /root/.colonus/*.db
    rm -fr /root/.colonus/policy.d/*
  fi
fi

echo -e "No agbot setting is $NOAGBOT"
HZN_AGBOT_API=${AGBOT_API}
if [ "$NOAGBOT" != "1" ] && [ "$TESTFAIL" != "1" ]
then
  if [ "${EXCH_APP_HOST}" = "http://exchange-api:8081/v1" ]; then
    # Check that the agbot is still alive
    if ! curl -sSL ${AGBOT_API}/agreement > /dev/null; then
      echo "Agreement Bot 1 verification failure."
      TESTFAIL="1"
      exit 1
    fi

    if [ "$MULTIAGBOT" == "1" ]; then
      if ! curl -sSL ${AGBOT2_API}/agreement > /dev/null; then
        echo "Agreement Bot 2 verification failure."
        TESTFAIL="1"
        exit 1
      fi
    fi
  fi
else
  echo -e "Agbot is disabled"
fi

# Setup real ARCH value in all policies, patterns & service definition files for tests
for in_file in /root/input_files/compcheck/*.json
do
  sed -i -e "s#__ARCH__#${ARCH}#g" $in_file
  if [ $? -ne 0 ]
  then
    echo "Providing real architecture value failure."
    TESTFAIL="1"
    exit 1
  fi
done

echo "TEST_PATTERNS=${TEST_PATTERNS}"

# Services can be run via patterns or from policy files
if [[ "${TEST_PATTERNS}" == "" ]] && [ "$TESTFAIL" != "1" ]
then
  echo -e "Making agreements based on policy files."

  set_exports
  export PATTERN=""

  ./start_node.sh
  if [ $? -ne 0 ]
  then
    echo "Node start failure."
    TESTFAIL="1"
  else
    run_delete_loops
    if [ $? -ne 0 ]
    then
      echo "Delete loop failure."
      TESTFAIL="1"
    fi
  fi

  if [ "$NOSVC_CONFIGSTATE" != "1" ]; then
    ./service_configstate_test.sh
    if [ $? -ne 0 ]
    then
      echo "Service configstate test failure."
      TESTFAIL="1"
    fi
  fi

elif [ "$TESTFAIL" != "1" ]; then
  # make agreements based on patterns
  last_pattern=$(echo $TEST_PATTERNS |sed -e 's/^.*,//')
  echo -e "Last pattern is $last_pattern"

  for pat in $(echo $TEST_PATTERNS | tr "," " "); do
    export PATTERN=$pat
    echo -e "***************************"
    echo -e "Start testing pattern $PATTERN..."

    # Because of the limitation of docker networks, if the pattern for 
    # the main agent is sall, the pattern for the multi-agent will be sns. 
    # Otherwide they will have the same pattern. 
    ma_pattern=$PATTERN
    if [ "${PATTERN}" == "sall" ]; then
      ma_pattern="sns"
    fi

    # Allocate port 80 to see what anax does
    # socat - TCP4-LISTEN:80,crlf &

    # start pattern test
    set_exports $pat

    # start main agent
    ./start_node.sh
    if [ $? -ne 0 ]
    then
      echo "Node start failure."
      TESTFAIL="1"
      break
    fi

    # start multiple agents
    source ./multiple_agents.sh
    if [ -n "$MULTIAGENTS" ] && [ "$MULTIAGENTS" != "0" ]; then
      echo "Starting multiple agents with pattern ${ma_pattern} ..."
      PATTERN=${ma_pattern} startMultiAgents
      if [ $? -ne 0 ]; then
        echo "Multiple agent startup failure."
        TESTFAIL="1"
        break
      fi
    fi

    run_delete_loops
    if [ $? -ne 0 ]
    then
      echo "Delete loop failure."
      TESTFAIL="1"
      break
    fi

    if [ -n "$MULTIAGENTS" ] && [ "$MULTIAGENTS" != "0" ]; then
      echo "Checking multiple agents..."
      PATTERN=${ma_pattern} verifyMultiAgentsAgreements
      if [ $? -ne 0 ]; then
        echo "Multiple agent agreement varification failure."
        TESTFAIL="1"
        break
      fi
    fi

    if [ "$NORETRY" != "1" ]; then
      ./service_retry_test.sh
      if [ $? -ne 0 ]
      then
        echo "Service retry failure."
        TESTFAIL="1"
        break
      fi
    fi

    if [ "$NOSVC_CONFIGSTATE" != "1" ]; then
      ./service_configstate_test.sh
      if [ $? -ne 0 ]
      then
        echo "Service configstate test failure."
        TESTFAIL="1"
        break
      fi
    fi

    echo -e "Done testing pattern $PATTERN"

    # unregister if it is not the last pattern
    if [ "$pat" != "$last_pattern" ]; then
      # Save off the existing log file, in case the next test fails and we need to look back to see how this
      # instance of anax actually ended.
      mv /tmp/anax.log /tmp/anax_$pat.log

      echo -e "Unregister the node. Anax will be shutdown."
      ./unregister.sh
      if [ $? -eq 0 ]; then
        sleep 10
      else
        exit 1
      fi
    fi
    echo -e "***************************"
  done

fi

if [ "$NOCOMPCHECK" != "1" ] && [ "$TESTFAIL" != "1" ]; then
  if [ "$TEST_PATTERNS" == "sall" ] || [ "$TEST_PATTERNS" == "" ]; then
    ./agbot_apitest.sh
    if [ $? -ne 0 ]
    then
      echo "Policy compatibility test using Agbot API failure."
      exit 1
    fi

    ./hzn_compcheck.sh
    if [ $? -ne 0 ]
    then
      echo "Policy compatibility test using hzn command failure."
      exit 1
    fi

    if [ "$NOVAULT" != "1" ]; then 
      ./hzn_secretsmanager.sh 
      if [ $? -ne 0 ]
      then
        echo "Policy compatibility test using hzn secretsmanager command failure."
        exit 1
      fi
    fi
  fi

fi

if [ "$NOSDO" != "1" ] && [ "$TESTFAIL" != "1" ]; then
  ./hzn_sdo.sh
  if [ $? -ne 0 ]; then
    echo "SDO test using hzn command failure."
    exit 1
  fi
fi

if [ "$NOSURFERR" != "1" ] && [ "$TESTFAIL" != "1" ] && [ "${EXCH_APP_HOST}" == "http://exchange-api:8081/v1" ]; then
  if [ "$TEST_PATTERNS" == "sall" ] || [ "$TEST_PATTERNS" == "" ] && [ "$NOLOOP" == "1" ] && [ "$NONS" == "" ] && [ "$NOGPS" == "" ] && [ "$NOPWS" == "" ] && [ "$NOLOC" == "" ] && [ "$NOHELLO" == "" ] && [ "$NOK8S" == "" ]; then
    ./verify_surfaced_error.sh
    if [ $? -ne 0 ]; then echo "Verify surfaced error failure."; exit 1; fi
  fi
fi

if [ "$NOUPGRADE" != "1" ] && [ "$TESTFAIL" != "1" ] && [ "${EXCH_APP_HOST}" == "http://exchange-api:8081/v1" ]; then
  if [ "$TEST_PATTERNS" == "sall" ]; then
    ./service_upgrading_downgrading_test.sh
    if [ $? -ne 0 ]; then echo "Service upgrading/downgrading test failure."; exit 1; fi
  fi
fi

if [ "$TEST_PATTERNS" == "" ] && [ "$NOVAULT" != "1" ] && [ "$TESTFAIL" != "1" ] && [ "$NOLOOP" == "1" ] && [ "$NONS" == "" ] && [ "$NOGPS" == "" ] && [ "$NOPWS" == "" ] && [ "$NOLOC" == "" ] && [ "$NOHELLO" == "" ] && [ "$NOK8S" == "" ]; then
  ./service_secrets_test.sh
  if [ $? -ne 0 ]; then echo "Service secret test failure."; exit 1; fi
fi

if [ "$NOHZNREG" != "1" ] && [ "$TESTFAIL" != "1" ]; then
  if [ "$TEST_PATTERNS" == "sall" ]; then
    echo "Sleeping 15 seconds..."
    sleep 15

    ./hzn_reg.sh
    if [ $? -ne 0 ]; then
      echo "Failed registering and unregistering tests with hzn commands."
      exit 1
    fi
  fi
fi

if [ "$TEST_PATTERNS" == "sall" ] && [ "$NOHZNLOG" != "1" ] && [ "$NOHZNREG" != "1" ] && [ "$TESTFAIL" != "1" ]; then
  ./service_log_test.sh
  if [ $? -ne 0 ]; then
    echo "Failed hzn service log tests."
    exit 1
  fi
fi

if [ "$NOPATTERNCHANGE" != "1" ] && [ "$TESTFAIL" != "1" ]; then
  if [ "$TEST_PATTERNS" == "sall" ]; then
    ./pattern_change.sh
    if [ $? -ne 0 ]; then
      echo "Failed node pattern change tests."
      exit 1
    fi
  fi
fi

# Start the node unconfigure tests if they have been enabled.
echo -e "Node unconfig setting is $UNCONFIG"
if [ "$UNCONFIG" == "1" ] && [ "$NOAGBOT" != "1" ] && [ "$TESTFAIL" != "1" ]
then
  echo "Starting unconfig loop tests. Giving time for 1st agreements to complete."
  sleep 120

  echo "Starting device unconfigure script"
  ./unconfig_loop.sh &
else
  echo -e "Unconfig loop tests are disabled."
fi

#Start the edge cluster verification test.
if [ "$NOKUBE" != "1" ] && [ "$TESTFAIL" != "1" ] && [ "${TEST_PATTERNS}" == "" ]
then
  echo -e "Verifying edge cluster agreement"
  ./verify_edge_cluster.sh
  if [ $? -ne 0 ]; then
    echo "Failed edge cluster verification tests."
    exit 1
  fi
else
  echo -e "Edge cluster agreement verification skipped."
fi

# Clean up remote environment
if [ "${EXCH_APP_HOST}" != "http://exchange-api:8081/v1" ]; then
  echo "Clean up remote environment"
  echo "Delete e2edev@somecomp.com..."
  DL8ORG=$(curl -X DELETE $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d '{"label":"E2EDev","description":"E2EDevTest","orgType":"IBM"}' "${EXCH_URL}/orgs/e2edev@somecomp.com" | jq -r '.msg')
  echo "$DL8ORG"

  echo "Delete userdev organization..."
  DL8UORG=$(curl -X DELETE $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d '{"label":"UserDev","description":"UserDevTest"}' "${EXCH_URL}/orgs/userdev" | jq -r '.msg')
  echo "$DL8UORG"

  echo "Delete Customer1 organization..."
  DL8C1ORG=$(curl -X DELETE $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d '{"label":"Customer1","description":"The Customer1 org"}' "${EXCH_URL}/orgs/Customer1" | jq -r '.msg')
  echo "$DL8C1ORG"

  echo "Delete Customer2 organization..."
  DL8C2ORG=$(curl -X DELETE $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d '{"label":"Customer2","description":"The Customer2 org"}' "${EXCH_URL}/orgs/Customer2" | jq -r '.msg')
  echo "$DL8C2ORG"

  # Delete an IBM admin user in the exchange
  echo "Delete an admin user for IBM org..."
  DL8IBM=$(curl -X DELETE $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d '{"password":"ibmadminpw","email":"ibmadmin%40ibm.com","admin":true}' "${EXCH_URL}/orgs/IBM/users/ibmadmin" | jq -r '.msg')
  echo "$DL8IBM"

  # Delete agreement bot user in the exchange
  echo "Delete Agbot user..."
  DL8AGBOT=$(curl -X DELETE $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d '{"password":"agbot1pw","email":"me%40gmail.com","admin":false}' "${EXCH_URL}/orgs/IBM/users/agbot1" | jq -r '.msg')
  echo "$DL8AGBOT"

  echo "Delete network_1.5.0 ..."
  DLHELM100=$(curl -X DELETE $CERT_VAR  --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" "${EXCH_URL}/orgs/IBM/services/bluehorizon.network-services-network_1.5.0_${ARCH}")
  echo "$DL150"

  echo "Delete network2_1.5.0 ..."
  DLHELM100=$(curl -X DELETE $CERT_VAR  --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" "${EXCH_URL}/orgs/IBM/services/bluehorizon.network-services-network2_1.5.0_${ARCH}")
  echo "$DL2150"

  echo "Delete helm-service_1.0.0 ..."
  DLHELM100=$(curl -X DELETE $CERT_VAR  --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" "${EXCH_URL}/orgs/IBM/services/my.company.com-services-helm-service_1.0.0_${ARCH}")
  echo "$DLHELM100"

  echo "Delete Userdev Org Definition ..."
  DL8USERDEVDEF=$(curl -X DELETE $CERT_VAR  --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" "${EXCH_URL}/orgs/IBM/agbots/${AGBOT_NAME}/businesspols/userdev_*_userdev")
  echo "$DL8USERDEVDEF"

  echo "Delete E2E Org Definition ..."
  DL8E2EDEF=$(curl -X DELETE $CERT_VAR  --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" "${EXCH_URL}/orgs/IBM/agbots/${AGBOT_NAME}/businesspols/e2edev@somecomp.com_*_e2edev@somecomp.com")
  echo "$DL8E2EDEF"

  echo "Delete Pattern Definition E2E ..."
  DL8PATTERNDEFE2E=$(curl -X DELETE $CERT_VAR  --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" "${EXCH_URL}/orgs/IBM/agbots/${AGBOT_NAME}/patterns/e2edev@somecomp.com_*_e2edev@somecomp.com")
  echo "$DL8PATTERNDEFE2E"

  echo "Delete Pattern Definition UserDev ..."
  DL8PATTERNDUSERDEV=$(curl -X DELETE $CERT_VAR  --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" "${EXCH_URL}/orgs/IBM/agbots/${AGBOT_NAME}/patterns/e2edev@somecomp.com_*_userdev")
  echo "$DL8PATTERNDUSERDEV"

  echo "Delete Pattern Definition SNS ..."
  DL8PATTERNSNS=$(curl -X DELETE $CERT_VAR  --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" "${EXCH_URL}/orgs/IBM/agbots/${AGBOT_NAME}/patterns/e2edev@somecomp.com_sns_e2edev@somecomp.com")
  echo "$DL8PATTERNSNS"
fi

if [ "$NOLOOP" == "1" ]; then
  if [ "$TESTFAIL" != "1" ]; then
    echo "All tests SUCCESSFUL"
  else
    echo "Test failures occured, check logs"
    exit 1
  fi
else
  # Keep everything alive
  while :
  do
    sleep 300
  done
fi
