#!/bin/sh

# Check env vars that we know should be set to verify that everything is working
function verify {
    if [ "$2" == "" ]
    then
        echo -e "Error: $1 should be set but is not."
        exit 2
    fi
}

verify "HELLO_VAR" $HELLO_VAR
verify "HELLO_PORT" $HELLO_PORT

socat TCP4-LISTEN:${HELLO_PORT},fork EXEC:./service.sh
