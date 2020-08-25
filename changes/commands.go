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

type UpdateIntervalCommand struct {
	// the update type. It can be RESET, set to MEDIAN which usually is (max+min)/2
	UpdateType string
}

func (c UpdateIntervalCommand) ShortString() string {
	return fmt.Sprintf("UpdateIntervalCommand: UpdateType: %v", c.UpdateType)
}

func NewUpdateIntervalCommand(updateType string) *UpdateIntervalCommand {
	return &UpdateIntervalCommand{UpdateType: updateType}
}
