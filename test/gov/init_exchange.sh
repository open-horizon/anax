#!/bin/bash

# bootstrap the exchange

TEST_DIFF_ORG=${TEST_DIFF_ORG:-1}

EXCH_URL="${EXCH_APP_HOST}"

# the horizon var base for storing the keys. It is the default value for HZN_VAR_BASE.
mkdir -p /var/horizon
mkdir -p /var/horizon/.colonus

#--cacert /certs/css.crt
if [ ${CERT_LOC} -eq "1" ]; then
  CERT_VAR="--cacert /certs/css.crt"
else
  CERT_VAR=""
fi

if [ "${EXCH_APP_HOST}" = "http://exchange-api:8080/v1" ]; then
  # Clean up the exchange DB to make sure we start out clean
  echo "Drop and recreate the exchange DB."

  # loop until DBTOK contains a string value
  while :
  do
    DBTOK=$(curl -sLX GET -u "root/root:${EXCH_ROOTPW}" "${EXCH_URL}/admin/dropdb/token" | jq -r '.token')
    if test -z "$DBTOK"
    then
      sleep 5
    else
      break
    fi
  done

  DROPDB=$(curl -sLX POST -u "root/root:"$DBTOK "${EXCH_URL}/admin/dropdb" | jq -r '.msg')
  echo "Exchange DB Drop Response: $DROPDB"

  INITDB=$(curl -sLX POST -u "root/root:${EXCH_ROOTPW}" "${EXCH_URL}/admin/initdb" | jq -r '.msg')
  echo "Exchange DB Init Response: $INITDB"

  cd /root

else

  cd /root

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
  DLHELM100=$(curl -X DELETE $CERT_VAR  --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" "${EXCH_URL}/orgs/IBM/services/bluehorizon.network-services-network_1.5.0_amd64")
  echo "$DL150"

  echo "Delete network2_1.5.0 ..."
  DLHELM100=$(curl -X DELETE $CERT_VAR  --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" "${EXCH_URL}/orgs/IBM/services/bluehorizon.network-services-network2_1.5.0_amd64")
  echo "$DL2150"

  echo "Delete helm-service_1.0.0 ..."
  DLHELM100=$(curl -X DELETE $CERT_VAR  --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" "${EXCH_URL}/orgs/IBM/services/my.company.com-services-helm-service_1.0.0_amd64")
  echo "$DLHELM100"

  sleep 30
fi

# Create the organizations we need
echo "Creating e2edev@somecomp.com organization..."

CR8EORG=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d '{"label":"E2EDev","description":"E2EDevTest","orgType":"IBM"}' "${EXCH_URL}/orgs/e2edev@somecomp.com" | jq -r '.msg')
echo "$CR8EORG"

echo "Creating userdev organization..."
CR8UORG=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d '{"label":"UserDev","description":"UserDevTest"}' "${EXCH_URL}/orgs/userdev" | jq -r '.msg')
echo "$CR8UORG"

if [ "${EXCH_APP_HOST}" = "http://exchange-api:8080/v1" ]; then
  echo "Creating IBM organization..."
  CR8IORG=$(curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d '{"label":"IBMorg","description":"IBM"}' "${EXCH_URL}/orgs/IBM" | jq -r '.msg')
  echo "$CR8IORG"
fi

echo "Creating Customer1 organization..."
CR8C1ORG=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d '{"label":"Customer1","description":"The Customer1 org"}' "${EXCH_URL}/orgs/Customer1" | jq -r '.msg')
echo "$CR8C1ORG"

echo "Creating Customer2 organization..."
CR8C2ORG=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d '{"label":"Customer2","description":"The Customer2 org"}' "${EXCH_URL}/orgs/Customer2" | jq -r '.msg')
echo "$CR8C2ORG"

# Register an e2edev@somecomp.com admin user in the exchange
echo "Creating an admin user for e2edev@somecomp.com organization..."
CR8EADM=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d '{"password":"e2edevadminpw","email":"me%40gmail.com","admin":true}' "${EXCH_URL}/orgs/e2edev@somecomp.com/users/e2edevadmin" | jq -r '.msg')
echo "$CR8EADM"

# Register an userdev admin user in the exchange
echo "Creating an admin user for userdev organization..."
CR8UADM=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d '{"password":"userdevadminpw","email":"me%40gmail.com","admin":true}' "${EXCH_URL}/orgs/userdev/users/userdevadmin" | jq -r '.msg')
echo "$CR8UADM"

# Register an ICP user in the customer1 org
echo "Creating an ICP admin user for Customer1 organization..."
CR81ICPADM=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d '{"password":"icpadminpw","email":"me%40gmail.com","admin":true}' "${EXCH_URL}/orgs/Customer1/users/icpadmin" | jq -r '.msg')
echo "$CR81ICPADM"

# Register an ICP user in the customer2 org
echo "Creating an ICP admin user for Customer2 organization..."
CR82ICPADM=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d '{"password":"icpadminpw","email":"me%40gmail.com","admin":true}' "${EXCH_URL}/orgs/Customer2/users/icpadmin" | jq -r '.msg')
echo "$CR82ICPADM"

# Register an IBM admin user in the exchange
echo "Creating an admin user for IBM org..."
CR8IBM=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d '{"password":"ibmadminpw","email":"ibmadmin%40ibm.com","admin":true}' "${EXCH_URL}/orgs/IBM/users/ibmadmin" | jq -r '.msg')
echo "$CR8IBM"

# Register agreement bot user in the exchange
echo "Creating Agbot user..."
CR8AGBOT=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d '{"password":"agbot1pw","email":"me%40gmail.com","admin":false}' "${EXCH_URL}/orgs/IBM/users/agbot1" | jq -r '.msg')
echo "$CR8AGBOT"

# Register users in the exchange
echo "Creating Anax user in e2edev@somecomp.com org..."
CR8ANAX=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d '{"password":"anax1pw","email":"me%40gmail.com","admin":false}' "${EXCH_URL}/orgs/e2edev@somecomp.com/users/anax1" | jq -r '.msg')
echo "$CR8ANAX"

echo "Creating Anax user in userdev org..."
CR8UANAX=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d '{"password":"useranax1pw","email":"me%40gmail.com","admin":false}' "${EXCH_URL}/orgs/userdev/users/useranax1" | jq -r '.msg')
echo "$CR8UANAX"

echo "Registering Anax device1..."
REGANAX1=$(curl -sLX PUT $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "e2edev@somecomp.com/anax1:anax1pw" -d '{"token":"abcdefg","name":"anaxdev","registeredServices":[],"msgEndPoint":"","softwareVersions":{},"publicKey":"","pattern":"","arch":"amd64"}' "${EXCH_URL}/orgs/e2edev@somecomp.com/nodes/an12345" | jq -r '.msg')
echo "$REGANAX1"

echo "Registering Anax device2..."
REGANAX2=$(curl -sLX PUT $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "e2edev@somecomp.com/anax1:anax1pw" -d '{"token":"abcdefg","name":"anaxdev","registeredServices":[],"msgEndPoint":"","softwareVersions":{},"publicKey":"","pattern":"","arch":"amd64"}' "${EXCH_URL}/orgs/e2edev@somecomp.com/nodes/an54321" | jq -r '.msg')
echo "$REGANAX2"

# register an anax devices for userdev in order to test the case where the pattern is from a different org than the device org.
echo "Registering Anax device1 in userdev org..."
REGUANAX1=$(curl -sLX PUT $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "userdev/useranax1:useranax1pw" -d '{"token":"abcdefg","name":"anaxdev","registeredServices":[],"msgEndPoint":"","softwareVersions":{},"publicKey":"","pattern":"","arch":"amd64"}' "${EXCH_URL}/orgs/userdev/nodes/an12345" | jq -r '.msg')
echo "$REGUANAX1"

echo "Registering Anax device2 in userdev org..."
REGUANAX2=$(curl -sLX PUT $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "userdev/useranax1:useranax1pw" -d '{"token":"abcdefg","name":"anaxdev","registeredServices":[],"msgEndPoint":"","softwareVersions":{},"publicKey":"","pattern":"","arch":"amd64"}' "${EXCH_URL}/orgs/userdev/nodes/an54321" | jq -r '.msg')
echo "$REGUANAX2"

echo "Registering Anax device1 in customer org..."
REGANAX1C=$(curl -sLX PUT $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "Customer1/icpadmin:icpadminpw" -d '{"token":"abcdefg","name":"anaxdev","registeredServices":[],"msgEndPoint":"","softwareVersions":{},"publicKey":"","pattern":"","arch":"amd64"}' "${EXCH_URL}/orgs/Customer1/nodes/an12345" | jq -r '.msg')
echo "$REGANAX1C"

# Clean up CSS
if [ "${EXCH_APP_HOST}" != "http://exchange-api:8080/v1" ]; then
  ./clean_css.sh
fi

# package resources
./resource_package.sh
if [ $? -ne 0 ]
then
  echo -e "Resource registration failure."
  exit -1
fi

# Start the API Key tests if it has been set
if [ ${API_KEY} != "0" ]; then
  echo -e "Starting API Key test."
  ./api_key.sh
  if [ $? -ne 0 ]
  then
    echo -e "API Key test failure."
    exit -1
  fi
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

# Register agreement bot in the exchange
if [ "$NOAGBOT" != "1" ] && [ "$TESTFAIL" != "1" ]
then
  if [ "${EXCH_APP_HOST}" = "http://exchange-api:8080/v1" ]; then
    AGBOT_AUTH="IBM/agbot1:agbot1pw"
  else
    AGBOT_AUTH="root/root:${EXCH_ROOTPW}"
  fi
  ORG="IBM"

  if [ "${EXCH_APP_HOST}" = "http://exchange-api:8080/v1" ]; then
    echo "Registering Agbot instance1..."
    echo -e "PATTERN setting is $PATTERN"
    REGAGBOT1=$(curl -sLX PUT --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$AGBOT_AUTH" -d '{"token":"abcdefg","name":"agbotdev","msgEndPoint":"","publicKey":""}' "${EXCH_URL}/orgs/$ORG/agbots/${AGBOT_NAME}" | jq -r '.msg')
    echo "$REGAGBOT1"
  fi

  # keep one just for testing this api
  REGAGBOTSNS=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$AGBOT_AUTH" -d '{"patternOrgid":"e2edev@somecomp.com","pattern":"sns"}' "${EXCH_URL}/orgs/$ORG/agbots/${AGBOT_NAME}/patterns" | jq -r '.msg')
  echo "$REGAGBOTSNS"

  # register all patterns and business policies for e2edev@somecomp.com org to agbot1
  REGAGBOTE2EDEV=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$AGBOT_AUTH" -d '{"patternOrgid":"e2edev@somecomp.com","pattern":"*", "nodeOrgid": "e2edev@somecomp.com"}' "${EXCH_URL}/orgs/$ORG/agbots/${AGBOT_NAME}/patterns" | jq -r '.msg')
  echo "$REGAGBOTE2EDEV"

  REGAGBOTE2EDEV=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$AGBOT_AUTH" -d '{"businessPolOrgid":"e2edev@somecomp.com","businessPol":"*", "nodeOrgid": "e2edev@somecomp.com"}' "${EXCH_URL}/orgs/$ORG/agbots/${AGBOT_NAME}/businesspols" | jq -r '.msg')
  echo "$REGAGBOTE2EDEV"

  # register all patterns and business policies for userdev org to agbot1
  REGAGBOTUSERDEV=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$AGBOT_AUTH" -d '{"patternOrgid":"e2edev@somecomp.com","pattern":"*", "nodeOrgid": "userdev"}' "${EXCH_URL}/orgs/$ORG/agbots/${AGBOT_NAME}/patterns" | jq -r '.msg')
  echo "$REGAGBOTUSERDEV"

  REGAGBOTUSERDEV=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$AGBOT_AUTH" -d '{"businessPolOrgid":"userdev","businessPol":"*", "nodeOrgid": "userdev"}' "${EXCH_URL}/orgs/$ORG/agbots/${AGBOT_NAME}/businesspols" | jq -r '.msg')
  echo "$REGAGBOTUSERDEV"

  # REGAGBOTSHELM=$(curl -sLX PUT $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$AGBOT_AUTH" -d '{"patternOrgid":"e2edev@somecomp.com","pattern":"shelm"}' "${EXCH_URL}/orgs/$ORG/agbots/${AGBOT_NAME}/patterns/e2edev@somecomp.com_shelm" | jq -r '.msg')
  # echo "$REGAGBOTSUH"

  if [ "${EXCH_APP_HOST}" = "http://exchange-api:8080/v1" ]; then
    echo "Registering Agbot instance2..."
    REGAGBOT2=$(curl -sLX PUT --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$AGBOT_AUTH" -d '{"token":"abcdefg","name":"agbotdev","msgEndPoint":"","publicKey":""}' "${EXCH_URL}/orgs/$ORG/agbots/ag54321" | jq -r '.msg')
    echo "$REGAGBOT2"
  fi

  # register msghub patterns to agbot1
  if [ "${TEST_MSGHUB}" = "1" ]; then
    REGAGBOTCPU2MSGHUB=$(curl -sLX PUT $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$AGBOT_AUTH" -d '{"patternOrgid":"e2edev@somecomp.com","pattern":"cpu2msghub"}' "${EXCH_URL}/orgs/$ORG/agbots/${AGBOT_NAME}/patterns/e2edev@somecomp.com_cpu2msghub" | jq -r '.msg')
    echo "$REGAGBOTCPU2MSGHUB"
  fi
fi
