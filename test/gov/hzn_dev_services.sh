#!/bin/bash

# Reusable functions

# Verify a response. The inputs are:
# $1 - the response
# $2 - expected result
# $3 - error message
function verify {
    respContains=$(echo $1 | grep "$2")
    if [ "${respContains}" == "" ]; then
        echo -e "\nERROR: $3. Output was:"
        echo -e "$1"
        exit 1
    fi
}

# Create and edit a new hzn dev service project. The inputs are:
# $1 - project directory
# $2 - project name
# $3 - correct container execution output
# $4 - serviceURL
# $5 - sharable setting
# $6 - userinput variable name
# $7 - userinput variable type
# $8 - userinput variable value
# $9 - deployment config service name
# $10 - MaxMemory config
# $11 - NanoCpus config
function createProject {
    echo -e "Building $2 service container."
    cd $1

    buildOut=$(make 2>&1)

    verify "${buildOut}" "Successfully built" "$2 container did not build"
    if [ $? -ne 0 ]; then exit $?; fi

    verify "${buildOut}" "$3" "$2 container did not produce output"
    if [ $? -ne 0 ]; then exit $?; fi

    buildStop=$(make stop 2>&1)

    echo -e "Removing any existing working directory content"
    rm -rf $1/horizon

    echo -e "Creating Horizon $2 service project."

    newProject=$(hzn dev service new -s $4 -V 1.0.0 -i "localhost:443/amd64_$9:1.0" --noImageGen --noPattern 2>&1)
    verify "${newProject}" "Created horizon metadata" "Horizon project was not created"
    if [ $? -ne 0 ]; then exit $?; fi

    echo -e "Editing $2 project metadata."
    serviceDef=$1/horizon/service.definition.json
    userInput=$1/horizon/userinput.json
    serviceURL=$4

    sed -e 's|"label": "$SERVICE_NAME for $ARCH"|"label": "'$2'service"|' ${serviceDef} > ${serviceDef}.tmp && mv ${serviceDef}.tmp ${serviceDef}
    sed -e 's|"description": ""|"description": "'$2' service"|' ${serviceDef} > ${serviceDef}.tmp && mv ${serviceDef}.tmp ${serviceDef}
    sed -e 's|"sharable": "multiple"|"sharable": "'$5'"|' ${serviceDef} > ${serviceDef}.tmp && mv ${serviceDef}.tmp ${serviceDef}
    sed -e 's|"name": ""|"name": "'$6'"|' ${serviceDef} > ${serviceDef}.tmp && mv ${serviceDef}.tmp ${serviceDef}
    sed -e 's|"type": ""|"type": "'$7'"|' ${serviceDef} > ${serviceDef}.tmp && mv ${serviceDef}.tmp ${serviceDef}
    sed -e 's|"label": ""|"label": "'$6'"|' ${serviceDef} > ${serviceDef}.tmp && mv ${serviceDef}.tmp ${serviceDef}
    sed -e 's|"defaultValue": ""|"defaultValue": "'$8'"|' ${serviceDef} > ${serviceDef}.tmp && mv ${serviceDef}.tmp ${serviceDef}

    if [ "${10}" != "" ]; then
      jq_filter=.deployment.services.amd64_$9.max_memory_mb=${10}
      jq $jq_filter ${serviceDef} > ${serviceDef}.tmp && mv ${serviceDef}.tmp ${serviceDef}
    fi

    if [ "${11}" != "" ]; then
      jq_filter=.deployment.services.amd64_$9.max_cpus=${11}
      jq $jq_filter ${serviceDef} > ${serviceDef}.tmp && mv ${serviceDef}.tmp ${serviceDef}
    fi

    echo -e "Verifying the $2 project."
    verifyProject=$(hzn dev service verify -v 2>&1)

    verify "${verifyProject}" "verified" "Horizon $2 project was not verifiable"
    if [ $? -ne 0 ]; then exit $?; fi
}

# Stop the services that are started in the hzn dev test environment. Implicitly uses
# the horizon project in PWD.
function stopServices {
    echo -e "Stopping the top level service in the Horizon test environment."
    stopDev=$(hzn dev service stop -v 2>&1)
    stoppedServices=$(echo ${stopDev} | grep -c "Stopped service.")
    if [ "${stoppedServices}" != "1" ]; then
        echo -e "${stoppedServices}"
        echo -e "\nERROR: Did not detect services stopped. Output was:"
        echo -e "${stopDev}"
        exit 1
    fi
}

# Deploy a new hzn dev service project. The inputs are:
# $1 - project directory
# $2 - project name
function deploy {
    cd $1
    deploy=$(hzn exchange service publish -v -k $KEY_TEST_DIR/*private.key -K $KEY_TEST_DIR/*public.pem -f ./horizon/service.definition.json 2>&1)
    deploying=$(echo ${deploy} | grep "HTTP code: 201")
    if [ "${deploying}" == "" ]; then
        echo -e "\nERROR: $2 did not deploy. Output was:"
        echo -e "${deploy}"
        exit 1
    fi
    echo -e "$2 service deployed."
}

# Undeploy a new hzn dev service project. The input is:
# $1 - service
function undeploy {
    undeploy=$(hzn exchange service remove -f $1)
    echo -e "$1 service undeployed."
}

# Check configured MaxMemory and NanoCpus for the service. The input is:
# $1 - service
# $2 - expected MaxMemory
# $3 - expected NanoCpus
function checkMemoryAndCpus {
    echo -e "Checking custom MaxMemory and NanoCpus for $1."
    service_id=$(docker ps -qf "name=$1")
    svc_memory=$(docker inspect $service_id | jq -r '.[0].HostConfig.Memory')
    svc_nano_cpus=$(docker inspect $service_id | jq -r '.[0].HostConfig.NanoCpus')

    if [ "$svc_memory" -ne $2 ]; then
      echo -e "${PREFIX} MaxMemory verification for $1 service failed."
      stopServices
      exit 1
    fi
    if [ "$svc_nano_cpus" -ne $3 ]; then
      echo -e "${PREFIX} MaxCPUs verification for $1 service failed."
      stopServices
      exit 1
    fi
}

# ============= Main =================================================
#
echo -e "Begin hzn dev service testing."

export HZN_ORG_ID="e2edev@somecomp.com"
export HZN_EXCHANGE_URL=$1
export ARCH=$(uname -m | sed -e 's/aarch64.*/arm64/' -e 's/x86_64.*/amd64/' -e 's/armv.*/arm/')
E2EDEV_ADMIN_AUTH=$2
CLEAN_UP=$3

PROJECT_HOME="/root/hzn/service"

CPU_HOME=${PROJECT_HOME}/cpu
HELLO_HOME=${PROJECT_HOME}/hello
USEHELLO_HOME=${PROJECT_HOME}/usehello

# ============= Service creation =====================================
#

createProject "${CPU_HOME}" "CPU" "\"cpu\":" "my.company.com.services.cpu2" "singleton" "MY_CPU_VAR" "string" "cpuVarValue" "cpu"
if [ $? -ne 0 ]; then exit $?; fi

createProject "${HELLO_HOME}" "Hello" "Star Wars" "my.company.com.services.hello2" "multiple" "MY_S_VAR1" "string" "inside" "helloservice"
if [ $? -ne 0 ]; then exit $?; fi

createProject "${USEHELLO_HOME}" "UseHello" "variables verified." "my.company.com.services.usehello2" "singleton" "MY_VAR1" "string" "inside" "usehello" "512" "0.5"
if [ $? -ne 0 ]; then exit $?; fi

# ============= Connect dependencies =================================

echo -e "Creating dependencies."

cd ${HELLO_HOME}

depCreate=$(hzn dev dependency fetch -p ${CPU_HOME}/horizon -v 2>&1)
verify "${depCreate}" "New dependency created" "Could not create hello dependency on CPU."

echo -e "Verifying the Hello project."
verifyProject=$(hzn dev service verify -v 2>&1)

verify "${verifyProject}" "verified" "Horizon Hello project was not verifiable"
if [ $? -ne 0 ]; then exit $?; fi

cd ${USEHELLO_HOME}
depCreate=$(hzn dev dependency fetch -p ${CPU_HOME}/horizon -v 2>&1)
verify "${depCreate}" "New dependency created" "Could not create usehello dependency on CPU."

depCreate=$(hzn dev dependency fetch -p ${HELLO_HOME}/horizon -v 2>&1)
verify "${depCreate}" "New dependency created" "Could not create usehello dependency on hello."

echo -e "Verifying the UseHello project."
verifyProject=$(hzn dev service verify -v 2>&1)

verify "${verifyProject}" "verified" "Horizon UseHello project was not verifiable"
if [ $? -ne 0 ]; then exit $?; fi

# ============= Start the top level service in the hzn test environment ============

echo -e "Starting the top level service in the Horizon test environment."

startDev=$(hzn dev service start -v -m /root/resources/private/basicres/basicres.tgz -m /root/resources/private/multires/multires.tgz -t model 2>&1)
startedServices=$(echo ${startDev} | sed 's/Running service./Running service.\n/g' | grep -c "Running service.")
if [ "${startedServices}" != "3" ]; then
    echo -e "${startedServices}"
    echo -e "\nERROR: Did not detect 3 services started. Output was:"
    echo -e "${startDev}"
    stopServices
    exit 1
fi

echo -e "Waiting for services to run a bit before stopping them."
sleep 15

containers=$(docker ps -a)
restarting=$(echo ${containers} | grep "Restarting")
if [ "${restarting}" != "" ]; then
    echo -e "\nERROR: One of the containers is restarting. Output was:"
    echo -e "${containers}"
    stopServices
    exit 1
fi

# make sure max memory and max CPUs for usehello service are configured correctly (512 MB & 0.5 CPUs)
checkMemoryAndCpus amd64_usehello 536870912 500000000

stopServices

# ============= Deploy the services ==================================

echo -e "Deploying services."

KEY_TEST_DIR="/tmp/keytest"
mkdir -p $KEY_TEST_DIR

cd $KEY_TEST_DIR
ls *.key &> /dev/null
if [ $? -eq 0 ]
then
    echo -e "Using existing key"
else
  echo -e "Generate new signing keys:"
  hzn key create -l 4096 e2edev@somecomp.com e2edev@gmail.com -d .
  if [ $? -ne 0 ]
  then
    echo -e "hzn key create failed."
    exit 2
  fi
fi

echo -e "Logging into the e2edev@somecomp.com docker registry."
echo ${DOCKER_REG_PW} | docker login -u=${DOCKER_REG_USER} --password-stdin localhost:443

if [ $? -ne 0 ]
then
    echo -e "docker login failed."
    exit 1
fi

deploy ${CPU_HOME} "CPU"
if [ $? -ne 0 ]; then exit $?; fi

deploy ${HELLO_HOME} "Hello"
if [ $? -ne 0 ]; then exit $?; fi

deploy ${USEHELLO_HOME} "UseHello"
if [ $? -ne 0 ]; then exit $?; fi

sleep 5

# ============= Clean Up ==================================

if [ $CLEAN_UP -ne 0 ]
then

  echo -e "Undeploying services."

  undeploy my.company.com.services.cpu2_1.0.0_amd64
  undeploy my.company.com.services.hello2_1.0.0_amd64
  undeploy my.company.com.services.usehello2_1.0.0_amd64

  echo -e "Removing keys"

  rm -rf $KEY_TEST_DIR/*public.pem
  rm -rf $KEY_TEST_DIR/*private.key
  rm -rf /root/.colonus/*public.pem

fi

echo -e "End of hzn dev service testing: success."
