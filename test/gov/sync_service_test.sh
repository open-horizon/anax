#!/bin/bash

echo "Testing model management APIs"

# Test what happens when an invalid user id format is attempted
if [ ${CERT_LOC} -eq "1" ]; then
	UFORMAT=$(curl -sLX GET -w "%{http_code}" --cacert /certs/css.crt -u fred:ethel "${CSS_URL}/api/v1/destinations/userdev")
else
	UFORMAT=$(curl -sLX GET -w "%{http_code}" -u fred:ethel "${CSS_URL}/api/v1/destinations/userdev")
fi

if [ "$UFORMAT" != "Unauthorized403" ]
then
	echo -e "Error testing CSS API with invalid user format, should have received 403, received $UFORMAT"
	exit -1
fi

# Test what happens when an unknown user id is attempted
if [ ${CERT_LOC} -eq "1" ]; then
	UUSER=$(curl -sLX GET -w "%{http_code}" --cacert /certs/css.crt -u userdev/ethel:murray "${CSS_URL}/api/v1/destinations/userdev")
else
	UUSER=$(curl -sLX GET -w "%{http_code}" -u userdev/ethel:murray "${CSS_URL}/api/v1/destinations/userdev")
fi

if [ "$UUSER" != "Unauthorized403" ]
then
	echo -e "Error testing CSS API with unknown user, should have received 403, received $UUSER"
	exit -1
fi

# Test what happens when an unknown node is attempted
if [ ${CERT_LOC} -eq "1" ]; then
	UNODE=$(curl -sLX GET -w "%{http_code}" --cacert /certs/css.crt -u fred/ethel/murray:ethel "${CSS_URL}/api/v1/destinations/userdev")
else
	UNODE=$(curl -sLX GET -w "%{http_code}" -u fred/ethel/murray:ethel "${CSS_URL}/api/v1/destinations/userdev")
fi

if [ "$UNODE" != "Unauthorized403" ]
then
	echo -e "Error testing CSS API with unknown node, should have received 403, received $UNODE"
	exit -1
fi

# Test what happens when a valid node tries to access an API
if [ ${CERT_LOC} -eq "1" ]; then
	KNODE=$(curl -sLX GET -w "%{http_code}" --cacert /certs/css.crt -u userdev/susehello/an12345:abcdefg  "${CSS_URL}/api/v1/destinations/userdev")
else
	KNODE=$(curl -sLX GET -w "%{http_code}" -u userdev/susehello/an12345:abcdefg  "${CSS_URL}/api/v1/destinations/userdev")
fi

if [ "$KNODE" != "Unauthorized403" ]
then
	echo -e "Error testing CSS API with known node, should have received 403, received $KNODE"
	exit -1
fi

echo "Testing model management APIs successful"
