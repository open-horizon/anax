---
copyright: Contributors to the Open Horizon project
years: 2022 - 2025
title: Multi-namespace for cluster agent
description: Documentation for Overview
lastupdated: 2025-08-22
nav_order: 21
parent: Agent (anax)
---
# Overview

Open Horizon supports two types of Edge cluster agents: cluster scope and namespace scope. 

- Cluster agent with **cluster scope** has permission to deploy and manage cluster service in all namespaces inside the Kubernetes cluster. Only one agent with cluster scope can be installed per Kubernetes cluster.

- Cluster agent with **namespace scope** only has permission to deploy and manage cluster service within its own namespace. One or multiple namespce-scoped cluster agents can be installed per Kubernetes cluster. Each cluster agent will need to be installed into its own namespace. 

## Install Namespace Scoped Agent

After configuring the edge cluster (see [here](./cluster_install.md)) run `agent-install.sh` with `--namespace <namespace-to-install-agent>` and `--namespace-scoped`

**Note**: `--namespace` is equivalent to environment variable `AGENT_NAMESPACE`
    
```bash
./agent-install.sh -D cluster -i anax: -c css: -k css: --namespace <namespace-to-install-agent> --namespace-scoped
```
