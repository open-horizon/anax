# How to install an edge cluster agent and register with the All-in-1 Management Hub

This guide is for installing a microk8s or k3s cluster agent and registering the agent with a previously installed All-in-1 Management Hub. For more information on installing the Hub, see the guide [here](./all-in-1-setup.md). This guide assumes the Management Hub is already running, and was configured to use SSL transport and is listening on an external IP that can be reached outside of the local network.

## Install and configure a k3s edge cluster

**Note**: If you already have a k3s cluster installed, skip to the next section: Installing the Cluster Agent.

This content provides a summary of how to install k3s, a lightweight and small Kubernetes cluster, on Ubuntu 18.04. For more information, see the k3s documentation.

**Note**: If installed, uninstall kubectl before completing the following steps.

1. Either login as root or elevate to root with sudo -i

2. The full hostname of your machine must contain at least two dots. Check the full hostname:

   ```bash
   hostname
   ```

3. Install k3s:

   ```bash
   curl -sfL https://get.k3s.io | sh -
   ```

4. Create the image registry service:

   a. Create a file called **k3s-persistent-claim.yml** with this content

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

    b. Create the persistent volume claim:

    ```bash
    kubectl apply -f k3s-persistent-claim.yml
    ```

    c. Verify that the persistent volume claim was created, and it is in "Pending" status

    ```bash
    kubectl get pvc
    ```

    d. Create a file called **k3s-registry-deployment.yml** with this content:

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

    e. Create the registry deployment and service:

    ```bash
    kubectl apply -f k3s-registry-deployment.yml
    ```

    f. Verify that the docker-registry deployment and docker-registry-service service were created:

    ```bash
    kubectl get deployment
    kubectl get service
    ```

    g. Define the registry endpoint:

    ```bash
    export REGISTRY_ENDPOINT=$(kubectl get service docker-registry-service | grep docker-registry-service | awk '{print $3;}'):5000
    cat << EOF >> /etc/rancher/k3s/registries.yaml
    mirrors:
      "$REGISTRY_ENDPOINT":
        endpoint:
          - "http://$REGISTRY_ENDPOINT"
    EOF
    ```

    h. Restart k3s to pick up the change to /etc/rancher/k3s/registries.yaml:

    ```bash
    systemctl restart k3s
    ```

5. Install docker (if not already installed):

   ```bash
   curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
   add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable"
   apt-get install docker-ce docker-ce-cli containerd.io
   ```

6. Install jq (if not already installed):

   ```bash
   apt-get install jq
   ```

7. Define this registry to docker as an insecure registry:

    a. Run the following to define an insecure registry route using the value of the $REGISTRY_ENDPOINT environment variable obtained in the last step and append it to the /etc/docker/daemon.json file.

    ```bash
    echo "{
        \"insecure-registries\": [ \"$REGISTRY_ENDPOINT\" ]
    }" >> /etc/docker/daemon.json
    ```

    b. (optional) Verify that docker is on your machine:

    ```bash
    curl -fsSL get.docker.com | sh
    ```

    c. Restart docker to pick up the change:

    ```bash
    systemctl restart docker
    ```

## Install and configure a microk8s edge cluster

**Note**: If you already have a microk8s cluster installed, skip to the next section: Installing the Cluster Agent.

This content provides a summary of how to install microk8s, a lightweight and small Kubernetes cluster, on Ubuntu 18.04. (For more information, see the microk8s documentation.)

**Note**: This type of edge cluster is meant for development and test because a single worker node Kubernetes cluster does not provide scalability or high availability.

1. Install microk8s:

    ```bash
    sudo snap install microk8s --classic --channel=stable
    ```

2. If you are not running as root, add your user to the microk8s group:

    ```bash
    sudo usermod -a -G microk8s $USER
    sudo chown -f -R $USER ~/.kube
    su - $USER
    ```

3. Enable dns and storage modules in microk8s:

    ```bash
    microk8s.enable dns
    microk8s.enable storage
    ```

    **Note**: Microk8s uses 8.8.8.8 and 8.8.4.4 as upstream name servers by default. If these name servers cannot resolve the management hub hostname, you must change the name servers that microk8s is using:

    a. Retrieve the list of upstream name servers in `/etc/resolv.conf` or `/run/system/resolve/resolv.conf`

    b. Edit coredns configmap in the kube-system namespace. Set the upstream nameservers in the forward section

    ```bash
    microk8s.kubectl edit -n kube-system cm/coredns
    ```

4. Check the status:

    ```bash
    microk8s.status --wait-ready
    ```

5. The microk8s kubectl command is called microk8s.kubectl to prevent conflicts with an already install kubectl command. Assuming that kubectl is not installed, add this alias for microk8s.kubectl:

    ```bash
    echo 'alias kubectl=microk8s.kubectl' >> ~/.bash_aliases
    source ~/.bash_aliases
    ```

6. Enable the container registry and configure docker to tolerate the insecure registry:

    a. Enable the container registry

    ```bash
    microk8s.enable registry
    export REGISTRY_ENDPOINT=localhost:32000
    export REGISTRY_IP_ENDPOINT=$(kubectl get service registry -n container-registry | grep registry | awk '{print $3;}'):5000
    ```

    b. Install docker (if not already installed):

    ```bash
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
    add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable"
    apt-get install docker-ce docker-ce-cli containerd.io
    ```

    c. Install jq (if not already installed):

    ```bash
    apt-get install jq
    ```

    d. Define this registry as insecure to docker. Create or add to /etc/docker/daemon.json.

    ```bash
    echo "{
        \"insecure-registries\": [ \"$REGISTRY_ENDPOINT\", \"$REGISTRY_IP_ENDPOINT\" ]
    }" >> /etc/docker/daemon.json
    ```

    e. (optional) Verify that docker is on your machine:

    ```bash
    curl -fsSL get.docker.com | sh
    ```

    f. Restart docker to pick up the change:

    ```bash
    sudo systemctl restart docker
    ```

## Install Agent on Edge Cluster

This content describes how to install the Open Horizon agent on k3s or microk8s - lightweight and small Kubernetes cluster solutions.

**Note** These instructions assume that the Hub was configured to listen on an external IP using SSL transport. For more information about setting up an All-in-1 Management Hub see [here](./all-in-1-setup.md)

1. Log in to your edge cluster as root

2. Export your Hub exchange credentials if not already done:

    ```bash
    export HZN_EXCHANGE_USER_AUTH=<your-exchange-username>:<your-exchange-password>
    export HZN_ORG_ID=<your-exchange-organization>
    ```

3. Export the HZN_EXCHANGE_URL, HZN_FSS_CSSURL, HZN_AGBOT_URL and HZN_SDO_SVC_URL environment variables needed to configure the agent to be able to talk to the Hub resources.

    **Note**: Running `hzn env` on the machine running the Management Hub will reveal the IP address as well as the variable values to use above

    ```bash
    export HZN_EXCHANGE_URL=https://<your-external-ip>:3090/v1
    export HZN_FSS_CSSURL=https://<your-external-ip>:9443/
    export HZN_AGBOT_URL=https://<your-external-ip>:3111/
    export HZN_SDO_SVC_URL=https://<your-external-ip>:9008/api
    ```

4. Download the latest agent-install.sh script to your new edge cluster

    ```bash
    curl -sSLO https://github.com/open-horizon/anax/releases/latest/download/agent-install.sh
    chmod +x agent-install.sh
    ```

5. The agent-install.sh script will store the Open Horizon agent in the edge cluster image registry. Set the full image path (minus the tag) that should be used.

    On k3s:

    ```bash
    REGISTRY_ENDPOINT=$(kubectl get service docker-registry-service | grep docker-registry-service | awk '{print $3;}'):5000
    export IMAGE_ON_EDGE_CLUSTER_REGISTRY=$REGISTRY_ENDPOINT/openhorizon-agent/amd64_anax_k8s
    ```

    On microk8s:

    ```bash
    export IMAGE_ON_EDGE_CLUSTER_REGISTRY=localhost:32000/openhorizon-agent/amd64_anax_k8s
    ```

6. Instruct agent-install.sh to use the default storage class:

    On k3s:

    ```bash
    export EDGE_CLUSTER_STORAGE_CLASS=local-path
    ```

    On microk8s:

    ```bash
    export EDGE_CLUSTER_STORAGE_CLASS=microk8s-hostpath
    ```

7. Run agent-install.sh to get the necessary files from Github, install and configure the Horizon agent, and register your edge cluster with policy.

    ```bash
    ./agent-install.sh -D cluster -i anax: -c css: -k css:
    ```

8. Verify that the agent pod is running:

    ```bash
    kubectl get namespaces
    kubectl -n openhorizon-agent get pods
    ```

9. Use the following command to connect to a bash instance on the agent pod to execute hzn commands

    ```bash
    kubectl exec -it $(kubectl get pod -l app=agent -n openhorizon-agent | grep "agent-" | cut -d " " -f1) -n openhorizon-agent -- bash
    ```

10. As a test, execute the following hzn command on the agent pod:

    ```bash
    hzn node ls
    ```

11. The Open Horizon cluster agent is now successfully installed and ready to deploy services
