package nodemanagement

import (
	"fmt"
	"github.com/open-horizon/anax/events"
)

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

type NodeConfiguredCommand struct {
	Msg *events.EdgeConfigCompleteMessage
}

func (d NodeConfiguredCommand) ShortString() string {
	return fmt.Sprintf("Msg: %v", d.Msg)
}

func (d NodeConfiguredCommand) String() string {
	return d.ShortString()
}

func NewNodeConfiguredCommand(msg *events.EdgeConfigCompleteMessage) *NodeConfiguredCommand {
	return &NodeConfiguredCommand{
		Msg: msg,
	}
}

type NMPDownloadCompleteCommand struct {
	Msg *events.NMPDownloadCompleteMessage
}

func (n NMPDownloadCompleteCommand) String() string {
	return fmt.Sprintf("Msg: %v", n.Msg)
}

func (n NMPDownloadCompleteCommand) ShortString() string {
	return n.String()
}

func NewNMPDownloadCompleteCommand(msg *events.NMPDownloadCompleteMessage) *NMPDownloadCompleteCommand {
	return &NMPDownloadCompleteCommand{Msg: msg}
}

type NodeShutdownCommand struct {
	Msg *events.NodeShutdownMessage
}

func (n NodeShutdownCommand) ShortString() string {
	return fmt.Sprintf("NodeShutdownCommand Msg: %v", n.Msg)
}

func NewNodeShutdownCommand(msg *events.NodeShutdownMessage) *NodeShutdownCommand {
	return &NodeShutdownCommand{
		Msg: msg,
	}
}

type NMPChangeCommand struct {
	Msg *events.ExchangeChangeMessage
}

func (n NMPChangeCommand) String() string {
	return fmt.Sprintf("Msg: %v", n.Msg)
}

func (n NMPChangeCommand) ShortString() string {
	return n.String()
}

func NewNMPChangeCommand(msg *events.ExchangeChangeMessage) *NMPChangeCommand {
	return &NMPChangeCommand{
		Msg: msg,
	}
}

type NodePolChangeCommand struct {
	Msg *events.ExchangeChangeMessage
}

func (n NodePolChangeCommand) String() string {
	return fmt.Sprintf("Msg: %v", n.Msg)
}

func (n NodePolChangeCommand) ShortString() string {
	return n.String()
}

func NewNodePolChangeCommand(msg *events.ExchangeChangeMessage) *NodePolChangeCommand {
	return &NodePolChangeCommand{
		Msg: msg,
	}
}

type AgentFileVersionChangeCommand struct {
	Msg *events.ExchangeChangeMessage
}

func (a AgentFileVersionChangeCommand) String() string {
	return fmt.Sprintf("Msg: %v", a.Msg)
}

func (a AgentFileVersionChangeCommand) ShortString() string {
	return a.String()
}

func NewAgentFileVersionChangeCommand(msg *events.ExchangeChangeMessage) *AgentFileVersionChangeCommand {
	return &AgentFileVersionChangeCommand{
		Msg: msg,
	}
}
