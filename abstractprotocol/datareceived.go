package abstractprotocol

import (
)

// =======================================================================================================
// DataReceived - This is the interface that Horizon uses to interact with data received messages
// in the agreement protocol.
//

type DataReceived interface {
}

// This struct is the data received message that flows from the consumer to the producer. It indicates
// that the consumer has seen data being received from the workloads on the device.
type BaseDataReceived struct {
    *BaseProtocolMessage
}

func (dr *BaseDataReceived) IsValid() bool {
    return dr.BaseProtocolMessage.IsValid() && dr.msgType == MsgTypeDataReceived
}

func (dr *BaseDataReceived) String() string {
    return dr.BaseProtocolMessage.String()
}

func (dr *BaseDataReceived) ShortString() string {
    return dr.BaseProtocolMessage.ShortString()
}

func NewDataReceived(name string, version int, id string) *BaseDataReceived {
    return &BaseDataReceived{
        BaseProtocolMessage: &BaseProtocolMessage{
            msgType:        MsgTypeDataReceived,
            protocol:       name,
            version:        version,
            agreementId:    id,
        },
    }
}