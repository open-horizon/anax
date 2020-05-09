#!/bin/bash

CONTAINERID=/swagger-validator-v2
VALIDATOR_URI=localhost:8050
EXCHANGE_URI=localhost:8090
ESS_URI=https://raw.githubusercontent.com/open-horizon/edge-sync-service/master/swagger.json

docker pull swaggerapi${CONTAINERID}

docker run -d -p 8050:8080/tcp -e "REJECT_LOCAL=false" -e "REJECT_REDIRECT=false" --name swagger-validator-v2 swaggerapi${CONTAINERID}

curl -Ss ${EXCHANGE_URI}/v1/api-docs/swagger.json > /tmp/swagger.json

while [[ "$(curl -s -o /dev/null -w "%{http_code}" ${VALIDATOR_URI})" != "200" ]]
do
	sleep 5
done

EXCHANGEOUTPUT=$(curl -Ss -d @/tmp/swagger.json -H 'Content-Type:application/json' ${VALIDATOR_URI}/validator/debug)

if [ "$EXCHANGEOUTPUT" == "{}" ]
then
	echo -e "No errors in the exchange swagger\n"
else
	echo -e "The errors in the exchange swagger are as follows:\n"
	echo ${EXCHANGEOUTPUT} | jq
fi

curl -Ss ${ESS_URI} > /tmp/swagger.json

ESSOUTPUT=$(curl -Ss -d @/tmp/swagger.json -H 'Content-Type:application/json' ${VALIDATOR_URI}/validator/debug)

if [ "$ESSOUTPUT" == "{}" ]
then
	echo -e "No errors in the ESS swagger\n"
else
	echo -e "The errors in the ESS swagger are as follows:\n"
	echo ${ESSOUTPUT} | jq
fi

docker kill $(docker ps -aqf 'name=/swagger-validator-v2')
docker rm $(docker ps -aqf 'name=/swagger-validator-v2')