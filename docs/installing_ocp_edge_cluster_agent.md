---
copyright: Contributors to the Open Horizon project
years: 2022 - 2026
title: Installing the agent on Red Hat OpenShift Container Platform Kubernetes edge cluster
description: Documentation for installing the agent on OpenShift edge clusters
lastupdated: 2026-01-30
nav_order: 18
parent: Agent (anax)
has_children: false
has_toc: false
---

{:new_window: target="blank"}
{:shortdesc: .shortdesc}
{:screen: .screen}
{:codeblock: .codeblock}
{:pre: .pre}
{:child: .link .ulchildlink}
{:childlinks: .ullinks}

# Installing the agent on Red Hat OpenShift Container Platform Kubernetes edge cluster
{: #installing_ocp_edge_cluster_agent}

This content describes how to install the {{site.data.keyword.ieam}} agent on your {{site.data.keyword.ocp}} edge cluster. Follow these steps on a host that has admin access to your edge cluster:
{:shortdesc}

## Prerequisites

* The architecture of the cluster must be one of the following:
  - AMD64 (x86_64)
  - ARM64 (AArch64)
  - ppc64le
  - s390x

* Operating system must be modern Linux variant with 64-bit and systemd support

**Note**: {{site.data.keyword.ieam}} cluster agent installation requires cluster admin access on the edge cluster. Additionally, the "jq" command-line JSON processor must be installed prior to running the agent install script.

## Procedure

1. Log in to your edge cluster as **admin**:

   ```bash
   oc login https://<api_endpoint_host>:<port> -u <admin_user> -p <admin_password> --insecure-skip-tls-verify=true
   ```
   {: codeblock}

2. If you don't already have your authorization information, get it now. For details about how to get the authorization information, see [Authentication overview](authentication_overview.md).

   ```bash
   export HZN_ORG_ID=<your-exchange-organization>
   export HZN_EXCHANGE_USER_AUTH=<authentication string>
   export HZN_EXCHANGE_URL= # example http://open-horizon.lfedge.iol.unh.edu:3090/v1
   export HZN_FSS_CSSURL= # example http://open-horizon.lfedge.iol.unh.edu:9443/
   export HZN_AGBOT_URL= # example http://open-horizon.lfedge.iol.unh.edu:3111
   export HZN_FDO_SVC_URL= # example http://open-horizon.lfedge.iol.unh.edu:9008/api
   export HZN_NODE_ID=<edge-cluster-node-name>
   ```
   {: codeblock}

3. Set the agent namespace variable to its default value (or whatever namespace you want to explicitly install the agent into):

   ```bash
   export AGENT_NAMESPACE=openhorizon-agent
   ```
   {: codeblock}

4. Set the storage class that you want the agent to use - either a built-in storage class or one that you created. You can view the available storage classes with the first of the following two commands, then substitute the name of the one you want to use into the second command. One storage class should be labeled `(default)`:

   ```bash
   oc get storageclass
   export EDGE_CLUSTER_STORAGE_CLASS=<rook-ceph-cephfs-internal>
   ```
   {: codeblock}

5. Determine whether a default route for the {{site.data.keyword.open_shift}} image registry has been created so that it is accessible from outside of the cluster:

   ```bash
   oc get route default-route -n openshift-image-registry --template='{{ .spec.host }}'
   ```
   {: codeblock}

   If the command response indicates the **default-route** is not found, you need to expose it (see [Exposing the registry ](https://docs.openshift.com/container-platform/4.6/registry/securing-exposing-registry.html){:target="_blank"}{: .externalLink} for details):

   ```bash
   oc patch configs.imageregistry.operator.openshift.io/cluster --patch '{"spec":{"defaultRoute":true}}' --type=merge
   ```
   {: codeblock}

6. Retrieve the repository route name that you need to use:

   ```bash
   export OCP_IMAGE_REGISTRY=`oc get route default-route -n openshift-image-registry --template='{{ .spec.host }}'`
   ```
   {: codeblock}

7. Create a new project to store your images:

   ```bash
   export OCP_PROJECT=$AGENT_NAMESPACE
   oc new-project $OCP_PROJECT
   ```
   {: codeblock}

8. Create a service account with a name of your choosing:

   ```bash
   export OCP_USER=<service-account-name>
   oc create serviceaccount $OCP_USER
   ```
   {: codeblock}

9. Add a role to your service account for the current project:

   ```bash
   oc policy add-role-to-user edit system:serviceaccount:$OCP_PROJECT:$OCP_USER
   ```
   {: codeblock}

10. Set your service account token to the following environment variable:

    ```bash
    export OCP_TOKEN=`oc serviceaccounts get-token $OCP_USER`
    ```
    {: codeblock}

11. Get the {{site.data.keyword.open_shift}} certificate and configure Docker to trust it:

    ```bash
    echo | openssl s_client -connect $OCP_IMAGE_REGISTRY:443 -showcerts | sed -n "/-----BEGIN CERTIFICATE-----/,/-----END CERTIFICATE-----/p" > ca.crt
    ```
    {: codeblock}

    On {{site.data.keyword.linux_notm}}:

    ```bash
    mkdir -p /etc/docker/certs.d/$OCP_IMAGE_REGISTRY
    cp ca.crt /etc/docker/certs.d/$OCP_IMAGE_REGISTRY
    systemctl restart docker.service
    ```
    {: codeblock}

    On {{site.data.keyword.macOS_notm}}:

    ```bash
    mkdir -p ~/.docker/certs.d/$OCP_IMAGE_REGISTRY
    cp ca.crt ~/.docker/certs.d/$OCP_IMAGE_REGISTRY
    ```
    {: codeblock}

    On {{site.data.keyword.macOS_notm}}, use the Docker Desktop icon on the right side of the desktop menu bar to restart Docker by clicking **Restart** in the dropdown menu.

12. Log in to the {{site.data.keyword.ocp}} Docker host:

    ```bash
    echo "$OCP_TOKEN" | docker login -u $OCP_USER --password-stdin $OCP_IMAGE_REGISTRY
    ```
    {: codeblock}

13. Configure additional trust stores for image registry access:

    ```bash
    oc create configmap registry-config --from-file=$OCP_IMAGE_REGISTRY=ca.crt -n openshift-config
    ```
    {: codeblock}

14. Edit the new `registry-config`:

    ```bash
    oc edit image.config.openshift.io cluster
    ```
    {: codeblock}

15. Update the `spec:` section:

    ```bash
    spec:
      additionalTrustedCA:
        name: registry-config
    ```
    {: codeblock}

16. The **agent-install.sh** script stores the {{site.data.keyword.ieam}} agent in the edge cluster container registry. Set the registry user, password, and full image path (minus the tag):

    ```bash
    export EDGE_CLUSTER_REGISTRY_USERNAME=$OCP_USER
    export EDGE_CLUSTER_REGISTRY_TOKEN="$OCP_TOKEN"
    export IMAGE_ON_EDGE_CLUSTER_REGISTRY=$OCP_IMAGE_REGISTRY/$OCP_PROJECT/amd64_anax_k8s
    ```
    {: codeblock}

     **Note**: The {{site.data.keyword.ieam}} agent image is stored in the local edge cluster registry because the edge cluster Kubernetes needs ongoing access to it, in case it needs to restart it or move it to another pod.

17. Download the **agent-install.sh** script from the Cloud Sync Service (CSS) and make it executable:

    ```bash
    curl -u "$HZN_ORG_ID/$HZN_EXCHANGE_USER_AUTH" -k -o agent-install.sh $HZN_FSS_CSSURL/api/v1/objects/IBM/agent_files/agent-install.sh/data
    chmod +x agent-install.sh
    ```
    {: codeblock}

18. Run **agent-install.sh** to get the necessary files from CSS, install and configure the {{site.data.keyword.horizon}} agent, and register your edge cluster with policy:

    ```bash
    ./agent-install.sh -D cluster -i 'css:'
    ```
    {: codeblock}

       **Notes**:
       * To see all of the available flags, run: **./agent-install.sh -h**
       * If an error causes **agent-install.sh** to fail, correct the error and run **agent-install.sh** again. If that does not work, run **agent-uninstall.sh** (see [Removing agent from edge cluster](removing_agent_from_cluster.md)) before running **agent-install.sh** again.
    
19. Change to the agent namespace (also known as project) and verify that the agent pod is running:

    ```bash
    oc project $AGENT_NAMESPACE
    oc get pods
    ```
    {: codeblock}

20. Now that the agent is installed on your edge cluster, you can run these commands if you want to familiarize yourself with the Kubernetes resources associated with the agent:

    ```bash
    oc get namespace $AGENT_NAMESPACE
    oc project $AGENT_NAMESPACE   # ensure this is the current namespace/project
    oc get deployment -o wide
    oc get deployment agent -o yaml   # get details of the deployment
    oc get configmap openhorizon-agent-config -o yaml
    oc get secret openhorizon-agent-secrets -o yaml
    oc get pvc openhorizon-agent-pvc -o yaml   # persistent volume
    ```
    {: codeblock}

21. Often, when an edge cluster is registered for policy, but does not have user-specified node policy, none of the deployment policies will deploy edge services to it. That is the case with the Horizon examples. Proceed to [Deploying services to your edge cluster](deploying_services_cluster.md) to set node policy so that an edge service will be deployed to this edge cluster.

## What's next

* [Deploying services to your edge cluster](deploying_services_cluster.md)
* [Removing the agent from an edge cluster](removing_agent_from_cluster.md)