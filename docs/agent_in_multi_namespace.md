---
copyright: Contributors to the Open Horizon project
years: 2022 - 2026
title: Namespace scoping for cluster agents 
description: Namespace scoping for cluster agents 
lastupdated: 2026-04-20
nav_order: 2
parent: Advanced features
grand_parent: Edge node agents (anax)
has_children: false
has_toc: false
---
# Overview

{{site.data.keyword.edge_notm}} supports two types of Edge cluster agents: cluster scope and namespace scope. 

- Cluster agent with **cluster scope** has permission to deploy and manage cluster service in all namespaces inside the Kubernetes cluster. Only one agent with cluster scope can be installed per Kubernetes cluster.

- Cluster agent with **namespace scope** only has permission to deploy and manage cluster service within its own namespace. One or multiple namespce-scoped cluster agents can be installed per Kubernetes cluster. Each cluster agent will need to be installed into its own namespace. 

## Install Namespace Scoped Agent

After configuring the edge cluster (see [Installing edge clusters](../../installing/edge_clusters.md)) run `agent-install.sh` with `--namespace <namespace-to-install-agent>` and `--namespace-scoped`

**Note**: `--namespace` is equivalent to environment variable `AGENT_NAMESPACE`
    
```bash
./agent-install.sh -D cluster -i anax: -c css: -k css: --namespace <namespace-to-install-agent> --namespace-scoped
```
