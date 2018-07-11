
#!/bin/bash

# 1. Get org id (WIOTP_ORG_ID), api key/token (WIOTP_API_KEY/WIOTP_API_TOKEN) from bluemix console https://console.bluemix.net/
# 2. Use hzn wiotp command to create a new device type and device
# 3. Create services and pattern in the e2edev exchange
# 4. Register a pattern
# 5. Verify data using wiotp api.

export WIOTP_GW_ID=e2egwid

export WIOTP_GW_TYPE=e2egwtype
export HZN_DEVICE_ID="g@${WIOTP_GW_TYPE}@$WIOTP_GW_ID"

export WIOTP_GW_TYPE_NOCORE=e2egwtypenocore
export HZN_DEVICE_ID_NOCORE="g@${WIOTP_GW_TYPE_NOCORE}@$WIOTP_GW_ID"


export E2E_WIOTP_ORG_ADMIN="e2ewiotpadmin"
export E2E_WIOTP_ORG_ADMIN_PW="e2ewiotpadminpw"

export WIOTP_DOMAIN=internetofthings.ibmcloud.com

if [ -z "$WIOTP_ORG_ID" ]; then 
    echo -e "WIOTP_ORG_ID is not defined. Cannot do WIOTP testing. Please go to https://console.bluemix.net/ to apply an org id."
    exit 2
else
    export HZN_ORG_ID=${WIOTP_ORG_ID}
fi

if [ -z "$WIOTP_API_KEY" ] || [ -z "$WIOTP_API_TOKEN" ] ; then 
    echo -e "WIOTP_API_KEY or WIOTP_API_TOKEN is not defined. Cannot do WIOTP testing. Please go to https://console.bluemix.net/ to apply an api key and token."
    exit 2
fi

if [ -z "$WIOTP_GW_TOKEN" ]; then 
    echo -e "WIOTP_GW_TOKEN is not defined. Cannot do WIOTP testing. Please run 'export WIOTP_GW_TOKEN=mytoken' where mytoken can be any string that is hard for others to guess."
    exit 2
fi

export HZN_EXCHANGE_URL="https://${HZN_ORG_ID}.${WIOTP_DOMAIN}/api/v0002/edgenode"

# cleaning up
echo -e "Removing wiotp devices..."
hzn wiotp device remove -f $WIOTP_GW_TYPE $WIOTP_GW_ID -o $HZN_ORG_ID -A "$WIOTP_API_KEY:$WIOTP_API_TOKEN"
hzn wiotp device remove -f $WIOTP_GW_TYPE_NOCORE $WIOTP_GW_ID -o $HZN_ORG_ID -A "$WIOTP_API_KEY:$WIOTP_API_TOKEN"


echo -e "Removing wiotp types..."
hzn wiotp type remove -f $WIOTP_GW_TYPE -o $HZN_ORG_ID -A "$WIOTP_API_KEY:$WIOTP_API_TOKEN"
hzn wiotp type remove -f $WIOTP_GW_TYPE_NOCORE -o $HZN_ORG_ID -A "$WIOTP_API_KEY:$WIOTP_API_TOKEN"

# create wiotp gateway type
echo -e "Creating wiotp types..."
hzn wiotp type create $WIOTP_GW_TYPE amd64 -o $HZN_ORG_ID -A "$WIOTP_API_KEY:$WIOTP_API_TOKEN"
if [ $? -ne 0 ]
then
    echo -e "hzn wiotp type create failed."
    exit 2
fi
hzn wiotp type create $WIOTP_GW_TYPE_NOCORE amd64 -o $HZN_ORG_ID -A "$WIOTP_API_KEY:$WIOTP_API_TOKEN"
if [ $? -ne 0 ]
then
    echo -e "hzn wiotp type (no core iot)create failed."
    exit 2
fi


#create wiotp device 
echo -e "Creating wiotp devices..."
hzn wiotp device create $WIOTP_GW_TYPE $WIOTP_GW_ID $WIOTP_GW_TOKEN -o $HZN_ORG_ID -A "$WIOTP_API_KEY:$WIOTP_API_TOKEN"
if [ $? -ne 0 ]
then
    echo -e "hzn wiotp device create failed."
    export HZN_EXCHANGE_URL="http://${EXCH_APP_HOST:-172.17.0.1}:8080/v1"
    exit 2
fi
hzn wiotp device create $WIOTP_GW_TYPE_NOCORE $WIOTP_GW_ID $WIOTP_GW_TOKEN -o $HZN_ORG_ID -A "$WIOTP_API_KEY:$WIOTP_API_TOKEN"
if [ $? -ne 0 ]
then
    echo -e "hzn wiotp device (no core iot) create failed."
    export HZN_EXCHANGE_URL="http://${EXCH_APP_HOST:-172.17.0.1}:8080/v1"
    exit 2
fi

KEY_TEST_DIR="/tmp/keytest"
mkdir -p $KEY_TEST_DIR

cd $KEY_TEST_DIR
ls *.key &> /dev/null
if [ $? -eq 0 ]
then
    echo -e "Using existing key"
else
  echo -e "Generate new signing keys:"
  hzn key create -l 4096 e2edev e2edev@gmail.com
  if [ $? -ne 0 ]
  then
    echo -e "hzn key create failed."
    export HZN_EXCHANGE_URL="http://${EXCH_APP_HOST:-172.17.0.1}:8080/v1"
    exit 2
  fi
fi

echo -e "Copy public key into anax folder:"
cp $KEY_TEST_DIR/*public.pem /root/.colonus/. &> /dev/null

export HZN_EXCHANGE_URL="http://${EXCH_APP_HOST:-172.17.0.1}:8080/v1"

# Create the org in e2edev exchange for wiotp
echo "Creating $HZN_ORG_ID organization..."
read -d '' input_org <<EOF
{
  "label": "$HZN_ORG_ID",
  "description": "$HZN_ORG_ID"
}
EOF
WIOTPORG=$(echo "$input_org" | curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:Horizon-Rul3s" --data @- "${HZN_EXCHANGE_URL}/orgs/${HZN_ORG_ID}" | jq -r '.msg')
echo "$WIOTPORG"

# Creat a wiotp admin user in the exchange
echo "Creating an admin user for $HZN_ORG_ID organization..."
read -d '' input_user <<EOF
{
    "password": "$E2E_WIOTP_ORG_ADMIN_PW", 
    "email": "me%40gmail.com",
    "admin": true
}
EOF
WIOTPADM=$(echo "$input_user" | curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:Horizon-Rul3s" --data @- "${HZN_EXCHANGE_URL}/orgs/${HZN_ORG_ID}/users/${E2E_WIOTP_ORG_ADMIN}" | jq -r '.msg')
echo "$WIOTPADM"


# Create the node
echo "Creating nodes for $HZN_ORG_ID organization..."
hzn exchange node create -n ${HZN_DEVICE_ID}:${WIOTP_GW_TOKEN} -o ${HZN_ORG_ID} -u "${E2E_WIOTP_ORG_ADMIN}:${E2E_WIOTP_ORG_ADMIN_PW}" 
if [ $? -ne 0 ]
then
    echo -e "hzn node create failed."
    exit 2
fi
hzn exchange node create -n ${HZN_DEVICE_ID_NOCORE}:${WIOTP_GW_TOKEN} -o ${HZN_ORG_ID} -u "${E2E_WIOTP_ORG_ADMIN}:${E2E_WIOTP_ORG_ADMIN_PW}" 
if [ $? -ne 0 ]
then
    echo -e "hzn node create for no core iot failed."
    exit 2
fi

#######
# create cpu service
VERS="1.2.2"
cat <<EOF >$KEY_TEST_DIR/svc_cpu.json
{
    "label": "cpu for amd64",
    "description": "Provides a REST API to query the CPU load",
    "public": true,
    "url": "https://internetofthings.ibmcloud.com/services/cpu",
    "version": "$VERS",
    "arch": "amd64",
    "sharable": "single",
    "matchHardware": {},
    "requiredServices": [],
    "userInput": [],
    "deployment": {
        "services": {
            "cpu": {
                "image": "openhorizon/amd64_cpu@sha256:4e611027b757c5f779a1667e303780f5cdc75037b1ee4ce97ec3d201e81e55ed"
            }
        }
    },
    "deploymentSignature": "",
    "imageStore": {
        "storeType": "dockerRegistry"
    }
}
EOF
echo -e "Register cpu service $VERS:"
hzn exchange service publish -I -u "${E2E_WIOTP_ORG_ADMIN}:${E2E_WIOTP_ORG_ADMIN_PW}" -o ${HZN_ORG_ID} -f $KEY_TEST_DIR/svc_cpu.json -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for cpu."
    exit 1
fi


# create core-iot service
VERS="2.4.0"
cat <<EOF >$KEY_TEST_DIR/svc_core-iot.json
{
    "label": "Edge Core IoT Service",
    "description": "Images for Edge Core IoT Service",
    "public": true,
    "url": "https://internetofthings.ibmcloud.com/wiotp-edge/services/core-iot",
    "version": "$VERS",
    "arch": "amd64",
    "sharable": "single",
    "matchHardware": {},
    "requiredServices": [],
    "userInput": [
        {
          "name": "WIOTP_DEVICE_AUTH_TOKEN",
          "label": "Watson IoT Platform auth token for this gateway ID.",
          "type": "string",
          "defaultValue": ""
        },
        {
          "name": "WIOTP_DOMAIN",
          "label": "Watson IoT Platform domain: myorgid.messaging.internetofthings.ibmcloud.com",
          "type": "string",
          "defaultValue": ""
        }
    ],
    "deployment": {
        "services":{
            "edge-connector":{
                "binds":["/etc/wiotp-edge:/etc/wiotp-edge", "/var/wiotp-edge:/var/wiotp-edge"],
                "image": "wiotp-connect/edge/amd64/edge-connector:2.4.1",
                "specific_ports":[{"HostIP":"0.0.0.0","HostPort":"8883"},{"HostIP":"0.0.0.0","HostPort":"1883"}]
            },
            "edge-im":{
                "binds":["/etc/wiotp-edge:/etc/wiotp-edge","/var/wiotp-edge:/var/wiotp-edge"],
                "image":"wiotp-infomgmt/edge/amd64/edge-im:1.0.15"
            },
            "edge-mqttbroker":{
                "binds":["/etc/wiotp-edge:/etc/wiotp-edge","/var/wiotp-edge:/var/wiotp-edge"],
                "image":"wiotp-connect/edge/amd64/edge-mqttbroker:1.1.3"
            }
        }
    },
    "deploymentSignature": "",
    "imageStore": {
        "signature": "YOBo1pcnUMMTISqSl7/BRe0v3UWPqCP8i3X2tAwxk316k/pCla2OHEtcmZdG3Vl5NDGqqonHfXkowjXNc+ENy3SGAxDDfdlRICjhxnXgSfWbnpTD4PLioRxHCaYLgyqYmB56+n+QMSRmXXDyVttgl/0godYXc74PgZt1MJwVYGDzBF8GF1WFP7Az+4Tb2XyqffQefMFhyKltYa/omyjIDXhw9THoDU8u1kIIq9a4Ox+Lo97IRsnxbwXNYpkjPM50b84OPcxRrzbt6TQ3W81fKrLHo/mZc7fk13Kk4wMer1xTYZ2L1VyKC6Ctjw4iLrZwqKtK84LXZ4/nVbJr6ybmStHDi4WptxvGRWTPi1MmRtjHSn5RmLqg07SVg87hsNDD/jdoVot4ogyzkvv34xpj/Barkr+CFqOdgyr2mJ++Oq8yhCxoiZPavNLJc63fmAJI525VCD92nLcM3W/w4X9uCEXnZChHAK/HfQE0E/a4RBTmSf1WXtWEV0/MA81mBLDUVGpMmCmE0tMuOSN17p0yUE8nRit+frsTQx9TeqQAqhSUNHhaJaJwVeM0b1RpCl+JzYJGqnL9NU1ZFt7dkAPJZNkPPGUkYoJHnFOcpMEAD8bwxRK3MXNs5I2k4H7H1fGMOfFMc884LluduCJQer++6y5xoJaNAP8H4MZ91slL+1k=",
        "storeType": "imageServer",
        "url": "https://us.internetofthings.ibmcloud.com/api/v0002/horizon-image/common/6089e568370b7ecbcf3815f887d5032ae7b1907e.json"
    }
}
EOF
echo -e "Register core-iot service $VERS:"
hzn exchange service publish -I -u ibmadmin:ibmadminpw  -o IBM -f $KEY_TEST_DIR/svc_core-iot.json -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for core-iot."
    exit 1
fi

# Create cpu2wiotp service.
VERS="1.2.2"
cat <<EOF >$KEY_TEST_DIR/svc_cpu2wiotp-no-core-iot.json
{
    "label": "cpu2wiotp Directly to WIoTP for amd64",
    "description": "Sample Horizon service that repeatedly reads the CPU load and sends it directly to WIoTP",
    "public": true,
    "url": "https://internetofthings.ibmcloud.com/services/cpu2wiotp-no-core-iot",
    "version": "1.2.2",
    "arch": "amd64",
    "sharable": "multiple",
    "matchHardware": {},
    "requiredServices": [
        {
            "url": "https://internetofthings.ibmcloud.com/services/cpu",
            "org": "$HZN_ORG_ID",
            "version": "1.2.2",
            "arch": "amd64"
        }
    ],
    "userInput": [
        {
            "name": "WIOTP_GW_TOKEN",
            "label": "The token of the WIoTP gateway needed to send directly to WIoTP cloud MQTT",
            "type": "string",
            "defaultValue": ""
        },
        {
            "name": "SAMPLE_SIZE",
            "label": "the number of samples to read before calculating the average",
            "type": "int",
            "defaultValue": "6"
        },
        {
            "name": "SAMPLE_INTERVAL",
            "label": "the number of seconds between samples",
            "type": "int",
            "defaultValue": "5"
        },
        {
            "name": "MOCK",
            "label": "mock the CPU sampling",
            "type": "boolean",
            "defaultValue": "false"
        },
        {
            "name": "PUBLISH",
            "label": "publish the CPU samples to WIoTP",
            "type": "boolean",
            "defaultValue": "true"
        },
        {
            "name": "VERBOSE",
            "label": "log everything that happens",
            "type": "string",
            "defaultValue": "0"
        }
    ],
    "deployment": {
        "services": {
            "cpu2wiotp": {
                "environment":["WIOTP_DOMAIN=internetofthings.ibmcloud.com"],
                "image": "openhorizon/amd64_cpu2wiotp@sha256:024ade8d8bf2dc774bfa45fe1400526da9b302d032be39cbff54dad621ba00d6"
            }
        }
    },
    "deploymentSignature": "",
    "imageStore": {
        "storeType": "dockerRegistry"
    }
}
EOF
echo -e "Register cpu2wiotp-no-core-iot service $VERS:"
hzn exchange service publish -I -u "${E2E_WIOTP_ORG_ADMIN}:${E2E_WIOTP_ORG_ADMIN_PW}" -o ${HZN_ORG_ID} -f $KEY_TEST_DIR/svc_cpu2wiotp-no-core-iot.json -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for cpu2wiotp-no-core-iot."
    exit 1
fi


VERS="1.2.2"
cat <<EOF >$KEY_TEST_DIR/svc_cpu2wiotp.json
{
    "label": "cpu2wiotp for amd64",
    "description": "Sample Horizon service that repeatedly reads the CPU load and sends it to WIoTP",
    "public": true,
    "url": "https://internetofthings.ibmcloud.com/services/cpu2wiotp",
    "version": "1.2.2",
    "arch": "amd64",
    "sharable": "multiple",
    "matchHardware": {},
    "requiredServices": [
        {
            "url": "https://internetofthings.ibmcloud.com/services/cpu",
            "org": "$HZN_ORG_ID",
            "version": "1.2.2",
            "arch": "amd64"
        },
        {
            "url": "https://internetofthings.ibmcloud.com/wiotp-edge/services/core-iot",
            "org": "IBM",
            "version": "2.4.0",
            "arch": "amd64"
        }
      ],
    "userInput": [
        {
            "name": "SAMPLE_SIZE",
            "label": "the number of samples to read before calculating the average",
            "type": "int",
            "defaultValue": "6"
        },
        {
            "name": "SAMPLE_INTERVAL",
            "label": "the number of seconds between samples",
            "type": "int",
            "defaultValue": "5"
        },
        {
            "name": "MOCK",
            "label": "mock the CPU sampling",
            "type": "boolean",
            "defaultValue": "false"
        },
        {
            "name": "PUBLISH",
            "label": "publish the CPU samples to WIoTP",
            "type": "boolean",
            "defaultValue": "true"
        },
        {
            "name": "VERBOSE",
            "label": "log everything that happens",
            "type": "string",
            "defaultValue": "0"
        }
      ],
    "deployment": {
        "services": {
            "cpu2wiotp": {
                "binds": ["/var/wiotp-edge:/var/wiotp-edge"],
                "environment":[
                    "WIOTP_DOMAIN=internetofthings.ibmcloud.com",
                    "WIOTP_EDGE_MQTT_IP=edge-connector",
                    "WIOTP_PEM_FILE=/var/wiotp-edge/persist/dc/ca/ca.pem"
                ],
                "image": "openhorizon/amd64_cpu2wiotp@sha256:6abe3696eadf32a2dee920c1befc059302a8403293fe5a264bb945594ac0c086"
            }
        }
    },
    "deploymentSignature": "",
    "imageStore": {
        "storeType": "dockerRegistry"
    }
}
EOF
echo -e "Register cpu2wiotp service $VERS:"
hzn exchange service publish -I -u "${E2E_WIOTP_ORG_ADMIN}:${E2E_WIOTP_ORG_ADMIN_PW}" -o ${HZN_ORG_ID} -f $KEY_TEST_DIR/svc_cpu2wiotp.json -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for cpu2wiotp."
    exit 1
fi

# Create a pattern with cpu2wiotp-no-core-iot in $HZN_ORG_ID org
cat <<EOF >$KEY_TEST_DIR/${WIOTP_GW_TYPE_NOCORE}.json
{
    "label": "cpu2wiotp-no-core-iot service pattern for amd64",
    "description": "Horizon deployment pattern that runs the cpu2wiotp-no-core-iot service to send the edge node CPU info",
    "public": true,
    "workloads": [],
    "services": [
    	{
          	"serviceUrl": "https://internetofthings.ibmcloud.com/services/cpu2wiotp-no-core-iot",
          	"serviceOrgid": "$HZN_ORG_ID",
          	"serviceArch": "amd64",
          	"serviceVersions": [
            	{
              		"version": "1.2.2",
              		"deployment_overrides": {
              			"services":{
              				"cpu2wiotp":{
              				  "environment":["WIOTP_DOMAIN=internetofthings.ibmcloud.com"]
              			   }
              		    }
              	     },
              	     "deployment_overrides_signature": "",
              	     "priority": {},
              	     "upgradePolicy": {}
            	}
          	],
          	"dataVerification": {
            	"metering": {}
          	},
          	"nodeHealth": {
            	"missing_heartbeat_interval": 600,
            	"check_agreement_status": 120
          	}
        }
    ],
    "agreementProtocols": [
        {
            "name": "Basic"
        }
    ]
}
EOF
echo -e "Register ${WIOTP_GW_TYPE}noiot pattern in the exchange:"
hzn exchange pattern publish -u "${E2E_WIOTP_ORG_ADMIN}:${E2E_WIOTP_ORG_ADMIN_PW}" -o ${HZN_ORG_ID} -f $KEY_TEST_DIR/${WIOTP_GW_TYPE_NOCORE}.json -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn exchange pattern publish failed for ${WIOTP_GW_TYPE_NOCORE}."
    exit 1
fi

# Create a pattern with cpu2wiotp in $HZN_ORG_ID org
cat <<EOF >$KEY_TEST_DIR/${WIOTP_GW_TYPE}.json
{
    "label": "cpu2wiotp service pattern for amd64",
    "description": "Horizon deployment pattern that runs the cpu2wiotp service to send the edge node CPU info",
    "public": true,
    "workloads": [],
    "services": [
        {
            "serviceUrl": "https://internetofthings.ibmcloud.com/wiotp-edge/services/core-iot",
            "serviceOrgid": "IBM",
            "serviceArch": "amd64",
            "agreementLess": true,
            "serviceVersions": [
                {
                    "version": "2.4.0",
                    "deployment_overrides": "",
                    "deployment_overrides_signature": "",
                    "priority": {},
                    "upgradePolicy": {}
                }
            ],
            "dataVerification": {
                "metering": {}
            },
            "nodeHealth": {
                "missing_heartbeat_interval": 600,
                "check_agreement_status": 120
            }
        },
        {
            "serviceUrl": "https://internetofthings.ibmcloud.com/services/cpu2wiotp",
            "serviceOrgid": "$HZN_ORG_ID",
            "serviceArch": "amd64",
            "serviceVersions": [
                {
                    "version": "1.2.2",
                    "deployment_overrides": {
                        "services": {
                            "cpu2wiotp":{
                                "environment":["WIOTP_DOMAIN=internetofthings.ibmcloud.com"]
                            }
                        }
                    },
                    "deployment_overrides_signature": "",
                    "priority": {},
                    "upgradePolicy": {}
                }
            ],
            "dataVerification": {
                "metering": {}
            },
            "nodeHealth": {
                "missing_heartbeat_interval": 600,
                "check_agreement_status": 120
            }
        }
    ],
    "agreementProtocols": [
        {
            "name": "Basic"
        }
    ]
}
EOF
echo -e "Register $WIOTP_GW_TYPE pattern in the exchange:"
hzn exchange pattern publish -u "${E2E_WIOTP_ORG_ADMIN}:${E2E_WIOTP_ORG_ADMIN_PW}" -o ${HZN_ORG_ID} -f $KEY_TEST_DIR/${WIOTP_GW_TYPE}.json -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn exchange pattern publish failed for $WIOTP_GW_TYPE."
    exit 1
fi

