package abstractprotocol

import (
    "fmt"
)

// =======================================================================================================
// ProposalReply - This is the interface that Horizon uses to interact with proposal replies in the
// agreement protocol.
//

// The interface for the base proposal type
type ProposalReply interface{
    ProposalAccepted() bool
    DeviceId()         string
    AcceptProposal()
    DoNotAcceptProposal()
}

// A concrete ProposalReply object that implements all the functions of a ProposalReply interface. This represents the base protocol
// object for a proposal reply. Other agreement protocols might wish to embed and then extend this object.
type BaseProposalReply struct {
    *BaseProtocolMessage
    decision  bool   `json:"decision"`
    deviceId  string `json:"deviceId"`
}

func (bp *BaseProposalReply) IsValid() bool {
    return bp.BaseProtocolMessage.IsValid() && bp.msgType == MsgTypeReply && len(bp.deviceId) != 0
}

func (bp *BaseProposalReply) String() string {
    return bp.BaseProtocolMessage.String() + fmt.Sprintf(", Decision: %v, DeviceId: %v", bp.decision, bp.deviceId)
}

func (bp *BaseProposalReply) ShortString() string {
    return bp.BaseProtocolMessage.ShortString() + fmt.Sprintf(", Decision: %v, DeviceId: %v", bp.decision, bp.deviceId)
}

func (bp *BaseProposalReply) ProposalAccepted() bool {
    return bp.decision
}

func (bp *BaseProposalReply) DeviceId() string {
    return bp.deviceId
}

func (bp *BaseProposalReply) AcceptProposal() {
    bp.decision = true
}

func (bp *BaseProposalReply) DoNotAcceptProposal() {
    bp.decision = false
}

func NewProposalReply(name string, version int, id string, deviceId string) *BaseProposalReply {
    return &BaseProposalReply{
        BaseProtocolMessage: &BaseProtocolMessage{
            msgType:        MsgTypeReply,
            protocol:       name,
            version:        version,
            agreementId:    id,
        },
        decision: false,
        deviceId: deviceId,
    }
}