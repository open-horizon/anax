#!/bin/bash

echo "Testing model management APIs"

# Test what happens when an invalid user id format is attempted
UFORMAT=$(curl -sLX GET -w "%{http_code}" --cacert /certs/css.crt -u fred:ethel "https://css-api:9443/api/v1/destinations/userdev")

if [ "$UFORMAT" != "Unauthorized403" ]
then
	echo -e "Error testing CSS API with invalid user format, should have received 403, received $UFORMAT"
	exit -1
fi

# Test what happens when an unknown user id is attempted
UUSER=$(curl -sLX GET -w "%{http_code}" --cacert /certs/css.crt -u userdev/ethel:murray "https://css-api:9443/api/v1/destinations/userdev")

if [ "$UUSER" != "Unauthorized403" ]
then
	echo -e "Error testing CSS API with unknown user, should have received 403, received $UUSER"
	exit -1
fi

# Test what happens when an unknown node is attempted
UNODE=$(curl -sLX GET -w "%{http_code}" --cacert /certs/css.crt -u fred/ethel/murray:ethel "https://css-api:9443/api/v1/destinations/userdev")

if [ "$UNODE" != "Unauthorized403" ]
then
	echo -e "Error testing CSS API with unknown node, should have received 403, received $UNODE"
	exit -1
fi

# Test what happens when a valid node tries to access an API
KNODE=$(curl -sLX GET -w "%{http_code}" --cacert /certs/css.crt -u userdev/susehello/an12345:abcdefg  "https://css-api:9443/api/v1/destinations/userdev")

if [ "$KNODE" != "Unauthorized403" ]
then
	echo -e "Error testing CSS API with known node, should have received 403, received $KNODE"
	exit -1
fi

echo "Testing model management APIs successful"
