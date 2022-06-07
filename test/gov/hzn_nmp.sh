#!/bin/bash

PREFIX="hzn exchange nmp CLI test:"
echo -e "$PREFIX start test"

cat <<'EOF' > /tmp/nmp_example_1.json
{
  "label": "nmp test 1",
  "description": "test nmp 1",
  "constraints": [
    "purpose==nmp-testing"
  ],
  "properties": [
    {
	  "name": "iame2edev",
      "value": true
	},
	{
	  "name": "NOAGENTAUTO",
	  "value": false
	}
  ],
  "patterns": [
    "e2edev@somecomp.com/test_pattern"
  ],
  "enabled": false,
  "start": "now",
  "startWindow": 0,
  "agentUpgradePolicy": {
	"manifest": "manifest_1.0.0",
	"allowDowngrade": false
  }
}
EOF

cat <<'EOF' > /tmp/nmp_example_2.json
{
  "label": "nmp test 2",
  "description": "test nmp 2",
  "properties": [
	{
	  "name": "iame2edev",
      "value": true
	},
	{
	  "name": "NOAGENTAUTO",
	  "value": false
	}
  ],
  "constraints": [
    "purpose==nmp-testing"
  ],
  "enabled": true,
  "start": "now",
  "startWindow": 0,
  "agentUpgradePolicy": {
    "manifest": "manifest_2.0.0",
    "allowDowngrade": false
  }
}
EOF

cat <<'EOF' > /tmp/nmp_example_3.json
{
  "label": "nmp test 3",
  "enabled": false
}
EOF

cat <<'EOF' > /tmp/nmp_example_4.json
{
  "label": "",
  "enabled": false
}
EOF

read -r -d '' inspectSampleNMP <<'EOF'
{
  "label": "",                               /* A short description of the policy. */
  "description": "",                         /* (Optional) A much longer description of the policy. */
  "constraints": [                           /* (Optional) A list of constraint expressions of the form <property name> <operator> <property value>, */
    "myproperty == myvalue"                  /* separated by boolean operators AND (&&) or OR (||).*/
  ],
  "properties": [                            /* (Optional) A list of policy properties that describe this policy. */
    {
      "name": "",
      "value": null
    }
  ],
  "patterns": [                              /* (Optional) This policy applies to nodes using one of these patterns. */
    ""
  ],
  "enabled": false,                          /* Is this policy enabled or disabled. */
  "start": "<RFC3339 timestamp> | now",      /* When to start an upgrade, default "now". */
  "startWindow": 0,                          /* Enable agents to randomize upgrade start time within start + startWindow, default 0. */
  "agentUpgradePolicy": {                    /* (Optional) Assertions on how the agent should update itself. */
    "manifest": "",                          /* The manifest file containing the software, config and cert files to upgrade. */
    "allowDowngrade": false                  /* Is this policy allowed to perform a downgrade to a previous version. */
  }
}
EOF

cat <<'EOF' > /tmp/nmp_status_1.json
{
  "agentUpgradePolicyStatus": {
    "scheduledTime": "0001-01-01T00:00:00Z",
    "startTime": "",
    "endTime": "",
    "upgradedVersions": {
      "softwareVersion": "1.0.0",
      "certVersion": "2.0.0",
      "configVersion": "3.0.0"
    },
    "status": "waiting",
    "errorMessage": ""
  }
}
EOF

# Get HZN_ORG_ID and HZN_EXCHANGE_USER_AUTH, if they are set, otherwise set
# to userdev defaults
if [[ -z "$HZN_ORG_ID" || "$HZN_ORG_ID" == *"e2edev@somecomp.com"* ]]
then
	NMP_ORG_ID="userdev"
else
	NMP_ORG_ID=$HZN_ORG_ID
fi
if [[ -z "$HZN_EXCHANGE_USER_AUTH" || "$HZN_EXCHANGE_USER_AUTH" == *"e2edevadmin:e2edevadminpw"* ]]
then
	NMP_EXCHANGE_USER_AUTH="userdevadmin:userdevadminpw"
else
	NMP_EXCHANGE_USER_AUTH=${HZN_EXCHANGE_USER_AUTH#*/}
fi
if [[ -z "$HZN_EXCHANGE_URL" ]]
then
	HZN_EXCHANGE_URL="http://localhost:3090/v1"
fi

HZN_ORG_ID_SAVE=$HZN_ORG_ID
HZN_EXCHANGE_USER_AUTH_SAVE=$HZN_EXCHANGE_USER_AUTH
HZN_EXCHANGE_URL_SAVE=$HZN_EXCHANGE_URL

function cleanup() {
    rm -f /tmp/nmp_example_1.json &> /dev/null
    rm -f /tmp/nmp_example_2.json &> /dev/null
    rm -f /tmp/nmp_example_3.json &> /dev/null
	rm -f /tmp/nmp_example_4.json &> /dev/null
	rm -f /tmp/nmp_status_1.json &> /dev/null
    hzn ex nmp rm -f test-nmp-1 -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH &> /dev/null
    hzn ex nmp rm -f test-nmp-2 -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH &> /dev/null
    hzn ex nmp rm -f test-nmp-3 -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH &> /dev/null
}

function test_env_variables() {
	local cmd_to_test
	cmd_to_test=$1
	local require_exchange_cred
	require_exchange_cred="$2"

	echo -e "${PREFIX} Testing '$cmd_to_test' without HZN_EXCHANGE_USER_AUTH set"
	unset HZN_EXCHANGE_USER_AUTH
	cmdOutput=$($cmd_to_test 2>&1)
	rc=$?
	if [[ $rc -eq 0 && $require_exchange_cred == "false" ]] || [[ $rc -ne 0 && $require_exchange_cred == "true" ]]; then
		echo -e "${PREFIX} completed."
	else
		echo -e "${PREFIX} Failed: Wrong error response from '$cmd_to_test' without HZN_EXCHANGE_USER_AUTH set: exit code: $rc, output: $cmdOutput."
		cleanup
		exit 1
	fi
	export HZN_EXCHANGE_USER_AUTH="$HZN_EXCHANGE_USER_AUTH_SAVE"

	echo -e "${PREFIX} Testing '$cmd_to_test' with incorrect HZN_EXCHANGE_USER_AUTH set"
	export HZN_EXCHANGE_USER_AUTH=fakeuser:fakepw
	cmdOutput=$($cmd_to_test 2>&1)
	rc=$?
	if [[ $rc -eq 0 && $require_exchange_cred == "false" ]] || [[ $rc -ne 0 && $require_exchange_cred == "true" ]]; then
		echo -e "${PREFIX} completed."
	else
		echo -e "${PREFIX} Failed: Wrong error response from '$cmd_to_test' with incorrect HZN_EXCHANGE_USER_AUTH set: exit code: $rc, output: $cmdOutput."
		cleanup
		exit 1
	fi
	export HZN_EXCHANGE_USER_AUTH="$HZN_EXCHANGE_USER_AUTH_SAVE"

	echo -e "${PREFIX} Testing '$cmd_to_test' without HZN_ORG_ID set"
	unset HZN_ORG_ID
	cmdOutput=$($cmd_to_test 2>&1)
	rc=$?
	if [[ $rc -eq 0 && $require_exchange_cred == "false" ]] || [[ $rc -ne 0 && $require_exchange_cred == "true" ]]; then
		echo -e "${PREFIX} completed."
	else
		echo -e "${PREFIX} Failed: Wrong error response from '$cmd_to_test' with incorrect HZN_ORG_ID set: exit code: $rc, output: $cmdOutput."
		cleanup
		exit 1
	fi
	export HZN_ORG_ID=$HZN_ORG_ID_SAVE

	echo -e "${PREFIX} Testing '$cmd_to_test' with incorrect HZN_ORG_ID set"
	export HZN_ORG_ID=fakeorg
	cmdOutput=$($cmd_to_test 2>&1)
	rc=$?
	if [[ $rc -eq 0 && $require_exchange_cred == "false" ]] || [[ $rc -ne 0 && $require_exchange_cred == "true" ]]; then
		echo -e "${PREFIX} completed."
	else
		echo -e "${PREFIX} Failed: Wrong error response from '$cmd_to_test' without HZN_ORG_ID set: exit code: $rc, output: $cmdOutput."
		cleanup
		exit 1
	fi
	export HZN_ORG_ID=$HZN_ORG_ID_SAVE

	echo -e "${PREFIX} Testing '$cmd_to_test' without HZN_EXCHANGE_URL set"
	unset HZN_EXCHANGE_URL
	mv /etc/default/horizon /etc/default/horizonOLD &> /dev/null
	cmdOutput=$($cmd_to_test 2>&1)
	rc=$?
	if [[ $rc -eq 0 && $require_exchange_cred == "false" ]] || [[ $rc -ne 0 && $require_exchange_cred == "true" ]]; then
		echo -e "${PREFIX} completed."
		mv /etc/default/horizonOLD /etc/default/horizon &> /dev/null
	else
		echo -e "${PREFIX} Failed: Wrong error response from '$cmd_to_test' without HZN_EXCHANGE_URL set: exit code: $rc, output: $cmdOutput."
		mv /etc/default/horizonOLD /etc/default/horizon &> /dev/null
		cleanup
		exit 1
	fi
	export HZN_EXCHANGE_URL=$HZN_EXCHANGE_URL_SAVE
}

# -----------------------
# ------- NMP NEW -------
# -----------------------

CMD_PREFIX="hzn exchange nmp new"
test_env_variables "$CMD_PREFIX" false

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
test_env_variables "$CMD_PREFIX fakenmp -f /tmp/nmp_example_1.json" true

echo -e "${PREFIX} Testing '$CMD_PREFIX' when constraints AND pattern(s) are defined"
cmdOutput=$($CMD_PREFIX test-nmp-1 -f /tmp/nmp_example_1.json -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 5 && "$cmdOutput" == *"invalid-input, you can not specify both constraints and patterns"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' when constraints and pattern(s) are defined: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX'"
cmdOutput=$($CMD_PREFIX test-nmp-2 -f /tmp/nmp_example_2.json -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"Node management policy $NMP_ORG_ID/test-nmp-2 added in the Horizon Exchange"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX': exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' when given nmp already exists in the Exchange"
cmdOutput=$($CMD_PREFIX test-nmp-2 -f /tmp/nmp_example_2.json -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"Node management policy $NMP_ORG_ID/test-nmp-2 updated in the Horizon Exchange"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' when given nmp already exists in the Exchange: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' without constraints defined and --no-constraints flag set"
cmdOutput=$($CMD_PREFIX test-nmp-3 -f /tmp/nmp_example_3.json --no-constraints -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"Node management policy $NMP_ORG_ID/test-nmp-3 added in the Horizon Exchange"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' without constraints defined and --no-constraints flag set: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' without constraints defined and without --no-constraints flag"
cmdOutput=$($CMD_PREFIX test-nmp-3 -f /tmp/nmp_example_3.json -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *"Error: The node management policy has no constraints which might result in the management policy being deployed to all nodes. Please specify --no-constraints to confirm that this is acceptable."* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' without constraints defined and without --no-constraints flag: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' without --json-file flag"
cmdOutput=$($CMD_PREFIX -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *"rror: required flag --json-file not provided"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' without --json-file flag: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' without nmp-name argument"
cmdOutput=$($CMD_PREFIX -f /tmp/nmp_example_2.json -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *"rror: required argument"*"not provided"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' without nmp-name argument: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' with incorrect format"
cmdOutput=$($CMD_PREFIX test-nmp-4 -f /tmp/nmp_example_4.json -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 1 && "$cmdOutput" == *"Incorrect node management policy format"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' with incorrect format: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' --applies-to"
cmdOutput=$($CMD_PREFIX test-nmp-2 -f /tmp/nmp_example_2.json --applies-to --dry-run -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"["*"$NMP_ORG_ID/an12345"*"]"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' --applies-to: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' --applies-to when NMP is not compatible with any nodes"
cmdOutput=$($CMD_PREFIX test-nmp-3 -f /tmp/nmp_example_3.json --no-constraints --applies-to --dry-run -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"[]"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' --applies-to when NMP is not compatible with any nodes: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

# -----------------------
# ------ NMP REMOVE -----
# -----------------------

CMD_PREFIX="hzn exchange nmp remove"
test_env_variables "$CMD_PREFIX fakenmp -f" true

echo -e "${PREFIX} Testing '$CMD_PREFIX' -f"
cmdOutput=$($CMD_PREFIX test-nmp-2 -f -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"Node management policy $NMP_ORG_ID/test-nmp-2 removed from the Horizon Exchange"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX': exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Removing remaining NMP's"
cmdOutput=$($CMD_PREFIX test-nmp-3 -f -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"Node management policy $NMP_ORG_ID/test-nmp-3 removed from the Horizon Exchange"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX': exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Checking that NMP's have been removed from the Exchange..."
cmdOutput=$(hzn ex nmp ls -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
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
hzn ex nmp add test-nmp-1 -f /tmp/nmp_example_2.json -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH &> /dev/null
if [[ $? != 0 ]]; then 
	echo -e "${PREFIX} failed to add test nmp to Exchange"
	cleanup
	exit 1
fi
echo -e "${PREFIX} done."

echo -e "${PREFIX} Testing '$CMD_PREFIX' without -f set and answering 'no'"
cmdOutput=$(echo "n" | $CMD_PREFIX test-nmp-1 -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"Are you sure you want to remove node management policy test-nmp-1 for org"*"from the Horizon Exchange? [y/N]: Exiting."* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' without -f set and answering 'no': exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' without -f set and answering 'yes'"
cmdOutput=$(yes | $CMD_PREFIX test-nmp-1 -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"Node management policy $NMP_ORG_ID/test-nmp-1 removed from the Horizon Exchange"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' without -f set and answering 'yes': exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' with incorrect nmp-name"
cmdOutput=$($CMD_PREFIX fake-nmp -f -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
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
test_env_variables "$CMD_PREFIX" true

echo -e "${PREFIX} Testing '$CMD_PREFIX' when no nmp's exists in the Exchange"
cmdOutput=$($CMD_PREFIX -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"[]"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' when no nmp's exists in the Exchange: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' --long when no nmp's exists in the Exchange"
cmdOutput=$($CMD_PREFIX -l -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"[]"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' --long when no nmp's exists in the Exchange: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} adding test nmp to exchange..."
hzn ex nmp add test-nmp-1 -f /tmp/nmp_example_2.json -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH -v &> /dev/null
if [[ $? != 0 ]]; then 
	echo -e "${PREFIX} failed to add test nmp to Exchange"
	cleanup
	exit 1
fi
echo -e "${PREFIX} done."

echo -e "${PREFIX} Testing '$CMD_PREFIX' when 1 nmp exists in the Exchange"
cmdOutput=$($CMD_PREFIX -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"["*"$NMP_ORG_ID/test-nmp-1"*"]"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' when 1 nmp exists in the Exchange: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

USERNAME=$(echo $NMP_EXCHANGE_USER_AUTH | awk -F : '{print $1}')
NMP_OUTPUT1="$NMP_ORG_ID/test-nmp-1"*"{"*"owner"*"$NMP_ORG_ID/$USERNAME"*"label"*"nmp test 2"*"description"*"test nmp 2"*"constraints"*"purpose==nmp-testing"*"properties"*"name"*"iame2edev"*"value"*"true"*"name"*"NOAGENTAUTO"*"value"*"false"*"patterns"*"[]"*"enabled"*"true"*"start"*"now"*"startWindow"*"0"*"agentUpgradePolicy"*"{"*"manifest"*"manifest_2.0.0"*"allowDowngrade"*"false"*"}"*"}"
NMP_OUTPUT2="$NMP_ORG_ID/test-nmp-2"*"{"*"owner"*"$NMP_ORG_ID/$USERNAME"*"label"*"nmp test 3"*"description"*"\"\""*"constraints"*"[]"*"properties"*"[]"*"patterns"*"[]"*"enabled"*"false"*"start"*"\"\""*"startWindow"*"0"*"agentUpgradePolicy"*"{"*"manifest"*"\"\""*"allowDowngrade"*"false"*"}"*"}"

echo -e "${PREFIX} Testing '$CMD_PREFIX' --long when 1 nmp exists in the Exchange"
cmdOutput=$($CMD_PREFIX -l -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"{"*$NMP_OUTPUT1*"}"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' --long when 1 nmp exists in the Exchange: exit code: $rc, output: $cmdOutput, expected: $NMP_OUTPUT1"
	cleanup
	exit 1
fi

echo -e "${PREFIX} adding second test nmp to exchange..."
hzn ex nmp add test-nmp-2 -f /tmp/nmp_example_3.json -v --no-constraints -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH &> /dev/null 
if [[ $? != 0 ]]; then 
	echo -e "${PREFIX} failed to add nm policy"
	cleanup
	exit 1
fi
echo -e "${PREFIX} done."

echo -e "${PREFIX} Testing '$CMD_PREFIX' when 2 nmp's exist in the Exchange"
cmdOutput=$($CMD_PREFIX -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 0 && ("$cmdOutput" == *"["*"$NMP_ORG_ID/test-nmp-1"*"$NMP_ORG_ID/test-nmp-2"*"]"* || "$cmdOutput" == *"["*"$NMP_ORG_ID/test-nmp-2"*"$NMP_ORG_ID/test-nmp-1"*"]"*) ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' when 2 nmp's exist in the Exchange: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' --long when 2 nmp's exist in the Exchange"
cmdOutput=$($CMD_PREFIX -l -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 0 && ("$cmdOutput" == *"{"*$NMP_OUTPUT1*$NMP_OUTPUT2*"}"* || "$cmdOutput" == *"{"*$NMP_OUTPUT2*$NMP_OUTPUT1*"}"*) ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' --long when 2 nmp's exist in the Exchange: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' [<nmp-name>] --long when 2 nmp's exist in the Exchange"
cmdOutput=$($CMD_PREFIX test-nmp-1 -l -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"{"*$NMP_OUTPUT1*"}"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' when 2 nmp's exist in the Exchange: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' with incorrect nmp-name"
cmdOutput=$($CMD_PREFIX fake-nmp -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 8 && "$cmdOutput" == *"Error: NMP fake-nmp not found in org"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' with incorrect nmp-name: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

# -----------------------------------
# ------- NODE MANAGEMENT LIST ------
# -----------------------------------

CMD_PREFIX="hzn exchange node management list"
test_env_variables "$CMD_PREFIX fakenode" true

echo -e "${PREFIX} Removing remaining nmp's in the Exchange"
hzn ex nmp rm test-nmp-1 -f -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH  &> /dev/null
if [[ $? != 0 ]]; then 
	echo -e "${PREFIX} failed to remove test-nmp-1"
	cleanup
	exit 1
fi
hzn ex nmp rm test-nmp-2 -f -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH &> /dev/null
if [[ $? != 0 ]]; then 
	echo -e "${PREFIX} failed to remove test-nmp-2"
	cleanup
	exit 1
fi
echo -e "${PREFIX} done."

echo -e "${PREFIX} Testing '$CMD_PREFIX' when no nmp's exists in the Exchange"
cmdOutput=$($CMD_PREFIX an12345 -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"[]"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' when no nmp's exists in the Exchange: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' --all when no nmp's exists in the Exchange"
cmdOutput=$($CMD_PREFIX an12345 --all -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"[]"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' --all when no nmp's exists in the Exchange: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} adding test nmp to exchange..."
hzn ex nmp add test-nmp-1 -f /tmp/nmp_example_2.json -v -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH &> /dev/null
if [[ $? != 0 ]]; then 
	echo -e "${PREFIX} failed to add test nmp to Exchange"
	cleanup
	exit 1
fi
echo -e "${PREFIX} done."

echo -e "${PREFIX} Testing '$CMD_PREFIX' when 1 nmp exists in the Exchange"
cmdOutput=$($CMD_PREFIX an12345 -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"{"*"$NMP_ORG_ID/test-nmp-1"*"enabled"*"}"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' when 1 nmp exists in the Exchange: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' --all when 1 nmp exists in the Exchange"
cmdOutput=$($CMD_PREFIX an12345 --all -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"{"*"$NMP_ORG_ID/test-nmp-1"*"enabled"*"}"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' --all when 1 nmp exists in the Exchange: exit code: $rc, output: $cmdOutput, expected: $NMP_OUTPUT1"
	cleanup
	exit 1
fi

echo -e "${PREFIX} adding second test nmp to exchange..."
sed -i 's/\"enabled\": true/\"enabled\": false/g' /tmp/nmp_example_2.json
hzn ex nmp add test-nmp-2 -f /tmp/nmp_example_2.json -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH &> /dev/null 
if [[ $? != 0 ]]; then 
	echo -e "${PREFIX} failed to add nm policy"
	cleanup
	exit 1
fi
echo -e "${PREFIX} done."

echo -e "${PREFIX} Testing '$CMD_PREFIX' when 2 nmp's exist in the Exchange and 1 is disabled"
cmdOutput=$($CMD_PREFIX an12345 -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"{"*"$NMP_ORG_ID/test-nmp-1"*"enabled"*"}"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' when 2 nmp's exist in the Exchange and 1 is disabled: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' --all when 2 nmp's exist in the Exchange"
cmdOutput=$($CMD_PREFIX an12345 --all -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 0 && ("$cmdOutput" == *"{"*"$NMP_ORG_ID/test-nmp-1"*"enabled"*"$NMP_ORG_ID/test-nmp-2"*"disabled"*"}"* || "$cmdOutput" == *"{"*"$NMP_ORG_ID/test-nmp-2"*"disabled"*"$NMP_ORG_ID/test-nmp-1"*"enabled"*"}"*"}"*) ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' --all when 2 nmp's exist in the Exchange: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' with incorrect node name"
cmdOutput=$($CMD_PREFIX fakenode -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 8 && "$cmdOutput" == *"Error: node 'fakenode' not found in org $NMP_ORG_ID"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' with incorrect node name: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

# -------------------------
# ------- NMP STATUS ------
# -------------------------

CMD_PREFIX="hzn exchange nmp status"
test_env_variables "$CMD_PREFIX fakenmp" true

# Manually add status to Exchange in case worker hasn't added it yet
echo -e "${PREFIX} adding test nmp status to exchange..."
curl -X PUT -u $NMP_ORG_ID/$NMP_EXCHANGE_USER_AUTH "$HZN_EXCHANGE_URL/orgs/userdev/nodes/an12345/managementStatus/test-nmp-1" -H "Content-Type: application/json" -d "$(cat /tmp/nmp_status_1.json)" &> /dev/null 
if [[ $? != 0 ]]; then 
	echo -e "${PREFIX} failed to add status for test-nmp-1"
	cleanup
	exit 1
fi
echo -e "${PREFIX} done."

echo -e "${PREFIX} Testing '$CMD_PREFIX' with disabled nmp"
cmdOutput=$($CMD_PREFIX test-nmp-2 -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 8 && "$cmdOutput" == *"Error: Status for NMP test-nmp-2 not found in org $NMP_ORG_ID"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' with disabled nmp: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' with enabled nmp"
cmdOutput=$($CMD_PREFIX test-nmp-1 -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"{"*"\"$NMP_ORG_ID/an12345\": \""*"\""*"}"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' with enabled nmp: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

STATUS_OUTPUT="{"*"$NMP_ORG_ID/test-nmp-1"*"{"*"agentUpgradePolicyStatus"*"{"*"scheduledTime"*"0001-01-01T00:00:00Z"*"upgradedVersions"*"{"*"softwareVersion"*"1.0.0"*"certVersion"*"2.0.0"*"configVersion"*"3.0.0"*"}"*"status"*"waiting"*"}"*"}"*"}"
echo -e "${PREFIX} Testing '$CMD_PREFIX' --long with enabled nmp"
cmdOutput=$($CMD_PREFIX test-nmp-1 --long -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"{"*$STATUS_OUTPUT*"}"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' --long with enabled nmp: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

# -------------------------------------
# ------- NODE MANAGEMENT STATUS ------
# -------------------------------------

CMD_PREFIX="hzn exchange node management status"
test_env_variables "$CMD_PREFIX fakenode" true

echo -e "${PREFIX} Testing '$CMD_PREFIX' with incorrect node name"
cmdOutput=$($CMD_PREFIX fakenode -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 8 && "$cmdOutput" == *"Error: Statuses for node fakenode not found in org $NMP_ORG_ID"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' with incorrect node name: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX'"
cmdOutput=$($CMD_PREFIX an12345 -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"{"*"\"$NMP_ORG_ID/test-nmp-1\": \""*"\""*"}"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX': exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

STATUS_OUTPUT="$NMP_ORG_ID/test-nmp-1"*"{"*"agentUpgradePolicyStatus"*"{"*"scheduledTime"*"0001-01-01T00:00:00Z"*"upgradedVersions"*"{"*"softwareVersion"*"1.0.0"*"certVersion"*"2.0.0"*"configVersion"*"3.0.0"*"}"*"status"*"waiting"*"}"*"}"
echo -e "${PREFIX} Testing '$CMD_PREFIX' --long"
cmdOutput=$($CMD_PREFIX an12345 --long -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"{"*$STATUS_OUTPUT*"}"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' --long: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' --policy"
cmdOutput=$($CMD_PREFIX an12345 --policy test-nmp-1 -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"{"*"\"$NMP_ORG_ID/test-nmp-1\": \""*"\""*"}"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' --policy: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' --policy with disabled policy"
cmdOutput=$($CMD_PREFIX an12345 --policy test-nmp-2 -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 8 && "$cmdOutput" == *"Error: Node an12345 does not contain a status for test-nmp-2 in org $NMP_ORG_ID"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' --policy with disabled policy: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' --long --policy"
cmdOutput=$($CMD_PREFIX an12345 --policy test-nmp-1 --long -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"{"*$STATUS_OUTPUT*"}"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' --long: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

# Enable the other NMP to create status path in Exchange
echo -e "${PREFIX} enabling second test nmp in exchange..."
sed -i 's/\"enabled\": false/\"enabled\": true/g' /tmp/nmp_example_2.json
hzn ex nmp add test-nmp-2 -f /tmp/nmp_example_2.json -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH &> /dev/null 
if [[ $? != 0 ]]; then 
	echo -e "${PREFIX} failed to add nm policy"
	cleanup
	exit 1
fi
echo -e "${PREFIX} done."

echo -e "${PREFIX} adding test nmp status to exchange..."
curl -X PUT -u $NMP_ORG_ID/$NMP_EXCHANGE_USER_AUTH "$HZN_EXCHANGE_URL/orgs/userdev/nodes/an12345/managementStatus/test-nmp-2" -H "Content-Type: application/json" -d "$(cat /tmp/nmp_status_1.json)" &> /dev/null 
if [[ $? != 0 ]]; then 
	echo -e "${PREFIX} failed to add status for test-nmp-2"
	cleanup
	exit 1
fi
echo -e "${PREFIX} done."

echo -e "${PREFIX} Testing '$CMD_PREFIX' when there are 2 status objects in the Exchange"
cmdOutput=$($CMD_PREFIX an12345 -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 0 && ("$cmdOutput" == *"{"*"\"$NMP_ORG_ID/test-nmp-1\": \""*"\""*"\"$NMP_ORG_ID/test-nmp-2\": \""*"\""*"}"* || "$cmdOutput" == *"{"*"\"$NMP_ORG_ID/test-nmp-2\": \""*"\""*"\"$NMP_ORG_ID/test-nmp-1\": \""*"\""*"}"*) ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' when there are 2 status objects in the Exchange: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

STATUS_OUTPUT1="$NMP_ORG_ID/test-nmp-1"*"{"*"agentUpgradePolicyStatus"*"{"*"scheduledTime"*"0001-01-01T00:00:00Z"*"upgradedVersions"*"{"*"softwareVersion"*"1.0.0"*"certVersion"*"2.0.0"*"configVersion"*"3.0.0"*"}"*"status"*"waiting"*"}"*"}"
STATUS_OUTPUT2="$NMP_ORG_ID/test-nmp-2"*"{"*"agentUpgradePolicyStatus"*"{"*"scheduledTime"*"0001-01-01T00:00:00Z"*"upgradedVersions"*"{"*"softwareVersion"*"1.0.0"*"certVersion"*"2.0.0"*"configVersion"*"3.0.0"*"}"*"status"*"waiting"*"}"*"}"
echo -e "${PREFIX} Testing '$CMD_PREFIX' --long when there are 2 status objects in the Exchange"
cmdOutput=$($CMD_PREFIX an12345 --long -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 0 && ("$cmdOutput" == *"{"*$STATUS_OUTPUT1*$STATUS_OUTPUT2*"}"* || "$cmdOutput" == *"{"*$STATUS_OUTPUT2*$STATUS_OUTPUT1*"}"*) ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' --long when there are 2 status objects in the Exchange: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' --policy when there are 2 status objects in the Exchange"
cmdOutput=$($CMD_PREFIX an12345 --policy test-nmp-1 -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"{"*"\"$NMP_ORG_ID/test-nmp-1\": \""*"\""*"}"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' --policy when there are 2 status objects in the Exchange: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

echo -e "${PREFIX} Testing '$CMD_PREFIX' --long --policy when there are 2 status objects in the Exchange"
cmdOutput=$($CMD_PREFIX an12345 --policy test-nmp-1 --long -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 0 && "$cmdOutput" == *"{"*$STATUS_OUTPUT*"}"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' --long when there are 2 status objects in the Exchange: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

cleanup

echo -e "${PREFIX} Testing '$CMD_PREFIX' when there are no status objects in the Exchange"
cmdOutput=$($CMD_PREFIX an12345 -o $NMP_ORG_ID -u $NMP_EXCHANGE_USER_AUTH 2>&1)
rc=$?
if [[ $rc -eq 8 && "$cmdOutput" == *"Error: Statuses for node an12345 not found in org $NMP_ORG_ID"* ]]; then
	echo -e "${PREFIX} completed."
else
	echo -e "${PREFIX} Failed: Wrong error response from '$CMD_PREFIX' when there are no status objects in the Exchange: exit code: $rc, output: $cmdOutput."
	cleanup
	exit 1
fi

# -----------------------
# ------- Clean-up ------
# -----------------------

cleanup
echo -e "${PREFIX} Done."
