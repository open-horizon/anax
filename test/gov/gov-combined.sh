#!/bin/bash

TEST_DIFF_ORG=${TEST_DIFF_ORG:-1}

function set_exports {
  if [ "$NOANAX" != "1" ]
  then
    export USER=anax1
    export PASS=anax1pw
    export DEVICE_ID="an12345"
    export DEVICE_NAME="anaxdev1"
    export DEVICE_ORG="e2edev@somecomp.com"
    export TOKEN="abcdefg"

    export ANAX_API="http://localhost"
    export EXCH="${EXCH_APP_HOST}"

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

    if [ "${PATTERN}" == "sall" ] || [ "${PATTERN}" == "sloc" ] || [ "${PATTERN}" == "sns" ] || [ "${PATTERN}" == "sgps" ] || [ "${PATTERN}" == "spws" ] || [ "${PATTERN}" == "susehello" ] || [ "${PATTERN}" == "cpu2msghub" ] || [ "${PATTERN}" == "shelm" ]; then
      echo -e "Starting service pattern verification scripts"
      if [ "$NOLOOP" == "1" ]; then
        ./verify_agreements.sh
        if [ $? -ne 0 ]; then echo "Verify agreement failure."; exit 1; fi
        echo -e "No cancellation setting is $NOCANCEL"
        if [ "$NOCANCEL" != "1" ]; then
          ./del_loop.sh
          if [ $? -ne 0 ]; then echo "Agreement deletion failure."; exit 1; fi
          echo -e "Sleeping for 30s between device and agbot agreement deletion"
          sleep 30
          ./agbot_del_loop.sh
          if [ $? -ne 0 ]; then echo "Agbot agreement deletion failure."; exit 1; fi
          ./verify_agreements.sh
          if [ $? -ne 0 ]; then echo "Agreement restart failure."; exit 1; fi
        else
          echo -e "Cancellation tests are disabled"
        fi
      else
        ./verify_agreements.sh &
      fi
    else
      echo -e "Verifying policy based workload deployment"
      echo -e "No cancellation setting is $NOCANCEL"
      if [ "$NOCANCEL" != "1" ]; then
        if [ "$NONS" == "1" ] || [ "$NOPWS" == "1" ] || [ "$NOLOC" == "1" ] || [ "$NOGPS" == "1" ] || [ "$NOHELLO" == "1" ]; then
          echo "Skipping agreement verification"
          sleep 30
        else
          ./verify_agreements.sh
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
      if [ "$NONS" == "1" ] || [ "$NOPWS" == "1" ] || [ "$NOLOC" == "1" ] || [ "$NOGPS" == "1" ] || [ "$NOHELLO" == "1" ]; then
        echo "Skipping agreement verification"
      else
        ./verify_agreements.sh
        if [ $? -ne 0 ]; then echo "Verify agreement failure."; exit 1; fi
      fi
    fi
  fi
}

EXCH_URL="${EXCH_APP_HOST}"

# the horizon var base for storing the keys. It is the default value for HZN_VAR_BASE.
mkdir -p /var/horizon
mkdir -p /var/horizon/.colonus

# Build an old anax if we need it
if [ "$OLDANAX" == "1" ] || [ "$OLDAGBOT" == "1" ]
then
  echo "Building old anax."
  chown -R root:root /root/.ssh
  mkdir -p /tmp/oldanax/anax-gopath/src/github.com/open-horizon
  mkdir -p /tmp/oldanax/anax-gopath/bin
  export GOPATH="/tmp/oldanax/anax-gopath"
  export TMPDIR="/tmp/oldanax/"
  cd /tmp/oldanax/anax-gopath/src/github.com/open-horizon
  git clone git@github.com:open-horizon/anax.git
  cd /tmp/oldanax/anax-gopath/src/github.com/open-horizon/anax
  make
  cp anax /usr/bin/old-anax
  export GOPATH="/tmp"
  unset TMPDIR
  cd /tmp
fi
#--cacert /certs/css.crt
if [ ${CERT_LOC} -eq "1" ]; then
  CERT_VAR="--cacert /certs/css.crt"
else
  CERT_VAR=""
fi

sleep 20

if [ "${EXCH_APP_HOST}" = "http://exchange-api:8080/v1" ]; then
  # Clean up the exchange DB to make sure we start out clean
  echo "Drop and recreate the exchange DB."

  UPGRADEDB=$(curl -sLX POST --header "Authorization:Basic root/root:${EXCH_ROOTPW}" "${EXCH_URL}/admin/upgradedb" | jq -r '.msg')
  echo "Exchange DB Upgrade Response: $UPGRADEDB"

  # loop until DBTOK contains a string value
  while :
  do
    DBTOK=$(curl -sLX GET --header "Authorization:Basic root/root:${EXCH_ROOTPW}" "${EXCH_URL}/admin/dropdb/token" | jq -r '.token')
    if test -z "$DBTOK"
    then
      sleep 5
    else
      break
    fi
  done

  DROPDB=$(curl -sLX POST --header "Authorization:Basic root/root:"$DBTOK "${EXCH_URL}/admin/dropdb" | jq -r '.msg')
  echo "Exchange DB Drop Response: $DROPDB"

  INITDB=$(curl -sLX POST --header "Authorization:Basic root/root:${EXCH_ROOTPW}" "${EXCH_URL}/admin/initdb" | jq -r '.msg')
  echo "Exchange DB Init Response: $INITDB"

  cd /root

else

  cd /root

  echo "Delete e2edev@somecomp.com..."
  DL8ORG=$(curl -X DELETE $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:${EXCH_ROOTPW}" -d '{"label":"E2EDev","description":"E2EDevTest","orgType":"IBM"}' "${EXCH_URL}/orgs/e2edev@somecomp.com" | jq -r '.msg')
  echo "$DL8ORG"

  echo "Delete userdev organization..."
  DL8UORG=$(curl -X DELETE $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:${EXCH_ROOTPW}" -d '{"label":"UserDev","description":"UserDevTest"}' "${EXCH_URL}/orgs/userdev" | jq -r '.msg')
  echo "$DL8UORG"

  echo "Delete Customer1 organization..."
  DL8C1ORG=$(curl -X DELETE $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:${EXCH_ROOTPW}" -d '{"label":"Customer1","description":"The Customer1 org"}' "${EXCH_URL}/orgs/Customer1" | jq -r '.msg')
  echo "$DL8C1ORG"

  echo "Delete Customer2 organization..."
  DL8C2ORG=$(curl -X DELETE $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:${EXCH_ROOTPW}" -d '{"label":"Customer2","description":"The Customer2 org"}' "${EXCH_URL}/orgs/Customer2" | jq -r '.msg')
  echo "$DL8C2ORG"

  # Delete an IBM admin user in the exchange
  echo "Delete an admin user for IBM org..."
  DL8IBM=$(curl -X DELETE $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:${EXCH_ROOTPW}" -d '{"password":"ibmadminpw","email":"ibmadmin%40ibm.com","admin":true}' "${EXCH_URL}/orgs/IBM/users/ibmadmin" | jq -r '.msg')
  echo "$DL8IBM"

  # Delete agreement bot user in the exchange
  echo "Delete Agbot user..."
  DL8AGBOT=$(curl -X DELETE $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:${EXCH_ROOTPW}" -d '{"password":"agbot1pw","email":"me%40gmail.com","admin":false}' "${EXCH_URL}/orgs/IBM/users/agbot1" | jq -r '.msg')
  echo "$DL8AGBOT"

  echo "Delete network_1.5.0 user..."
  DL150=$(curl -X DELETE $CERT_VAR "${EXCH_URL}/orgs/IBM/services/bluehorizon.network-services-network_1.5.0_amd64?username=root%2Froot&password=${EXCH_ROOTPW}" -H "accept: application/json")
  echo "$DL150"

  echo "Delete network2_1.5.0 user..."
  DL2150=$(curl -X DELETE $CERT_VAR "${EXCH_URL}/orgs/IBM/services/bluehorizon.network-services-network2_1.5.0_amd64?username=root%2Froot&password=${EXCH_ROOTPW}" -H "accept: application/json")
  echo "$DL2150"

  echo "Delete helm-service_1.0.0 user..."
  DLHELM100=$(curl -X DELETE $CERT_VAR "${EXCH_URL}/orgs/IBM/services/my.company.com-services-helm-service_1.0.0_amd64?username=root%2Froot&password=${EXCH_ROOTPW}" -H "accept: application/json")
  echo "$DLHELM100"

  sleep 30
fi

# Create the organizations we need
echo "Creating e2edev@somecomp.com organization..."

CR8EORG=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:${EXCH_ROOTPW}" -d '{"label":"E2EDev","description":"E2EDevTest","orgType":"IBM"}' "${EXCH_URL}/orgs/e2edev@somecomp.com" | jq -r '.msg')
echo "$CR8EORG"

echo "Creating userdev organization..."
CR8UORG=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:${EXCH_ROOTPW}" -d '{"label":"UserDev","description":"UserDevTest"}' "${EXCH_URL}/orgs/userdev" | jq -r '.msg')
echo "$CR8UORG"

if [ "${EXCH_APP_HOST}" = "http://exchange-api:8080/v1" ]; then
  echo "Creating IBM organization..."
  CR8IORG=$(curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:${EXCH_ROOTPW}" -d '{"label":"IBMorg","description":"IBM"}' "${EXCH_URL}/orgs/IBM" | jq -r '.msg')
  echo "$CR8IORG"
fi

echo "Creating Customer1 organization..."
CR8C1ORG=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:${EXCH_ROOTPW}" -d '{"label":"Customer1","description":"The Customer1 org"}' "${EXCH_URL}/orgs/Customer1" | jq -r '.msg')
echo "$CR8C1ORG"

echo "Creating Customer2 organization..."
CR8C2ORG=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:${EXCH_ROOTPW}" -d '{"label":"Customer2","description":"The Customer2 org"}' "${EXCH_URL}/orgs/Customer2" | jq -r '.msg')
echo "$CR8C2ORG"

# Register an e2edev@somecomp.com admin user in the exchange
echo "Creating an admin user for e2edev@somecomp.com organization..."
CR8EADM=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:${EXCH_ROOTPW}" -d '{"password":"e2edevadminpw","email":"me%40gmail.com","admin":true}' "${EXCH_URL}/orgs/e2edev@somecomp.com/users/e2edevadmin" | jq -r '.msg')
echo "$CR8EADM"

# Register an userdev admin user in the exchange
echo "Creating an admin user for userdev organization..."
CR8UADM=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:${EXCH_ROOTPW}" -d '{"password":"userdevadminpw","email":"me%40gmail.com","admin":true}' "${EXCH_URL}/orgs/userdev/users/userdevadmin" | jq -r '.msg')
echo "$CR8UADM"

# Register an ICP user in the customer1 org
echo "Creating an ICP admin user for Customer1 organization..."
CR81ICPADM=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:${EXCH_ROOTPW}" -d '{"password":"icpadminpw","email":"me%40gmail.com","admin":true}' "${EXCH_URL}/orgs/Customer1/users/icpadmin" | jq -r '.msg')
echo "$CR81ICPADM"

# Register an ICP user in the customer2 org
echo "Creating an ICP admin user for Customer2 organization..."
CR82ICPADM=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:${EXCH_ROOTPW}" -d '{"password":"icpadminpw","email":"me%40gmail.com","admin":true}' "${EXCH_URL}/orgs/Customer2/users/icpadmin" | jq -r '.msg')
echo "$CR82ICPADM"

# Register an IBM admin user in the exchange
echo "Creating an admin user for IBM org..."
CR8IBM=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:${EXCH_ROOTPW}" -d '{"password":"ibmadminpw","email":"ibmadmin%40ibm.com","admin":true}' "${EXCH_URL}/orgs/IBM/users/ibmadmin" | jq -r '.msg')
echo "$CR8IBM"

# Register agreement bot user in the exchange
echo "Creating Agbot user..."
CR8AGBOT=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:${EXCH_ROOTPW}" -d '{"password":"agbot1pw","email":"me%40gmail.com","admin":false}' "${EXCH_URL}/orgs/IBM/users/agbot1" | jq -r '.msg')
echo "$CR8AGBOT"

# Register users in the exchange
echo "Creating Anax user in e2edev@somecomp.com org..."
CR8ANAX=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:${EXCH_ROOTPW}" -d '{"password":"anax1pw","email":"me%40gmail.com","admin":false}' "${EXCH_URL}/orgs/e2edev@somecomp.com/users/anax1" | jq -r '.msg')
echo "$CR8ANAX"

echo "Creating Anax user in userdev org..."
CR8UANAX=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:${EXCH_ROOTPW}" -d '{"password":"useranax1pw","email":"me%40gmail.com","admin":false}' "${EXCH_URL}/orgs/userdev/users/useranax1" | jq -r '.msg')
echo "$CR8UANAX"

echo "Registering Anax device1..."
REGANAX1=$(curl -sLX PUT $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic e2edev@somecomp.com/anax1:anax1pw" -d '{"token":"abcdefg","name":"anaxdev","registeredServices":[],"msgEndPoint":"","softwareVersions":{},"publicKey":"","pattern":"","arch":"amd64"}' "${EXCH_URL}/orgs/e2edev@somecomp.com/nodes/an12345" | jq -r '.msg')
echo "$REGANAX1"

echo "Registering Anax device2..."
REGANAX2=$(curl -sLX PUT $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic e2edev@somecomp.com/anax1:anax1pw" -d '{"token":"abcdefg","name":"anaxdev","registeredServices":[],"msgEndPoint":"","softwareVersions":{},"publicKey":"","pattern":"","arch":"amd64"}' "${EXCH_URL}/orgs/e2edev@somecomp.com/nodes/an54321" | jq -r '.msg')
echo "$REGANAX2"

# register an anax devices for userdev in order to test the case where the pattern is from a different org than the device org.
echo "Registering Anax device1 in userdev org..."
REGUANAX1=$(curl -sLX PUT $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic userdev/useranax1:useranax1pw" -d '{"token":"abcdefg","name":"anaxdev","registeredServices":[],"msgEndPoint":"","softwareVersions":{},"publicKey":"","pattern":"","arch":"amd64"}' "${EXCH_URL}/orgs/userdev/nodes/an12345" | jq -r '.msg')
echo "$REGUANAX1"

echo "Registering Anax device2 in userdev org..."
REGUANAX2=$(curl -sLX PUT $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic userdev/useranax1:useranax1pw" -d '{"token":"abcdefg","name":"anaxdev","registeredServices":[],"msgEndPoint":"","softwareVersions":{},"publicKey":"","pattern":"","arch":"amd64"}' "${EXCH_URL}/orgs/userdev/nodes/an54321" | jq -r '.msg')
echo "$REGUANAX2"

echo "Registering Anax device1 in customer org..."
REGANAX1C=$(curl -sLX PUT $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic Customer1/icpadmin:icpadminpw" -d '{"token":"abcdefg","name":"anaxdev","registeredServices":[],"msgEndPoint":"","softwareVersions":{},"publicKey":"","pattern":"","arch":"amd64"}' "${EXCH_URL}/orgs/Customer1/nodes/an12345" | jq -r '.msg')
echo "$REGANAX1C"

# package resources
./resource_package.sh
if [ $? -ne 0 ]
then
  echo -e "Resource registration failure."
  exit -1
fi

# test the CSS API
./sync_service_test.sh
if [ $? -ne 0 ]
then
  echo -e "Model management sync service test failure."
  exit -1
fi

echo "Register services"
./service_apireg.sh
if [ $? -ne 0 ]
then
  echo -e "Service registration failure."
  TESTFAIL="1"
else
  echo "Register services SUCCESSFUL"
fi

TEST_MSGHUB=0
for pat in $(echo $TEST_PATTERNS | tr "," " "); do
  if [ "$pat" == "cpu2msghub" ]; then
    TEST_MSGHUB=1
  fi
done

# Register msghub services and patterns
if [ "$TESTFAIL" != "1" ]; then
  if [ $TEST_MSGHUB -eq 1 ]; then
    echo "Register services and patterns for msghub test"

    ./msghub_rsrcreg.sh
    if [ $? -ne 0 ]
    then
      echo -e "Service registration failure for msghub."
      TESTFAIL="1"
    else
      echo "Register services for msghub SUCCESSFUL"
    fi
  fi
fi

# Setup to use the anax registration APIs
if [ "$TESTFAIL" != "1" ]
then
  export USER=anax1
  export PASS=anax1pw
  export DEVICE_ID="an12345"
  export DEVICE_NAME="an12345"
  export DEVICE_ORG="e2edev@somecomp.com"
  export ANAX_API="http://localhost"
  export EXCH="${EXCH_APP_HOST}"
  export TOKEN="abcdefg"
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
  else
    echo "API tests completed SUCCESSFULLY."

    echo "Killing anax and cleaning up."
    kill $(pidof anax)
    rm -fr /root/.colonus/*.db
    rm -fr /root/.colonus/policy.d/*
  fi
fi

echo -e "No agbot setting is $NOAGBOT"
if [ "$NOAGBOT" != "1" ] && [ "$TESTFAIL" != "1" ]
then

  if [ "${EXCH_APP_HOST}" = "http://exchange-api:8080/v1" ]; then
    AGBOT_AUTH="IBM/agbot1:agbot1pw"
  else
    AGBOT_AUTH="root/root:${EXCH_ROOTPW}"
  fi
  ORG="IBM"
  # Register agreement bot in the exchange

  if [ "${EXCH_APP_HOST}" = "http://exchange-api:8080/v1" ]; then
    echo "Registering Agbot instance1..."
    echo -e "PATTERN setting is $PATTERN"
    REGAGBOT1=$(curl -sLX PUT --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $AGBOT_AUTH" -d '{"token":"abcdefg","name":"agbotdev","msgEndPoint":"","publicKey":""}' "${EXCH_URL}/orgs/$ORG/agbots/${AGBOT_NAME}" | jq -r '.msg')
    echo "$REGAGBOT1"
  fi

  # keep one just for testing this api
  REGAGBOTSNS=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $AGBOT_AUTH" -d '{"patternOrgid":"e2edev@somecomp.com","pattern":"sns"}' "${EXCH_URL}/orgs/$ORG/agbots/${AGBOT_NAME}/patterns" | jq -r '.msg')
  echo "$REGAGBOTSNS"

  # register all patterns and business policies for e2edev@somecomp.com org to agbot1
  REGAGBOTE2EDEV=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $AGBOT_AUTH" -d '{"patternOrgid":"e2edev@somecomp.com","pattern":"*", "nodeOrgid": "e2edev@somecomp.com"}' "${EXCH_URL}/orgs/$ORG/agbots/${AGBOT_NAME}/patterns" | jq -r '.msg')
  echo "$REGAGBOTE2EDEV"

  REGAGBOTE2EDEV=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $AGBOT_AUTH" -d '{"businessPolOrgid":"e2edev@somecomp.com","businessPol":"*", "nodeOrgid": "e2edev@somecomp.com"}' "${EXCH_URL}/orgs/$ORG/agbots/${AGBOT_NAME}/businesspols" | jq -r '.msg')
  echo "$REGAGBOTE2EDEV"

  # register all patterns and business policies for userdev org to agbot1
  REGAGBOTUSERDEV=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $AGBOT_AUTH" -d '{"patternOrgid":"e2edev@somecomp.com","pattern":"*", "nodeOrgid": "userdev"}' "${EXCH_URL}/orgs/$ORG/agbots/${AGBOT_NAME}/patterns" | jq -r '.msg')
  echo "$REGAGBOTUSERDEV"

  REGAGBOTUSERDEV=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $AGBOT_AUTH" -d '{"businessPolOrgid":"userdev","businessPol":"*", "nodeOrgid": "userdev"}' "${EXCH_URL}/orgs/$ORG/agbots/${AGBOT_NAME}/businesspols" | jq -r '.msg')
  echo "$REGAGBOTUSERDEV"

  REGAGBOTSHELM=$(curl -sLX PUT $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $AGBOT_AUTH" -d '{"patternOrgid":"e2edev@somecomp.com","pattern":"shelm"}' "${EXCH_URL}/orgs/$ORG/agbots/${AGBOT_NAME}/patterns/e2edev@somecomp.com_shelm" | jq -r '.msg')
  echo "$REGAGBOTSUH"

  if [ "${EXCH_APP_HOST}" = "http://exchange-api:8080/v1" ]; then
    echo "Registering Agbot instance2..."
    REGAGBOT2=$(curl -sLX PUT --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $AGBOT_AUTH" -d '{"token":"abcdefg","name":"agbotdev","msgEndPoint":"","publicKey":""}' "${EXCH_URL}/orgs/$ORG/agbots/ag54321" | jq -r '.msg')
    echo "$REGAGBOT2"
  fi

  # register msghub patterns to agbot1
  if [ $TEST_MSGHUB -eq 1 ]; then
    REGAGBOTCPU2MSGHUB=$(curl -sLX PUT $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $AGBOT_AUTH" -d '{"patternOrgid":"e2edev@somecomp.com","pattern":"cpu2msghub"}' "${EXCH_URL}/orgs/$ORG/agbots/${AGBOT_NAME}/patterns/e2edev@somecomp.com_cpu2msghub" | jq -r '.msg')
    echo "$REGAGBOTCPU2MSGHUB"
  fi

  if [ "${EXCH_APP_HOST}" = "http://exchange-api:8080/v1" ]; then
    # Start the agbot
    su - agbotuser -c "mkdir -p /home/agbotuser/policy.d"
    su - agbotuser -c "mkdir -p /home/agbotuser/policy2.d"
    su - agbotuser -c "mkdir -p /home/agbotuser/.colonus"
    export HZN_VAR_BASE=/home/agbotuser

    if [ "$OLDAGBOT" == "1" ]
    then
      echo "Starting the OLD Agreement Bot 1."
      su agbotuser -c "/usr/bin/old-anax -v=5 -alsologtostderr=true -config /etc/agbot/agbot.config >/tmp/agbot.log 2>&1 &"
    else
      echo "Starting Agreement Bot 1."
      su agbotuser -c "/usr/local/bin/anax -v=5 -alsologtostderr=true -config /etc/agbot/agbot.config >/tmp/agbot.log 2>&1 &"

      if [ "$MULTIAGBOT" == "1" ]; then
        sleep 5
        echo "Starting Agreement Bot 2."
        su agbotuser -c "/usr/local/bin/anax -v=5 -alsologtostderr=true -config /etc/agbot/agbot2.config >/tmp/agbot2.log 2>&1 &"
      fi
    fi

    sleep 5

    # Check that the agbot is still alive
    if ! curl -sSL http://localhost:8082/agreement > /dev/null; then
      echo "Agreement Bot 1 startup failure."
      TESTFAIL="1"
      exit 1
    fi

    if [ "$MULTIAGBOT" == "1" ]; then
      if ! curl -sSL http://localhost:8083/agreement > /dev/null; then
        echo "Agreement Bot 2 startup failure."
        TESTFAIL="1"
        exit 1
      fi
    fi
  fi
else
  echo -e "Agbot is disabled"
fi

# echo "Starting Agbot API tests"
# ./agbot_upgrade_test.sh
# if [ $? -ne 0 ]
# then
#     echo "Agbot API Test failure."
#     exit -1
# fi
# echo "Agbot API tests completed."

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

elif [ "$TESTFAIL" != "1" ]; then
  # make agreements based on patterns
  last_pattern=$(echo $TEST_PATTERNS |sed -e 's/^.*,//')
  echo -e "Last pattern is $last_pattern"

  for pat in $(echo $TEST_PATTERNS | tr "," " "); do
    export PATTERN=$pat
    echo -e "***************************"
    echo -e "Start testing pattern $PATTERN..."

    # Allocate port 80 to see what anax does
    # socat - TCP4-LISTEN:80,crlf &

    # start pattern test
    set_exports $pat

    ./start_node.sh
    if [ $? -ne 0 ]
    then
      echo "Node start failure."
      TESTFAIL="1"
      break
    fi

    run_delete_loops
    if [ $? -ne 0 ]
    then
      echo "Delete loop failure."
      TESTFAIL="1"
      break
    fi

    ./service_retry_test.sh
    if [ $? -ne 0 ]
    then
      echo "Service retry failure."
      TESTFAIL="1"
      break
    fi

    ./service_configstate_test.sh
    if [ $? -ne 0 ]
    then
      echo "Service configstate test failure."
      TESTFAIL="1"
      break
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

./verify_surfaced_error.sh
if [ $? -ne 0 ]; then echo "Verify surfaced error failure."; exit 1; fi

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

# Clean up remote environment
if [ "${EXCH_APP_HOST}" != "http://exchange-api:8080/v1" ]; then
  echo "Clean up remote environment"
  echo "Delete e2edev@somecomp.com..."
  DL8ORG=$(curl -X DELETE $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:${EXCH_ROOTPW}" -d '{"label":"E2EDev","description":"E2EDevTest","orgType":"IBM"}' "${EXCH_URL}/orgs/e2edev@somecomp.com" | jq -r '.msg')
  echo "$DL8ORG"

  echo "Delete userdev organization..."
  DL8UORG=$(curl -X DELETE $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:${EXCH_ROOTPW}" -d '{"label":"UserDev","description":"UserDevTest"}' "${EXCH_URL}/orgs/userdev" | jq -r '.msg')
  echo "$DL8UORG"

  echo "Delete Customer1 organization..."
  DL8C1ORG=$(curl -X DELETE $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:${EXCH_ROOTPW}" -d '{"label":"Customer1","description":"The Customer1 org"}' "${EXCH_URL}/orgs/Customer1" | jq -r '.msg')
  echo "$DL8C1ORG"

  echo "Delete Customer2 organization..."
  DL8C2ORG=$(curl -X DELETE $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:${EXCH_ROOTPW}" -d '{"label":"Customer2","description":"The Customer2 org"}' "${EXCH_URL}/orgs/Customer2" | jq -r '.msg')
  echo "$DL8C2ORG"

  # Delete an IBM admin user in the exchange
  echo "Delete an admin user for IBM org..."
  DL8IBM=$(curl -X DELETE $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:${EXCH_ROOTPW}" -d '{"password":"ibmadminpw","email":"ibmadmin%40ibm.com","admin":true}' "${EXCH_URL}/orgs/IBM/users/ibmadmin" | jq -r '.msg')
  echo "$DL8IBM"

  # Delete agreement bot user in the exchange
  echo "Delete Agbot user..."
  DL8AGBOT=$(curl -X DELETE $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:${EXCH_ROOTPW}" -d '{"password":"agbot1pw","email":"me%40gmail.com","admin":false}' "${EXCH_URL}/orgs/IBM/users/agbot1" | jq -r '.msg')
  echo "$DL8AGBOT"

  echo "Delete network_1.5.0 user..."
  DL150=$(curl -X DELETE $CERT_VAR "${EXCH_URL}/orgs/IBM/services/bluehorizon.network-services-network_1.5.0_amd64?username=root%2Froot&password=${EXCH_ROOTPW}" -H "accept: application/json")
  echo "$DL150"

  echo "Delete network2_1.5.0 user..."
  DL2150=$(curl -X DELETE $CERT_VAR "${EXCH_URL}/orgs/IBM/services/bluehorizon.network-services-network2_1.5.0_amd64?username=root%2Froot&password=${EXCH_ROOTPW}" -H "accept: application/json")
  echo "$DL2150"

  echo "Delete helm-service_1.0.0 user..."
  DLHELM100=$(curl -X DELETE $CERT_VAR "${EXCH_URL}/orgs/IBM/services/my.company.com-services-helm-service_1.0.0_amd64?username=root%2Froot&password=${EXCH_ROOTPW}" -H "accept: application/json")
  echo "$DLHELM100"
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
