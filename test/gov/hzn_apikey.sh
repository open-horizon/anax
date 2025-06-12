#!/bin/bash

PREFIX="hzn exchange user apikey test:"
echo -e "$PREFIX start"

# Get defaults if not explicitly set
if [[ -z "$HZN_ORG_ID" || "$HZN_ORG_ID" == *"e2edev@somecomp.com"* ]]; then
  export HZN_ORG_ID="userdev"
fi

if [[ -z "$HZN_EXCHANGE_USER_AUTH" || "$HZN_EXCHANGE_USER_AUTH" == *"e2edevadmin:e2edevadminpw"* ]]; then
  export HZN_EXCHANGE_USER_AUTH="userdevadmin:userdevadminpw"
fi

if [[ -z "$HZN_EXCHANGE_URL" ]]; then
  export HZN_EXCHANGE_URL="http://localhost:3090/v1"
fi

if command -v hzn >/dev/null 2>&1; then
  HZN_CMD="hzn"
elif [[ -x "../../cli/hzn" ]]; then
  HZN_CMD="../../cli/hzn"
else
  echo >&2 "$PREFIX 'hzn' command not found. Please install Horizon CLI or build it and ensure ./cli/hzn exists."
  exit 1
fi

# Check for jq
if ! command -v jq >/dev/null 2>&1; then
  echo >&2 "$PREFIX 'jq' command not found. Please install jq."
  exit 1
fi

# Extract username (after last slash, before colon)
USERNAME=$(echo "$HZN_EXCHANGE_USER_AUTH" | sed -E 's|.*/([^:]+):.*|\1|')
KEY_DESC="Integration test key"
KEY_ID=""

fail_exit() {
  echo -e "$PREFIX Failed: $1"
  exit 1
}

# List user and check if JSON is valid
list_output=$($HZN_CMD  exchange user list -o "$HZN_ORG_ID" -u "$HZN_EXCHANGE_USER_AUTH" 2>&1)
echo "$list_output" | jq . >/dev/null 2>&1 || fail_exit "user list did not return valid JSON"
echo -e "$PREFIX list ok"

# Create API key
create_output=$($HZN_CMD  exchange user createkey "$USERNAME" "$KEY_DESC" -o "$HZN_ORG_ID" -u "$HZN_EXCHANGE_USER_AUTH" 2>&1)
echo "$create_output" | jq -e '.id' >/dev/null || fail_exit "createkey response missing id"
KEY_ID=$(echo "$create_output" | jq -r '.id')
[[ -z "$KEY_ID" ]] && fail_exit "createkey failed to return key id"
echo -e "$PREFIX create ok: $KEY_ID"

# Get API key and check description
get_output=$($HZN_CMD  exchange user getkey "$USERNAME" "$KEY_ID" -o "$HZN_ORG_ID" -u "$HZN_EXCHANGE_USER_AUTH" 2>&1)
echo "$get_output" | jq -e --arg desc "$KEY_DESC" '.description == $desc' >/dev/null || fail_exit "getkey did not return correct description"
echo -e "$PREFIX get ok"

# Delete API key
$HZN_CMD  exchange user removekey "$USERNAME" "$KEY_ID" -f -o "$HZN_ORG_ID" -u "$HZN_EXCHANGE_USER_AUTH" >/dev/null 2>&1 || fail_exit "removekey failed"
echo -e "$PREFIX remove ok"

# Confirm deletion
check_output=$($HZN_CMD  exchange user getkey "$USERNAME" "$KEY_ID" -o "$HZN_ORG_ID" -u "$HZN_EXCHANGE_USER_AUTH" 2>&1 || true)
echo "$check_output" | grep -i 'not found' >/dev/null || fail_exit "getkey should have failed after deletion"
echo -e "$PREFIX verify deletion ok"

echo -e "$PREFIX success"
exit 0