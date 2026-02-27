#!/bin/bash

# Enable debug tracing when DEBUG=1 or RUNNER_DEBUG=1 (GitHub Actions debug mode).
if [ "${DEBUG:-0}" = "1" ] || [ "${RUNNER_DEBUG:-0}" = "1" ]; then
    set -x
fi

# Check agbot archived agreements, looking for k8s agreements.
# $1 - policy name (should be in format of {org}/{policy})
# $2 - anax_api
# $3 - kubectl command
# $4 - pod id
# $5 - namespace
checkArchivedAgreementForPolicy() {
  local policyName="$1" #userdev/bp_location
  local anax_api="$2"
  local kubecmd="$3"
  local pod_id="$4"
  local namespace="$5"
  AGSR=$($kubecmd exec -it "$pod_id" -n "$namespace" -- curl -sSL "${anax_api}/agreement" | jq -r '.agreements.archived')
  NUM_AGS=$(echo "${AGSR}" | jq -r '. | length')
  if [ "${NUM_AGS}" != "0" ]; then
    echo -e "Looking for kube service in archived agreements: ${NUM_AGS}"
    ECAG=$(echo "$AGSR" | jq -r --arg pn "$policyName" '.[] | select(.name | contains($pn)) | .current_agreement_id') # Name: Policy for userdev/agent-in-kube merged with userdev/bp_k8s_embedded_ns. Policy name:userdev/bp_k8s_embedded_ns
    ECAGT=$(echo "$AGSR" | jq -r --arg pn "$policyName" '.[] | select(.name | contains($pn)) | .terminated_description')
    if [ "${ECAG}" = "" ]; then
      echo -e "No terminated agreements found for the edge cluster node for policy ${policyName}, there should be an active agreement."
      return 1
    else
      echo -e "Found agreement(s) ${ECAG} terminated because ${ECAGT}, so agreements are being made with the edge cluster node."
      return 0
    fi
  fi
}

# $1 - policy name
# $2 - anax_api
# $3 - kubectl command
# $4 - pod id
# $5 - namespace
checkAndWaitForActiveAgreementForPolicy() {
  local policyName="$1" #userdev/bp_location
  local anax_api="$2"
  local kubecmd="$3"
  local pod_id="$4"
  local namespace="$5"
  echo -e "kubecmd inside checkAndWaitForActiveAgreementForPolicy is $kubecmd"
  # Since there are no archived agreements, we need to wait for an active agreement to appear.
  LOOPCOUNT=0
  while [ ${LOOPCOUNT} -le 10 ]
  do
    AGSA=$($kubecmd exec -it "$pod_id" -n "$namespace" -- curl -sSL "${anax_api}/agreement" | jq -r '.agreements.active')
    NUM_AGS=$(echo "${AGSA}" | jq -r '. | length')
    if [ "${NUM_AGS}" != "0" ]; then
      echo -e "Looking for kube service in active agreements: ${NUM_AGS}"
      ECAG=$(echo "$AGSA" | jq -r --arg pn "$policyName" '.[] | select(.name | contains($pn)) | .current_agreement_id')
      if [ "${ECAG}" = "" ]; then
          echo -e "Edge Cluster workload should be present but is not, waiting for it to appear."
          sleep 10
          (( LOOPCOUNT+=1 ))
      else
          echo "Edge cluster agreement ${ECAG} found"
          return 0
      fi
    else
      echo -e "No active agreements, but there should be at least one."
      sleep 10
      (( LOOPCOUNT+=1 ))
    fi
  done

  echo "Edge cluster agreement for policy $policyName did not appear"
  return 1
}

# $1 - policy name
# $2 - anax_api
# $3 - kubectl command
# $4 - pod id
# $5 - namespace
function checkAgreementForPolicy() {
  local policyName="$1"
  local anax_api="$2"
  local kubecmd="$3"
  local pod_id="$4"
  local namespace="$5"

  checkArchivedAgreementForPolicy "$policyName" "$anax_api" "$kubecmd" "$pod_id" "$namespace"
  local rc=$?
  if [ $rc -ne 0 ]; then
    checkAndWaitForActiveAgreementForPolicy "$policyName" "$anax_api" "$kubecmd" "$pod_id" "$namespace"
    rc=$?
    if [ $rc -ne 0 ]; then return $rc; fi
  fi
}

# Check agbot archived agreements, looking for k8s agreements.
# $1 - pattern name
# $2 - anax_api
# $3 - kubectl command
# $4 - pod id
# $5 - namespace
checkArchivedAgreementForPattern() {
  local patternName="$1" #e2edev@somecomp.com/sk8s
  local anax_api="$2"
  local kubecmd="$3"
  local pod_id="$4"
  local namespace="$5"

  AGSR=$($kubecmd exec -it "$pod_id" -n "$namespace" -- curl -sSL "${anax_api}/agreement" | jq -r '.agreements.archived')
  NUM_AGS=$(echo "${AGSR}" | jq -r '. | length')
  if [ "${NUM_AGS}" != "0" ]; then
    echo -e "Looking for kube service in archived agreements: ${NUM_AGS}"
    pattern_org=$(echo "$patternName" | cut -d "/" -f 1)
    pattern_name=$(echo "$patternName" | cut -d "/" -f 2)
    ECAG=$(echo "$AGSA" | jq -r --arg po "$pattern_org" --arg pn "$pattern_name" '.[] | select(.name | contains($po) and contains($pn)) | .current_agreement_id') # Name: sk8s-with-embedded-ns_k8s-service-embedded-ns_e2edev@somecomp.com_amd64 merged with sk8s-with-embedded-ns_k8s-service-embedded-ns_e2edev@somecomp.com_amd64,
                                                                                                          # pattern name: e2edev@somecomp.com/sk8s-with-embedded-ns
    ECAGT=$(echo "$AGSA" | jq -r --arg po "$pattern_org" --arg pn "$pattern_name" '.[] | select(.name | contains($po) and contains($pn)) | .terminated_description')
    if [ "${ECAG}" = "" ]; then
      echo -e "No terminated agreements found for the edge cluster node for pattern ${patternName}, there should be an active agreement."
      return 1
    else
      echo -e "Found agreement(s) ${ECAG} terminated because ${ECAGT}, so agreements are being made with the edge cluster node."
      return 0
    fi
  fi
}

# $1 - pattern name
# $2 - anax_api
# $3 - kubectl command
# $4 - pod id
# $5 - namespace
checkAndWaitForActiveAgreementForPattern() {
  local patternName="$1" #e2edev@somecomp.com/sk8s
  local anax_api="$2"
  local kubecmd="$3"
  local pod_id="$4"
  local namespace="$5"

  # Since there are no archived agreements, we need to wait for an active agreement to appear.
  LOOPCOUNT=0
  while [ ${LOOPCOUNT} -le 10 ]
  do
    AGSA=$($kubecmd exec -it "$pod_id" -n "$namespace" -- curl -sSL "${anax_api}/agreement" | jq -r '.agreements.active')
    NUM_AGS=$(echo "${AGSA}" | jq -r '. | length')
    if [ "${NUM_AGS}" != "0" ]; then
      echo -e "Looking for kube service in active agreements: ${NUM_AGS}"
      pattern_org=$(echo "$patternName" | cut -d "/" -f 1)
      pattern_name=$(echo "$patternName" | cut -d "/" -f 2)
      ECAG=$(echo "$AGSA" | jq -r --arg po "$pattern_org" --arg pn "$pattern_name" '.[] | select(.name | contains($po) and contains($pn)) | .current_agreement_id')
      if [ "${ECAG}" = "" ]; then
          echo -e "Edge Cluster workload should be present but is not, waiting for it to appear."
          sleep 10
          (( LOOPCOUNT+=1 ))
      else
          echo "Edge cluster agreement ${ECAG} found"
          return 0
      fi
    else
      echo -e "No active agreements, but there should be at least one."
      sleep 10
      (( LOOPCOUNT+=1 ))
    fi
  done

  echo -e "Edge cluster agreement for pattern $patternName did not appear"
  return 1
}

# $1 - pattern name
# $2 - anax_api
# $3 - kubectl command
# $4 - pod id
# $5 - namespace
checkAgreementForPattern() {
  local patternName="$1"
  local anax_api="$2"
  local kubecmd="$3"
  local pod_id="$4"
  local namespace="$5"

  checkArchivedAgreementForPattern "$patternName" "$anax_api" "$kubecmd" "$pod_id" "$namespace"
  local rc=$?
  if [ $rc -ne 0 ]; then
    checkAndWaitForActiveAgreementForPattern "$patternName" "$anax_api" "$kubecmd" "$pod_id" "$namespace"
    rc=$?
    if [ $rc -ne 0 ]; then return $rc; fi
  fi
}

# $1 - kubectl command
# $2 - deployment name
# $3 - namespace
checkDeploymentInNamespace() {
  local kubecmd="$1"
  local deploymentName="$2"
  local namespace="$3"

  LOOPCOUNT=0
  while [ ${LOOPCOUNT} -le 10 ]
  do
    if ! $kubecmd get deployment "$deploymentName" -n "$namespace"; then
      echo -e "No $deploymentName deployment found in $namespace namespace, waiting for it to appear"
      sleep 10
      (( LOOPCOUNT+=1 ))
    else
      echo -e "Deployment $deploymentName found in $namespace namespace"
      return 0
    fi
  done

  echo "Edge cluster service deployment $deploymentName did not appear in $namespace namespace"
  return 1
}