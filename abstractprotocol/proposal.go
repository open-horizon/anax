package abstractprotocol

import (
    "fmt"
)

// =======================================================================================================
// Proposal - This is the interface that Horizon uses to interact with proposals in the agreement
// protocol.
//

// The interface for the base proposal type
type Proposal interface{
    ProtocolMessage
    TsAndCs()        string
    ProducerPolicy() string
    ConsumerId()     string
}

// A concrete Proposal object that implements all the functions of a Proposal interface. This represents the base protocol object for a proposal. Other
// agreement protocols might wish to embed and then extend this object.
type BaseProposal struct {
    *BaseProtocolMessage
    tsAndCs        string `json:"tsandcs"`        // This is a JSON serialized policy file, merged between consumer and producer. It has 1 workload array element.
    producerPolicy string `json:"producerPolicy"`
    consumerId     string `json:"consumerId"`
}

func NewProposal(name string, version int, tsandcs string, pPol string, agId string, cId string) *BaseProposal {
    return &BaseProposal{
        BaseProtocolMessage: &BaseProtocolMessage{
            msgType:        MsgTypeProposal,
            protocol:       name,
            version:        version,
            agreementId:    agId,
        },
        tsAndCs:        tsandcs,
        producerPolicy: pPol,
        consumerId:     cId,
    }
}

func (bp *BaseProposal) IsValid() bool {
    return bp.BaseProtocolMessage.IsValid() && bp.msgType == MsgTypeProposal && len(bp.tsAndCs) != 0 && len(bp.producerPolicy) != 0 && len(bp.consumerId) != 0
}

func (bp *BaseProposal) String() string {
    return bp.BaseProtocolMessage.String() + fmt.Sprintf(", ConsumerId: %v", bp.consumerId)
}

func (bp *BaseProposal) ShortString() string {
    res := ""
    res += bp.BaseProtocolMessage.String() + fmt.Sprintf(", ConsumerId: %v", bp.consumerId)
    res += fmt.Sprintf(", TsAndCs: %v", bp.tsAndCs[:40])
    res += fmt.Sprintf(", Producer Policy: %v", bp.producerPolicy[:40])
    return res
}

func (bp *BaseProposal) TsAndCs() string {
    return bp.tsAndCs
}

func (bp *BaseProposal) ProducerPolicy() string {
    return bp.producerPolicy
}

func (bp *BaseProposal) ConsumerId() string {
    return bp.consumerId
}