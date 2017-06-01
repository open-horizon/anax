package abstractprotocol

import (
    "fmt"
)

// =======================================================================================================
// ProposalReplyAck - This is the interface that Horizon uses to interact with proposal reply acks
// in the agreement protocol.
//

type ReplyAck interface {
    ReplyAgreementStillValid() bool
}

// This struct is the reply ack that flows from the consumer to the producer. The StillValid field tells
// the producer whether (true) or not (false) the consumer is still pursuing the agreement.
type BaseReplyAck struct {
    *BaseProtocolMessage
    stillValid bool `json:"decision"`
}

func (br *BaseReplyAck) IsValid() bool {
    return br.BaseProtocolMessage.IsValid() && br.msgType == MsgTypeReplyAck
}

func (br *BaseReplyAck) String() string {
    return br.BaseProtocolMessage.String() + fmt.Sprintf(", StillValid: %v", br.stillValid)
}

func (br *BaseReplyAck) ShortString() string {
    return br.BaseProtocolMessage.ShortString() + fmt.Sprintf(", StillValid: %v", br.stillValid)
}

func (br *BaseReplyAck) ReplyAgreementStillValid() bool {
    return br.stillValid
}

func NewReplyAck(name string, version int, decision bool, id string) *BaseReplyAck {
    return &BaseReplyAck{
        BaseProtocolMessage: &BaseProtocolMessage{
            msgType:        MsgTypeReplyAck,
            protocol:       name,
            version:        version,
            agreementId:    id,
        },
        stillValid: decision,
    }
}