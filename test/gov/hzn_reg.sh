#!/bin/bash

PREFIX="hzn reg test:"

echo ""
echo -e "${PREFIX} Registering and unregitering with hzn command."

USERDEV_ADMIN_AUTH="userdev/userdevadmin:userdevadminpw"
export HZN_EXCHANGE_URL="${EXCH_APP_HOST}"

unset HZN_ORG_ID

# preparing the userinput file and the policy file
cat <<EOF > /tmp/node_policy.json
{
  "properties": [
    {
      "name": "purpose",
      "value": "network-testing"
    },
    {
      "name": "group",
      "value": "bluenode"
    }
  ],
  "constraints": [
    "iame2edev == true",
    "NONS==false || NOGPS == true || NOLOC == true || NOPWS == true || NOHELLO == true || NOK8S == true"
  ]
}
EOF

cat <<EOF > /tmp/reg_userinput.json
{
  "services": [
    {
      "org": "IBM",
      "url": "https://bluehorizon.network/services/netspeed",
      "versionRange": "[2.2.0,INFINITY)",
      "variables": {
        "var1": "aString",
        "var2": 5,
        "var3": 22.2
      }
    },
    {
      "org": "IBM",
      "url": "https://bluehorizon.network/service-cpu",
      "versionRange": "[0.0.0,INFINITY)",
      "variables": {
        "cpu_var1": "ibmnodevar1"
      }
    }
  ]
}
EOF

cat <<EOF > /tmp/reg_userinput_all.json
{
  "services": [
    {
      "org": "IBM",
      "url": "https://bluehorizon.network/services/netspeed",
      "versionRange": "[2.2.0,INFINITY)",
      "variables": {
        "var1": "aString",
        "var2": 5,
        "var3": 22.2
      }
    },
    {
      "org": "e2edev@somecomp.com",
      "url": "https://bluehorizon.network/services/netspeed",
      "versionRange": "2.2.0",
      "variables": {
        "var1": "node_String",
        "var2": 20,
        "var3": 23.2
      }
    },
    {
      "org": "IBM",
      "url": "https://bluehorizon.network/service-cpu",
      "versionRange": "[0.0.0,INFINITY)",
      "variables": {
        "cpu_var1": "ibmnodevar1"
      }
    },
    {
      "org": "e2edev@somecomp.com",
      "url": "https://bluehorizon.network/service-cpu",
      "versionRange": "1.0.0",
      "variables": {
        "cpu_var1": "e2edevnodevar1"
      }
    },
    {
      "org": "e2edev@somecomp.com",
      "url": "https://bluehorizon.network/services/locgps",
      "versionRange": "2.0.3",
      "variables": {
        "test": "testValue",
        "extra": "extraValue",
        "HZN_LAT": 41.921766,
        "HZN_LON": -73.894224,
        "HZN_LOCATION_ACCURACY_KM": 0.5,
        "HZN_USE_GPS": false
      }
    },
    {
      "org": "IBM",
      "url": "https://bluehorizon.network/service-gps",
      "versionRange": "2.0.3",
      "variables": {
        "HZN_LAT": 41.921766,
        "HZN_LON": -73.894224,
        "HZN_LOCATION_ACCURACY_KM": 0.5,
        "HZN_USE_GPS": false
      }
    },
    {
      "org": "e2edev@somecomp.com",
      "url": "https://bluehorizon.network/services/weather",
      "versionRange": "1.5.0",
      "variables": {
        "HZN_LAT": 41.921766,
        "HZN_LON": -73.894224,
        "HZN_LOCATION_ACCURACY_KM": 0.5,
        "HZN_USE_GPS": false,
        "HZN_WUGNAME": "e2edev mocked pws",
        "HZN_PWS_MODEL": "LaCrosse WS2317",
        "MTN_PWS_MODEL": "LaCrosse WS2317",
        "HZN_PWS_ST_TYPE": "WS23xx",
        "MTN_PWS_ST_TYPE": "WS23xx"
      }
    }
  ]
}
EOF

function reg_node {
  cmd=$1
  echo -e "$cmd"
  ${cmd}
  if [ $? -ne 0 ]; then 
    echo -e "${PREFIX} Failed to register node with hzn register"
    exit 1
  fi
}

# unregister the node using hzn
function unreg_node {
  hzn unregister -f
  if [ $? -ne 0 ]; then
    echo -e "${PREFIX} Failed to unregister the node."
    exit 1
  fi
}


# make sure agreements are up and running
# $1 - org ID for node check
# $2 - auth for node check
function verify_agreements {
  ORG_ID=$1 ADMIN_AUTH=$2 HZN_REG_TEST=1 ./verify_agreements.sh
  if [ $? -ne 0 ]; then
    echo -e "${PREFIX} Failed to verify agreement."
    exit 1
  fi
}


## first unregister the node
echo -e "${PREFIX} Testing 'hzn unregister -fr'"
hzn unregister -fr
if [ $? -ne 0 ]; then
	echo -e "${PREFIX} Failed to unregister the node."
	exit 1
fi

## test unregister while the node is already unregistered
echo -e "${PREFIX} Testing 'hzn unregister' while the node is not registered."
ret=$(hzn unregister -f)
if [ $? != 0 ]; then
  echo -e "${PREFIX} Error: 'hzn unregister' should have return 0. $ret"
  exit 1
elif [[ $ret != *"The node is not registered"* ]]; then
  echo -e "${PREFIX} Wrong error message for 'hzn unregister': $ret."
  echo -e "${PREFIX} It should contain 'The node is not registered'"
  exit 1
else
  echo -e "$ret"
fi

## test registering node with both flags and positional arguments, conflict
echo -e "${PREFIX} Testing 'hzn register' with conflict inputs"
cmd="hzn register -u $USERDEV_ADMIN_AUTH -n an12345:abcdefghijk -o userdev -f /tmp/reg_userinput.json --policy /tmp/node_policy.json e2edev@somecomp.com sns"
echo -e "$cmd"
ret=`$cmd 2>&1`
if [ $? -eq 0 ]; then
  echo -e "${PREFIX} 'hzn register' should have failed because of the conflict input."
  exit 1
elif [[ $ret != *"-o and -p are mutually exclusive with <nodeorg> and <pattern> arguments"* ]]; then
  echo -e "${PREFIX} Wrong error message for 'hzn register': $ret."
  echo -e "${PREFIX} It should have said '-o and -p are mutually exclusive with <nodeorg> and <pattern> arguments'."
  exit 1
fi

## test registering policy based node, update token
echo -e "${PREFIX} Testing 'hzn register' with policy."
cmd="hzn register -u $USERDEV_ADMIN_AUTH -n an12345:abcdefghijk -o userdev -f /tmp/reg_userinput.json --policy /tmp/node_policy.json"
reg_node "$cmd"
verify_agreements "userdev" "userdevadmin:userdevadminpw"

## test register while the node is registered
echo -e "${PREFIX} Testing 'hzn register' while the node is registered."
ret=$(hzn register -u $USERDEV_ADMIN_AUTH -n an12345:abcdefg -o userdev -f /tmp/reg_userinput.json --policy /tmp/node_policy.json 2>&1)
if [ $? -eq 0 ]; then
  echo -e "${PREFIX} 'hzn register' should have failed because the node is registered already."
  exit 1
elif [[ $ret != *"this Horizon node is already registered or in the process of being registered"* ]]; then
  echo -e "${PREFIX} Wrong error message for 'hzn register': $ret."
  echo -e "${PREFIX} It should have said 'this Horizon node is already registered or in the process of being registered'."
  exit 1
else
  echo -e "$ret"
fi

## register pattern sns, node will be created by this command
unreg_node
hzn exchange -u e2edevadmin:e2edevadminpw -o e2edev@somecomp.com node remove an12345 -f 

echo -e "${PREFIX} Testing 'hzn register' with pattern sns."
cmd="hzn register -u e2edevadmin:e2edevadminpw -n an12345:abcdefg -f /tmp/reg_userinput.json -o e2edev@somecomp.com -p e2edev@somecomp.com/sns"
reg_node "$cmd"
verify_agreements "e2edev@somecomp.com" "e2edevadmin:e2edevadminpw"

## register pattern sns using positional argument, node already exists
unreg_node
echo -e "${PREFIX} Testing 'hzn register' with pattern sns using positional argument."
cmd="hzn register -n e2edev@somecomp.com/an12345:abcdefg -f /tmp/reg_userinput.json e2edev@somecomp.com sns"
reg_node "$cmd"
verify_agreements "e2edev@somecomp.com" "e2edevadmin:e2edevadminpw"

## test registering node with both sns pattern and policy
unreg_node
echo -e "${PREFIX} Testing 'hzn register' with both pattern sns and policy."
cmd="hzn register -n an12345:abcdefg -o e2edev@somecomp.com -f /tmp/reg_userinput.json -p sns --policy /tmp/node_policy.json"
reg_node "$cmd"

# make sure node has pattern associated.
ret=$(hzn node list |jq '.pattern')
if [ $ret != '"e2edev@somecomp.com/sns"' ]; then
  echo -e "${PREFIX} the node should have pattern e2edev@somecomp.com/sns, but got: $ret"
  exit 1
fi

verify_agreements "e2edev@somecomp.com" "e2edevadmin:e2edevadminpw"

## register pattern sall using flags, take HZN_ORG_ID. Use sall as the last test to restore the status before this script.
unreg_node
export HZN_ORG_ID=e2edev@somecomp.com
echo -e "${PREFIX} Testing 'hzn register' with pattern sall with HZN_ORG_ID"
cmd="hzn register -n an12345:abcdefg -f /tmp/reg_userinput_all.json -p sall"
reg_node "$cmd"

ORG_ID="e2edev@somecomp.com" ADMIN_AUTH="e2edevadmin:e2edevadminpw" ./verify_agreements.sh
if [ $? -ne 0 ]; then 
  echo -e "${PREFIX} Failed to verify agreement."
  exit 1
fi
unset HZN_ORG_ID

echo -e "${PREFIX} Done"

