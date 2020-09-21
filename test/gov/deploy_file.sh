#!/bin/bash

# Deploy the input file to the file sync service. The parameters are:
# 1 - the path and file name to be deployed. The file's object id will be the filename without the path.
# 2 - the object version.
# 3 - the org into which the file should be deployed.
# 4 - the object type.
# 5 - the destination type. The type of node that can receive the file (i.e. the node's pattern). Specify "none" to leave the field unset.
# 6 - the destination id. The node's id. Specify "none" to leave the field unset.
# 7 - the object policy (optional).
# 8 - is public file

if [ ${CERT_LOC} -eq "1" ]; then
  CERT_VAR="--cacert /certs/css.crt"
else
  CERT_VAR=""
fi

DEST_TYPE=${5}
if [ "${5}" == "none" ]
then
  DEST_TYPE=""
fi

DEST_ID=${6}
if [ "${6}" == "none" ]
then
  DEST_ID=""
fi

OBJ_POLICY=${7}
if [ "${7}" == "none" ]
then
  OBJ_POLICY="null"
fi

echo "Deploying file ${1} version ${2} into ${3} as type ${4}, targetting nodes of type ${5} or node id ${6}, using policy ${7}"

FILENAME=$(basename ${1})

if [ "${OBJ_POLICY}" != "null" ]
then
  FILENAME=policy-${FILENAME}
fi

IS_PUBLIC_OBJ=false
if [ "${3}" == "IBM" ]; then
  IS_PUBLIC_OBJ=true
fi

# Setup the file sync service object metadata, based on the input parameters.
read -d '' resmeta <<EOF
{
  "data": [],
  "meta": {
  	"objectID": "${FILENAME}",
  	"objectType": "${4}",
  	"destinationID": "${DEST_ID}",
  	"destinationType": "${DEST_TYPE}",
  	"version": "${2}",
    "description": "a file",
    "destinationPolicy": ${OBJ_POLICY},
    "public": ${8}
  }
}
EOF

if [ "${3}" == "IBM" ]; then
    # deploy public object to IBM using exchange root credentials
    ADDM=$(echo "$resmeta" | curl -sLX PUT -w "%{http_code}" $CERT_VAR -u root/root:${EXCH_ROOTPW} "${CSS_URL}/api/v1/objects/${3}/${4}/${FILENAME}" --data @-)

    if [ "$ADDM" != "204" ]
    then
        echo -e "$resmeta \nPUT returned:"
        echo $ADDM
        exit -1
    fi

    ADDF=$(curl -sLX PUT -w "%{http_code}" $CERT_VAR -u root/root:${EXCH_ROOTPW} --header 'Content-Type:application/octet-stream' "${CSS_URL}/api/v1/objects/${3}/${4}/${FILENAME}/data" --data-binary @${1})

    if [ "$ADDF" == "204" ]
    then
        echo -e "Data file ${1} added successfully"
    else
        echo -e "Data file PUT returned:"
        echo $ADDF
        exit -1
    fi

else
    admin_user="${3}admin"
    admin_pw="${3}adminpw"
    if [ "${3}" == "e2edev@somecomp.com" ]; then
        admin_user="e2edevadmin"
        admin_pw="e2edevadminpw"
    fi

  ADDM=$(echo "$resmeta" | curl -sLX PUT -w "%{http_code}" $CERT_VAR -u ${3}/${admin_user}:${admin_pw} "${CSS_URL}/api/v1/objects/${3}/${4}/${FILENAME}" --data @-)

  if [ "$ADDM" != "204" ]
  then
    echo -e "$resmeta \nPUT returned:"
    echo $ADDM
    exit -1
  fi
  
  ADDF=$(curl -sLX PUT -w "%{http_code}" $CERT_VAR -u ${3}/${admin_user}:${admin_pw} --header 'Content-Type:application/octet-stream' "${CSS_URL}/api/v1/objects/${3}/${4}/${FILENAME}/data" --data-binary @${1})

  if [ "$ADDF" == "204" ]
  then
    echo -e "Data file ${1} added successfully"
  else
    echo -e "Data file PUT returned:"
    echo $ADDF
    exit -1
  fi

fi
