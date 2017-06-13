package abstractprotocol

import (
	"fmt"
)

// =======================================================================================================
// ProposalReply - This is the interface that Horizon uses to interact with proposal replies in the
// agreement protocol.
//

// The interface for the base proposal type
type ProposalReply interface {
	ProtocolMessage
	ProposalAccepted() bool
	DeviceId() string
	AcceptProposal()
	DoNotAcceptProposal()
}

// A concrete ProposalReply object that implements all the functions of a ProposalReply interface. This represents the base protocol
// object for a proposal reply. Other agreement protocols might wish to embed and then extend this object.
type BaseProposalReply struct {
	*BaseProtocolMessage
	Decision bool   `json:"decision"`
	Deviceid string `json:"deviceId"`
}

func (bp *BaseProposalReply) IsValid() bool {
	return bp.BaseProtocolMessage.IsValid() && bp.MsgType == MsgTypeReply && len(bp.Deviceid) != 0
}

func (bp *BaseProposalReply) String() string {
	return bp.BaseProtocolMessage.String() + fmt.Sprintf(", Decision: %v, DeviceId: %v", bp.Decision, bp.Deviceid)
}

func (bp *BaseProposalReply) ShortString() string {
	return bp.BaseProtocolMessage.ShortString() + fmt.Sprintf(", Decision: %v, DeviceId: %v", bp.Decision, bp.Deviceid)
}

func (bp *BaseProposalReply) ProposalAccepted() bool {
	return bp.Decision
}

func (bp *BaseProposalReply) DeviceId() string {
	return bp.Deviceid
}

func (bp *BaseProposalReply) AcceptProposal() {
	bp.Decision = true
}

func (bp *BaseProposalReply) DoNotAcceptProposal() {
	bp.Decision = false
}

func NewProposalReply(name string, version int, id string, deviceId string) *BaseProposalReply {
	return &BaseProposalReply{
		BaseProtocolMessage: &BaseProtocolMessage{
			MsgType:   MsgTypeReply,
			AProtocol: name,
			AVersion:  version,
			AgreeId:   id,
		},
		Decision: false,
		Deviceid: deviceId,
	}
}
