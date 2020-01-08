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

# When apply no flag, should get all 5 results
TARGET_NUM_OBJS=5
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
TARGET_NUM_OBJS=5
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
TARGET_NUM_OBJS=5
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
TARGET_NUM_OBJS=4
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





echo "Testing model management APIs successful"
