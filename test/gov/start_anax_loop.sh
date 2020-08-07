#!/bin/bash

export HZN_AGENT_PORT=8510

while (true) do
  if [ "$OLDANAX" == "1" ]
  then
      echo "Starting OLD Anax1 to run workloads."
      if [ ${CERT_LOC} -eq "1" ]; then
        /usr/bin/old-anax -v=3 -alsologtostderr=true -config /etc/colonus/anax-combined.config >/tmp/anax.log 2>&1 > /dev/null
      else
        /usr/bin/old-anax -v=3 -alsologtostderr=true -config /etc/colonus/anax-combined-no-cert.config >/tmp/anax.log 2>&1  > /dev/null
      fi
  else
      echo "Starting Anax1 to run workloads."
      if [ ${CERT_LOC} -eq "1" ]; then
	/usr/local/bin/anax -v=3 -alsologtostderr=true -config /etc/colonus/anax-combined.config >>/tmp/anax.log 2>&1
      else
	/usr/local/bin/anax -v=3 -alsologtostderr=true -config /etc/colonus/anax-combined-no-cert.config >>/tmp/anax.log 2>&1
      fi
  fi

   rc=$?
   echo "anax exited with exit code $rc"
done
