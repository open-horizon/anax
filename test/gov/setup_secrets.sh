#!/bin/bash

# Enable debug tracing when DEBUG=1 or RUNNER_DEBUG=1 (GitHub Actions debug mode).
if [ "${DEBUG:-0}" = "1" ] || [ "${RUNNER_DEBUG:-0}" = "1" ]; then
    set -x
fi

# First create the secret that the service will need

if [ "${NOVAULT}" = "1" ]
then
  echo -e "Skipping secret setup"
  exit 0
fi

# Set default AGBOT_SAPI_URL if not provided
AGBOT_SAPI_URL=${AGBOT_SAPI_URL:-http://127.0.0.1:3110}

USERDEV_ORG="userdev"
USERDEV_ADMIN_AUTH="userdev/userdevadmin:userdevadminpw"
# Agbot credentials for secrets manager API
AGBOT_AUTH="IBM/agbot1:${AGBOT_TOKEN:-Abcdefghijklmno1}"

CREATE_ORG_SECRET1="netspeed-secret1"
CREATE_ORG_SECRET2="netspeed-secret2"
CREATE_ORG_SECRET3="netspeed-secret3"
CREATE_ORG_SECRET4="sqltoken"
CREATE_ORG_SECRET5="k8s-hello-secret1"
CREATE_ORG_SECRET6="k8s-hello-secret2"

CREATE_USER_SECRET5="user/userdevadmin/aitoken"

ORG_SECRET_KEY="test"
ORG_SECRET_VALUE1="netspeed-password"
ORG_SECRET_VALUE2="netspeed-other-password"
ORG_SECRET_VALUE3="k8s-password1"
ORG_SECRET_VALUE4="k8s-password2"

# set HZN_AGBOT_URL for the cli
export HZN_AGBOT_URL=${AGBOT_SAPI_URL}

# Create secrets in userdev org
echo -e "Create netspeed secret1"
if ! RES=$(hzn secretsmanager secret add -o "${USERDEV_ORG}" -u "${AGBOT_AUTH}" --secretKey "${ORG_SECRET_KEY}" -d "${ORG_SECRET_VALUE1}" "${CREATE_ORG_SECRET1}" -O)
then
  echo -e "Error: the creation command resulted in an error when it should not have: \n"
  exit 2
fi
echo "$RES"

echo -e "Create netspeed secret2"
if ! RES=$(hzn secretsmanager secret add -o "${USERDEV_ORG}" -u "${AGBOT_AUTH}" --secretKey "${ORG_SECRET_KEY}" -d "${ORG_SECRET_VALUE2}" "${CREATE_ORG_SECRET2}" -O)
then
  echo -e "Error: the creation command resulted in an error when it should not have: \n"
  exit 2
fi
echo "$RES"

echo -e "Create netspeed secret3"
if ! RES=$(hzn secretsmanager secret add -o "${USERDEV_ORG}" -u "${AGBOT_AUTH}" --secretKey "${ORG_SECRET_KEY}" -d "${ORG_SECRET_VALUE2}" "${CREATE_ORG_SECRET3}" -O)
then
  echo -e "Error: the creation command resulted in an error when it should not have: \n"
  exit 2
fi
echo "$RES"

# create secrets for k8s secret test
echo -e "Create k8s secret 1"
if ! RES=$(hzn secretsmanager secret add -o "${USERDEV_ORG}" -u "${AGBOT_AUTH}" --secretKey "${ORG_SECRET_KEY}" -d "${ORG_SECRET_VALUE3}" "${CREATE_ORG_SECRET5}" -O)
then
  echo -e "Error: the creation command resulted in an error when it should not have: \n"
  exit 2
fi
echo "$RES"

echo -e "Create k8s secret 2"
if ! RES=$(hzn secretsmanager secret add -o "${USERDEV_ORG}" -u "${AGBOT_AUTH}" --secretKey "${ORG_SECRET_KEY}" -d "${ORG_SECRET_VALUE4}" "${CREATE_ORG_SECRET6}" -O)
then
  echo -e "Error: the creation command resulted in an error when it should not have: \n"
  exit 2
fi
echo "$RES"

# creating secrets for compcheck tests
echo -e "Create org secret sqltoken"
if ! RES=$(hzn secretsmanager secret add -o "${USERDEV_ORG}" -u "${AGBOT_AUTH}" --secretKey sqltoken -d mysqltoken "${CREATE_ORG_SECRET4}" -O)
then
  echo -e "Error: the creation command resulted in an error when it should not have: \n"
  exit 2
fi
echo "$RES"

# ==================================================
# Create secrets in e2edev@somecomp.com org
E2EDEV_ORG="e2edev@somecomp.com"

echo -e "Create netspeed secret1"
if ! RES=$(hzn secretsmanager secret add -o "${E2EDEV_ORG}" -u "${AGBOT_AUTH}" --secretKey "${ORG_SECRET_KEY}" -d "${ORG_SECRET_VALUE1}" "${CREATE_ORG_SECRET1}" -O)
then
  echo -e "Error: the creation command resulted in an error when it should not have: \n"
  exit 2
fi
echo "$RES"

echo -e "Create user secret aitoken"
if ! RES=$(hzn secretsmanager secret add -o "${USERDEV_ORG}" -u "${USERDEV_ADMIN_AUTH}" --secretKey aitoken -d myaitoken "${CREATE_USER_SECRET5}" -O)
then
  echo -e "Error: the creation command resulted in an error when it should not have: \n"
  exit 2
fi
echo "$RES"

echo -e "Create netspeed secret2"
if ! RES=$(hzn secretsmanager secret add -o "${E2EDEV_ORG}" -u "${AGBOT_AUTH}" --secretKey "${ORG_SECRET_KEY}" -d "${ORG_SECRET_VALUE2}" "${CREATE_ORG_SECRET2}" -O)
then
  echo -e "Error: the creation command resulted in an error when it should not have: \n"
  exit 2
fi
echo "$RES"

echo -e "Create netspeed secret3"
if ! RES=$(hzn secretsmanager secret add -o "${E2EDEV_ORG}" -u "${AGBOT_AUTH}" --secretKey "${ORG_SECRET_KEY}" -d "${ORG_SECRET_VALUE2}" "${CREATE_ORG_SECRET3}" -O)
then
  echo -e "Error: the creation command resulted in an error when it should not have: \n"
  exit 2
fi
echo "$RES"
