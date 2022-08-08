#!/bin/bash

if [ "$HA" != "1" ]; then
    echo "Skipping $0"
    exit
fi

export HZN_EXCHANGE_URL="${EXCH_APP_HOST}"
USERDEV_ADMIN_AUTH="userdev/userdevadmin:userdevadminpw"
E2EDEV_ADMIN_AUTH="e2edev@somecomp.com/e2edevadmin:e2edevadminpw" 
IBM_ADMIN_AUTH="IBM/ibmadmin:ibmadminpw"
ORG="userdev"

PREFIX="HA test:"


function verify_ha_group_name {
    node=$1
    local_port=$2

    echo -e "\n${PREFIX} checking local node $node on port $local_port..."
    res=$(HORIZON_URL=http://localhost:${local_port} hzn node list)
    node_id=$(echo "$res" | jq -r '.id')
    ha_group_name=$(echo "$res" | jq -r '.ha_group')

    if [ "$node_id" != "$node" ]; then
        echo -e "\n${PREFIX} the local node id should be ${node} but got: ${node_id}."
        exit 2
    fi

    if [ "$ha_group_name" != "group1" ]; then
        echo -e "\n${PREFIX} the HA group name for node ${node} should be group1 but got: ${ha_group_name}."
        exit 2
    fi


    echo -e "\n${PREFIX} checking exchange node ${ORG}/${node} ..."
    res=$(hzn exchange node list -o $ORG -u $USERDEV_ADMIN_AUTH ${node})
    ha_group_name=$(echo "$res" | jq -r ".\"${ORG}/${node}\".ha_group")

    if [ "$ha_group_name" != "group1" ]; then
        echo -e "\n${PREFIX} the HA group name for node ${node} should be group1 but got: ${ha_group_name}."
        exit 2
    fi
}

function publish_new_netspeed_service {
    echo -e "\n${PREFIX} publish netspeed service 2.4.0..."
    read -d '' sdef <<EOF
{
    "label": "Netspeed for amd64",
    "description": "Netspeed service",
    "documentation": "",
    "public": true,
    "url": "https://bluehorizon.network/services/netspeed",
    "version": "2.4.0",
    "arch": "amd64",
    "sharable": "multiple",
    "matchHardware": {},
    "requiredServices": [
      {
        "url": "https://bluehorizon.network/services/network",
        "org": "IBM",
        "versionRange": "1.0.0",
        "arch": "amd64"
      },
      {
        "url": "https://bluehorizon.network/services/network2",
        "org": "IBM",
        "versionRange": "1.0.0",
        "arch": "amd64"
      },
      {
        "url": "https://bluehorizon.network/service-cpu",
        "org": "IBM",
        "versionRange": "1.0.0",
        "arch": "amd64"
      }
    ],
    "userInput": [
      {
        "name": "var1",
        "label": "",
        "type": "string",
        "defaultValue": ""
      },
      {
        "name": "var2",
        "label": "",
        "type": "int",
        "defaultValue": ""
      },
      {
        "name": "var3",
        "label": "",
        "type": "float",
        "defaultValue": ""
      },
      {
        "name": "var4",
        "label": "",
        "type": "list of strings",
        "defaultValue": ""
      },
      {
        "name": "var5",
        "label": "",
        "type": "string",
        "defaultValue": "default"
      },
      {
        "name": "var6",
        "label": "",
        "type": "string",
        "defaultValue": "default"
      }
    ],
    "deployment": {
        "services":{
            "netspeed":{
                "image":"openhorizon/example_ms_x86_cpu:1.2.2",
                "secrets":{
                    "sec3":{
                        "description":"Secret 3 for IBM netspeed."
                    }
                }
            }
        }
    }
}
EOF
    res=$(echo "$sdef" | hzn exchange service publish -f- -O -P -o IBM -u ${IBM_ADMIN_AUTH} 2>&1)
    if [ $? -ne 0 ]; then
        echo -e "\n${PREFIX} failed to create netspeed service version 2.4.0. $res"
        exit 2
    fi 
}

function update_sns_pattern {
    echo -e "\n${PREFIX} updating pattern sns with netspeed service 2.4.0..."
    read -d '' sns <<EOF
{
    "label": "Netspeed",
    "description": "a netspeed service based pattern",
    "public": true,
    "services": [
      {
        "serviceUrl": "https://bluehorizon.network/services/netspeed",
        "serviceOrgid": "IBM",
        "serviceArch": "amd64",
        "serviceVersions": [
          {
            "version": "2.3.0",
            "priority": {
              "priority_value": 3,
              "retries": 1,
              "retry_durations": 1800,
              "verified_durations": 45
            },
            "upgradePolicy": {},
            "deployment_overrides": "",
            "deployment_overrides_signature": ""
          },
          {
            "version": "2.4.0",
            "priority": {
              "priority_value": 1,
              "retries": 1,
              "retry_durations": 3600
            },
            "upgradePolicy": {},
            "deployment_overrides": "",
            "deployment_overrides_signature": ""
          }
        ],
        "dataVerification": {
          "metering": {}
        },
        "nodeHealth": {
          "missing_heartbeat_interval": 120,
          "check_agreement_status": 30
        },
        "agreementLess": false
      }
    ],
    "agreementProtocols": [
      {
        "name": "Basic"
      }
    ],
    "userInput": [
      {
        "serviceOrgid": "IBM",
        "serviceUrl": "https://bluehorizon.network/services/netspeed",
        "serviceVersionRange": "2.2.0",
        "inputs": [
          {
            "name": "var1",
            "value": "bString"
          },
          {
            "name": "var2",
            "value": 10
          },
          {
            "name": "var3",
            "value": 10.22
          },
          {
            "name": "var4",
            "value": [
              "abcd",
              "1234"
            ]
          },
          {
            "name": "var5",
            "value": "override2"
          }
        ]
      },
      {
        "serviceOrgid": "IBM",
        "serviceUrl": "https://bluehorizon.network/service-cpu",
        "serviceVersionRange": "1.0.0",
        "inputs": [
          {
            "name": "cpu_var1",
            "value": "ibm_var1"
          }
        ]
      }
    ],
    "secretBinding": [
      {
        "serviceOrgid": "IBM",
        "serviceUrl": "https://bluehorizon.network/services/netspeed",
        "serviceVersionRange": "2.2.0",
        "secrets": [
          {
            "sec3": "netspeed-secret3"
          }
        ]
      },
      {
        "serviceOrgid": "IBM",
        "serviceUrl": "https://bluehorizon.network/service-cpu",
        "serviceVersionRange": "1.2.2",
        "secrets": [
          {
            "secret-dep1": "netspeed-secret1"
          }
        ]
      },
      {
        "serviceOrgid": "e2edev@somecomp.com",
        "serviceUrl": "https://bluehorizon.network/service-cpu",
        "serviceVersionRange": "1.0.0",
        "secrets": [
          {
            "secret-dep2": "netspeed-secret2"
          }
        ]
      }
    ]
}
EOF
    res=$(echo "$sns" | hzn exchange pattern publish -f- -p sns -o e2edev@somecomp.com -u ${E2EDEV_ADMIN_AUTH} 2>&1)
    if [ $? -ne 0 ]; then
        echo -e "\n${PREFIX} failed to update pattern sns with netspeed service 2.4.0. $res"
        exit 2
    fi 
}

function update_ns_policy {
    echo -e "\n${PREFIX} updating deployment policy bp_netspeed with netspeed service 2.4.0..."
    read -d '' bp_ns <<EOF
  {
    "label": "business policy for netspeed",
    "description": "for netspeed",
    "service": {
      "name": "https://bluehorizon.network/services/netspeed",
      "org": "IBM",
      "arch": "*",
      "serviceVersions": [
        {
          "version": "2.3.0",
          "priority": {
            "priority_value": 3,
            "retries": 1,
            "retry_durations": 1800,
            "verified_durations": 45
          },
          "upgradePolicy": {}
        },
        {
          "version": "2.4.0",
          "priority": {
            "priority_value": 1,
            "retries": 1,
            "retry_durations": 3600
          },
          "upgradePolicy": {}
        }
 
      ],
      "nodeHealth": {}
    },
    "properties": [
      {
        "name": "iame2edev",
        "value": "true"
      },
      {
        "name": "NONS",
        "value": false
      },
      {
        "name": "number",
        "value": "12"
      },
      {
        "name": "foo",
        "value": "bar"
      }
    ],
    "constraints": [
      "purpose==network-testing"
    ],
    "userInput": [
      {
        "serviceOrgid": "IBM",
        "serviceUrl": "https://bluehorizon.network/services/netspeed",
        "serviceVersionRange": "2.2.0",
        "inputs": [
          {
            "name": "var1",
            "value": "bString"
          },
          {
            "name": "var2",
            "value": 10
          },
          {
            "name": "var3",
            "value": 10.22
          },
          {
            "name": "var4",
            "value": [
              "abcd",
              "1234"
            ]
          },
          {
            "name": "var5",
            "value": "override2"
          }
        ]
      },
      {
        "serviceOrgid": "IBM",
        "serviceUrl": "https://bluehorizon.network/service-cpu",
        "serviceVersionRange": "1.0.0",
        "inputs": [
          {
            "name": "cpu_var1",
            "value": "ibm_var1"
          }
        ]
      }
    ],
    "secretBinding": [
      {
        "serviceOrgid": "IBM",
        "serviceUrl": "https://bluehorizon.network/services/netspeed",
        "serviceVersionRange": "2.2.0",
        "secrets": [
          {
            "sec3": "netspeed-secret3"
          }
        ]
      },
      {
        "serviceOrgid": "IBM",
        "serviceUrl": "https://bluehorizon.network/service-cpu",
        "serviceVersionRange": "1.2.2",
        "secrets": [
          {
            "secret-dep1": "netspeed-secret1"
          }
        ]
      },
      {
        "serviceOrgid": "e2edev@somecomp.com",
        "serviceUrl": "https://bluehorizon.network/service-cpu",
        "serviceVersionRange": "1.0.0",
        "secrets": [
          {
            "secret-dep2": "netspeed-secret2"
          }
        ]
      }
    ]
  }
EOF
    res=$(echo "$bp_ns" | hzn exchange deployment addpolicy -f- -o userdev -u ${USERDEV_ADMIN_AUTH} bp_netspeed 2>&1)
    if [ $? -ne 0 ]; then
        echo -e "\n${PREFIX} failed to update deployment policy bp_netspeed with netspeed service 2.4.0. $res"
        exit 2
    fi 
}

# make sure the service 2.4.0 are running on both nodes
# and they were upgraded one by one 
function verify_rolling_upgrade {
    source ./utils.sh

    NS_URL="https://bluehorizon.network/services/netspeed"
    NS_ORG="IBM"
    NS_VERSION="2.4.0"
    ANAX_API1="http://localhost:8510"
    ANAX_API2="http://localhost:8511"

    # wait util both nodes have netspeed service version 2.4.0 running
    echo -e "\n${PREFIX} Checking service upgrade on node an12345..."
    ANAX_API=$ANAX_API1 MAX_ITERATION=60 WaitForService $NS_URL $NS_ORG $NS_VERSION
    if [ $? -ne 0 ]; then hzn eventlog list; exit $?; fi
    echo -e "\n${PREFIX} Checking service upgrade on node an54321..."
    ANAX_API=$ANAX_API2 MAX_ITERATION=60 WaitForService $NS_URL $NS_ORG $NS_VERSION
    if [ $? -ne 0 ]; then HORIZON_URL=$ANAX_API2 hzn eventlog list; exit $?; fi

    # now make sure they were upgraded in a rolling fashion
    ag1=$(curl -s  $ANAX_API1/agreement |jq -r ".agreements.active[] | select(.workload_to_run.url==\"$NS_URL\") | select(.workload_to_run.version==\"$NS_VERSION\") | select(.workload_to_run.org==\"$NS_ORG\")" 2>&1)
    ag2=$(curl -s  $ANAX_API2/agreement |jq -r ".agreements.active[] | select(.workload_to_run.url==\"$NS_URL\") | select(.workload_to_run.version==\"$NS_VERSION\") | select(.workload_to_run.org==\"$NS_ORG\")" 2>&1)
    ag_creation_time1=$(echo "$ag1" | jq -r '.agreement_creation_time')
    ag_creation_time2=$(echo "$ag2" | jq -r '.agreement_creation_time')
    ag_svc_start_time1=$(echo "$ag1" |jq -r '.agreement_execution_start_time')
    ag_svc_start_time2=$(echo "$ag2" |jq -r '.agreement_execution_start_time')

    echo -e "\n${PREFIX} ag_creation_time1=$ag_creation_time1 ag_svc_start_time1=$ag_svc_start_time1"
    echo -e "\n${PREFIX} ag_creation_time2=$ag_creation_time2 ag_svc_start_time2=$ag_svc_start_time2"
    if [ $ag_creation_time1 -le $ag_creation_time2 ] && [ $ag_svc_start_time1 -ge $ag_creation_time2 ]; then
        echo -e "\n${PREFIX} the HA group nodes did not upgrade the services with rolling fashion."
        exit 2
    fi 
    if [ $ag_creation_time1 -gt $ag_creation_time2 ] && [ $ag_svc_start_time2 -ge $ag_creation_time1 ]; then
        echo -e "\n${PREFIX} the HA group nodes did not upgrade the services with rolling fashion."
        exit 2
    fi 
}


echo ""
echo -e "${PREFIX} HA test started."

# make sure that the node an12345 and an54321 are in the ha group
verify_ha_group_name "an12345" "8510"
verify_ha_group_name "an54321" "8511"

if  [ "$PATTERN" != "" ]; then
    if  [ "$PATTERN" == "sns" ]; then
        # add new netspeed service version 2.4.0 to pattern sns
        publish_new_netspeed_service
        update_sns_pattern

        # check service rolling upgrade
        verify_rolling_upgrade
    fi
else
    # add new netspeed service version 2.4.0 to deployment policy bp_netspeed
    publish_new_netspeed_service
    update_ns_policy

    # check rolling upgrade
    verify_rolling_upgrade
fi
echo -e "${PREFIX} Done"



