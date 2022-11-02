#!/bin/bash

# checks the http code from a http call with "-w %{http_code}"
# $1 -- the expected code
# $2 -- the http call output
function check_api_result {

  rc="${2: -3}"
  output="${2::-3}"

  # check http code
  if [ "$rc" != $1 ]
  then
    echo -e "Error: $(echo "$output" | jq -r '.')\n"
    exit 2
  fi

  #statements
  echo -e "Result expected."
}


# $1 - Service url to wait for
# $2 - Service org to wait for
# $3 - Service version to wait for (optional)
# $4 - Service with error (bool, optional, default to false)
# please export ANAX_API, MAX_ITERATION(default 25)
function WaitForService() {
  current_svc_version=""
  TIMEOUT=0

  # set default 
  if [ -z "$MAX_ITERATION" ]; then
    MAX_ITERATION=25
  fi

  echo "ANAX_API=$ANAX_API, MAX_ITERATION=$MAX_ITERATION" 

  while [[ $TIMEOUT -le $MAX_ITERATION ]]
  do
    if [ "${3}" == "" ]; then
        echo -e "Waiting for service $2/$1 with any version."
        svc_inst=$(curl -s $ANAX_API/service | jq -r ".instances.active[] | select (.ref_url == \"$1\") | select (.organization == \"$2\")")
    else
        echo -e "Waiting for service $2/$1 with version $3."
        svc_inst=$(curl -s $ANAX_API/service | jq -r ".instances.active[] | select (.ref_url == \"$1\") | select (.organization == \"$2\") | select (.version == \"$3\")")
    fi
    if [ $? -ne 0 ]; then
        echo -e "Failed to get $1 service instace. ${svc_inst}"
        exit 2
    fi

    echo "svc_inst=$svc_inst"
    if [ "$4" == "true" ]; then
        echo -e "Found service $2/$1 with version $3. Checking for err service: $4"
	break
    elif [ "$svc_inst" != "" ]; then
        svc_start_time=$(echo "$svc_inst" |jq -r '.execution_start_time')
    fi

    if [ "$svc_inst" == "" ] || [ "$svc_start_time" == "0" ]; then
        sleep 5s
        ((TIMEOUT++))
    else
        echo -e "Found service $2/$1 with version $3."
        break
    fi

    if [[ $TIMEOUT == `expr $MAX_ITERATION + 1` ]]; then echo -e "Timeout waiting for service $1 to start"; exit 2; fi
  done
}
