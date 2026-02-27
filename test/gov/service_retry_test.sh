#!/bin/bash

# Enable debug tracing when DEBUG=1 or RUNNER_DEBUG=1 (GitHub Actions debug mode).
if [ "${DEBUG:-0}" = "1" ] || [ "${RUNNER_DEBUG:-0}" = "1" ]; then
    set -x
fi

PREFIX="Service retry test:"
CPU_CONTAINER_ID=""

# this function gets the cpu container docker id
get_cpu_container_id() {
    CPU_CONTAINER_ID=""

    # get the instance id of the cpu service with quotes removed
    if ! cpu_inst=$(curl -s "$ANAX_API/service" | jq -r '.instances.active[] | select (.ref_url == "https://bluehorizon.network/service-cpu") | select (.organization == "IBM")'); then
        echo -e "${PREFIX} failed to get cpu service instance."
        return 2
    fi
    inst_id=$(echo "$cpu_inst" | jq '.instance_id')
    inst_id="${inst_id%\"}"
    inst_id="${inst_id#\"}"

    # get the cpu service container for the main agent
    if ! cpu_container=$(docker ps | grep "$inst_id"); then
        echo -e "${PREFIX} cannot not find cpu container."
        return 2
    fi

    if ! docker_id=$(echo "$cpu_container" | cut -d ' ' -f1); then
        echo -e "${PREFIX} failed to get the cpu container id."
        return 2
    fi

    CPU_CONTAINER_ID=${docker_id}
}

# This verifies that there is a cpu container up and running.
verify_cpu_container() {
    # Look for cpu container to appear.
    LOOP_CNT=0
    CPU_CONTAINER_ID=""
    while [ "$LOOP_CNT" -le 30 ]
    do
        echo -e "${PREFIX} waiting for cpu container up and running"
        get_cpu_container_id
        if [ ! -z "${CPU_CONTAINER_ID}" ]; then
            echo -e "${PREFIX} found cpu container: ${CPU_CONTAINER_ID}"
            return 0
        fi
        (( LOOP_CNT+=1 ))
        sleep 10
    done    

    echo -e "${PREFIX} Timed out looking for cpu container."
    exit 1                 
}

echo -e "${PREFIX} starting"
if [ "$PATTERN" = "sloc" ] ||[ "$PATTERN" = "sall" ]; then

    expected_retry_duration=2400
    if [ "$PATTERN" = "sloc" ]; then
        expected_retry_duration=3600
    fi

    # get the docker id for the cpu container
    if ! get_cpu_container_id && [ -z "$CPU_CONTAINER_ID" ]; then
        exit 2
    fi

    if ! cpu_inst_before=$(curl -s "$ANAX_API/service" | jq -r '.instances.active[] | select (.ref_url == "https://bluehorizon.network/service-cpu") | select (.organization == "IBM")'); then
        echo -e "${PREFIX} failed to get cpu service instance."
        exit 2
    fi

    # delete the cpu container
    echo -e "${PREFIX} deleting cpu container ${CPU_CONTAINER_ID}"
    if ! docker rm -f "${CPU_CONTAINER_ID}" > /dev/null; then
        echo -e "${PREFIX} failed to delete cpu container ${CPU_CONTAINER_ID}"
        exit 2
    fi

    # waiting for cpu container
    if ! verify_cpu_container; then
        exit 1
    fi

    if ! cpu_inst_after=$(curl -s "$ANAX_API/service" | jq -r '.instances.active[] | select (.ref_url == "https://bluehorizon.network/service-cpu") | select (.organization == "IBM")'); then
        echo -e "${PREFIX} failed to get cpu service instance."
        exit 2
    fi

    # cpu is a dependent service, it's down time could cause either service retry or the cancellation of
    # all the associated agreements depending on the timing.  In the first case, the instance_id stays the same and
    # retry parameters will get set.
    instance_id_before=$(echo "$cpu_inst_before" | jq '.instance_id')
    instance_id_after=$(echo "$cpu_inst_after" | jq '.instance_id')
    if [ "$instance_id_before" = "$instance_id_after" ]; then
        echo -e "${PREFIX} retry happened."

        # checking the retry paramters
        max_retries=$(echo "$cpu_inst_after" | jq '.max_retries')
        max_retry_duration=$(echo "$cpu_inst_after" | jq '.max_retry_duration')
        current_retry_count=$(echo "$cpu_inst_after" | jq '.current_retry_count')
        retry_start_time=$(echo "$cpu_inst_after" | jq '.retry_start_time')
        if [ "$max_retries" != "2" ] || [ "$max_retry_duration" != "$expected_retry_duration" ] || [ "$current_retry_count" != "2" ] || [ "$retry_start_time" = "0" ]; then
             echo -e "${PREFIX} retry parameters are not right: max_retries=$max_retries, max_retry_duration=$max_retry_duration, current_retry_count=$current_retry_count, retry_start_time=$retry_start_time"
            exit 2
        fi
    else
        echo -e "${PREFIX} retry did not happen, but cpu service down time triggered a new agreement. This is okay."
    fi

    echo -e "${PREFIX} done "
else
    echo -e "${PREFIX} the pattern is not sloc or sall, skip this test." 
fi

