#!/bin/sh

# Check env vars that we know should be set to verify that everything is working
verify() {
  if [ "$2" = "" ]
  then
    echo "Error: $1 should be set but is not."
    exit 2
  fi
}

# If the container is running in the Horizon environment, then the Horizon platform env vars should all be there.
# Otherwise, assume it is running outside Horizon and running in a non-Horizon environment.

if [ "$HZN_HARDWAREID" != "" ]
then
  verify "HZN_RAM" "$HZN_RAM"
  verify "HZN_ARCH" "$HZN_ARCH"
  verify "HZN_CPUS" "$HZN_CPUS"
  verify "HZN_NODE_ID" "$HZN_NODE_ID"
  verify "HZN_ORGANIZATION" "$HZN_ORGANIZATION"
  verify "HZN_EXCHANGE_URL" "$HZN_EXCHANGE_URL"
  echo "All Horizon platform env vars verified."
else
  echo "Running outside Horizon, skip Horizon platform env var checks."
fi

verify "MY_S_VAR1" "$MY_S_VAR1"
echo "All Service variables verified."

/usr/local/bin/server &

# Keep everything alive
while :
do
  echo "Service helloservice running."
  sleep 10
done
