#!/bin/bash

# Modify the anax config for the bluehorizon environment

# Check the exit status of the previously run command and exit if nonzero
checkrc() {
  if [[ $1 -ne 0 ]]; then
  	if [[ -n "$2" ]]; then
  		fromStr="from: $2"
  	else
  		fromStr="from the last command"
  	fi
    echo "Error: exit code $1 $fromStr"
    exit $1
  fi
}

# Update several fields in anax.json that are needed specifically for the bluehorizon environment
anaxJsonFile='/etc/horizon/anax.json'
echo "Modifying $anaxJsonFile for IBM Cloud stg..."

# Read the json object in /etc/horizon/anax.json
anaxJson=$(jq . $anaxJsonFile)
checkrc $? "read anax.json"
cp $anaxJsonFile $anaxJsonFile.orig
checkrc $? "back up anax.json"

# Change the value of ExchangeURL to point to bluehorizon staging
if [[ -n "$HZN_EXCHANGE_URL" ]]; then
	HZN_EXCHANGE_URL="${HZN_EXCHANGE_URL%/}/"  # make sure it ends in a slash
else
	HZN_EXCHANGE_URL=https://stg-edge-cluster.us-south.containers.appdomain.cloud/v1/
fi
echo "Setting exchange url to $HZN_EXCHANGE_URL"
anaxJson=$(jq ".Edge.ExchangeURL = \"$HZN_EXCHANGE_URL\" " <<< $anaxJson)
checkrc $? "change ExchangeURL"

# Write the new json back to the file
echo "$anaxJson" > $anaxJsonFile
checkrc $? "write anax.json"
