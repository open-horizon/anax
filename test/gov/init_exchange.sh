#!/bin/bash

# bootstrap the exchange

TEST_DIFF_ORG=${TEST_DIFF_ORG:-1}

EXCH_URL="${EXCH_APP_HOST}"

export ARCH=${ARCH}

# the horizon var base for storing the keys. It is the default value for HZN_VAR_BASE.
mkdir -p /var/horizon
mkdir -p /var/horizon/.colonus

docker version

# update host file if needed
if [ "$ICP_HOST_IP" != "0" ]
then
  echo "Updating hosts file."
  HOST_NAME_ICP=`echo $EXCH_URL | awk -F/ '{print $3}' | sed 's/:.*//g'`
  HOST_NAME=`echo $EXCH_URL | awk -F/ '{print $3}' | sed 's/:.*//g' | sed 's/\.icp*//g'`
  echo "$ICP_HOST_IP $HOST_NAME_ICP $HOST_NAME"
  echo "$ICP_HOST_IP $HOST_NAME_ICP $HOST_NAME" >> /etc/hosts
fi

#--cacert /certs/css.crt
if [ ${CERT_LOC} -eq "1" ]; then
  CERT_VAR="--cacert /certs/css.crt"
else
  CERT_VAR=""
fi

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

  sleep 30

# Create the organizations we need
echo "Creating e2edev@somecomp.com organization..."

CR8EORG=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d '{"label":"E2EDev","description":"E2EDevTest","orgType":"IBM"}' "${EXCH_URL}/orgs/e2edev@somecomp.com" | jq -r '.msg')
echo "$CR8EORG"

echo "Creating userdev organization..."
CR8UORG=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d '{"label":"UserDev","description":"UserDevTest"}' "${EXCH_URL}/orgs/userdev" | jq -r '.msg')
echo "$CR8UORG"

echo "Creating Customer1 organization..."
CR8C1ORG=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d '{"label":"Customer1","description":"The Customer1 org"}' "${EXCH_URL}/orgs/Customer1" | jq -r '.msg')
echo "$CR8C1ORG"

echo "Creating Customer2 organization..."
CR8C2ORG=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d '{"label":"Customer2","description":"The Customer2 org"}' "${EXCH_URL}/orgs/Customer2" | jq -r '.msg')
echo "$CR8C2ORG"

# Register a hub admin user in the exchange
echo "Creating a hub admin user in the exchange"
CR8EADM=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d "{\"password\":\"${EXCHANGE_HUB_ADMIN_PW}\",\"email\":\"me%40gmail.com\",\"hubAdmin\":true}" "${EXCH_URL}/orgs/root/users/hubadmin" | jq -r '.msg')
echo "$CR8EADM"

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
CR8IBM=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d "{\"password\":\"${EXCHANGE_SYSTEM_ADMIN_PW}\",\"email\":\"ibmadmin%40ibm.com\",\"admin\":true}" "${EXCH_URL}/orgs/IBM/users/ibmadmin" | jq -r '.msg')
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
REGANAX1=$(curl -sLX PUT $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "e2edev@somecomp.com/anax1:anax1pw" -d '{"token":"abcdefg","name":"anaxdev","registeredServices":[],"msgEndPoint":"","softwareVersions":{},"publicKey":"","pattern":"","arch":"${ARCH}"}' "${EXCH_URL}/orgs/e2edev@somecomp.com/nodes/an12345" | jq -r '.msg')
echo "$REGANAX1"

echo "Registering Anax device2..."
REGANAX2=$(curl -sLX PUT $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "e2edev@somecomp.com/anax1:anax1pw" -d '{"token":"abcdefg","name":"anaxdev","registeredServices":[],"msgEndPoint":"","softwareVersions":{},"publicKey":"","pattern":"","arch":"${ARCH}"}' "${EXCH_URL}/orgs/e2edev@somecomp.com/nodes/an54321" | jq -r '.msg')
echo "$REGANAX2"

# register an anax devices for userdev in order to test the case where the pattern is from a different org than the device org.
echo "Registering Anax device1 in userdev org..."
REGUANAX1=$(curl -sLX PUT $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "userdev/useranax1:useranax1pw" -d '{"token":"abcdefg","name":"anaxdev","registeredServices":[],"msgEndPoint":"","softwareVersions":{},"publicKey":"","pattern":"","arch":"${ARCH}"}' "${EXCH_URL}/orgs/userdev/nodes/an12345" | jq -r '.msg')
echo "$REGUANAX1"

echo "Registering Anax device2 in userdev org..."
REGUANAX2=$(curl -sLX PUT $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "userdev/useranax1:useranax1pw" -d '{"token":"abcdefg","name":"anaxdev","registeredServices":[],"msgEndPoint":"","softwareVersions":{},"publicKey":"","pattern":"","arch":"${ARCH}"}' "${EXCH_URL}/orgs/userdev/nodes/an54321" | jq -r '.msg')
echo "$REGUANAX2"

echo "Registering Anax device1 in customer org..."
REGANAX1C=$(curl -sLX PUT $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "Customer1/icpadmin:icpadminpw" -d '{"token":"abcdefg","name":"anaxdev","registeredServices":[],"msgEndPoint":"","softwareVersions":{},"publicKey":"","pattern":"","arch":"${ARCH}"}' "${EXCH_URL}/orgs/Customer1/nodes/an12345" | jq -r '.msg')
echo "$REGANAX1C"

DEVICE_NUM=6
NUM_AGENTS=$((${MULTIAGENTS}+$DEVICE_NUM))
while [ ${DEVICE_NUM} -lt ${NUM_AGENTS} ]; do
  echo "Registering Anax device${DEVICE_NUM}..."
  REGANAXMUL=$(curl -sLX PUT $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "e2edev@somecomp.com/anax1:anax1pw" -d '{"token":"abcdefg","name":"anaxdev","registeredServices":[],"msgEndPoint":"","softwareVersions":{},"publicKey":"","pattern":"","arch":"${ARCH}"}' "${EXCH_URL}/orgs/e2edev@somecomp.com/nodes/anaxdevice${DEVICE_NUM}" | jq -r '.msg')
  echo "$REGANAXMUL"
  echo "Registering Anax device${DEVICE_NUM} in userdev org..."
  REGUANAXMULU=$(curl -sLX PUT $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "userdev/useranax1:useranax1pw" -d '{"token":"abcdefg","name":"anaxdev","registeredServices":[],"msgEndPoint":"","softwareVersions":{},"publicKey":"","pattern":"","arch":"${ARCH}"}' "${EXCH_URL}/orgs/userdev/nodes/anaxdevice${DEVICE_NUM}" | jq -r '.msg')
  echo "$REGUANAXMULU"
  echo "Registering Anax device${DEVICE_NUM} in customer org..."
  REGANAXMULC=$(curl -sLX PUT $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "Customer1/icpadmin:icpadminpw" -d '{"token":"abcdefg","name":"anaxdev","registeredServices":[],"msgEndPoint":"","softwareVersions":{},"publicKey":"","pattern":"","arch":"${ARCH}"}' "${EXCH_URL}/orgs/Customer1/nodes/anaxdevice${DEVICE_NUM}" | jq -r '.msg')
  echo "$REGANAXMULC"
  let DEVICE_NUM=DEVICE_NUM+1
done


# Register agreement bot in the exchange
if [ "$NOAGBOT" != "1" ] && [ "$TESTFAIL" != "1" ]
then
  AGBOT_AUTH="root/root:${EXCH_ROOTPW}"
  ORG="IBM"

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

  sleep 30
fi


# Create Orgs in CSS
 ./init_sync_service.sh

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
  exit -1
else
  echo "Register services SUCCESSFUL"
fi

# add just one specific pattern for agbot served patterns, just for testing.
if [ "$NOAGBOT" != "1" ] && [ "$TESTFAIL" != "1" ]
then
  AGBOT_AUTH="root/root:${EXCH_ROOTPW}"
  ORG="IBM"
  # keep one just for testing this api
  REGAGBOTSNS=$(curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$AGBOT_AUTH" -d '{"patternOrgid":"e2edev@somecomp.com","pattern":"sns"}' "${EXCH_URL}/orgs/$ORG/agbots/${AGBOT_NAME}/patterns" | jq -r '.msg')
  echo "$REGAGBOTSNS"
fi
