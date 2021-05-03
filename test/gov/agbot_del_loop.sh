#!/bin/bash

# set -x

for (( ; ; ))
do
   if [ "${EXCH_APP_HOST}" = "http://exchange-api:8080/v1" ]; then
     AGID1=$(curl -sS ${AGBOT_API}/agreement | jq -r '.agreements.active[0].current_agreement_id')
     AGID2=$(curl -sS ${AGBOT_API}/agreement | jq -r '.agreements.active[1].current_agreement_id')
     AGID3=$(curl -sS ${AGBOT_API}/agreement | jq -r '.agreements.active[2].current_agreement_id')
     AGID4=$(curl -sS ${AGBOT_API}/agreement | jq -r '.agreements.active[3].current_agreement_id')

     echo "Agbot deleting agreements"
     echo "Deleting $AGID1"
     DEL=$(curl -sS -X DELETE ${AGBOT_API}/agreement/$AGID1)

     echo "Deleting $AGID2"
     DEL=$(curl -sS -X DELETE ${AGBOT_API}/agreement/$AGID2)

     echo "Deleting $AGID3"
     DEL=$(curl -sS -X DELETE ${AGBOT_API}/agreement/$AGID3)

     echo "Deleting $AGID4"
     DEL=$(curl -sS -X DELETE ${AGBOT_API}/agreement/$AGID4)
   else
     AGID1=$(kubectl -n kube-system exec -ti $(kubectl -n kube-system get pods | grep -v 'agbot-db' | grep -m 1 'edge-computing-agbot' |  awk '{print $1}') -- sh -c "export ANAX_PORT=\$(cat /etc/horizon/anax.json | jq -r .AgreementBot.APIListen | awk -F: '{print \$NF}'); curl http://localhost:\$ANAX_PORT/agreement | jq -r '.agreements.active[0].current_agreement_id'")
     AGID2=$(kubectl -n kube-system exec -ti $(kubectl -n kube-system get pods | grep -v 'agbot-db' | grep -m 1 'edge-computing-agbot' |  awk '{print $1}') -- sh -c "export ANAX_PORT=\$(cat /etc/horizon/anax.json | jq -r .AgreementBot.APIListen | awk -F: '{print \$NF}'); curl http://localhost:\$ANAX_PORT/agreement | jq -r '.agreements.active[1].current_agreement_id'")
     AGID3=$(kubectl -n kube-system exec -ti $(kubectl -n kube-system get pods | grep -v 'agbot-db' | grep -m 1 'edge-computing-agbot' |  awk '{print $1}') -- sh -c "export ANAX_PORT=\$(cat /etc/horizon/anax.json | jq -r .AgreementBot.APIListen | awk -F: '{print \$NF}'); curl http://localhost:\$ANAX_PORT/agreement | jq -r '.agreements.active[2].current_agreement_id'")
     AGID4=$(kubectl -n kube-system exec -ti $(kubectl -n kube-system get pods | grep -v 'agbot-db' | grep -m 1 'edge-computing-agbot' |  awk '{print $1}') -- sh -c "export ANAX_PORT=\$(cat /etc/horizon/anax.json | jq -r .AgreementBot.APIListen | awk -F: '{print \$NF}'); curl http://localhost:\$ANAX_PORT/agreement | jq -r '.agreements.active[3].current_agreement_id'")

     echo "Agbot deleting agreements"
     echo "Deleting $AGID1"
     DEL=$(kubectl -n kube-system exec -ti $(kubectl -n kube-system get pods | grep -v 'agbot-db' | grep -m 1 'edge-computing-agbot' |  awk '{print $1}') -- sh -c "export ANAX_PORT=\$(cat /etc/horizon/anax.json | jq -r .AgreementBot.APIListen | awk -F: '{print \$NF}'); curl DELETE http://localhost:\$ANAX_PORT/agreement/$AGID1")

     echo "Deleting $AGID2"
     DEL=$(kubectl -n kube-system exec -ti $(kubectl -n kube-system get pods | grep -v 'agbot-db' | grep -m 1 'edge-computing-agbot' |  awk '{print $1}') -- sh -c "export ANAX_PORT=\$(cat /etc/horizon/anax.json | jq -r .AgreementBot.APIListen | awk -F: '{print \$NF}'); curl DELETE http://localhost:\$ANAX_PORT/agreement/$AGID2")

     echo "Deleting $AGID3"
     DEL=$(kubectl -n kube-system exec -ti $(kubectl -n kube-system get pods | grep -v 'agbot-db' | grep -m 1 'edge-computing-agbot' |  awk '{print $1}') -- sh -c "export ANAX_PORT=\$(cat /etc/horizon/anax.json | jq -r .AgreementBot.APIListen | awk -F: '{print \$NF}'); curl DELETE http://localhost:\$ANAX_PORT/agreement/$AGID3")

     echo "Deleting $AGID4"
     DEL=$(kubectl -n kube-system exec -ti $(kubectl -n kube-system get pods | grep -v 'agbot-db' | grep -m 1 'edge-computing-agbot' |  awk '{print $1}') -- sh -c "export ANAX_PORT=\$(cat /etc/horizon/anax.json | jq -r .AgreementBot.APIListen | awk -F: '{print \$NF}'); curl DELETE http://localhost:\$ANAX_PORT/agreement/$AGID4")
   fi

   if [ "$NOLOOP" == "1" ]; then
      echo -e "Sleeping for 30s to allow cancelled agreements to flush"
      sleep 30
      exit 0
   else
      echo -e "Sleeping now\n"

      sleep 180

      echo -e "Current workload usages\n"
      curl -sS ${AGBOT_API}/workloadusage | jq -r '.'
      sleep 420
   fi
done
