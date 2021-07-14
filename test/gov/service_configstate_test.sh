#!/bin/bash

PREFIX="Service config state test:"

E2EDEV_NETSPEED_AG_ID=""
E2EDEV_LOCATION_AG_ID=""

EXCH_URL="${EXCH_APP_HOST}"
USERDEV_ADMIN_AUTH="userdev/userdevadmin:userdevadminpw"

if [ ${CERT_LOC} -eq "1" ]; then
  CERT_VAR="--cacert /certs/css.crt"
else
  CERT_VAR=""
fi

# check the result of http call
function results {
  if [ "$(echo "$1" | jq -r '.code')" != "ok" ]
  then
    echo -e "Error: $(echo "$1" | jq -r '.msg')"
    exit 2
  fi
}

# get the agreements ids for e2edev@somecomp.com/netspeed and e2edev@somecomp.com/location services.
function getNetspeedLocationAgreements {
	E2EDEV_NETSPEED_AG_ID=$(hzn agreement list | jq '.[] | select(.workload_to_run.url == "https://bluehorizon.network/services/netspeed") | select(.workload_to_run.org == "e2edev@somecomp.com") | .current_agreement_id')
	echo -e "${PREFIX} agreement for e2edev@somecomp.com/netspeed: $E2EDEV_NETSPEED_AG_ID"
	E2EDEV_LOCATION_AG_ID=$(hzn agreement list | jq '.[] | select(.workload_to_run.url == "https://bluehorizon.network/services/location") | select(.workload_to_run.org == "e2edev@somecomp.com") | .current_agreement_id')
	echo -e "${PREFIX} agreement for e2edev@somecomp.com/location: $E2EDEV_LOCATION_AG_ID"
}

# get if the docker container for the service with given org and url is up.
# $1 - service org
# $2 - service url
# returns 0 for up and non-zero for down
function checkContainer {
    # get the instance id of the cpu service with quotes removed
    service_inst=$(curl -s $ANAX_API/service | jq -r ".instances.active[] | select (.ref_url == \"${2}\") | select (.organization == \"${1}\")")
    if [ -n "$service_inst" ]; then
    	inst_id=$(echo "$service_inst" | jq '.instance_id')
    	inst_id="${inst_id%\"}"
    	inst_id="${inst_id#\"}"
    	out=$(docker ps | grep $inst_id)
    	return $?
    else
    	return 1
    fi
}

# check if the containers for e2edev@somecomp.com/netspeed and e2edev@somecomp.com/location services
# are up/down.
function checkNetspeedLocationContainers {
	# remove the quotes
	ns_ag="${2%\"}"
	ns_ag="${ns_ag#\"}"
	out=$(docker ps | grep $ns_ag)
	ret=$?
	if ([ "$1" == "up" ] && [ $ret -ne 0 ]) || ([ "$1" == "down" ] && [ $ret -eq 0 ]); then
		echo -e "${PREFIX} container for e2edev@somecomp.com/netspeed is not $1."
		return 1
	fi

	loc_ag="${3%\"}"
	loc_ag="${loc_ag#\"}"
	out=$(docker ps | grep $loc_ag)
	ret=$?
	if ([ "$1" == "up" ] && [ $ret -ne 0 ]) || ([ "$1" == "down" ] && [ $ret -eq 0 ]); then
		echo -e "${PREFIX} container for e2edev@somecomp.com/location is not $1."
		return 1
	fi

	checkContainer "e2edev@somecomp.com" "https://bluehorizon.network/services/locgps"
	ret=$?
	if [[ "$PATTERN" == "sall" ]]; then
		# in this pattern case, locgps is agreementless service, so it should stay up
		# all the time.  
		if [ $ret -ne 0 ]; then
			echo -e "${PREFIX} container for e2edev@somecomp.com/locgps is missing."
			return 1
		fi
	else
		if ([ "$1" == "up" ] && [ $ret -ne 0 ]) || ([ "$1" == "down" ] && [ $ret -eq 0 ]); then
			echo -e "${PREFIX} container for e2edev@somecomp.com/locgps is missing."
			return 1
		fi
	fi		

	checkContainer "e2edev@somecomp.com" "my.company.com.services.cpu2"
	ret=$?
	if [ $ret -ne 0 ]; then
		echo -e "${PREFIX} container for e2edev@somecomp.com/cpu is missing."
		return 1
	fi

	# only check if the containers for IBM/cpu are up.
	ret=$?
	checkContainer "IBM" "https://bluehorizon.network/service-cpu"
	if [ $ret -ne 0 ]; then
		echo -e "${PREFIX} container for IBM/cpu is not up."
		return 1
	fi
}

# main code starts here
if ([ "${PATTERN}" != "" ] && [ "${PATTERN}" != "sall" ]); then
	echo -e "${PREFIX} will not perform this test because the pattern is not sall and not a policy."
	exit 0
fi

# get current config state
echo -e "${PREFIX} making sure all the registered services are in the 'active' state."
output=$(hzn service configstate list | jq '.configstates[] | select(.configState == "suspended")')
if [ "$output" != "" ]; then
  echo -e "${PREFIX} error: the following services are in the 'suspended' state:\n $output"
  exit 2
fi

# check the agreements exist
getNetspeedLocationAgreements
if [ "$E2EDEV_NETSPEED_AG_ID" == "" ]; then
  echo -e "${PREFIX} error: cannot find agreement for e2edev@somecomp.com/netspeed."
  exit 2
fi
if [ "$E2EDEV_LOCATION_AG_ID" == "" ]; then
  echo -e "${PREFIX} error: cannot find agreement for e2edev@somecomp.com/location."
  exit 2
fi

# Sleeping to allow existing agreements to renew if needed
echo -e "${PREFIX} wait for 30 seconds..."
sleep 30

saved_ns_ag=$E2EDEV_NETSPEED_AG_ID
saved_loc_ag=$E2EDEV_LOCATION_AG_ID

# check the containers exist
echo -e "${PREFIX} checking containers..."
checkNetspeedLocationContainers "up" "$saved_ns_ag" "$saved_loc_ag"
if [ $? -ne 0 ]; then
	echo -e "${PREFIX} failed checking containers."
	exit 2
fi

# suspending the two services: e2edev@somecomp.com/netspeed, e2edev@somecomp.com/location
echo -e "${PREFIX} suspending the e2edev@somecomp.com/netspeed service..."
out=$(hzn service configstate suspend e2edev@somecomp.com https://bluehorizon.network/services/netspeed -f)
if [ $? -ne 0 ]; then
	echo -e "${PREFIX} error suspending e2edev@somecomp.com/netspeed: $out"
    exit 2
fi

echo -e "${PREFIX} suspending the e2edev@somecomp.com/location service..."
read -d '' sconfig <<EOF
{
   "url":         "https://bluehorizon.network/services/location",
   "org":          "e2edev@somecomp.com",
   "configState": "suspended"
}
EOF
out=$(echo "$sconfig" | curl -sLX POST $CERT_VAR -u $USERDEV_ADMIN_AUTH -H "Content-Type: application/json" -H "Accept: application/json" --data @- ${EXCH_URL}/orgs/userdev/nodes/an12345/services_configstate | jq -r '.')
results "$out"

# make sure the service configstate for netspeed is suspended
echo -e "${PREFIX} checking service config state for netspeed service..."
netspeed_configstate=$(hzn service configstate list | jq '.configstates[] | select(.url == "https://bluehorizon.network/services/netspeed") | select(.org == "e2edev@somecomp.com") |.configState')
if [ "$netspeed_configstate" != "\"suspended\"" ]; then
  echo -e "${PREFIX} error: the e2edev@somecomp.com/netspeed service is still in the 'active' state."
  exit 2
else
  echo -e "${PREFIX} e2edev@somecomp.com/netspeed service: suspended"
fi

loop_cnt=0
ag_canceled=0
test_good_togo=0
if [ "${EXCH_APP_HOST}" != "http://exchange-api:8081/v1" ]; then
  loop_max=40
else
  loop_max=18
fi

while [ $loop_cnt -le $loop_max ]
do
    let loop_cnt+=1
	echo -e "${PREFIX} wait for 10 seconds..."
    sleep 10

    if [ $ag_canceled -ne 1 ]; then
		# make sure the agreement is gone
		echo -e "${PREFIX} making sure the agreements are cancelled..."
		getNetspeedLocationAgreements
		if [ "$E2EDEV_NETSPEED_AG_ID" != "" ]; then
			echo -e "${PREFIX} error: agreement for e2edev@somecomp.com/netspeed not canceled."
			continue
		fi
		if [ "$E2EDEV_LOCATION_AG_ID" != "" ]; then
 			echo -e "${PREFIX} error: agreement for e2edev@somecomp.com/location not canceled."
			continue
		fi
	fi

	ag_canceled=1

	# make sure the containers are gone
	echo -e "${PREFIX} making sure the containers removed..."
	checkNetspeedLocationContainers "down" "$saved_ns_ag" "$saved_loc_ag"
	if [ $? -ne 0 ]; then
		continue
	else
		test_good_togo=1
		break
	fi
done

if [ $test_good_togo -ne 1 ]; then
	exit 2
fi

# make sure the configstate for location service is suspended
echo -e "${PREFIX} checking service config state for location service..."
location_configstate=$(hzn service configstate list | jq '.configstates[] | select(.url == "https://bluehorizon.network/services/location") | select(.org == "e2edev@somecomp.com") |.configState')
if [ "$location_configstate" != "\"suspended\"" ]; then
  echo -e "${PREFIX} error: the e2edev@somecomp.com/location service is still in the 'active' state."
  exit 2
else
  echo -e "${PREFIX} e2edev@somecomp.com/location service: suspended"
fi

echo -e "${PREFIX} wait for 10 seconds..."
sleep 10

# resume the services
echo -e "${PREFIX} resuming e2edev@somecomp.com/netspeed service..."
out=$(hzn service configstate resume e2edev@somecomp.com https://bluehorizon.network/services/netspeed)
if [ $? -ne 0 ]; then
	echo -e "${PREFIX} error resuming e2edev@somecomp.com/netspeed: $out"
    exit 2
fi
echo -e "${PREFIX} resuming e2edev@somecomp.com/location service..."
read -d '' sconfig <<EOF
{
   "url":         "https://bluehorizon.network/services/location",
   "org":          "e2edev@somecomp.com",
   "configState": "active"
}
EOF
out=$(echo "$sconfig" | curl -sLX POST $CERT_VAR -u $USERDEV_ADMIN_AUTH -H "Content-Type: application/json" -H "Accept: application/json" --data @- ${EXCH_URL}/orgs/userdev/nodes/an12345/services_configstate | jq -r '.')
results "$out"

# make sure the new configstate is set for netspeed
echo -e "${PREFIX} checking service config state for netspeed service..."
netspeed_configstate=$(hzn service configstate list | jq '.configstates[] | select(.url == "https://bluehorizon.network/services/netspeed") | select(.org == "e2edev@somecomp.com") |.configState')
if [ "$netspeed_configstate" != "\"active\"" ]; then
  echo -e "${PREFIX} error: the e2edev@somecomp.com/netspeed service is still in the 'suspended' state."
  exit 2
else
  echo -e "${PREFIX} e2edev@somecomp.com/netspeed service: active"
fi

# make sure the agreements and the containers are up
loop_cnt=0
ag_formed=0
while [ $loop_cnt -le $loop_max ]
do
    let loop_cnt+=1
	echo -e "${PREFIX} wait for 10 seconds..."
    sleep 10

    if [ $ag_formed -ne 1 ]; then
		echo -e "${PREFIX} making sure the agreements are formed..."
		getNetspeedLocationAgreements
		if [ "$E2EDEV_NETSPEED_AG_ID" == "" ]; then
			echo -e "${PREFIX} cannot find agreement for e2edev@somecomp.com/netspeed."
  			continue
		fi
		if [ "$E2EDEV_LOCATION_AG_ID" == "" ]; then
			echo -e "${PREFIX} cannot find agreement for e2edev@somecomp.com/location."
  			continue
		fi
	fi

	ag_formed=1

	echo -e "${PREFIX} making sure the containers are up and running..."
	checkNetspeedLocationContainers "up" "$E2EDEV_NETSPEED_AG_ID" "$E2EDEV_LOCATION_AG_ID"
	if [ $? -ne 0 ]; then
		continue
	else
		echo -e "${PREFIX} test successful! Done. "
		exit 0
	fi
done

if [ $ag_formed -ne 1 ]; then
	exit 2
fi

# make sure the new configstate is set for location service
echo -e "${PREFIX} checking service config state for location service..."
location_configstate=$(hzn service configstate list | jq '.configstates[] | select(.url == "https://bluehorizon.network/services/location") | select(.org == "e2edev@somecomp.com") |.configState')
if [ "$location_configstate" != "\"active\"" ]; then
  echo -e "${PREFIX} error: the e2edev@somecomp.com/location service is still in the 'suspended' state."
  exit 2
else
  echo -e "${PREFIX} e2edev@somecomp.com/location service: active"
fi