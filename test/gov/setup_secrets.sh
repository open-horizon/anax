#!/bin/bash
# First create the secret that the service will need

# check the the result to see if it matches the expected http code and error
function results {

  rc="${1: -3}"
  output="${1::-3}"

  echo "$1" | jq -r '.'

  if [ "$rc" == "200" ]; then
    COMP_RESULT=$output
  fi

  # check http code
  if [ "$rc" != $2 ]
  then
    echo -e "Error: $(echo "$output" | jq -r '.')\n"
    exit 2
  fi

  # check if error text contains all of the test text snippets
  for (( i=3; i<=$#; i++))
  {
    eval TEST_ARG='$'$i
    if [ ! -z "$TEST_ARG" ]; then
      res=$(echo "$1" | grep "$TEST_ARG")
      if [ $? -ne 0 ]; then
        echo -e "Error: the response should have contained \"$TEST_ARG\", but did not. \n"
        exit 2
      fi
    fi
  }

  #statements
  echo -e "Result expected."
}

if [ "${EXCH_APP_HOST}" != "http://exchange-api:8080/v1" ]; then
  exit 0
fi

if [ "$HZN_VAULT" != "true" ]
then
  echo -e "Skipping secret setup"
  exit 0
fi

# get the cert file
if [ ${CERT_LOC} -eq "1" ]; then
  CERT_VAR="--cacert /certs/agbotapi.crt"
else
  CERT_VAR=""
fi

# ensure the agbot API URL is set
if [ -z ${AGBOT_SAPI_URL} ]; then
  echo -e "Envvar AGBOT_SAPI_URL must be set.\n"
  exit 1
fi

USERDEV_ADMIN_AUTH="userdev/userdevadmin:userdevadminpw"

CREATE_ORG_SECRET1="org/userdev/secrets/netspeed-secret1"
CREATE_ORG_SECRET2="org/userdev/secrets/netspeed-secret2"

read -d '' create_secret <<EOF
{
  \"key\":\"test\",
  \"value\":\"netspeed-password\"
}
EOF

echo -e "Create netspeed secret1"
CMD="curl -sLX POST -w %{http_code} ${CERT_VAR} -u ${USERDEV_ADMIN_AUTH} -d ${create_secret} ${AGBOT_SAPI_URL}/${CREATE_ORG_SECRET1}"
echo "$CMD"
RES=$(curl -sLX POST -w %{http_code} ${CERT_VAR} -u ${USERDEV_ADMIN_AUTH} -d "${create_secret}" ${AGBOT_SAPI_URL}/${CREATE_ORG_SECRET1})
results "$RES"

echo -e "Create netspeed secret2"
CMD="curl -sLX POST -w %{http_code} ${CERT_VAR} -u ${USERDEV_ADMIN_AUTH} -d ${create_secret} ${AGBOT_SAPI_URL}/${CREATE_ORG_SECRET2}"
echo "$CMD"
RES=$(curl -sLX POST -w %{http_code} ${CERT_VAR} -u ${USERDEV_ADMIN_AUTH} -d "${create_secret}" ${AGBOT_SAPI_URL}/${CREATE_ORG_SECRET2})
results "$RES"