#!/bin/bash

#set -x

# The purpose of this test is to verify that the DELETE /node API works correctly in a full
# runtime context. Some parts of this test simulate the fact that anax is configured to auto-restart
# when it terminates.

EXCH_URL="http://${EXCH_APP_HOST:-172.17.0.1}:8080/v1"
E2EDEV_ADMIN_AUTH="e2edev/e2edevadmin:e2edevadminpw"
export HZN_EXCHANGE_URL="http://${EXCH_APP_HOST:-172.17.0.1}:8080/v1"

echo "Unregister node, non-blocking"
hzn unregister -f

if [ $? -ne 0 ]
then
   echo -e "Error unregistering the node."
   exit 2
fi

# Start polling for unconfig completion. Unconfig could take several minutes if we are running this test with a blockchain
# configuration.
echo -e "Polling anax API for completion of device unconfigure."
COUNT=1
while :
do
   GET=$(curl -sSL 'http://localhost/node')
   if [ $? -eq 7 ]; then
      break
   else
      echo -e "Is anax still up: $GET"

      # Since anax is still up, verify that a POST to /node will return the correct error.
      pat=$PATTERN
      if [[ "$PATTERN" != "" ]]; then
        pat="e2edev/$PATTERN"
      fi
read -d '' newhzndevice <<EOF
{
  "id": "$DEVICE_ID",
  "token": "$TOKEN",
  "name": "$DEVICE_NAME",
  "organization": "$DEVICE_ORG",
  "pattern": "$pat"
}
EOF
      HDS=$(echo "$newhzndevice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost/node")

      rc=$?

      # We only want to look at the response if it's a json document. Everything else we can ignore because anax could have terminated
      # between the GET call above and this POST call.
      if [ $rc -eq 0 ] && [ "$HDS" != "null" ] && [ "${HDS:0:1}" == "{" ]
      then
         ERR=$(echo $HDS | jq -r '.error')
         if [ "${ERR:0:19}" != "Node is restarting," ]
         then
            echo -e "node object has the wrong state: $HDS"
            exit 2
         fi
      elif [ $rc -eq 7 ]; then
        echo -e "Anax is down."
        break
      fi
   fi

   # This is the loop/timeout control. Exit the test in error after 4 mins without an anax termination.
   if [ "$COUNT" == "48" ]
   then
      echo -e "Error, anax is taking too long to terminate."
      exit 2
   fi
   sleep 5
   COUNT=COUNT+1
done

# Following the API call, the node's entry in the exchange should have some changes in it. The messaging key should be empty,
# and the list of registered microservices should be empty.
echo -e "Checking node status in the exchange."
NST=$(curl -sSL --header 'Accept: application/json' -H "Authorization:Basic e2edev/e2edevadmin:e2edevadminpw" "${EXCH_URL}/orgs/e2edev/nodes/an12345" | jq -r '.')
PK=$(echo "$NST" | jq -r '.publicKey')
if [ "$PK" != "null" ]
then
   echo -e "publicKey should be empty: $PK"
   exit 2
fi

RM=$(echo "$NST" | jq -r '.registeredMicroservices[0]')
if [ "$RM" != "null" ]
then
   echo -e "registeredMicroservices should be empty: $RM"
   exit 2
fi


