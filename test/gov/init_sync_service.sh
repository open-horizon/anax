#!/bin/bash

#--cacert /certs/css.crt
if [ ${CERT_LOC} -eq "1" ]; then
  CERT_VAR="--cacert /certs/css.crt"
else
  CERT_VAR=""
fi

# create orgs in sync service
echo "Creating e2edev@somecomp.com organization in CSS..."

CR8EORG=$(curl -sLX PUT -w "%{http_code}" $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d '{"orgID":"e2edev@somecomp.com"}' "${CSS_URL}/api/v1/organizations/e2edev@somecomp.com")
echo "$CR8EORG"

echo "Creating userdev organization in CSS..."
CR8UORG=$(curl -sLX PUT -w "%{http_code}" $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d '{"orgID":"userdev"}' "${CSS_URL}/api/v1/organizations/userdev")
echo "$CR8UORG"

echo "Creating IBM organization..."
CR8IORG=$(curl -sLX PUT -w "%{http_code}" $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d '{"orgID":"IBM"}' "${CSS_URL}/api/v1/organizations/IBM")
echo "$CR8IORG"

echo "Creating Customer1 organization in CSS..."
CR8C1ORG=$(curl -sLX PUT -w "%{http_code}" $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d '{"orgID":"Customer1"}' "${CSS_URL}/api/v1/organizations/Customer1")
echo "$CR8C1ORG"

echo "Creating Customer2 organization in CSS..."
CR8C2ORG=$(curl -sLX PUT -w "%{http_code}" $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d '{"orgID":"Customer2"}' "${CSS_URL}/api/v1/organizations/Customer2")

echo "$CR8C2ORG"
