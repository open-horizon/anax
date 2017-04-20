package governance

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/citizenscientist"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/device"
	"github.com/open-horizon/anax/ethblockchain"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/metering"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/worker"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// TODO: make this module more aware of long-running setup operations like torrent downloading and dockerfile loading
// the max time we'll let a contract remain unconfigured by the provider
const MAX_CONTRACT_UNCONFIGURED_TIME_M = 20

const MAX_CONTRACT_PRELAUNCH_TIME_M = 10

const MAX_MICROPAYMENT_UNPAID_RUN_DURATION_M = 60

// enforced only after the workloads are running
const MAX_AGREEMENT_ACCEPTANCE_WAIT_TIME_M = 20

// related to agreement cleanup status
const STATUS_WORKLOAD_DESTROYED = 500
const STATUS_AG_PROTOCOL_TERMINATED = 501

type GovernanceWorker struct {
	worker.Worker   // embedded field
	db              *bolt.DB
	httpClient      *http.Client
	bc              *ethblockchain.BaseContracts
	deviceId        string
	deviceToken     string
	pm              *policy.PolicyManager
	bcReady         bool // This field will be turned to true when the blockchain client is running
	bcWritesEnabled bool // This field will be turned to true when the blockchain account has ether, which means
	// block chain writes (cancellations) can be done.
}

func NewGovernanceWorker(config *config.HorizonConfig, db *bolt.DB, pm *policy.PolicyManager) *GovernanceWorker {
	messages := make(chan events.Message)
	commands := make(chan worker.Command, 200)

	id, _ := device.Id()

	token := ""
	if dev, _ := persistence.FindExchangeDevice(db); dev != nil {
		token = dev.Token
	}

	worker := &GovernanceWorker{

		Worker: worker.Worker{
			Manager: worker.Manager{
				Config:   config,
				Messages: messages,
			},

			Commands: commands,
		},

		db:              db,
		httpClient:      &http.Client{},
		pm:              pm,
		deviceId:        id,
		deviceToken:     token,
		bcReady:         false,
		bcWritesEnabled: false,
	}

	worker.start()
	return worker
}

func (w *GovernanceWorker) Messages() chan events.Message {
	return w.Worker.Manager.Messages
}

func (w *GovernanceWorker) NewEvent(incoming events.Message) {

	switch incoming.(type) {
	case *events.EdgeRegisteredExchangeMessage:
		msg, _ := incoming.(*events.EdgeRegisteredExchangeMessage)
		w.deviceToken = msg.Token()

	case *events.WorkloadMessage:
		msg, _ := incoming.(*events.WorkloadMessage)

		switch msg.Event().Id {
		case events.EXECUTION_BEGUN:
			glog.Infof("Begun execution of containers according to agreement %v", msg.AgreementId)

			cmd := w.NewStartGovernExecutionCommand(msg.Deployment, msg.AgreementProtocol, msg.AgreementId)
			w.Commands <- cmd
		case events.EXECUTION_FAILED:
			cmd := w.NewCleanupExecutionCommand(msg.AgreementProtocol, msg.AgreementId, citizenscientist.CANCEL_CONTAINER_FAILURE, msg.Deployment)
			w.Commands <- cmd
		case events.WORKLOAD_DESTROYED:
			cmd := w.NewCleanupStatusCommand(msg.AgreementProtocol, msg.AgreementId, STATUS_WORKLOAD_DESTROYED)
			w.Commands <- cmd
		}

	case *events.TorrentMessage:
		msg, _ := incoming.(*events.TorrentMessage)
		switch msg.Event().Id {
		case events.TORRENT_FAILURE:
			switch msg.LaunchContext.(type) {
			case events.AgreementLaunchContext:
				lc := msg.LaunchContext.(events.AgreementLaunchContext)
				cmd := w.NewCleanupExecutionCommand(lc.AgreementProtocol, lc.AgreementId, citizenscientist.CANCEL_TORRENT_FAILURE, nil)
				w.Commands <- cmd
			}
		}
	case *events.InitAgreementCancelationMessage:
		msg, _ := incoming.(*events.InitAgreementCancelationMessage)
		switch msg.Event().Id {
		case events.AGREEMENT_ENDED:
			cmd := w.NewCleanupExecutionCommand(msg.AgreementProtocol, msg.AgreementId, msg.Reason, msg.Deployment)
			w.Commands <- cmd
		}
	case *events.ApiAgreementCancelationMessage:
		msg, _ := incoming.(*events.ApiAgreementCancelationMessage)
		switch msg.Event().Id {
		case events.AGREEMENT_ENDED:
			cmd := w.NewCleanupExecutionCommand(msg.AgreementProtocol, msg.AgreementId, citizenscientist.CANCEL_USER_REQUESTED, msg.Deployment)
			w.Commands <- cmd
		}
	case *events.BlockchainClientInitializedMessage:
		msg, _ := incoming.(*events.BlockchainClientInitializedMessage)
		switch msg.Event().Id {
		case events.BC_CLIENT_INITIALIZED:
			w.bcReady = true
		}
	case *events.AccountFundedMessage:
		msg, _ := incoming.(*events.AccountFundedMessage)
		switch msg.Event().Id {
		case events.ACCOUNT_FUNDED:
			w.bcWritesEnabled = true
		}
	case *events.EthBlockchainEventMessage:
		msg, _ := incoming.(*events.EthBlockchainEventMessage)
		switch msg.Event().Id {
		case events.BC_EVENT:
			cmd := NewBlockchainEventCommand(*msg)
			w.Commands <- cmd
		}
	case *events.ExchangeDeviceMessage:
		msg, _ := incoming.(*events.ExchangeDeviceMessage)
		switch msg.Event().Id {
		case events.RECEIVED_EXCHANGE_DEV_MSG:
			cmd := NewExchangeMessageCommand(*msg)
			w.Commands <- cmd
		}

	default: //nothing
	}

	glog.V(4).Infof(logString(fmt.Sprintf("command channel length %v added", len(w.Commands))))

	return
}

func (w *GovernanceWorker) governAgreements(protocolHandler *citizenscientist.ProtocolHandler) {

	glog.V(3).Infof(logString(fmt.Sprintf("governing pending agreements")))

	// Create a new filter for unfinalized agreements
	notYetFinalFilter := func() persistence.EAFilter {
		return func(a persistence.EstablishedAgreement) bool {
			return a.AgreementCreationTime != 0 && a.AgreementTerminatedTime == 0
		}
	}

	if establishedAgreements, err := persistence.FindEstablishedAgreements(w.db, citizenscientist.PROTOCOL_NAME, []persistence.EAFilter{persistence.UnarchivedEAFilter(), notYetFinalFilter()}); err != nil {
		glog.Errorf(logString(fmt.Sprintf("Unable to retrieve not yet final agreements from database: %v. Error: %v", err, err)))
	} else {

		// If there are agreemens in the database then we will assume that the device is already registered
		for _, ag := range establishedAgreements {
			if ag.AgreementFinalizedTime == 0 {
				// Cancel the agreement if finalization doesn't occur before the timeout
				glog.V(5).Infof(logString(fmt.Sprintf("checking agreement %v for finalization.", ag.CurrentAgreementId)))

				// Check to see if the agreement is in the blockchain. This call to the blockchain should be very fast.
				// The device might have been down for some time and/or restarted, causing it to miss events on the blockchain.
				if recorded, err := protocolHandler.VerifyAgreementRecorded(ag.CurrentAgreementId, ag.CounterPartyAddress, ag.ProposalSig, w.bc.Agreements); err != nil {
					glog.Errorf(logString(fmt.Sprintf("encountered error verifying agreement %v on blockchain, error %v", ag.CurrentAgreementId, err)))
				} else if recorded {
					if err := w.finalizeAgreement(ag, protocolHandler); err != nil {
						glog.Errorf(err.Error())
					}
				} else {

					// Not in the blockchain yet, check for a timeout
					now := uint64(time.Now().Unix())
					if ag.AgreementCreationTime+w.Worker.Manager.Config.Edge.AgreementTimeoutS < now {
						// Start timing out the agreement
						glog.V(3).Infof(logString(fmt.Sprintf("detected agreement %v timed out.", ag.CurrentAgreementId)))

						reason := uint(citizenscientist.CANCEL_NOT_FINALIZED_TIMEOUT)
						if ag.AgreementAcceptedTime == 0 {
							reason = citizenscientist.CANCEL_NO_REPLY_ACK
						}
						w.cancelAgreement(ag.CurrentAgreementId, ag.AgreementProtocol, reason, citizenscientist.DecodeReasonCode(uint64(reason)))

						// cleanup workloads
						w.Messages() <- events.NewGovernanceCancelationMessage(events.AGREEMENT_ENDED, events.AG_TERMINATED, ag.AgreementProtocol, ag.CurrentAgreementId, ag.CurrentDeployment)
					}
				}
			} else {
				// For finalized agreements, make sure the workload has been started in time
				if ag.AgreementExecutionStartTime == 0 {
					// workload not started yet and in an agreement ...
					if (int64(ag.AgreementAcceptedTime) + (MAX_CONTRACT_PRELAUNCH_TIME_M * 60)) < time.Now().Unix() {
						glog.Infof(logString(fmt.Sprintf("terminating agreement %v because it hasn't been launched in max allowed time. This could be because of a workload failure.", ag.CurrentAgreementId)))
						w.cancelAgreement(ag.CurrentAgreementId, ag.AgreementProtocol, citizenscientist.CANCEL_NOT_EXECUTED_TIMEOUT, citizenscientist.DecodeReasonCode(citizenscientist.CANCEL_NOT_EXECUTED_TIMEOUT))
						// cleanup workloads if needed
						w.Messages() <- events.NewGovernanceCancelationMessage(events.AGREEMENT_ENDED, events.AG_TERMINATED, ag.AgreementProtocol, ag.CurrentAgreementId, ag.CurrentDeployment)
					}
				}
			}
		}
	}
}

func (w *GovernanceWorker) governContainers() {

	// go govern
	go func() {

		for {
			glog.V(4).Infof(logString(fmt.Sprintf("governing containers")))

			// Create a new filter for unfinalized agreements
			runningFilter := func() persistence.EAFilter {
				return func(a persistence.EstablishedAgreement) bool {
					return a.AgreementExecutionStartTime != 0 && a.AgreementTerminatedTime == 0 && a.CounterPartyAddress != ""
				}
			}

			if establishedAgreements, err := persistence.FindEstablishedAgreements(w.db, citizenscientist.PROTOCOL_NAME, []persistence.EAFilter{persistence.UnarchivedEAFilter(), runningFilter()}); err != nil {
				glog.Errorf(logString(fmt.Sprintf("Unable to retrieve running agreements from database, error: %v", err)))
			} else {

				for _, ag := range establishedAgreements {

					// Make sure containers are still running.
					glog.V(3).Infof(logString(fmt.Sprintf("fire event to ensure containers are still up for agreement %v.", ag.CurrentAgreementId)))

					// current contract, ensure workloads still running
					w.Messages() <- events.NewGovernanceMaintenanceMessage(events.CONTAINER_MAINTAIN, ag.AgreementProtocol, ag.CurrentAgreementId, ag.CurrentDeployment)

				}
			}

			time.Sleep(1 * time.Minute)
		}
	}()
}

// It cancels the given agreement. Please take note that the system is very asynchronous. It is
// possible for multiple cancellations to occur in the time it takes to actually stop workloads and
// cancel on the blockchain, therefore this code needs to be prepared to run multiple times for the
// same agreement id.
func (w *GovernanceWorker) cancelAgreement(agreementId string, agreementProtocol string, reason uint, desc string) {
	protocolHandler := citizenscientist.NewProtocolHandler(w.Config.Edge.GethURL, w.pm)

	// Update the database
	var ag *persistence.EstablishedAgreement
	if agreement, err := persistence.AgreementStateTerminated(w.db, agreementId, uint64(reason), desc, agreementProtocol); err != nil {
		glog.Errorf(logString(fmt.Sprintf("error marking agreement %v terminated: %v", agreementId, err)))
	} else {
		ag = agreement
	}

	// Delete from the exchange
	if ag != nil && ag.AgreementAcceptedTime != 0 {
		if err := deleteProducerAgreement(w.Config.Edge.ExchangeURL, w.deviceId, w.deviceToken, agreementId); err != nil {
			glog.Errorf(logString(fmt.Sprintf("error deleting agreement %v in exchange: %v", agreementId, err)))
		}
	}

	// Put the rest of the cancel processing into it's own go routine. Most of the time, this go routine will
	// be waiting for the blockchain cancel to run. In general it will take around 30 seconds, but could be
	// double or triple that time. This will free up the governance thread to handle protocol messages.
	go func() {

		// Get the policy we used in the agreement and then cancel on the blockchain, just in case.
		glog.V(3).Infof(logString(fmt.Sprintf("terminating agreement %v on blockchain.", agreementId)))

		if ag != nil && w.bcWritesEnabled == true {
			if proposal, err := protocolHandler.DemarshalProposal(ag.Proposal); err != nil {
				glog.Errorf(logString(fmt.Sprintf("error demarshalling agreement %v proposal: %v", agreementId, err)))
			} else if pPolicy, err := policy.DemarshalPolicy(proposal.ProducerPolicy); err != nil {
				glog.Errorf(logString(fmt.Sprintf("error demarshalling agreement %v Producer Policy: %v", agreementId, err)))
			} else if err := protocolHandler.TerminateAgreement(pPolicy, ag.CounterPartyAddress, agreementId, reason, w.bc.Agreements); err != nil {
				glog.Errorf(logString(fmt.Sprintf("error terminating agreement %v on the blockchain: %v", agreementId, err)))
			}

		}

		// report the cleanup status
		cmd := w.NewCleanupStatusCommand(agreementProtocol, agreementId, STATUS_AG_PROTOCOL_TERMINATED)
		w.Commands <- cmd
	}()

	// Concurrently write out the metering record. This is done in its own go routine for the same reason that
	// the blockchain cancel is done in a separate go routine. This go routine can complete after the agreement is
	// archived without any side effects.
	go func() {
		// If there are metering notifications, write them onto the blockchain also
		if ag.MeteringNotificationMsg != (persistence.MeteringNotification{}) {
			if mn := metering.ConvertFromPersistent(ag.MeteringNotificationMsg, agreementId); mn == nil {
				glog.Errorf(logString(fmt.Sprintf("error converting from persistent Metering Notification %v for %v, returned nil.", ag.MeteringNotificationMsg, agreementId)))
			} else if err := protocolHandler.RecordMeter(agreementId, mn, w.bc.Metering); err != nil {
				glog.Errorf(logString(fmt.Sprintf("error writing meter %v for agreement %v on the blockchain: %v", ag.MeteringNotificationMsg, agreementId, err)))
			}
		}
	}()

}

func (w *GovernanceWorker) start() {
	go func() {

		sendMessage := func(mt interface{}, pay []byte) error {
			// The mt parameter is an abstract message target object that is passed to this routine
			// by the agreement protocol. It's na interface{} type so that we can avoid the protocol knowing
			// about non protocol types.

			var messageTarget *exchange.ExchangeMessageTarget
			switch mt.(type) {
			case *exchange.ExchangeMessageTarget:
				messageTarget = mt.(*exchange.ExchangeMessageTarget)
			default:
				return errors.New(fmt.Sprintf("input message target is %T, expecting exchange.MessageTarget", mt))
			}

			// If the message target is using whisper, then send via whisper
			if len(messageTarget.ReceiverMsgEndPoint) != 0 {
				return errors.New(fmt.Sprintf("Message target should never be whisper, %v", messageTarget))

				// The message target is using the exchange message queue, so use it
			} else {

				// Grab the exchange ID of the message receiver
				glog.V(3).Infof("Sending exchange message to: %v, message %v", messageTarget.ReceiverExchangeId, string(pay))

				// Get my own keys
				myPubKey, myPrivKey, _ := exchange.GetKeys("")

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
					return errors.New(fmt.Sprintf("Unable to construct encrypted message from %v, error %v", pay, err))
					// Marshal it into a byte array
				} else if msgBody, err := json.Marshal(encryptedMsg); err != nil {
					return errors.New(fmt.Sprintf("Unable to marshal exchange message %v, error %v", encryptedMsg, err))
					// Send it to the device's message queue
				} else {
					pm := exchange.CreatePostMessage(msgBody, w.Worker.Manager.Config.Edge.ExchangeMessageTTL)
					var resp interface{}
					resp = new(exchange.PostDeviceResponse)
					targetURL := w.Worker.Manager.Config.Edge.ExchangeURL + "agbots/" + messageTarget.ReceiverExchangeId + "/msgs"
					for {
						if err, tpErr := exchange.InvokeExchange(w.httpClient, "POST", targetURL, w.deviceId, w.deviceToken, pm, &resp); err != nil {
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
			}
			return nil
		}

		// Fire up the eth container after the device is registered.
		for {
			if w.deviceToken != "" {
				break
			} else {
				glog.V(3).Infof("GovernanceWorker command processor waiting for device registration")
				time.Sleep(time.Duration(5) * time.Second)
			}
		}

		// Tell the eth worker to start the ethereum client container.
		w.Worker.Manager.Messages <- events.NewNewEthContainerMessage(events.NEW_ETH_CLIENT, w.Worker.Manager.Config.Edge.ExchangeURL, w.deviceId, w.deviceToken)

		// Hold the governance functions until we have blockchain funding. If there are events occurring that
		// we need to react to, they will queue up on the command queue while we wait here. The agreement worker
		// should not be blocked by this.
		for {
			if w.bcReady == false {
				time.Sleep(time.Duration(5) * time.Second)
				glog.V(3).Infof("GovernanceWorker command processor waiting for ethereum")
			} else {
				break
			}
		}

		// Establish the go objects that are used to interact with the ethereum blockchain.
		// This code should probably be in the protocol library.
		acct, _ := ethblockchain.AccountId()
		dir, _ := ethblockchain.DirectoryAddress()
		if bc, err := ethblockchain.InitBaseContracts(acct, w.Worker.Manager.Config.Edge.GethURL, dir); err != nil {
			glog.Errorf(logString(fmt.Sprintf("unable to initialize platform contracts, error: %v", err)))
			return
		} else {
			w.bc = bc
		}

		// Fire up the container governor
		w.governContainers()

		protocolHandler := citizenscientist.NewProtocolHandler(w.Config.Edge.GethURL, w.pm)

		deferredCommands := make([]worker.Command, 0, 10)

		// Fire up the command processor
		for {

			glog.V(3).Infof("GovernanceWorker command processor about to select command (non-blocking)")

			select {
			case command := <-w.Commands:
				glog.V(2).Infof("GovernanceWorker received command: %v", command.ShortString())
				glog.V(5).Infof("GovernanceWorker received command: %v", command)
				glog.V(4).Infof(logString(fmt.Sprintf("command channel length %v removed", len(w.Commands))))

				// TODO: consolidate DB update cases
				switch command.(type) {
				case *StartGovernExecutionCommand:
					// TODO: update db start time and tc so it can be governed
					cmd, _ := command.(*StartGovernExecutionCommand)
					glog.V(3).Infof("Starting governance on resources in agreement: %v", cmd.AgreementId)

					if _, err := persistence.AgreementStateExecutionStarted(w.db, cmd.AgreementId, cmd.AgreementProtocol, &cmd.Deployment); err != nil {
						glog.Errorf("Failed to update local contract record to start governing Agreement: %v. Error: %v", cmd.AgreementId, err)
					}

				case *CleanupExecutionCommand:
					cmd, _ := command.(*CleanupExecutionCommand)

					agreementId := cmd.AgreementId
					if ags, err := persistence.FindEstablishedAgreements(w.db, citizenscientist.PROTOCOL_NAME, []persistence.EAFilter{persistence.UnarchivedEAFilter(), persistence.IdEAFilter(agreementId)}); err != nil {
						glog.Errorf(logString(fmt.Sprintf("unable to retrieve agreement %v from database, error %v", agreementId, err)))
					} else if len(ags) != 1 {
						glog.V(5).Infof(logString(fmt.Sprintf("ignoring the event, unable to retrieve unarchived single agreement %v from the database.", agreementId)))
					} else if ags[0].AgreementTerminatedTime != 0 && ags[0].AgreementForceTerminatedTime == 0 {
						glog.V(3).Infof(logString(fmt.Sprintf("ignoring the event, agreement %v is already terminating", agreementId)))
					} else if w.bcWritesEnabled == true {
						glog.V(3).Infof("Ending the agreement: %v", agreementId)
						w.cancelAgreement(agreementId, cmd.AgreementProtocol, cmd.Reason, citizenscientist.DecodeReasonCode(uint64(cmd.Reason)))

						// send the event to the container in case it has started the workloads.
						w.Messages() <- events.NewGovernanceCancelationMessage(events.AGREEMENT_ENDED, events.AG_TERMINATED, cmd.AgreementProtocol, agreementId, cmd.Deployment)
					} else {
						// Requeue
						glog.V(3).Infof("Deferring ending the agreement: %v", agreementId)
						deferredCommands = append(deferredCommands, cmd)
					}

				case *ExchangeMessageCommand:
					cmd, _ := command.(*ExchangeMessageCommand)

					exchangeMsg := new(exchange.DeviceMessage)
					if err := json.Unmarshal(cmd.Msg.ExchangeMessage(), &exchangeMsg); err != nil {
						glog.Errorf(logString(fmt.Sprintf("unable to demarshal exchange device message %v, error %v", cmd.Msg.ExchangeMessage(), err)))
						continue
					} else if there, err := w.messageInExchange(exchangeMsg.MsgId); err != nil {
						glog.Errorf(logString(fmt.Sprintf("unable to get messages from the exchange, error %v", err)))
						continue
					} else if !there {
						glog.V(3).Infof(logString(fmt.Sprintf("ignoring message %v, already deleted from the exchange.", exchangeMsg.MsgId)))
						continue
					}

					glog.V(3).Infof(logString(fmt.Sprintf("received message %v from the exchange", exchangeMsg.MsgId)))

					deleteMessage := false
					// ReplyAck messages could indicate that the agbot has decided not to pursue the agreement any longer.
					if replyAck, err := protocolHandler.ValidateReplyAck(cmd.Msg.ProtocolMessage()); err != nil {
						glog.Warningf(logString(fmt.Sprintf("ReplyAck handler ignoring non-reply ack message: %v due to %v", cmd.Msg.ShortProtocolMessage(), err)))
					} else if ags, err := persistence.FindEstablishedAgreements(w.db, citizenscientist.PROTOCOL_NAME, []persistence.EAFilter{persistence.UnarchivedEAFilter(), persistence.IdEAFilter(replyAck.AgreementId())}); err != nil {
						glog.Errorf(logString(fmt.Sprintf("unable to retrieve agreement %v from database, error %v", replyAck.AgreementId(), err)))
					} else if len(ags) != 1 {
						glog.Warningf(logString(fmt.Sprintf("unable to retrieve single agreement %v from database.", replyAck.AgreementId())))
						deleteMessage = true
					} else if ags[0].AgreementAcceptedTime != 0 || ags[0].AgreementTerminatedTime != 0 {
						glog.Errorf(logString(fmt.Sprintf("ignoring replyack for %v because we already received one or are cancelling", replyAck.AgreementId())))
						deleteMessage = true
					} else if replyAck.ReplyAgreementStillValid() {
						if proposal, err := protocolHandler.DemarshalProposal(ags[0].Proposal); err != nil {
							glog.Errorf(logString(fmt.Sprintf("unable to demarshal proposal for agreement %v from database", replyAck.AgreementId())))
						} else if err := w.RecordReply(proposal, citizenscientist.PROTOCOL_NAME); err != nil {
							glog.Errorf(logString(fmt.Sprintf("unable to record reply %v, error: %v", *replyAck, err)))
						} else {
							deleteMessage = true
						}
					} else {
						deleteMessage = true
						if w.bcWritesEnabled == true {
							w.cancelAgreement(replyAck.AgreementId(), citizenscientist.PROTOCOL_NAME, citizenscientist.CANCEL_AGBOT_REQUESTED, citizenscientist.DecodeReasonCode(uint64(citizenscientist.CANCEL_AGBOT_REQUESTED)))
							// There is no need to send an event to stop workload containers because they dont get started until we get a positive reply ack
						} else {
							glog.Infof(logString(fmt.Sprintf("deferring termination of agreement %v until the device is funded.", ags[0].CurrentAgreementId)))
							deferredCommands = append(deferredCommands, cmd)
						}
					}

					// Data notification message indicates that the agbot has found that data is being received from the workload.
					if dataReceived, err := protocolHandler.ValidateDataReceived(cmd.Msg.ProtocolMessage()); err != nil {
						glog.Warningf(logString(fmt.Sprintf("DataReceived handler ignoring non-data received message: %v due to %v", cmd.Msg.ShortProtocolMessage(), err)))
					} else if ags, err := persistence.FindEstablishedAgreements(w.db, citizenscientist.PROTOCOL_NAME, []persistence.EAFilter{persistence.UnarchivedEAFilter(), persistence.IdEAFilter(dataReceived.AgreementId())}); err != nil {
						glog.Errorf(logString(fmt.Sprintf("unable to retrieve agreement %v from database, error %v", dataReceived.AgreementId(), err)))
					} else if len(ags) != 1 {
						glog.Warningf(logString(fmt.Sprintf("unable to retrieve single agreement %v from database, error %v", dataReceived.AgreementId(), err)))
						deleteMessage = true
					} else if _, err := persistence.AgreementStateDataReceived(w.db, dataReceived.AgreementId(), citizenscientist.PROTOCOL_NAME); err != nil {
						glog.Errorf(logString(fmt.Sprintf("unable to update data received time for %v, error: %v", dataReceived.AgreementId(), err)))
					} else if messageTarget, err := exchange.CreateMessageTarget(exchangeMsg.AgbotId, nil, exchangeMsg.AgbotPubKey, ""); err != nil {
						glog.Errorf(logString(fmt.Sprintf("error creating message target: %v", err)))
					} else if err := protocolHandler.NotifyDataReceiptAck(dataReceived.AgreementId(), messageTarget, sendMessage); err != nil {
						glog.Errorf(logString(fmt.Sprintf("unable to send data received ack for %v, error: %v", dataReceived.AgreementId(), err)))
					} else {
						deleteMessage = true
					}

					// Metering notification messages indicate that the agbot is metering data sent to the data ingest.
					if mnReceived, err := protocolHandler.ValidateMeterNotification(cmd.Msg.ProtocolMessage()); err != nil {
						glog.Warningf(logString(fmt.Sprintf("Meter Notification handler ignoring non-metering message: %v due to %v", cmd.Msg.ShortProtocolMessage(), err)))
					} else if ags, err := persistence.FindEstablishedAgreements(w.db, citizenscientist.PROTOCOL_NAME, []persistence.EAFilter{persistence.UnarchivedEAFilter(), persistence.IdEAFilter(mnReceived.AgreementId())}); err != nil {
						glog.Errorf(logString(fmt.Sprintf("unable to retrieve agreement %v from database, error %v", mnReceived.AgreementId(), err)))
					} else if len(ags) != 1 {
						glog.Warningf(logString(fmt.Sprintf("unable to retrieve single agreement %v from database, error %v", mnReceived.AgreementId(), err)))
						deleteMessage = true
					} else if ags[0].AgreementTerminatedTime != 0 {
						glog.Warningf(logString(fmt.Sprintf("ignoring metering notification, agreement %v is terminating", mnReceived.AgreementId())))
						deleteMessage = true
					} else if mn, err := metering.ConvertToPersistent(mnReceived.Meter()); err != nil {
						glog.Errorf(logString(fmt.Sprintf("unable to convert metering notification string %v to persistent metering notification for %v, error: %v", mnReceived.Meter(), mnReceived.AgreementId(), err)))
						deleteMessage = true
					} else if _, err := persistence.MeteringNotificationReceived(w.db, mnReceived.AgreementId(), *mn, citizenscientist.PROTOCOL_NAME); err != nil {
						glog.Errorf(logString(fmt.Sprintf("unable to update metering notification for %v, error: %v", mnReceived.AgreementId(), err)))
						deleteMessage = true
					} else {
						deleteMessage = true
					}

					// Get rid of the exchange message when we're done with it
					if deleteMessage {
						if err := w.deleteMessage(exchangeMsg); err != nil {
							glog.Errorf(logString(fmt.Sprintf("error deleting exchange message %v, error %v", exchangeMsg.MsgId, err)))
						}
					}

				case *BlockchainEventCommand:
					cmd, _ := command.(*BlockchainEventCommand)

					// Unmarshal the raw event
					if rawEvent, err := protocolHandler.DemarshalEvent(cmd.Msg.RawEvent()); err != nil {
						glog.Errorf(logString(fmt.Sprintf("unable to demarshal raw event %v, error: %v", cmd.Msg.RawEvent(), err)))

						// If the event is a consumer termination event
					} else if protocolHandler.ConsumerTermination(rawEvent) {
						// Grab the agreement id from the event
						agreementId := protocolHandler.GetAgreementId(rawEvent)

						// If we have that agreement in our DB, then cancel it
						if ags, err := persistence.FindEstablishedAgreements(w.db, citizenscientist.PROTOCOL_NAME, []persistence.EAFilter{persistence.UnarchivedEAFilter(), persistence.IdEAFilter(agreementId)}); err != nil {
							glog.Errorf(logString(fmt.Sprintf("unable to retrieve agreement %v from database, error %v", agreementId, err)))
						} else if len(ags) != 1 {
							glog.V(5).Infof(logString(fmt.Sprintf("ignoring event, not our agreement id")))
						} else if ags[0].AgreementTerminatedTime != 0 {
							glog.V(5).Infof(logString(fmt.Sprintf("ignoring event, agreement %v is already terminating", ags[0].CurrentAgreementId)))
						} else if reason, err := protocolHandler.GetReasonCode(rawEvent); err != nil {
							glog.Errorf(logString(fmt.Sprintf("unable to retrieve reason code from %v, error %v", rawEvent, err)))
						} else {
							if w.bcWritesEnabled == true {
								glog.Infof(logString(fmt.Sprintf("terminating agreement %v because it has been cancelled on the blockchain.", ags[0].CurrentAgreementId)))
								w.cancelAgreement(ags[0].CurrentAgreementId, ags[0].AgreementProtocol, uint(reason), citizenscientist.DecodeReasonCode(reason))
								// cleanup workloads if needed
								w.Messages() <- events.NewGovernanceCancelationMessage(events.AGREEMENT_ENDED, events.AG_TERMINATED, ags[0].AgreementProtocol, ags[0].CurrentAgreementId, ags[0].CurrentDeployment)
							} else {
								glog.Infof(logString(fmt.Sprintf("deferring termination of agreement %v until the device is funded.", ags[0].CurrentAgreementId)))
								deferredCommands = append(deferredCommands, cmd)
							}
						}

						// If the event is an agreement created event
					} else if protocolHandler.AgreementCreated(rawEvent) {
						// Grab the agreement id from the event
						agreementId := protocolHandler.GetAgreementId(rawEvent)

						// If we have that agreement in our DB and it's not already terminating, then finalize it
						if ags, err := persistence.FindEstablishedAgreements(w.db, citizenscientist.PROTOCOL_NAME, []persistence.EAFilter{persistence.UnarchivedEAFilter(), persistence.IdEAFilter(agreementId)}); err != nil {
							glog.Errorf(logString(fmt.Sprintf("unable to retrieve agreement %v from database, error %v", agreementId, err)))
						} else if len(ags) != 1 {
							glog.V(5).Infof(logString(fmt.Sprintf("ignoring event, not our agreement id")))
						} else if ags[0].AgreementTerminatedTime != 0 {
							glog.V(5).Infof(logString(fmt.Sprintf("ignoring event, agreement %v is terminating", ags[0].CurrentAgreementId)))

						// Finalize the agreement
						} else if err := w.finalizeAgreement(ags[0], protocolHandler); err != nil {
							glog.Errorf(err.Error())
						}
					} else {
						glog.V(5).Infof(logString(fmt.Sprintf("ignoring event")))
					}

				case *CleanupStatusCommand:
					cmd, _ := command.(*CleanupStatusCommand)

					glog.V(5).Infof(logString(fmt.Sprintf("Received CleanupStatusCommand: %v.", cmd)))
					if ags, err := persistence.FindEstablishedAgreements(w.db, cmd.AgreementProtocol, []persistence.EAFilter{persistence.UnarchivedEAFilter(), persistence.IdEAFilter(cmd.AgreementId)}); err != nil {
						glog.Errorf(logString(fmt.Sprintf("unable to retrieve agreement %v from database, error %v", cmd.AgreementId, err)))
					} else if len(ags) != 1 {
						glog.V(5).Infof(logString(fmt.Sprintf("ignoring event, not our agreement id")))
					} else if ags[0].AgreementAcceptedTime == 0 {
						// The only place the agreement is known is in the DB, so we can just delete the record. In the situation where
						// the agbot changes its mind about the proposal, we don't want to create an archived agreement because an
						// agreement was never really established.
						if err := persistence.DeleteEstablishedAgreement(w.db, cmd.AgreementId, cmd.AgreementProtocol); err != nil {
							glog.Errorf(logString(fmt.Sprintf("unable to delete record for agreement %v, error: %v", cmd.AgreementId, err)))
						}
					} else {
						// writes the cleanup status into the db
						var archive = false
						switch cmd.Status {
						case STATUS_WORKLOAD_DESTROYED:
							if agreement, err := persistence.AgreementStateWorkloadTerminated(w.db, cmd.AgreementId, cmd.AgreementProtocol); err != nil {
								glog.Errorf(logString(fmt.Sprintf("error marking agreement %v workload terminated: %v", cmd.AgreementId, err)))
							} else if agreement.AgreementProtocolTerminatedTime != 0 {
								archive = true
							}
						case STATUS_AG_PROTOCOL_TERMINATED:
							if agreement, err := persistence.AgreementStateAgreementProtocolTerminated(w.db, cmd.AgreementId, cmd.AgreementProtocol); err != nil {
								glog.Errorf(logString(fmt.Sprintf("error marking agreement %v agreement protocol terminated: %v", cmd.AgreementId, err)))
							} else if agreement.WorkloadTerminatedTime != 0 {
								archive = true
							}
						default:
							glog.Errorf(logString(fmt.Sprintf("The cleanup status %v is not supported for agreement %v.", cmd.Status, cmd.AgreementId)))
						}

						// archive the agreement if all the cleanup processes are done
						if archive {
							glog.V(5).Infof(logString(fmt.Sprintf("archiving agreement %v", cmd.AgreementId)))
							if _, err := persistence.ArchiveEstablishedAgreement(w.db, cmd.AgreementId, cmd.AgreementProtocol); err != nil {
								glog.Errorf(logString(fmt.Sprintf("error archiving terminated agreement: %v, error: %v", cmd.AgreementId, err)))
							}
						}
					}

				default:
					glog.Errorf("GovernanceWorker received unknown command (%T): %v", command, command)
				}
				glog.V(5).Infof("GovernanceWorker handled command")

			case <-time.After(time.Duration(10) * time.Second):
				// Make sure that all known agreements are maintained
				if w.bcWritesEnabled == true {
					w.governAgreements(protocolHandler)
				}
				// Any commands that have been deferred should be written back to the command queue now. The commands have been
				// accumulating and have endured at least a 10 second break since they were last tried (because we are executing
				// in the channel timeout path).
				glog.V(5).Infof("GovernanceWorker requeue-ing deferred commands")
				for _, c := range deferredCommands {
					w.Commands <- c
				}
				deferredCommands = make([]worker.Command, 0, 10)
			}

		}
	}()
}

// This function encapsulates finalization of an agreement for re-use
func (w *GovernanceWorker) finalizeAgreement(agreement persistence.EstablishedAgreement, protocolHandler *citizenscientist.ProtocolHandler) error {

	// The reply ack might have been lost or mishandled. Since we are now seeing evidence on the blockchain that the agreement
	// was created by the agbot, we will assume we should have gotten a positive reply ack.
	if agreement.AgreementAcceptedTime == 0 {
		if proposal, err := protocolHandler.DemarshalProposal(agreement.Proposal); err != nil {
			return errors.New(logString(fmt.Sprintf("unable to demarshal proposal for agreement %v from database, error %v", agreement.CurrentAgreementId, err)))
		} else if err := w.RecordReply(proposal, citizenscientist.PROTOCOL_NAME); err != nil {
			return errors.New(logString(fmt.Sprintf("unable to accept agreement %v, error: %v", agreement.CurrentAgreementId, err)))
		}
	}

	// Finalize the agreement in the DB
	if _, err := persistence.AgreementStateFinalized(w.db, agreement.CurrentAgreementId, citizenscientist.PROTOCOL_NAME); err != nil {
		return errors.New(logString(fmt.Sprintf("error persisting agreement %v finalized: %v", agreement.CurrentAgreementId, err)))
	} else {
		glog.V(3).Infof(logString(fmt.Sprintf("agreement %v finalized", agreement.CurrentAgreementId)))
	}

	// Update state in exchange
	if proposal, err := protocolHandler.DemarshalProposal(agreement.Proposal); err != nil {
		return errors.New(logString(fmt.Sprintf("could not hydrate proposal, error: %v", err)))
	} else if tcPolicy, err := policy.DemarshalPolicy(proposal.TsAndCs); err != nil {
		return errors.New(logString(fmt.Sprintf("error demarshalling TsAndCs policy for agreement %v, error %v", agreement.CurrentAgreementId, err)))
	} else if err := recordProducerAgreementState(w.Config.Edge.ExchangeURL, w.deviceId, w.deviceToken, agreement.CurrentAgreementId, tcPolicy.APISpecs[0].SpecRef, "Finalized Agreement"); err != nil {
		return errors.New(logString(fmt.Sprintf("error setting agreement %v finalized state in exchange: %v", agreement.CurrentAgreementId, err)))
	}

	return nil
}

func (w *GovernanceWorker) RecordReply(proposal *citizenscientist.Proposal, protocol string) error {

	// Update the state in the database
	if _, err := persistence.AgreementStateAccepted(w.db, proposal.AgreementId, protocol); err != nil {
		return errors.New(logString(fmt.Sprintf("received error updating database state, %v", err)))

		// Update the state in the exchange
	} else if tcPolicy, err := policy.DemarshalPolicy(proposal.TsAndCs); err != nil {
		return errors.New(logString(fmt.Sprintf("received error demarshalling TsAndCs, %v", err)))
	} else if err := recordProducerAgreementState(w.Config.Edge.ExchangeURL, w.deviceId, w.deviceToken, proposal.AgreementId, tcPolicy.APISpecs[0].SpecRef, "Agree to proposal"); err != nil {
		return errors.New(logString(fmt.Sprintf("received error setting state for agreement %v", err)))
	} else {

		// Publish the "agreement reached" event to the message bus so that torrent can start downloading the workload
		// hash is same as filename w/out extension
		hashes := make(map[string]string, 0)
		signatures := make(map[string]string, 0)
		workload := tcPolicy.NextHighestPriorityWorkload(0,0,0)
		for _, image := range workload.Torrent.Images {
			bits := strings.Split(image.File, ".")
			if len(bits) < 2 {
				return errors.New(fmt.Sprintf("Ill-formed image filename: %v", bits))
			} else {
				hashes[image.File] = bits[0]
			}
			signatures[image.File] = image.Signature
		}
		if url, err := url.Parse(workload.Torrent.Url); err != nil {
			return errors.New(fmt.Sprintf("Ill-formed URL: %v", workload.Torrent.Url))
		} else {
			cc := events.NewContainerConfig(*url, hashes, signatures, workload.Deployment, workload.DeploymentSignature, workload.DeploymentUserInfo)

			lc := new(events.AgreementLaunchContext)
			lc.Configure = *cc
			lc.AgreementId = proposal.AgreementId

			// get environmental settings for the workload
			envAdds := make(map[string]string)
			sensorUrl := tcPolicy.APISpecs[0].SpecRef
			if envAdds, err = w.GetWorkloadPreference(sensorUrl); err != nil {
				glog.Errorf("Error: %v", err)
			}
			envAdds[config.ENVVAR_PREFIX+"AGREEMENTID"] = proposal.AgreementId
			envAdds[config.COMPAT_ENVVAR_PREFIX+"AGREEMENTID"] = proposal.AgreementId
			envAdds[config.ENVVAR_PREFIX+"CONTRACT"] = w.Config.Edge.DVPrefix + proposal.AgreementId
			envAdds[config.COMPAT_ENVVAR_PREFIX+"CONTRACT"] = w.Config.Edge.DVPrefix + proposal.AgreementId
			// Temporary hack
			if workload.WorkloadPassword == "" {
				envAdds[config.ENVVAR_PREFIX+"CONFIGURE_NONCE"] = proposal.AgreementId
				envAdds[config.COMPAT_ENVVAR_PREFIX+"CONFIGURE_NONCE"] = proposal.AgreementId
			} else {
				envAdds[config.ENVVAR_PREFIX+"CONFIGURE_NONCE"] = workload.WorkloadPassword
				envAdds[config.COMPAT_ENVVAR_PREFIX+"CONFIGURE_NONCE"] = workload.WorkloadPassword
			}
			envAdds[config.ENVVAR_PREFIX+"HASH"] = workload.WorkloadPassword
			// For workload compatibility, the DEVICE_ID env var is passed with and without the prefix. We would like to drop
			// the env var without prefix once all the workloads have ben updated.
			envAdds["DEVICE_ID"] = w.deviceId
			envAdds[config.ENVVAR_PREFIX+"DEVICE_ID"] = w.deviceId
			envAdds[config.COMPAT_ENVVAR_PREFIX+"DEVICE_ID"] = w.deviceId

			// Add in the exchange URL so that the workload knows which ecosystem its part of
			envAdds[config.ENVVAR_PREFIX+"EXCHANGE_URL"] = w.Config.Edge.ExchangeURL

			lc.EnvironmentAdditions = &envAdds
			lc.AgreementProtocol = citizenscientist.PROTOCOL_NAME
			w.Worker.Manager.Messages <- events.NewAgreementMessage(events.AGREEMENT_REACHED, lc)
		}
	}

	return nil
}

// get the environmental variables for the workload (this is about launching)
func (w *GovernanceWorker) GetWorkloadPreference(url string) (map[string]string, error) {
	attrs, err := persistence.FindApplicableAttributes(w.db, url)
	if err != nil {
		return nil, fmt.Errorf("Unable to fetch workload preferences. Err: %v", err)
	}

	// temporarily create duplicate env var map holding the old names for compatibility and the new names for migration
	// TODO: remove compatMap once Horizon workloads have migrated
	if baseMap, err := persistence.AttributesToEnvvarMap(attrs, config.ENVVAR_PREFIX); err != nil {
		return baseMap, err
	} else if compatMap, err := persistence.AttributesToEnvvarMap(attrs, config.COMPAT_ENVVAR_PREFIX); err != nil {
		return baseMap, err
	} else {
		for k, v := range compatMap {
			baseMap[k] = v
		}

		return baseMap, nil
	}
}

func recordProducerAgreementState(url string, deviceId string, token string, agreementId string, microservice string, state string) error {

	glog.V(5).Infof(logString(fmt.Sprintf("setting agreement %v state to %v", agreementId, state)))

	as := new(exchange.PutAgreementState)
	as.Microservice = microservice
	as.State = state
	var resp interface{}
	resp = new(exchange.PostDeviceResponse)
	targetURL := url + "devices/" + deviceId + "/agreements/" + agreementId
	for {
		if err, tpErr := exchange.InvokeExchange(&http.Client{}, "PUT", targetURL, deviceId, token, &as, &resp); err != nil {
			glog.Errorf(logString(fmt.Sprintf(err.Error())))
			return err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(5).Infof(logString(fmt.Sprintf("set agreement %v to state %v", agreementId, state)))
			return nil
		}
	}

}

func deleteProducerAgreement(url string, deviceId string, token string, agreementId string) error {

	glog.V(5).Infof(logString(fmt.Sprintf("deleting agreement %v in exchange", agreementId)))

	var resp interface{}
	resp = new(exchange.PostDeviceResponse)
	targetURL := url + "devices/" + deviceId + "/agreements/" + agreementId
	for {
		if err, tpErr := exchange.InvokeExchange(&http.Client{}, "DELETE", targetURL, deviceId, token, nil, &resp); err != nil {
			glog.Errorf(logString(fmt.Sprintf(err.Error())))
			return err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(5).Infof(logString(fmt.Sprintf("deleted agreement %v from exchange", agreementId)))
			return nil
		}
	}

}

func (w *GovernanceWorker) deleteMessage(msg *exchange.DeviceMessage) error {
	var resp interface{}
	resp = new(exchange.PostDeviceResponse)
	targetURL := w.Manager.Config.Edge.ExchangeURL + "devices/" + w.deviceId + "/msgs/" + strconv.Itoa(msg.MsgId)
	for {
		if err, tpErr := exchange.InvokeExchange(w.httpClient, "DELETE", targetURL, w.deviceId, w.deviceToken, nil, &resp); err != nil {
			glog.Errorf(err.Error())
			return err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(3).Infof(logString(fmt.Sprintf("deleted message %v", msg.MsgId)))
			return nil
		}
	}
}

func (w *GovernanceWorker) messageInExchange(msgId int) (bool, error) {
	var resp interface{}
	resp = new(exchange.GetDeviceMessageResponse)
	targetURL := w.Manager.Config.Edge.ExchangeURL + "devices/" + w.deviceId + "/msgs"
	for {
		if err, tpErr := exchange.InvokeExchange(w.httpClient, "GET", targetURL, w.deviceId, w.deviceToken, nil, &resp); err != nil {
			glog.Errorf(err.Error())
			return false, err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			msgs := resp.(*exchange.GetDeviceMessageResponse).Messages
			for _, msg := range msgs {
				if msg.MsgId == msgId {
					return true, nil
				}
			}
			return false, nil
		}
	}
}

var logString = func(v interface{}) string {
	return fmt.Sprintf("GovernanceWorker: %v", v)
}
