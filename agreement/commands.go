package agreement

import (
	"fmt"
)

// ===============================================================================================
// Commands supported by the Agreement Worker

type DeviceRegisteredCommand struct {
	Token string
}

func (d DeviceRegisteredCommand) ShortString() string {
	return fmt.Sprintf("%v", d)
}

func NewDeviceRegisteredCommand(token string) *DeviceRegisteredCommand {
	return &DeviceRegisteredCommand{
		Token: token,
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
