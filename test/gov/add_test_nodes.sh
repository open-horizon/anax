#!/bin/bash

# A test script used to create additional nodes in the system for testing agbot searches. These nodes dont do anything, they just get created so that the
# agbot can find them once in the exchange. The agbot might send proposal messages, but those messages will timeout, which is ok.

# Run this script from the command line outside the e2edev container. The exchange URL is setup to use the local e2edev exchange.
# The nodes are created using the default e2edev user for the embedded agent.

# set -x

export EXCH_URL="http://localhost:8090/v1"

NUM=1

while :
do
	ADD=$(curl -sLX PUT --header 'Content-Type: application/json' --header 'Accept: application/json' -u "userdev/useranax1:useranax1pw" -d '{"token":"abcdefg","name":"anaxdev${NUM}","registeredServices":[],"msgEndPoint":"","softwareVersions":{},"publicKey":"MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA1Ccj6TOUxvVUoIlqyrZUjR3RSdOiBWbWUsgbkhWHcWMNMxD7Y/sLqTl1kZCayFE+bqBvdRmJ4KV7p2g4i/Q+IhBk6Ea+rjVuk5Rwq1OXG2xNRCDX/I9Xc6udoC5qFjf0WG9PAGAqkTSkCpK2wDEvSNAEI8nEXh4l4fPQTCGPDiXxZNCdvi3GAxdw3FN6H89CQRQ7MwO/QiDg11bK5hHb0pVhMOmoYUxFxKeJMEF0kg88dbDrty1lrhI/pf+ZzHZ1BqjDSrazpYieCU2Et2cowsiAyBBTRrIIxy4n5pzWPfAay5tBx1UJDzbJPk2ut1yGWMrHhk+QpXpqgXDBnAfWCQIDAQAB","pattern":"","arch":"amd64"}' "${EXCH_URL}/orgs/userdev/nodes/an12345${NUM}")

	echo $ADD

	HB=$(curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -u "userdev/an12345${NUM}:abcdefg" -d '{"changeId":0,"maxRecords":1000,"orgList":["userdev"]}' "${EXCH_URL}/orgs/userdev/changes")

	# echo $HB

	if [ ${NUM} -gt "7" ]; then
		break
	else
		let NUM=NUM+1
	fi
done

#set +x
