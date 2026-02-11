---
copyright: Contributors to the Open Horizon project
years: 2022 - 2026
title: Configuring a storage class
description: Documentation for configuring storage class for edge cluster agents
lastupdated: 2026-01-30
nav_order: 21
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

# Configuring a storage class
{: #configuring_storage_class}

## StorageClass attribute
{: #storageclass_attribute}

A PersistentVolumeClaim will be created during the agent install process. It will be used by agent to store data for agent and cronjob. The storageclass must satisfy the following requirements:
{:shortdesc}

- Supports both read and write
- Can be made available immediately
- Supports `ReadWriteMany` mode if agent is running in multi-node cluster