package abstractprotocol

import ()

// =======================================================================================================
// DataReceived - This is the interface that Horizon uses to interact with data received messages
// in the agreement protocol.
//

type DataReceivedAck interface {
}

// This struct is the data received message that flows from the consumer to the producer. It indicates
// that the consumer has seen data being received from the workloads on the device.
type BaseDataReceivedAck struct {
	*BaseProtocolMessage
}

func (dr *BaseDataReceivedAck) IsValid() bool {
	return dr.BaseProtocolMessage.IsValid() && dr.MsgType == MsgTypeDataReceivedAck
}

func (dr *BaseDataReceivedAck) String() string {
	return dr.BaseProtocolMessage.String()
}

func (dr *BaseDataReceivedAck) ShortString() string {
	return dr.BaseProtocolMessage.ShortString()
}

func NewDataReceivedAck(name string, version int, id string) *BaseDataReceivedAck {
	return &BaseDataReceivedAck{
		BaseProtocolMessage: &BaseProtocolMessage{
			MsgType:   MsgTypeDataReceivedAck,
			AProtocol: name,
			AVersion:  version,
			AgreeId:   id,
		},
	}
}
