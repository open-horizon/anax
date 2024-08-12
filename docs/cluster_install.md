---
copyright:
years: 2022 - 2024
lastupdated: "2024-04-10"

title: "All-in-One cluster agent"

parent: Agent (anax)
nav_order: 20
---
# How to install an edge cluster agent and register with the All-in-1 Management Hub

This guide is for installing a MicroK8s or K3s cluster agent and registering the agent with a previously installed All-in-1 Management Hub. For more information on installing the Hub, see the guide [here](./all-in-1-setup.md). This guide assumes the Management Hub is already running, and was configured to use SSL transport and is listening on an external IP that can be reached outside of the local network.

## Install and configure a K3s edge cluster

**Note**: If you already have a K3s cluster installed, skip to the next section: [Installing the Cluster Agent](#install-agent-on-edge-cluster).

This content provides a summary of how to install K3s, a lightweight and small Kubernetes cluster, on [Ubuntu 22.04.4 LTS ](https://ubuntu.com/download/server){:target="_blank"}{: .externalLink}. For more information, see [the K3s documentation ](https://docs.k3s.io/){:target="_blank"}{: .externalLink}.

**Note:**: If installed, uninstall kubectl before completing the following steps.

1. Either login as root or elevate to root with sudo -i

2. The full hostname of your machine must contain at least two dots. Check the full hostname:

   ```bash
   hostname
   ```

   If you need to update it (for example, from `k3s` to `k.3.s`), use the following pattern:

   ```bash
   hostnamectl hostname k.3.s
   ```

3. Install K3s:

   ```bash
   curl -sfL https://get.k3s.io | sh -
   ```

4. Choose the image registry types: remote image registry or edge cluster local registry. Image registry is the place that will hold the agent image and agent cronjob image. 

- [Remote image registry](#remote-image-registry)
- [Setup edge cluster local image registry for K3s](#k3s-local-image-registry-setup)

### <a id="remote-image-registry"></a>Remote image registry
{: #remote-image-registry}

Set `USE_EDGE_CLUSTER_REGISTRY` environment variable to `false` to instruct `agent-install.sh` script to use remote image registry. The following environment variables need to be set if use remote image registry:

```bash
export USE_EDGE_CLUSTER_REGISTRY=false
export EDGE_CLUSTER_REGISTRY_USERNAME=<remote-image-registry-username>
export EDGE_CLUSTER_REGISTRY_TOKEN=<remote-image-registry-password>
export IMAGE_ON_EDGE_CLUSTER_REGISTRY=<remote-image-registry-host>/<repository-name>/amd64_anax_k8s
or
export IMAGE_ON_EDGE_CLUSTER_REGISTRY=<remote-image-registry-host>/<repository-name>/s390x_anax_k8s
```
{: codeblock}


### <a id="k3s-local-image-registry-setup"></a>Setup edge cluster local image registry for K3s
{: #k3s-local-image-registry-setup}

**Note: Skip this section if using remote image registry**

1. Create the K3s image registry service:

   a. set `USE_EDGE_CLUSTER_REGISTRY` environment variable to `true`. This env indicates `agent-install.sh` script to use local image registry

      ```bash
      export USE_EDGE_CLUSTER_REGISTRY=true
      ```
      {: codeblock}

   b. Create a file called **k3s-persistent-claim.yaml** with this content:
      ```yaml
      apiVersion: v1
      kind: PersistentVolumeClaim
      metadata:
      name: docker-registry-pvc
      spec:
      storageClassName: "local-path"
      accessModes:
         - ReadWriteOnce
      resources:
         requests:
            storage: 10Gi
      ```
      {: codeblock}

      or download it from the server:

      ```bash
      curl -sSLO https://raw.githubusercontent.com/open-horizon/open-horizon.github.io/master/docs/installing/k3s-persistent-claim.yaml
      ```

   c. Create the persistent volume claim:

      ```bash
      kubectl apply -f k3s-persistent-claim.yaml
      ```
      {: codeblock}

   d. Verify that the persistent volume claim was created and it is in "Pending" status

      ```bash
      kubectl get pvc
      ```
      {: codeblock}

   e. Create a file called **k3s-registry-deployment.yaml** with this content:

      ```yaml
      apiVersion: apps/v1
      kind: Deployment
      metadata:
      name: docker-registry
      labels:
         app: docker-registry
      spec:
      replicas: 1
      selector:
         matchLabels:
            app: docker-registry
      template:
         metadata:
            labels:
            app: docker-registry
         spec:
            volumes:
            - name: registry-pvc-storage
            persistentVolumeClaim:
               claimName: docker-registry-pvc
            containers:
            - name: docker-registry
            image: registry
            ports:
            - containerPort: 5000
            volumeMounts:
            - name: registry-pvc-storage
               mountPath: /var/lib/registry
      ---
      apiVersion: v1
      kind: Service
      metadata:
      name: docker-registry-service
      spec:
      selector:
         app: docker-registry
      type: NodePort
      ports:
         - protocol: TCP
            port: 5000
      ```
      {: codeblock}

      or download it from the server:

      ```bash
      curl -sSLO https://raw.githubusercontent.com/open-horizon/open-horizon.github.io/master/docs/installing/k3s-registry-deployment.yaml
      ```

   f. Create the registry deployment and service:

      ```bash
      kubectl apply -f k3s-registry-deployment.yaml
      ```
      {: codeblock}

   g. Verify that the service was created:

      ```bash
      kubectl get deployment
      kubectl get service
      ```
      {: codeblock}

   h. Define the registry endpoint:

      ```bash
      export REGISTRY_ENDPOINT=$(kubectl get service docker-registry-service | grep docker-registry-service | awk '{print $3;}'):5000
      cat << EOF >> /etc/rancher/k3s/registries.yaml
      mirrors:
      "$REGISTRY_ENDPOINT":
         endpoint:
            - "http://$REGISTRY_ENDPOINT"
      EOF
      ```
      {: codeblock}

   i. Restart K3s to pick up the change to **/etc/rancher/k3s/registries.yaml**:

      ```bash
      systemctl restart k3s
      ```
      {: codeblock}

2. Define this registry to Docker as an insecure registry:

   a. Install Docker (if not already installed, `docker --version` to check):

      ```bash
      curl -fsSL get.docker.com | sh
      ```
      {: codeblock}

   b. Create or add to **/etc/docker/daemon.json** (replacing `<registry-endpoint>` with the value of the `$REGISTRY_ENDPOINT` environment variable you obtained in a previous step).

      ```json
      {
         "insecure-registries": [ "<registry-endpoint>" ]
      }
      ```
      {: codeblock}

   c. Restart Docker to pick up the change:

      ```bash
      systemctl restart docker
      ```
      {: codeblock}
   
   d. Install `jq`:

   ```bash
   apt-get -y install jq
   ```

## Install and configure a MicroK8s edge cluster

**Note**: If you already have a MicroK8s cluster installed, skip to the next section: Installing the Cluster Agent.

This content provides a summary of how to install MicroK8s, a lightweight and small Kubernetes cluster, on Ubuntu 22.04.4 LTS. (For more information, see the MicroK8s documentation.)

**Note**: This type of edge cluster is meant for development and test because a single worker node Kubernetes cluster does not provide scalability or high availability.

1. Install MicroK8s:

    ```bash
    sudo snap install microk8s --classic --channel=stable
    ```

2. If you are not running as root, add your user to the MicroK8s group:

    ```bash
    sudo usermod -a -G microk8s $USER
    sudo chown -f -R $USER ~/.kube
    su - $USER
    ```

3. Enable dns and storage modules in MicroK8s:

    ```bash
    microk8s.enable dns
    microk8s.enable hostpath-storage
    ```

    **Note**: MicroK8s uses 8.8.8.8 and 8.8.4.4 as upstream name servers by default. If these name servers cannot resolve the management hub hostname, you must change the name servers that MicroK8s is using:

    a. Retrieve the list of upstream name servers in `/etc/resolv.conf` or `/run/system/resolve/resolv.conf`

    b. Edit coredns configmap in the kube-system namespace. Set the upstream nameservers in the forward section

    ```bash
    microk8s.kubectl edit -n kube-system cm/coredns
    ```

4. Check the status:

    ```bash
    microk8s.status --wait-ready
    ```

5. The MicroK8s kubectl command is called microk8s.kubectl to prevent conflicts with an already install kubectl command. Assuming that kubectl is not installed, add this alias for microk8s.kubectl:

    ```bash
    echo 'alias kubectl=microk8s.kubectl' >> ~/.bash_aliases
    source ~/.bash_aliases
    ```

6. Choose the image registry types: remote image registry or edge cluster local registry. Image registry is the place that will hold the agent image and agent cronjob image. 

- [Remote image registry](#remote-image-registry)
- [Setup edge cluster local image registry for MicroK8s](#microk8s-local-image-registry-setup)

### <a id="microk8s-local-image-registry-setup"></a>Setup edge cluster local image registry for MicroK8s
{: #microk8s-local-image-registry-setup}

**Note: Skip this section if using remote image registry.** Enable the container registry and configure Docker to tolerate the insecure registry:

1. Enable the container registry

  ```bash
  microk8s.enable registry
  export REGISTRY_ENDPOINT=localhost:32000
  export REGISTRY_IP_ENDPOINT=$(kubectl get service registry -n container-registry | grep registry | awk '{print $3;}'):5000
  ```

2. Install Docker (if not already installed, `docker --version` to check):

      ```bash
      curl -fsSL get.docker.com | sh
      ```
      {: codeblock}

3. Install jq (if not already installed):

  ```bash
  apt-get -y install jq
  ```

4. Define this registry as insecure to Docker. Create or add to `/etc/docker/daemon.json`.

  ```bash
  echo "{
      \"insecure-registries\": [ \"$REGISTRY_ENDPOINT\", \"$REGISTRY_IP_ENDPOINT\" ]
  }" >> /etc/docker/daemon.json
  ```

5. Restart Docker to pick up the change:

  ```bash
  systemctl restart docker
  ```

## Install Agent on Edge Cluster
{: #install-agent-on-edge-cluster}

This content describes how to install the Open Horizon agent on K3s or MicroK8s - lightweight and small Kubernetes cluster solutions.

**Note**: These instructions assume that the Hub was configured to listen on an external IP using SSL transport. For more information about setting up an All-in-1 Management Hub see [here](./all-in-1-setup.md)

1. Log in to your edge cluster as root

2. Export your Hub exchange credentials if not already done:

    ```bash
    export HZN_EXCHANGE_USER_AUTH=<your-exchange-username>:<your-exchange-password>
    export HZN_ORG_ID=<your-exchange-organization>
    ```

3. Export the HZN_EXCHANGE_URL, HZN_FSS_CSSURL and HZN_AGBOT_URL environment variables needed to configure the agent to be able to talk to the Hub resources.

    **Note**: Running `hzn env` on the machine running the Management Hub will reveal the IP address as well as the variable values to use above

    ```bash
    export HZN_EXCHANGE_URL=https://<your-external-ip>:3090/v1
    export HZN_FSS_CSSURL=https://<your-external-ip>:9443/
    export HZN_AGBOT_URL=https://<your-external-ip>:3111/
    ```

4. Download the latest agent-install.sh script to your new edge cluster

    ```bash
    curl -sSLO https://github.com/open-horizon/anax/releases/latest/download/agent-install.sh
    chmod +x agent-install.sh
    ```

5. Instruct agent-install.sh to use the default storage class:

    On K3s:

    ```bash
    export EDGE_CLUSTER_STORAGE_CLASS=local-path
    ```

    On MicroK8s:

    ```bash
    export EDGE_CLUSTER_STORAGE_CLASS=microk8s-hostpath
    ```

    If the cluster agent will use other storageclass than the above, please find the storage class satisfy [these attributes](#storageclass_attribute)

6. Run agent-install.sh to get the necessary files from Github, install and configure the Horizon agent, and register your edge cluster with policy.

    **Note**: You should be logged in as root or elevated to root.  If you are not, preface the agent-install.sh script command below with `sudo -s -E`.
    
    Set `AGENT_NAMESPACE` to the namespace that will install the cluster agent. If not set, the agent will be installed to `openhorizon-agent` default namespace.
    ```bash
    AGENT_NAMESPACE=<namespace-to-install-agent>
    ```

    If you are not using `https` as the transport, you need to create an empty installation certificate file in the current directory:
    ```bash
    touch agent-install.crt
    ```

    To install a cluster-scoped agent: 
    ```bash
    ./agent-install.sh -D cluster -i anax: -c css: -k css: --namespace $AGENT_NAMESPACE
    ```

    To install a namespace-scoped agent:
    ```bash
    ./agent-install.sh -D cluster -i anax: -c css: -k css: --namespace $AGENT_NAMESPACE --namespace-scoped
    ```

7. Verify that the agent pod is running:

    ```bash
    kubectl get namespaces
    kubectl -n $AGENT_NAMESPACE get pods
    ```

    **Note**: See [here](./agent_in_multi_namespace.md) for more information about agent in multi-namespace.


8. Use the following command to connect to a bash instance on the agent pod to execute hzn commands

    ```bash
    kubectl exec -it $(kubectl get pod -l app=agent -n $AGENT_NAMESPACE | grep "agent-" | cut -d " " -f1) -n $AGENT_NAMESPACE -- bash
    ```

9. As a test, execute the following hzn command on the agent pod:

    ```bash
    hzn node ls
    ```

10. The Open Horizon cluster agent is now successfully installed and ready to deploy services

## <a id="storageclass_attribute"></a>StorageClass attribute
{: #storageclass_attribute}

A PersistentVolumeClaim will be created during the agent install process. It will be used by agent to store data for agent and cronjob. The storageclass must satisfy the following requirements:

- supports both read and write
- can be made available immediately
- supports `ReadWriteMany` mode if agent is running in multi-node cluster
