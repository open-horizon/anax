#!/bin/bash
# First create the secret that the service will need

if [ "${NOVAULT}" == "1" ]
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

USERDEV_ORG="userdev"
USERDEV_ADMIN_AUTH="userdev/userdevadmin:userdevadminpw"

CREATE_ORG_SECRET1="netspeed-secret1"
CREATE_ORG_SECRET2="netspeed-secret2"
CREATE_ORG_SECRET3="netspeed-secret3"
CREATE_ORG_SECRET4="sqltoken"
CREATE_USER_SECRET5="user/userdevadmin/aitoken"

ORG_SECRET_KEY="test"
ORG_SECRET_VALUE1="netspeed-password"
ORG_SECRET_VALUE2="netspeed-other-password"

# set HZN_AGBOT_URL for the cli
export HZN_AGBOT_URL=${AGBOT_SAPI_URL}

# Create secrets in userdev org
echo -e "Create netspeed secret1"
CMD="hzn secretsmanager secret add -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} --secretKey ${ORG_SECRET_KEY} -d ${ORG_SECRET_VALUE1} ${CREATE_ORG_SECRET1} -O"
echo "$CMD"
RES=$(hzn secretsmanager secret add -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} --secretKey ${ORG_SECRET_KEY} -d ${ORG_SECRET_VALUE1} ${CREATE_ORG_SECRET1} -O) 

# check for erroneous return 
if [ $? -ne 0 ]; then 
  echo -e "Error: the creation command resulted in an error when it should not have: \n"
  exit 2
fi
# otherwise, print the usual results
echo "$RES"

echo -e "Create netspeed secret2"
CMD="hzn secretsmanager secret add -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} --secretKey ${ORG_SECRET_KEY} -d ${ORG_SECRET_VALUE2} ${CREATE_ORG_SECRET2} -O"
echo "$CMD"
RES=$(hzn secretsmanager secret add -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} --secretKey ${ORG_SECRET_KEY} -d ${ORG_SECRET_VALUE2} ${CREATE_ORG_SECRET2} -O)

# check for erroneous return 
if [ $? -ne 0 ]; then 
  echo -e "Error: the creation command resulted in an error when it should not have: \n"
  exit 2
fi
# otherwise, print the usual results
echo "$RES"

echo -e "Create netspeed secret3"
CMD="hzn secretsmanager secret add -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} --secretKey ${ORG_SECRET_KEY} -d ${ORG_SECRET_VALUE2} ${CREATE_ORG_SECRET3} -O"
echo "$CMD"
RES=$(hzn secretsmanager secret add -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} --secretKey ${ORG_SECRET_KEY} -d ${ORG_SECRET_VALUE2} ${CREATE_ORG_SECRET3} -O)

# creating secrets for compcheck tests
echo -e "Create org secret sqltoken"
CMD="hzn secretsmanager secret add -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} --secretKey sqltoken -d mysqltoken ${CREATE_ORG_SECRET4} -O"
echo "$CMD"
RES=$(hzn secretsmanager secret add -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} --secretKey sqltoken -d mysqltoken ${CREATE_ORG_SECRET4} -O)

# check for erroneous return 
if [ $? -ne 0 ]; then 
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
CMD="hzn secretsmanager secret add -o ${E2EDEV_ORG} -u ${E2EDEV_ADMIN_AUTH} --secretKey ${ORG_SECRET_KEY} -d ${ORG_SECRET_VALUE1} ${CREATE_ORG_SECRET1} -O"
echo "$CMD"
RES=$(hzn secretsmanager secret add -o ${E2EDEV_ORG} -u ${E2EDEV_ADMIN_AUTH} --secretKey ${ORG_SECRET_KEY} -d ${ORG_SECRET_VALUE1} ${CREATE_ORG_SECRET1} -O)

echo -e "Create user secret aitoken"
CMD="hzn secretsmanager secret add -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} --secretKey aitoken -d myaitoken ${CREATE_USER_SECRET5} -O"
echo "$CMD"
RES=$(hzn secretsmanager secret add -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} --secretKey aitoken -d myaitoken  ${CREATE_USER_SECRET5} -O)

# check for erroneous return 
if [ $? -ne 0 ]; then 
  echo -e "Error: the creation command resulted in an error when it should not have: \n"
  exit 2
fi
# otherwise, print the usual results
echo "$RES"

echo -e "Create netspeed secret2"
CMD="hzn secretsmanager secret add -o ${E2EDEV_ORG} -u ${E2EDEV_ADMIN_AUTH} --secretKey ${ORG_SECRET_KEY} -d ${ORG_SECRET_VALUE2} ${CREATE_ORG_SECRET2} -O"
echo "$CMD"
RES=$(hzn secretsmanager secret add -o ${E2EDEV_ORG} -u ${E2EDEV_ADMIN_AUTH} --secretKey ${ORG_SECRET_KEY} -d ${ORG_SECRET_VALUE2} ${CREATE_ORG_SECRET2} -O)

# check for erroneous return
if [ $? -ne 0 ]; then
  echo -e "Error: the creation command resulted in an error when it should not have: \n"
  exit 2
fi
# otherwise, print the usual results
echo "$RES"

echo -e "Create netspeed secret3"
CMD="hzn secretsmanager secret add -o ${E2EDEV_ORG} -u ${E2EDEV_ADMIN_AUTH} --secretKey ${ORG_SECRET_KEY} -d ${ORG_SECRET_VALUE2} ${CREATE_ORG_SECRET3} -O"
echo "$CMD"
RES=$(hzn secretsmanager secret add -o ${E2EDEV_ORG} -u ${E2EDEV_ADMIN_AUTH} --secretKey ${ORG_SECRET_KEY} -d ${ORG_SECRET_VALUE2} ${CREATE_ORG_SECRET3} -O)

# check for erroneous return
if [ $? -ne 0 ]; then
  echo -e "Error: the creation command resulted in an error when it should not have: \n"
  exit 2
fi
# otherwise, print the usual results
echo "$RES"
