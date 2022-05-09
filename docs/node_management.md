# Node Management Policy

Node management policy (NMP) enables node administrators to remotely configure settings of their nodes throughout the node's lifecycle. Nodes are responsible for finding and applying the intents represented by node management policies. Nodes match themselves against availible NMPs using the policy and pattern selectors written in the NMP.

## Agent Auto Upgrade

The agent auto upgrade section of node management policy enables nodes to update any combination of their configuration, certificate, and agent software.
- `configuation`: This contains information about the management hub the node is registered to. See [here](../agent-install/README.md#configuration-file) for the expected contents of this file.