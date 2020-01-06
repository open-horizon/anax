package changes

import (
	"fmt"
	"github.com/open-horizon/anax/events"
)

// Commands used to communicate with the worker, directing it to do something, usually based on the
// arrival of an event from the internal message bus.

type DeviceRegisteredCommand struct {
	Msg *events.EdgeRegisteredExchangeMessage
}

func (c DeviceRegisteredCommand) ShortString() string {
	return fmt.Sprintf("DeviceRegisteredCommand Msg: %v", c.Msg)
}

func NewDeviceRegisteredCommand(msg *events.EdgeRegisteredExchangeMessage) *DeviceRegisteredCommand {
	return &DeviceRegisteredCommand{Msg: msg}
}

type AgreementCommand struct {
}

func (c AgreementCommand) ShortString() string {
	return fmt.Sprintf("AgreementCommand")
}

func NewAgreementCommand() *AgreementCommand {
	return &AgreementCommand{}
}

type ResetIntervalCommand struct {
}

func (c ResetIntervalCommand) ShortString() string {
	return fmt.Sprintf("ResetIntervalCommand")
}

func NewResetIntervalCommand() *ResetIntervalCommand {
	return &ResetIntervalCommand{}
}
