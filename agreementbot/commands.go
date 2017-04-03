package agreementbot

import (
	"fmt"
	"github.com/open-horizon/anax/events"
)

// ==============================================================================================================
type ReceivedWhisperMessageCommand struct {
	Msg events.WhisperReceivedMessage
}

func (r ReceivedWhisperMessageCommand) ShortString() string {
	return fmt.Sprintf("%v", r)
}

func NewReceivedWhisperMessageCommand(msg events.WhisperReceivedMessage) *ReceivedWhisperMessageCommand {
	return &ReceivedWhisperMessageCommand{
		Msg: msg,
	}
}

// ==============================================================================================================
type AgreementTimeoutCommand struct {
	AgreementId string
	Protocol    string
	Reason      uint
}

func (a AgreementTimeoutCommand) ShortString() string {
	return fmt.Sprintf("%v", a)
}

func NewAgreementTimeoutCommand(agreementId string, protocol string, reason uint) *AgreementTimeoutCommand {
	return &AgreementTimeoutCommand{
		AgreementId: agreementId,
		Protocol:    protocol,
		Reason:      reason,
	}
}

// ==============================================================================================================
type NewPolicyCommand struct {
	PolicyFile string
}

func (p NewPolicyCommand) ShortString() string {
	return fmt.Sprintf("%v", p)
}

func NewNewPolicyCommand(fileName string) *NewPolicyCommand {
	return &NewPolicyCommand{
		PolicyFile: fileName,
	}
}

// ==============================================================================================================
type NewProtocolMessageCommand struct {
	Message   []byte
	MessageId int
	From      string
	PubKey    []byte
}

func (p NewProtocolMessageCommand) ShortString() string {
	return fmt.Sprintf("%v", p)
}

func NewNewProtocolMessageCommand(msg []byte, msgId int, deviceId string, pubkey []byte) *NewProtocolMessageCommand {
	return &NewProtocolMessageCommand{
		Message:   msg,
		MessageId: msgId,
		From:      deviceId,
		PubKey:    pubkey,
	}
}

// ==============================================================================================================
type BlockchainEventCommand struct {
	Msg events.EthBlockchainEventMessage
}

func (e BlockchainEventCommand) ShortString() string {
	return e.Msg.ShortString()
}

func NewBlockchainEventCommand(msg events.EthBlockchainEventMessage) *BlockchainEventCommand {
	return &BlockchainEventCommand{
		Msg: msg,
	}
}
