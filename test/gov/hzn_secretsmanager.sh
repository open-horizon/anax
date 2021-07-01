#!/bin/bash

# ----------------------------
# ----- HELPER FUNCTIONS -----
# ----------------------------

# Print a command and its response on separate lines. The inputs are:
# $1 - the command 
# $2 - the response
function print_command_and_response {
  echo -e "\n$1"
  echo -e "$2"
}

# Verify a response. The inputs are:
# $1 - the command
# $2 - the response
# $3 - expected result
# $4 - error message
function verify {

    print_command_and_response "$1" "$2" 

    respContains=$(echo $2 | grep "$3")
    if [ "${respContains}" == "" ]; then
        echo -e "\nERROR: $4"
        exit 1
    fi
}

# ----------------------------
# ----- SETUP -----
# ----------------------------

# check environment variables
if [ "$HZN_VAULT" != "true" ]
then
  echo -e "Skipping hzn secretsmanager tests"
  exit 0
fi

if [ -z ${AGBOT_SAPI_URL} ]; then
  echo -e "\n${PREFIX} Envvar AGBOT_SAPI_URL is empty. Skip test\n"
  exit 0
fi

# set HZN_AGBOT_URL for the cli
export HZN_AGBOT_URL=${AGBOT_SAPI_URL}

PREFIX="\nhzn secretsmanager CLI test: "

echo -e "$PREFIX start test"

# user authentication variables
E2EDEV_ORG="e2edev@somecomp.com"
E2EDEV_ADMIN_AUTH="e2edevadmin:e2edevadminpw"
USERDEV_ORG="userdev"
USERDEV_ADMIN_AUTH="userdevadmin:userdevadminpw"
USERDEV_USER_AUTH="userdevuser:userdevuserpw"

# -----------------------
# ----- ORG SECRETS -----
# -----------------------
echo -e "$PREFIX testing org level secrets"

# list a secret that doesn't exist - expecting "{ exists: false }"
echo -e "$PREFIX list an org secret that doesn't exist (expecting false)"

CMD="hzn secretsmanager secret list -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} no-password"
RES=$($CMD)
verify "$CMD" "$RES" "false" "secret shouldn't exist"

# add an org secret and check existence
echo -e "$PREFIX add an org secret and check existence using 'list'"

CMD="hzn secretsmanager secret add --secretKey password -d password123 -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} test-password"
RES=$($CMD)
print_command_and_response "$CMD" "$RES"

CMD="hzn secretsmanager secret list -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH}"
RES=$($CMD)
verify "$CMD" "$RES" "test-password" "secret should exist after add"

CMD="hzn secretsmanager secret list -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} test-password"
RES=$($CMD)
verify "$CMD" "$RES" "true" "secret should exist after add"

CMD="hzn secretsmanager secret read -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} test-password"
RES=$($CMD)
verify "$CMD" "$RES" "password123" "secret details should be returned on read"

# update the org secret and check with vault and horizon cli
echo -e "$PREFIX update an org secret and check the updated details"

CMD="hzn secretsmanager secret add --secretKey password -d password321 -O -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} test-password"
RES=$($CMD)
print_command_and_response "$CMD" "$RES"

CMD="hzn secretsmanager secret read -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} test-password"
RES=$($CMD)
verify "$CMD" "$RES" "password321" "secret detail should have been updated after add"

# remove the org secret and check existence
echo -e "$PREFIX remove an org secret and check its existence using 'list'"

CMD="hzn secretsmanager secret remove -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} test-password"
RES=$($CMD)
print_command_and_response "$CMD" "$RES"

CMD="hzn secretsmanager secret list -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} test-password"
RES=$($CMD) 
verify "$CMD" "$RES" "false" "secret shouldn't exist after remove"

# ----------------------------
# ----- USER SECRETS -----
# ----------------------------
echo -e "$PREFIX testing user level secrets"

# list a secret that doesn't exist - expecting "{ exists: false }"
echo -e "$PREFIX list a user secret that doesn't exist (expecting false)"

CMD="hzn secretsmanager secret list -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} user/userdevadmin/no-password"
RES=$($CMD)
verify "$CMD" "$RES" "false" "secret shouldn't exist"

# add a user secret and check existence 
echo -e "$PREFIX add a user secret and check existence using 'list'"

CMD="hzn secretsmanager secret add --secretKey password -d password123 -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} user/userdevadmin/test-password"
RES=$($CMD)
print_command_and_response "$CMD" "$RES"

CMD="hzn secretsmanager secret list -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} user/userdevadmin"
RES=$($CMD)
verify "$CMD" "$RES" "test-password" "secret should exist after add"

CMD="hzn secretsmanager secret list -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} user/userdevadmin/test-password"
RES=$($CMD)
verify "$CMD" "$RES" "true" "secret should exist after add"

CMD="hzn secretsmanager secret read -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} user/userdevadmin/test-password"
RES=$($CMD)
verify "$CMD" "$RES" "password123" "secret details should be returned on read"

# update the user secret and check with vault cli 
echo -e "$PREFIX update a user secret and check the updated details"

CMD="hzn secretsmanager secret add --secretKey password -d password321 -O -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} user/userdevadmin/test-password"
RES=$($CMD)
print_command_and_response "$CMD" "$RES"

CMD="hzn secretsmanager secret read -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} user/userdevadmin/test-password"
RES=$($CMD)
verify "$CMD" "$RES" "password321" "secret detail should have been updated after add"

# remove the user secret and check existence 
echo -e "$PREFIX remove a user secret and check its existence using 'list'"

CMD="hzn secretsmanager secret remove -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} user/userdevadmin/test-password"
RES=$($CMD)
print_command_and_response "$CMD" "$RES"

CMD="hzn secretsmanager secret list -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} user/userdevadmin/test-password"
RES=$($CMD)
verify "$CMD" "$RES" "false" "secret shouldn't exist after remove"

# ----------------------------
# ----- EXPECTED ERRORS -----
# ----------------------------
echo -e "$PREFIX testing expected errors"

echo -e "$PREFIX adding secret for ${USERDEV_ORG} organization"
CMD="hzn secretsmanager secret add --secretKey password -d password123 -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} test-password"
RES=$($CMD)
print_command_and_response "$CMD" "$RES"

echo -e "$PREFIX adding user secret for ${USERDEV_ORG} organization"
CMD="hzn secretsmanager secret add --secretKey password -d password123 -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} user/userdevadmin/test-password"
RES=$($CMD)
print_command_and_response "$CMD" "$RES"

echo -e "$PREFIX adding user secret for ${E2EDEV_ORG} organization"
CMD="hzn secretsmanager secret add --secretKey password -d password321 -o ${E2EDEV_ORG} -u ${E2EDEV_ADMIN_AUTH} user/e2edevadmin/test-password"
RES=$($CMD)
print_command_and_response "$CMD" "$RES"

echo -e "$PREFIX adding test user for ${USERDEV_ORG} organization"
CMD="hzn exchange user create -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} userdevuser userdevuserpw"
RES=$($CMD)
print_command_and_response "$CMD" "$RES"

# error on `list` - secret owned by a different user 
echo -e "$PREFIX listing a secret owned by a different user"

CMD="hzn secretsmanager secret list -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} user/e2edevadmin/test-password"
echo -e "$CMD"
$($CMD)
if [ $? -eq 0 ]; then 
  echo -e "\nERROR: shouldn't be able to list a secret owned by a different user"
  exit 1
fi 

# error on `remove` - secret owned by a different user
echo -e "$PREFIX removing a secret owned by a different user"

CMD="hzn secretsmanager secret remove -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} user/e2edevadmin/test-password" 
echo -e "$CMD"
$($CMD)
if [ $? -eq 0 ]; then 
  echo -e "\nERROR: shouldn't be able to remove a secret owned by a different user"
  exit 1
fi 

# error on `remove` - secret doesn't exist at the org level
echo -e "$PREFIX removing a secret that doesn't exist at the org level"

CMD="hzn secretsmanager secret remove -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} fake-password"
echo -e "$CMD"
$($CMD)
if [ $? -eq 0 ]; then 
  echo -e "\nERROR: shouldn't be able to remove a secret that doesn't exist"
  exit 1
fi 

# error on `remove` - secret doesn't exist at the user level
echo -e "$PREFIX removing a secret that doesn't exist at the user level"

CMD="hzn secretsmanager secret remove -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} user/userdevadmin/fake-password"
echo -e "$CMD"
$($CMD)
if [ $? -eq 0 ]; then 
  echo -e "\nERROR: shouldn't be able to remove a secret that doesn't exist"
  exit 1
fi 

# error on `read` - secret doesn't exist
echo -e "$PREFIX reading a secret that doesn't exist"

CMD="hzn secretsmanager secret read -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} fake-password"
echo -e "$CMD"
$($CMD)
if [ $? -eq 0 ]; then 
  echo -e "\nERROR: shouldn't be able to read a secret that doesn't exist"
  exit 1
fi 

# error on `read` - user can't read org level secrets 
echo -e "$PREFIX non-admin shouldn't read org level secrets"

CMD="hzn secretsmanager secret read -o ${USERDEV_ORG} -u ${USERDEV_USER_AUTH} test-password"
echo -e "$CMD"
$($CMD)
if [ $? -eq 0 ]; then 
  echo -e "\nERROR: user shouldn't be able to read org-level secrets"
  exit 1
fi 

# error on `read` - user can't read another user's secrets
echo -e "$PREFIX user shouldn't read another user's secrets"

CMD="hzn secretsmanager secret read -o ${USERDEV_ORG} -u ${USERDEV_USER_AUTH} user/userdevadmin/test-password"
echo -e "$CMD"
$($CMD)
if [ $? -eq 0 ]; then 
  echo -e "\nERROR: user shouldn't be able to read org-level secrets"
  exit 1
fi 

# error on `add` - secret owned by a different user 
echo -e "$PREFIX adding a secret owned by a different user"

CMD="hzn secretsmanager secret add --secretKey password -d password456 -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} user/e2edevadmin/fake-password"
echo -e "$CMD"
$($CMD)
if [ $? -eq 0 ]; then 
  echo -e "\nERROR: shouldn't be able to remove a secret owned by a different user"
  exit 1
fi 

# ----------------------------
# ----- CLEANUP -----
# ----------------------------
echo -e "$PREFIX starting cleanup"

# remove secrets
echo -e "$PREFIX removing org secrets"
CMD="hzn secretsmanager secret remove -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} test-password"
RES=$($CMD)
print_command_and_response "$CMD" "$RES"

echo -e "$PREFIX removing user secrets"
CMD="hzn secretsmanager secret remove -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} user/userdevadmin/test-password"
RES=$($CMD)
print_command_and_response "$CMD" "$RES"

CMD="hzn secretsmanager secret remove -o ${E2EDEV_ORG} -u ${E2EDEV_ADMIN_AUTH} user/e2edevadmin/test-password"
RES=$($CMD)
print_command_and_response "$CMD" "$RES"

echo -e "$PREFIX removing test user"
CMD="hzn exchange user remove -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} -f userdevuser"
RES=$($CMD)
print_command_and_response "$CMD" "$RES"

echo -e "$PREFIX complete test"



