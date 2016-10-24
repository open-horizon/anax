package citizenscientist

import (
    "encoding/hex"
    "encoding/json"
    "errors"
    "fmt"
    "github.com/golang/glog"
    "golang.org/x/crypto/sha3"
    "math/rand"
    "net/http"
    "github.com/open-horizon/anax/ethblockchain"
    "github.com/open-horizon/anax/exchange"
    "github.com/open-horizon/anax/policy"
    "github.com/open-horizon/go-solidity/contract_api"
    gwhisper "github.com/open-horizon/go-whisper"
)

const PROTOCOL_NAME = "Citizen Scientist"

// This struct is the proposal body that flows from the consumer to the producer.
const MsgTypeProposal = "proposal"
const MsgTypeReply = "reply"
type Proposal struct {
    Type           string `json:"type"`
    TsAndCs        string `json:"tsandcs"`
    ProducerPolicy string `json:"producerPolicy"`
    AgreementId    string `json:"agreementId"`
    Address        string `json:"address"`
}

// This struct is the proposal reply body that flows from the producer to the consumer.
type ProposalReply struct {
    Type           string `json:"type"`
    Decision       bool   `json:"decision"`
    Signature      string `json:"signature"`
    Address        string `json:"address"`
    AgreeId        string `json:"agreementId"`
}

func (p *ProposalReply) ProposalAccepted() bool {
    return p.Decision
}

func (p *ProposalReply) AgreementId() string {
    return p.AgreeId
}

func NewProposalReply (decision bool, id string) *ProposalReply {
    return &ProposalReply{
        Type: MsgTypeReply,
        Decision: false,
        AgreeId: id,
    }
}

type ProtocolHandler struct {
    GethURL        string
    httpClient    *http.Client
    random        *rand.Rand
    pm            *policy.PolicyManager    // TODO: Get rid of this field
}

func NewProtocolHandler (gethURL string, pm *policy.PolicyManager) *ProtocolHandler {
    return &ProtocolHandler{
        GethURL: gethURL,
        httpClient: &http.Client{},
        pm: pm,
    }
}

func (p *ProtocolHandler) InitiateAgreement(agreementId []byte, producerPolicy *policy.Policy, consumerPolicy *policy.Policy, device *exchange.Device, myAddress string) (*Proposal, error) {

    if TCPolicy, err := policy.Create_Terms_And_Conditions(producerPolicy, consumerPolicy); err != nil {
        return nil, errors.New(fmt.Sprintf("CS Protocol initiation received error trying to merge policy %v and %v, error: %v",  producerPolicy, consumerPolicy, err))
    } else {
        glog.V(5).Infof("Merged Policy %v", *TCPolicy)

        newProposal := new(Proposal)
        if tcBytes, err := json.Marshal(TCPolicy); err != nil {
            return nil, errors.New(fmt.Sprintf("Error marshalling TsAndCs %v, error: %v", *TCPolicy, err))
        } else if prodBytes, err := json.Marshal(producerPolicy); err != nil {
            return nil, errors.New(fmt.Sprintf("Error marshalling Producer Policy %v, error: %v", *producerPolicy, err))
        } else {
            newProposal.Type = MsgTypeProposal
            newProposal.TsAndCs = string(tcBytes)
            newProposal.ProducerPolicy = string(prodBytes)
            newProposal.AgreementId = hex.EncodeToString(agreementId)
            newProposal.Address = myAddress

            topic := TCPolicy.AgreementProtocols[0].Name
            glog.V(5).Infof("Sending proposal %v", *newProposal)

            // Send a whisper message to the device to initiate the agreement protocol.
            if err := p. sendProposal(device.MsgEndPoint, topic, newProposal); err != nil {
                return nil, errors.New(fmt.Sprintf("Error sending proposal %v, %v", *newProposal, err))
            } else {
                return newProposal, nil
            }
        }
    }

}

func (p *ProtocolHandler) DecideOnProposal(proposal *Proposal, from string) (*ProposalReply, error) {
    glog.V(3).Infof(fmt.Sprintf("Processing New proposal from %v, %v", from, proposal))

    reply := NewProposalReply(false, proposal.AgreementId)

    var termsAndConditions, producerPolicy *policy.Policy

    // Marshal the policies in the proposal into in memory policy objects
    if tcPolicy, err := policy.DemarshalPolicy(proposal.TsAndCs); err != nil {
        return nil, errors.New(fmt.Sprintf("CS Protocol decide on proposal received error demarshalling TsAndCs, %v", err))
    } else if pPolicy, err := policy.DemarshalPolicy(proposal.ProducerPolicy); err != nil {
        return nil, errors.New(fmt.Sprintf("CS Protocol decide on proposal received error demarshalling Producer Policy, %v", err))
    } else {
        termsAndConditions = tcPolicy
        producerPolicy = pPolicy
    }

    // The consumer will send 2 policies, one is the merged policy that represents the
    // terms and conditions of the agreement. The other is a copy of my policy that he thinks
    // he is matching. Let's make sure it is one of my policies.
    if err := p.pm.MatchesMine(producerPolicy); err != nil {
        return nil, errors.New(fmt.Sprintf("CS Protocol decide on proposal received error, producer policy from proposal is not one of our current policies, rejecting proposal: %v", err))

    // Now check to make sure that the merged policy is acceptable.
    } else if err := policy.Are_Compatible(producerPolicy, termsAndConditions); err != nil {
        return nil, errors.New(fmt.Sprintf("CS Protocol decide on proposal received error, T and C policy is not compatible, rejecting proposal: %v", err))
    } else {

        hash := sha3.Sum256([]byte(proposal.TsAndCs))
        glog.V(5).Infof(fmt.Sprintf("CS Protocol decide on proposal using hash %v with agreement %v", hex.EncodeToString(hash[:]), proposal.AgreementId))

        if sig, err := ethblockchain.SignHash(hex.EncodeToString(hash[:]), p.GethURL); err != nil {
            return nil, errors.New(fmt.Sprintf("CS Protocol decide on proposal received error signing hash %v, error %v", hex.EncodeToString(hash[:]), err))
        } else {
            reply.Decision = true
            reply.Address, _ = ethblockchain.AccountId()
            reply.Signature = sig[2:]
        }
    }

    // Respond to the Proposer.
    if err := p.sendResponse(from, "Citizen Scientist", reply); err != nil {
        reply.Decision = false
        return nil, errors.New(fmt.Sprintf("CS Protocol decide on proposal received error trying to send proposal response, error: %v", err))
    }

    return reply, nil
}



func (p *ProtocolHandler) sendProposal(to string, topic string, proposal *Proposal) error {
    pay, err := json.Marshal(proposal)
    if err != nil {
        return errors.New(fmt.Sprintf("Unable to serialize payload %v, error: %v", *proposal, err))
    }

    glog.V(3).Infof("Sending proposal message: %v to: %v", string(pay), to)

    if from, err := gwhisper.AccountId(p.GethURL); err != nil {
        return errors.New(fmt.Sprintf("Error obtaining whisper id: %v", err))
    } else {
        // this is to last long enough to be read by even an overloaded governor but still expire before a new worker might try to pick up the contract
        msg, err := gwhisper.TopicMsgParams(from, to, []string{topic}, string(pay), 900, 50)
        if err != nil {
            return errors.New(fmt.Sprintf("Error creating whisper message topic parameters: %v", err))
        }

        _, err = gwhisper.WhisperSend(p.httpClient, p.GethURL, gwhisper.POST, msg, 3)
        if err != nil {
            return errors.New(fmt.Sprintf("Error sending whisper message: %v, error: %v", msg, err))
        }
        return nil
    }
}


func (p *ProtocolHandler) sendResponse(to string, topic string, reply *ProposalReply) error {
    pay, err := json.Marshal(reply)
    if err != nil {
        return errors.New(fmt.Sprintf("Unable to serialize payload %v, error %v", *reply, err))
    }

    glog.V(3).Infof("Sending proposal response message: %v to: %v", string(pay), to)

    if from, err := gwhisper.AccountId(p.GethURL); err != nil {
        glog.Errorf("Error obtaining whisper id, %v", err)
        return err
    } else {

        // this has to last long enough to be read by even an overloaded governor but still expire before a new worker might try to pick up the contract
        msg, err := gwhisper.TopicMsgParams(from, to, []string{topic}, string(pay), 900, 50)
        if err != nil {
            return err
        }

        _, err = gwhisper.WhisperSend(p.httpClient, p.GethURL, gwhisper.POST, msg, 3)
        if err != nil {
            return err
        }
        return nil
    }
}

func (p *ProtocolHandler) ValidateReply(reply string) (*ProposalReply, error) {

    // attempt deserialization of message from msg payload
    proposalReply := new(ProposalReply)

    if err := json.Unmarshal([]byte(reply), &proposalReply); err != nil {
        return nil, errors.New(fmt.Sprintf("Error deserializing reply: %s, error: %v", reply, err))
    } else if proposalReply.Type == MsgTypeReply && len(proposalReply.Signature) != 0 && len(proposalReply.Address) != 0 && len(proposalReply.AgreeId) != 0 {
        return proposalReply, nil
    } else {
        return nil, errors.New(fmt.Sprintf("Reply message: %s, is not a Proposal reply.", reply))
    }

}

func (p *ProtocolHandler) ValidateProposal(proposal string) (*Proposal, error) {

    // attempt deserialization of message
    prop := new(Proposal)

    if err := json.Unmarshal([]byte(proposal), &prop); err != nil {
        return nil, errors.New(fmt.Sprintf("Error deserializing proposal: %s, error: %v", proposal, err))
    } else if prop.Type == MsgTypeProposal && len(prop.TsAndCs) != 0 && len(prop.ProducerPolicy) != 0 && len(prop.AgreementId) != 0 && len(prop.Address) != 0 {
        return prop, nil
    } else {
        return nil, errors.New(fmt.Sprintf("Proposal message: %s, is not a Proposal.", proposal))
    }

}


func (p *ProtocolHandler) RecordAgreement(newProposal *Proposal, reply *ProposalReply, con *contract_api.SolidityContract) error {

    if binaryAgreementId, err := hex.DecodeString(newProposal.AgreementId); err != nil {
        return errors.New(fmt.Sprintf("Error converting agreement ID %v to binary, error: %v", newProposal.AgreementId, err))
    } else {

        tcHash := sha3.Sum256([]byte(newProposal.TsAndCs))
        glog.V(5).Infof("Using hash %v to record agreement %v", hex.EncodeToString(tcHash[:]), newProposal.AgreementId)

        params := make([]interface{}, 0, 10)
        params = append(params, binaryAgreementId)
        params = append(params, tcHash[:])
        params = append(params, reply.Signature)
        params = append(params, reply.Address)

        if _, err := con.Invoke_method("create_agreement", params); err != nil {
            return errors.New(fmt.Sprintf("Error invoking create_agreement with %v, error: %v", params, err))
        }
    }

    return nil

}

func (p *ProtocolHandler) TerminateAgreement(counterParty string, agreementId string, reason uint, con *contract_api.SolidityContract) error {

    if binaryAgreementId, err := hex.DecodeString(agreementId); err != nil {
        return errors.New(fmt.Sprintf("Error converting agreement ID %v to binary, error: %v", agreementId, err))
    } else {

        params := make([]interface{}, 0, 10)
        params = append(params, counterParty)
        params = append(params, binaryAgreementId)
        params = append(params, int(reason))

        if _, err := con.Invoke_method("terminate_agreement", params); err != nil {
            return errors.New(fmt.Sprintf("Error invoking terminate_agreement with %v, error: %v", params, err))
        }
    }

    return nil

}

func (p *ProtocolHandler) VerifyAgreementRecorded(agreementId string, counterPartyAddress string, expectedSignature string, con *contract_api.SolidityContract) (bool, error) {

    if binaryAgreementId, err := hex.DecodeString(agreementId); err != nil {
        return false, errors.New(fmt.Sprintf("Error converting agreement ID %v to binary, error: %v", agreementId, err))
    } else {

        // glog.V(5).Infof("Using hash %v to record agreement %v", hex.EncodeToString(tcHash[:]), newProposal.AgreementId)

        params := make([]interface{}, 0, 10)
        params = append(params, counterPartyAddress)
        params = append(params, binaryAgreementId)        

        if returnedSig, err := con.Invoke_method("get_producer_signature", params); err != nil {
            return false, errors.New(fmt.Sprintf("Error invoking get_contract_signature with %v, error: %v", params, err))
        } else {
            sigString := hex.EncodeToString(returnedSig.([]byte))
            glog.V(5).Infof("Verify agreement for %v with %v returned signature: %v", agreementId, counterPartyAddress, sigString)
            if sigString == expectedSignature {
                return true, nil
            } else {
                return false, nil
            }
        }
    }

    return false, nil
}


