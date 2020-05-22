#!/bin/bash

# Check agbot agreements, looking for k8s agreements.
AGSR=$(curl -sSL http://localhost:8082/agreement | jq -r '.agreements.archived')
NUM_AGS=$(echo ${AGSR} | jq -r '. | length')
if [ "${NUM_AGS}" != "0" ]; then
  echo -e "Looking for kube service in archived agreements: ${NUM_AGS}"
  ECAG=$(echo $AGSR | jq -r '.[] | select(.pattern=="e2edev@somecomp.com/sk8s") | .current_agreement_id')
  ECAGT=$(echo $AGSR | jq -r '.[] | select(.pattern=="e2edev@somecomp.com/sk8s") | .terminated_description')
  if [ "${ECAG}" == "" ]; then
    echo -e "No terminated agreements found for the edge cluster node, there should be an active agreement."
  else
    echo -e "Found agreement(s) ${ECAG} terminated because ${ECAGT}, so agreements are being made with the edge cluster node."
    exit 0
  fi
fi

# Since there are no archived agreements, we need to wait for an active agreement to appear.
LOOPCOUNT=0
while [ ${LOOPCOUNT} -le 10 ]
do
  AGSA=$(curl -sSL http://localhost:8082/agreement | jq -r '.agreements.active')
  NUM_AGS=$(echo ${AGSA} | jq -r '. | length')
  if [ "${NUM_AGS}" != "0" ]; then
    echo -e "Looking for kube service in active agreements: ${NUM_AGS}"
    ECAG=$(echo $AGSA | jq -r '.[] | select(.pattern=="e2edev@somecomp.com/sk8s") | .current_agreement_id')
    if [ "${ECAG}" == "" ]; then
        echo -e "Edge Cluster workload should be present but is not, waiting for it to appear."
        sleep 10
        let LOOPCOUNT+=1
    else
        echo "Edge cluster agreement ${ECAG} found"
        exit 0
    fi
  else
    echo -e "No active agreements, but there should be at least one."
    sleep 10
    let LOOPCOUNT+=1
  fi
done

echo "Edge cluster agreement did not appear"
exit 1
