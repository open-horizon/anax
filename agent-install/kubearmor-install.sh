#!/bin/bash

# Installs the kube armor operator on the Open Horizon cluster agent

set -e   #future: remove?

echo "Starting KubeArmor installation..."

# Step 1: Install Helm (if not already installed)
if ! command -v helm &> /dev/null; then
  echo "Helm not found, installing Helm..."
  curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
else
  echo "Helm is already installed"
fi

# Step 2: Create a new working directory for a new horizon project
echo "Create a new working directory for a new horizon project"
hzn dev service new -V 1.0.0 -s kubearmor-operator -c cluster

# Step 3: Making the Operator file
echo "Making the operator file"
helm repo add kubearmor https://kubearmor.github.io/charts
helm repo update kubearmor
helm template kubearmor/kubearmor-operator -n openhorizon-agent >> kubearmor-operator.yaml

curl https://raw.githubusercontent.com/kubearmor/KubeArmor/main/deployments/helm/KubeArmorOperator/crds/operator.kubearmor.com_kubearmorconfigs.yaml > kubearmor-crd.yaml

# Step 4: Compress the .yaml file
echo "Compressing the .yaml helm file"
tar -czvf operator.tar.gz kubearmor-operator.yaml

# Step 5: Configure the KubeArmor operator
echo "KubeArmor operator config"
kubectl apply -f https://raw.githubusercontent.com/kubearmor/KubeArmor/main/deployments/helm/KubeArmorOperator/crds/operator.kubearmor.com_kubearmorconfigs.yaml

# Step 6: Edit the horizon/service.definition.json file to point to the operator's yaml archive created in the previous step
echo "Editing the horizon/service.definition.json file to point to the operator's yaml archive
# Defining the JSON file
SERVICE_DEF_JSON= "horizon/service.definition.json"
jq '.operatorYamlArchive = "../operator.tar.gz"' $SERVICE_DEF_JSON > temp.json && mv temp.json $SERVICE_DEF_JSON

# Step 5: Publish operator service
echo "Publishing operator service"
hzn exchange service publish -f horizon/service.definition.json

# Step 6: Create a deployment policy file:
echo "Creating a deployment.policy.json file"
cat << 'EOF' > horizon/deployment.policy.json
{
  "label": "$SERVICE_NAME Deployment Policy",
  "description": "A super-simple sample Horizon Deployment Policy",
  "service": {
    "name": "$SERVICE_NAME",
    "org": "$HZN_ORG_ID",
    "arch": "*",
    "serviceVersions": [
      {
        "version": "$SERVICE_VERSION",
        "priority":{}
      }
    ]
  },
  "properties": [
  ],
  "constraints": [
    "example == kubearmor-operator"
  ],
  "userInput": [
  ]
}
EOF


# Step 7: Publish your deployment policy
echo "Publishing your deployment policy"
hzn exchange deployment addpolicy -f horizon/deployment.policy.json kubearmor-operator

# Step 8: Create a node.policy.json file
echo "Creating node policy file"
cat << 'EOF' > node.policy.json
{
  "properties": [
    { "name": "example", "value": "kubearmor-operator" }
  ]
}
EOF

# Step 9: Register your edge cluster with your new node policy
echo "Registering edge cluster with new node policy"
hznpod register -u $HZN_EXCHANGE_USER_AUTH
cat node.policy.json | hznpod policy update -f-
hznpod policy list

# Step 10: Check to see the agreement has been created (this can take approximately 15 seconds)
echo "Checking for agreement creation"
sleep 15
hznpod agreement list

# Step 11: Check if the operator is up in the cluster
echo "Checking if the operator is up in the cluster"
kubectl get pods -n openhorizon-agent

# Step 12: Download the sample configuration file
echo "Downloading sample configuration file"
wget https://raw.githubusercontent.com/kubearmor/KubeArmor/main/pkg/KubeArmorOperator/config/samples/sample-config.yml -O sample-config.yml

# Step 13: Modify the sample configuration file to set the namespace to openhorizon-agent
echo "Modifying sample configuration file to set the namespace to openhorizon-agent"
sed -i 's/namespace: .*/namespace: openhorizon-agent/' sample-config.yml

# Step 14: Apply the modified configuration file
echo "Applying modified configuration file"
kubectl apply -f sample-config.yml

echo "KubeArmor installation and configuration completed successfully!"