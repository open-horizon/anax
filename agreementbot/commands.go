package agreementbot

import (
	"github.com/open-horizon/anax/events"
)

type ReceivedWhisperMessageCommand struct {
	Msg events.WhisperReceivedMessage
}

func NewReceivedWhisperMessageCommand(msg events.WhisperReceivedMessage) *ReceivedWhisperMessageCommand {
	return &ReceivedWhisperMessageCommand{
		Msg: msg,
	}
}

type AgreementTimeoutCommand struct {
	AgreementId string
	Protocol    string
	Reason      uint
}

func NewAgreementTimeoutCommand(agreementId string, protocol string, reason uint) *AgreementTimeoutCommand {
	return &AgreementTimeoutCommand{
		AgreementId: agreementId,
		Protocol:    protocol,
		Reason:      reason,
	}
}

type NewPolicyCommand struct {
	PolicyFile string
}

func NewNewPolicyCommand(fileName string) *NewPolicyCommand {
	return &NewPolicyCommand{
		PolicyFile: fileName,
	}
}

type NewProtocolMessageCommand struct {
	Message   []byte
	MessageId int
	From      string
	PubKey    []byte
}

func NewNewProtocolMessageCommand(msg []byte, msgId int, deviceId string, pubkey []byte) *NewProtocolMessageCommand {
	return &NewProtocolMessageCommand{
		Message:   msg,
		MessageId: msgId,
		From:      deviceId,
		PubKey:    pubkey,
	}
}
