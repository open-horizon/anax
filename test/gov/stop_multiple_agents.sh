#!/bin/bash
  
# this script is called by the "make stop" to stop the multiple agents

# get containers with name horizonx where x is a number
containers=$(docker ps -a --format '{{.Names}}' -f name=^horizon)
if [ $? -ne 0 ]; then
    echo -e "Failed to get docker containers: ${containers}"
    exit 1
fi

# delete agent containers one by one
for cont_name in $containers; do
    echo "Delete agent container $cont_name ..."
    horizon_num=$(echo "${cont_name//[^0-9]/}")
    let agent_port=$horizon_num+8506
    ret=$(docker exec -e HORIZON_URL=http://localhost:${agent_port} ${cont_name} hzn unregister -f -r)
    echo "$ret"
    ${HC_BASE}/horizon-container stop ${horizon_num}

    # forcfuly remove the agent containers, just in case
    docker rm -f ${cont_name} 2>/dev/null || true
    docker volume rm "${cont_name}_var" "${cont_name}_etc" 2>/dev/null || true
done
