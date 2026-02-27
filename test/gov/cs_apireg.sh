#!/bin/bash

# Enable debug tracing when DEBUG=1 or RUNNER_DEBUG=1 (GitHub Actions debug mode).
if [ "${DEBUG:-0}" = "1" ] || [ "${RUNNER_DEBUG:-0}" = "1" ]; then
    set -x
fi

# Complete configuration, transition from configuring to configured
echo "Testing Configstate API"

read -dr '' newhzndevice <<EOF
{
  "state": "configured"
}
EOF

echo "Testing for config change in configstate API"
RES=$(echo "$newhzndevice" | curl -sS -X PUT -H "Content-Type: application/json" --data @- "$ANAX_API/node/configstate")

if [ "$RES" = "" ]
then
  echo -e "$newhzndevice \nresulted in empty response"
  exit 2
fi

ERR=$(echo "$RES" | jq -r ".error")
if [ "$ERR" != "null" ]
then
  echo -e "$newhzndevice \nresulted in incorrect response: $RES"
  exit 2
else
  echo -e "found expected response: $RES"
fi
