package agreement

import (
	"fmt"
	"github.com/open-horizon/anax/events"
)

// ===============================================================================================
// Commands supported by the Agreement Worker

type DeviceRegisteredCommand struct {
	Msg *events.EdgeRegisteredExchangeMessage
}

func (d DeviceRegisteredCommand) ShortString() string {
	return fmt.Sprintf("%v", d)
}

func NewDeviceRegisteredCommand(msg *events.EdgeRegisteredExchangeMessage) *DeviceRegisteredCommand {
	return &DeviceRegisteredCommand{
		Msg: msg,
	}
}

// ==============================================================================================================
type AdvertisePolicyCommand struct {
	PolicyFile string
}

func (a AdvertisePolicyCommand) ShortString() string {
	return fmt.Sprintf("%v", a)
}

func NewAdvertisePolicyCommand(fileName string) *AdvertisePolicyCommand {
	return &AdvertisePolicyCommand{
		PolicyFile: fileName,
	}
}

// ==============================================================================================================
type EdgeConfigCompleteCommand struct {
	Msg *events.EdgeConfigCompleteMessage
}

func (d EdgeConfigCompleteCommand) ShortString() string {
	return fmt.Sprintf("%v", d)
}

func NewEdgeConfigCompleteCommand(msg *events.EdgeConfigCompleteMessage) *EdgeConfigCompleteCommand {
	return &EdgeConfigCompleteCommand{
		Msg: msg,
	}
}

// ==============================================================================================================
type NodePolicyChangedCommand struct {
	Msg *events.NodePolicyMessage
}

func (d NodePolicyChangedCommand) ShortString() string {
	return fmt.Sprintf("%v", d)
}

func NewNodePolicyChangedCommand(msg *events.NodePolicyMessage) *NodePolicyChangedCommand {
	return &NodePolicyChangedCommand{
		Msg: msg,
	}
}

// ==============================================================================================================
type NodeChangeCommand struct {
}

func (c NodeChangeCommand) ShortString() string {
	return fmt.Sprintf("NodeChangeCommand")
}

func NewNodeChangeCommand() *NodeChangeCommand {
	return &NodeChangeCommand{}
}

// ==============================================================================================================
type NodePolicyChangeCommand struct {
}

func (c NodePolicyChangeCommand) ShortString() string {
	return fmt.Sprintf("NodePolicyChangeCommand")
}

func NewNodePolicyChangeCommand() *NodePolicyChangeCommand {
	return &NodePolicyChangeCommand{}
}
