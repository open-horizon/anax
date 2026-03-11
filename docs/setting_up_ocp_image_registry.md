---
copyright: Contributors to the Open Horizon project
years: 2020 - 2026
title: Setting up a local image registry for a Red Hat OpenShift Container Platform edge cluster
description: Setting up a local image registry for OpenShift edge clusters
lastupdated: 2026-01-30
nav_order: 12
parent: Planning to install an edge cluster agent
grand_parent: Agent (anax)
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

# Setting up a local image registry for a Red Hat OpenShift Container Platform edge cluster
{: #setting_up_ocp_image_registry}

The image registry is the location of the agent image and agent cronjob image. If an edge service image includes assets that are not appropriate to include in a public registry, you can use a local image registry, where access is tightly controlled. Use this procedure to specify the local image registry that you want to use.
{:shortdesc}

Organization administrators can see all the organization users and their API keys in the {{site.data.keyword.edge_notm}} console, and can delete keys too.

## Procedure

1. Verify that a default route for the {{site.data.keyword.open_shift}} image registry is created and that it is accessible from outside of the cluster:

   ```bash
   oc get route default-route -n openshift-image-registry --template='{{ .spec.host }}'
   ```
   {: codeblock}

   If the command response indicates the `default-route` is not found, you need to expose it (see [Exposing the registry ](https://docs.openshift.com/container-platform/4.6/registry/securing-exposing-registry.html){:target="_blank"}{: .externalLink} for details):

   ```bash
   oc patch configs.imageregistry.operator.openshift.io/cluster --patch '{"spec":{"defaultRoute":true}}' --type=merge
   ```
   {: codeblock}

2. Retrieve the repository route name that you need to use:

   ```bash
   export OCP_IMAGE_REGISTRY=`oc get route default-route -n openshift-image-registry --template='{{ .spec.host }}'`
   ```
   {: codeblock}

3. Create a new project to store your images:

   ```bash
   export OCP_PROJECT=$AGENT_NAMESPACE
   oc new-project $OCP_PROJECT
   ```
   {: codeblock}

4. Create a service account with a name of your choosing:

   ```bash
   export OCP_USER=<service-account-name>
   oc create serviceaccount $OCP_USER
   ```
   {: codeblock}

5. Add a role to your service account for the current project:

   ```bash
   oc policy add-role-to-user edit system:serviceaccount:$OCP_PROJECT:$OCP_USER
   ```
   {: codeblock}

6. Set your service account token to the following environment variable:

   a. Determine if you can extract the token with this command:

   ```bash
   oc serviceaccounts get-token $OCP_USER
   ```
   {: codeblock}

   b. If the above command returns a token, run:

   ```bash
   export OCP_TOKEN=`oc serviceaccounts get-token $OCP_USER`
   ```
   {: codeblock}

   c. If the command from step a did not return a token, run:

   ```bash
   export OCP_TOKEN=`oc serviceaccounts new-token $OCP_USER`
   ```
   {: codeblock}

7. Get the {{site.data.keyword.open_shift}} certificate and allow Docker to trust it:

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

   Use the Docker Desktop icon on the right side of the desktop menu bar to restart Docker by clicking **Restart** in the dropdown menu.

8. Log in to the {{site.data.keyword.ocp}} Docker host:

   ```bash
   echo "$OCP_TOKEN" | docker login -u $OCP_USER --password-stdin $OCP_IMAGE_REGISTRY
   ```
   {: codeblock}

9. Configure additional trust stores for image registry access:

   ```bash
   oc create configmap registry-config --from-file=$OCP_IMAGE_REGISTRY=ca.crt -n openshift-config
   ```
   {: codeblock}

10. Edit the new `registry-config`:

    ```bash
    oc edit image.config.openshift.io cluster
    ```
    {: codeblock}

11. Update the `spec:` section:

    ```bash
    spec:
      additionalTrustedCA:
        name: registry-config
    ```
    {: codeblock}

12. The `agent-install.sh` script stores the {{site.data.keyword.edge_notm}} agent in the edge cluster container registry. Set the registry user, password, and the full image path without the tag:

    ```bash
    export EDGE_CLUSTER_REGISTRY_USERNAME=$OCP_USER
    export EDGE_CLUSTER_REGISTRY_TOKEN="$OCP_TOKEN"
    export IMAGE_ON_EDGE_CLUSTER_REGISTRY=$OCP_IMAGE_REGISTRY/$OCP_PROJECT/amd64_anax_k8s
    ```
    {: codeblock}

    Or for s390x architecture:

    ```bash
    export IMAGE_ON_EDGE_CLUSTER_REGISTRY=$OCP_IMAGE_REGISTRY/$OCP_PROJECT/s390x_anax_k8s
    ```
    {: codeblock}

    **Note**: The {{site.data.keyword.edge_notm}} agent image is stored in the local edge cluster registry because the edge cluster Kubernetes needs ongoing access to it, in case it needs to restart it or move it to another pod.

## What's next

* [Installing the agent on Red Hat OpenShift Container Platform Kubernetes edge cluster](installing_ocp_edge_cluster_agent.md)