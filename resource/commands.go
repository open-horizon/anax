package resource

import (
	"fmt"
	"github.com/open-horizon/anax/events"
)

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