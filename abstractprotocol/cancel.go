package abstractprotocol

import (
    "fmt"
)

// =======================================================================================================
// Cancel - This is the interface that Horizon uses to interact with cancel messages in the agreement
// protocol.
//

type Cancel interface {
    ProtocolMessage
    Reason() uint
}

// This struct is the cancel that flows from the consumer to the producer or producer to consumer.
type BaseCancel struct {
    *BaseProtocolMessage
    TheReason uint `json:"reason"`
}

func (bc *BaseCancel) IsValid() bool {
    return bc.BaseProtocolMessage.IsValid() && bc.MsgType == MsgTypeCancel
}

func (bc *BaseCancel) String() string {
    return bc.BaseProtocolMessage.String() + fmt.Sprintf(", Reason: %v", bc.TheReason)
}

func (bc *BaseCancel) ShortString() string {
    return bc.BaseProtocolMessage.ShortString() + fmt.Sprintf(", Reason: %v", bc.TheReason)
}

func (bc *BaseCancel) Reason() uint {
    return bc.TheReason
}

func NewBaseCancel(name string, version int, id string, reason uint) *BaseCancel {
    return &BaseCancel{
        BaseProtocolMessage: &BaseProtocolMessage{
            MsgType:   MsgTypeCancel,
            AProtocol: name,
            AVersion:  version,
            AgreeId:   id,
        },
        TheReason:    reason,
    }
}