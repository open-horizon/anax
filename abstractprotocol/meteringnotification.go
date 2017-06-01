package abstractprotocol

import (
    "fmt"
)

// =======================================================================================================
// NotifyMetering - This is the interface that Horizon uses to interact with metering notification
// messages in the agreement protocol.
//

type NotifyMetering interface {
    Meter() string
}

// This struct is the metering notification that flows from the consumer to the producer. It indicates
// that the consumer has seen data being received from the workloads on the device and is granting
// some metering tokens.
type BaseNotifyMetering struct {
    *BaseProtocolMessage
    meterReading string `json:"meter_reading"`
}

func (bn *BaseNotifyMetering) IsValid() bool {
    return bn.BaseProtocolMessage.IsValid() && bn.msgType == MsgTypeNotifyMetering && bn.meterReading != ""
}

func (bn *BaseNotifyMetering) String() string {
    return bn.BaseProtocolMessage.String() + fmt.Sprintf(", MeterReading: %v", bn.meterReading)
}

func (bn *BaseNotifyMetering) ShortString() string {
    return bn.BaseProtocolMessage.ShortString() + fmt.Sprintf(", MeterReading: %v", bn.meterReading)
}

func (bn *BaseNotifyMetering) Meter() string {
    return bn.meterReading
}

func NewNotifyMetering(name string, version int, id string, m string) *BaseNotifyMetering {
    return &BaseNotifyMetering{
        BaseProtocolMessage: &BaseProtocolMessage{
            msgType:        MsgTypeNotifyMetering,
            protocol:       name,
            version:        version,
            agreementId:    id,
        },
        meterReading: m,
    }
}