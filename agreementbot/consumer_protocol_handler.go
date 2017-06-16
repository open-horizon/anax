package agreementbot

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/abstractprotocol"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/metering"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/worker"
	"net/http"
	"time"
)

func CreateConsumerPH(name string, cfg *config.HorizonConfig, db *bolt.DB, pm *policy.PolicyManager) ConsumerProtocolHandler {
	if handler := NewCSProtocolHandler(name, cfg, db, pm); handler != nil {
		return handler
	} else if handler := NewBasicProtocolHandler(name, cfg, db, pm); handler != nil {
		return handler
	} // Add new consumer side protocol handlers here
	return nil
}

type ConsumerProtocolHandler interface {
	Initialize()
	Name() string
	ExchangeId() string
	ExchangeToken() string
	AcceptCommand(cmd worker.Command) bool
	AgreementProtocolHandler() abstractprotocol.ProtocolHandler
	WorkQueue() chan AgreementWork
	DispatchProtocolMessage(cmd *NewProtocolMessageCommand, cph ConsumerProtocolHandler) error
	PersistAgreement(wi *InitiateAgreement, proposal abstractprotocol.Proposal, workerID string) error
	PersistReply(reply abstractprotocol.ProposalReply, pol *policy.Policy, workerID string) error
	HandleAgreementTimeout(cmd *AgreementTimeoutCommand, cph ConsumerProtocolHandler)
	HandleBlockchainEvent(cmd *BlockchainEventCommand)
	HandlePolicyChanged(cmd *PolicyChangedCommand, cph ConsumerProtocolHandler)
	HandlePolicyDeleted(cmd *PolicyDeletedCommand, cph ConsumerProtocolHandler)
	HandleWorkloadUpgrade(cmd *WorkloadUpgradeCommand, cph ConsumerProtocolHandler)
	HandleMakeAgreement(cmd *MakeAgreementCommand, cph ConsumerProtocolHandler)
	GetTerminationCode(reason string) uint
	GetTerminationReason(code uint) string
	GetSendMessage() func(mt interface{}, pay []byte) error
	RecordConsumerAgreementState(agreementId string, workloadID string, state string, workerID string) error
	DeleteMessage(msgId int) error
	CreateMeteringNotification(mp policy.Meter, agreement *Agreement) (*metering.MeteringNotification, error)
	TerminateAgreement(agreement *Agreement, reason uint, workerId string)
	GetDeviceMessageEndpoint(deviceId string, workerId string) (string, []byte, error)
}

type BaseConsumerProtocolHandler struct {
	name       string
	pm         *policy.PolicyManager
	db         *bolt.DB
	config     *config.HorizonConfig
	httpClient *http.Client
	agbotId    string
	token      string
}

func (b *BaseConsumerProtocolHandler) GetSendMessage() func(mt interface{}, pay []byte) error {
	return b.sendMessage
}

func (b *BaseConsumerProtocolHandler) Name() string {
	return b.name
}

func (b *BaseConsumerProtocolHandler) ExchangeId() string {
	return b.agbotId
}

func (b *BaseConsumerProtocolHandler) ExchangeToken() string {
	return b.token
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

	// Grab the exchange ID of the message receiver
	glog.V(3).Infof(BCPHlogstring(w.Name(), fmt.Sprintf("sending exchange message to: %v, message %v", messageTarget.ReceiverExchangeId, string(pay))))

	// Get my own keys
	myPubKey, myPrivKey, _ := exchange.GetKeys(w.config.AgreementBot.MessageKeyPath)

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
		pm := exchange.CreatePostMessage(msgBody, w.config.AgreementBot.ExchangeMessageTTL)
		var resp interface{}
		resp = new(exchange.PostDeviceResponse)
		targetURL := w.config.AgreementBot.ExchangeURL + "devices/" + messageTarget.ReceiverExchangeId + "/msgs"
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

	return nil
}

func (b *BaseConsumerProtocolHandler) DispatchProtocolMessage(cmd *NewProtocolMessageCommand, cph ConsumerProtocolHandler) error {

	glog.V(5).Infof(BCPHlogstring(b.Name(), fmt.Sprintf("received inbound exchange message.")))
	// Figure out what kind of message this is
	if reply, rerr := cph.AgreementProtocolHandler().ValidateReply(string(cmd.Message)); rerr == nil {
		agreementWork := HandleReply{
			workType:     REPLY,
			Reply:        reply,
			SenderId:     cmd.From,
			SenderPubKey: cmd.PubKey,
			MessageId:    cmd.MessageId,
		}
		cph.WorkQueue() <- agreementWork
		glog.V(5).Infof(BCPHlogstring(b.Name(), fmt.Sprintf("queued reply message")))
	} else if _, aerr := cph.AgreementProtocolHandler().ValidateDataReceivedAck(string(cmd.Message)); aerr == nil {
		agreementWork := HandleDataReceivedAck{
			workType:     DATARECEIVEDACK,
			Ack:          string(cmd.Message),
			SenderId:     cmd.From,
			SenderPubKey: cmd.PubKey,
			MessageId:    cmd.MessageId,
		}
		cph.WorkQueue() <- agreementWork
		glog.V(5).Infof(BCPHlogstring(b.Name(), fmt.Sprintf("queued data received ack message")))
	} else if can, aerr := cph.AgreementProtocolHandler().ValidateCancel(string(cmd.Message)); aerr == nil {
		// Before dispatching the cancel to a worker thread, make sure it's a valid cancel
		if ag, err := FindSingleAgreementByAgreementId(b.db, can.AgreementId(), can.Protocol(), []AFilter{}); err != nil {
			glog.Errorf(BCPHlogstring(b.Name(), fmt.Sprintf("error finding agreement %v in the db", can.AgreementId())))
		} else if ag.DeviceId != cmd.From {
			glog.Warningf(BCPHlogstring(b.Name(), fmt.Sprintf("cancel ignored, cancel message for %v came from id %v but agreement is with %v", can.AgreementId(), cmd.From, ag.DeviceId)))
		} else {
			agreementWork := CancelAgreement{
				workType:    CANCEL,
				AgreementId: can.AgreementId(),
				Protocol:    can.Protocol(),
				Reason:      can.Reason(),
			}
			cph.WorkQueue() <- agreementWork
			glog.V(5).Infof(BCPHlogstring(b.Name(), fmt.Sprintf("queued cancel message")))
		}
	} else {
		glog.V(5).Infof(BCPHlogstring(b.Name(), fmt.Sprintf("ignoring  message: %v due to errors: %v or %v", string(cmd.Message), rerr, aerr)))
		return errors.New(BCPHlogstring(b.Name(), fmt.Sprintf("unexpected protocol msg %v", cmd.Message)))
	}
	return nil

}

func (b *BaseConsumerProtocolHandler) HandleAgreementTimeout(cmd *AgreementTimeoutCommand, cph ConsumerProtocolHandler) {

	glog.V(5).Infof(BCPHlogstring(b.Name(), "received agreement cancellation."))
	agreementWork := CancelAgreement{
		workType:    CANCEL,
		AgreementId: cmd.AgreementId,
		Protocol:    cmd.Protocol,
		Reason:      cmd.Reason,
	}
	cph.WorkQueue() <- agreementWork
	glog.V(5).Infof(BCPHlogstring(b.Name(), "queued agreement cancellation"))

}

func (b *BaseConsumerProtocolHandler) HandlePolicyChanged(cmd *PolicyChangedCommand, cph ConsumerProtocolHandler) {

	glog.V(5).Infof(BCPHlogstring(b.Name(), "received policy changed command."))

	if eventPol, err := policy.DemarshalPolicy(cmd.Msg.PolicyString()); err != nil {
		glog.Errorf(BCPHlogstring(b.Name(), fmt.Sprintf("error demarshalling change policy event %v, error: %v", cmd.Msg.PolicyString(), err)))
	} else {

		InProgress := func() AFilter {
			return func(e Agreement) bool { return e.AgreementCreationTime != 0 && e.AgreementTimedout == 0 }
		}

		if agreements, err := FindAgreements(b.db, []AFilter{UnarchivedAFilter(), InProgress()}, cph.Name()); err == nil {
			for _, ag := range agreements {

				if pol, err := policy.DemarshalPolicy(ag.Policy); err != nil {
					glog.Errorf(BCPHlogstring(b.Name(), fmt.Sprintf("unable to demarshal policy for agreement %v, error %v", ag.CurrentAgreementId, err)))

				} else if eventPol.Header.Name != pol.Header.Name {
					// This agreement is using a policy different from the one that changed.
					glog.V(5).Infof(BCPHlogstring(b.Name(), fmt.Sprintf("policy change handler skipping agreement %v because it is using a policy that did not change.", ag.CurrentAgreementId)))
					continue
				} else if err := b.pm.MatchesMine(pol); err != nil {
					glog.Warningf(BCPHlogstring(b.Name(), fmt.Sprintf("agreement %v has a policy %v that has changed: %v", ag.CurrentAgreementId, pol.Header.Name, err)))

					// Remove any workload usage records (non-HA) or mark for pending upgrade (HA). There might not be a workload usage record
					// if the consumer policy does not specify the workload priority section.
					if wlUsage, err := FindSingleWorkloadUsageByDeviceAndPolicyName(b.db, ag.DeviceId, ag.PolicyName); err != nil {
						glog.Warningf(BCPHlogstring(b.Name(), fmt.Sprintf("error retreiving workload usage for %v using policy %v, error: %v", ag.DeviceId, ag.PolicyName, err)))
					} else if wlUsage != nil && len(wlUsage.HAPartners) != 0 && wlUsage.PendingUpgradeTime != 0 {
						// Skip this agreement, it is part of an HA group where another member is upgrading
						continue
					} else if wlUsage != nil && len(wlUsage.HAPartners) != 0 && wlUsage.PendingUpgradeTime == 0 {
						for _, partnerId := range wlUsage.HAPartners {
							if _, err := UpdatePendingUpgrade(b.db, partnerId, ag.PolicyName); err != nil {
								glog.Warningf(BCPHlogstring(b.Name(), fmt.Sprintf("could not update pending workload upgrade for %v using policy %v, error: %v", partnerId, ag.PolicyName, err)))
							}
						}
						// Choose this device's agreement within the HA group to start upgrading.
						// Delete this workload usage record so that a new agreement will be made starting from the highest priority workload
						if err := DeleteWorkloadUsage(b.db, ag.DeviceId, ag.PolicyName); err != nil {
							glog.Warningf(BCPHlogstring(b.Name(), fmt.Sprintf("error deleting workload usage for %v using policy %v, error: %v", ag.DeviceId, ag.PolicyName, err)))
						}
						agreementWork := CancelAgreement{
							workType:    CANCEL,
							AgreementId: ag.CurrentAgreementId,
							Protocol:    ag.AgreementProtocol,
							Reason:      cph.GetTerminationCode(TERM_REASON_POLICY_CHANGED),
						}
						cph.WorkQueue() <- agreementWork
					} else {
						// Non-HA device or agrement without workload priority in the policy, re-make the agreement
						// Delete this workload usage record so that a new agreement will be made starting from the highest priority workload
						if err := DeleteWorkloadUsage(b.db, ag.DeviceId, ag.PolicyName); err != nil {
							glog.Warningf(BCPHlogstring(b.Name(), fmt.Sprintf("error deleting workload usage for %v using policy %v, error: %v", ag.DeviceId, ag.PolicyName, err)))
						}
						agreementWork := CancelAgreement{
							workType:    CANCEL,
							AgreementId: ag.CurrentAgreementId,
							Protocol:    ag.AgreementProtocol,
							Reason:      cph.GetTerminationCode(TERM_REASON_POLICY_CHANGED),
						}
						cph.WorkQueue() <- agreementWork
					}
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

	InProgress := func() AFilter {
		return func(e Agreement) bool { return e.AgreementCreationTime != 0 && e.AgreementTimedout == 0 }
	}

	if agreements, err := FindAgreements(b.db, []AFilter{UnarchivedAFilter(), InProgress()}, cph.Name()); err == nil {
		for _, ag := range agreements {

			if pol, err := policy.DemarshalPolicy(ag.Policy); err != nil {
				glog.Errorf(BCPHlogstring(b.Name(), fmt.Sprintf("unable to demarshal policy for agreement %v, error %v", ag.CurrentAgreementId, err)))
			} else if existingPol := b.pm.GetPolicy(pol.Header.Name); existingPol == nil {
				glog.Errorf(BCPHlogstring(b.Name(), fmt.Sprintf("agreement %v has a policy %v that doesn't exist anymore", ag.CurrentAgreementId, pol.Header.Name)))

				// Remove any workload usage records so that a new agreement will be made starting from the highest priority workload.
				if err := DeleteWorkloadUsage(b.db, ag.DeviceId, ag.PolicyName); err != nil {
					glog.Warningf(BCPHlogstring(b.Name(), fmt.Sprintf("error deleting workload usage for %v using policy %v, error: %v", ag.DeviceId, ag.PolicyName, err)))
				}

				// Queue up a cancellation command for this agreement.
				agreementWork := CancelAgreement{
					workType:    CANCEL,
					AgreementId: ag.CurrentAgreementId,
					Protocol:    ag.AgreementProtocol,
					Reason:      cph.GetTerminationCode(TERM_REASON_POLICY_CHANGED),
				}
				cph.WorkQueue() <- agreementWork

			}

		}
	} else {
		glog.Errorf(BCPHlogstring(b.Name(), fmt.Sprintf("error searching database: %v", err)))
	}
}

func (b *BaseConsumerProtocolHandler) HandleWorkloadUpgrade(cmd *WorkloadUpgradeCommand, cph ConsumerProtocolHandler) {
	glog.V(5).Infof(BCPHlogstring(b.Name(), fmt.Sprintf("received workload upgrade command.")))
	upgradeWork := HandleWorkloadUpgrade{
		workType:    WORKLOAD_UPGRADE,
		AgreementId: cmd.Msg.AgreementId,
		Device:      cmd.Msg.DeviceId,
		Protocol:    cmd.Msg.AgreementProtocol,
		PolicyName:  cmd.Msg.PolicyName,
	}
	cph.WorkQueue() <- upgradeWork
	glog.V(5).Infof(BCPHlogstring(b.Name(), fmt.Sprintf("queued workload upgrade command.")))
}

func (b *BaseConsumerProtocolHandler) HandleMakeAgreement(cmd *MakeAgreementCommand, cph ConsumerProtocolHandler) {
	glog.V(5).Infof(BCPHlogstring(b.Name(), fmt.Sprintf("received make agreement command.")))
	agreementWork := InitiateAgreement{
		workType:       INITIATE,
		ProducerPolicy: cmd.ProducerPolicy,
		ConsumerPolicy: cmd.ConsumerPolicy,
		Device:         cmd.Device,
	}
	cph.WorkQueue() <- agreementWork
	glog.V(5).Infof(BCPHlogstring(b.Name(), fmt.Sprintf("queued make agreement command.")))
}

func (b *BaseConsumerProtocolHandler) PersistBaseAgreement(wi *InitiateAgreement, proposal abstractprotocol.Proposal, workerID string, hash string, sig string) error {

	if polBytes, err := json.Marshal(wi.ConsumerPolicy); err != nil {
		return errors.New(BCPHlogstring2(workerID, fmt.Sprintf("error marshalling policy for storage %v, error: %v", wi.ConsumerPolicy, err)))
	} else if pBytes, err := json.Marshal(proposal); err != nil {
		return errors.New(BCPHlogstring2(workerID, fmt.Sprintf("error marshalling proposal for storage %v, error: %v", proposal, err)))
	} else if pol, err := policy.DemarshalPolicy(proposal.TsAndCs()); err != nil {
		return errors.New(BCPHlogstring2(workerID, fmt.Sprintf("error demarshalling TsandCs policy from pending agreement %v, error: %v", proposal.AgreementId(), err)))
	} else if _, err := AgreementUpdate(b.db, proposal.AgreementId(), string(pBytes), string(polBytes), pol.DataVerify, b.config.AgreementBot.ProcessGovernanceIntervalS, hash, sig, b.Name()); err != nil {
		return errors.New(BCPHlogstring2(workerID, fmt.Sprintf("error updating agreement with proposal %v in DB, error: %v", proposal, err)))

		// Record that the agreement was initiated, in the exchange
	} else if err := b.RecordConsumerAgreementState(proposal.AgreementId(), wi.ConsumerPolicy.APISpecs[0].SpecRef, "Formed Proposal", workerID); err != nil {
		return errors.New(BCPHlogstring2(workerID, fmt.Sprintf("error setting agreement state for %v", proposal.AgreementId())))
	}
	return nil
}

func (b *BaseConsumerProtocolHandler) PersistReply(reply abstractprotocol.ProposalReply, pol *policy.Policy, workerID string) error {

	if _, err := AgreementMade(b.db, reply.AgreementId(), reply.DeviceId(), "", b.Name(), pol.HAGroup.Partners); err != nil {
		return errors.New(BCPHlogstring2(workerID, fmt.Sprintf("error updating agreement %v with reply info in DB, error: %v", reply.AgreementId(), err)))
	}
	return nil

}

func (b *BaseConsumerProtocolHandler) RecordConsumerAgreementState(agreementId string, workloadID string, state string, workerID string) error {

	glog.V(5).Infof(BCPHlogstring2(workerID, fmt.Sprintf("setting agreement %v state to %v", agreementId, state)))

	as := new(exchange.PutAgbotAgreementState)
	as.Workload = workloadID
	as.State = state
	var resp interface{}
	resp = new(exchange.PostDeviceResponse)
	targetURL := b.config.AgreementBot.ExchangeURL + "agbots/" + b.agbotId + "/agreements/" + agreementId
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

func (b *BaseConsumerProtocolHandler) TerminateAgreement(ag *Agreement, reason uint, mt interface{}, workerId string, cph ConsumerProtocolHandler) {
	if pol, err := policy.DemarshalPolicy(ag.Policy); err != nil {
		glog.Errorf(BCPHlogstring2(workerId, fmt.Sprintf("unable to demarshal policy while trying to cancel %v, error %v", ag.CurrentAgreementId, err)))
	} else if err := cph.AgreementProtocolHandler().TerminateAgreement(pol, ag.CounterPartyAddress, ag.CurrentAgreementId, reason, mt, b.GetSendMessage()); err != nil {
		glog.Errorf(BCPHlogstring2(workerId, fmt.Sprintf("error terminating agreement %v on the blockchain: %v", ag.CurrentAgreementId, err)))
	}
}

func (b *BaseConsumerProtocolHandler) GetDeviceMessageEndpoint(deviceId string, workerId string) (string, []byte, error) {

	glog.V(5).Infof(BCPHlogstring2(workerId, fmt.Sprintf("retrieving device %v msg endpoint from exchange", deviceId)))

	if dev, err := b.getDevice(deviceId, workerId); err != nil {
		return "", nil, err
	} else {
		glog.V(5).Infof(BCPHlogstring2(workerId, fmt.Sprintf("retrieved device %v msg endpoint from exchange %v", deviceId, dev.MsgEndPoint)))
		return dev.MsgEndPoint, dev.PublicKey, nil
	}

}

func (b *BaseConsumerProtocolHandler) getDevice(deviceId string, workerId string) (*exchange.Device, error) {

	glog.V(5).Infof(BCPHlogstring2(workerId, fmt.Sprintf("retrieving device %v from exchange", deviceId)))

	var resp interface{}
	resp = new(exchange.GetDevicesResponse)
	targetURL := b.config.AgreementBot.ExchangeURL + "devices/" + deviceId
	for {
		if err, tpErr := exchange.InvokeExchange(&http.Client{Timeout: time.Duration(config.HTTPDEFAULTTIMEOUT*time.Millisecond)}, "GET", targetURL, b.agbotId, b.token, nil, &resp); err != nil {
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

var BCPHlogstring = func(p string, v interface{}) string {
	return fmt.Sprintf("Base Consumer Protocol Handler (%v) %v", p, v)
}

var BCPHlogstring2 = func(workerID string, v interface{}) string {
	return fmt.Sprintf("Base Consumer Protocol Handler (%v): %v", workerID, v)
}
