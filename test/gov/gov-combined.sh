#!/bin/bash

function set_exports {
    if [ "$NOANAX" != "1" ]
    then
        export USER=anax1
        export PASS=anax1pw
        export DEVICE_ID="an12345"
        export DEVICE_NAME="anaxdev1"
        export TOKEN="abcdefg"
        export ORG="e2edev"

        export ANAX_API="http://localhost"
        export EXCH="http://${EXCH_APP_HOST:-172.17.0.1}:8080/v1"

    else
        echo -e "Anax is disabled"
    fi
}

function run_delete_loops {
    # Start the deletion loop tests if they have not been disabled.
    echo -e "No loop setting is $NOLOOP"
    if [ "$NOLOOP" != "1" ] && [ "$NOAGBOT" != "1" ]
    then
        echo "Starting deletion loop tests. Giving time for 1st agreement to complete."
        sleep 240

        echo "Starting device delete agreement script"
        ./del_loop.sh &

        # Give the device script time to get started and get into it's 10 min cycle. Wait 5 mins
        # and then start the agbot delete cycle, so that it is interleaved with the device cycle.
        sleep 300
        ./agbot_del_loop.sh &
    else
        echo -e "Deletion loop tests set to only run once."

        if [ "${PATTERN}" == "sall" ] || [ "${PATTERN}" == "sloc" ] || [ "${PATTERN}" == "sns" ] || [ "${PATTERN}" == "sgps" ] || [ "${PATTERN}" == "spws" ] || [ "${PATTERN}" == "susehello" ] || [ "${PATTERN}" == "cpu2msghub" ]; then
            echo -e "Starting service pattern verification scripts"
            if [ "$NOLOOP" == "1" ]; then
                ./verify_agreements.sh
                if [ $? -ne 0 ]; then echo "Verify agreement failure."; exit 1; fi
                echo -e "No cancellation setting is $NOCANCEL"
                if [ "$NOCANCEL" != "1" ]; then
                    ./del_loop.sh
                    if [ $? -ne 0 ]; then echo "Agreement deletion failure."; exit 1; fi
                    echo -e "Sleeping for 30s between device and agbot agreement deletion"
                    sleep 30
                    ./agbot_del_loop.sh
                    if [ $? -ne 0 ]; then echo "Agbot agreement deletion failure."; exit 1; fi
                    ./verify_agreements.sh
                    if [ $? -ne 0 ]; then echo "Agreement restart failure."; exit 1; fi
                else
                    echo -e "Cancellation tests are disabled"
                fi
            else
                ./verify_agreements.sh &
            fi
        else
            echo -e "No cancellation setting is $NOCANCEL"
            if [ "$NOCANCEL" != "1" ]; then
                ./del_loop.sh
                if [ $? -ne 0 ]; then echo "Agreement deletion failure."; exit 1; fi
                echo -e "Sleeping for 30s between device and agbot agreement deletion"
                sleep 30
                ./agbot_del_loop.sh
                if [ $? -ne 0 ]; then echo "Agbot agreement deletion failure."; exit 1; fi
            else
                echo -e "Cancellation tests are disabled"
            fi
        fi
    fi
}

EXCH_URL="http://${EXCH_APP_HOST:-172.17.0.1}:8080/v1"

# Build an old anax if we need it
if [ "$OLDANAX" == "1" ] || [ "$OLDAGBOT" == "1" ]
then
    echo "Building old anax."
    chown -R root:root /root/.ssh
    mkdir -p /tmp/oldanax/anax-gopath/src/github.com/open-horizon
    mkdir -p /tmp/oldanax/anax-gopath/bin
    export GOPATH="/tmp/oldanax/anax-gopath"
    export TMPDIR="/tmp/oldanax/"
    cd /tmp/oldanax/anax-gopath/src/github.com/open-horizon
    git clone git@github.com:open-horizon/anax.git
    cd /tmp/oldanax/anax-gopath/src/github.com/open-horizon/anax
    make
    cp anax /usr/bin/old-anax
    export GOPATH="/tmp"
    unset TMPDIR
    cd /tmp
fi

# Clean up the exchange DB to make sure we start out clean
echo "Drop and recreate the exchange DB."

UPGRADEDB=$(curl -sLX POST --header 'Authorization:Basic root/root:Horizon-Rul3s' "${EXCH_URL}/admin/upgradedb" | jq -r '.msg')
echo "Exchange DB Upgrade Response: $UPGRADEDB"

# loop until DBTOK contains a string value
while :
do
    DBTOK=$(curl -sLX GET --header 'Authorization:Basic root/root:Horizon-Rul3s' "${EXCH_URL}/admin/dropdb/token" | jq -r '.token')
    if test -z "$DBTOK"
    then
        sleep 5
    else
        break
    fi
done

DROPDB=$(curl -sLX POST --header 'Authorization:Basic root/root:'$DBTOK "${EXCH_URL}/admin/dropdb" | jq -r '.msg')
echo "Exchange DB Drop Response: $DROPDB"

INITDB=$(curl -sLX POST --header 'Authorization:Basic root/root:Horizon-Rul3s' "${EXCH_URL}/admin/initdb" | jq -r '.msg')
echo "Exchange DB Init Response: $INITDB"

cd /root

# Create the organizations we need
echo "Creating e2edev organization..."
CR8EORG=$(curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:Horizon-Rul3s" -d '{"label":"E2EDev","description":"E2EDevTest"}' "${EXCH_URL}/orgs/e2edev" | jq -r '.msg')
echo "$CR8EORG"

echo "Creating IBM organization..."
CR8IORG=$(curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:Horizon-Rul3s" -d '{"label":"IBMorg","description":"IBM"}' "${EXCH_URL}/orgs/IBM" | jq -r '.msg')
echo "$CR8IORG"

# Register an e2edev admin user in the exchange
echo "Creating an admin user for e2edev organization..."
CR8EADM=$(curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:Horizon-Rul3s" -d '{"password":"e2edevadminpw","email":"me%40gmail.com","admin":true}' "${EXCH_URL}/orgs/e2edev/users/e2edevadmin" | jq -r '.msg')
echo "$CR8EADM"

# Register an IBM admin user in the exchange
echo "Creating an admin user for IBM org..."
CR8IBM=$(curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:Horizon-Rul3s" -d '{"password":"ibmadminpw","email":"ibmadmin%40ibm.com","admin":true}' "${EXCH_URL}/orgs/IBM/users/ibmadmin" | jq -r '.msg')
echo "$CR8IBM"

# Register agreement bot user in the exchange
echo "Creating Agbot user..."
CR8AGBOT=$(curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:Horizon-Rul3s" -d '{"password":"agbot1pw","email":"me%40gmail.com","admin":false}' "${EXCH_URL}/orgs/IBM/users/agbot1" | jq -r '.msg')
echo "$CR8AGBOT"

# Register IBM users in the exchange
echo "Creating Anax user..."
CR8ANAX=$(curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:Horizon-Rul3s" -d '{"password":"anax1pw","email":"me%40gmail.com","admin":false}' "${EXCH_URL}/orgs/e2edev/users/anax1" | jq -r '.msg')
echo "$CR8ANAX"


echo "Registering Anax device1..."
REGANAX1=$(curl -sLX PUT --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic e2edev/anax1:anax1pw" -d '{"token":"abcdefg","name":"anaxdev","registeredMicroservices":[],"msgEndPoint":"","softwareVersions":{},"publicKey":"","pattern":""}' "${EXCH_URL}/orgs/e2edev/nodes/an12345" | jq -r '.msg')
echo "$REGANAX1"

echo "Registering Anax device2..."
REGANAX2=$(curl -sLX PUT --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic e2edev/anax1:anax1pw" -d '{"token":"abcdefg","name":"anaxdev","registeredMicroservices":[],"msgEndPoint":"","softwareVersions":{},"publicKey":"","pattern":""}' "${EXCH_URL}/orgs/e2edev/nodes/an54321" | jq -r '.msg')
echo "$REGANAX2"

echo "Register services"
./service_apireg.sh
if [ $? -ne 0 ]
then
    echo -e "Service registration failure."
    TESTFAIL="1"
else
    echo "Register services SUCCESSFUL"
fi

if [ "$TESTFAIL" != "1" ]
then
    echo "Register microservices, workloads, and patterns"
    ./workload_apireg.sh
    if [ $? -ne 0 ]
    then
        echo -e "Microservice and workload registration failure."
        TESTFAIL="1"
    else
        echo "Register microservices and workloads SUCCESSFUL"
    fi
fi

# Register a microservice and workload via the hzn exchange commands
if [ "$TESTFAIL" != "1" ]
then
    echo "Register microservices, workloads, and patterns for keytest"
    ./keytest_reg.sh
    if [ $? -ne 0 ]
    then
        echo -e "hzn microservice, workload and pattern registration with signing keys failed."
        TESTFAIL="1"
    else
        echo "Register microservices and workloads for keytest SUCCESSFUL"
    fi
fi



TEST_MSGHUB=0
for pat in $(echo $TEST_PATTERNS | tr "," " "); do
    if [ "$pat" == "cpu2msghub" ]; then
        TEST_MSGHUB=1
    fi
done

# Register msghub services and patterns
if [ "$TESTFAIL" != "1" ]; then
    if [ $TEST_MSGHUB -eq 1 ]; then
        echo "Register services and patterns for msghub test"

        ./msghub_rsrcreg.sh
        if [ $? -ne 0 ]
        then
            echo -e "Service registration failure for msghub."
            TESTFAIL="1"
        else
            echo "Register services for msghub SUCCESSFUL"
        fi
    fi
fi

# Setup to use the anax registration APIs
if [ "$TESTFAIL" != "1" ]
then
    export USER=anax1
    export PASS=anax1pw
    export DEVICE_ID="an12345"
    export DEVICE_NAME="anaxdev1"
    export ANAX_API="http://localhost"
    export EXCH="http://${EXCH_APP_HOST:-172.17.0.1}:8080/v1"
    export TOKEN="abcdefg"
    export ORG="e2edev"

    #export mtn_soliditycontract=1

    # Start Anax
    echo "Starting Anax1 for tests."
    /usr/local/bin/anax -v=5 -alsologtostderr=true -config /etc/colonus/anax-combined.config >/tmp/anax.log 2>&1 &

    sleep 5

    TESTFAIL="0"
    echo "Running API tests"
    ./apitest.sh
    if [ $? -ne 0 ]
    then
        echo "API Test failure."
        TESTFAIL="1"
    else
        echo "API tests completed SUCCESSFULLY."

        echo "Killing anax and cleaning up."
        kill $(pidof anax)
        rm -fr /root/.colonus/*.db
        rm -fr /root/.colonus/policy.d/*
    fi
fi

echo -e "No agbot setting is $NOAGBOT"
if [ "$NOAGBOT" != "1" ] && [ "$TESTFAIL" != "1" ]
then

    AGBOT_AUTH="IBM/agbot1:agbot1pw"
    ORG="IBM"
    # Register agreement bot in the exchange

    echo "Registering Agbot instance1..."
    echo -e "PATTERN setting is $PATTERN"
    REGAGBOT1=$(curl -sLX PUT --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $AGBOT_AUTH" -d '{"token":"abcdefg","name":"agbotdev","msgEndPoint":"","publicKey":""}' "${EXCH_URL}/orgs/$ORG/agbots/ag12345" | jq -r '.msg')
    echo "$REGAGBOT1"

    REGAGBOTNS=$(curl -sLX PUT --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $AGBOT_AUTH" -d '{"patternOrgid":"e2edev","pattern":"ns"}' "${EXCH_URL}/orgs/$ORG/agbots/ag12345/patterns/e2edev_ns" | jq -r '.msg')
    echo "$REGAGBOTNS"

    REGAGBOTSNS=$(curl -sLX PUT --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $AGBOT_AUTH" -d '{"patternOrgid":"e2edev","pattern":"sns"}' "${EXCH_URL}/orgs/$ORG/agbots/ag12345/patterns/e2edev_sns" | jq -r '.msg')
    echo "$REGAGBOTSNS"

    REGAGBOTLOC=$(curl -sLX PUT --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $AGBOT_AUTH" -d '{"patternOrgid":"e2edev","pattern":"loc"}' "${EXCH_URL}/orgs/$ORG/agbots/ag12345/patterns/e2edev_loc" | jq -r '.msg')
    echo "$REGAGBOTLOC"

    REGAGBOTSLOC=$(curl -sLX PUT --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $AGBOT_AUTH" -d '{"patternOrgid":"e2edev","pattern":"sloc"}' "${EXCH_URL}/orgs/$ORG/agbots/ag12345/patterns/e2edev_sloc" | jq -r '.msg')
    echo "$REGAGBOTSLOC"

    REGAGBOTGPS=$(curl -sLX PUT --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $AGBOT_AUTH" -d '{"patternOrgid":"e2edev","pattern":"gps"}' "${EXCH_URL}/orgs/$ORG/agbots/ag12345/patterns/e2edev_gps" | jq -r '.msg')
    echo "$REGAGBOTGPS"

    REGAGBOTSGPS=$(curl -sLX PUT --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $AGBOT_AUTH" -d '{"patternOrgid":"e2edev","pattern":"sgps"}' "${EXCH_URL}/orgs/$ORG/agbots/ag12345/patterns/e2edev_sgps" | jq -r '.msg')
    echo "$REGAGBOTSGPS"

    REGAGBOTPWS=$(curl -sLX PUT --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $AGBOT_AUTH" -d '{"patternOrgid":"e2edev","pattern":"pws"}' "${EXCH_URL}/orgs/$ORG/agbots/ag12345/patterns/e2edev_pws" | jq -r '.msg')
    echo "$REGAGBOTPWS"

    REGAGBOTSPWS=$(curl -sLX PUT --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $AGBOT_AUTH" -d '{"patternOrgid":"e2edev","pattern":"spws"}' "${EXCH_URL}/orgs/$ORG/agbots/ag12345/patterns/e2edev_spws" | jq -r '.msg')
    echo "$REGAGBOTSPWS"

    REGAGBOT_NETSPEEDKEYTEST=$(curl -sLX PUT --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $AGBOT_AUTH" -d '{"patternOrgid":"e2edev","pattern":"ns-keytest"}' "${EXCH_URL}/orgs/$ORG/agbots/ag12345/patterns/e2edev_ns-keytest" | jq -r '.msg')
    echo "$REGAGBOT_NETSPEEDKEYTEST"

    REGAGBOTALL=$(curl -sLX PUT --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $AGBOT_AUTH" -d '{"patternOrgid":"e2edev","pattern":"all"}' "${EXCH_URL}/orgs/$ORG/agbots/ag12345/patterns/e2edev_all" | jq -r '.msg')
    echo "$REGAGBOTALL"

    REGAGBOTSALL=$(curl -sLX PUT --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $AGBOT_AUTH" -d '{"patternOrgid":"e2edev","pattern":"sall"}' "${EXCH_URL}/orgs/$ORG/agbots/ag12345/patterns/e2edev_sall" | jq -r '.msg')
    echo "$REGAGBOTSALL"

    REGAGBOTSUH=$(curl -sLX PUT --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $AGBOT_AUTH" -d '{"patternOrgid":"e2edev","pattern":"susehello"}' "${EXCH_URL}/orgs/$ORG/agbots/ag12345/patterns/e2edev_susehello" | jq -r '.msg')
    echo "$REGAGBOTSUH"

    echo "Registering Agbot instance2..."
    REGAGBOT2=$(curl -sLX PUT --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $AGBOT_AUTH" -d '{"token":"abcdefg","name":"agbotdev","msgEndPoint":"","publicKey":""}' "${EXCH_URL}/orgs/$ORG/agbots/ag54321" | jq -r '.msg')
    echo "$REGAGBOT2" 


    # register msghub patterns to agbot1
    if [ $TEST_MSGHUB -eq 1 ]; then
        REGAGBOTCPU2MSGHUB=$(curl -sLX PUT --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $AGBOT_AUTH" -d '{"patternOrgid":"e2edev","pattern":"cpu2msghub"}' "${EXCH_URL}/orgs/$ORG/agbots/ag12345/patterns/e2edev_cpu2msghub" | jq -r '.msg')
        echo "$REGAGBOTCPU2MSGHUB"
    fi

    # Start the agbot
    if [ "$OLDAGBOT" == "1" ]
    then
        echo "Starting the OLD Agreement Bot 1."
        /usr/bin/old-anax -v=5 -alsologtostderr=true -config /etc/agbot/agbot.config >/tmp/agbot.log 2>&1 &
    else
        echo "Starting Agreement Bot 1."
        /usr/local/bin/anax -v=5 -alsologtostderr=true -config /etc/agbot/agbot.config >/tmp/agbot.log 2>&1 &
    fi

    sleep 5
else
    echo -e "Agbot is disabled"
fi

# echo "Starting Agbot API tests"
# ./agbot_upgrade_test.sh
# if [ $? -ne 0 ]
# then
#     echo "Agbot API Test failure."
#     exit -1
# fi
# echo "Agbot API tests completed."

# Services can be run via patterns or from policy files
if [ "${TEST_PATTERNS}" == "" ] && [ "$TESTFAIL" != "1" ]
then
    echo -e "Making agreements based on policy files."

    set_exports

    ./start_node.sh
    if [ $? -ne 0 ]
    then
        echo "Node start failure."
        TESTFAIL="1"
    else
        run_delete_loops
        if [ $? -ne 0 ]
        then
            echo "Delete loop failure."
            TESTFAIL="1"
        fi
    fi

elif [ "$TESTFAIL" != "1" ]; then
    # make agreements based on patterns
    last_pattern=$(echo $TEST_PATTERNS |sed -e 's/^.*,//')
    echo -e "Last pattern is $last_pattern"

    for pat in $(echo $TEST_PATTERNS | tr "," " "); do
        export PATTERN=$pat
        echo -e "***************************"
        echo -e "Start testing pattern $PATTERN..."

        # start pattern test
        set_exports $pat

        ./start_node.sh
        if [ $? -ne 0 ]
        then
            echo "Node start failure."
            TESTFAIL="1"
            break
        fi

        run_delete_loops
        if [ $? -ne 0 ]
        then
            echo "Delete loop failure."
            TESTFAIL="1"
            break
        fi

        echo -e "Done testing pattern $PATTERN"

        # unregister if it is not the last pattern
        if [ "$pat" != "$last_pattern" ]; then
            # Save off the existing log file, in case the next test fails and we need to look back to see how this
            # instance of anax actually ended.
            mv /tmp/anax.log /tmp/anax_$pat.log

            echo -e "Unregister the node. Anax will be shutdown."
            ./unregister.sh
            if [ $? -eq 0 ]; then
                sleep 10
            else
                exit 1
            fi
        fi
        echo -e "***************************"
    done

fi


# Start the node unconfigure tests if they have been enabled.
echo -e "Node unconfig setting is $UNCONFIG"
if [ "$UNCONFIG" == "1" ] && [ "$NOAGBOT" != "1" ] && [ "$TESTFAIL" != "1" ]
then
    echo "Starting unconfig loop tests. Giving time for 1st agreements to complete."
    sleep 120

    echo "Starting device unconfigure script"
    ./unconfig_loop.sh &
else
    echo -e "Unconfig loop tests are disabled."
fi

if [ "$NOLOOP" == "1" ]; then
    if [ "$TESTFAIL" != "1" ]; then
      echo "All tests SUCCESSFUL"
    else
      echo "Test failures occured, check logs"
      exit 1
    fi
else
    # Keep everything alive 
    while :
    do
        sleep 300
    done
fi
