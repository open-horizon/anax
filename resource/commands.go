package resource

import (
	"fmt"
	"github.com/open-horizon/anax/events"
)

// This worker command is used to tell the worker than the node is configured and it can init itself.
type NodeConfigCommand struct {
	msg *events.EdgeRegisteredExchangeMessage
}

func (n NodeConfigCommand) String() string {
	return n.ShortString()
}

func (n NodeConfigCommand) ShortString() string {
	return fmt.Sprintf("NodeConfig Command, Msg: %v", n.msg)
}

func NewNodeConfigCommand(msg *events.EdgeRegisteredExchangeMessage) *NodeConfigCommand {
	return &NodeConfigCommand{
		msg: msg,
	}
}

// This worker command is used to tell the worker than the node is done shutting down and so it can terminate itself.
type NodeUnconfigCommand struct {
	msg *events.NodeShutdownMessage
}

func (n NodeUnconfigCommand) String() string {
	return n.ShortString()
}

func (n NodeUnconfigCommand) ShortString() string {
	return fmt.Sprintf("NodeUnconfig Command, Msg: %v", n.msg)
}

func NewNodeUnconfigCommand(msg *events.NodeShutdownMessage) *NodeUnconfigCommand {
	return &NodeUnconfigCommand{
		msg: msg,
	}
}
