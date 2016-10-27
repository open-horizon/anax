package agreement

import (
	"github.com/open-horizon/anax/events"
)

// ===============================================================================================
// Commands supported by the Exchange Worker

type InitEdgeCommand struct {
}

func NewInitEdgeCommand() *InitEdgeCommand {
	return &InitEdgeCommand{}
}

type DeviceRegisteredCommand struct {
	Id    string
	Token string
}

func NewDeviceRegisteredCommand(id string, token string) *DeviceRegisteredCommand {
	return &DeviceRegisteredCommand{
		Id:    id,
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
