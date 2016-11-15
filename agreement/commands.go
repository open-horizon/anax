package agreement

import (
	"github.com/open-horizon/anax/events"
)

// ===============================================================================================
// Commands supported by the Exchange Worker

type DeviceRegisteredCommand struct {
	Token string
}

func NewDeviceRegisteredCommand(token string) *DeviceRegisteredCommand {
	return &DeviceRegisteredCommand{
		Token: token,
	}
}

type TerminateCommand struct {
	reason string
}

func NewTerminateCommand(reason string) *TerminateCommand {
	return &TerminateCommand{
		reason: reason,
	}
}

type AdvertisePolicyCommand struct {
	PolicyFile string
}

func NewAdvertisePolicyCommand(fileName string) *AdvertisePolicyCommand {
	return &AdvertisePolicyCommand{
		PolicyFile: fileName,
	}
}

type ReceivedProposalCommand struct {
	Msg events.WhisperReceivedMessage
}

func NewReceivedProposalCommand(msg events.WhisperReceivedMessage) *ReceivedProposalCommand {
	return &ReceivedProposalCommand{
		Msg: msg,
	}
}
