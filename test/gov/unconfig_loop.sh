#!/bin/bash

#set -x

# The purpose of this test is to verify that the DELETE /node API works correctly in a full
# runtime context. Some parts of this test simulate the fact that anax is configured to auto-restart
# when it terminates.

EXCH_URL="${EXCH_APP_HOST}"

for (( ; ; ))
do

   # The node is already running, so start with the blocking form of the unconfig API. The API should always be
   # successful and should always be empty.
   echo "Unconfig node, blocking"
   DEL=$(curl -sSLX DELETE http://localhost/node)
   if [ $? -ne 0 ]
   then
      echo -e "Error return from DELETE: $?"
      exit 2
   fi
   if [ "$DEL" != "" ]
   then
      echo -e "Non-empty response to DELETE node: $DEL"
      exit 2
   fi

   # Following the API call, the node's entry in the exchange should have some changes in it. The messaging key should be empty,
   # and the list of registered microservices should be empty.
   if [ ${CERT_LOC} -eq "1" ] && [ "${EXCH_APP_HOST}" != "http://exchange-api:8080/v1" ]; then
     NST=$(curl -sSL --cacert /certs/css.crt --header 'Accept: application/json' -H "Authorization:Basic e2edev@somecomp.com/e2edevadmin:e2edevadminpw" "${EXCH_URL}/orgs/$DEVICE_ORG/nodes/an12345" | jq -r '.')
   else
     NST=$(curl -sSL --header 'Accept: application/json' -H "Authorization:Basic e2edev@somecomp.com/e2edevadmin:e2edevadminpw" "${EXCH_URL}/orgs/$DEVICE_ORG/nodes/an12345" | jq -r '.')
   fi
   PK=$(echo "$NST" | jq -r '.publicKey')
   if [ "$PK" != "null" ]
   then
      echo -e "publicKey should be empty: $PK"
      exit 2
   fi

   RM=$(echo "$NST" | jq -r '.registeredServices[0]')
   if [ "$RM" != "null" ]
   then
      echo -e "registeredServices should be empty: $RM"
      exit 2
   fi

   # This part of the test is to ensure that anax actually terminates. We will give anax 2 mins to terminate which should be
   # much more time than it needs. Normal behavior should be termination in seconds.
   echo -e "Making sure old anax has ended."
   COUNT=1
   while :
   do
      # Wait for the "connection refused" message
      GET=$(curl -sSL 'http://localhost/node')
      if [ $? -eq 7 ]; then
         break
      else
         echo -e "Is anax still up: $GET"

         # Since anax is still up, verify that a POST to /node will return the correct error.
         pat=$PATTERN
         if [[ "$PATTERN" != "" ]]; then
            pat="e2edev@somecomp.com/$PATTERN"
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

         # We only want to look at the response if it's a json document. Everything else we can ignore because anax could have terminated
         # between the GET call above and this POST call.
         if [ $? -eq 0 ] && [ "$HDS" != "null" ] && [ "${HDS:0:1}" == "{" ]
         then
            ERR=$(echo $HDS | jq -r '.error')
            if [ "${ERR:0:19}" != "Node is restarting," ]
            then
               echo -e "node object has the wrong state: $HDS"
               exit 2
            fi
         fi

      fi

      # This is the loop/timeout control. Exit the test in error after 2 mins without an anax termination.
      if [ "$COUNT" == "24" ]
      then
         echo -e "Error, anax is taking too long to terminate."
         exit 2
      fi
      sleep 5
      COUNT=COUNT+1
   done

   # Save off the existing log file, in case the next test fails and we need to look back to see how this
   # instance of anax actually ended.
   mv /tmp/anax.log /tmp/anax1.log

   # Simulate the auto-restart of anax and reconfig of the node.
   echo "Node unconfigured. Restart and reconfig node."
   ./start_node.sh > /tmp/start_node.log
   if [ $? -ne 0 ]
   then
      exit 2
   fi

   # Wait a random length of time before issuing the DELETE /node again. This is to try to catch
   # anax in a state where it doesnt handle the shutdown correctly.
   t=$((1+ RANDOM % 90))
   echo -e "Sleeping $t now to make agreements"
   sleep $t

   # Log the current state of agreements and previous agreements before we unconfigure again.
   echo -e "Current agreements"
   ACT=$(curl -sSL http://localhost/agreement | jq -r '.agreements.active' | grep "current_agreement_id")
   echo $ACT

   echo -e "Previous terminations"
   ARC=$(curl -sSL http://localhost/agreement | jq -r '.agreements.archived' | grep "terminated_description" | awk '{print $0,"\n"}')
   echo $ARC

   # =======================================================================================================================
   # This is phase 2 of the main test loop. The node is already running, so this time use the non-blocking form of the
   # unconfig API. This form requires that we poll GET /node to figure out when unconfiguration is complete.
   echo "Unconfig node, non-blocking"
   DEL=$(curl -sSLX DELETE 'http://localhost/node?block=false')
   if [ $? -ne 0 ]
   then
      echo -e "Error return from DELETE: $?"
      exit 2
   fi
   if [ "$DEL" != "" ]
   then
      echo -e "Non-empty response to DELETE node: $DEL"
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
            pat="e2edev@somecomp.com/$PATTERN"
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

         # We only want to look at the response if it's a json document. Everything else we can ignore because anax could have terminated
         # between the GET call above and this POST call.
         if [ $? -eq 0 ] && [ "$HDS" != "null" ] && [ "${HDS:0:1}" == "{" ]
         then
            ERR=$(echo $HDS | jq -r '.error')
            if [ "${ERR:0:19}" != "Node is restarting," ]
            then
               echo -e "node object has the wrong state: $HDS"
               exit 2
            fi
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
   if [ ${CERT_LOC} -eq "1" ] && [ "${EXCH_APP_HOST}" != "http://exchange-api:8080/v1" ]; then
     NST=$(curl -sSL --cacert /certs/css.crt --header 'Accept: application/json' -H "Authorization:Basic e2edev@somecomp.com/e2edevadmin:e2edevadminpw" "${EXCH_URL}/orgs/$DEVICE_ORG/nodes/an12345" | jq -r '.')
   else
     NST=$(curl -sSL --header 'Accept: application/json' -H "Authorization:Basic e2edev@somecomp.com/e2edevadmin:e2edevadminpw" "${EXCH_URL}/orgs/$DEVICE_ORG/nodes/an12345" | jq -r '.')
   fi
   PK=$(echo "$NST" | jq -r '.publicKey')
   if [ "$PK" != "null" ]
   then
      echo -e "publicKey should be empty: $PK"
      exit 2
   fi

   RM=$(echo "$NST" | jq -r '.registeredServices[0]')
   if [ "$RM" != "null" ]
   then
      echo -e "registeredServices should be empty: $RM"
      exit 2
   fi

   # Save off the existing log file, in case the next test fails and we need to look back to see how this
   # instance of anax actually ended.
   mv /tmp/anax.log /tmp/anax1.log

   # Simulate the auto-restart of anax and reconfig of the node.
   echo "Node unconfigured. Restart and reconfig node."
   ./start_node.sh > /tmp/start_node.log
   if [ $? -ne 0 ]
   then
      exit 2
   fi

   # Wait a random length of time before issuing the DELETE /node again. This is to try to catch
   # anax in a state where it doesnt handle the shutdown correctly.
   t=$((1+ RANDOM % 90))
   echo -e "Sleeping $t now to make agreements"
   sleep $t

   # Log the current state of agreements and previous agreements before we unconfigure again.
   echo -e "Current agreements"
   ACT=$(curl -sSL http://localhost/agreement | jq -r '.agreements.active' | grep "current_agreement_id")
   echo $ACT

   echo -e "Previous terminations"
   ARC=$(curl -sSL http://localhost/agreement | jq -r '.agreements.archived' | grep "terminated_description" | awk '{print $0,"\n"}')
   echo $ARC

done
