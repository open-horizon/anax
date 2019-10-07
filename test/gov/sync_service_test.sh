#!/bin/bash

echo "Testing model management APIs"

if [ ${CERT_LOC} -eq "1" ]; then
  CERT_VAR="--cacert /certs/css.crt"
else
  CERT_VAR=""
fi

# Test what happens when an invalid user id format is attempted
UFORMAT=$(curl -sLX GET -w "%{http_code}" $CERT_VAR -u fred:ethel "${CSS_URL}/api/v1/destinations/userdev")

if [ "$UFORMAT" != "Unauthorized403" ]
then
  echo -e "Error testing CSS API with invalid user format, should have received 403, received $UFORMAT"
  exit -1
fi

# Test what happens when an unknown user id is attempted
UUSER=$(curl -sLX GET -w "%{http_code}" $CERT_VAR -u userdev/ethel:murray "${CSS_URL}/api/v1/destinations/userdev")

if [ "$UUSER" != "Unauthorized403" ]
then
  echo -e "Error testing CSS API with unknown user, should have received 403, received $UUSER"
  exit -1
fi

# Test what happens when an unknown node is attempted
UNODE=$(curl -sLX GET -w "%{http_code}" $CERT_VAR -u fred/ethel/murray:ethel "${CSS_URL}/api/v1/destinations/userdev")

if [ "$UNODE" != "Unauthorized403" ]
then
  echo -e "Error testing CSS API with unknown node, should have received 403, received $UNODE"
  exit -1
fi

# Test what happens when a valid node tries to access an API
KNODE=$(curl -sLX GET -w "%{http_code}" $CERT_VAR -u userdev/susehello/an12345:abcdefg  "${CSS_URL}/api/v1/destinations/userdev")

if [ "$KNODE" != "Unauthorized403" ]
then
  echo -e "Error testing CSS API with known node, should have received 403, received $KNODE"
  exit -1
fi

#Test timeout support for upload of large files to the CSS
dd if=/dev/zero of=/tmp/data.txt count=512 bs=1048576

RESOURCE_ORG1=e2edev@somecomp.com
RESOURCE_TYPE=test

export HZN_FSS_CSSURL=${CSS_URL}

read -d '' resmeta <<EOF
{
  "objectID": "test1",
  "objectType": "test",
  "destinationType": "test",
  "version": "1.0.0",
  "description": "a file"
}
EOF

echo "$resmeta" > /tmp/meta.json

hzn mms object publish -m /tmp/meta.json -f /tmp/data.txt
RC=$?
if [ $RC -ne 0 ]
then
  echo -e "Got unexpected error uploading 512MB model object: $RC"
  exit -1
fi

# Now, shorten the HTTP request timeout so that the upload fails. Internally, the CLI will retry
# before giving up with the appropriate HTTP Client timeout error.
export HZN_HTTP_TIMEOUT="2"

hzn mms object publish -m /tmp/meta.json -f /tmp/data.txt
RC=$?
if [ $RC -eq 5 ]
then
  echo -e "Got expected error with 521MB object upload using short HTTP request timeout: $RC"
else
  echo -e "Got unexpected error with 521MB object upload using short HTTP request timeout: $RC"
  exit -1
fi

# Reset the HTTP timeout env var to the default for the CLI.
unset HZN_HTTP_TIMEOUT

echo "Testing model management APIs successful"
