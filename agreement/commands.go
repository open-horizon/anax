package agreement

import (
	"fmt"
)

// ===============================================================================================
// Commands supported by the Agreement Worker

type DeviceRegisteredCommand struct {
	DeviceId string
	Token    string
}

func (d DeviceRegisteredCommand) ShortString() string {
	return fmt.Sprintf("%v", d)
}

func NewDeviceRegisteredCommand(device_id string, token string) *DeviceRegisteredCommand {
	return &DeviceRegisteredCommand{
		DeviceId: device_id,
		Token:    token,
	}
}

// ==============================================================================================================
type TerminateCommand struct {
	reason string
}

func (t TerminateCommand) ShortString() string {
	return fmt.Sprintf("%v", t)
}

func NewTerminateCommand(reason string) *TerminateCommand {
	return &TerminateCommand{
		reason: reason,
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
