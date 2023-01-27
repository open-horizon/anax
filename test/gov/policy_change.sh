#!/bin/bash

export HZN_EXCHANGE_URL="${EXCH_APP_HOST}"
USERDEV_ADMIN_AUTH="userdev/userdevadmin:userdevadminpw"
E2EDEV_ADMIN_AUTH="e2edev@somecomp.com/e2edevadmin:e2edevadminpw"
PREFIX="policy change test "

timeout=6
pws_ag=$(hzn agreement list | jq -r '.[] | select(.name | contains("userdev/bp_pws")).current_agreement_id' )
netspeed_ag=$(hzn agreement list | jq -r '.[] | select(.name | contains("userdev/bp_netspeed")).current_agreement_id' )
while [[ "$pws_ag" == "" || "$netspeed_ag" == "" ]]; do
	sleep 5s
	if [[ $(hzn agreement list | jq -r '.[] | select(.name | contains("userdev/bp_pws")).agreement_execution_start_time') != "" ]]; then
		pws_ag=$(hzn agreement list | jq -r '.[] | select(.name | contains("userdev/bp_pws")).current_agreement_id' )
	fi
	if [[ $(hzn agreement list | jq -r '.[] | select(.name | contains("userdev/bp_netspeed")).agreement_execution_start_time' ) != "" ]]; then
		netspeed_ag=$(hzn agreement list | jq -r '.[] | select(.name | contains("userdev/bp_netspeed")).current_agreement_id' )
	fi
	let timeout=$timeout-1
	if [[ $timeout == 0 ]]; then
		echo "timed out waiting for agreements to be formed before starting test."
		echo "$(hzn agreement list)"
		exit 1
	fi
done

new_deployment_props=$(echo "{\"properties\": $(echo  $(hzn ex dep listpolicy bp_pws -u $USERDEV_ADMIN_AUTH -o userdev | jq '.[].properties += [{"name":"location","value":"buildingA"}]') | jq '.[].properties' )} ")
echo $new_deployment_props | hzn ex dep updatepolicy bp_pws -u $USERDEV_ADMIN_AUTH -o userdev -f-
if [ $? -ne 0 ]; then
  echo -e "${PREFIX} Failed to update deployment policy."
  exit 1
fi

new_service_pol=$(hzn ex service listpolicy e2edev@somecomp.com/bluehorizon.network-services-netspeed_2.3.0_amd64 -u $E2EDEV_ADMIN_AUTH -o e2edev@somecomp.com | jq '.properties += [{"name":"location","value":"buildingA"}]')
echo $new_service_pol | hzn ex service addpolicy e2edev@somecomp.com/bluehorizon.network-services-netspeed_2.3.0_amd64 -u $E2EDEV_ADMIN_AUTH -o "e2edev@somecomp.com" -f-
if [ $? -ne 0 ]; then
  echo -e "${PREFIX} Failed to update service policy."
  exit 1
fi

sleep 20s

new_node_pol=$(hzn policy list | jq '.deployment.constraints += ["location = buildingA"]')
echo $new_node_pol | hzn policy update -f-
if [ $? -ne 0 ]; then
  echo -e "${PREFIX} Failed to update node policy."
  exit 1
fi

timeout=6
while [[ $(hzn agreement list | jq 'length') > 2 && timeout > 0 ]]; do
	sleep 5s
	let timeout=$timeout-1
	if [ $timeout == 0 ]; then
		echo "timed out waiting for incompatible agreements to be cancelled."
		exit 1
	fi
done

echo "${PREFIX} $(hzn agreement list | jq 'length') agreements remain after policy changes."

new_pws_ag=$(hzn agreement list | jq -r '.[] | select(.name | contains("userdev/bp_pws")).current_agreement_id' )
new_netspeed_ag=$(hzn agreement list | jq -r '.[] | select(.name | contains("userdev/bp_netspeed")).current_agreement_id' )

if [[ $pws_ag != $new_pws_ag ]]; then
	echo "Error: agreement id for bp_pws changed \"$pws_ag\" \"$new_pws_ag\", indicating unexpected cancellation after compatible policy change."
	exit 1
fi

if [[ $netspeed_ag != $new_netspeed_ag ]]; then
	echo "Error: agreement id for bp_netspeed changed \"$netspeed_ag\" \"$new_netspeed_ag\", indicating unexpected cancellation after compatible policy change."
	exit 1
fi

new_node_pol=$(hzn policy list | jq '.deployment.constraints -= ["location = buildingA"]')
echo $new_node_pol | hzn policy update -f-
if [ $? -ne 0 ]; then
  echo -e "${PREFIX} Failed to update deployment policy."
  exit 1
fi

timeout=6
while [[ $(hzn agreement list | jq 'length') != 5 && timeout > 0 ]]; do
	sleep 5s
	let timeout=$timeout-1
	if [ $timeout == 0 ]; then
		echo "timed out waiting for agreements to be reformed."
		exit 1
	fi
done

echo "${PREFIX} 5 agreements restored after node policy reverted."
