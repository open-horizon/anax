package abstractprotocol

import (
	"fmt"
	"github.com/open-horizon/anax/cutil"
)

// =======================================================================================================
// Proposal - This is the interface that Horizon uses to interact with proposals in the agreement
// protocol.
//

// The interface for the base proposal type
type Proposal interface {
	ProtocolMessage
	TsAndCs() string
	ProducerPolicy() string
	ConsumerId() string
}

// A concrete Proposal object that implements all the functions of a Proposal interface. This represents the base protocol object for a proposal. Other
// agreement protocols might wish to embed and then extend this object.
type BaseProposal struct {
	*BaseProtocolMessage
	TsandCs        string `json:"tsandcs"` // This is a JSON serialized policy file, merged between consumer and producer. It has 1 workload array element.
	Producerpolicy string `json:"producerPolicy"`
	Consumerid     string `json:"consumerId"`
}

func NewProposal(name string, version int, tsandcs string, pPol string, agId string, cId string) *BaseProposal {
	return &BaseProposal{
		BaseProtocolMessage: &BaseProtocolMessage{
			MsgType:   MsgTypeProposal,
			AProtocol: name,
			AVersion:  version,
			AgreeId:   agId,
		},
		TsandCs:        tsandcs,
		Producerpolicy: pPol,
		Consumerid:     cId,
	}
}

func (bp *BaseProposal) IsValid() bool {
	return bp.BaseProtocolMessage.IsValid() && bp.MsgType == MsgTypeProposal && len(bp.TsandCs) != 0 && len(bp.Producerpolicy) != 0 && len(bp.Consumerid) != 0
}

func (bp *BaseProposal) String() string {
	return bp.BaseProtocolMessage.String() + fmt.Sprintf(", ConsumerId: %v", bp.Consumerid)
}

func (bp *BaseProposal) ShortString() string {
	res := ""
	res += bp.BaseProtocolMessage.String() + fmt.Sprintf(", ConsumerId: %v", bp.Consumerid)
	res += fmt.Sprintf(", TsAndCs: %v", cutil.TruncateDisplayString(bp.TsandCs, 40))
	res += fmt.Sprintf(", Producer Policy: %v", cutil.TruncateDisplayString(bp.Producerpolicy, 40))
	return res
}

func (bp *BaseProposal) TsAndCs() string {
	return bp.TsandCs
}

func (bp *BaseProposal) ProducerPolicy() string {
	return bp.Producerpolicy
}

func (bp *BaseProposal) ConsumerId() string {
	return bp.Consumerid
}
