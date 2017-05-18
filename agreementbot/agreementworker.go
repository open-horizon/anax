package agreementbot

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/citizenscientist"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/ethblockchain"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/policy"
	"github.com/satori/go.uuid"
	"math/rand"
	"net/http"
	"runtime"
	"strconv"
	"time"
)

type CSAgreementWorker struct {
	pm         *policy.PolicyManager
	db         *bolt.DB
	config     *config.HorizonConfig
	httpClient *http.Client
	agbotId    string
	token      string
	protocol   string
	alm        *AgreementLockManager
}

func NewCSAgreementWorker(policyManager *policy.PolicyManager, config *config.HorizonConfig, db *bolt.DB, alm *AgreementLockManager) *CSAgreementWorker {

	p := &CSAgreementWorker{
		pm:         policyManager,
		db:         db,
		config:     config,
		httpClient: &http.Client{},
		agbotId:    config.AgreementBot.ExchangeId,
		token:      config.AgreementBot.ExchangeToken,
		alm:        alm,
	}

	return p
}

// These structs are the event bodies that flows from the processor to the agreement workers
const INITIATE = "INITIATE_AGREEMENT"
const REPLY = "AGREEMENT_REPLY"
const CANCEL = "AGREEMENT_CANCEL"
const DATARECEIVEDACK = "AGREEMENT_DATARECEIVED_ACK"
const BC_RECORDED = "AGREEMENT_BC_RECORDED"
const BC_TERMINATED = "AGREEMENT_BC_TERMINATED"

type CSAgreementWork interface {
	Type() string
}

type CSInitiateAgreement struct {
	workType       string
	ProducerPolicy policy.Policy               // the producer policy received from the exchange
	ConsumerPolicy policy.Policy               // the consumer policy we're matched up with
	Device         exchange.SearchResultDevice // the device entry in the exchange
}

func (c CSInitiateAgreement) String() string {
	res := ""
	res += fmt.Sprintf("Workitem: %v\n", c.workType)
	res += fmt.Sprintf("Producer Policy: %v\n", c.ProducerPolicy)
	res += fmt.Sprintf("Consumer Policy: %v\n", c.ConsumerPolicy)
	res += fmt.Sprintf("Device: %v", c.Device)
	return res
}

func (c CSInitiateAgreement) Type() string {
	return c.workType
}

type CSHandleReply struct {
	workType     string
	Reply        string
	From         string // deprecated whisper address
	SenderId     string // exchange Id of sender
	SenderPubKey []byte
	MessageId    int
}

func (c CSHandleReply) String() string {
	return fmt.Sprintf("Workitem: %v, SenderId: %v, MessageId: %v, From: %v, Reply: %v, SenderPubKey: %x", c.workType, c.SenderId, c.MessageId, c.From, c.Reply, c.SenderPubKey)
}

func (c CSHandleReply) Type() string {
	return c.workType
}

type CSHandleDataReceivedAck struct {
	workType     string
	Ack          string
	From         string // deprecated whisper address
	SenderId     string // exchange Id of sender
	SenderPubKey []byte
	MessageId    int
}

func (c CSHandleDataReceivedAck) String() string {
	return fmt.Sprintf("Workitem: %v, SenderId: %v, MessageId: %v, From: %v, Ack: %v, SenderPubKey: %x", c.workType, c.SenderId, c.MessageId, c.From, c.Ack, c.SenderPubKey)
}

func (c CSHandleDataReceivedAck) Type() string {
	return c.workType
}

type CSCancelAgreement struct {
	workType    string
	AgreementId string
	Protocol    string
	Reason      uint
}

func (c CSCancelAgreement) Type() string {
	return c.workType
}

type CSHandleBCRecorded struct {
	workType    string
	AgreementId string
	Protocol    string
}

func (c CSHandleBCRecorded) Type() string {
	return c.workType
}

type CSHandleBCTerminated struct {
	workType    string
	AgreementId string
	Protocol    string
}

func (c CSHandleBCTerminated) Type() string {
	return c.workType
}

// This function receives an event to "make a new agreement" from the Process function, and then synchronously calls a function
// to actually work through the agreement protocol.
func (a *CSAgreementWorker) start(work chan CSAgreementWork, random *rand.Rand, bc *ethblockchain.BaseContracts) {
	workerID := uuid.NewV4().String()

	logString := func(v interface{}) string {
		return fmt.Sprintf("CSAgreementWorker (%v): %v", workerID, v)
	}

	sendMessage := func(mt interface{}, pay []byte) error {
		// The mt parameter is an abstract message target object that is passed to this routine
		// by the agreement protocol. It's an interface{} type so that we can avoid the protocol knowing
		// about non protocol types.

		var messageTarget *exchange.ExchangeMessageTarget
		switch mt.(type) {
		case *exchange.ExchangeMessageTarget:
			messageTarget = mt.(*exchange.ExchangeMessageTarget)
		default:
			return errors.New(fmt.Sprintf("input message target is %T, expecting exchange.MessageTarget", mt))
		}

		// Grab the exchange ID of the message receiver
		glog.V(3).Infof("Sending exchange message to: %v, message %v", messageTarget.ReceiverExchangeId, string(pay))

		// Get my own keys
		myPubKey, myPrivKey, _ := exchange.GetKeys(a.config.AgreementBot.MessageKeyPath)

		// Demarshal the receiver's public key if we need to
		if messageTarget.ReceiverPublicKeyObj == nil {
			if mtpk, err := exchange.DemarshalPublicKey(messageTarget.ReceiverPublicKeyBytes); err != nil {
				return errors.New(fmt.Sprintf("Unable to demarshal device's public key %x, error %v", messageTarget.ReceiverPublicKeyBytes, err))
			} else {
				messageTarget.ReceiverPublicKeyObj = mtpk
			}
		}

		// Create an encrypted message
		if encryptedMsg, err := exchange.ConstructExchangeMessage(pay, myPubKey, myPrivKey, messageTarget.ReceiverPublicKeyObj); err != nil {
			return errors.New(fmt.Sprintf("Unable to construct encrypted message, error %v for message %s", err, pay))
			// Marshal it into a byte array
		} else if msgBody, err := json.Marshal(encryptedMsg); err != nil {
			return errors.New(fmt.Sprintf("Unable to marshal exchange message, error %v for message %v", err, encryptedMsg))
			// Send it to the device's message queue
		} else {
			pm := exchange.CreatePostMessage(msgBody, a.config.AgreementBot.ExchangeMessageTTL)
			var resp interface{}
			resp = new(exchange.PostDeviceResponse)
			targetURL := a.config.AgreementBot.ExchangeURL + "devices/" + messageTarget.ReceiverExchangeId + "/msgs"
			for {
				if err, tpErr := exchange.InvokeExchange(a.httpClient, "POST", targetURL, a.agbotId, a.token, pm, &resp); err != nil {
					return err
				} else if tpErr != nil {
					glog.V(5).Infof(tpErr.Error())
					time.Sleep(10 * time.Second)
					continue
				} else {
					glog.V(5).Infof("Sent message for %v to exchange.", messageTarget.ReceiverExchangeId)
					return nil
				}
			}
		}

		return nil
	}

	protocolHandler := citizenscientist.NewProtocolHandler(a.config.AgreementBot.GethURL, a.pm)

	for {
		glog.V(5).Infof(logString(fmt.Sprintf("blocking for work")))
		workItem := <-work // block waiting for work
		glog.V(2).Infof(logString(fmt.Sprintf("received work: %v", workItem)))

		if workItem.Type() == INITIATE {
			wi := workItem.(CSInitiateAgreement)
			a.initiateNewAgreement(wi, random, protocolHandler, sendMessage)

		} else if workItem.Type() == REPLY {
			wi := workItem.(CSHandleReply)
			a.handleAgreementReply(wi, protocolHandler, bc, sendMessage)

		} else if workItem.Type() == DATARECEIVEDACK {
			wi := workItem.(CSHandleDataReceivedAck)
			a.handleDataReceivedAck(wi, protocolHandler)

		} else if workItem.Type() == CANCEL {
			wi := workItem.(CSCancelAgreement)

			// Get the agreement id lock to prevent any other thread from processing this same agreement.
			lock := a.alm.getAgreementLock(wi.AgreementId)
			lock.Lock()

			a.cancelAgreement(wi.AgreementId, wi.Reason, protocolHandler, bc)

			lock.Unlock()

			// Don't need the agreement lock anymore
			a.alm.deleteAgreementLock(wi.AgreementId)

		} else if workItem.Type() == BC_RECORDED {
			// the agreement is recorded on the blockchain
			wi := workItem.(CSHandleBCRecorded)

			// Get the agreement id lock to prevent any other thread from processing this same agreement.
			lock := a.alm.getAgreementLock(wi.AgreementId)
			lock.Lock()

			if ag, err := FindSingleAgreementByAgreementId(a.db, wi.AgreementId, citizenscientist.PROTOCOL_NAME, []AFilter{UnarchivedAFilter()}); err != nil {
				glog.Errorf(logString(fmt.Sprintf("error querying agreement %v from database, error: %v", wi.AgreementId, err)))
			} else if ag == nil {
				glog.V(3).Infof(logString(fmt.Sprintf("nothing to do for agreement %v, no database record.", wi.AgreementId)))
			} else {
				// Update state in the database
				if _, err := AgreementFinalized(a.db, wi.AgreementId, citizenscientist.PROTOCOL_NAME); err != nil {
					glog.Errorf(logString(fmt.Sprintf("error persisting agreement %v finalized: %v", wi.AgreementId, err)))
				}

				// Update state in exchange
				if pol, err := policy.DemarshalPolicy(ag.Policy); err != nil {
					glog.Errorf(logString(fmt.Sprintf("error demarshalling policy from agreement %v, error: %v", wi.AgreementId, err)))
				} else if err := a.recordConsumerAgreementState(wi.AgreementId, pol.APISpecs[0].SpecRef, "Finalized Agreement"); err != nil {
					glog.Errorf(logString(fmt.Sprintf("error setting agreement %v finalized state in exchange: %v", wi.AgreementId, err)))
				}
			}

			// Drop the lock. The code above must always flow through this point.
			lock.Unlock()

		} else if workItem.Type() == BC_TERMINATED {
			// the agreement is terminated on the blockchain
			wi := workItem.(CSHandleBCTerminated)

			// Get the agreement id lock to prevent any other thread from processing this same agreement.
			lock := a.alm.getAgreementLock(wi.AgreementId)
			lock.Lock()

			a.cancelAgreement(wi.AgreementId, citizenscientist.AB_CANCEL_DISCOVERED, protocolHandler, bc)

			lock.Unlock()

			// Don't need the agreement lock anymore
			a.alm.deleteAgreementLock(wi.AgreementId)

		} else {
			glog.Errorf(logString(fmt.Sprintf("received unknown work request: %v", workItem)))
		}

		glog.V(5).Infof(logString(fmt.Sprintf("handled work: %v", workItem)))
		runtime.Gosched()

	}
}

func (a *CSAgreementWorker) initiateNewAgreement(wi             CSInitiateAgreement,
												random          *rand.Rand,
												protocolHandler *citizenscientist.ProtocolHandler,
												sendMessage     func(mt interface{}, pay []byte) error) {

	// Generate an agreement ID
	agreementId := generateAgreementId(random)
	myAddress, _ := ethblockchain.AccountId()
	agreementIdString := hex.EncodeToString(agreementId)
	glog.V(5).Infof(logString(fmt.Sprintf("using AgreementId %v", agreementIdString)))

	// Get the agreement id lock to prevent any other thread from processing this same agreement.
	lock := a.alm.getAgreementLock(agreementIdString)
	lock.Lock()
	defer lock.Unlock()

	// Determine which workload we should propose. This is based on the priority of each workload and
	// whether or not this workload has been tried before.
	var workload *policy.Workload
	if wlUsage, err := FindSingleWorkloadUsageByDeviceAndPolicyName(a.db, wi.Device.Id, wi.ConsumerPolicy.Header.Name); err != nil {
		glog.Errorf(logString(fmt.Sprintf("error searching for persistent workload usage records for device %v with policy %v, error: %v", wi.Device.Id, wi.ConsumerPolicy.Header.Name, err)))
		return
	} else if wlUsage == nil {
		workload = wi.ConsumerPolicy.NextHighestPriorityWorkload(0, 0, 0)
	} else if wlUsage.DisableRetry {
		workload = wi.ConsumerPolicy.NextHighestPriorityWorkload(wlUsage.Priority, 0, wlUsage.FirstTryTime)
	} else if wlUsage != nil {
		workload = wi.ConsumerPolicy.NextHighestPriorityWorkload(wlUsage.Priority, wlUsage.RetryCount+1, wlUsage.FirstTryTime)
	}

	// Create pending agreement in database
	if err := AgreementAttempt(a.db, agreementIdString, wi.Device.Id, wi.ConsumerPolicy.Header.Name, "Citizen Scientist"); err != nil {
		glog.Errorf(logString(fmt.Sprintf("error persisting agreement attempt: %v", err)))

	// Create message target for protocol message
	} else if mt, err := exchange.CreateMessageTarget(wi.Device.Id, nil, wi.Device.PublicKey, wi.Device.MsgEndPoint); err != nil {
		glog.Errorf(logString(fmt.Sprintf("error creating message target: %v", err)))

	// Initiate the protocol
	} else if proposal, hash, sig, err := protocolHandler.InitiateAgreement(agreementIdString, &wi.ProducerPolicy, &wi.ConsumerPolicy, myAddress, a.agbotId, mt, workload, a.config.AgreementBot.DefaultWorkloadPW, a.config.AgreementBot.NoDataIntervalS, sendMessage); err != nil {
		glog.Errorf(logString(fmt.Sprintf("error initiating agreement: %v", err)))

		// Remove pending agreement from database
		if err := DeleteAgreement(a.db, agreementIdString, citizenscientist.PROTOCOL_NAME); err != nil {
			glog.Errorf(logString(fmt.Sprintf("error deleting pending agreement: %v, error %v", agreementIdString, err)))
		}

		// TODO: Publish error on the message bus

	// Update the agreement in the DB with the proposal and policy
	} else if polBytes, err := json.Marshal(wi.ConsumerPolicy); err != nil {
		glog.Errorf(logString(fmt.Sprintf("error marshalling policy for storage %v, error: %v", wi.ConsumerPolicy, err)))
	} else if pBytes, err := json.Marshal(proposal); err != nil {
		glog.Errorf(logString(fmt.Sprintf("error marshalling proposal for storage %v, error: %v", *proposal, err)))
	} else if pol, err := policy.DemarshalPolicy(proposal.TsAndCs); err != nil {
		glog.Errorf(logString(fmt.Sprintf("error demarshalling TsandCs policy from pending agreement %v, error: %v", agreementIdString, err)))
	} else if _, err := AgreementUpdate(a.db, agreementIdString, string(pBytes), string(polBytes), pol.DataVerify, a.config.AgreementBot.ProcessGovernanceIntervalS, hash, sig, citizenscientist.PROTOCOL_NAME); err != nil {
		glog.Errorf(logString(fmt.Sprintf("error updating agreement with proposal %v in DB, error: %v", *proposal, err)))

	// Record that the agreement was initiated, in the exchange
	} else if err := a.recordConsumerAgreementState(agreementIdString, wi.ConsumerPolicy.APISpecs[0].SpecRef, "Formed Proposal"); err != nil {
		glog.Errorf(logString(fmt.Sprintf("error setting agreement state for %v", agreementIdString)))
	}
}

func (a *CSAgreementWorker) handleAgreementReply(wi             CSHandleReply,
												protocolHandler *citizenscientist.ProtocolHandler,
												bc              *ethblockchain.BaseContracts,
												sendMessage     func(mt interface{}, pay []byte) error) {

	// The reply message is usually deleted before recording on the blockchain. For now assume it will be deleted at the end. Early exit from
	// this function is NOT allowed.
	deletedMessage := false

	if reply, err := protocolHandler.ValidateReply(wi.Reply); err != nil {
		glog.Warningf(logString(fmt.Sprintf("discarding message: %v", wi.Reply)))
	} else {

		// Get the agreement id lock to prevent any other thread from processing this same agreement.
		lock := a.alm.getAgreementLock(reply.AgreementId())
		lock.Lock()

		// The lock is dropped at the end of this function or right before the blockchain write. Early exit from this function is NOT allowed.
		droppedLock := false

		if reply.ProposalAccepted() {
			// The producer is happy with the proposal. Assume we will ack negatively unless we find out that everything is ok.
			ackReplyAsValid := false

			// Find the saved agreement in the database
			if agreement, err := FindSingleAgreementByAgreementId(a.db, reply.AgreementId(), citizenscientist.PROTOCOL_NAME, []AFilter{UnarchivedAFilter()}); err != nil {
				glog.Errorf(logString(fmt.Sprintf("error querying pending agreement %v, error: %v", reply.AgreementId(), err)))
			} else if agreement == nil {
				glog.V(5).Infof(logString(fmt.Sprintf("discarding reply, agreement id %v not in our database", reply.AgreementId())))
			} else if agreement.CounterPartyAddress != "" {
				glog.V(5).Infof(logString(fmt.Sprintf("discarding reply, agreement id %v already received a reply", agreement.CurrentAgreementId)))
				// this will cause us to not send a reply ack, which is what we want in this case
				ackReplyAsValid = true

			// Now we need to write the info to the exchange and the database
			} else if proposal, err := protocolHandler.DemarshalProposal(agreement.Proposal); err != nil {
				glog.Errorf(logString(fmt.Sprintf("error validating proposal from pending agreement %v, error: %v", reply.AgreementId(), err)))
			} else if pol, err := policy.DemarshalPolicy(proposal.TsAndCs); err != nil {
				glog.Errorf(logString(fmt.Sprintf("error demarshalling tsandcs policy from pending agreement %v, error: %v", reply.AgreementId(), err)))

			} else if _, err := AgreementMade(a.db, reply.AgreementId(), reply.Address, reply.Signature, citizenscientist.PROTOCOL_NAME, pol.HAGroup.Partners); err != nil {
				glog.Errorf(logString(fmt.Sprintf("error updating agreement with proposal %v in DB, error: %v", *proposal, err)))

			} else if err := a.recordConsumerAgreementState(reply.AgreementId(), pol.APISpecs[0].SpecRef, "Producer agreed"); err != nil {
				glog.Errorf(logString(fmt.Sprintf("error setting agreement state for %v", reply.AgreementId())))

				// We need to send a reply ack and write the info to the blockchain
			} else if consumerPolicy, err := policy.DemarshalPolicy(agreement.Policy); err != nil {
				glog.Errorf(logString(fmt.Sprintf("unable to demarshal policy for agreement %v, error %v", reply.AgreementId(), err)))
			} else {
				// Done handling the response successfully
				ackReplyAsValid = true

				// If we dont have a workload usage record for this device, then we need to create one. If there is already a
				// workload usage record and workload rollback retry counting is enabled, then check to see if the workload priority
				// has changed. If so, update the record and reset the retry count and time. Othwerwise just update the retry count.
				if wlUsage, err := FindSingleWorkloadUsageByDeviceAndPolicyName(a.db, wi.SenderId, consumerPolicy.Header.Name); err != nil {
					glog.Errorf(logString(fmt.Sprintf("error searching for persistent workload usage records for device %v with policy %v, error: %v", wi.SenderId, consumerPolicy.Header.Name, err)))
				} else if wlUsage == nil {
					if !pol.Workloads[0].HasEmptyPriority() {
						if err := NewWorkloadUsage(a.db, wi.SenderId, pol.HAGroup.Partners, agreement.Policy, consumerPolicy.Header.Name, pol.Workloads[0].Priority.PriorityValue, pol.Workloads[0].Priority.RetryDurationS, pol.Workloads[0].Priority.VerifiedDurationS, reply.AgreementId()); err != nil {
							glog.Errorf(logString(fmt.Sprintf("error creating persistent workload usage records for device %v with policy %v, error: %v", wi.SenderId, consumerPolicy.Header.Name, err)))
						}
					}
				} else if !wlUsage.DisableRetry {
					if pol.Workloads[0].Priority.PriorityValue != wlUsage.Priority {
						if _, err := UpdatePriority(a.db, wi.SenderId, consumerPolicy.Header.Name, pol.Workloads[0].Priority.PriorityValue, pol.Workloads[0].Priority.RetryDurationS, pol.Workloads[0].Priority.VerifiedDurationS, reply.AgreementId()); err != nil {
							glog.Errorf(logString(fmt.Sprintf("error updating workload usage prioroty for device %v with policy %v, error: %v", wi.SenderId, consumerPolicy.Header.Name, err)))
						}
					} else if _, err := UpdateRetryCount(a.db, wi.SenderId, consumerPolicy.Header.Name, wlUsage.RetryCount+1, reply.AgreementId()); err != nil {
						glog.Errorf(logString(fmt.Sprintf("error updating workload usage retry count for device %v with policy %v, error: %v", wi.SenderId, consumerPolicy.Header.Name, err)))
					}
				} else if _, err := UpdateWUAgreementId(a.db, wi.SenderId, consumerPolicy.Header.Name, reply.AgreementId()); err != nil {
					glog.Errorf(logString(fmt.Sprintf("error updating agreement id %v in workload usage for %v for policy %v, error: %v", reply.AgreementId(), wi.SenderId, consumerPolicy.Header.Name, err)))
				}

				// Send the reply Ack
				if mt, err := exchange.CreateMessageTarget(wi.SenderId, nil, wi.SenderPubKey, wi.From); err != nil {
					glog.Errorf(logString(fmt.Sprintf("error creating message target: %v", err)))
				} else if err := protocolHandler.Confirm(ackReplyAsValid, reply.AgreementId(), mt, sendMessage); err != nil {
					glog.Errorf(logString(fmt.Sprintf("error trying to send reply ack for %v to %v, error: %v", reply.AgreementId(), mt, err)))
				}

				// Delete the original reply message
				if wi.MessageId != 0 {
					if err := a.deleteMessage(wi.MessageId); err != nil {
						glog.Errorf(logString(fmt.Sprintf("error deleting message %v from exchange for agbot %v", wi.MessageId, a.agbotId)))
					}
				}
				deletedMessage = true
				droppedLock = true
				lock.Unlock()

				// Recording the agreement on the blockchain could take a long time, so it needs to be the last thing we do.
				if err := protocolHandler.RecordAgreement(proposal, reply, consumerPolicy, bc.Agreements); err != nil {
					glog.Errorf(logString(fmt.Sprintf("error trying to record agreement in blockchain, %v", err)))
				}

				glog.V(3).Infof(logString(fmt.Sprintf("recorded agreement %v", reply.AgreementId())))
			}

			// Always send an ack for a reply with a positive decision in it
			if !ackReplyAsValid {
				if mt, err := exchange.CreateMessageTarget(wi.SenderId, nil, wi.SenderPubKey, wi.From); err != nil {
					glog.Errorf(logString(fmt.Sprintf("error creating message target: %v", err)))
				} else if err := protocolHandler.Confirm(ackReplyAsValid, reply.AgreementId(), mt, sendMessage); err != nil {
					glog.Errorf(logString(fmt.Sprintf("error trying to send reply ack for %v to %v, error: %v", reply.AgreementId(), wi.From, err)))
				}
			}

		} else {
			glog.Errorf(logString(fmt.Sprintf("received rejection from producer %v", *reply)))

			a.cancelAgreement(reply.AgreementId(), citizenscientist.AB_CANCEL_NEGATIVE_REPLY, protocolHandler, bc)
		}

		// Get rid of the lock
		if !droppedLock {
			lock.Unlock()
		}

	}

	// Get rid of the exchange message if there is one
	if wi.MessageId != 0 && !deletedMessage {
		if err := a.deleteMessage(wi.MessageId); err != nil {
			glog.Errorf(logString(fmt.Sprintf("error deleting message %v from exchange for agbot %v", wi.MessageId, a.agbotId)))
		}
	}

}

func (a *CSAgreementWorker) handleDataReceivedAck(wi            CSHandleDataReceivedAck,
												protocolHandler *citizenscientist.ProtocolHandler) {

	if drAck, err := protocolHandler.ValidateDataReceivedAck(wi.Ack); err != nil {
		glog.Warningf(logString(fmt.Sprintf("discarding message: %v", wi.Ack)))
	} else {

		// Get the agreement id lock to prevent any other thread from processing this same agreement.
		lock := a.alm.getAgreementLock(drAck.AgreementId())
		lock.Lock()

		if ag, err := FindSingleAgreementByAgreementId(a.db, drAck.AgreementId(), citizenscientist.PROTOCOL_NAME, []AFilter{UnarchivedAFilter()}); err != nil {
			glog.Errorf(logString(fmt.Sprintf("error querying timed out agreement %v, error: %v", drAck.AgreementId(), err)))
		} else if ag == nil {
			glog.V(3).Infof(logString(fmt.Sprintf("nothing to terminate for agreement %v, no database record.", drAck.AgreementId())))
		} else if _, err := DataNotification(a.db, ag.CurrentAgreementId, citizenscientist.PROTOCOL_NAME); err != nil {
			glog.Errorf(logString(fmt.Sprintf("unable to record data notification, error: %v", err)))
		}

		// Drop the lock. The code block above must always flow through this point.
		lock.Unlock()

	}

	// Get rid of the exchange message if there is one
	if wi.MessageId != 0 {
		if err := a.deleteMessage(wi.MessageId); err != nil {
			glog.Errorf(logString(fmt.Sprintf("error deleting message %v from exchange for agbot %v", wi.MessageId, a.agbotId)))
		}
	}

}

// This function is used to generate a random 32 byte bitstring that is used as the agreement id.
func generateAgreementId(random *rand.Rand) []byte {

	b := make([]byte, 32, 32)
	for i := range b {
		b[i] = byte(random.Intn(256))
	}
	return b
}

func (a *CSAgreementWorker) recordConsumerAgreementState(agreementId string, workloadID string, state string) error {

	glog.V(5).Infof("CSAgreementWorker setting agreement %v state to %v", agreementId, state)

	as := new(exchange.PutAgbotAgreementState)
	as.Workload = workloadID
	as.State = state
	var resp interface{}
	resp = new(exchange.PostDeviceResponse)
	targetURL := a.config.AgreementBot.ExchangeURL + "agbots/" + a.agbotId + "/agreements/" + agreementId
	for {
		if err, tpErr := exchange.InvokeExchange(a.httpClient, "PUT", targetURL, a.agbotId, a.token, &as, &resp); err != nil {
			glog.Errorf(err.Error())
			return err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(5).Infof("CSAgreementWorker set agreement %v to state %v", agreementId, state)
			return nil
		}
	}

}

func (a *CSAgreementWorker) deleteMessage(msgId int) error {
	var resp interface{}
	resp = new(exchange.PostDeviceResponse)
	targetURL := a.config.AgreementBot.ExchangeURL + "agbots/" + a.agbotId + "/msgs/" + strconv.Itoa(msgId)
	for {
		if err, tpErr := exchange.InvokeExchange(a.httpClient, "DELETE", targetURL, a.agbotId, a.token, nil, &resp); err != nil {
			glog.Errorf(err.Error())
			return err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(3).Infof("CSAgreementWorker: deleted message %v", msgId)
			return nil
		}
	}
}

func (a *CSAgreementWorker) cancelAgreement(agreementId string, reason uint, protocolHandler *citizenscientist.ProtocolHandler, bc *ethblockchain.BaseContracts) {
	// Start timing out the agreement
	glog.V(3).Infof("CSAgreementWorker: terminating agreement %v.", agreementId)

	// Update the database
	if _, err := AgreementTimedout(a.db, agreementId, citizenscientist.PROTOCOL_NAME); err != nil {
		glog.Errorf("CSAgreementWorker: error marking agreement %v terminated: %v", agreementId, err)
	}

	// Update state in exchange
	if err := DeleteConsumerAgreement(a.config.AgreementBot.ExchangeURL, a.agbotId, a.token, agreementId); err != nil {
		glog.Errorf("CSAgreementWorker: error deleting agreement %v in exchange: %v", agreementId, err)
	}

	// Find the agreement record
	if ag, err := FindSingleAgreementByAgreementId(a.db, agreementId, citizenscientist.PROTOCOL_NAME, []AFilter{UnarchivedAFilter()}); err != nil {
		glog.Errorf("CSAgreementWorker: error querying agreement %v from database, error: %v", agreementId, err)
	} else if ag == nil {
		glog.V(3).Infof("CSAgreementWorker: nothing to terminate for agreement %v, no database record.", agreementId)
	} else {

		// Update the workload usage record to clear the agreement. There might not be a workload usage record if there is no workload priority
		// specified in the workload section of the policy.
		if _, err := UpdateWUAgreementId(a.db, ag.DeviceId, ag.PolicyName, ""); err != nil {
			glog.Warningf("CSAgreementWorker: error updating agreement id in workload usage for %v for policy %v, error: %v", ag.DeviceId, ag.PolicyName, err)
		}

		// Remove the long blockchain cancel from the worker thread
		go func() {
			// Remove from the blockchain
			if pol, err := policy.DemarshalPolicy(ag.Policy); err != nil {
				glog.Errorf("CSAgreementWorker: unable to demarshal policy while trying to cancel %v, error %v", agreementId, err)
			} else if err := protocolHandler.TerminateAgreement(pol, ag.CounterPartyAddress, agreementId, reason, bc.Agreements); err != nil {
				glog.Errorf("CSAgreementWorker: error terminating agreement %v on the blockchain: %v", agreementId, err)
			}
		}()

		// Archive the record
		if _, err := ArchiveAgreement(a.db, agreementId, citizenscientist.PROTOCOL_NAME, reason, citizenscientist.DecodeReasonCode(uint64(reason))); err != nil {
			glog.Errorf("CSAgreementWorker: error archiving terminated agreement: %v, error: %v", agreementId, err)
		}
	}
}
