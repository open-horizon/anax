package clusterupgrade

import (
	"fmt"
	"github.com/open-horizon/anax/events"
)

type ClusterUpgradeCommand struct {
	Msg *events.AgentPackageDownloadedMessage
}

func NewClusterUpgradeCommand(msg *events.AgentPackageDownloadedMessage) *ClusterUpgradeCommand {
	return &ClusterUpgradeCommand{Msg: msg}
}

func (s ClusterUpgradeCommand) String() string {
	return fmt.Sprintf("Msg: %v", s.Msg)
}

func (s ClusterUpgradeCommand) ShortString() string {
	return s.String()
}

type NodeRegisteredCommand struct {
	Msg *events.EdgeRegisteredExchangeMessage
}

func (d NodeRegisteredCommand) ShortString() string {
	return fmt.Sprintf("Msg: %v", d.Msg)
}

func (d NodeRegisteredCommand) String() string {
	return d.ShortString()
}

func NewNodeRegisteredCommand(msg *events.EdgeRegisteredExchangeMessage) *NodeRegisteredCommand {
	return &NodeRegisteredCommand{
		Msg: msg,
	}
}
