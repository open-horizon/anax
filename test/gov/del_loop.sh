#!/bin/bash

# set -x

for (( ; ; ))
do

   AGID1=$(curl -sS $ANAX_API/agreement | jq -r '.agreements.active[0].current_agreement_id')
   AGID2=$(curl -sS $ANAX_API/agreement | jq -r '.agreements.active[1].current_agreement_id')
   AGID3=$(curl -sS $ANAX_API/agreement | jq -r '.agreements.active[2].current_agreement_id')
   AGID4=$(curl -sS $ANAX_API/agreement | jq -r '.agreements.active[3].current_agreement_id')

   echo "Device deleting agreements"
   echo "Deleting $AGID1"
   DEL=$(curl -sS -X DELETE $ANAX_API/agreement/$AGID1)

   echo "Deleting $AGID2"
   DEL=$(curl -sS -X DELETE $ANAX_API/agreement/$AGID2)

   echo "Deleting $AGID3"
   DEL=$(curl -sS -X DELETE $ANAX_API/agreement/$AGID3)

   echo "Deleting $AGID4"
   DEL=$(curl -sS -X DELETE $ANAX_API/agreement/$AGID4)

   if [ "$NOLOOP" == "1" ]; then
     exit 0
   else
      echo -e "Sleeping now\n"
      sleep 600
   fi
done
