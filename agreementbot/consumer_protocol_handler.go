package agreementbot

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/abstractprotocol"
	"github.com/open-horizon/anax/agreementbot/persistence"
	"github.com/open-horizon/anax/agreementbot/secrets"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/metering"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/worker"
	"net/http"
	"time"
)

func CreateConsumerPH(name string, cfg *config.HorizonConfig, db persistence.AgbotDatabase, pm *policy.PolicyManager, msgq chan events.Message, mmsObjMgr *MMSObjectPolicyManager, secretsMgr secrets.AgbotSecrets) ConsumerProtocolHandler {
	if handler := NewBasicProtocolHandler(name, cfg, db, pm, msgq, mmsObjMgr, secretsMgr); handler != nil {
		return handler
	} // Add new consumer side protocol handlers here
	return nil
}

type ConsumerProtocolHandler interface {
	Initialize()
	Name() string
	AcceptCommand(cmd worker.Command) bool
	AgreementProtocolHandler(typeName string, name string, org string) abstractprotocol.ProtocolHandler
	WorkQueue() *PrioritizedWorkQueue
	DispatchProtocolMessage(cmd *NewProtocolMessageCommand, cph ConsumerProtocolHandler) error
	PersistAgreement(wi *InitiateAgreement, proposal abstractprotocol.Proposal, workerID string) error
	PersistReply(reply abstractprotocol.ProposalReply, pol *policy.Policy, workerID string) error
	HandleAgreementTimeout(cmd *AgreementTimeoutCommand, cph ConsumerProtocolHandler)
	HandleBlockchainEvent(cmd *BlockchainEventCommand)
	HandlePolicyChanged(cmd *PolicyChangedCommand, cph ConsumerProtocolHandler)
	HandlePolicyDeleted(cmd *PolicyDeletedCommand, cph ConsumerProtocolHandler)
	HandleServicePolicyChanged(cmd *ServicePolicyChangedCommand, cph ConsumerProtocolHandler)
	HandleServicePolicyDeleted(cmd *ServicePolicyDeletedCommand, cph ConsumerProtocolHandler)
	HandleMMSObjectPolicy(cmd *MMSObjectPolicyEventCommand, cph ConsumerProtocolHandler)
	HandleWorkloadUpgrade(cmd *WorkloadUpgradeCommand, cph ConsumerProtocolHandler)
	HandleMakeAgreement(cmd *MakeAgreementCommand, cph ConsumerProtocolHandler)
	HandleStopProtocol(cph ConsumerProtocolHandler)
	GetTerminationCode(reason string) uint
	GetTerminationReason(code uint) string
	IsTerminationReasonNodeShutdown(code uint) bool
	GetSendMessage() func(mt interface{}, pay []byte) error
	RecordConsumerAgreementState(agreementId string, pol *policy.Policy, org string, state string, workerID string) error
	DeleteMessage(msgId int) error
	CreateMeteringNotification(mp policy.Meter, agreement *persistence.Agreement) (*metering.MeteringNotification, error)
	TerminateAgreement(agreement *persistence.Agreement, reason uint, workerId string)
	VerifyAgreement(ag *persistence.Agreement, cph ConsumerProtocolHandler)
	UpdateAgreement(ag *persistence.Agreement, updateType string, metadata interface{}, cph ConsumerProtocolHandler)
	GetDeviceMessageEndpoint(deviceId string, workerId string) (string, []byte, error)
	SetBlockchainClientAvailable(ev *events.BlockchainClientInitializedMessage)
	SetBlockchainClientNotAvailable(ev *events.BlockchainClientStoppingMessage)
	SetBlockchainWritable(ev *events.AccountFundedMessage)
	IsBlockchainWritable(typeName string, name string, org string) bool
	CanCancelNow(agreement *persistence.Agreement) bool
	DeferCommand(cmd AgreementWork)
	GetDeferredCommands() []AgreementWork
	HandleDeferredCommands()
	PostReply(agreementId string, proposal abstractprotocol.Proposal, reply abstractprotocol.ProposalReply, consumerPolicy *policy.Policy, org string, workerId string) error
	UpdateProducer(ag *persistence.Agreement)
	HandleExtensionMessage(cmd *NewProtocolMessageCommand) error
	AlreadyReceivedReply(ag *persistence.Agreement) bool
	GetKnownBlockchain(ag *persistence.Agreement) (string, string, string)
	CanSendMeterRecord(ag *persistence.Agreement) bool
	GetExchangeId() string
	GetExchangeToken() string
	GetExchangeURL() string
	GetCSSURL() string
	GetServiceBased() bool
	GetHTTPFactory() *config.HTTPClientFactory
	SendEventMessage(event events.Message)
}

type BaseConsumerProtocolHandler struct {
	name             string
	pm               *policy.PolicyManager
	db               persistence.AgbotDatabase
	config           *config.HorizonConfig
	httpClient       *http.Client // shared HTTP client instance
	agbotId          string
	token            string
	deferredCommands []AgreementWork // The agreement related work that has to be deferred and retried
	messages         chan events.Message
	mmsObjMgr        *MMSObjectPolicyManager
	secretsMgr       secrets.AgbotSecrets
}

func (b *BaseConsumerProtocolHandler) GetSendMessage() func(mt interface{}, pay []byte) error {
	return b.sendMessage
}

func (b *BaseConsumerProtocolHandler) Name() string {
	return b.name
}

func (b *BaseConsumerProtocolHandler) GetExchangeId() string {
	return b.agbotId
}

func (b *BaseConsumerProtocolHandler) GetExchangeToken() string {
	return b.token
}

func (b *BaseConsumerProtocolHandler) GetExchangeURL() string {
	return b.config.AgreementBot.ExchangeURL
}

func (b *BaseConsumerProtocolHandler) GetCSSURL() string {
	return b.config.GetAgbotCSSURL()
}

func (b *BaseConsumerProtocolHandler) GetServiceBased() bool {
	return false
}

func (b *BaseConsumerProtocolHandler) GetHTTPFactory() *config.HTTPClientFactory {
	return b.config.Collaborators.HTTPClientFactory
}

func (w *BaseConsumerProtocolHandler) sendMessage(mt interface{}, pay []byte) error {
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

	logMsg := string(pay)

	// Try to demarshal pay into Proposal struct
	if glog.V(5) {
		if newProp, err := abstractprotocol.DemarshalProposal(logMsg); err == nil {
			// check if log message is a byte-encoded Proposal struct
			if len(newProp.AgreementId()) > 0 {
				// if it is a proposal, obscure the secrets
				if logMsg, err = abstractprotocol.ObscureProposalSecret(logMsg); err != nil {
					// something went wrong, send empty string to ensure secret protection
					logMsg = ""
				}
			}
		}
	}

	// Grab the exchange ID of the message receiver
	glog.V(3).Infof(BCPHlogstring(w.Name(), fmt.Sprintf("sending exchange message to: %v, message %v", messageTarget.ReceiverExchangeId, cutil.TruncateDisplayString(string(pay), 300))))
	glog.V(5).Infof(BCPHlogstring(w.Name(), fmt.Sprintf("sending exchange message to: %v, message %v", messageTarget.ReceiverExchangeId, logMsg)))

	// Get my own keys
	myPubKey, myPrivKey, keyErr := exchange.GetKeys(w.config.AgreementBot.MessageKeyPath)
	if keyErr != nil {
		return errors.New(fmt.Sprintf("error getting keys: %v", keyErr))
	}

	// Demarshal the receiver's public key if we need to
	if messageTarget.ReceiverPublicKeyObj == nil {
		if mtpk, err := exchange.DemarshalPublicKey(messageTarget.ReceiverPublicKeyBytes); err != nil {
			return errors.New(fmt.Sprintf("Unable to demarshal device's public key %x, error %v", messageTarget.ReceiverPublicKeyBytes, err))
		} else {
			messageTarget.ReceiverPublicKeyObj = mtpk
		}
	}

	exchDev, err := exchange.GetExchangeDevice(w.GetHTTPFactory(), messageTarget.ReceiverExchangeId, w.agbotId, w.token, w.config.AgreementBot.ExchangeURL)
	if err != nil {
		return fmt.Errorf("Unable to get device from exchange: %v", err)
	}
	maxHb := exchDev.HeartbeatIntv.MaxInterval
	if maxHb == 0 {
		exchOrg, err := exchange.GetOrganization(w.GetHTTPFactory(), exchange.GetOrg(messageTarget.ReceiverExchangeId), w.config.AgreementBot.ExchangeURL, w.agbotId, w.token)
		if err != nil {
			return fmt.Errorf("Unable to get org from exchange: %v", err)
		}
		maxHb = exchOrg.HeartbeatIntv.MaxInterval
	}
	exchangeMessageTTL := w.config.AgreementBot.GetExchangeMessageTTL(maxHb)

	// Create an encrypted message
	if encryptedMsg, err := exchange.ConstructExchangeMessage(pay, myPubKey, myPrivKey, messageTarget.ReceiverPublicKeyObj); err != nil {
		return errors.New(fmt.Sprintf("Unable to construct encrypted message, error %v for message %s", err, pay))
		// Marshal it into a byte array
	} else if msgBody, err := json.Marshal(encryptedMsg); err != nil {
		return errors.New(fmt.Sprintf("Unable to marshal exchange message, error %v for message %v", err, encryptedMsg))
		// Send it to the device's message queue
	} else {
		pm := exchange.CreatePostMessage(msgBody, exchangeMessageTTL)
		var resp interface{}
		resp = new(exchange.PostDeviceResponse)
		targetURL := w.config.AgreementBot.ExchangeURL + "orgs/" + exchange.GetOrg(messageTarget.ReceiverExchangeId) + "/nodes/" + exchange.GetId(messageTarget.ReceiverExchangeId) + "/msgs"
		for {
			if err, tpErr := exchange.InvokeExchange(w.httpClient, "POST", targetURL, w.agbotId, w.token, pm, &resp); err != nil {
				return err
			} else if tpErr != nil {
				glog.Warningf(tpErr.Error())
				time.Sleep(10 * time.Second)
				continue
			} else {
				glog.V(5).Infof(BCPHlogstring(w.Name(), fmt.Sprintf("sent message for %v to exchange.", messageTarget.ReceiverExchangeId)))
				return nil
			}
		}
	}

}

func (b *BaseConsumerProtocolHandler) DispatchProtocolMessage(cmd *NewProtocolMessageCommand, cph ConsumerProtocolHandler) error {

	glog.V(5).Infof(BCPHlogstring(b.Name(), fmt.Sprintf("received inbound exchange message.")))

	// Figure out what kind of message this is
	if reply, rerr := cph.AgreementProtocolHandler("", "", "").ValidateReply(string(cmd.Message)); rerr == nil {
		agreementWork := NewHandleReply(reply, cmd.From, cmd.PubKey, cmd.MessageId)
		cph.WorkQueue().InboundHigh() <- &agreementWork
		glog.V(5).Infof(BCPHlogstring(b.Name(), fmt.Sprintf("queued reply message")))
	} else if _, aerr := cph.AgreementProtocolHandler("", "", "").ValidateDataReceivedAck(string(cmd.Message)); aerr == nil {
		agreementWork := NewHandleDataReceivedAck(string(cmd.Message), cmd.From, cmd.PubKey, cmd.MessageId)
		cph.WorkQueue().InboundHigh() <- &agreementWork
		glog.V(5).Infof(BCPHlogstring(b.Name(), fmt.Sprintf("queued data received ack message")))
	} else if can, cerr := cph.AgreementProtocolHandler("", "", "").ValidateCancel(string(cmd.Message)); cerr == nil {
		// Before dispatching the cancel to a worker thread, make sure it's a valid cancel
		if ag, err := b.db.FindSingleAgreementByAgreementId(can.AgreementId(), can.Protocol(), []persistence.AFilter{}); err != nil {
			glog.Errorf(BCPHlogstring(b.Name(), fmt.Sprintf("error finding agreement %v in the db", can.AgreementId())))
		} else if ag == nil {
			glog.Warningf(BCPHlogstring(b.Name(), fmt.Sprintf("cancel ignored, cannot find agreement %v in the db", can.AgreementId())))
		} else if ag.DeviceId != cmd.From {
			glog.Warningf(BCPHlogstring(b.Name(), fmt.Sprintf("cancel ignored, cancel message for %v came from id %v but agreement is with %v", can.AgreementId(), cmd.From, ag.DeviceId)))
		} else {
			agreementWork := NewCancelAgreement(can.AgreementId(), can.Protocol(), can.Reason(), cmd.MessageId)
			cph.WorkQueue().InboundHigh() <- &agreementWork
			glog.V(5).Infof(BCPHlogstring(b.Name(), fmt.Sprintf("queued cancel message")))
		}
	} else if exerr := cph.HandleExtensionMessage(cmd); exerr == nil {
		// nothing to do
	} else {
		glog.V(5).Infof(BCPHlogstring(b.Name(), fmt.Sprintf("ignoring  message: %v because it is an unknown type", string(cmd.Message))))
		return errors.New(BCPHlogstring(b.Name(), fmt.Sprintf("unexpected protocol msg %v", cmd.Message)))
	}
	return nil

}

func (b *BaseConsumerProtocolHandler) HandleAgreementTimeout(cmd *AgreementTimeoutCommand, cph ConsumerProtocolHandler) {

	glog.V(5).Infof(BCPHlogstring(b.Name(), "received agreement cancellation."))
	agreementWork := NewCancelAgreement(cmd.AgreementId, cmd.Protocol, cmd.Reason, 0)
	cph.WorkQueue().InboundHigh() <- &agreementWork
	glog.V(5).Infof(BCPHlogstring(b.Name(), "queued agreement cancellation"))

}

func (b *BaseConsumerProtocolHandler) HandlePolicyChanged(cmd *PolicyChangedCommand, cph ConsumerProtocolHandler) {

	glog.V(5).Infof(BCPHlogstring(b.Name(), "received policy changed command."))

	if eventPol, err := policy.DemarshalPolicy(cmd.Msg.PolicyString()); err != nil {
		glog.Errorf(BCPHlogstring(b.Name(), fmt.Sprintf("error demarshalling change policy event %v, error: %v", cmd.Msg.PolicyString(), err)))
	} else {

		InProgress := func() persistence.AFilter {
			return func(e persistence.Agreement) bool { return e.AgreementCreationTime != 0 && e.AgreementTimedout == 0 }
		}

		if agreements, err := b.db.FindAgreements([]persistence.AFilter{persistence.UnarchivedAFilter(), InProgress()}, cph.Name()); err == nil {
			for _, ag := range agreements {

				if pol, err := policy.DemarshalPolicy(ag.Policy); err != nil {
					glog.Errorf(BCPHlogstring(b.Name(), fmt.Sprintf("unable to demarshal policy for agreement %v, error %v", ag.CurrentAgreementId, err)))

				} else if eventPol.Header.Name != pol.Header.Name {
					// This agreement is using a policy different from the one that changed.
					glog.V(5).Infof(BCPHlogstring(b.Name(), fmt.Sprintf("policy change handler skipping agreement %v because it is using a policy that did not change.", ag.CurrentAgreementId)))
					continue
				} else if err := b.pm.MatchesMine(cmd.Msg.Org(), pol); err != nil {
					glog.Warningf(BCPHlogstring(b.Name(), fmt.Sprintf("agreement %v has a policy %v that has changed: %v", ag.CurrentAgreementId, pol.Header.Name, err)))
					b.CancelAgreement(ag, TERM_REASON_POLICY_CHANGED, cph)
				} else {
					glog.V(5).Infof(BCPHlogstring(b.Name(), fmt.Sprintf("for agreement %v, no policy content differences detected", ag.CurrentAgreementId)))
				}

			}
		} else {
			glog.Errorf(BCPHlogstring(b.Name(), fmt.Sprintf("error searching database: %v", err)))
		}
	}
}

func (b *BaseConsumerProtocolHandler) HandlePolicyDeleted(cmd *PolicyDeletedCommand, cph ConsumerProtocolHandler) {
	glog.V(5).Infof(BCPHlogstring(b.Name(), "received policy deleted command."))

	InProgress := func() persistence.AFilter {
		return func(e persistence.Agreement) bool { return e.AgreementCreationTime != 0 && e.AgreementTimedout == 0 }
	}

	if agreements, err := b.db.FindAgreements([]persistence.AFilter{persistence.UnarchivedAFilter(), InProgress()}, cph.Name()); err == nil {
		for _, ag := range agreements {

			if pol, err := policy.DemarshalPolicy(ag.Policy); err != nil {
				glog.Errorf(BCPHlogstring(b.Name(), fmt.Sprintf("unable to demarshal policy for agreement %v, error %v", ag.CurrentAgreementId, err)))
			} else if cmd.Msg.Org() == ag.Org {
				if existingPol := b.pm.GetPolicy(cmd.Msg.Org(), pol.Header.Name); existingPol == nil {
					glog.Errorf(BCPHlogstring(b.Name(), fmt.Sprintf("agreement %v has a policy %v that doesn't exist anymore", ag.CurrentAgreementId, pol.Header.Name)))

					// Remove any workload usage records so that a new agreement will be made starting from the highest priority workload.
					if err := b.db.DeleteWorkloadUsage(ag.DeviceId, ag.PolicyName); err != nil {
						glog.Warningf(BCPHlogstring(b.Name(), fmt.Sprintf("error deleting workload usage for %v using policy %v, error: %v", ag.DeviceId, ag.PolicyName, err)))
					}

					// Queue up a cancellation command for this agreement.
					agreementWork := NewCancelAgreement(ag.CurrentAgreementId, ag.AgreementProtocol, cph.GetTerminationCode(TERM_REASON_POLICY_CHANGED), 0)
					cph.WorkQueue().InboundHigh() <- &agreementWork

				}
			}
		}
	} else {
		glog.Errorf(BCPHlogstring(b.Name(), fmt.Sprintf("error searching database: %v", err)))
	}
}

func (b *BaseConsumerProtocolHandler) HandleServicePolicyChanged(cmd *ServicePolicyChangedCommand, cph ConsumerProtocolHandler) {

	glog.V(5).Infof(BCPHlogstring(b.Name(), "received service policy changed command: %v. cmd"))

	InProgress := func() persistence.AFilter {
		return func(e persistence.Agreement) bool { return e.AgreementCreationTime != 0 && e.AgreementTimedout == 0 }
	}

	if agreements, err := b.db.FindAgreements([]persistence.AFilter{persistence.UnarchivedAFilter(), InProgress()}, cph.Name()); err == nil {
		for _, ag := range agreements {
			if ag.Pattern == "" && ag.PolicyName == fmt.Sprintf("%v/%v", cmd.Msg.BusinessPolOrg, cmd.Msg.BusinessPolName) && ag.ServiceId[0] == cmd.Msg.ServiceId {

				glog.Warningf(BCPHlogstring(b.Name(), fmt.Sprintf("agreement %v has a service policy %v that has changed.", ag.CurrentAgreementId, ag.ServiceId)))
				b.CancelAgreement(ag, TERM_REASON_POLICY_CHANGED, cph)
			}
		}
	} else {
		glog.Errorf(BCPHlogstring(b.Name(), fmt.Sprintf("error searching database: %v", err)))
	}
}

func (b *BaseConsumerProtocolHandler) HandleServicePolicyDeleted(cmd *ServicePolicyDeletedCommand, cph ConsumerProtocolHandler) {
	glog.V(5).Infof(BCPHlogstring(b.Name(), "received policy deleted command."))

	InProgress := func() persistence.AFilter {
		return func(e persistence.Agreement) bool { return e.AgreementCreationTime != 0 && e.AgreementTimedout == 0 }
	}

	if agreements, err := b.db.FindAgreements([]persistence.AFilter{persistence.UnarchivedAFilter(), InProgress()}, cph.Name()); err == nil {
		for _, ag := range agreements {

			if ag.Pattern == "" && ag.PolicyName == fmt.Sprintf("%v/%v", cmd.Msg.BusinessPolOrg, cmd.Msg.BusinessPolName) && ag.ServiceId[0] == cmd.Msg.ServiceId {
				glog.Errorf(BCPHlogstring(b.Name(), fmt.Sprintf("agreement %v has a service policy %v that doesn't exist anymore", ag.CurrentAgreementId, ag.ServiceId)))

				// Remove any workload usage records so that a new agreement will be made starting from the highest priority workload.
				if err := b.db.DeleteWorkloadUsage(ag.DeviceId, ag.PolicyName); err != nil {
					glog.Warningf(BCPHlogstring(b.Name(), fmt.Sprintf("error deleting workload usage for %v using policy %v, error: %v", ag.DeviceId, ag.PolicyName, err)))
				}

				// Queue up a cancellation command for this agreement.
				agreementWork := NewCancelAgreement(ag.CurrentAgreementId, ag.AgreementProtocol, cph.GetTerminationCode(TERM_REASON_POLICY_CHANGED), 0)
				cph.WorkQueue().InboundHigh() <- &agreementWork

			}
		}
	} else {
		glog.Errorf(BCPHlogstring(b.Name(), fmt.Sprintf("error searching database: %v", err)))
	}
}

func (b *BaseConsumerProtocolHandler) HandleMMSObjectPolicy(cmd *MMSObjectPolicyEventCommand, cph ConsumerProtocolHandler) {
	glog.V(5).Infof(BCPHlogstring(b.Name(), fmt.Sprintf("received object policy change command.")))
	agreementWork := NewObjectPolicyChange(cmd.Msg)
	cph.WorkQueue().InboundHigh() <- &agreementWork
	glog.V(5).Infof(BCPHlogstring(b.Name(), fmt.Sprintf("queued object policy change command.")))
}

func (b *BaseConsumerProtocolHandler) CancelAgreement(ag persistence.Agreement, reason string, cph ConsumerProtocolHandler) {
	// Remove any workload usage records (non-HA) or mark for pending upgrade (HA). There might not be a workload usage record
	// if the consumer policy does not specify the workload priority section.
	if wlUsage, err := b.db.FindSingleWorkloadUsageByDeviceAndPolicyName(ag.DeviceId, ag.PolicyName); err != nil {
		glog.Warningf(BCPHlogstring(b.Name(), fmt.Sprintf("error retreiving workload usage for %v using policy %v, error: %v", ag.DeviceId, ag.PolicyName, err)))
	} else if wlUsage != nil && len(wlUsage.HAPartners) != 0 && wlUsage.PendingUpgradeTime != 0 {
		// Skip this agreement, it is part of an HA group where another member is upgrading
		return
	} else if wlUsage != nil && len(wlUsage.HAPartners) != 0 && wlUsage.PendingUpgradeTime == 0 {
		for _, partnerId := range wlUsage.HAPartners {
			if _, err := b.db.UpdatePendingUpgrade(partnerId, ag.PolicyName); err != nil {
				glog.Warningf(BCPHlogstring(b.Name(), fmt.Sprintf("could not update pending workload upgrade for %v using policy %v, error: %v", partnerId, ag.PolicyName, err)))
			}
		}
		// Choose this device's agreement within the HA group to start upgrading.
		// Delete this workload usage record so that a new agreement will be made starting from the highest priority workload
		if err := b.db.DeleteWorkloadUsage(ag.DeviceId, ag.PolicyName); err != nil {
			glog.Warningf(BCPHlogstring(b.Name(), fmt.Sprintf("error deleting workload usage for %v using policy %v, error: %v", ag.DeviceId, ag.PolicyName, err)))
		}
		agreementWork := NewCancelAgreement(ag.CurrentAgreementId, ag.AgreementProtocol, cph.GetTerminationCode(reason), 0)
		cph.WorkQueue().InboundHigh() <- &agreementWork
	} else {
		// Non-HA device or agreement without workload priority in the policy, re-make the agreement.
		// Delete this workload usage record so that a new agreement will be made starting from the highest priority workload
		if err := b.db.DeleteWorkloadUsage(ag.DeviceId, ag.PolicyName); err != nil {
			glog.Warningf(BCPHlogstring(b.Name(), fmt.Sprintf("error deleting workload usage for %v using policy %v, error: %v", ag.DeviceId, ag.PolicyName, err)))
		}
		agreementWork := NewCancelAgreement(ag.CurrentAgreementId, ag.AgreementProtocol, cph.GetTerminationCode(reason), 0)
		cph.WorkQueue().InboundHigh() <- &agreementWork
	}
}

func (b *BaseConsumerProtocolHandler) HandleWorkloadUpgrade(cmd *WorkloadUpgradeCommand, cph ConsumerProtocolHandler) {
	glog.V(5).Infof(BCPHlogstring(b.Name(), fmt.Sprintf("received workload upgrade command.")))
	upgradeWork := NewHandleWorkloadUpgrade(cmd.Msg.AgreementId, cmd.Msg.AgreementProtocol, cmd.Msg.DeviceId, cmd.Msg.PolicyName)
	cph.WorkQueue().InboundHigh() <- &upgradeWork
	glog.V(5).Infof(BCPHlogstring(b.Name(), fmt.Sprintf("queued workload upgrade command.")))
}

func (b *BaseConsumerProtocolHandler) HandleMakeAgreement(cmd *MakeAgreementCommand, cph ConsumerProtocolHandler) {
	glog.V(5).Infof(BCPHlogstring(b.Name(), fmt.Sprintf("received make agreement command.")))
	agreementWork := NewInitiateAgreement(cmd.ProducerPolicy, cmd.ConsumerPolicy, cmd.Org, cmd.Device, cmd.ConsumerPolicyName, cmd.ServicePolicies)
	cph.WorkQueue().InboundLow() <- &agreementWork
	glog.V(5).Infof(BCPHlogstring(b.Name(), fmt.Sprintf("queued make agreement command.")))
}

func (b *BaseConsumerProtocolHandler) HandleStopProtocol(cph ConsumerProtocolHandler) {
	glog.V(5).Infof(BCPHlogstring(b.Name(), fmt.Sprintf("received stop protocol command.")))

	for ix := 0; ix < b.config.AgreementBot.AgreementWorkers; ix++ {
		work := NewStopWorker()
		cph.WorkQueue().InboundHigh() <- &work
	}

	glog.V(5).Infof(BCPHlogstring(b.Name(), fmt.Sprintf("queued %x stop protocol commands.", b.config.AgreementBot.AgreementWorkers)))
}

func (b *BaseConsumerProtocolHandler) PersistBaseAgreement(wi *InitiateAgreement, proposal abstractprotocol.Proposal, workerID string, hash string, sig string) error {

	if polBytes, err := json.Marshal(wi.ConsumerPolicy); err != nil {
		return errors.New(BCPHlogstring2(workerID, fmt.Sprintf("error marshalling policy for storage %v, error: %v", wi.ConsumerPolicy, err)))
	} else if pBytes, err := json.Marshal(proposal); err != nil {
		return errors.New(BCPHlogstring2(workerID, fmt.Sprintf("error marshalling proposal for storage %v, error: %v", proposal, err)))
	} else if pol, err := policy.DemarshalPolicy(proposal.TsAndCs()); err != nil {
		return errors.New(BCPHlogstring2(workerID, fmt.Sprintf("error demarshalling TsandCs policy from pending agreement %v, error: %v", proposal.AgreementId(), err)))
	} else if _, err := b.db.AgreementUpdate(proposal.AgreementId(), string(pBytes), string(polBytes), pol.DataVerify, b.config.AgreementBot.ProcessGovernanceIntervalS, hash, sig, b.Name(), proposal.Version()); err != nil {
		return errors.New(BCPHlogstring2(workerID, fmt.Sprintf("error updating agreement with proposal %v in DB, error: %v", proposal, err)))

		// Record that the agreement was initiated, in the exchange
	} else if err := b.RecordConsumerAgreementState(proposal.AgreementId(), pol, wi.Org, "Formed Proposal", workerID); err != nil {
		return errors.New(BCPHlogstring2(workerID, fmt.Sprintf("error setting agreement state for %v", proposal.AgreementId())))
	}

	return nil
}

func (b *BaseConsumerProtocolHandler) PersistReply(reply abstractprotocol.ProposalReply, pol *policy.Policy, workerID string) error {

	if _, err := b.db.AgreementMade(reply.AgreementId(), reply.DeviceId(), "", b.Name(), pol.HAGroup.Partners, "", "", ""); err != nil {
		return errors.New(BCPHlogstring2(workerID, fmt.Sprintf("error updating agreement %v with reply info in DB, error: %v", reply.AgreementId(), err)))
	}
	return nil

}

func (b *BaseConsumerProtocolHandler) RecordConsumerAgreementState(agreementId string, pol *policy.Policy, org string, state string, workerID string) error {

	workload := pol.Workloads[0].WorkloadURL

	glog.V(5).Infof(BCPHlogstring2(workerID, fmt.Sprintf("setting agreement %v for workload %v/%v state to %v", agreementId, org, workload, state)))

	as := new(exchange.PutAgbotAgreementState)
	as.Service = exchange.WorkloadAgreement{
		Org:     org,
		Pattern: exchange.GetId(pol.PatternId),
		URL:     workload,
	}
	as.State = state

	var resp interface{}
	resp = new(exchange.PostDeviceResponse)
	targetURL := b.config.AgreementBot.ExchangeURL + "orgs/" + exchange.GetOrg(b.agbotId) + "/agbots/" + exchange.GetId(b.agbotId) + "/agreements/" + agreementId
	for {
		if err, tpErr := exchange.InvokeExchange(b.httpClient, "PUT", targetURL, b.agbotId, b.token, &as, &resp); err != nil {
			glog.Errorf(err.Error())
			return err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(5).Infof(BCPHlogstring2(workerID, fmt.Sprintf("set agreement %v to state %v", agreementId, state)))
			return nil
		}
	}

}

func (b *BaseConsumerProtocolHandler) DeleteMessage(msgId int) error {

	return DeleteMessage(msgId, b.agbotId, b.token, b.config.AgreementBot.ExchangeURL, b.httpClient)

}

func (b *BaseConsumerProtocolHandler) TerminateAgreement(ag *persistence.Agreement, reason uint, mt interface{}, workerId string, cph ConsumerProtocolHandler) {
	if pol, err := policy.DemarshalPolicy(ag.Policy); err != nil {
		glog.Errorf(BCPHlogstring2(workerId, fmt.Sprintf("unable to demarshal policy while trying to cancel %v, error %v", ag.CurrentAgreementId, err)))
	} else {
		bcType, bcName, bcOrg := cph.GetKnownBlockchain(ag)
		if aph := cph.AgreementProtocolHandler(bcType, bcName, bcOrg); aph == nil {
			glog.Warningf(BCPHlogstring2(workerId, fmt.Sprintf("for %v agreement protocol handler not ready", ag.CurrentAgreementId)))
		} else if err := aph.TerminateAgreement([]policy.Policy{*pol}, ag.CounterPartyAddress, ag.CurrentAgreementId, ag.Org, reason, mt, b.GetSendMessage()); err != nil {
			glog.Errorf(BCPHlogstring2(workerId, fmt.Sprintf("error terminating agreement %v: %v", ag.CurrentAgreementId, err)))
		}
	}
}

func (b *BaseConsumerProtocolHandler) VerifyAgreement(ag *persistence.Agreement, cph ConsumerProtocolHandler) {

	if aph := cph.AgreementProtocolHandler(b.GetKnownBlockchain(ag)); aph == nil {
		glog.Warningf(BCPHlogstring(b.Name(), fmt.Sprintf("for %v agreement protocol handler not ready", ag.CurrentAgreementId)))
	} else if whisperTo, pubkeyTo, err := b.GetDeviceMessageEndpoint(ag.DeviceId, b.Name()); err != nil {
		glog.Errorf(BCPHlogstring(b.Name(), fmt.Sprintf("error obtaining message target for verify message: %v", err)))
	} else if mt, err := exchange.CreateMessageTarget(ag.DeviceId, nil, pubkeyTo, whisperTo); err != nil {
		glog.Errorf(BCPHlogstring(b.Name(), fmt.Sprintf("error creating message target: %v", err)))
	} else if _, err := aph.VerifyAgreement(ag.CurrentAgreementId, "", "", mt, b.GetSendMessage()); err != nil {
		glog.Errorf(BCPHlogstring(b.Name(), fmt.Sprintf("error verifying agreement %v: %v", ag.CurrentAgreementId, err)))
	}

}

func (b *BaseConsumerProtocolHandler) UpdateAgreement(ag *persistence.Agreement, updateType string, metadata interface{}, cph ConsumerProtocolHandler) {

	if aph := cph.AgreementProtocolHandler(b.GetKnownBlockchain(ag)); aph == nil {
		glog.Warningf(BCPHlogstring(b.Name(), fmt.Sprintf("for %v agreement protocol handler not ready", ag.CurrentAgreementId)))
	} else if whisperTo, pubkeyTo, err := b.GetDeviceMessageEndpoint(ag.DeviceId, b.Name()); err != nil {
		glog.Errorf(BCPHlogstring(b.Name(), fmt.Sprintf("error obtaining message target for verify message: %v", err)))
	} else if mt, err := exchange.CreateMessageTarget(ag.DeviceId, nil, pubkeyTo, whisperTo); err != nil {
		glog.Errorf(BCPHlogstring(b.Name(), fmt.Sprintf("error creating message target: %v", err)))
	} else if err := aph.UpdateAgreement(ag.CurrentAgreementId, updateType, metadata, mt, b.GetSendMessage()); err != nil {
		glog.Errorf(BCPHlogstring(b.Name(), fmt.Sprintf("error updating agreement %v: %v", ag.CurrentAgreementId, err)))
	}

}

func (b *BaseConsumerProtocolHandler) GetDeviceMessageEndpoint(deviceId string, workerId string) (string, []byte, error) {

	glog.V(5).Infof(BCPHlogstring2(workerId, fmt.Sprintf("retrieving device %v msg endpoint from exchange", deviceId)))

	if dev, err := b.getDevice(deviceId, workerId); err != nil {
		return "", nil, err
	} else if publicKeyBytes, err := base64.StdEncoding.DecodeString(dev.PublicKey); err != nil {
		return "", nil, errors.New(fmt.Sprintf("Error decoding device publicKey for %s, %v", deviceId, err))
	} else {
		glog.V(5).Infof(BCPHlogstring2(workerId, fmt.Sprintf("retrieved device %v msg endpoint from exchange %v", deviceId, dev.MsgEndPoint)))
		return dev.MsgEndPoint, publicKeyBytes, nil
	}

}

func (b *BaseConsumerProtocolHandler) getDevice(deviceId string, workerId string) (*exchange.Device, error) {

	glog.V(5).Infof(BCPHlogstring2(workerId, fmt.Sprintf("retrieving device %v from exchange", deviceId)))

	var resp interface{}
	resp = new(exchange.GetDevicesResponse)
	targetURL := b.config.AgreementBot.ExchangeURL + "orgs/" + exchange.GetOrg(deviceId) + "/nodes/" + exchange.GetId(deviceId)
	for {
		if err, tpErr := exchange.InvokeExchange(b.config.Collaborators.HTTPClientFactory.NewHTTPClient(nil), "GET", targetURL, b.agbotId, b.token, nil, &resp); err != nil {
			glog.Errorf(BCPHlogstring2(workerId, fmt.Sprintf(err.Error())))
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(BCPHlogstring2(workerId, tpErr.Error()))
			time.Sleep(10 * time.Second)
			continue
		} else {
			devs := resp.(*exchange.GetDevicesResponse).Devices
			if dev, there := devs[deviceId]; !there {
				return nil, errors.New(fmt.Sprintf("device %v not in GET response %v as expected", deviceId, devs))
			} else {
				glog.V(5).Infof(BCPHlogstring2(workerId, fmt.Sprintf("retrieved device %v from exchange %v", deviceId, dev)))
				return &dev, nil
			}
		}
	}
}

func (b *BaseConsumerProtocolHandler) DeferCommand(cmd AgreementWork) {
	b.deferredCommands = append(b.deferredCommands, cmd)
}

func (b *BaseConsumerProtocolHandler) GetDeferredCommands() []AgreementWork {
	res := b.deferredCommands
	b.deferredCommands = make([]AgreementWork, 0, 10)
	return res
}

func (b *BaseConsumerProtocolHandler) UpdateProducer(ag *persistence.Agreement) {
	return
}

func (b *BaseConsumerProtocolHandler) HandleExtensionMessage(cmd *NewProtocolMessageCommand) error {
	return nil
}

func (c *BaseConsumerProtocolHandler) SetBlockchainClientAvailable(ev *events.BlockchainClientInitializedMessage) {
	return
}

func (c *BaseConsumerProtocolHandler) SetBlockchainClientNotAvailable(ev *events.BlockchainClientStoppingMessage) {
	return
}

func (c *BaseConsumerProtocolHandler) AlreadyReceivedReply(ag *persistence.Agreement) bool {
	if ag.CounterPartyAddress != "" {
		return true
	}
	return false
}

func (c *BaseConsumerProtocolHandler) GetKnownBlockchain(ag *persistence.Agreement) (string, string, string) {
	return "", "", ""
}

func (c *BaseConsumerProtocolHandler) CanSendMeterRecord(ag *persistence.Agreement) bool {
	return true
}

func (b *BaseConsumerProtocolHandler) SendEventMessage(event events.Message) {
	if len(b.messages) < int(b.config.GetAgbotAgreementQueueSize()) {
		b.messages <- event
	}
}

// The list of termination reasons that should be supported by all agreement protocols. The caller can pass these into
// the GetTerminationCode API to get a protocol specific reason code for that termination reason.
const TERM_REASON_POLICY_CHANGED = "PolicyChanged"
const TERM_REASON_NOT_FINALIZED_TIMEOUT = "NotFinalized"
const TERM_REASON_NO_DATA_RECEIVED = "NoData"
const TERM_REASON_NO_REPLY = "NoReply"
const TERM_REASON_USER_REQUESTED = "UserRequested"
const TERM_REASON_DEVICE_REQUESTED = "DeviceRequested"
const TERM_REASON_NEGATIVE_REPLY = "NegativeReply"
const TERM_REASON_CANCEL_DISCOVERED = "CancelDiscovered"
const TERM_REASON_CANCEL_FORCED_UPGRADE = "ForceUpgrade"
const TERM_REASON_CANCEL_BC_WRITE_FAILED = "WriteFailed"
const TERM_REASON_NODE_HEARTBEAT = "NodeHeartbeat"
const TERM_REASON_AG_MISSING = "AgreementMissing"

var BCPHlogstring = func(p string, v interface{}) string {
	return fmt.Sprintf("Base Consumer Protocol Handler (%v) %v", p, v)
}

var BCPHlogstring2 = func(workerID string, v interface{}) string {
	return fmt.Sprintf("Base Consumer Protocol Handler (%v): %v", workerID, v)
}
