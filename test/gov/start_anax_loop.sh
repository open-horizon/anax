#!/bin/bash

num=$1

export HZN_AGENT_PORT=$((8509 + ${num}))
 
if [[ "$num" == "1" ]]; then
  num=""
fi

while (true) do
  if [ "$OLDANAX" == "1" ]
  then
      echo "Starting OLD Anax to run workloads."
      if [ ${CERT_LOC} -eq "1" ]; then
        /usr/bin/old-anax -v=5 -alsologtostderr=true -config /etc/colonus/anax-combined${num}.config >/tmp/anax${num}.log 2>&1 > /dev/null
      else
        /usr/bin/old-anax -v=5 -alsologtostderr=true -config /etc/colonus/anax-combined${num}-no-cert.config >/tmp/anax${num}.log 2>&1  > /dev/null
      fi
  else
      echo "Starting Anax to run workloads."
      if [ ${CERT_LOC} -eq "1" ]; then
	     /usr/local/bin/anax -v=5 -alsologtostderr=true -config /etc/colonus/anax-combined${num}.config >>/tmp/anax${num}.log 2>&1
      else
	     /usr/local/bin/anax -v=5 -alsologtostderr=true -config /etc/colonus/anax-combined${num}-no-cert.config >>/tmp/anax${num}.log 2>&1
      fi
  fi

   rc=$?
   echo "${anax} exited with exit code $rc"
done
