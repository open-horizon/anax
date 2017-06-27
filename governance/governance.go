package governance

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/abstractprotocol"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/device"
	"github.com/open-horizon/anax/ethblockchain"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/metering"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/producer"
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
	producerPH      map[string]producer.ProducerProtocolHandler
}

func NewGovernanceWorker(cfg *config.HorizonConfig, db *bolt.DB, pm *policy.PolicyManager) *GovernanceWorker {
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
				Config:   cfg,
				Messages: messages,
			},

			Commands: commands,
		},

		db:              db,
		httpClient:      &http.Client{Timeout: time.Duration(config.HTTPDEFAULTTIMEOUT*time.Millisecond)},
		pm:              pm,
		deviceId:        id,
		deviceToken:     token,
		bcReady:         false,
		bcWritesEnabled: false,
		producerPH:      make(map[string]producer.ProducerProtocolHandler),
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
			cmd := w.NewCleanupExecutionCommand(msg.AgreementProtocol, msg.AgreementId, w.producerPH[msg.AgreementProtocol].GetTerminationCode(producer.TERM_REASON_CONTAINER_FAILURE), msg.Deployment)
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
				cmd := w.NewCleanupExecutionCommand(lc.AgreementProtocol, lc.AgreementId, w.producerPH[lc.AgreementProtocol].GetTerminationCode(producer.TERM_REASON_TORRENT_FAILURE), nil)
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
			cmd := w.NewCleanupExecutionCommand(msg.AgreementProtocol, msg.AgreementId, w.producerPH[msg.AgreementProtocol].GetTerminationCode(producer.TERM_REASON_USER_REQUESTED), msg.Deployment)
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
			cmd := producer.NewBlockchainEventCommand(*msg)
			w.Commands <- cmd
		}
	case *events.ExchangeDeviceMessage:
		msg, _ := incoming.(*events.ExchangeDeviceMessage)
		switch msg.Event().Id {
		case events.RECEIVED_EXCHANGE_DEV_MSG:
			cmd := producer.NewExchangeMessageCommand(*msg)
			w.Commands <- cmd
		}

	default: //nothing
	}

	glog.V(4).Infof(logString(fmt.Sprintf("command channel length %v added", len(w.Commands))))

	return
}

func (w *GovernanceWorker) governAgreements() {

	glog.V(3).Infof(logString(fmt.Sprintf("governing pending agreements")))

	// Create a new filter for unfinalized agreements
	notYetFinalFilter := func() persistence.EAFilter {
		return func(a persistence.EstablishedAgreement) bool {
			return a.AgreementCreationTime != 0 && a.AgreementTerminatedTime == 0
		}
	}

	if establishedAgreements, err := persistence.FindEstablishedAgreementsAllProtocols(w.db, policy.AllAgreementProtocols(), []persistence.EAFilter{persistence.UnarchivedEAFilter(), notYetFinalFilter()}); err != nil {
		glog.Errorf(logString(fmt.Sprintf("Unable to retrieve not yet final agreements from database: %v. Error: %v", err, err)))
	} else {

		// If there are agreemens in the database then we will assume that the device is already registered
		for _, ag := range establishedAgreements {
			protocolHandler := w.producerPH[ag.AgreementProtocol].AgreementProtocolHandler()
			if ag.AgreementFinalizedTime == 0 {   // TODO: might need to change this to be a protocol specific check
				// Cancel the agreement if finalization doesn't occur before the timeout
				glog.V(5).Infof(logString(fmt.Sprintf("checking agreement %v for finalization.", ag.CurrentAgreementId)))

				// Check to see if the agreement is in the blockchain. This call to the blockchain should be very fast.
				// The device might have been down for some time and/or restarted, causing it to miss events on the blockchain.
				if recorded, err := protocolHandler.VerifyAgreement(ag.CurrentAgreementId, ag.CounterPartyAddress, ag.ProposalSig); err != nil {
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

						reason := w.producerPH[ag.AgreementProtocol].GetTerminationCode(producer.TERM_REASON_NOT_FINALIZED_TIMEOUT)
						if ag.AgreementAcceptedTime == 0 {
							reason = w.producerPH[ag.AgreementProtocol].GetTerminationCode(producer.TERM_REASON_NO_REPLY_ACK)
						}
						w.cancelAgreement(ag.CurrentAgreementId, ag.AgreementProtocol, reason, w.producerPH[ag.AgreementProtocol].GetTerminationReason(reason))

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
						reason := w.producerPH[ag.AgreementProtocol].GetTerminationCode(producer.TERM_REASON_NOT_EXECUTED_TIMEOUT)
						w.cancelAgreement(ag.CurrentAgreementId, ag.AgreementProtocol, reason, w.producerPH[ag.AgreementProtocol].GetTerminationReason(reason))
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

			if establishedAgreements, err := persistence.FindEstablishedAgreementsAllProtocols(w.db, policy.AllAgreementProtocols(), []persistence.EAFilter{persistence.UnarchivedEAFilter(), runningFilter()}); err != nil {
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

	// Put the rest of the cancel processing into it's own go routine. In some agreement protocols, this go
	// routine will be waiting for a blockchain cancel to run. In general it will take around 30 seconds, but could be
	// double or triple that time. This will free up the governance thread to handle other protocol messages.
	go func() {

		// Get the policy we used in the agreement and then cancel, just in case.
		glog.V(3).Infof(logString(fmt.Sprintf("terminating agreement %v", agreementId)))

		if ag != nil && w.bcWritesEnabled == true {
			w.producerPH[agreementProtocol].TerminateAgreement(ag, reason)
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
			glog.V(3).Infof(logString(fmt.Sprintf("Writing Metering Notification %v to the blockchain for %v.", ag.MeteringNotificationMsg, agreementId)))
			if mn := metering.ConvertFromPersistent(ag.MeteringNotificationMsg, agreementId); mn == nil {
				glog.Errorf(logString(fmt.Sprintf("error converting from persistent Metering Notification %v for %v, returned nil.", ag.MeteringNotificationMsg, agreementId)))
			} else if err := w.producerPH[agreementProtocol].AgreementProtocolHandler().RecordMeter(agreementId, mn); err != nil {
				glog.Errorf(logString(fmt.Sprintf("error writing meter %v for agreement %v on the blockchain: %v", ag.MeteringNotificationMsg, agreementId, err)))
			}
		}
	}()

}

func (w *GovernanceWorker) start() {
	go func() {

		// Fire up the eth container after the device is registered.
		for {
			if w.deviceToken != "" {
				break
			} else {
				glog.V(3).Infof("GovernanceWorker command processor waiting for device registration")
				time.Sleep(time.Duration(5) * time.Second)
			}
		}

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

		// Establish agreement protocol handlers
		for _, protocolName := range policy.AllAgreementProtocols() {
			pph := producer.CreateProducerPH(protocolName, w.Worker.Manager.Config, w.db, w.pm, w.deviceId, w.deviceToken)
			pph.Initialize()
			w.producerPH[protocolName] = pph
		}

		// Fire up the container governor
		w.governContainers()

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
					if ags, err := persistence.FindEstablishedAgreements(w.db, cmd.AgreementProtocol, []persistence.EAFilter{persistence.UnarchivedEAFilter(), persistence.IdEAFilter(agreementId)}); err != nil {
						glog.Errorf(logString(fmt.Sprintf("unable to retrieve agreement %v from database, error %v", agreementId, err)))
					} else if len(ags) != 1 {
						glog.V(5).Infof(logString(fmt.Sprintf("ignoring the event, unable to retrieve unarchived single agreement %v from the database.", agreementId)))
					} else if ags[0].AgreementTerminatedTime != 0 && ags[0].AgreementForceTerminatedTime == 0 {
						glog.V(3).Infof(logString(fmt.Sprintf("ignoring the event, agreement %v is already terminating", agreementId)))
					} else if w.bcWritesEnabled == true {
						glog.V(3).Infof("Ending the agreement: %v", agreementId)
						w.cancelAgreement(agreementId, cmd.AgreementProtocol, cmd.Reason, w.producerPH[cmd.AgreementProtocol].GetTerminationReason(cmd.Reason))

						// send the event to the container in case it has started the workloads.
						w.Messages() <- events.NewGovernanceCancelationMessage(events.AGREEMENT_ENDED, events.AG_TERMINATED, cmd.AgreementProtocol, agreementId, cmd.Deployment)
					} else {
						// Requeue
						glog.V(3).Infof("Deferring ending the agreement: %v", agreementId)
						deferredCommands = append(deferredCommands, cmd)
					}

				case *producer.ExchangeMessageCommand:
					cmd, _ := command.(*producer.ExchangeMessageCommand)

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

					deleteMessage := true
					protocolMsg := cmd.Msg.ProtocolMessage()

					// Pull the agreement protocol out of the message
					if msgProtocol, err := abstractprotocol.ExtractProtocol(protocolMsg); err != nil {
						glog.Errorf(logString(fmt.Sprintf("unable to extract agreement protocol name from message %v", protocolMsg)))
					} else if _, ok := w.producerPH[msgProtocol]; !ok {
						glog.Infof(logString(fmt.Sprintf("unable to direct exchange message %v to a protocol handler, deleting it.", protocolMsg)))
					} else {

						deleteMessage = false
						protocolHandler := w.producerPH[msgProtocol].AgreementProtocolHandler()
						// ReplyAck messages could indicate that the agbot has decided not to pursue the agreement any longer.
						if replyAck, err := protocolHandler.ValidateReplyAck(protocolMsg); err != nil {
							glog.V(5).Infof(logString(fmt.Sprintf("ReplyAck handler ignoring non-reply ack message: %s due to %v", cmd.Msg.ShortProtocolMessage(), err)))
						} else if ags, err := persistence.FindEstablishedAgreements(w.db, msgProtocol, []persistence.EAFilter{persistence.UnarchivedEAFilter(), persistence.IdEAFilter(replyAck.AgreementId())}); err != nil {
							glog.Errorf(logString(fmt.Sprintf("unable to retrieve agreement %v from database, error %v", replyAck.AgreementId(), err)))
						} else if len(ags) != 1 {
							glog.Warningf(logString(fmt.Sprintf("unable to retrieve single agreement %v from database.", replyAck.AgreementId())))
							deleteMessage = true
						} else if replyAck.ReplyAgreementStillValid() {
							if ags[0].AgreementAcceptedTime != 0 || ags[0].AgreementTerminatedTime != 0 {
								glog.V(5).Infof(logString(fmt.Sprintf("ignoring replyack for %v because we already received one or are cancelling", replyAck.AgreementId())))
								deleteMessage = true
							} else if proposal, err := protocolHandler.DemarshalProposal(ags[0].Proposal); err != nil {
								glog.Errorf(logString(fmt.Sprintf("unable to demarshal proposal for agreement %v from database", replyAck.AgreementId())))
							} else if err := w.RecordReply(proposal, msgProtocol); err != nil {
								glog.Errorf(logString(fmt.Sprintf("unable to record reply %v, error: %v", replyAck, err)))
							} else {
								deleteMessage = true
							}
						} else {
							deleteMessage = true
							if w.bcWritesEnabled == true {
								w.Messages() <- events.NewGovernanceCancelationMessage(events.AGREEMENT_ENDED, events.AG_TERMINATED, ags[0].AgreementProtocol, ags[0].CurrentAgreementId, ags[0].CurrentDeployment)
								reason := w.producerPH[msgProtocol].GetTerminationCode(producer.TERM_REASON_AGBOT_REQUESTED)
								w.cancelAgreement(replyAck.AgreementId(), msgProtocol, reason, w.producerPH[msgProtocol].GetTerminationReason(reason))
							} else {
								glog.Infof(logString(fmt.Sprintf("deferring termination of agreement %v until the device is funded.", ags[0].CurrentAgreementId)))
								deferredCommands = append(deferredCommands, cmd)
							}
						}

						// Data notification message indicates that the agbot has found that data is being received from the workload.
						if dataReceived, err := protocolHandler.ValidateDataReceived(protocolMsg); err != nil {
							glog.V(5).Infof(logString(fmt.Sprintf("DataReceived handler ignoring non-data received message: %v due to %v", cmd.Msg.ShortProtocolMessage(), err)))
						} else if ags, err := persistence.FindEstablishedAgreements(w.db, msgProtocol, []persistence.EAFilter{persistence.UnarchivedEAFilter(), persistence.IdEAFilter(dataReceived.AgreementId())}); err != nil {
							glog.Errorf(logString(fmt.Sprintf("unable to retrieve agreement %v from database, error %v", dataReceived.AgreementId(), err)))
						} else if len(ags) != 1 {
							glog.Warningf(logString(fmt.Sprintf("unable to retrieve single agreement %v from database, error %v", dataReceived.AgreementId(), err)))
							deleteMessage = true
						} else if _, err := persistence.AgreementStateDataReceived(w.db, dataReceived.AgreementId(), msgProtocol); err != nil {
							glog.Errorf(logString(fmt.Sprintf("unable to update data received time for %v, error: %v", dataReceived.AgreementId(), err)))
						} else if messageTarget, err := exchange.CreateMessageTarget(exchangeMsg.AgbotId, nil, exchangeMsg.AgbotPubKey, ""); err != nil {
							glog.Errorf(logString(fmt.Sprintf("error creating message target: %v", err)))
						} else if err := protocolHandler.NotifyDataReceiptAck(dataReceived.AgreementId(), messageTarget, w.producerPH[msgProtocol].GetSendMessage()); err != nil {
							glog.Errorf(logString(fmt.Sprintf("unable to send data received ack for %v, error: %v", dataReceived.AgreementId(), err)))
						} else {
							deleteMessage = true
						}

						// Metering notification messages indicate that the agbot is metering data sent to the data ingest.
						if mnReceived, err := protocolHandler.ValidateMeterNotification(protocolMsg); err != nil {
							glog.V(5).Infof(logString(fmt.Sprintf("Meter Notification handler ignoring non-metering message: %v due to %v", cmd.Msg.ShortProtocolMessage(), err)))
						} else if ags, err := persistence.FindEstablishedAgreements(w.db, msgProtocol, []persistence.EAFilter{persistence.UnarchivedEAFilter(), persistence.IdEAFilter(mnReceived.AgreementId())}); err != nil {
							glog.Errorf(logString(fmt.Sprintf("unable to retrieve agreement %v from database, error %v", mnReceived.AgreementId(), err)))
						} else if len(ags) != 1 {
							glog.Warningf(logString(fmt.Sprintf("unable to retrieve single agreement %v from database, error %v", mnReceived.AgreementId(), err)))
							deleteMessage = true
						} else if ags[0].AgreementTerminatedTime != 0 {
							glog.V(5).Infof(logString(fmt.Sprintf("ignoring metering notification, agreement %v is terminating", mnReceived.AgreementId())))
							deleteMessage = true
						} else if mn, err := metering.ConvertToPersistent(mnReceived.Meter()); err != nil {
							glog.Errorf(logString(fmt.Sprintf("unable to convert metering notification string %v to persistent metering notification for %v, error: %v", mnReceived.Meter(), mnReceived.AgreementId(), err)))
							deleteMessage = true
						} else if _, err := persistence.MeteringNotificationReceived(w.db, mnReceived.AgreementId(), *mn, msgProtocol); err != nil {
							glog.Errorf(logString(fmt.Sprintf("unable to update metering notification for %v, error: %v", mnReceived.AgreementId(), err)))
							deleteMessage = true
						} else {
							deleteMessage = true
						}

						// Cancel messages indicate that the agbot wants to get rid of the agreement.
						if canReceived, err := protocolHandler.ValidateCancel(protocolMsg); err != nil {
							glog.V(5).Infof(logString(fmt.Sprintf("Cancel handler ignoring non-cancel message: %v due to %v", cmd.Msg.ShortProtocolMessage(), err)))
						} else if ags, err := persistence.FindEstablishedAgreements(w.db, msgProtocol, []persistence.EAFilter{persistence.UnarchivedEAFilter(), persistence.IdEAFilter(canReceived.AgreementId())}); err != nil {
							glog.Errorf(logString(fmt.Sprintf("unable to retrieve agreement %v from database, error %v", canReceived.AgreementId(), err)))
						} else if len(ags) != 1 {
							glog.Warningf(logString(fmt.Sprintf("unable to retrieve single agreement %v from database, error %v", canReceived.AgreementId(), err)))
							deleteMessage = true
						} else if exchangeMsg.AgbotId != ags[0].ConsumerId {
							glog.Warningf(logString(fmt.Sprintf("cancel ignored, cancel message for %v came from id %v but agreement is with %v", canReceived.AgreementId(), exchangeMsg.AgbotId, ags[0].ConsumerId)))
							deleteMessage = true
						} else if ags[0].AgreementTerminatedTime != 0 {
							glog.V(5).Infof(logString(fmt.Sprintf("ignoring cancel, agreement %v is terminating", canReceived.AgreementId())))
							deleteMessage = true
						} else {
							if w.bcWritesEnabled == true {
								w.cancelAgreement(canReceived.AgreementId(), msgProtocol, canReceived.Reason(), w.producerPH[msgProtocol].GetTerminationReason(canReceived.Reason()))
								// cleanup workloads if needed
								w.Messages() <- events.NewGovernanceCancelationMessage(events.AGREEMENT_ENDED, events.AG_TERMINATED, ags[0].AgreementProtocol, ags[0].CurrentAgreementId, ags[0].CurrentDeployment)
								deleteMessage = true
							} else {
								glog.Infof(logString(fmt.Sprintf("deferring termination of agreement %v until the device is funded.", ags[0].CurrentAgreementId)))
								deferredCommands = append(deferredCommands, cmd)
							}
						}
					}

					// Get rid of the exchange message when we're done with it
					if deleteMessage {
						if err := w.deleteMessage(exchangeMsg); err != nil {
							glog.Errorf(logString(fmt.Sprintf("error deleting exchange message %v, error %v", exchangeMsg.MsgId, err)))
						}
					}

				case *producer.BlockchainEventCommand:
					cmd, _ := command.(*producer.BlockchainEventCommand)

					for _, protocol := range policy.AllAgreementProtocols() {
						if !w.producerPH[protocol].AcceptCommand(cmd) {
							continue
						}

						protocolHandler := w.producerPH[protocol].AgreementProtocolHandler()

						if agreementId, termination, reason, creation, err := w.producerPH[protocol].HandleBlockchainEventMessage(cmd); err != nil {
							glog.Errorf(err.Error())
						} else if termination {

							// If we have that agreement in our DB, then cancel it
							if ags, err := persistence.FindEstablishedAgreements(w.db, protocol, []persistence.EAFilter{persistence.UnarchivedEAFilter(), persistence.IdEAFilter(agreementId)}); err != nil {
								glog.Errorf(logString(fmt.Sprintf("unable to retrieve agreement %v from database, error %v", agreementId, err)))
							} else if len(ags) != 1 {
								glog.V(5).Infof(logString(fmt.Sprintf("ignoring event, not our agreement id")))
							} else if ags[0].AgreementTerminatedTime != 0 {
								glog.V(5).Infof(logString(fmt.Sprintf("ignoring event, agreement %v is already terminating", ags[0].CurrentAgreementId)))
							} else {
								if w.bcWritesEnabled == true {
									glog.Infof(logString(fmt.Sprintf("terminating agreement %v because it has been cancelled on the blockchain.", ags[0].CurrentAgreementId)))
									w.cancelAgreement(ags[0].CurrentAgreementId, ags[0].AgreementProtocol, uint(reason), w.producerPH[protocol].GetTerminationReason(uint(reason)))
									// cleanup workloads if needed
									w.Messages() <- events.NewGovernanceCancelationMessage(events.AGREEMENT_ENDED, events.AG_TERMINATED, ags[0].AgreementProtocol, ags[0].CurrentAgreementId, ags[0].CurrentDeployment)
								} else {
									glog.Infof(logString(fmt.Sprintf("deferring termination of agreement %v until the device is funded.", ags[0].CurrentAgreementId)))
									deferredCommands = append(deferredCommands, cmd)
								}
							}

							// If the event is an agreement created event
						} else if creation {

							// If we have that agreement in our DB and it's not already terminating, then finalize it
							if ags, err := persistence.FindEstablishedAgreements(w.db, protocol, []persistence.EAFilter{persistence.UnarchivedEAFilter(), persistence.IdEAFilter(agreementId)}); err != nil {
								glog.Errorf(logString(fmt.Sprintf("unable to retrieve agreement %v from database, error %v", agreementId, err)))
							} else if len(ags) != 1 {
								glog.V(5).Infof(logString(fmt.Sprintf("ignoring event, not our agreement id")))
							} else if ags[0].AgreementTerminatedTime != 0 {
								glog.V(5).Infof(logString(fmt.Sprintf("ignoring event, agreement %v is terminating", ags[0].CurrentAgreementId)))

							// Finalize the agreement
							} else if err := w.finalizeAgreement(ags[0], protocolHandler); err != nil {
								glog.Errorf(err.Error())
							}
						}
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
					w.governAgreements()
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
func (w *GovernanceWorker) finalizeAgreement(agreement persistence.EstablishedAgreement, protocolHandler abstractprotocol.ProtocolHandler) error {

	// The reply ack might have been lost or mishandled. Since we are now seeing evidence on the blockchain that the agreement
	// was created by the agbot, we will assume we should have gotten a positive reply ack.
	if agreement.AgreementAcceptedTime == 0 {
		if proposal, err := protocolHandler.DemarshalProposal(agreement.Proposal); err != nil {
			return errors.New(logString(fmt.Sprintf("unable to demarshal proposal for agreement %v from database, error %v", agreement.CurrentAgreementId, err)))
		} else if err := w.RecordReply(proposal, protocolHandler.Name()); err != nil {
			return errors.New(logString(fmt.Sprintf("unable to accept agreement %v, error: %v", agreement.CurrentAgreementId, err)))
		}
	}

	// Finalize the agreement in the DB
	if _, err := persistence.AgreementStateFinalized(w.db, agreement.CurrentAgreementId, protocolHandler.Name()); err != nil {
		return errors.New(logString(fmt.Sprintf("error persisting agreement %v finalized: %v", agreement.CurrentAgreementId, err)))
	} else {
		glog.V(3).Infof(logString(fmt.Sprintf("agreement %v finalized", agreement.CurrentAgreementId)))
	}

	// Update state in exchange
	if proposal, err := protocolHandler.DemarshalProposal(agreement.Proposal); err != nil {
		return errors.New(logString(fmt.Sprintf("could not hydrate proposal, error: %v", err)))
	} else if tcPolicy, err := policy.DemarshalPolicy(proposal.TsAndCs()); err != nil {
		return errors.New(logString(fmt.Sprintf("error demarshalling TsAndCs policy for agreement %v, error %v", agreement.CurrentAgreementId, err)))
	} else if err := recordProducerAgreementState(w.Config.Edge.ExchangeURL, w.deviceId, w.deviceToken, agreement.CurrentAgreementId, tcPolicy.APISpecs[0].SpecRef, "Finalized Agreement"); err != nil {
		return errors.New(logString(fmt.Sprintf("error setting agreement %v finalized state in exchange: %v", agreement.CurrentAgreementId, err)))
	}

	return nil
}

func (w *GovernanceWorker) RecordReply(proposal abstractprotocol.Proposal, protocol string) error {

	// Update the state in the database
	if _, err := persistence.AgreementStateAccepted(w.db, proposal.AgreementId(), protocol); err != nil {
		return errors.New(logString(fmt.Sprintf("received error updating database state, %v", err)))

		// Update the state in the exchange
	} else if tcPolicy, err := policy.DemarshalPolicy(proposal.TsAndCs()); err != nil {
		return errors.New(logString(fmt.Sprintf("received error demarshalling TsAndCs, %v", err)))
	} else if err := recordProducerAgreementState(w.Config.Edge.ExchangeURL, w.deviceId, w.deviceToken, proposal.AgreementId(), tcPolicy.APISpecs[0].SpecRef, "Agree to proposal"); err != nil {
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
			lc.AgreementId = proposal.AgreementId()

			// get environmental settings for the workload
			envAdds := make(map[string]string)
			sensorUrl := tcPolicy.APISpecs[0].SpecRef
			if envAdds, err = w.GetWorkloadPreference(sensorUrl); err != nil {
				glog.Errorf("Error: %v", err)
			}
			envAdds[config.ENVVAR_PREFIX+"AGREEMENTID"] = proposal.AgreementId()
			envAdds[config.ENVVAR_PREFIX+"HASH"] = workload.WorkloadPassword
			envAdds[config.ENVVAR_PREFIX+"DEVICE_ID"] = w.deviceId

			// Add in the exchange URL so that the workload knows which ecosystem its part of
			envAdds[config.ENVVAR_PREFIX+"EXCHANGE_URL"] = w.Config.Edge.ExchangeURL

			lc.EnvironmentAdditions = &envAdds
			lc.AgreementProtocol = protocol
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

	return persistence.AttributesToEnvvarMap(attrs, config.ENVVAR_PREFIX)

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
		if err, tpErr := exchange.InvokeExchange(&http.Client{Timeout: time.Duration(config.HTTPDEFAULTTIMEOUT*time.Millisecond)}, "PUT", targetURL, deviceId, token, &as, &resp); err != nil {
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
		if err, tpErr := exchange.InvokeExchange(&http.Client{Timeout: time.Duration(config.HTTPDEFAULTTIMEOUT*time.Millisecond)}, "DELETE", targetURL, deviceId, token, nil, &resp); err != nil {
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
