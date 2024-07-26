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
