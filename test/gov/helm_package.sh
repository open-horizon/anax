#!/bin/bash

# Enable debug tracing when DEBUG=1 or RUNNER_DEBUG=1 (GitHub Actions debug mode).
if [ "${DEBUG:-0}" = "1" ] || [ "${RUNNER_DEBUG:-0}" = "1" ]; then
    set -x
fi

# Reusable functions

# Verify a response. The inputs are:
# $1 - the response
# $2 - expected result
# $3 - error message
verify() {
    respContains=$(echo "$1" | grep "$2")
    if [ "${respContains}" = "" ]; then
        echo -e "\nERROR: $3. Output was:"
        echo -e "$1"
        exit 1
    fi
}

#
# Do whatever prep is necessary for the Helm based service.
#
# ============= Main =================================================
#
echo -e "Begin Helm preparation."

PROJECT_HOME="/root/helm/hello"
PROJECT_HELM_HOME="/root/helm/hello/external"

cd ${PROJECT_HOME} || exit

buildContainer=$(make build)

verify "${buildContainer}" "Successfully built" "hello container did not build"

mkdir -p /root/.helm

cd ${PROJECT_HELM_HOME} || exit
buildPackage=$(make package)

verify "${buildPackage}" "Successfully packaged" "hello Helm package did not build"

echo -e "End of Helm preparation."
