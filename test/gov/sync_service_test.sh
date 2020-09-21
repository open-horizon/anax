#!/bin/bash

echo "Testing model management APIs"

# Verify a response. The inputs are:
# $1 - the response
# $2 - expected result
# $3 - error message
function verify {
    respContains=$(echo $1 | grep "$2")
    if [ "${respContains}" == "" ]; then
        echo -e "\nERROR: $3. Output was:"
        echo -e "$1"
        exit 1
    fi
}

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

if [ "${EXCH_APP_HOST}" != "http://exchange-api:8080/v1" ]; then
  FILE_SIZE=128
else
  FILE_SIZE=512
fi

echo "test hzn mms cli with user:"
hzn exchange user list

#Test timeout support for upload of large files to the CSS
dd if=/dev/zero of=/tmp/data.txt count=$FILE_SIZE bs=1048576

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
  echo -e "Got unexpected error uploading 512MB/128MB (Local/Remote) model object: $RC"
  exit -1
fi

# Now, shorten the HTTP request timeout so that the upload fails. Internally, the CLI will retry
# before giving up with the appropriate HTTP Client timeout error.
export HZN_HTTP_TIMEOUT="2"

hzn mms object publish -m /tmp/meta.json -f /tmp/data.txt
RC=$?
if [ $RC -eq 5 ]
then
  echo -e "Got expected error with 512MB/128MB (Local/Remote) object upload using short HTTP request timeout: $RC"
else
  echo -e "Got unexpected error with 512MB/128MB (Local/Remote) object upload using short HTTP request timeout: $RC"
  exit -1
fi

# Reset the HTTP timeout env var to the default for the CLI.
unset HZN_HTTP_TIMEOUT

# Test object list with flags
echo "Start testing hzn mms object list"

# Adding objects
read -d '' resmeta <<EOF
{
  "objectID": "test2",
  "objectType": "test",
  "destinationType": "test",
  "destinationID": "testDestId2",
  "version": "1.0.0",
  "description": "a file",
  "expiration": "2022-10-02T15:00:00Z"
}
EOF

echo "$resmeta" > /tmp/meta.json

hzn mms object publish -m /tmp/meta.json
RC=$?
if [ $RC -ne 0 ]
then
  echo -e "Failed to publish mms object: $RC"
  exit -1
fi

read -d '' resmeta <<EOF
{
  "objectID": "test3",
  "objectType": "test",
  "destinationType": "test",
  "destinationID": "testDestId3",
  "version": "1.0.0",
  "description": "a file",
  "noData": true,
  "expiration": "2032-10-02T15:00:00Z"
}
EOF

echo "$resmeta" > /tmp/meta.json

hzn mms object publish -m /tmp/meta.json
RC=$?
if [ $RC -ne 0 ]
then
  echo -e "Failed to publish mms object: $RC"
  exit -1
fi

# adding an object with data for MMS access testing
read -d '' resmeta <<EOF
{
  "objectID": "test_user_access",
  "objectType": "test",
  "destinationType": "",
  "destinationID": "",
  "version": "1.0.0",
  "description": "test acl",
  "expiration": "2032-10-02T15:00:00Z"
}
EOF

echo "$resmeta" > /tmp/meta.json


hzn mms object publish -m /tmp/meta.json -f /tmp/data.txt
RC=$?
if [ $RC -ne 0 ]
then
  echo -e "Failed to publish mms object: $RC"
  exit -1
fi

echo "Start testing hzn mms object list for user "

# When apply no flag, should get all 6 results
TARGET_NUM_OBJS=6
OBJS_CMD=$(hzn mms object list | awk '{if(NR>1)print}')
NUM_OBJS=$(echo $OBJS_CMD | jq '. | length')
if [ "${TARGET_NUM_OBJS}" != "${NUM_OBJS}" ]
then
  echo -e "Got unexpected number of objects when listing all objects"
  exit -1
fi

for (( ix=0; ix<$NUM_OBJS; ix++ ))
do
  if [ $(echo $OBJS_CMD | jq -r '.['${ix}'].instanceID') != null ]; then
    echo -e "Got unexpected field listing without -l"
    exit -1
  fi
done

# -l
TARGET_NUM_OBJS=6
OBJS_CMD=$(hzn mms object list -l | awk '{if(NR>1)print}')
NUM_OBJS=$(echo $OBJS_CMD | jq '. | length')
if [ "${TARGET_NUM_OBJS}" != "${NUM_OBJS}" ]
then
  echo -e "Got unexpected number of objects listing all objects with -l"
  exit -1
fi

for (( ix=0; ix<$NUM_OBJS; ix++ ))
do
  if [ $(echo $OBJS_CMD | jq -r '.['${ix}'].instanceID') == null ]; then
    echo -e "Got unexpected field listing with -l"
    exit -1
  fi
done

# -d
TARGET_NUM_OBJS=6
OBJS_CMD=$(hzn mms object list -d | awk '{if(NR>1)print}')
NUM_OBJS=$(echo $OBJS_CMD | jq '. | length')
if [ "${TARGET_NUM_OBJS}" != "${NUM_OBJS}" ]
then
  echo -e "Got unexpected number of objects listing all objects with -d"
  exit -1
fi

for (( ix=0; ix<$NUM_OBJS; ix++ ))
do
  if [ $(echo $OBJS_CMD | jq -r '.['${ix}'].objectStatus') == null ]; then
    echo -e "Got unexpected field listing with -l"
    exit -1
  fi
done

if [ "${TEST_PATTERNS}" != "" ]
then
  OBJECT_ID="basicres.tgz"

else
  OBJECT_ID="policy-basicres.tgz"
fi
OBJECT_TYPE=model
WRONG_OBJECT_ID=test1

# --objectType
TARGET_NUM_OBJS=2
OBJS_CMD=$(hzn mms object list --objectType=${OBJECT_TYPE} | awk '{if(NR>1)print}')
NUM_OBJS=$(echo $OBJS_CMD | jq '. | length')
if [ "${TARGET_NUM_OBJS}" != "${NUM_OBJS}" ]
then
  echo -e "Got unexpected number of objects with --objectType"
  exit -1
fi

# --objectType --objectId
TARGET_NUM_OBJS=1
OBJS_CMD=$(hzn mms object list --objectType=${OBJECT_TYPE} --objectId=${OBJECT_ID} | awk '{if(NR>1)print}')
NUM_OBJS=$(echo $OBJS_CMD | jq '. | length')
if [ "${TARGET_NUM_OBJS}" != "${NUM_OBJS}" ]; then
  echo -e "Got unexpected number of objects using with --objectType and --objectId"
  exit -1
fi

if [ $(echo ${OBJS_CMD} | jq -r '.[0].objectID') != ${OBJECT_ID} ] && [ $(echo ${OBJS_CMD} | jq -r '.[0].objectType') != ${OBJECT_TYPE} ]; then
  echo -e "Got unexpected objects listing with --objectType and --objectId"
  exit -1
fi

# list with wrong objectId
hzn mms object list --objectType=${OBJECT_TYPE} --objectId=${WRONG_OBJECT_ID}
RC=$?
if [ $RC -eq 0 ]; then
  echo -e "Should return error message when list with wrong objectId"
  exit -1
fi

if [ "${TEST_PATTERNS}" != "" ]
then
  # pattern case
  # --destinationType
  DEST_TYPE=test
  DEST_ID=testDestId2
  WRONG_DEST_TYPE=wrongDestType
  WRONG_DEST_ID=wrongDestId

  TARGET_NUM_OBJS=3
  OBJS_CMD=$(hzn mms object list --destinationType=${DEST_TYPE} | awk '{if(NR>1)print}')
  NUM_OBJS=$(echo $OBJS_CMD | jq '. | length')
  if [ "${TARGET_NUM_OBJS}" != "${NUM_OBJS}" ]; then
    echo -e "Got unexpected number of objects listing with --destinationType"
    exit -1
  fi

  # --destinationType --destinationId
  TARGET_NUM_OBJS=1
  OBJS_CMD=$(hzn mms object list --destinationType=${DEST_TYPE} --destinationId=${DEST_ID} | awk '{if(NR>1)print}')
  NUM_OBJS=$(echo $OBJS_CMD | jq '. | length')
  if [ "${TARGET_NUM_OBJS}" != "${NUM_OBJS}" ]; then
    echo -e "Got unexpected number of objects listing with --destinationType and --destinationId"
    exit -1
  fi

  # list with wrong destinationType
  hzn mms object list --destinationType=${WRONG_DEST_TYPE}
  if [ $? -eq 0 ]; then
    echo -e "Should return error message when list with wrong destinationType"
    exit -1
  fi

  # list destinationId only
  hzn mms object list --destinationId=${DEST_ID}
  if [ $? -eq 0 ]; then
    echo -e "Should return error message when list with destinationId only"
    exit -1
  fi

  # list with wrong destinationId
  hzn mms object list --destinationType=${DEST_TYPE} --destinationId=${WRONG_DEST_ID}
  if [ $? -eq 0 ]; then
    echo -e "Should return error message when list with wrong destinationId"
    exit -1
  fi

  # hzn mms object list --policy should not return any objects
  hzn mms object list --policy=true
  if [ $? -eq 0 ]; then
    echo -e "Should return error message when list with --policy when TEST_PATTERN is not empty"
    exit -1
  fi

else
  # policy case
  # hzn mms object list --policy
  TARGET_NUM_OBJS=2
  OBJS_CMD=$(hzn mms object list --policy=true | awk '{if(NR>1)print}')
  NUM_OBJS=$(echo $OBJS_CMD | jq '. | length')
  if [ "${TARGET_NUM_OBJS}" != "${NUM_OBJS}" ]; then
    echo -e "Got unexpected number of objects listing with --policy=true"
    exit -1
  fi

  # --property
  TARGET_NUM_OBJS=1
  PROP_NAME=prop_name1
  RESULT_OBJ_ID="policy-basicres.tgz"
  OBJS_CMD=$(hzn mms object list --property=${PROP_NAME} | awk '{if(NR>1)print}')
  NUM_OBJS=$(echo $OBJS_CMD | jq '. | length')
  if [ "${TARGET_NUM_OBJS}" != "${NUM_OBJS}" ]; then
    echo -e "Got unexpected number of objects listing with --property"
    exit -1
  fi

  if [ $(echo $OBJS_CMD | jq -r '.[0].objectID') != ${RESULT_OBJ_ID} ]; then
    echo -e "Got unexpected objects listing with --property"
    exit -1
  fi

  # --service
  TARGET_NUM_OBJS=2
  SERV_NAME="${RESOURCE_ORG1}/my.company.com.services.usehello2"
  WRONG_SERV_NAME=wrong_serve_name
  WRONGFMT_SERV_NAME="my.company.com.services.usehello2"

  OBJS_CMD=$(hzn mms object list --service=${SERV_NAME} | awk '{if(NR>1)print}')
  NUM_OBJS=$(echo $OBJS_CMD | jq '. | length')
  if [ "${TARGET_NUM_OBJS}" != "${NUM_OBJS}" ]; then
    echo -e "Got unexpected number of objects listing with --service"
    exit -1
  fi

  hzn mms object list --service=${WRONG_SERV_NAME}
  if [ $? -eq 0 ]; then
    echo -e "Should return error message when list with wrong destination policy service"
    exit -1
  fi

  hzn mms object list --service=${WRONGFMT_SERV_NAME}
  if [ $? -eq 0 ]; then
    echo -e "Should return error message when list with destination policy service in wrong format"
    exit -1
  fi

  # --updateTime
  TARGET_NUM_OBJS=2
  UPDATE_TIME="2000-01-01T03:00:00Z"
  OBJS_CMD=$(hzn mms object list --updateTime=${UPDATE_TIME} | awk '{if(NR>1)print}')
  NUM_OBJS=$(echo $OBJS_CMD | jq '. | length')
  if [ "${TARGET_NUM_OBJS}" != "${NUM_OBJS}" ]; then
    echo -e "Got unexpected number of objects listing with --updateTime, should get ${TARGET_NUM_OBJS} object(s)"
    exit -1
  fi

  UPDATE_TIME="2000-01-01"
  OBJS_CMD=$(hzn mms object list --updateTime=${UPDATE_TIME} | awk '{if(NR>1)print}')
  NUM_OBJS=$(echo $OBJS_CMD | jq '. | length')
  if [ "${TARGET_NUM_OBJS}" != "${NUM_OBJS}" ]; then
    echo -e "Got unexpected number of objects listing with --updateTime, should get ${TARGET_NUM_OBJS} object(s)"
    exit -1
  fi

  UPDATE_TIME="2040-01-01T03:00:00Z"
  hzn mms object list --updateTime=${UPDATE_TIME}
  if [ $? -eq 0 ]; then
    echo -e "Should return error message when list with wrong updateTime"
    exit -1
  fi

  WRONGFMT_UPDATE_TIME="20000101T030000Z"
  hzn mms object list --updateTime=${WRONGFMT_UPDATE_TIME}
  if [ $? -eq 0 ]; then
    echo -e "Should return error message when list with updateTime in wrong format"
    exit -1
  fi

  # --property --service --updateTime
  TARGET_NUM_OBJS=1
  PROP_NAME=prop_name1
  SERV_NAME="${RESOURCE_ORG1}/my.company.com.services.usehello2"
  UPDATE_TIME="2000-01-01T03:00:00Z"
  RESULT_OBJ_ID="policy-basicres.tgz"
  OBJS_CMD=$(hzn mms object list --policy=true --property=${PROP_NAME} --service=${SERV_NAME} --updateTime=${UPDATE_TIME} | awk '{if(NR>1)print}')
  NUM_OBJS=$(echo $OBJS_CMD | jq '. | length')
  if [ "${TARGET_NUM_OBJS}" != "${NUM_OBJS}" ]; then
    echo -e "Got unexpected number of objects listing with --policy, --property, --service, and --updateTime"
    exit -1
  fi

  if [ $(echo $OBJS_CMD | jq -r '.[0].objectID') != ${RESULT_OBJ_ID} ]; then
    echo -e "Got unexpected objects listing with --policy, --property, --service, and --updateTime"
    exit -1
  fi

  # --property --service --updateTime without setting --policy
  OBJS_CMD=$(hzn mms object list --property=${PROP_NAME} --service=${SERV_NAME} --updateTime=${UPDATE_TIME} | awk '{if(NR>1)print}')
  NUM_OBJS=$(echo $OBJS_CMD | jq '. | length')
  if [ "${TARGET_NUM_OBJS}" != "${NUM_OBJS}" ]; then
    echo -e "Got unexpected number of objects when specify --property --service --updateTime without setting --policy"
    exit -1
  fi
fi

# --data=false
OBJS_CMD=$(hzn mms object list --data=false | awk '{if(NR>1)print}')
NUM_OBJS=$(echo $OBJS_CMD | jq '. | length')
RESULT_OBJ_ID="test3"
if [ "${TARGET_NUM_OBJS}" != "${NUM_OBJS}" ]; then
  echo -e "Got unexpected number of objects listing with --data=false"
  exit -1
fi

if [ $(echo $OBJS_CMD | jq -r '.[0].objectID') != ${RESULT_OBJ_ID} ]; then
  echo -e "Got unexpected objects listing with --data=false"
  exit -1
fi

# --data=true
TARGET_NUM_OBJS=5
OBJS_CMD=$(hzn mms object list --data=true | awk '{if(NR>1)print}')
NUM_OBJS=$(echo $OBJS_CMD | jq '. | length')
RESULT_OBJ_ID="test3"
if [ "${TARGET_NUM_OBJS}" != "${NUM_OBJS}" ]; then
  echo -e "Got unexpected number of objects listing with --data=true"
  exit -1
fi

for (( ix=0; ix<$NUM_OBJS; ix++ ))
do
  if [ $(echo $OBJS_CMD | jq -r '.['${ix}'].objectID') == ${RESULT_OBJ_ID} ]; then
    echo -e "Got unexpected object listing with --data=true"
    exit -1
  fi
done

# --expirationTime
TARGET_NUM_OBJS=1
EXP_TIME_BEFORE="2025-10-02T15:00:00Z"
OBJS_CMD=$(hzn mms object list --expirationTime=${EXP_TIME_BEFORE} | awk '{if(NR>1)print}')
NUM_OBJS=$(echo $OBJS_CMD | jq '. | length')
RESULT_OBJ_ID="test2"
if [ "${TARGET_NUM_OBJS}" != "${NUM_OBJS}" ]; then
  echo -e "Got unexpected number of objects listing with --expirationTime"
  exit -1
fi

if [ $(echo $OBJS_CMD | jq -r '.[0].objectID') != ${RESULT_OBJ_ID} ]; then
  echo -e "Got unexpected objects listing with --expirationTime"
  exit -1
fi

WRONGFMT_EXP_TIME_BEFORE="20251002T150000Z"
hzn mms object list --expirationTime=${WRONGFMT_EXP_TIME_BEFORE}
if [ $? -eq 0 ]; then
    echo -e "Should return error message when list with --expirationTime in wrong format"
    exit -1
fi

echo "Testing MMS ACL object access"

HZN_ORG_ID_BEFORE_MODIFY=$HZN_ORG_ID
HZN_EX_USER_AUTH_BEFORE_MODIFY=$HZN_EXCHANGE_USER_AUTH

# test user/node should have READ and WRITE access to all object types in their org
# $1 - USER_ORG
# $2 - USER_REG_USERNAME
# $3 - USER_REG_USERPWD
# $4 - Expected number of object returned by list object cli
# $5 - Object Type to download
# $6 - Object ID to download
# $7 - Object Type to publish
# $8 - Object ID to publish
function testUserHaveAccessToALLObjects {
    echo "Testing MMS ACL access in same org for user/node ${1}/${2}"

    USER_REG_USER_AUTH="${1}/${2}:${3}"
    export HZN_EXCHANGE_USER_AUTH=${USER_REG_USER_AUTH}
    export HZN_ORG_ID=${1}

    # list
    OBJS_CMD=$(hzn mms object list | awk '{if(NR>1)print}')
    NUM_OBJS=$(echo $OBJS_CMD | jq '. | length')
    if [ "${4}" != "${NUM_OBJS}" ]
    then
        echo -e "Got unexpected number of objects when listing all objects for user ${USER_REG_USERNAME} in org ${USER_ORG}"
        exit -1
    fi

    # download
    echo "user ${2} is dowloading object: ${5} ${6}"
    
    DOWNLOADED_FILE="${5}_${6}"
    if [ -f "$DOWNLOADED_FILE" ]; then
        echo "$DOWNLOADED_FILE already exists. Deleted before downloading..."
        rm -f $DOWNLOADED_FILE
	if [ $? -ne 0 ]; then
	    echo -e "Failed to remove $DOWNLOADED_FILE"
	    exit -1
	fi
    fi

    resp=$(hzn mms object download -t ${5} -i ${6} 2>&1)
    respContains=$(echo $resp | grep "Unauthorized")
    if [ "${respContains}" != "" ]; then
        echo -e "\nERROR: Failed to download mms object ${5} ${6} for user ${2}. Output was:"
        echo -e "$resp"
        exit 1
    fi

    # publish
    # have access to update object in user's org
    read -d '' resmeta <<EOF
    {
      "objectID": "${8}",
      "objectType": "${7}",
      "destinationType": "",
      "destinationID": "",
      "version": "1.0.0",
      "description": "test write access",
      "expiration": "2032-10-02T15:00:00Z"
    }
EOF

    echo "$resmeta" > /tmp/meta.json
    hzn mms object publish -m /tmp/meta.json -f /tmp/data.txt
    RC=$?
    if [ $RC -ne 0 ]
    then
        echo -e "Failed to publish mms object ${7} ${8} by user ${2} in the org ${1}: $RC"
        exit -1
    fi
}

# test user/node who has Write access to some object types
# $1 - USER_ORG
# $2 - USER_REG_USERNAME
# $3 - USER_REG_USERPWD
# $4 - Expected number of object returned by list object cli
# $5 - Object Type that user doesn't have access
# $6 - Object ID that user doesn't have access
function testUserNotHaveAccessToPrivateObjects {
    echo "Testing MMS ACL access for ${2} in ${1} org"
    USER_REG_USER_AUTH="${1}/${2}:${3}"
    export HZN_EXCHANGE_USER_AUTH=${USER_REG_USER_AUTH}
    export HZN_ORG_ID=${1}

    # list
    hzn mms object list
    OBJS_CMD=$(hzn mms object list | awk '{if(NR>1)print}')
    NUM_OBJS=$(echo $OBJS_CMD | jq '. | length')
    if [ "${4}" != "${NUM_OBJS}" ]
    then
        echo -e "Got unexpected number of objects when listing all objects for ${2} in org ${1}"
        exit -1
    fi

    # don't have access to get private object
    echo "user ${2} is dowloading object: ${5} ${6}"
    resp=$(hzn mms object download -t ${5} -i ${6} 2>&1)
    verify "$resp" "Unauthorized" "User ${2} should not have access to download mms object ${5} ${6}"

     # don't have access to update private object
    echo "user ${2} is publishing object: ${5} ${6}"
    read -d '' resmeta <<EOF
    {
      "objectID": "${6}",
      "objectType": "${5}",
      "destinationType": "test",
      "version": "1.0.0",
      "description": "test acl - test update by user"
    }
EOF

    echo "$resmeta" > /tmp/meta.json
    resp=$(hzn mms object publish -m /tmp/meta.json -f /tmp/data.txt 2>&1)
    verify "$resp" "Unauthorized" "Got unexpected error with updating object in object type ${5} by ${2}"

}


# test user/node can only GET public object, but can't update object
# $1 - USER_ORG
# $2 - USER_REG_USERNAME
# $3 - USER_REG_USERPWD
# $4 - Org of public object
# $5 - Object Type of the public object
# $6 - Object ID of the public object
function verifyUserAccessForPublicObject {
    echo "Verify user $1/$2 has READ access to public object in $4 org"

    # user can get object metadata and object data
    # Test what happens when an unknown user id is attempted
    GET_OBJ_CODE=$(curl -o -IL -s -X GET -w "%{http_code}" $CERT_VAR -u ${1}/${2}:${3} --header 'Content-Type: application/json' "${CSS_URL}/api/v1/objects/${4}/${5}/${6}")
    echo "GET_OBJ_CODE: $GET_OBJ_CODE"
    if [ "$GET_OBJ_CODE" != "200" ]
    then
        echo -e "Error testing CSS API with get public object, should have received 200, received $GET_OBJ_CODE"
        exit -1
    fi

    GET_OBJ_DATA_CODE=$(curl -o -IL -s -X GET -w "%{http_code}" $CERT_VAR -u ${1}/${2}:${3} --header 'Content-Type:application/octet-stream' "${CSS_URL}/api/v1/objects/${4}/${5}/${6}/data")
    echo "GET_OBJ_DATA_CODE: $GET_OBJ_DATA_CODE"
    if [ "$GET_OBJ_DATA_CODE" != "200" ]
    then
        echo -e "Error testing CSS API with get public object data, should have received 200, received $GET_OBJ_DATA_CODE"
        exit -1
    fi

    echo "Verify user $1/$2 doesn't have WRITE access to public object in $4 org"
    # user can't update object metadata or object data
read -d '' resmeta <<EOF
{
  "data": [],
  "meta": {
  	"objectID": "${6}",
  	"objectType": "${5}",
  	"destinationID": "",
  	"destinationType": "",
  	"version": "2.0.0",
    "description": "test update public object",
    "public": true
  }
}
EOF

    ADDM=$(echo "$resmeta" | curl -sLX PUT -w "%{http_code}" $CERT_VAR -u ${1}/${2}:${3} "${CSS_URL}/api/v1/objects/${4}/${5}/${6}" --data @-)
    echo "PUT_OBJ_CODE: $ADDM"
    if [ "$ADDM" == "204" ]
    then
        echo -e "$resmeta \nPUT returned:"
        echo $ADDM
        echo -e "Public object should not be updated by user $2 in org $1"
        exit -1
    fi

    DATA=/tmp/data.txt

    ADDM=$(echo "$resmeta" | curl -sLX PUT -w "%{http_code}" $CERT_VAR -u ${1}/${2}:${3} "${CSS_URL}/api/v1/objects/${4}/${5}/${6}/data" --data-binary @${DATA})
    echo "PUT_OBJ_DATA_CODE: $ADDM"
    if [ "$ADDM" == "204" ]
    then
        echo -e "$resmeta \nPUT returned:"
        echo $ADDM
        echo -e "Public object data should not be updated by user $2 in org $1"
        exit -1
    fi
}

# test admin user can create public object
# $1 - USER_ORG
# $2 - USER_REG_USERNAME
# $3 - USER_REG_USERPWD
# $4 - Org of public object
function verifyAdminUserCanCreatePublicObject {
    echo "Verify user $1/$2 has WRITE access to public object in $4 org"

    USER_REG_USER_AUTH="${1}/${2}:${3}"
    export HZN_EXCHANGE_USER_AUTH=${USER_REG_USER_AUTH}
    export HZN_ORG_ID=${1}

    # hub admin can create public object in anyorg
read -d '' resmeta <<EOF
{
  "objectID": "public_obj",
  "objectType": "public",
  "destinationOrgID": "${4}",
  "destinationID": "",
  "destinationType": "",
  "version": "2.0.0",
  "description": "test update public object by user ${2} in org ${1}",
  "public": true
}
EOF

    echo "$resmeta" > /tmp/meta.json
    hzn mms object publish -o ${4} -m /tmp/meta.json -f /tmp/data.txt
    RC=$?
    if [ $RC -ne 0 ]
    then
        echo -e "Failed to publish mms object by user ${2} in the org ${1}: $RC"
        exit -1
    fi

}

PUBLIC_OBJ_ORG="IBM"
PUBLIC_OBJ_TYPE="public"
PUBLIC_OBJ_ID="public.tgz"

#e2edev@somecomp.com/anax1 has READ and WRITE access to all object types in e2edev@somecomp.com org
USER_ORG="e2edev@somecomp.com"
USER_REG_USERNAME="anax1"
USER_REG_USERPWD="anax1pw"
TARGET_NUM_OBJS=6
testUserHaveAccessToALLObjects $USER_ORG $USER_REG_USERNAME $USER_REG_USERPWD $TARGET_NUM_OBJS $OBJECT_TYPE $OBJECT_ID "test" "test_user_access"
verifyUserAccessForPublicObject $USER_ORG $USER_REG_USERNAME $USER_REG_USERPWD $PUBLIC_OBJ_ORG $PUBLIC_OBJ_TYPE $PUBLIC_OBJ_ID

# node e2edev@somecomp.com/an12345 has READ and WRITE access to all object types in e2edev@somecomp.com org
NODE_ID="an12345"
NODE_TOKEN="abcdefg"
TARGET_NUM_OBJS=0
testUserNotHaveAccessToPrivateObjects  $USER_ORG $NODE_ID $NODE_TOKEN $TARGET_NUM_OBJS "test" "test_user_access"
verifyUserAccessForPublicObject $USER_ORG $NODE_ID $NODE_TOKEN $PUBLIC_OBJ_ORG $PUBLIC_OBJ_TYPE $PUBLIC_OBJ_ID

# root/hubadmin should be able to create object in IBM org
USER_ORG="root"
USER_REG_USERNAME="hubadmin"
USER_REG_USERPWD="hubadminpw"
verifyAdminUserCanCreatePublicObject $USER_ORG $USER_REG_USERNAME $USER_REG_USERPWD $PUBLIC_OBJ_ORG

# ibm org admin should be able to create object in IBM org
USER_ORG="IBM"
USER_REG_USERNAME="ibmadmin"
USER_REG_USERPWD="ibmadminpw"
verifyAdminUserCanCreatePublicObject $USER_ORG $USER_REG_USERNAME $USER_REG_USERPWD $PUBLIC_OBJ_ORG

# set back to the value before sync service testing
export HZN_ORG_ID=${HZN_ORG_ID_BEFORE_MODIFY}
export HZN_EXCHANGE_USER_AUTH=${HZN_EX_USER_AUTH_BEFORE_MODIFY}

echo "Testing model management APIs successful"
