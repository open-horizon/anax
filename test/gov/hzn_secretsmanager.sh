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
if [ "${NOVAULT}" == "1" ]
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

# -----------------------
# ----- ORG SECRETS -----
# -----------------------
echo -e "$PREFIX testing org level secrets"

# list a secret that doesn't exist - expecting "{ exists: false }"
echo -e "$PREFIX list an org secret that doesn't exist (expecting false)"

CMD="hzn sm secret list -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} no-password"
RES=$($CMD)
verify "$CMD" "$RES" "false" "secret shouldn't exist"

# add an org secret and check existence
echo -e "$PREFIX add an org secret and check existence using 'list'"

CMD="hzn sm secret add --secretKey password -d password123 -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} test-password"
RES=$($CMD)
print_command_and_response "$CMD" "$RES"

CMD="hzn sm secret list -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH}"
RES=$($CMD)
verify "$CMD" "$RES" "test-password" "secret should exist after add"

CMD="hzn sm secret list -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} test-password"
RES=$($CMD)
verify "$CMD" "$RES" "true" "secret should exist after add"

CMD="hzn sm secret read -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} test-password"
RES=$($CMD)
verify "$CMD" "$RES" "password123" "secret details should be returned on read"

# update the org secret and check with vault and horizon cli
echo -e "$PREFIX update an org secret and check the updated details"

CMD="hzn sm secret add --secretKey password -d password321 -O -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} test-password"
RES=$($CMD)
print_command_and_response "$CMD" "$RES"

CMD="hzn sm secret read -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} test-password"
RES=$($CMD)
verify "$CMD" "$RES" "password321" "secret detail should have been updated after add"

# remove the org secret and check existence
echo -e "$PREFIX remove an org secret and check its existence using 'list'"

CMD="hzn sm secret remove -f -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} test-password"
RES=$($CMD)
print_command_and_response "$CMD" "$RES"

CMD="hzn sm secret list -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} test-password"
RES=$($CMD) 
verify "$CMD" "$RES" "false" "secret shouldn't exist after remove"

# ----------------------------
# ----- USER SECRETS -----
# ----------------------------
echo -e "$PREFIX testing user level secrets"

# list a secret that doesn't exist - expecting "{ exists: false }"
echo -e "$PREFIX list a user secret that doesn't exist (expecting false)"

CMD="hzn sm secret list -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} user/userdevadmin/no-password"
RES=$($CMD)
verify "$CMD" "$RES" "false" "secret shouldn't exist"

# add a user secret and check existence 
echo -e "$PREFIX add a user secret and check existence using 'list'"

CMD="hzn sm secret add --secretKey password -d password123 -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} user/userdevadmin/test-password"
RES=$($CMD)
print_command_and_response "$CMD" "$RES"

CMD="hzn sm secret list -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} user/userdevadmin"
RES=$($CMD)
verify "$CMD" "$RES" "test-password" "secret should exist after add"

CMD="hzn sm secret list -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} user/userdevadmin/test-password"
RES=$($CMD)
verify "$CMD" "$RES" "true" "secret should exist after add"

CMD="hzn sm secret read -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} user/userdevadmin/test-password"
RES=$($CMD)
verify "$CMD" "$RES" "password123" "secret details should be returned on read"

# update the user secret and check with vault cli 
echo -e "$PREFIX update a user secret and check the updated details"

CMD="hzn sm secret add --secretKey password -d password321 -O -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} user/userdevadmin/test-password"
RES=$($CMD)
print_command_and_response "$CMD" "$RES"

CMD="hzn sm secret read -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} user/userdevadmin/test-password"
RES=$($CMD)
verify "$CMD" "$RES" "password321" "secret detail should have been updated after add"

# remove the user secret and check existence 
echo -e "$PREFIX remove a user secret and check its existence using 'list'"

CMD="hzn sm secret remove -f -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} user/userdevadmin/test-password"
RES=$($CMD)
print_command_and_response "$CMD" "$RES"

CMD="hzn sm secret list -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} user/userdevadmin/test-password"
RES=$($CMD)
verify "$CMD" "$RES" "false" "secret shouldn't exist after remove"

# ----------------------------
# ----- MULTI-PART SECRETS -----
# ----------------------------
echo -e "$PREFIX testing multi-part secrets"

# nested 1 level 
echo -e "$PREFIX two-part secrets"

CMD="hzn sm secret add -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} --secretKey password -d password123 test/password1"
RES=$($CMD)
print_command_and_response "$CMD" "$RES"

CMD="hzn sm secret list -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH}"
RES=$($CMD)
verify "$CMD" "$RES" "test/password1" "secret 'test/password1' should be listed"

# nested 2 levels 
echo -e "$PREFIX three-part secrets"

CMD="hzn sm secret add -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} --secretKey password -d password123 test/more-passwords/password2"
RES=$($CMD)
print_command_and_response "$CMD" "$RES"

CMD="hzn sm secret list -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH}"
RES=$($CMD)
verify "$CMD" "$RES" "test/more-passwords/password2" "secret 'test/more-passwords/password2' should be listed"

# 'list' on a directory should return false 
echo -e "$PREFIX 'list' on directories should return false"

CMD="hzn sm secret list -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} test"
RES=$($CMD)
verify "$CMD" "$RES" "false" "secret 'test' should not exist"

CMD="hzn sm secret list -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} test/more-passwords"
RES=$($CMD)
verify "$CMD" "$RES" "false" "secret 'test/more-passwords' should not exist"

# ----------------------------
# ----- EXPECTED ERRORS -----
# ----------------------------
echo -e "$PREFIX testing expected errors"

echo -e "$PREFIX adding secret for ${USERDEV_ORG} organization"
CMD="hzn sm secret add --secretKey password -d password123 -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} test-password"
RES=$($CMD)
print_command_and_response "$CMD" "$RES"

echo -e "$PREFIX adding test user 1 for ${USERDEV_ORG} organization"
CMD="hzn exchange user create -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} userdevuser1 userdevuser1pw"
RES=$($CMD)
print_command_and_response "$CMD" "$RES"

echo -e "$PREFIX adding test user 2 for ${USERDEV_ORG} organization"
CMD="hzn exchange user create -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} userdevuser2 userdevuser2pw"
RES=$($CMD)
print_command_and_response "$CMD" "$RES"

echo -e "$PREFIX adding user secret for ${USERDEV_ORG} organization"
CMD="hzn sm secret add --secretKey password -d password123 -o ${USERDEV_ORG} -u userdevuser1:userdevuser1pw user/userdevuser1/test-password"
RES=$($CMD)
print_command_and_response "$CMD" "$RES"

# error on `list` - secret owned by a different user 
echo -e "$PREFIX listing a secret owned by a different user"

CMD="hzn sm secret list -o ${USERDEV_ORG} -u userdevuser2:userdevuser2pw user/userdevuser1/test-password"
RES=$($CMD 2>&1)
verify "$CMD" "$RES" "Permission denied" "shouldn't be able to list a secret owned by a different user"

# error on `remove` - secret owned by a different user
echo -e "$PREFIX removing a secret owned by a different user"

CMD="hzn sm secret remove -f -o ${USERDEV_ORG} -u userdevuser2:userdevuser2pw user/userdevuser1/test-password"  
RES=$($CMD 2>&1)
verify "$CMD" "$RES" "Permission denied" "shouldn't be able to remove a secret owned by a different user"

# error on `remove` - secret doesn't exist at the org level
echo -e "$PREFIX removing a secret that doesn't exist at the org level"

CMD="hzn sm secret remove -f -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} fake-password"
RES=$($CMD 2>&1)
verify "$CMD" "$RES" "nothing to remove" "shouldn't be able to remove a secret that doesn't exist"

# error on `remove` - secret doesn't exist at the user level
echo -e "$PREFIX removing a secret that doesn't exist at the user level"

CMD="hzn sm secret remove -f -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} user/userdevadmin/fake-password"
RES=$($CMD 2>&1)
verify "$CMD" "$RES" "nothing to remove" "shouldn't be able to remove a secret that doesn't exist"

# error on `read` - secret doesn't exist
echo -e "$PREFIX reading a secret that doesn't exist"

CMD="hzn sm secret read -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} fake-password"
RES=$($CMD 2>&1)
verify "$CMD" "$RES" "No secret(s) found" "shouldn't be able to read a secret that doesn't exist"

# error on `read` - user can't read org level secrets 
echo -e "$PREFIX non-admin shouldn't read org level secrets"

CMD="hzn sm secret read -o ${USERDEV_ORG} -u userdevuser1:userdevuser1pw test-password"
RES=$($CMD 2>&1)
verify "$CMD" "$RES" "Permission denied" "user shouldn't be able to read org-level secrets"

# error on `read` - user can't read another user's secrets
echo -e "$PREFIX user shouldn't read another user's secrets"

CMD="hzn sm secret read -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} user/userdevuser1/test-password"
RES=$($CMD 2>&1)
verify "$CMD" "$RES" "Permission denied" "user shouldn't be able to read another user's secrets"

# error on `add` - secret owned by a different user 
echo -e "$PREFIX adding a secret owned by a different user"

CMD="hzn sm secret add --secretKey password -d password456 -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} user/userdevuser1/fake-password"
RES=$($CMD 2>&1)
verify "$CMD" "$RES" "Permission denied" "user shouldn't be able to add to another user's secrets"

# error on `read` - bad request 
echo -e "$PREFIX passing an incorrect secret name into add"

CMD="hzn sm secret read -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} user"
RES=$($CMD 2>&1)
verify "$CMD" "$RES" "User must be specified" "shouldn't be able to create secret with incorrect name"

# error on `read` - bad request 
echo -e "$PREFIX passing an incorrect secret name into add"

CMD="hzn sm secret read -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} user/userdevadmin"
RES=$($CMD 2>&1)
verify "$CMD" "$RES" "Incorrect secret name" "shouldn't be able to create secret with incorrect name"

# error on `list` - incorrect credentials
echo -e "$PREFIX passing incorrect exchange credentials into list"

CMD="hzn sm secret list -o ${USERDEV_ORG} -u userdevfake:userdevfakepw test-password"
RES=$($CMD 2>&1)
verify "$CMD" "$RES" "Failed to authenticate" "shouldn't be able to access secrets with incorrect credentials"

# ----------------------------
# ----- CLEANUP -----
# ----------------------------
echo -e "$PREFIX starting cleanup"

# remove secrets
echo -e "$PREFIX removing org secrets"
CMD="hzn sm secret remove -f -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} test-password"
RES=$($CMD)
print_command_and_response "$CMD" "$RES"

echo -e "$PREFIX removing user secrets"
CMD="hzn sm secret remove -f -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} user/userdevuser1/test-password"
RES=$($CMD)
print_command_and_response "$CMD" "$RES"

echo -e "$PREFIX removing multi-part secrets"
CMD="hzn sm secret remove -f -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} test/password1"
RES=$($CMD)
print_command_and_response "$CMD" "$RES"

CMD="hzn sm secret remove -f -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} test/more-passwords/password2"
RES=$($CMD)
print_command_and_response "$CMD" "$RES"

# remove test user
echo -e "$PREFIX removing test users"
CMD="hzn exchange user remove -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} -f userdevuser1"
RES=$($CMD)
print_command_and_response "$CMD" "$RES"

CMD="hzn exchange user remove -o ${USERDEV_ORG} -u ${USERDEV_ADMIN_AUTH} -f userdevuser2"
RES=$($CMD)
print_command_and_response "$CMD" "$RES"

echo -e "$PREFIX complete test"



