package agreementbot

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/agreementbot/persistence"
	"github.com/open-horizon/anax/agreementbot/secrets"
	"github.com/open-horizon/anax/basicprotocol"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/worker"
	"github.com/satori/go.uuid"
	"math/rand"
	"runtime"
)

type BasicAgreementWorker struct {
	*BaseAgreementWorker
	protocolHandler *BasicProtocolHandler
}

func NewBasicAgreementWorker(c *BasicProtocolHandler, cfg *config.HorizonConfig, db persistence.AgbotDatabase, pm *policy.PolicyManager, alm *AgreementLockManager, mmsObjMgr *MMSObjectPolicyManager, secretsMgr secrets.AgbotSecrets) *BasicAgreementWorker {

	id, err := uuid.NewV4()
	if err != nil {
		panic(fmt.Sprintf("Error generating worker UUID: %v", err))
	}

	p := &BasicAgreementWorker{
		BaseAgreementWorker: &BaseAgreementWorker{
			pm:         pm,
			db:         db,
			config:     cfg,
			alm:        alm,
			workerID:   id.String(),
			httpClient: cfg.Collaborators.HTTPClientFactory.NewHTTPClient(nil),
			ec:         worker.NewExchangeContext(cfg.AgreementBot.ExchangeId, cfg.AgreementBot.ExchangeToken, cfg.AgreementBot.ExchangeURL, cfg.GetAgbotCSSURL(), cfg.Collaborators.HTTPClientFactory),
			mmsObjMgr:  mmsObjMgr,
			secretsMgr: secretsMgr,
		},
		protocolHandler: c,
	}

	return p
}

// These are work items that represent extensions to the protocol.
const AGREEMENT_VERIFICATION = "AGREEMENT_VERIFY"

type BAgreementVerification struct {
	workType     string
	Verify       basicprotocol.BAgreementVerify
	From         string // deprecated whisper address
	SenderId     string // exchange Id of sender
	SenderPubKey []byte
	MessageId    int
}

func NewBAgreementVerification(verify *basicprotocol.BAgreementVerify, from string, senderPubKey []byte, messageId int) AgreementWork {
	return BAgreementVerification{
		workType:     AGREEMENT_VERIFICATION,
		Verify:       *verify,
		SenderId:     from,
		SenderPubKey: senderPubKey,
		MessageId:    messageId,
	}
}

func (b BAgreementVerification) Type() string {
	return b.workType
}

func (b BAgreementVerification) String() string {
	pkey := "not set"
	if len(b.SenderPubKey) != 0 {
		pkey = "set"
	}
	return fmt.Sprintf("WorkType: %v, "+
		"Verify: %v, "+
		"MsgEndpoint: %v, "+
		"SenderId: %v, "+
		"SenderPubKey: %v, "+
		"MessageId: %v",
		b.workType, b.Verify, b.From, b.SenderId, pkey, b.MessageId)
}

func (b BAgreementVerification) ShortString() string {
	return b.String()
}

// These are work items that represent extensions to the protocol.
const AGREEMENT_VERIFICATION_REPLY = "AGREEMENT_VERIFY_REPLY"

type BAgreementVerificationReply struct {
	workType     string
	VerifyReply  basicprotocol.BAgreementVerifyReply
	From         string // deprecated whisper address
	SenderId     string // exchange Id of sender
	SenderPubKey []byte
	MessageId    int
}

func NewBAgreementVerificationReply(verifyr *basicprotocol.BAgreementVerifyReply, senderId string, senderPubKey []byte, messageId int) AgreementWork {
	return BAgreementVerificationReply{
		workType:     AGREEMENT_VERIFICATION_REPLY,
		VerifyReply:  *verifyr,
		SenderId:     senderId,
		SenderPubKey: senderPubKey,
		MessageId:    messageId,
	}
}

func (b BAgreementVerificationReply) Type() string {
	return b.workType
}

func (b BAgreementVerificationReply) String() string {
	pkey := "not set"
	if len(b.SenderPubKey) != 0 {
		pkey = "set"
	}
	return fmt.Sprintf("WorkType: %v, "+
		"VerifyReply: %v, "+
		"MsgEndpoint: %v, "+
		"SenderId: %v, "+
		"SenderPubKey: %v, "+
		"MessageId: %v",
		b.workType, b.VerifyReply, b.From, b.SenderId, pkey, b.MessageId)
}

func (b BAgreementVerificationReply) ShortString() string {
	return b.String()
}

// These are work items that represent extensions to the protocol.
const AGREEMENT_UPDATE = "AGREEMENT_UPDATE"

type BAgreementUpdate struct {
	workType     string
	Update       basicprotocol.BAgreementUpdate
	From         string // deprecated whisper address
	SenderId     string // exchange Id of sender
	SenderPubKey []byte
	MessageId    int
}

func NewBAgreementUpdate(update *basicprotocol.BAgreementUpdate, from string, senderPubKey []byte, messageId int) AgreementWork {
	return BAgreementUpdate{
		workType:     AGREEMENT_UPDATE,
		Update:       *update,
		SenderId:     from,
		SenderPubKey: senderPubKey,
		MessageId:    messageId,
	}
}

func (b BAgreementUpdate) Type() string {
	return b.workType
}

func (b BAgreementUpdate) String() string {
	pkey := "not set"
	if len(b.SenderPubKey) != 0 {
		pkey = "set"
	}
	return fmt.Sprintf("WorkType: %v, "+
		"Update: %v, "+
		"MsgEndpoint: %v, "+
		"SenderId: %v, "+
		"SenderPubKey: %v, "+
		"MessageId: %v",
		b.workType, b.Update, b.From, b.SenderId, pkey, b.MessageId)
}

func (b BAgreementUpdate) ShortString() string {
	return b.String()
}

// These are work items that represent extensions to the protocol.
const AGREEMENT_UPDATE_REPLY = "AGREEMENT_UPDATE_REPLY"

type BAgreementUpdateReply struct {
	workType     string
	Reply        basicprotocol.BAgreementUpdateReply
	From         string // deprecated whisper address
	SenderId     string // exchange Id of sender
	SenderPubKey []byte
	MessageId    int
}

func NewBAgreementUpdateReply(reply *basicprotocol.BAgreementUpdateReply, from string, senderPubKey []byte, messageId int) AgreementWork {
	return BAgreementUpdateReply{
		workType:     AGREEMENT_UPDATE_REPLY,
		Reply:        *reply,
		SenderId:     from,
		SenderPubKey: senderPubKey,
		MessageId:    messageId,
	}
}

func (b BAgreementUpdateReply) Type() string {
	return b.workType
}

func (b BAgreementUpdateReply) String() string {
	pkey := "not set"
	if len(b.SenderPubKey) != 0 {
		pkey = "set"
	}
	return fmt.Sprintf("WorkType: %v, "+
		"Reply: %v, "+
		"MsgEndpoint: %v, "+
		"SenderId: %v, "+
		"SenderPubKey: %v, "+
		"MessageId: %v",
		b.workType, b.Reply, b.From, b.SenderId, pkey, b.MessageId)
}

func (b BAgreementUpdateReply) ShortString() string {
	return b.String()
}

// This function receives an event to "make a new agreement" from the Process function, and then synchronously calls a function
// to actually work through the agreement protocol.

func (a *BasicAgreementWorker) start(work *PrioritizedWorkQueue, random *rand.Rand) {

	worker.GetWorkerStatusManager().SetSubworkerStatus("BasicProtocolHandler", a.workerID, worker.STATUS_STARTED)
	for {
		glog.V(5).Infof(bwlogstring(a.workerID, fmt.Sprintf("blocking for work")))
		workItemPtr := <-work.Receive() // block waiting for work
		if workItemPtr == nil {
			glog.V(3).Infof(bwlogstring(a.workerID, fmt.Sprintf("received nil work item")))
			continue
		}
		workItem := *workItemPtr
		glog.V(2).Infof(bwlogstring(a.workerID, fmt.Sprintf("received work: %v", workItem.ShortString())))

		if workItem.Type() == INITIATE {
			wi := workItem.(InitiateAgreement)
			a.InitiateNewAgreement(a.protocolHandler, &wi, random, a.workerID)

		} else if workItem.Type() == REPLY {
			wi := workItem.(HandleReply)
			if ok := a.HandleAgreementReply(a.protocolHandler, &wi, a.workerID); ok {
				// Update state in the database
				if ag, err := a.db.AgreementFinalized(wi.Reply.AgreementId(), a.protocolHandler.Name()); err != nil {
					glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("error persisting agreement %v finalized: %v", wi.Reply.AgreementId(), err)))

					// Update state in exchange
				} else if pol, err := policy.DemarshalPolicy(ag.Policy); err != nil {
					glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("error demarshalling policy from agreement %v, error: %v", wi.Reply.AgreementId(), err)))
				} else if err := a.protocolHandler.RecordConsumerAgreementState(wi.Reply.AgreementId(), pol, ag.Org, "Finalized Agreement", a.workerID); err != nil {
					glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("error setting agreement %v finalized state in exchange: %v", wi.Reply.AgreementId(), err)))
				}
			}

		} else if workItem.Type() == DATARECEIVEDACK {
			wi := workItem.(HandleDataReceivedAck)
			a.HandleDataReceivedAck(a.protocolHandler, &wi, a.workerID)

		} else if workItem.Type() == CANCEL {
			wi := workItem.(CancelAgreement)
			deleteMessage := a.CancelAgreementWithLock(a.protocolHandler, wi.AgreementId, wi.Reason, a.workerID)

			// Get rid of the original agreement cancellation message if the agreement is owned by this agbot.
			if wi.MessageId != 0 && deleteMessage {
				if err := a.protocolHandler.DeleteMessage(wi.MessageId); err != nil {
					glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("error deleting message %v from exchange", wi.MessageId)))
				}
			}

		} else if workItem.Type() == WORKLOAD_UPGRADE {
			// upgrade a workload on a device
			wi := workItem.(HandleWorkloadUpgrade)
			a.HandleWorkloadUpgrade(a.protocolHandler, &wi, a.workerID)

		} else if workItem.Type() == ASYNC_CANCEL {
			wi := workItem.(AsyncCancelAgreement)
			a.ExternalCancel(a.protocolHandler, wi.AgreementId, wi.Reason, a.workerID)

		} else if workItem.Type() == AGREEMENT_VERIFICATION {
			wi := workItem.(BAgreementVerification)

			// Archived and terminating agreements are considered to be non-existent.
			exists := false
			deleteMessage := true
			sendReply := true
			if agreement, err := a.db.FindSingleAgreementByAgreementId(wi.Verify.AgreementId(), a.protocolHandler.Name(), []persistence.AFilter{}); err != nil {
				glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("error querying agreement %v, error: %v", wi.Verify.AgreementId(), err)))
				sendReply = false
			} else if agreement != nil && agreement.Archived {
				// The agreement is not active and it is archived, so this message belongs to this agbot, and verify will return false.
				glog.V(3).Infof(bwlogstring(a.workerID, fmt.Sprintf("verify is for a cancelled agreement %v, deleting verify message.", wi.Verify.AgreementId())))
			} else if agreement == nil {
				// The verify is for an agreement that this agbot doesnt know anything about, so ignore the verify msg.
				glog.Warningf(bwlogstring(a.workerID, fmt.Sprintf("discarding verify %v for agreement id %v not in this agbot's database", wi.MessageId, wi.Verify.AgreementId())))
				deleteMessage = false
				sendReply = false
			} else if agreement.AgreementTimedout == 0 {
				exists = true
			}

			// Reply to the sender with our decision on the agreement.
			if sendReply {
				if mt, err := exchange.CreateMessageTarget(wi.SenderId, nil, wi.SenderPubKey, wi.From); err != nil {
					glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("error creating message target: %v", err)))
				} else if aph, ok := a.protocolHandler.AgreementProtocolHandler("", "", "").(*basicprotocol.ProtocolHandler); !ok {
					glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("error casting to basic protocol handler (%T): %v", a.protocolHandler.AgreementProtocolHandler("", "", ""), err)))
				} else if err := aph.SendAgreementVerificationReply(wi.Verify.AgreementId(), exists, mt, a.protocolHandler.GetSendMessage()); err != nil {
					glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("error trying to send agreement verification reply for %v to %v, error: %v", wi.Verify.AgreementId(), mt, err)))
				}
			}

			// Get rid of the original agreement validation request message.
			if wi.MessageId != 0 && deleteMessage {
				if err := a.protocolHandler.DeleteMessage(wi.MessageId); err != nil {
					glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("error deleting message %v from exchange", wi.MessageId)))
				}
			}

		} else if workItem.Type() == AGREEMENT_VERIFICATION_REPLY {
			wi := workItem.(BAgreementVerificationReply)

			cancel := false
			deleteMessage := true
			if agreement, err := a.db.FindSingleAgreementByAgreementId(wi.VerifyReply.AgreementId(), a.protocolHandler.Name(), []persistence.AFilter{}); err != nil {
				glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("error querying agreement %v, error: %v", wi.VerifyReply.AgreementId(), err)))
			} else if agreement != nil && agreement.Archived {
				// The agreement is not active and it is archived, so this message belongs to this agbot.
				glog.V(3).Infof(bwlogstring(a.workerID, fmt.Sprintf("verify reply is for a cancelled agreement %v, deleting verify reply message.", wi.VerifyReply.AgreementId())))
			} else if agreement == nil {
				// The verify is for an agreement that this agbot doesnt know anything about, so ignore the verify reply msg.
				glog.Warningf(bwlogstring(a.workerID, fmt.Sprintf("discarding verify reply %v for agreement id %v not in this agbot's database", wi.MessageId, wi.VerifyReply.AgreementId())))
				deleteMessage = false
			} else if agreement.AgreementTimedout == 0 && !wi.VerifyReply.Exists {
				cancel = true
			}

			if cancel {
				deleteMessage = a.CancelAgreementWithLock(a.protocolHandler, wi.VerifyReply.AgreementId(), basicprotocol.AB_CANCEL_AG_MISSING, a.workerID)
			}

			// Get rid of the original message if the agreement is owned by this agbot.
			if wi.MessageId != 0 && deleteMessage {
				if err := a.protocolHandler.DeleteMessage(wi.MessageId); err != nil {
					glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("error deleting message %v from exchange", wi.MessageId)))
				}
			}

		} else if workItem.Type() == MMS_OBJECT_POLICY {
			// Handle an update to an object policy. The source for this function is in the policy.go file.
			wi := workItem.(ObjectPolicyChange)
			a.HandleMMSObjectPolicy(a.protocolHandler, &wi, a.workerID)

		} else if workItem.Type() == STOP {
			// At this point, we assume that the parent agreement bot worker has already decided that it's ok to terminate.
			break

		} else if workItem.Type() == AGREEMENT_UPDATE {
			wi := workItem.(BAgreementUpdate)

			// Assume the original message is always deleted.
			deleteMessage := true

			// Agreement update request received from an agent.
			glog.V(3).Infof(bwlogstring(a.workerID, fmt.Sprintf("no support for agreement update %v from an agent, replying with rejection.", wi.Update.ShortString())))

			// Always reply not accepted.
			accepted := false
			if mt, err := exchange.CreateMessageTarget(wi.SenderId, nil, wi.SenderPubKey, wi.From); err != nil {
				glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("error creating message target: %v", err)))
			} else if aph, ok := a.protocolHandler.AgreementProtocolHandler("", "", "").(*basicprotocol.ProtocolHandler); !ok {
				glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("error casting to basic protocol handler (%T): %v", a.protocolHandler.AgreementProtocolHandler("", "", ""), err)))
			} else if err := aph.SendAgreementUpdateReply(wi.Update.AgreementId(), wi.Update.UpdateType(), accepted, mt, a.protocolHandler.GetSendMessage()); err != nil {
				glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("error trying to send agreement update reply for %v to %v, error: %v", wi.Update.ShortString(), mt, err)))
			}

			// Get rid of the original agreement update message.
			if wi.MessageId != 0 && deleteMessage {
				if err := a.protocolHandler.DeleteMessage(wi.MessageId); err != nil {
					glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("error deleting message %v from exchange", wi.MessageId)))
				}
			}

		} else if workItem.Type() == AGREEMENT_UPDATE_REPLY {
			wi := workItem.(BAgreementUpdateReply)

			// Assume the original message is always deleted
			deleteMessage := true

			if wi.Reply.IsAccepted() {
				if wi.Reply.IsSecretUpdate() {

					// Get the agreement id lock to prevent any other thread from processing this same agreement.
					lock := a.alm.getAgreementLock(wi.Reply.AgreementId())
					lock.Lock()

					// update the system to indicate that the secret update is complete.
					glog.V(5).Infof(bwlogstring(a.workerID, fmt.Sprintf("secret update accepted %v", wi.Reply.ShortString())))

					// Record the secret update ACK message.
					if agreement, err := a.db.FindSingleAgreementByAgreementId(wi.Reply.AgreementId(), a.protocolHandler.Name(), []persistence.AFilter{}); err != nil {
						glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("error querying agreement %v, error: %v", wi.Reply.AgreementId(), err)))
					} else {
						if agreement != nil {
							if _, err := a.db.AgreementSecretUpdateAckTime(wi.Reply.AgreementId(), a.protocolHandler.Name(), agreement.LastSecretUpdateTime); err != nil {
								glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("unable to save secret update ack time for %s, error: %v", wi.Reply.AgreementId(), err)))
							}
						} else {
							// Agreement must belong to other agbot
							deleteMessage = false
						}
					}

					// Drop the agreement lock
					lock.Unlock()

				}

			} else {
				// Log the reject and then update the state of the system to prevent further updates for this change.
				glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("agent rejected update %v", wi.Reply.ShortString())))

			}

			// Get rid of the original agreement update reply message.
			if wi.MessageId != 0 && deleteMessage {
				if err := a.protocolHandler.DeleteMessage(wi.MessageId); err != nil {
					glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("error deleting message %v from exchange", wi.MessageId)))
				}
			}

		} else {
			glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("received unknown work request: %v", workItem)))
		}

		glog.V(5).Infof(bwlogstring(a.workerID, fmt.Sprintf("handled work: %v", workItem)))
		runtime.Gosched()

	}

	glog.V(5).Infof(bwlogstring(a.workerID, fmt.Sprintf("terminating")))

}

var bwlogstring = func(workerID string, v interface{}) string {
	return fmt.Sprintf("BasicAgreementWorker (%v): %v", workerID, v)
}
