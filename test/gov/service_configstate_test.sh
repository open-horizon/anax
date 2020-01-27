#!/bin/bash

PREFIX="Service config state test:"

E2EDEV_NETSPEED_AG_ID=""
E2EDEV_LOCATION_AG_ID=""


# get the agreements ids for e2edev@somecomp.com/netspeed and e2edev@somecomp.com/location services.
function getNetspeedLocationAgreements {
	E2EDEV_NETSPEED_AG_ID=$(hzn agreement list | jq '.[] | select(.workload_to_run.url == "https://bluehorizon.network/services/netspeed") | select(.workload_to_run.org == "e2edev@somecomp.com") | .current_agreement_id')
	echo -e "${PREFIX} agreement for e2edev@somecomp.com/netspeed: $E2EDEV_NETSPEED_AG_ID"
	E2EDEV_LOCATION_AG_ID=$(hzn agreement list | jq '.[] | select(.workload_to_run.url == "https://bluehorizon.network/services/location") | select(.workload_to_run.org == "e2edev@somecomp.com") | .current_agreement_id')
	echo -e "${PREFIX} agreement for e2edev@somecomp.com/location: $E2EDEV_LOCATION_AG_ID"
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
	out=$(docker ps | grep locgps)
	ret=$?
	if ([ "$1" == "up" ] && [ $ret -ne 0 ]) || ([ "$1" == "down" ] && [ $ret -eq 0 ]); then
		echo -e "${PREFIX} container for e2edev@somecomp.com/locgps is not $1."
		return 1
	fi
	out=$(docker ps | grep cpu | grep -v "my.company.com" | grep e2edev)
	ret=$?
	if ([ "$1" == "up" ] && [ $ret -ne 0 ]) || ([ "$1" == "down" ] && [ $ret -eq 0 ]); then
		echo -e "${PREFIX} container for e2edev@somecomp.com/cpu is not $1."
		return 1
	fi

	# only check if the containers for IBM/cpu are up.
	# not checking down state because it is used by another service which is now down.
	out=$(docker ps | grep cpu | grep IBM)
	ret=$?
	if [ "$1" == "up" ] && [ $ret -ne 0 ]; then
		echo -e "${PREFIX} container for IBM/cpu is not up."
		return 1
	fi
}

# main code starts here
if [ "${PATTERN}" != "sall" ]; then
	echo -e "${PREFIX} will not perform this test because the pattern is not sall."
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

saved_ns_ag=$E2EDEV_NETSPEED_AG_ID
saved_loc_ag=$E2EDEV_LOCATION_AG_ID

# check the containers exist
echo -e "${PREFIX} checking containers..."
checkNetspeedLocationContainers "up" "$saved_ns_ag" "$saved_loc_ag"
if [ $? -ne 0 ]; then
	echo -e "${PREFIX} failed checking containers."
	exit 2
fi

# suspending the two servicess: e2edev@somecomp.com/netspeed, e2edev@somecomp.com/location
echo -e "${PREFIX} suspending the e2edev@somecomp.com/netspeed service..."
out=$(hzn service configstate suspend e2edev@somecomp.com https://bluehorizon.network/services/netspeed -f)
if [ $? -ne 0 ]; then
	echo -e "${PREFIX} error suspending e2edev@somecomp.com/netspeed: $out"
    exit 2
fi
echo -e "${PREFIX} suspending the e2edev@somecomp.com/location service..."
out=$(hzn service configstate suspend e2edev@somecomp.com https://bluehorizon.network/services/location -f)
if [ $? -ne 0 ]; then
	echo -e "${PREFIX} error suspending e2edev@somecomp.com/location: $out"
    exit 2
fi

# make sure the service configstate is suspended
echo -e "${PREFIX} checking service config state..."
location_configstate=$(hzn service configstate list | jq '.configstates[] | select(.url == "https://bluehorizon.network/services/location") | select(.org == "e2edev@somecomp.com") |.configState')
if [ "$location_configstate" != "\"suspended\"" ]; then
  echo -e "${PREFIX} error: the e2edev@somecomp.com/location service is still in the 'active' state."
  exit 2
else
  echo -e "${PREFIX} e2edev@somecomp.com/location service: suspended"
fi
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
if [ "${EXCH_APP_HOST}" != "http://exchange-api:8080/v1" ]; then
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
		echo -e "${PREFIX} making sure the agreements are canceled..."
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
out=$(hzn service configstate resume e2edev@somecomp.com https://bluehorizon.network/services/location)
if [ $? -ne 0 ]; then
	echo -e "${PREFIX} error resuming e2edev@somecomp.com/location: $out"
    exit 2
fi

# make sure the new configstate is set
echo -e "${PREFIX} checking service config state..."
location_configstate=$(hzn service configstate list | jq '.configstates[] | select(.url == "https://bluehorizon.network/services/location") | select(.org == "e2edev@somecomp.com") |.configState')
if [ "$location_configstate" != "\"active\"" ]; then
  echo -e "${PREFIX} error: the e2edev@somecomp.com/location service is still in the 'suspended' state."
  exit 2
else
  echo -e "${PREFIX} e2edev@somecomp.com/location service: active"
fi
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
  			echo -e "${PREFIX} error: cannot find agreement for e2edev@somecomp.com/netspeed."
  			continue
		fi
		if [ "$E2EDEV_LOCATION_AG_ID" == "" ]; then
  			echo -e "${PREFIX} error: cannot find agreement for e2edev@somecomp.com/location."
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
