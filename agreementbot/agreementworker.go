package agreementbot

import (
	"encoding/hex"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/abstractprotocol"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/policy"
	"math/rand"
	"net/http"
	// "time"
)

// These structs are the event bodies that flows from the processor to the agreement workers
const INITIATE = "INITIATE_AGREEMENT"
const REPLY = "AGREEMENT_REPLY"
const CANCEL = "AGREEMENT_CANCEL"
const DATARECEIVEDACK = "AGREEMENT_DATARECEIVED_ACK"
const WORKLOAD_UPGRADE = "WORKLOAD_UPGRADE"

type AgreementWork interface {
	Type() string
}

type InitiateAgreement struct {
	workType       string
	ProducerPolicy policy.Policy               // the producer policy received from the exchange
	ConsumerPolicy policy.Policy               // the consumer policy we're matched up with
	Device         exchange.SearchResultDevice // the device entry in the exchange
}

func (c InitiateAgreement) String() string {
	res := ""
	res += fmt.Sprintf("Workitem: %v\n", c.workType)
	res += fmt.Sprintf("Producer Policy: %v\n", c.ProducerPolicy)
	res += fmt.Sprintf("Consumer Policy: %v\n", c.ConsumerPolicy)
	res += fmt.Sprintf("Device: %v", c.Device)
	return res
}

func (c InitiateAgreement) Type() string {
	return c.workType
}

type HandleReply struct {
	workType     string
	Reply        abstractprotocol.ProposalReply
	From         string // deprecated whisper address
	SenderId     string // exchange Id of sender
	SenderPubKey []byte
	MessageId    int
}

func (c HandleReply) String() string {
	return fmt.Sprintf("Workitem: %v, SenderId: %v, MessageId: %v, From: %v, Reply: %v, SenderPubKey: %x", c.workType, c.SenderId, c.MessageId, c.From, c.Reply, c.SenderPubKey)
}

func (c HandleReply) Type() string {
	return c.workType
}

type HandleDataReceivedAck struct {
	workType     string
	Ack          string
	From         string // deprecated whisper address
	SenderId     string // exchange Id of sender
	SenderPubKey []byte
	MessageId    int
}

func (c HandleDataReceivedAck) String() string {
	return fmt.Sprintf("Workitem: %v, SenderId: %v, MessageId: %v, From: %v, Ack: %v, SenderPubKey: %x", c.workType, c.SenderId, c.MessageId, c.From, c.Ack, c.SenderPubKey)
}

func (c HandleDataReceivedAck) Type() string {
	return c.workType
}

type CancelAgreement struct {
	workType    string
	AgreementId string
	Protocol    string
	Reason      uint
}

func (c CancelAgreement) Type() string {
	return c.workType
}

type HandleWorkloadUpgrade struct {
	workType    string
	AgreementId string
	Protocol    string
	Device      string
	PolicyName  string
}

func (c HandleWorkloadUpgrade) Type() string {
	return c.workType
}

type AgreementWorker interface {
	AgreementLockManager() *AgreementLockManager
}

type BaseAgreementWorker struct {
	pm         *policy.PolicyManager
	db         *bolt.DB
	config     *config.HorizonConfig
	alm        *AgreementLockManager
	workerID   string
	httpClient *http.Client
}

func (b *BaseAgreementWorker) AgreementLockManager() *AgreementLockManager {
	return b.alm
}

func (b *BaseAgreementWorker) InitiateNewAgreement(cph ConsumerProtocolHandler, wi *InitiateAgreement, random *rand.Rand, workerId string) {

	// Generate an agreement ID
	agreementId := generateAgreementId(random)
	agreementIdString := hex.EncodeToString(agreementId)
	glog.V(5).Infof(BAWlogstring(workerId, fmt.Sprintf("using AgreementId %v", agreementIdString)))

	protocolHandler := cph.AgreementProtocolHandler()

	// Get the agreement id lock to prevent any other thread from processing this same agreement.
	lock := b.alm.getAgreementLock(agreementIdString)
	lock.Lock()
	defer lock.Unlock()

	// Determine which workload we should propose. This is based on the priority of each workload and
	// whether or not this workload has been tried before.

	var workload *policy.Workload
	if wlUsage, err := FindSingleWorkloadUsageByDeviceAndPolicyName(b.db, wi.Device.Id, wi.ConsumerPolicy.Header.Name); err != nil {
		glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error searching for persistent workload usage records for device %v with policy %v, error: %v", wi.Device.Id, wi.ConsumerPolicy.Header.Name, err)))
		return
	} else if wlUsage == nil {
		workload = wi.ConsumerPolicy.NextHighestPriorityWorkload(0, 0, 0)
	} else if wlUsage.DisableRetry {
		workload = wi.ConsumerPolicy.NextHighestPriorityWorkload(wlUsage.Priority, 0, wlUsage.FirstTryTime)
	} else if wlUsage != nil {
		workload = wi.ConsumerPolicy.NextHighestPriorityWorkload(wlUsage.Priority, wlUsage.RetryCount+1, wlUsage.FirstTryTime)
	}

	// Create pending agreement in database
	if err := AgreementAttempt(b.db, agreementIdString, wi.Device.Id, wi.ConsumerPolicy.Header.Name, cph.Name()); err != nil {
		glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error persisting agreement attempt: %v", err)))

	// Create message target for protocol message
	} else if mt, err := exchange.CreateMessageTarget(wi.Device.Id, nil, wi.Device.PublicKey, wi.Device.MsgEndPoint); err != nil {
		glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error creating message target: %v", err)))

	// Initiate the protocol
	} else if proposal, err := protocolHandler.InitiateAgreement(agreementIdString, &wi.ProducerPolicy, &wi.ConsumerPolicy, cph.ExchangeId(), mt, workload, b.config.AgreementBot.DefaultWorkloadPW, b.config.AgreementBot.NoDataIntervalS, cph.GetSendMessage()); err != nil {
		glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error initiating agreement: %v", err)))

		// Remove pending agreement from database
		if err := DeleteAgreement(b.db, agreementIdString, cph.Name()); err != nil {
			glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error deleting pending agreement: %v, error %v", agreementIdString, err)))
		}

		// TODO: Publish error on the message bus

	// Update the agreement in the DB with the proposal and policy
	} else if err := cph.PersistAgreement(wi, proposal, workerId); err != nil {
		glog.Errorf(err.Error())
	}

}

func (b *BaseAgreementWorker) HandleAgreementReply(cph ConsumerProtocolHandler, wi *HandleReply, workerId string) bool {

	reply := wi.Reply
	protocolHandler := cph.AgreementProtocolHandler()

	// The reply message is usually deleted before recording on the blockchain. For now assume it will be deleted at the end. Early exit from
	// this function is NOT allowed.
	deletedMessage := false

	// Get the agreement id lock to prevent any other thread from processing this same agreement.
	lock := b.alm.getAgreementLock(wi.Reply.AgreementId())
	lock.Lock()

	// The lock is dropped at the end of this function or right before the blockchain write. Early exit from this function is NOT allowed.
	droppedLock := false

	// Assume we will ack negatively unless we find out that everything is ok.
	ackReplyAsValid := false
	sendReply := true

	if reply.ProposalAccepted() {

		// Find the saved agreement in the database
		if agreement, err := FindSingleAgreementByAgreementId(b.db, reply.AgreementId(), cph.Name(), []AFilter{UnarchivedAFilter()}); err != nil {
			glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error querying pending agreement %v, error: %v", reply.AgreementId(), err)))
		} else if agreement == nil {
			glog.V(5).Infof(BAWlogstring(workerId, fmt.Sprintf("discarding reply, agreement id %v not in our database", reply.AgreementId())))
		} else if agreement.CounterPartyAddress != "" {
			glog.V(5).Infof(BAWlogstring(workerId, fmt.Sprintf("discarding reply, agreement id %v already received a reply", agreement.CurrentAgreementId)))
			// this will cause us to not send a reply ack, which is what we want in this case
			sendReply = false

		// Now we need to write the info to the exchange and the database
		} else if proposal, err := protocolHandler.DemarshalProposal(agreement.Proposal); err != nil {
			glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error validating proposal from pending agreement %v, error: %v", reply.AgreementId(), err)))
		} else if pol, err := policy.DemarshalPolicy(proposal.TsAndCs()); err != nil {
			glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error demarshalling tsandcs policy from pending agreement %v, error: %v", reply.AgreementId(), err)))

		} else if err := cph.PersistReply(reply, pol, workerId); err != nil {
			glog.Errorf(err.Error())

		} else if err := cph.RecordConsumerAgreementState(reply.AgreementId(), pol.APISpecs[0].SpecRef, "Producer agreed", b.workerID); err != nil {
			glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error setting agreement state for %v", reply.AgreementId())))

		// We need to send a reply ack and write the info to the blockchain
		} else if consumerPolicy, err := policy.DemarshalPolicy(agreement.Policy); err != nil {
			glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("unable to demarshal policy for agreement %v, error %v", reply.AgreementId(), err)))
		} else {
			// Done handling the response successfully
			ackReplyAsValid = true

			// If we dont have a workload usage record for this device, then we need to create one. If there is already a
			// workload usage record and workload rollback retry counting is enabled, then check to see if the workload priority
			// has changed. If so, update the record and reset the retry count and time. Othwerwise just update the retry count.
			if wlUsage, err := FindSingleWorkloadUsageByDeviceAndPolicyName(b.db, wi.SenderId, consumerPolicy.Header.Name); err != nil {
				glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error searching for persistent workload usage records for device %v with policy %v, error: %v", wi.SenderId, consumerPolicy.Header.Name, err)))
			} else if wlUsage == nil {
				// There is no workload usage record. Make sure that the current workload chosen is the highest priority workload.
				// There could have been a change in the system such that the chosen workload is no longer the right choice. If this
				// is the case, then we need to reject the agreement and start over.

				workload := consumerPolicy.NextHighestPriorityWorkload(0, 0, 0)
				if !workload.Priority.IsSame(pol.Workloads[0].Priority) {
					// Need a new workload usage record but not the same as the highest priority. That can't be right.
					ackReplyAsValid = false
				} else if !pol.Workloads[0].HasEmptyPriority() {
					if err := NewWorkloadUsage(b.db, wi.SenderId, pol.HAGroup.Partners, agreement.Policy, consumerPolicy.Header.Name, pol.Workloads[0].Priority.PriorityValue, pol.Workloads[0].Priority.RetryDurationS, pol.Workloads[0].Priority.VerifiedDurationS, reply.AgreementId()); err != nil {
						glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error creating persistent workload usage records for device %v with policy %v, error: %v", wi.SenderId, consumerPolicy.Header.Name, err)))
					}
				}
			} else if !wlUsage.DisableRetry {
				if pol.Workloads[0].Priority.PriorityValue != wlUsage.Priority {
					if _, err := UpdatePriority(b.db, wi.SenderId, consumerPolicy.Header.Name, pol.Workloads[0].Priority.PriorityValue, pol.Workloads[0].Priority.RetryDurationS, pol.Workloads[0].Priority.VerifiedDurationS, reply.AgreementId()); err != nil {
						glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error updating workload usage prioroty for device %v with policy %v, error: %v", wi.SenderId, consumerPolicy.Header.Name, err)))
					}
				} else if _, err := UpdateRetryCount(b.db, wi.SenderId, consumerPolicy.Header.Name, wlUsage.RetryCount+1, reply.AgreementId()); err != nil {
					glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error updating workload usage retry count for device %v with policy %v, error: %v", wi.SenderId, consumerPolicy.Header.Name, err)))
				}
			} else if _, err := UpdateWUAgreementId(b.db, wi.SenderId, consumerPolicy.Header.Name, reply.AgreementId()); err != nil {
				glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error updating agreement id %v in workload usage for %v for policy %v, error: %v", reply.AgreementId(), wi.SenderId, consumerPolicy.Header.Name, err)))
			}

			// Send the reply Ack
			if mt, err := exchange.CreateMessageTarget(wi.SenderId, nil, wi.SenderPubKey, wi.From); err != nil {
				glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error creating message target: %v", err)))
			} else if err := protocolHandler.Confirm(ackReplyAsValid, reply.AgreementId(), mt, cph.GetSendMessage()); err != nil {
				glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error trying to send reply ack for %v to %v, error: %v", reply.AgreementId(), mt, err)))
			}

			// Delete the original reply message
			if wi.MessageId != 0 {
				if err := cph.DeleteMessage(wi.MessageId); err != nil {
					glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error deleting message %v from exchange for agbot %v", wi.MessageId, cph.ExchangeId())))
				}
			}

			deletedMessage = true
			droppedLock = true
			lock.Unlock()

			// Recording the agreement on the blockchain could take a long time, so it needs to be the last thing we do.
			if err := protocolHandler.RecordAgreement(proposal, reply, consumerPolicy); err != nil {
				glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error trying to record agreement in blockchain, %v", err)))
				b.CancelAgreementWithLock(cph, reply.AgreementId(), cph.GetTerminationCode(TERM_REASON_CANCEL_BC_WRITE_FAILED), workerId)
				ackReplyAsValid = false
			} else {
				glog.V(3).Infof(BAWlogstring(workerId, fmt.Sprintf("recorded agreement %v", reply.AgreementId())))
			}
		}

		// Always send an ack for a reply with a positive decision in it
		if !ackReplyAsValid && sendReply {
			if mt, err := exchange.CreateMessageTarget(wi.SenderId, nil, wi.SenderPubKey, wi.From); err != nil {
				glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error creating message target: %v", err)))
			} else if err := protocolHandler.Confirm(ackReplyAsValid, reply.AgreementId(), mt, cph.GetSendMessage()); err != nil {
				glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error trying to send reply ack for %v to %v, error: %v", reply.AgreementId(), wi.From, err)))
			}
		}

	} else {
		glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("received rejection from producer %v", reply)))

		b.CancelAgreement(cph, reply.AgreementId(), cph.GetTerminationCode(TERM_REASON_NEGATIVE_REPLY), workerId)
	}

	// Get rid of the lock
	if !droppedLock {
		lock.Unlock()
	}

	// Get rid of the exchange message if there is one
	if wi.MessageId != 0 && !deletedMessage {
		if err := cph.DeleteMessage(wi.MessageId); err != nil {
			glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error deleting message %v from exchange for agbot %v", wi.MessageId, cph.ExchangeId())))
		}
	}

	return ackReplyAsValid

}

func (b *BaseAgreementWorker) HandleDataReceivedAck(cph ConsumerProtocolHandler, wi *HandleDataReceivedAck, workerId string) {

	protocolHandler := cph.AgreementProtocolHandler()

	if d, err := protocolHandler.ValidateDataReceivedAck(wi.Ack); err != nil {
		glog.Warningf(BAWlogstring(workerId, fmt.Sprintf("discarding message: %v", wi.Ack)))
	} else if drAck, ok := d.(*abstractprotocol.BaseDataReceivedAck); !ok {
		glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("unable to cast Data Received Ack %v to %v Proposal Reply, is %T", d, cph.Name(), d)))
	} else {

		// Get the agreement id lock to prevent any other thread from processing this same agreement.
		lock := b.alm.getAgreementLock(drAck.AgreementId())
		lock.Lock()

		if ag, err := FindSingleAgreementByAgreementId(b.db, drAck.AgreementId(), cph.Name(), []AFilter{UnarchivedAFilter()}); err != nil {
			glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error querying timed out agreement %v, error: %v", drAck.AgreementId(), err)))
		} else if ag == nil {
			glog.V(3).Infof(BAWlogstring(workerId, fmt.Sprintf("nothing to terminate for agreement %v, no database record.", drAck.AgreementId())))
		} else if _, err := DataNotification(b.db, ag.CurrentAgreementId, cph.Name()); err != nil {
			glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("unable to record data notification, error: %v", err)))
		}

		// Drop the lock. The code block above must always flow through this point.
		lock.Unlock()

	}

	// Get rid of the exchange message if there is one
	if wi.MessageId != 0 {
		if err := cph.DeleteMessage(wi.MessageId); err != nil {
			glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error deleting message %v from exchange for agbot %v", wi.MessageId, cph.ExchangeId())))
		}
	}

}

func (b *BaseAgreementWorker) HandleWorkloadUpgrade(cph ConsumerProtocolHandler, wi *HandleWorkloadUpgrade, workerId string) {

	// Force an upgrade of a workload on a specific device, given a specific policy that delivered the workload.
	// The upgrade request will contain a specific device and policy name, but it might not contain an agreement
	// id. At this point we assume that the originator of the workload upgrade event validated that the agreement id
	// (if specified) matches the device and policy name. Further, the caller has also validated that the device does
	// (or did) have a workload running from the specified policy name.

	// If there is no agreement id specified then find one for the current device and policy name. If we find one,
	// grab the agreement id lock, cancel the agreement and delete the workload usage record.

	if wi.AgreementId == "" {
		if ags, err := FindAgreements(b.db, []AFilter{DevPolAFilter(wi.Device, wi.PolicyName)}, cph.Name()); err != nil {
			glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error finding agreement for device %v and policyName %v, error: %v", wi.Device, wi.PolicyName, err)))
		} else if len(ags) == 0 {
			// If there is no agreement found, is it a problem? We could have caught the system in a state where there is no
			// agreement, but there still might be a workload usage record for the device and policy name. It should be safe to
			// just delete the workload usage record. When an agreement reply is processed, the code will verify that the
			// highest priority workload is being used when creating a new workload usage record.
			glog.V(5).Infof(BAWlogstring(workerId, fmt.Sprintf("forced workload upgrade found no current agreement for device %v and policy name %v", wi.Device, wi.PolicyName)))
		} else {
			// Cancel all agreements
			for _, ag := range ags {
				// Terminate the agreement
				b.CancelAgreementWithLock(cph, ag.CurrentAgreementId, cph.GetTerminationCode(TERM_REASON_CANCEL_FORCED_UPGRADE), workerId)
			}
		}
	} else {
		// Terminate the agreement
		b.CancelAgreementWithLock(cph, wi.AgreementId, cph.GetTerminationCode(TERM_REASON_CANCEL_FORCED_UPGRADE), workerId)
	}

	// Find the workload usage record and delete it. This will cause any new agreement negotiations to start with the highest priority
	// workload.
	if err := DeleteWorkloadUsage(b.db, wi.Device, wi.PolicyName); err != nil {
		glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error deleting workload usage record for device %v and policyName %v, error: %v", wi.Device, wi.PolicyName, err)))
	}

}

func (b *BaseAgreementWorker) CancelAgreementWithLock(cph ConsumerProtocolHandler, agreementId string, reason uint, workerId string) {
	// Get the agreement id lock to prevent any other thread from processing this same agreement.
	lock := b.AgreementLockManager().getAgreementLock(agreementId)
	lock.Lock()

	// Terminate the agreement
	b.CancelAgreement(cph, agreementId, reason, workerId)

	lock.Unlock()

	// Don't need the agreement lock anymore
	b.AgreementLockManager().deleteAgreementLock(agreementId)
}

func (b *BaseAgreementWorker) CancelAgreement(cph ConsumerProtocolHandler, agreementId string, reason uint, workerId string) {

	// Start timing out the agreement
	glog.V(3).Infof(BAWlogstring(workerId, fmt.Sprintf("terminating agreement %v.", agreementId)))

	// Update the database
	if _, err := AgreementTimedout(b.db, agreementId, cph.Name()); err != nil {
		glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error marking agreement %v terminated: %v", agreementId, err)))
	}

	// Update state in exchange
	if err := DeleteConsumerAgreement(b.config.AgreementBot.ExchangeURL, cph.ExchangeId(), cph.ExchangeToken(), agreementId); err != nil {
		glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error deleting agreement %v in exchange: %v", agreementId, err)))
	}

	// Find the agreement record
	if ag, err := FindSingleAgreementByAgreementId(b.db, agreementId, cph.Name(), []AFilter{UnarchivedAFilter()}); err != nil {
		glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error querying agreement %v from database, error: %v", agreementId, err)))
	} else if ag == nil {
		glog.V(3).Infof(BAWlogstring(workerId, fmt.Sprintf("nothing to terminate for agreement %v, no database record.", agreementId)))
	} else {

		// Update the workload usage record to clear the agreement. There might not be a workload usage record if there is no workload priority
		// specified in the workload section of the policy.
		if _, err := UpdateWUAgreementId(b.db, ag.DeviceId, ag.PolicyName, ""); err != nil {
			glog.Warningf(BAWlogstring(workerId, fmt.Sprintf("error updating agreement id in workload usage for %v for policy %v, error: %v", ag.DeviceId, ag.PolicyName, err)))
		}

		// Remove the long blockchain cancel from the worker thread. It is important to give the protocol handler a chance to
		// do whatever cleanup and termination it needs to do so we should never skip calling this function.
		go b.DoBlockchainCancel(cph, ag, reason, workerId)

		// Archive the record
		if _, err := ArchiveAgreement(b.db, agreementId, cph.Name(), reason, cph.GetTerminationReason(reason)); err != nil {
			glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error archiving terminated agreement: %v, error: %v", agreementId, err)))
		}
	}
}

func (b *BaseAgreementWorker) DoBlockchainCancel(cph ConsumerProtocolHandler, ag *Agreement, reason uint, workerId string) {

	cph.TerminateAgreement(ag, reason, workerId)
}

func generateAgreementId(random *rand.Rand) []byte {

	b := make([]byte, 32, 32)
	for i := range b {
		b[i] = byte(random.Intn(256))
	}
	return b
}

var BAWlogstring = func(workerID string, v interface{}) string {
	return fmt.Sprintf("Base Agreement Worker (%v): %v", workerID, v)
}
