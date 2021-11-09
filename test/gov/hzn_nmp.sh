#!/bin/bash

PREFIX="hzn exchange nmp CLI test:"
echo -e "$PREFIX start test"

cat <<'EOF' > /tmp/nmp_example_1.json
{
  "label": "nmp test 1",
  "description": "test nmp 1",
  "properties": [
      {
          "name": "name_value",
          "value": "value_value"
      }
  ],
  "constraints": [
      "myproperty == myvalue"
  ],
  "patterns": [
      "e2edev@somecomp.com/test_pattern"
  ],
  "enabled": false,
  "agentUpgradePolicy": {
      "atLeastVersion": "current",
      "start": "now",
      "duration": 0
  }
}
EOF

cat <<'EOF' > /tmp/nmp_example_2.json
{
  "label": "nmp test 2",
  "description": "test nmp 2",
  "properties": [
      {
          "name": "name_value",
          "value": "value_value"
      }
  ],
  "constraints": [
      "myproperty == myvalue"
  ],
  "enabled": true,
  "agentUpgradePolicy": {
      "atLeastVersion": "current",
      "start": "now",
      "duration": 0
  }
}
EOF

cat <<'EOF' > /tmp/nmp_example_3.json
{
  "label": "nmp test 3",
  "enabled": false
}
EOF

read -r -d '' inspectSampleNMP <<'EOF'
{
  "label": "",                               /* A short description of the policy. */
  "description": "",                         /* (Optional) A much longer description of the policy. */
  "properties": [                            /* (Optional) A list of policy properties that describe this policy. */
    {
      "name": "",
      "value": null
    }
  ],
  "constraints": [                           /* (Optional) A list of constraint expressions of the form <property name> <operator> <property value>, */
    "myproperty == myvalue"                  /* separated by boolean operators AND (&&) or OR (||).*/
  ],
  "patterns": [                              /* (Optional) This policy applies to nodes using one of these patterns. */
    ""
  ],
  "enabled": false,                          /* Is this policy enabled or disabled. */
  "agentUpgradePolicy": {                    /* (Optional) Assertions on how the agent should update itself. */
    "atLeastVersion": "<version> | current", /* Specify the minimum agent version these nodes should have, default "current". */
    "start": "<RFC3339 timestamp> | now",    /* When to start an upgrade, default "now". */
    "duration": 0                            /* Enable agents to randomize upgrade start time within start + duration, default 0. */
  }
}
EOF

read -r -d '' inspectSingleExchangeNMP <<'EOF'
{
  "NMP_ORG_ID/test-nmp-1": {
    "owner": "NMP_ORG_ID/NMP_USER_AUTH",
    "label": "nmp test 2",
    "description": "test nmp 2",
    "constraints": [
        "myproperty == myvalue"
    ],
    "properties": [
        {
            "name": "name_value",
            "value": "value_value"
        }
    ],
    "patterns": [],
    "enabled": true,
    "agentUpgradePolicy": {
        "atLeastVersion": "current",
        "start": "now",
        "duration": 0
    },
    "lastUpdated":
EOF

read -r -d '' inspectDoubleExchangeNMP1 <<'EOF'
{
  "NMP_ORG_ID/test-nmp-1": {
    "owner": "NMP_ORG_ID/NMP_USER_AUTH",
    "label": "nmp test 2",
    "description": "test nmp 2",
    "constraints": [
        "myproperty == myvalue"
    ],
    "properties": [
        {
            "name": "name_value",
            "value": "value_value"
        }
    ],
    "patterns": [],
    "enabled": true,
    "agentUpgradePolicy": {
        "atLeastVersion": "current",
        "start": "now",
        "duration": 0
    },
    "lastUpdated":
EOF

read -r -d '' inspectDoubleExchangeNMP2 <<'EOF'
"NMP_ORG_ID/test-nmp-2": {
    "owner": "NMP_ORG_ID/NMP_USER_AUTH",
    "label": "nmp test 3",
    "description": "",
    "constraints": [],
    "properties": [],
    "patterns": [],
    "enabled": false,
    "agentUpgradePolicy": {
        "atLeastVersion": "",
        "start": "",
        "duration": 0
    },
    "lastUpdated":
EOF

# Get HZN_ORG_ID and HZN_EXCHANGE_USER_AUTH, if they are set, otherwise set
# to e2edev defaults
if [ -z "$HZN_ORG_ID" ]
then
	NMP_ORG_ID="e2edev@somecomp.com"
else
	NMP_ORG_ID=$HZN_ORG_ID
fi
if [ -z "$HZN_EXCHANGE_USER_AUTH" ]
then
	NMP_USER_AUTH="e2edevadmin"
else
	NMP_USER_AUTH=${HZN_EXCHANGE_USER_AUTH#*/}
fi

# fill in key inpections with correct credentials
inspectSingleExchangeNMP="${inspectSingleExchangeNMP//NMP_ORG_ID/$NMP_ORG_ID}"
inspectSingleExchangeNMP="${inspectSingleExchangeNMP//NMP_USER_AUTH/${NMP_USER_AUTH%:*}}"
inspectDoubleExchangeNMP1="${inspectDoubleExchangeNMP1//NMP_ORG_ID/$NMP_ORG_ID}"
inspectDoubleExchangeNMP1="${inspectDoubleExchangeNMP1//NMP_USER_AUTH/${NMP_USER_AUTH%:*}}"
inspectDoubleExchangeNMP2="${inspectDoubleExchangeNMP2//NMP_ORG_ID/$NMP_ORG_ID}"
inspectDoubleExchangeNMP2="${inspectDoubleExchangeNMP2//NMP_USER_AUTH/${NMP_USER_AUTH%:*}}"

HZN_ORG_ID_SAVE=$HZN_ORG_ID
HZN_EXCHANGE_USER_AUTH_SAVE=$HZN_EXCHANGE_USER_AUTH
HZN_EXCHANGE_URL_SAVE=$HZN_EXCHANGE_URL

cleanup() {
    rm -f /tmp/nmp_example_1.json &> /dev/null
    rm -f /tmp/nmp_example_2.json &> /dev/null
    rm -f /tmp/nmp_example_3.json &> /dev/null
    hzn ex nmp rm -f test-nmp-1 &> /dev/null
    hzn ex nmp rm -f test-nmp-2 &> /dev/null
    hzn ex nmp rm -f test-nmp-3 &> /dev/null
}

# -----------------------
# ------- NMP NEW -------
# -----------------------

CMD_PREFIX="hzn exchange nmp new"

echo -e "${PREFIX} Testing '$CMD_PREFIX' without HZN_EXCHANGE_USER_AUTH set"
unset HZN_EXCHANGE_USER_AUTH
cmdOutput=$($CMD_PREFIX 2>&1)
rc=$?
if [[ $rc -eq 0 ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' without HZN_EXCHANGE_USER_AUTH set: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi
export HZN_EXCHANGE_USER_AUTH="$HZN_EXCHANGE_USER_AUTH_SAVE"

echo -e "${PREFIX} Testing '$CMD_PREFIX' with incorrect HZN_EXCHANGE_USER_AUTH set"
export HZN_EXCHANGE_USER_AUTH=fakeuser:fakepw
cmdOutput=$($CMD_PREFIX 2>&1)
rc=$?
if [[ $rc -eq 0 ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' with incorrect HZN_EXCHANGE_USER_AUTH set: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi
export HZN_EXCHANGE_USER_AUTH="$HZN_EXCHANGE_USER_AUTH_SAVE"

echo -e "${PREFIX} Testing '$CMD_PREFIX' without HZN_ORG_ID set"
unset HZN_ORG_ID
cmdOutput=$($CMD_PREFIX 2>&1)
rc=$?
if [[ $rc -eq 0 ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' with incorrect HZN_ORG_ID set: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi
export HZN_ORG_ID=$HZN_ORG_ID_SAVE

echo -e "${PREFIX} Testing '$CMD_PREFIX' with incorrect HZN_ORG_ID set"
export HZN_ORG_ID=fakeorg
cmdOutput=$($CMD_PREFIX 2>&1)
rc=$?
if [[ $rc -eq 0 ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' without HZN_ORG_ID set: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi
export HZN_ORG_ID=$HZN_ORG_ID_SAVE

echo -e "${PREFIX} Testing '$CMD_PREFIX' without HZN_EXCHANGE_URL set"
unset HZN_EXCHANGE_URL
mv /etc/default/horizon /etc/default/horizonOLD &> /dev/null
cmdOutput=$($CMD_PREFIX 2>&1)
rc=$?
if [[ $rc -eq 0 ]]; then
	echo -e "${PREFIX} completed."
	mv /etc/default/horizonOLD /etc/default/horizon &> /dev/null
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' without HZN_EXCHANGE_URL set: exit code: $rc, output: $cmdOutput."
	mv /etc/default/horizonOLD /etc/default/horizon &> /dev/null
	cleanup
	exit 1
fi
export HZN_EXCHANGE_URL=$HZN_EXCHANGE_URL_SAVE

echo -e "${PREFIX} Testing '$CMD_PREFIX'"
cmdOutput=$($CMD_PREFIX 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"$inspectSampleNMP"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX': exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

# -----------------------
# ------- NMP ADD -------
# -----------------------

CMD_PREFIX="hzn exchange nmp add"

echo -e "${PREFIX} Testing '$CMD_PREFIX' without HZN_EXCHANGE_USER_AUTH set"
unset HZN_EXCHANGE_USER_AUTH
cmdOutput=$($CMD_PREFIX 2>&1)
rc=$?
if [[ $rc -eq 1 ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' without HZN_EXCHANGE_USER_AUTH set: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi
export HZN_EXCHANGE_USER_AUTH="$HZN_EXCHANGE_USER_AUTH_SAVE"

echo -e "${PREFIX} Testing '$CMD_PREFIX' with incorrect HZN_EXCHANGE_USER_AUTH set"
export HZN_EXCHANGE_USER_AUTH=fakeuser:fakepw
cmdOutput=$($CMD_PREFIX 2>&1)
rc=$?
if [[ $rc -eq 1 ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' with incorrect HZN_EXCHANGE_USER_AUTH set: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi
export HZN_EXCHANGE_USER_AUTH="$HZN_EXCHANGE_USER_AUTH_SAVE"

echo -e "${PREFIX} Testing '$CMD_PREFIX' without HZN_ORG_ID set"
unset HZN_ORG_ID
cmdOutput=$($CMD_PREFIX 2>&1)
rc=$?
if [[ $rc -eq 1 ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' with incorrect HZN_ORG_ID set: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi
export HZN_ORG_ID=$HZN_ORG_ID_SAVE

echo -e "${PREFIX} Testing '$CMD_PREFIX' with incorrect HZN_ORG_ID set"
export HZN_ORG_ID=fakeorg
cmdOutput=$($CMD_PREFIX 2>&1)
rc=$?
if [[ $rc -eq 1 ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' without HZN_ORG_ID set: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi
export HZN_ORG_ID=$HZN_ORG_ID_SAVE

echo -e "${PREFIX} Testing '$CMD_PREFIX' without HZN_EXCHANGE_URL set"
unset HZN_EXCHANGE_URL
mv /etc/default/horizon /etc/default/horizonOLD &> /dev/null
cmdOutput=$($CMD_PREFIX test-nmp -f /tmp/nmp_example_1.json 2>&1)
rc=$?
if [[ $rc -eq 7 ]]; then
	echo -e "${PREFIX} completed."
	mv /etc/default/horizonOLD /etc/default/horizon &> /dev/null
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' without HZN_EXCHANGE_URL set: exit code: $rc, output: $cmdOutput."
	mv /etc/default/horizonOLD /etc/default/horizon &> /dev/null
	cleanup
	exit 1
fi
export HZN_EXCHANGE_URL=$HZN_EXCHANGE_URL_SAVE

echo -e "${PREFIX} Testing '$CMD_PREFIX' when constraints AND pattern(s) are defined"
cmdOutput=$($CMD_PREFIX test-nmp-1 -f /tmp/nmp_example_1.json 2>&1)
rc=$?
if [[ $rc -eq 5 && "$cmdOutput" == *"invalid-input, you can not specify both constraints and patterns"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' when constraints and pattern(s) are defined: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX'"
cmdOutput=$($CMD_PREFIX test-nmp-2 -f /tmp/nmp_example_2.json 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"Node management policy:"*"test-nmp-2"*"in the Horizon Exchange"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX': exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' when given nmp already exists in the Exchange"
cmdOutput=$($CMD_PREFIX test-nmp-2 -f /tmp/nmp_example_2.json 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"Node management policy:"*"test-nmp-2"*"in the Horizon Exchange"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' when given nmp already exists in the Exchange: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' without constraints defined and --no-constraints flag set"
cmdOutput=$($CMD_PREFIX test-nmp-3 -f /tmp/nmp_example_3.json --no-constraints 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"Node management policy:"*"test-nmp-3"*"in the Horizon Exchange"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' without constraints defined and --no-constraints flag set: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' without constraints defined and without --no-constraints flag"
cmdOutput=$($CMD_PREFIX test-nmp-3 -f /tmp/nmp_example_3.json 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *"Error: The node management policy has no constraints which might result in the management policy being deployed to all nodes. Please specify --no-constraints to confirm that this is acceptable."* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' without constraints defined and without --no-constraints flag: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' without --json-file flag"
cmdOutput=$($CMD_PREFIX 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *"rror: required flag --json-file not provided"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' without --json-file flag: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' without nmp-name argument"
cmdOutput=$($CMD_PREFIX -f /tmp/nmp_example_2.json 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *"rror: required argument"*"not provided"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' without nmp-name argument: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

# -----------------------
# ------ NMP REMOVE -----
# -----------------------

CMD_PREFIX="hzn exchange nmp remove"

echo -e "${PREFIX} Testing '$CMD_PREFIX' without HZN_EXCHANGE_USER_AUTH set"
unset HZN_EXCHANGE_USER_AUTH
cmdOutput=$($CMD_PREFIX test-nmp -f 2>&1)
rc=$?
if [[ $rc -eq 1 ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' without HZN_EXCHANGE_USER_AUTH set: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi
export HZN_EXCHANGE_USER_AUTH="$HZN_EXCHANGE_USER_AUTH_SAVE"

echo -e "${PREFIX} Testing '$CMD_PREFIX' with incorrect HZN_EXCHANGE_USER_AUTH set"
export HZN_EXCHANGE_USER_AUTH=fakeuser:fakepw
cmdOutput=$($CMD_PREFIX test-nmp -f 2>&1)
rc=$?
if [[ $rc -eq 5 ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' with incorrect HZN_EXCHANGE_USER_AUTH set: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi
export HZN_EXCHANGE_USER_AUTH="$HZN_EXCHANGE_USER_AUTH_SAVE"

echo -e "${PREFIX} Testing '$CMD_PREFIX' without HZN_ORG_ID set"
unset HZN_ORG_ID
cmdOutput=$($CMD_PREFIX test-nmp -f 2>&1)
rc=$?
if [[ $rc -eq 1 ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' with incorrect HZN_ORG_ID set: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi
export HZN_ORG_ID=$HZN_ORG_ID_SAVE

echo -e "${PREFIX} Testing '$CMD_PREFIX' with incorrect HZN_ORG_ID set"
export HZN_ORG_ID=fakeorg
cmdOutput=$($CMD_PREFIX test-nmp -f 2>&1)
rc=$?
if [[ $rc -eq 5 ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' without HZN_ORG_ID set: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi
export HZN_ORG_ID=$HZN_ORG_ID_SAVE

echo -e "${PREFIX} Testing '$CMD_PREFIX' without HZN_EXCHANGE_URL set"
unset HZN_EXCHANGE_URL
mv /etc/default/horizon /etc/default/horizonOLD &> /dev/null
cmdOutput=$($CMD_PREFIX test-nmp -f 2>&1)
rc=$?
if [[ $rc -eq 7 ]]; then
	echo -e "${PREFIX} completed."
	mv /etc/default/horizonOLD /etc/default/horizon &> /dev/null
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' without HZN_EXCHANGE_URL set: exit code: $rc, output: $cmdOutput."
	mv /etc/default/horizonOLD /etc/default/horizon &> /dev/null
	cleanup
	exit 1
fi
export HZN_EXCHANGE_URL=$HZN_EXCHANGE_URL_SAVE

echo -e "${PREFIX} Testing '$CMD_PREFIX' -f"
cmdOutput=$($CMD_PREFIX test-nmp-2 -f 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"Removing node management policy"*"and re-evaluating all agreements"*"Node management policy"*"/test-nmp-2 removed"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX': exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Removing remaining NMP's"
cmdOutput=$($CMD_PREFIX test-nmp-3 -f 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"Removing node management policy"*"and re-evaluating all agreements"*"Node management policy"*"/test-nmp-3 removed"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX': exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Checking that NMP's have been removed from the Exchange..."
cmdOutput=$(hzn ex nmp ls 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"[]"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' when no nmp's exists in the Exchange: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi
echo -e "${PREFIX} done."

echo -e "${PREFIX} adding test nmp to exchange..."
hzn ex nmp add test-nmp-1 -f /tmp/nmp_example_2.json &> /dev/null
echo -e "${PREFIX} done."

echo -e "${PREFIX} Testing '$CMD_PREFIX' without -f set and answering 'no'"
cmdOutput=$(echo "n" | $CMD_PREFIX test-nmp-1 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"Are you sure you want to remove node management policy test-nmp-1 for org"*"from the Horizon Exchange? [y/N]: Exiting."* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' without -f set and answering 'no': exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' without -f set and answering 'yes'"
cmdOutput=$(yes | $CMD_PREFIX test-nmp-1 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"Removing node management policy"*"and re-evaluating all agreements"*"Node management policy"*"/test-nmp-1 removed"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' without -f set and answering 'yes': exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' with incorrect nmp-name"
cmdOutput=$($CMD_PREFIX fake-nmp -f 2>&1)
rc=$?
if [[ $rc -eq 8 && "$cmdOutput" == *"Error: Node management policy fake-nmp not found in org"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' with incorrect nmp-name: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

# -----------------------
# ------- NMP LIST ------
# -----------------------

CMD_PREFIX="hzn exchange nmp list"

echo -e "${PREFIX} Testing '$CMD_PREFIX' without HZN_EXCHANGE_USER_AUTH set"
unset HZN_EXCHANGE_USER_AUTH
cmdOutput=$($CMD_PREFIX 2>&1)
rc=$?
if [[ $rc -eq 1 ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' without HZN_EXCHANGE_USER_AUTH set: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi
export HZN_EXCHANGE_USER_AUTH="$HZN_EXCHANGE_USER_AUTH_SAVE"

echo -e "${PREFIX} Testing '$CMD_PREFIX' with incorrect HZN_EXCHANGE_USER_AUTH set"
export HZN_EXCHANGE_USER_AUTH=fakeuser:fakepw
cmdOutput=$($CMD_PREFIX 2>&1)
rc=$?
if [[ $rc -eq 5 ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' with incorrect HZN_EXCHANGE_USER_AUTH set: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi
export HZN_EXCHANGE_USER_AUTH="$HZN_EXCHANGE_USER_AUTH_SAVE"

echo -e "${PREFIX} Testing '$CMD_PREFIX' without HZN_ORG_ID set"
unset HZN_ORG_ID
cmdOutput=$($CMD_PREFIX 2>&1)
rc=$?
if [[ $rc -eq 1 ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' with incorrect HZN_ORG_ID set: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi
export HZN_ORG_ID=$HZN_ORG_ID_SAVE

echo -e "${PREFIX} Testing '$CMD_PREFIX' with incorrect HZN_ORG_ID set"
export HZN_ORG_ID=fakeorg
cmdOutput=$($CMD_PREFIX 2>&1)
rc=$?
if [[ $rc -eq 5 ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' without HZN_ORG_ID set: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi
export HZN_ORG_ID=$HZN_ORG_ID_SAVE

echo -e "${PREFIX} Testing '$CMD_PREFIX' without HZN_EXCHANGE_URL set"
unset HZN_EXCHANGE_URL
mv /etc/default/horizon /etc/default/horizonOLD &> /dev/null
cmdOutput=$($CMD_PREFIX 2>&1)
rc=$?
if [[ $rc -eq 7 ]]; then
	echo -e "${PREFIX} completed."
	mv /etc/default/horizonOLD /etc/default/horizon &> /dev/null
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' without HZN_EXCHANGE_URL set: exit code: $rc, output: $cmdOutput."
	mv /etc/default/horizonOLD /etc/default/horizon &> /dev/null
	cleanup
	exit 1
fi
export HZN_EXCHANGE_URL=$HZN_EXCHANGE_URL_SAVE

echo -e "${PREFIX} Testing '$CMD_PREFIX' when no nmp's exists in the Exchange"
cmdOutput=$($CMD_PREFIX 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"[]"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' when no nmp's exists in the Exchange: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' --long when no nmp's exists in the Exchange"
cmdOutput=$($CMD_PREFIX -l 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"[]"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' --long when no nmp's exists in the Exchange: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} adding test nmp to exchange..."
hzn ex nmp add test-nmp-1 -f /tmp/nmp_example_2.json &> /dev/null
echo -e "${PREFIX} done."

echo -e "${PREFIX} Testing '$CMD_PREFIX' when 1 nmp exists in the Exchange"
cmdOutput=$($CMD_PREFIX 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"["*"$HZN_ORG_ID/test-nmp-1"*"]"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' when 1 nmp exists in the Exchange: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' --long when 1 nmp exists in the Exchange"
cmdOutput=$($CMD_PREFIX -l | tr -d '[:space:]' 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"$(echo $inspectSingleExchangeNMP | tr -d '[:space:]')"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' --long when 1 nmp exists in the Exchange: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} adding second test nmp to exchange..."
hzn ex nmp add test-nmp-2 -f /tmp/nmp_example_3.json --no-constraints &> /dev/null
echo -e "${PREFIX} done."

echo -e "${PREFIX} Testing '$CMD_PREFIX' when 2 nmp's exist in the Exchange"
cmdOutput=$($CMD_PREFIX 2>&1)
rc=$?
if [[ $rc -eq 0 && ("$cmdOutput" == *"["*"$HZN_ORG_ID/test-nmp-1"*"$HZN_ORG_ID/test-nmp-2"*"]"* || "$cmdOutput" == *"["*"$HZN_ORG_ID/test-nmp-2"*"$HZN_ORG_ID/test-nmp-1"*"]"*) ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' when 2 nmp's exist in the Exchange: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' --long when 2 nmp's exist in the Exchange"
cmdOutput=$($CMD_PREFIX -l | tr -d '[:space:]' 2>&1)
rc=$?
if [[ $rc -eq 0 && ("$cmdOutput" == *"$(echo $inspectDoubleExchangeNMP1 | tr -d '[:space:]')"*"$(echo $inspectDoubleExchangeNMP2 | tr -d '[:space:]')"* || "$cmdOutput" == *"$(echo $inspectDoubleExchangeNMP2 | tr -d '[:space:]')"*"$(echo $inspectDoubleExchangeNMP1 | tr -d '[:space:]')"*) ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' when 2 nmp's exist in the Exchange: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' [<nmp-name>] --long when 2 nmp's exist in the Exchange"
cmdOutput=$($CMD_PREFIX test-nmp-1 -l | tr -d '[:space:]' 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"$(echo $inspectDoubleExchangeNMP1 | tr -d '[:space:]')"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' when 2 nmp's exist in the Exchange: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' with incorrect nmp-name"
cmdOutput=$($CMD_PREFIX fake-nmp 2>&1)
rc=$?
if [[ $rc -eq 8 && "$cmdOutput" == *"Error: NMP fake-nmp not found in org"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' with incorrect nmp-name: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

# -----------------------
# ------- Clean-up ------
# -----------------------

cleanup
echo -e "${PREFIX} Done."
