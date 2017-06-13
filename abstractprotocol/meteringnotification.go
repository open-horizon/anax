package abstractprotocol

import (
	"fmt"
)

// =======================================================================================================
// NotifyMetering - This is the interface that Horizon uses to interact with metering notification
// messages in the agreement protocol.
//

type NotifyMetering interface {
	ProtocolMessage
	Meter() string
}

// This struct is the metering notification that flows from the consumer to the producer. It indicates
// that the consumer has seen data being received from the workloads on the device and is granting
// some metering tokens.
type BaseNotifyMetering struct {
	*BaseProtocolMessage
	MeterReading string `json:"meter_reading"`
}

func (bn *BaseNotifyMetering) IsValid() bool {
	return bn.BaseProtocolMessage.IsValid() && bn.MsgType == MsgTypeNotifyMetering && bn.MeterReading != ""
}

func (bn *BaseNotifyMetering) String() string {
	return bn.BaseProtocolMessage.String() + fmt.Sprintf(", MeterReading: %v", bn.MeterReading)
}

func (bn *BaseNotifyMetering) ShortString() string {
	return bn.BaseProtocolMessage.ShortString() + fmt.Sprintf(", MeterReading: %v", bn.MeterReading)
}

func (bn *BaseNotifyMetering) Meter() string {
	return bn.MeterReading
}

func NewNotifyMetering(name string, version int, id string, m string) *BaseNotifyMetering {
	return &BaseNotifyMetering{
		BaseProtocolMessage: &BaseProtocolMessage{
			MsgType:   MsgTypeNotifyMetering,
			AProtocol: name,
			AVersion:  version,
			AgreeId:   id,
		},
		MeterReading: m,
	}
}
