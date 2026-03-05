#!/bin/bash

# Enable debug tracing when DEBUG=1 or RUNNER_DEBUG=1 (GitHub Actions debug mode).
if [ "${DEBUG:-0}" = "1" ] || [ "${RUNNER_DEBUG:-0}" = "1" ]; then
    set -x
fi

#--cacert /certs/css.crt
if [ "${CERT_LOC}" = "1" ]; then
  CERT_VAR=(--cacert /certs/css.crt)
else
  CERT_VAR=(--silent)
fi

# create orgs in sync service
echo "Creating e2edev@somecomp.com organization in CSS..."

CR8EORG=$(curl -sLX PUT "${CERT_VAR[@]}" --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d '{"orgID":"e2edev@somecomp.com"}' "${CSS_URL}/api/v1/organizations/e2edev@somecomp.com" | jq)
echo "$CR8EORG"

echo "Creating userdev organization in CSS..."
CR8UORG=$(curl -sLX PUT "${CERT_VAR[@]}" --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d '{"orgID":"userdev"}' "${CSS_URL}/api/v1/organizations/userdev" | jq)
echo "$CR8UORG"

echo "Creating IBM organization..."
CR8IORG=$(curl -sLX PUT "${CERT_VAR[@]}" --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d '{"orgID":"IBM"}' "${CSS_URL}/api/v1/organizations/IBM" | jq)
echo "$CR8IORG"

echo "Creating Customer1 organization in CSS..."
CR8C1ORG=$(curl -sLX PUT "${CERT_VAR[@]}" --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d '{"orgID":"Customer1"}' "${CSS_URL}/api/v1/organizations/Customer1" | jq)
echo "$CR8C1ORG"

echo "Creating Customer2 organization in CSS..."
CR8C2ORG=$(curl -sLX PUT "${CERT_VAR[@]}" --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" -d '{"orgID":"Customer2"}' "${CSS_URL}/api/v1/organizations/Customer2" | jq)
echo "$CR8C2ORG"
