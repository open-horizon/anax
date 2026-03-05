#!/bin/bash

# Enable debug tracing when DEBUG=1 or RUNNER_DEBUG=1 (GitHub Actions debug mode).
if [ "${DEBUG:-0}" = "1" ] || [ "${RUNNER_DEBUG:-0}" = "1" ]; then
    set -x
fi

# test 1 ================================================
cat > /tmp/upgradetest.tmp <<'EOF'
{
  "agreementId": "1234567890"
}
EOF
upgradetest=$(cat /tmp/upgradetest.tmp)

echo -e "\n\n[D] test payload: $upgradetest"

echo "Trying with unknown policy name"

RES=$(echo "$upgradetest" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost:81/policy/fred/upgrade")
ERR=$(echo "$RES" | jq -r ".error")
if [ "$ERR" != "no policies with the name fred" ]
then
  echo -e "$upgradetest \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# test 2 ================================================
cat > /tmp/upgradetest.tmp <<'EOF'
EOF
upgradetest=$(cat /tmp/upgradetest.tmp)

echo -e "\n\n[D] test payload: $upgradetest"

echo "Trying with known policy, no input body"

RES=$(echo "$upgradetest" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost:81/policy/netspeed%20policy/upgrade")
ERR=$(echo "$RES" | jq -r ".error")
if [ "$ERR" != "user submitted data couldn't be deserialized to struct: . Error: unexpected end of JSON input" ]
then
  echo -e "$upgradetest \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# test 3 ================================================
cat > /tmp/upgradetest.tmp <<'EOF'
EOF
upgradetest=$(cat /tmp/upgradetest.tmp)

echo -e "\n\n[D] test payload: $upgradetest"

echo "Trying with known policy by file name, no input body"

RES=$(echo "$upgradetest" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost:81/policy/netspeed.policy/upgrade")
ERR=$(echo "$RES" | jq -r ".error")
if [ "$ERR" != "user submitted data couldn't be deserialized to struct: . Error: unexpected end of JSON input" ]
then
  echo -e "$upgradetest \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# test 4 ================================================
cat > /tmp/upgradetest.tmp <<'EOF'
{}
EOF
upgradetest=$(cat /tmp/upgradetest.tmp)

echo -e "\n\n[D] test payload: $upgradetest"

echo "Trying with known policy, empty input body"

RES=$(echo "$upgradetest" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost:81/policy/netspeed%20policy/upgrade")
ERR=$(echo "$RES" | jq -r ".error")
if [ "$ERR" != "must specify either device or agreementId" ]
then
  echo -e "$upgradetest \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# test 5 ================================================
cat > /tmp/upgradetest.tmp <<'EOF'
{
    "fred": 4
}
EOF
upgradetest=$(cat /tmp/upgradetest.tmp)

echo -e "\n\n[D] test payload: $upgradetest"

echo "Trying with known policy, missing required keywords"

RES=$(echo "$upgradetest" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost:81/policy/netspeed%20policy/upgrade")
ERR=$(echo "$RES" | jq -r ".error")
if [ "$ERR" != "must specify either device or agreementId" ]
then
  echo -e "$upgradetest \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# test 6 ================================================
cat > /tmp/upgradetest.tmp <<'EOF'
{
    "agreementId": "1234567890"
}
EOF
upgradetest=$(cat /tmp/upgradetest.tmp)

echo -e "\n\n[D] test payload: $upgradetest"

echo "Trying with known policy, unknown agreement id"

RES=$(echo "$upgradetest" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost:81/policy/netspeed%20policy/upgrade")
ERR=$(echo "$RES" | jq -r ".error")
if [ "$ERR" != "agreement id not found" ]
then
  echo -e "$upgradetest \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# test 7 ================================================
cat > /tmp/upgradetest.tmp <<'EOF'
{
    "device": "abcdef"
}
EOF
upgradetest=$(cat /tmp/upgradetest.tmp)

echo -e "\n\n[D] test payload: $upgradetest"

echo "Trying with known policy, unknown device id"

RES=$(echo "$upgradetest" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost:81/policy/netspeed%20policy/upgrade")
ERR=$(echo "$RES" | jq -r ".error")
if [ "$ERR" != "device abcdef with policy netspeed policy is not using the workload rollback feature" ]
then
  echo -e "$upgradetest \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# test 8 ================================================
cat > /tmp/upgradetest.tmp <<'EOF'
{
    "device": "abcdef"
}
EOF
upgradetest=$(cat /tmp/upgradetest.tmp)

echo -e "\n\n[D] test payload: $upgradetest"

echo "Trying with known policy file, unknown device id"

RES=$(echo "$upgradetest" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost:81/policy/netspeed.policy/upgrade")
ERR=$(echo "$RES" | jq -r ".error")
if [ "$ERR" != "device abcdef with policy netspeed policy is not using the workload rollback feature" ]
then
  echo -e "$upgradetest \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# test 9 ================================================

while :
do
    AGID=$(curl -sS http://localhost:81/agreement | jq -r '.agreements.active[0].current_agreement_id')
    if [ "$AGID" != "null" ]
    then
        break
    fi
    echo "waiting for a valid agreement id to appear"
    sleep 10
done

cat > /tmp/upgradetest.tmp <<EOF
{
    "agreementId": "$AGID"
}
EOF
upgradetest=$(cat /tmp/upgradetest.tmp)

echo -e "\n\n[D] test payload: $upgradetest"

echo "Trying with known policy and known agreement id that dont match"

RES=$(echo "$upgradetest" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost:81/policy/Never%20Netspeed/upgrade")
ERR=$(echo "$RES" | jq -r ".error")
if [ "$ERR" != "agreement $AGID not upgraded, not using policy Never Netspeed" ]
then
  echo -e "$upgradetest \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# test 10 ================================================
cat > /tmp/upgradetest.tmp <<EOF
{
    "agreementId": "$AGID"
}
EOF
upgradetest=$(cat /tmp/upgradetest.tmp)

echo -e "\n\n[D] test payload: $upgradetest"

echo "Trying with known policy file and known agreement id that dont match"

RES=$(echo "$upgradetest" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost:81/policy/never-netspeed.policy/upgrade")
ERR=$(echo "$RES" | jq -r ".error")
if [ "$ERR" != "agreement $AGID not upgraded, not using policy Never Netspeed" ]
then
  echo -e "$upgradetest \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi

# test 11 ================================================
cat > /tmp/upgradetest.tmp <<EOF
{
    "agreementId": "$AGID",
    "device": "abcdef"
}
EOF
upgradetest=$(cat /tmp/upgradetest.tmp)

echo -e "\n\n[D] test payload: $upgradetest"

echo "Trying with known policy and known agreement id but wrong device id"

RES=$(echo "$upgradetest" | curl -sS -X POST -H "Content-Type: application/json" --data @- "http://localhost:81/policy/Never%20Netspeed/upgrade")
ERR=$(echo "$RES" | jq -r ".error")
if [ "$ERR" != "agreement $AGID not upgraded, not with specified device id abcdef" ]
then
  echo -e "$upgradetest \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi
