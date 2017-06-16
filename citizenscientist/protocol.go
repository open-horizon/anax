package citizenscientist

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/abstractprotocol"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/ethblockchain"
	"github.com/open-horizon/anax/metering"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/go-solidity/contract_api"
	"golang.org/x/crypto/sha3"
	"net/http"
	"strconv"
	"time"
)

const PROTOCOL_NAME = "Citizen Scientist"
const PROTOCOL_CURRENT_VERSION = 1

// This struct is the proposal body that flows from the consumer to the producer. It has an additional field for the ethereum account address.
type CSProposal struct {
	*abstractprotocol.BaseProposal
	Address string `json:"address"`
}

func (p *CSProposal) String() string {
	return p.BaseProposal.String() + fmt.Sprintf(", Address: %v", p.Address)
}

func (p *CSProposal) ShortString() string {
	return p.BaseProposal.ShortString() + fmt.Sprintf(", Address: %v", p.Address)
}

func (p *CSProposal) IsValid() bool {
    return p.BaseProposal.IsValid() && len(p.Address) != 0
}

func NewCSProposal(bp *abstractprotocol.BaseProposal, myAddress string) *CSProposal {
	return &CSProposal{
		BaseProposal: bp,
		Address:      myAddress,
	}
}

// This struct is the proposal reply body that flows from the producer to the consumer.
type CSProposalReply struct {
	*abstractprotocol.BaseProposalReply
	Signature string `json:"signature"`
	Address   string `json:"address"`
}

func (pr *CSProposalReply) String() string {
	return pr.BaseProposalReply.String() + fmt.Sprintf(", Address: %v, Signature: %v", pr.Address, pr.Signature)
}

func (pr *CSProposalReply) ShortString() string {
	return pr.BaseProposalReply.ShortString() + fmt.Sprintf(", Address: %v, Signature: %v", pr.Address, pr.Signature)
}

func (pr *CSProposalReply) IsValid() bool {
    return pr.BaseProposalReply.IsValid()
}

func (pr *CSProposalReply) SetSignature(s string) {
    pr.Signature = s
}

func (pr *CSProposalReply) SetAddress(a string) {
    pr.Address = a
}

func NewCSProposalReply(bp *abstractprotocol.BaseProposalReply, sig string, myAddress string) *CSProposalReply {
	return &CSProposalReply{
		BaseProposalReply: bp,
		Signature:         sig,
		Address:           myAddress,
	}
}

// This is the object which users of the agreement protocol use to get access to the protocol functions. It MUST
// implement all the functions in the abstract ProtocolHandler interface.
type ProtocolHandler struct {
	*abstractprotocol.BaseProtocolHandler
	GethURL              string
	MyAddress            string
	EthAgreementContract *contract_api.SolidityContract
	EthMeterContract     *contract_api.SolidityContract
}

func NewProtocolHandler(gethURL string, pm *policy.PolicyManager) *ProtocolHandler {

	acct, _ := ethblockchain.AccountId()

	dir, _ := ethblockchain.DirectoryAddress()
	bc, err := ethblockchain.InitBaseContracts(acct, gethURL, dir)
	if err != nil {
		panic(fmt.Sprintf("%v Protocol Handler unable to initialize platform contracts, error: %v", PROTOCOL_NAME, err))
	}

	bph := abstractprotocol.NewBaseProtocolHandler(PROTOCOL_NAME,
													PROTOCOL_CURRENT_VERSION,
													&http.Client{Timeout: time.Duration(config.HTTPDEFAULTTIMEOUT*time.Millisecond)},
													pm)

	return &ProtocolHandler{
		BaseProtocolHandler:  bph,
		GethURL:              gethURL,
		MyAddress:            acct,
		EthAgreementContract: bc.Agreements,
		EthMeterContract:     bc.Metering,
	}
}

// The implementation of this protocol method augments the proposal with an ethereum specific field before sending the proposal to
// the other party.
func (p *ProtocolHandler) InitiateAgreement(agreementId string,
											producerPolicy *policy.Policy,
											consumerPolicy *policy.Policy,
											myId string,
											messageTarget interface{},
											workload *policy.Workload,
											defaultPW string,
											defaultNoData uint64,
											sendMessage func(msgTarget interface{}, pay []byte) error) (abstractprotocol.Proposal, error) {

	// Create a proposal and augment it with the additional data we need in this protocol.
	var newProposal *CSProposal

	if bp, err := abstractprotocol.CreateProposal(p, agreementId, producerPolicy, consumerPolicy, myId, workload, defaultPW, defaultNoData); err != nil {
		return nil, err
	} else {
		newProposal = NewCSProposal(bp, p.MyAddress)
	}

	// Send the proposal to the other party
	glog.V(5).Infof("Protocol %v sending proposal %s", p.Name(), newProposal)

	if err := abstractprotocol.SendProposal(p, newProposal, consumerPolicy, messageTarget, sendMessage); err != nil {
		return nil, err
	}
	return newProposal, nil

}

// This is an extra method in the citizen scientist protocol that is not part of the base agrement protocol because it is ethereum specific.
// The hash and ethereum signature of the propsal are needed to support metering.
func (p *ProtocolHandler) SignProposal(newProposal abstractprotocol.Proposal) (string, string, error) {
	// Save the hash and our signature of it for later usage
	sig := ""
	hashBytes := sha3.Sum256([]byte(newProposal.TsAndCs()))
	hash := hex.EncodeToString(hashBytes[:])
	glog.V(5).Infof(fmt.Sprintf("CS Protocol initiate agreement using hash %v with agreement %v", hash, newProposal.AgreementId()))

	if signature, err := ethblockchain.SignHash(hash, p.GethURL); err != nil {
		return "", "", errors.New(fmt.Sprintf("CS Protocol initiate agreement received error signing hash %v, error %v", hash, err))
	} else if len(signature) <= 2 {
		return "", "", errors.New(fmt.Sprintf("CS Protocol initiate agreement received incorrect signature %v from eth_sign.", signature))
	} else {
		sig = signature[2:]
	}
	return hash, sig, nil
}

// This is an implementation of the Decide on proposal API. It has been extended to support ethereum and a signature
// of the proposal from the producer.
func (p *ProtocolHandler) DecideOnProposal(proposal abstractprotocol.Proposal,
										myId string,
										messageTarget interface{},
										sendMessage func(mt interface{}, pay []byte) error) (abstractprotocol.ProposalReply, error) {

	reply, replyErr := abstractprotocol.DecideOnProposal(p, proposal, myId)
	newReply := NewCSProposalReply(reply, "", "")

	if replyErr == nil {

		// The proposal reply is set to decision == true at this point. If any of the extended processing done in this
		// function encounters an error then we need to change the decision to false so that the other party will
		// know what we decided.

		hash := sha3.Sum256([]byte(proposal.TsAndCs()))
		glog.V(5).Infof(fmt.Sprintf("Protocol %v decide on proposal using hash %v with agreement %v", p.Name(), hex.EncodeToString(hash[:]), proposal.AgreementId))

		if sig, err := ethblockchain.SignHash(hex.EncodeToString(hash[:]), p.GethURL); err != nil {
			replyErr = errors.New(fmt.Sprintf("Protocol %v decide on proposal received error signing hash %v, error %v", p.Name(), hex.EncodeToString(hash[:]), err))
			newReply.DoNotAcceptProposal()
		} else if len(sig) > 2 {
			newReply.SetSignature(sig[2:])
			newReply.SetAddress(p.MyAddress)
		} else {
			replyErr = errors.New(fmt.Sprintf("Protocol %v received incorrect signature %v from eth_sign.", p.Name(), sig))
			newReply.DoNotAcceptProposal()
		}
	}

	// Always respond to the Proposer
	return abstractprotocol.SendResponse(p, proposal, newReply, replyErr, messageTarget, sendMessage)

}

// The following methods dont implement any extensions to the base agreement protocol.
func (p *ProtocolHandler) Confirm(replyValid bool,
								agreementId string,
								messageTarget interface{},
								sendMessage func(mt interface{}, pay []byte) error) error {
	return abstractprotocol.Confirm(p, replyValid, agreementId, messageTarget, sendMessage)
}

func (p *ProtocolHandler) NotifyDataReceipt(agreementId string,
											messageTarget interface{},
											sendMessage func(mt interface{}, pay []byte) error) error {
	return abstractprotocol.NotifyDataReceipt(p, agreementId, messageTarget, sendMessage)
}

func (p *ProtocolHandler) NotifyDataReceiptAck(agreementId string,
												messageTarget interface{},
												sendMessage func(mt interface{}, pay []byte) error) error {
	return abstractprotocol.NotifyDataReceiptAck(p, agreementId, messageTarget, sendMessage)
}

func (p *ProtocolHandler) NotifyMetering(agreementId string,
										mn *metering.MeteringNotification,
										messageTarget interface{},
										sendMessage func(mt interface{}, pay []byte) error) (string, error) {

	// The metering notification is almost complete. We need to sign the hash.
	hash := mn.GetMeterHash()
	glog.V(5).Infof("CS Protocol signing hash %v for %v, metering notification %v", hash, agreementId, mn)
	sig := ""
	if signature, err := ethblockchain.SignHash(hash, p.GethURL); err != nil {
		return "", errors.New(fmt.Sprintf("CS Protocol sending meter notification received error signing hash %v, error %v", hash, err))
	} else if len(signature) <= 2 {
		return "", errors.New(fmt.Sprintf("CS Protocol sending meter notification received incorrect signature %v from eth_sign.", signature))
	} else {
		sig = signature[2:]
	}

	mn.SetConsumerMeterSignature(sig)
	glog.V(5).Infof("Completed metering notification %v for %v", mn, agreementId)

	// The metering notification is setup, now we can send it.
	return abstractprotocol.NotifyMeterReading(p, agreementId, mn, messageTarget, sendMessage)
}

func (p *ProtocolHandler) ValidateProposal(proposal string) (abstractprotocol.Proposal, error) {

	// attempt deserialization of message
	prop := new(CSProposal)

	if err := json.Unmarshal([]byte(proposal), prop); err != nil {
		return nil, errors.New(fmt.Sprintf("Error deserializing proposal: %s, error: %v", proposal, err))
	} else if !prop.IsValid() {
		return nil, errors.New(fmt.Sprintf("Message is not a Proposal."))
	} else {
		return prop, nil
	}

}

func (p *ProtocolHandler) ValidateReply(reply string) (abstractprotocol.ProposalReply, error) {

	// attempt deserialization of message from msg payload
	proposalReply := new(CSProposalReply)

	if err := json.Unmarshal([]byte(reply), proposalReply); err != nil {
		return nil, errors.New(fmt.Sprintf("Error deserializing reply: %s, error: %v", reply, err))
	} else if proposalReply.IsValid() {
		return proposalReply, nil
	} else {
		return nil, errors.New(fmt.Sprintf("Message is not a Proposal Reply."))
	}

}

func (p *ProtocolHandler) ValidateReplyAck(replyack string) (abstractprotocol.ReplyAck, error) {
	return abstractprotocol.ValidateReplyAck(replyack)
}

func (p *ProtocolHandler) ValidateDataReceived(dr string) (abstractprotocol.DataReceived, error) {
	return abstractprotocol.ValidateDataReceived(dr)
}

func (p *ProtocolHandler) ValidateDataReceivedAck(dra string) (abstractprotocol.DataReceivedAck, error) {
	return abstractprotocol.ValidateDataReceivedAck(dra)
}

func (p *ProtocolHandler) ValidateMeterNotification(mn string) (abstractprotocol.NotifyMetering, error) {
	return abstractprotocol.ValidateMeterNotification(mn)
}

func (p *ProtocolHandler) ValidateCancel(can string) (abstractprotocol.Cancel, error) {
    return abstractprotocol.ValidateCancel(can)
}

func (p *ProtocolHandler) DemarshalProposal(proposal string) (abstractprotocol.Proposal, error) {

	// attempt deserialization of the proposal
	prop := new(CSProposal)

	if err := json.Unmarshal([]byte(proposal), &prop); err != nil {
		return nil, errors.New(fmt.Sprintf("Error deserializing proposal: %s, error: %v", proposal, err))
	} else {
		return prop, nil
	}

}

func (p *ProtocolHandler) RecordAgreement(newProposal abstractprotocol.Proposal,
										reply abstractprotocol.ProposalReply,
										consumerPolicy *policy.Policy) error {

	if csReply, ok := reply.(*CSProposalReply); !ok {
		return errors.New(fmt.Sprintf("Error casting reply %v to %v extended reply, input reply is %T.", reply, p.Name(), reply))
	} else if binaryAgreementId, err := hex.DecodeString(newProposal.AgreementId()); err != nil {
		return errors.New(fmt.Sprintf("Error converting agreement ID %v to binary, error: %v", newProposal.AgreementId(), err))
	} else {

		// Tell the policy manager that we're in this agreement
		if cerr := abstractprotocol.RecordAgreement(p, newProposal, consumerPolicy); cerr != nil {
			glog.Errorf(fmt.Sprintf("Error finalizing agreement %v in PM %v", newProposal.AgreementId(), cerr))
		}

		tcHash := sha3.Sum256([]byte(newProposal.TsAndCs()))
		glog.V(5).Infof("CS Protocol using hash %v to record agreement %v", hex.EncodeToString(tcHash[:]), newProposal.AgreementId())

		params := make([]interface{}, 0, 10)
		params = append(params, binaryAgreementId)
		params = append(params, tcHash[:])
		params = append(params, csReply.Signature)
		params = append(params, csReply.Address)

		if _, err := p.EthAgreementContract.Invoke_method("create_agreement", params); err != nil {
			return errors.New(fmt.Sprintf("Error invoking create_agreement %v with %v, error: %v", newProposal.AgreementId(), params, err))
		}
	}

	return nil

}

func (p *ProtocolHandler) TerminateAgreement(policy *policy.Policy,
											counterParty string,
											agreementId string,
											reason uint,
											messageTarget interface{},
											sendMessage func(mt interface{}, pay []byte) error) error {

	if binaryAgreementId, err := hex.DecodeString(agreementId); err != nil {
		return errors.New(fmt.Sprintf("Error converting agreement ID %v to binary, error: %v", agreementId, err))
	} else {

		// Tell the policy manager that we're terminating this agreement
		if cerr := abstractprotocol.TerminateAgreement(p, policy, agreementId, reason); cerr != nil {
			glog.Errorf(fmt.Sprintf("Error cancelling agreement %v in PM %v", agreementId, cerr))
		}

		// If the cancel reason is due to a blockchain write failure, then we dont need to do the cancel on the blockchain.
		if counterParty != "" && reason != AB_CANCEL_BC_WRITE_FAILED {
			// Setup parameters for call to the blockchain
			params := make([]interface{}, 0, 10)
			params = append(params, counterParty)
			params = append(params, binaryAgreementId)
			params = append(params, int(reason))

			if _, err := p.EthAgreementContract.Invoke_method("terminate_agreement", params); err != nil {
				return errors.New(fmt.Sprintf("Error invoking terminate_agreement %v with %v, error: %v", agreementId, params, err))
			}
		}
	}

	return nil

}

func (p *ProtocolHandler) VerifyAgreement(agreementId string,
										counterPartyAddress string,
										expectedSignature string) (bool, error) {

	if binaryAgreementId, err := hex.DecodeString(agreementId); err != nil {
		return false, errors.New(fmt.Sprintf("Error converting agreement ID %v to binary, error: %v", agreementId, err))
	} else {

		params := make([]interface{}, 0, 10)
		params = append(params, counterPartyAddress)
		params = append(params, binaryAgreementId)

		if returnedSig, err := p.EthAgreementContract.Invoke_method("get_producer_signature", params); err != nil {
			return false, errors.New(fmt.Sprintf("Error invoking get_contract_signature for %v with %v, error: %v", agreementId, params, err))
		} else {
			sigString := hex.EncodeToString(returnedSig.([]byte))
			glog.V(5).Infof("Verify agreement for %v with %v returned signature: %v", agreementId, counterPartyAddress, sigString)
			if sigString == expectedSignature {
				return true, nil
			} else {
				glog.V(3).Infof("CS Protocol returned signature %v does not match expected signature %v for %v", sigString, expectedSignature, agreementId)
				return false, nil
			}
		}
	}

	return false, nil
}

func (p *ProtocolHandler) RecordMeter(agreementId string, mn *metering.MeteringNotification) error {

	if binaryAgreementId, err := hex.DecodeString(agreementId); err != nil {
		return errors.New(fmt.Sprintf("Error converting agreement ID %v to binary, error: %v", agreementId, err))
	} else {
		glog.V(5).Infof("CS Protocol writing Metering Notification %v to the blockchain for %v.", *mn, agreementId)
		params := make([]interface{}, 0, 10)
		params = append(params, mn.Amount)
	    params = append(params, mn.CurrentTime)
	    params = append(params, binaryAgreementId)
	    params = append(params, mn.GetMeterHash()[2:])
	    params = append(params, mn.ConsumerMeterSignature)
	    params = append(params, mn.AgreementHash)
	    params = append(params, mn.ProducerSignature)
	    params = append(params, mn.ConsumerSignature)
	    params = append(params, mn.ConsumerAddress)
	    if _, err = p.EthMeterContract.Invoke_method("create_meter", params); err != nil {
			return errors.New(fmt.Sprintf("Error invoking create_meter %v with %v, error: %v", agreementId, p, err))
		}
	}

	return nil

}

// Functions that work with blockchain events

const AGREEMENT_CREATE        = "0x0000000000000000000000000000000000000000000000000000000000000000"
const AGREEMENT_DETAIL        = "0x0000000000000000000000000000000000000000000000000000000000000001"
const AGREEMENT_FRAUD         = "0x0000000000000000000000000000000000000000000000000000000000000002"
const AGREEMENT_CONSUMER_TERM = "0x0000000000000000000000000000000000000000000000000000000000000003"
const AGREEMENT_PRODUCER_TERM = "0x0000000000000000000000000000000000000000000000000000000000000004"
const AGREEMENT_FRAUD_TERM    = "0x0000000000000000000000000000000000000000000000000000000000000005"
const AGREEMENT_ADMIN_TERM    = "0x0000000000000000000000000000000000000000000000000000000000000006"

func (p *ProtocolHandler) DemarshalEvent(ev string) (*ethblockchain.Raw_Event, error) {
	rawEvent := new(ethblockchain.Raw_Event)
	if err := json.Unmarshal([]byte(ev), rawEvent); err != nil {
		return nil, err
	} else {
		return rawEvent, nil
	}
}

func (p *ProtocolHandler) AgreementCreated(ev *ethblockchain.Raw_Event) bool {
	return ev.Topics[0] == AGREEMENT_CREATE
}

func (p *ProtocolHandler) ConsumerTermination(ev *ethblockchain.Raw_Event) bool {
	return ev.Topics[0] == AGREEMENT_CONSUMER_TERM
}

func (p *ProtocolHandler) ProducerTermination(ev *ethblockchain.Raw_Event) bool {
	return ev.Topics[0] == AGREEMENT_PRODUCER_TERM
}

func (p *ProtocolHandler) GetAgreementId(ev *ethblockchain.Raw_Event) string {
	return ev.Topics[3][2:]
}

func (p *ProtocolHandler) GetReasonCode(ev *ethblockchain.Raw_Event) (uint64, error) {
	return strconv.ParseUint(ev.Data[2:], 16, 64)
}

// constants indicating why an agreement is cancelled by the producer
const CANCEL_NOT_FINALIZED_TIMEOUT = 100  // x64
const CANCEL_POLICY_CHANGED        = 101
const CANCEL_TORRENT_FAILURE       = 102
const CANCEL_CONTAINER_FAILURE     = 103
const CANCEL_NOT_EXECUTED_TIMEOUT  = 104
const CANCEL_USER_REQUESTED        = 105
const CANCEL_AGBOT_REQUESTED       = 106  // x6a
const CANCEL_NO_REPLY_ACK          = 107

// These constants represent consumer cancellation reason codes
const AB_CANCEL_NOT_FINALIZED_TIMEOUT = 200  // xc8
const AB_CANCEL_NO_REPLY              = 201
const AB_CANCEL_NEGATIVE_REPLY        = 202
const AB_CANCEL_NO_DATA_RECEIVED      = 203
const AB_CANCEL_POLICY_CHANGED        = 204
const AB_CANCEL_DISCOVERED            = 205  // xcd
const AB_USER_REQUESTED               = 206
const AB_CANCEL_FORCED_UPGRADE        = 207
const AB_CANCEL_BC_WRITE_FAILED       = 208  // xd0

func DecodeReasonCode(code uint64) string {

	codeMeanings := map[uint64]string{CANCEL_NOT_FINALIZED_TIMEOUT:    "agreement never appeared on the blockchain",
									CANCEL_POLICY_CHANGED:           "producer policy changed",
									CANCEL_TORRENT_FAILURE:          "torrent failed to download",
									CANCEL_CONTAINER_FAILURE:        "workload terminated",
									CANCEL_NOT_EXECUTED_TIMEOUT:     "workload start timeout",
									CANCEL_USER_REQUESTED:           "user requested",
									CANCEL_AGBOT_REQUESTED:          "agbot requested",
									CANCEL_NO_REPLY_ACK:             "agreement protocol incomplete, no reply ack received",
									AB_CANCEL_NOT_FINALIZED_TIMEOUT: "agreement bot never detected agreement on the blockchain",
									AB_CANCEL_NO_REPLY:              "agreement bot never received reply to proposal",
									AB_CANCEL_NEGATIVE_REPLY:        "agreement bot received negative reply",
									AB_CANCEL_NO_DATA_RECEIVED:      "agreement bot did not detect data",
									AB_CANCEL_POLICY_CHANGED:        "agreement bot policy changed",
									AB_CANCEL_DISCOVERED:            "agreement bot discovered cancellation from producer",
									AB_USER_REQUESTED:               "agreement bot user requested",
									AB_CANCEL_FORCED_UPGRADE:        "agreement bot user requested workload upgrade",
									AB_CANCEL_BC_WRITE_FAILED:       "agreement bot agreement write failed"}

	if reasonString, ok := codeMeanings[code]; !ok {
		return "unknown reason code, device might be downlevel"
	} else {
		return reasonString
	}
}
