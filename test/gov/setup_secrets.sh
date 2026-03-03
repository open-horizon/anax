#!/bin/bash

AGBOT_SAPI_URL=${AGBOT_SAPI_URL:-http://127.0.0.1:3110}

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

# ensure the agbot API URL is set
if [ -z "${AGBOT_SAPI_URL}" ]; then
  echo -e "Envvar AGBOT_SAPI_URL must be set.\n"
  exit 1
fi

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
CMD="hzn secretsmanager secret add -o ${USERDEV_ORG} -u ${AGBOT_AUTH} --secretKey ${ORG_SECRET_KEY} -d ${ORG_SECRET_VALUE1} ${CREATE_ORG_SECRET1} -O"
echo "$CMD"

# check for erroneous return
if ! RES=$(hzn secretsmanager secret add -o ${USERDEV_ORG} -u "${AGBOT_AUTH}" --secretKey ${ORG_SECRET_KEY} -d ${ORG_SECRET_VALUE1} ${CREATE_ORG_SECRET1} -O)
then
  echo -e "Error: the creation command resulted in an error when it should not have: \n"
  exit 2
fi
# otherwise, print the usual results
echo "$RES"

echo -e "Create netspeed secret2"
CMD="hzn secretsmanager secret add -o ${USERDEV_ORG} -u ${AGBOT_AUTH} --secretKey ${ORG_SECRET_KEY} -d ${ORG_SECRET_VALUE2} ${CREATE_ORG_SECRET2} -O"
echo "$CMD"

# check for erroneous return
if ! RES=$(hzn secretsmanager secret add -o ${USERDEV_ORG} -u "${AGBOT_AUTH}" --secretKey ${ORG_SECRET_KEY} -d ${ORG_SECRET_VALUE2} ${CREATE_ORG_SECRET2} -O)
then
  echo -e "Error: the creation command resulted in an error when it should not have: \n"
  exit 2
fi
# otherwise, print the usual results
echo "$RES"

echo -e "Create netspeed secret3"
CMD="hzn secretsmanager secret add -o ${USERDEV_ORG} -u ${AGBOT_AUTH} --secretKey ${ORG_SECRET_KEY} -d ${ORG_SECRET_VALUE2} ${CREATE_ORG_SECRET3} -O"
echo "$CMD"
RES=$(hzn secretsmanager secret add -o ${USERDEV_ORG} -u "${AGBOT_AUTH}" --secretKey ${ORG_SECRET_KEY} -d ${ORG_SECRET_VALUE2} ${CREATE_ORG_SECRET3} -O)

# create secrets for k8s secret test
echo -e "Create k8s secret"
CMD="hzn secretsmanager secret add -o ${USERDEV_ORG} -u ${AGBOT_AUTH} --secretKey ${ORG_SECRET_KEY} -d ${ORG_SECRET_VALUE3} ${CREATE_ORG_SECRET5} -O"
echo "$CMD"
RES=$(hzn secretsmanager secret add -o ${USERDEV_ORG} -u "${AGBOT_AUTH}" --secretKey ${ORG_SECRET_KEY} -d ${ORG_SECRET_VALUE3} ${CREATE_ORG_SECRET5} -O)

CMD="hzn secretsmanager secret add -o ${USERDEV_ORG} -u ${AGBOT_AUTH} --secretKey ${ORG_SECRET_KEY} -d ${ORG_SECRET_VALUE4} ${CREATE_ORG_SECRET6} -O"
echo "$CMD"
RES=$(hzn secretsmanager secret add -o ${USERDEV_ORG} -u "${AGBOT_AUTH}" --secretKey ${ORG_SECRET_KEY} -d ${ORG_SECRET_VALUE4} ${CREATE_ORG_SECRET6} -O)

# creating secrets for compcheck tests
echo -e "Create org secret sqltoken"
CMD="hzn secretsmanager secret add -o ${USERDEV_ORG} -u ${AGBOT_AUTH} --secretKey sqltoken -d mysqltoken ${CREATE_ORG_SECRET4} -O"
echo "$CMD"

# check for erroneous return
if ! RES=$(hzn secretsmanager secret add -o ${USERDEV_ORG} -u "${AGBOT_AUTH}" --secretKey sqltoken -d mysqltoken ${CREATE_ORG_SECRET4} -O)
then
  echo -e "Error: the creation command resulted in an error when it should not have: \n"
  exit 2
fi
# otherwise, print the usual results
echo "$RES"

# ==================================================
# Create secrets in e2edev@somecomp.com org
E2EDEV_ORG="e2edev@somecomp.com"
E2EDEV_ADMIN_AUTH="e2edev@somecomp.com/e2edevadmin:e2edevadminpw"

echo -e "Create netspeed secret1"
CMD="hzn secretsmanager secret add -o ${E2EDEV_ORG} -u ${AGBOT_AUTH} --secretKey ${ORG_SECRET_KEY} -d ${ORG_SECRET_VALUE1} ${CREATE_ORG_SECRET1} -O"
echo "$CMD"
RES=$(hzn secretsmanager secret add -o ${E2EDEV_ORG} -u "${AGBOT_AUTH}" --secretKey ${ORG_SECRET_KEY} -d ${ORG_SECRET_VALUE1} ${CREATE_ORG_SECRET1} -O)

echo -e "Create user secret aitoken"
CMD="hzn secretsmanager secret add -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} --secretKey aitoken -d myaitoken ${CREATE_USER_SECRET5} -O"
echo "$CMD"

# check for erroneous return 
if ! RES=$(hzn secretsmanager secret add -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} --secretKey aitoken -d myaitoken  ${CREATE_USER_SECRET5} -O)
then
  echo -e "Error: the creation command resulted in an error when it should not have: \n"
  exit 2
fi
# otherwise, print the usual results
echo "$RES"

echo -e "Create netspeed secret2"
CMD="hzn secretsmanager secret add -o ${E2EDEV_ORG} -u ${AGBOT_AUTH} --secretKey ${ORG_SECRET_KEY} -d ${ORG_SECRET_VALUE2} ${CREATE_ORG_SECRET2} -O"
echo "$CMD"

# check for erroneous return
if ! RES=$(hzn secretsmanager secret add -o ${E2EDEV_ORG} -u "${AGBOT_AUTH}" --secretKey ${ORG_SECRET_KEY} -d ${ORG_SECRET_VALUE2} ${CREATE_ORG_SECRET2} -O)
then
  echo -e "Error: the creation command resulted in an error when it should not have: \n"
  exit 2
fi
# otherwise, print the usual results
echo "$RES"

echo -e "Create netspeed secret3"
CMD="hzn secretsmanager secret add -o ${E2EDEV_ORG} -u ${AGBOT_AUTH} --secretKey ${ORG_SECRET_KEY} -d ${ORG_SECRET_VALUE2} ${CREATE_ORG_SECRET3} -O"
echo "$CMD"

# check for erroneous return
if ! RES=$(hzn secretsmanager secret add -o ${E2EDEV_ORG} -u "${AGBOT_AUTH}" --secretKey ${ORG_SECRET_KEY} -d ${ORG_SECRET_VALUE2} ${CREATE_ORG_SECRET3} -O)
then
  echo -e "Error: the creation command resulted in an error when it should not have: \n"
  exit 2
fi
# otherwise, print the usual results
echo "$RES"
