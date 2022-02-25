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
