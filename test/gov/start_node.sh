#!/bin/bash

# Please set the following env variable before calling this script.
# For example:
# export USER=anax1
# export PASS=anax1pw
# export DEVICE_ID="an12345"
# export DEVICE_ORG="e2edev@somecomp.com"
# export DEVICE_NAME="anaxdev1"
# export ANAX_API="http://localhost"
# export EXCH="${EXCH_APP_HOST}"
# export TOKEN="abcdefg"
# This env var can be changed to whatever pattern you want to run.
# export PATTERN="sall"

if [ "$OLDANAX" == "1" ]
then
    echo "Starting OLD Anax1 to run workloads."
    if [ ${CERT_LOC} -eq "1" ]; then
      /usr/bin/old-anax -v=5 -alsologtostderr=true -config /etc/colonus/anax-combined.config >/tmp/anax.log 2>&1 &
    else
      /usr/bin/old-anax -v=5 -alsologtostderr=true -config /etc/colonus/anax-combined-no-cert.config >/tmp/anax.log 2>&1 &
    fi
else
    echo "Starting Anax1 to run workloads."
    if [ ${CERT_LOC} -eq "1" ]; then
      /usr/local/bin/anax -v=5 -alsologtostderr=true -config /etc/colonus/anax-combined.config >/tmp/anax.log 2>&1 &
    else
      /usr/local/bin/anax -v=5 -alsologtostderr=true -config /etc/colonus/anax-combined-no-cert.config >/tmp/anax.log 2>&1 &
    fi
fi

sleep 5

# Make sure org is null
DST=$(curl -sSL http://localhost/node | jq -r '.')
THEORG=$(echo "$DST" | jq -r '.organization')
if [ "$THEORG" != "null" ]
then
  echo -e "organization should be empty: $THEORG"
  exit 2
fi

# Setup anax itself through APIs.
if [ "$HA" == "1" ]
then
    export PARTNERID="$DEVICE_ORG/an54321"
    ./apireg.sh

    if [ $? -ne 0 ]
    then
        echo "HA registration failed"
        TESTFAIL="1"
        exit 2
    else
        echo "Anax1 ready to run workloads."
        sleep 5

        echo "Starting Anax2."
        export DEVICE_ID="an54321"
        export DEVICE_NAME="anaxdev2"
        export ANAX_API="http://localhost:82"
        export PARTNERID="an54321"

        if [ "$OLDANAX" == "1" ]
        then
            echo "Starting OLD Anax2 to run workloads."
            if [ ${CERT_LOC} -eq "1" ]; then
              /usr/bin/old-anax -v=5 -alsologtostderr=true -config /etc/colonus/anax-combined2.config >/tmp/anax2.log 2>&1 &
            else
              /usr/bin/old-anax -v=5 -alsologtostderr=true -config /etc/colonus/anax-combined2-no-cert.config >/tmp/anax2.log 2>&1 &
            fi
        else
            echo "Starting Anax2 to run workloads."
            if [ ${CERT_LOC} -eq "1" ]; then
              /usr/local/bin/anax -v=5 -alsologtostderr=true -config /etc/colonus/anax-combined2.config >/tmp/anax2.log 2>&1 &
            else
              /usr/local/bin/anax -v=5 -alsologtostderr=true -config /etc/colonus/anax-combined2-no-cert.config >/tmp/anax2.log 2>&1 &
            fi
        fi

        sleep 5

        export PARTNERID="$DEVICE_ORG/an12345"
        ./apireg.sh
        if [ $? -ne 0 ]
        then
            TESTFAIL="1"
            exit 2
        fi
    fi

else
    ./apireg.sh
    if [ $? -ne 0 ]
    then
        TESTFAIL="1"
        exit 2
    fi
fi
