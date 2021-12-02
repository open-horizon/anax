package download

import (
	"fmt"
	"github.com/open-horizon/anax/events"
)

type StartDownloadCommand struct {
	Msg *events.NMPStartDownloadMessage
}

func NewStartDownloadCommand(msg *events.NMPStartDownloadMessage) *StartDownloadCommand {
	return &StartDownloadCommand{Msg: msg}
}

func (s StartDownloadCommand) String() string {
	return fmt.Sprintf("Msg: %v", s.Msg)
}

func (s StartDownloadCommand) ShortString() string {
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
