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
	"os"
	"sort"
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
	worker.Worker // embedded field
	db            *bolt.DB
	httpClient    *http.Client
	bc            *ethblockchain.BaseContracts
	deviceId      string
	deviceToken   string
	pm            *policy.PolicyManager
	producerPH    map[string]producer.ProducerProtocolHandler
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

		db:          db,
		httpClient:  &http.Client{Timeout: time.Duration(config.HTTPDEFAULTTIMEOUT * time.Millisecond)},
		pm:          pm,
		deviceId:    id,
		deviceToken: token,
		producerPH:  make(map[string]producer.ProducerProtocolHandler),
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
			cmd := producer.NewBCInitializedCommand(msg)
			w.Commands <- cmd
		}
	case *events.BlockchainClientStoppingMessage:
		msg, _ := incoming.(*events.BlockchainClientStoppingMessage)
		switch msg.Event().Id {
		case events.BC_CLIENT_STOPPING:
			cmd := producer.NewBCStoppingCommand(msg)
			w.Commands <- cmd
		}
	case *events.AccountFundedMessage:
		msg, _ := incoming.(*events.AccountFundedMessage)
		switch msg.Event().Id {
		case events.ACCOUNT_FUNDED:
			cmd := producer.NewBCWritableCommand(msg)
			w.Commands <- cmd
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
	case *events.StartMicroserviceMessage:
		msg, _ := incoming.(*events.StartMicroserviceMessage)
		switch msg.Event().Id {
		case events.START_MICROSERVICE:
			cmd := w.NewStartMicroserviceCommand(msg.MsDefKey)
			w.Commands <- cmd
		}
	case *events.ContainerMessage:
		msg, _ := incoming.(*events.ContainerMessage)
		if msg.LaunchContext.Blockchain.Name == "" { // microservice case
			switch msg.Event().Id {
			case events.EXECUTION_BEGUN:
				cmd := w.NewUpdateMicroserviceInstanceCommand(msg.LaunchContext.Name, true, 0, "")
				w.Commands <- cmd
			case events.EXECUTION_FAILED:
				cmd := w.NewUpdateMicroserviceInstanceCommand(msg.LaunchContext.Name, false, 1, "Failed to launch containers.")
				w.Commands <- cmd
			}
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

		// If there are agreements in the database then we will assume that the device is already registered
		for _, ag := range establishedAgreements {
			bcType, bcName := w.producerPH[ag.AgreementProtocol].GetKnownBlockchain(&ag)
			protocolHandler := w.producerPH[ag.AgreementProtocol].AgreementProtocolHandler(bcType, bcName)
			if ag.AgreementFinalizedTime == 0 { // TODO: might need to change this to be a protocol specific check

				// Cancel the agreement if finalization doesn't occur before the timeout
				glog.V(5).Infof(logString(fmt.Sprintf("checking agreement %v for finalization.", ag.CurrentAgreementId)))

				// Check to see if we need to update the consumer with our blockchain specific pieces of the agreement
				if w.producerPH[ag.AgreementProtocol].IsBlockchainClientAvailable(bcType, bcName) && ag.AgreementBCUpdateAckTime == 0 {
					w.producerPH[ag.AgreementProtocol].UpdateConsumer(&ag)
				}

				// Check to see if the agreement is in the blockchain. This call to the blockchain should be very fast if the client is up and running.
				// Remember, the device might have been down for some time and/or restarted, causing it to miss events on the blockchain.
				if w.producerPH[ag.AgreementProtocol].IsBlockchainClientAvailable(bcType, bcName) && w.producerPH[ag.AgreementProtocol].IsAgreementVerifiable(&ag) {

					if recorded, err := protocolHandler.VerifyAgreement(ag.CurrentAgreementId, ag.CounterPartyAddress, ag.ProposalSig); err != nil {
						glog.Errorf(logString(fmt.Sprintf("encountered error verifying agreement %v on blockchain, error %v", ag.CurrentAgreementId, err)))
					} else if recorded {
						if err := w.finalizeAgreement(ag, protocolHandler); err != nil {
							glog.Errorf(err.Error())
						} else {
							continue
						}
					}
				}
				// If we fall through to here, then the agreement is Not finalized yet, check for a timeout.
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
					w.Messages() <- events.NewGovernanceWorkloadCancelationMessage(events.AGREEMENT_ENDED, events.AG_TERMINATED, ag.AgreementProtocol, ag.CurrentAgreementId, ag.CurrentDeployment)
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
						w.Messages() <- events.NewGovernanceWorkloadCancelationMessage(events.AGREEMENT_ENDED, events.AG_TERMINATED, ag.AgreementProtocol, ag.CurrentAgreementId, ag.CurrentDeployment)
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

func (w *GovernanceWorker) reportBlockchains() {

	// go govern
	go func() {

		for {

			glog.Info(logString(fmt.Sprintf("started blockchain need governance")))

			// This is the amount of time for the governance routine to wait.
			waitTime := uint64(60)

			// Find all agreements that need a blockchain by searching through all the agreement protocol DB buckets
			for _, agp := range policy.AllAgreementProtocols() {

				// If the agreement protocol doesnt require a blockchain then we can skip it.
				if bcType := policy.RequiresBlockchainType(agp); bcType == "" {
					continue
				} else {

					// Make a map of all blockchain names that we need to have running
					neededBCs := make(map[string]bool)
					if agreements, err := persistence.FindEstablishedAgreements(w.db, agp, []persistence.EAFilter{persistence.UnarchivedEAFilter()}); err == nil {
						for _, ag := range agreements {
							_, bcName := w.producerPH[agp].GetKnownBlockchain(&ag)
							if bcName != "" {
								neededBCs[bcName] = true
							}
						}

						// If we captured any needed blockchains, inform the blockchain worker
						if len(neededBCs) != 0 {
							w.Messages() <- events.NewReportNeededBlockchainsMessage(events.BC_NEEDED, bcType, neededBCs)
						}

					} else {
						glog.Errorf(logString(fmt.Sprintf("unable to read agreements from database for protocol %v, error: %v", agp, err)))
					}

				}
			}

			// Sleep
			glog.V(5).Infof(logString(fmt.Sprintf("blockchain need governance sleeping for %v seconds.", waitTime)))
			time.Sleep(time.Duration(waitTime) * time.Second)

		}
	}()
}

func (w *GovernanceWorker) governMicroservices() {

	go func() {
		for {

			// handle microservice instance containers down
			glog.V(4).Infof(logString(fmt.Sprintf("governing microservice containers")))
			if ms_instances, err := persistence.FindMicroserviceInstances(w.db, []persistence.MIFilter{persistence.AllMIFilter(), persistence.UnarchivedMIFilter()}); err != nil {
				glog.Errorf(logString(fmt.Sprintf("Error retrieving all microservice instances from database, error: %v", err)))
			} else if ms_instances != nil {
				for _, msi := range ms_instances {
					// only check the ones that has containers started already
					if msi.ExecutionStartTime != 0 {
						glog.V(3).Infof(logString(fmt.Sprintf("fire event to ensure microservice containers are still up for microservice instance %v.", msi.GetKey())))

						// ensure containers are still running
						w.Messages() <- events.NewMicroserviceMaintenanceMessage(events.CONTAINER_MAINTAIN, msi.GetKey())
					}
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

	// Update the microservice if any
	if err := persistence.DeleteAsscAgmtsFromMSInstances(w.db, agreementId); err != nil {
		glog.Errorf(logString(fmt.Sprintf("error removing agreement id %v from the microservice db: %v", agreementId, err)))
	}

	// Delete from the exchange
	if ag != nil && ag.AgreementAcceptedTime != 0 {
		if err := deleteProducerAgreement(w.Config.Edge.ExchangeURL, w.deviceId, w.deviceToken, agreementId); err != nil {
			glog.Errorf(logString(fmt.Sprintf("error deleting agreement %v in exchange: %v", agreementId, err)))
		}
	}

	// If we can do the termination now, do it. Otherwise we will queue a command to do it later.
	w.externalTermination(ag, agreementId, agreementProtocol, reason)
	if !w.producerPH[agreementProtocol].IsBlockchainWritable(ag) {
		// create deferred external termination command
		w.Commands <- NewAsyncTerminationCommand(agreementId, agreementProtocol, reason)
	}

}

func (w *GovernanceWorker) externalTermination(ag *persistence.EstablishedAgreement, agreementId string, agreementProtocol string, reason uint) {

	// Put the rest of the cancel processing into it's own go routine. In some agreement protocols, this go
	// routine will be waiting for a blockchain cancel to run. In general it will take around 30 seconds, but could be
	// double or triple that time. This will free up the governance thread to handle other protocol messages.

	go func() {

		// Get the policy we used in the agreement and then cancel, just in case.
		glog.V(3).Infof(logString(fmt.Sprintf("terminating agreement %v", agreementId)))

		if ag != nil {
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
		if ag.MeteringNotificationMsg != (persistence.MeteringNotification{}) && !ag.Archived {
			glog.V(3).Infof(logString(fmt.Sprintf("Writing Metering Notification %v to the blockchain for %v.", ag.MeteringNotificationMsg, agreementId)))
			bcType, bcName := w.producerPH[agreementProtocol].GetKnownBlockchain(ag)
			if mn := metering.ConvertFromPersistent(ag.MeteringNotificationMsg, agreementId); mn == nil {
				glog.Errorf(logString(fmt.Sprintf("error converting from persistent Metering Notification %v for %v, returned nil.", ag.MeteringNotificationMsg, agreementId)))
			} else if aph := w.producerPH[agreementProtocol].AgreementProtocolHandler(bcType, bcName); aph == nil {
				glog.Warningf(logString(fmt.Sprintf("cannot write meter record for %v, agreement protocol handler is not ready.", agreementId)))
			} else if err := aph.RecordMeter(agreementId, mn); err != nil {
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

		// Establish agreement protocol handlers
		for _, protocolName := range policy.AllAgreementProtocols() {
			pph := producer.CreateProducerPH(protocolName, w.Worker.Manager.Config, w.db, w.pm, w.deviceId, w.deviceToken)
			pph.Initialize()
			w.producerPH[protocolName] = pph
		}

		// Fire up the container governor
		w.governContainers()

		// Fire up the blockchain reporter
		w.reportBlockchains()

		// Fire up the microservice governor
		w.governMicroservices()

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
					} else {
						glog.V(3).Infof("Ending the agreement: %v", agreementId)
						w.cancelAgreement(agreementId, cmd.AgreementProtocol, cmd.Reason, w.producerPH[cmd.AgreementProtocol].GetTerminationReason(cmd.Reason))

						// send the event to the container in case it has started the workloads.
						w.Messages() <- events.NewGovernanceWorkloadCancelationMessage(events.AGREEMENT_ENDED, events.AG_TERMINATED, cmd.AgreementProtocol, agreementId, cmd.Deployment)
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
						protocolHandler := w.producerPH[msgProtocol].AgreementProtocolHandler("", "")
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
							w.Messages() <- events.NewGovernanceWorkloadCancelationMessage(events.AGREEMENT_ENDED, events.AG_TERMINATED, ags[0].AgreementProtocol, ags[0].CurrentAgreementId, ags[0].CurrentDeployment)
							reason := w.producerPH[msgProtocol].GetTerminationCode(producer.TERM_REASON_AGBOT_REQUESTED)
							w.cancelAgreement(replyAck.AgreementId(), msgProtocol, reason, w.producerPH[msgProtocol].GetTerminationReason(reason))

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
							w.cancelAgreement(canReceived.AgreementId(), msgProtocol, canReceived.Reason(), w.producerPH[msgProtocol].GetTerminationReason(canReceived.Reason()))
							// cleanup workloads if needed
							w.Messages() <- events.NewGovernanceWorkloadCancelationMessage(events.AGREEMENT_ENDED, events.AG_TERMINATED, ags[0].AgreementProtocol, ags[0].CurrentAgreementId, ags[0].CurrentDeployment)
							deleteMessage = true

						}

						// Allow the message extension handler to see the message
						if handled, err := w.producerPH[msgProtocol].HandleExtensionMessages(&cmd.Msg, exchangeMsg); err != nil {
							glog.Errorf(logString(fmt.Sprintf("unable to handle extension message %v , error: %v", protocolMsg, err)))
						} else if handled {
							deleteMessage = handled
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
								glog.Infof(logString(fmt.Sprintf("terminating agreement %v because it has been cancelled on the blockchain.", ags[0].CurrentAgreementId)))
								w.cancelAgreement(ags[0].CurrentAgreementId, ags[0].AgreementProtocol, uint(reason), w.producerPH[protocol].GetTerminationReason(uint(reason)))
								// cleanup workloads if needed
								w.Messages() <- events.NewGovernanceWorkloadCancelationMessage(events.AGREEMENT_ENDED, events.AG_TERMINATED, ags[0].AgreementProtocol, ags[0].CurrentAgreementId, ags[0].CurrentDeployment)
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
							} else if err := w.finalizeAgreement(ags[0], w.producerPH[protocol].AgreementProtocolHandler(ags[0].BlockchainType, ags[0].BlockchainName)); err != nil {
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

				case *producer.BCInitializedCommand:
					cmd, _ := command.(*producer.BCInitializedCommand)
					for _, pph := range w.producerPH {
						pph.SetBlockchainClientAvailable(cmd)
					}

				case *producer.BCStoppingCommand:
					cmd, _ := command.(*producer.BCStoppingCommand)
					for _, pph := range w.producerPH {
						pph.SetBlockchainClientNotAvailable(cmd)
					}

				case *producer.BCWritableCommand:
					cmd, _ := command.(*producer.BCWritableCommand)
					for _, pph := range w.producerPH {
						pph.SetBlockchainWritable(cmd)
						pph.UpdateConsumers()
					}

				case *AsyncTerminationCommand:
					cmd, _ := command.(*AsyncTerminationCommand)
					if ags, err := persistence.FindEstablishedAgreements(w.db, cmd.AgreementProtocol, []persistence.EAFilter{persistence.IdEAFilter(cmd.AgreementId)}); err != nil {
						glog.Errorf(logString(fmt.Sprintf("unable to retrieve agreement %v from database, error %v", cmd.AgreementId, err)))
					} else if len(ags) != 1 {
						glog.V(5).Infof(logString(fmt.Sprintf("ignoring command, not our agreement id")))
					} else if w.producerPH[cmd.AgreementProtocol].IsBlockchainWritable(&ags[0]) {
						glog.Infof(logString(fmt.Sprintf("external agreement termination of %v reason %v.", cmd.AgreementId, cmd.Reason)))
						w.externalTermination(&ags[0], cmd.AgreementId, cmd.AgreementProtocol, cmd.Reason)
					} else {
						deferredCommands = append(deferredCommands, cmd)
					}
				case *StartMicroserviceCommand:
					cmd, _ := command.(*StartMicroserviceCommand)

					if err := w.StartMicroservice(cmd.MsDefKey); err != nil {
						glog.Errorf(logString(fmt.Sprintf("Error starting microservice. %v", err)))
					}

				case *UpdateMicroserviceInstanceCommand:
					cmd, _ := command.(*UpdateMicroserviceInstanceCommand)

					// update the execution status for microservice instance
					glog.V(5).Infof(logString(fmt.Sprintf("Updating microservice execution status %v", cmd)))
					if _, err := persistence.UpdateMSInstanceExecutionState(w.db, cmd.MsInstKey, cmd.ExecutionStarted, cmd.ExecutionFailureCode, cmd.ExecutionFailureDesc); err != nil {
						glog.Errorf(logString(fmt.Sprintf("Error updating microservice execution status. %v", err)))
					}

					if !cmd.ExecutionStarted {
						// for execution failur, we need to check if it is time to delete it and restart a new microservice
						if ms, err := persistence.FindMicroserviceInstanceWithKey(w.db, cmd.MsInstKey); err != nil {
							glog.Errorf(logString(fmt.Sprintf("Error retrieving microservice instance %v from the db. %v", cmd.MsInstKey, err)))
						} else if ms != nil {
							if err := w.CleanupMicroservice(ms.SpecRef, ms.Version, cmd.MsInstKey, true); err != nil {
								glog.Errorf(logString(fmt.Sprintf("Error restarting microservice instance %v. %v", cmd.MsInstKey, err)))
							}
						}
					}

				default:
					glog.Errorf("GovernanceWorker received unknown command (%T): %v", command, command)
				}
				glog.V(5).Infof("GovernanceWorker handled command")

			case <-time.After(time.Duration(10) * time.Second):
				// Make sure that all known agreements are maintained
				w.governAgreements()

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
	} else if err := recordProducerAgreementState(w.Config.Edge.ExchangeURL, w.deviceId, w.deviceToken, agreement.CurrentAgreementId, &tcPolicy.APISpecs, "Finalized Agreement"); err != nil {
		return errors.New(logString(fmt.Sprintf("error setting agreement %v finalized state in exchange: %v", agreement.CurrentAgreementId, err)))
	}

	return nil
}

func (w *GovernanceWorker) RecordReply(proposal abstractprotocol.Proposal, protocol string) error {

	// Update the state in the database
	if ag, err := persistence.AgreementStateAccepted(w.db, proposal.AgreementId(), protocol); err != nil {
		return errors.New(logString(fmt.Sprintf("received error updating database state, %v", err)))

		// Update the state in the exchange
	} else if tcPolicy, err := policy.DemarshalPolicy(proposal.TsAndCs()); err != nil {
		return errors.New(logString(fmt.Sprintf("received error demarshalling TsAndCs, %v", err)))
	} else if err := recordProducerAgreementState(w.Config.Edge.ExchangeURL, w.deviceId, w.deviceToken, proposal.AgreementId(), &tcPolicy.APISpecs, "Agree to proposal"); err != nil {
		return errors.New(logString(fmt.Sprintf("received error setting state for agreement %v", err)))
	} else {
		// Publish the "agreement reached" event to the message bus so that torrent can start downloading the workload
		// hash is same as filename w/out extension
		hashes := make(map[string]string, 0)
		signatures := make(map[string]string, 0)
		workload := tcPolicy.NextHighestPriorityWorkload(0, 0, 0)
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

			// Before the ms split, the attributes assigned to the service (sensorUrl) are added to the workload.
			// After the split, the workload config variables are stored in the workload config database.
			if workload.WorkloadURL == "" {
				sensorUrl := tcPolicy.APISpecs[0].SpecRef
				if envAdds, err = w.GetWorkloadPreference(sensorUrl); err != nil {
					glog.Errorf("Error: %v", err)
				}
			} else {
				if envAdds, err = w.GetWorkloadConfig(workload.WorkloadURL, workload.Version); err != nil {
					glog.Errorf("Error: %v", err)
				}
				// The workload config we have might be from a lower version of the workload. Go to the exchange and
				// get the metadata for the version we are running and then add in any unset default user inputs.
				if exWkld, err := exchange.GetWorkload(workload.WorkloadURL, workload.Version, workload.Arch, w.Config.Edge.ExchangeURL, w.deviceId, w.deviceToken); err != nil {
					return errors.New(logString(fmt.Sprintf("received error querying excahnge for workload metadata, error %v", err)))
				} else {
					for _, ui := range exWkld.UserInputs {
						if ui.DefaultValue != "" {
							if _, ok := envAdds[ui.Name]; !ok {
								envAdds[ui.Name] = ui.DefaultValue
							}
						}
					}
				}
			}

			envAdds[config.ENVVAR_PREFIX+"AGREEMENTID"] = proposal.AgreementId()
			envAdds[config.ENVVAR_PREFIX+"DEVICE_ID"] = w.deviceId
			envAdds[config.ENVVAR_PREFIX+"HASH"] = workload.WorkloadPassword

			// Add in the exchange URL so that the workload knows which ecosystem its part of
			envAdds[config.ENVVAR_PREFIX+"EXCHANGE_URL"] = w.Config.Edge.ExchangeURL

			lc.EnvironmentAdditions = &envAdds
			lc.AgreementProtocol = protocol

			// get a list of microservices associated with this agreement and store them in the AgreementLaunchContext
			ms_specs := []events.MicroserviceSpec{}
			for _, as := range tcPolicy.APISpecs {
				if msdef, err := persistence.FindMicroserviceDef(w.db, as.SpecRef, as.Version); err != nil {
					return errors.New(logString(fmt.Sprintf("Error finding microservice definition from the local db for %v version %v. %v", as.SpecRef, as.Version, err)))
				} else if msdef != nil { // if msdef is nil then it is old behaviour before the ms split
					msspec := events.MicroserviceSpec{SpecRef: msdef.SpecRef, Version: msdef.Version, Arch: msdef.Arch}
					ms_specs = append(ms_specs, msspec)
				}
			}
			lc.Microservices = ms_specs

			w.Worker.Manager.Messages <- events.NewAgreementMessage(events.AGREEMENT_REACHED, lc)
		}

		// Tell the BC worker to start the BC client container(s) if we need to.
		if ag.BlockchainType != "" && ag.BlockchainName != "" {
			if overrideName := os.Getenv("CMTN_BLOCKCHAIN"); ag.BlockchainType == policy.Ethereum_bc && overrideName != "" {
				w.Worker.Manager.Messages <- events.NewNewBCContainerMessage(events.NEW_BC_CLIENT, policy.Ethereum_bc, overrideName, w.Config.Edge.ExchangeURL, w.deviceId, w.deviceToken)
			} else {
				w.Worker.Manager.Messages <- events.NewNewBCContainerMessage(events.NEW_BC_CLIENT, ag.BlockchainType, ag.BlockchainName, w.Config.Edge.ExchangeURL, w.deviceId, w.deviceToken)
			}
		}
	}

	return nil
}

// get the environmental variables for the workload (this is about launching), pre MS split
func (w *GovernanceWorker) GetWorkloadPreference(url string) (map[string]string, error) {
	attrs, err := persistence.FindApplicableAttributes(w.db, url)
	if err != nil {
		return nil, fmt.Errorf("Unable to fetch workload preferences. Err: %v", err)
	}

	return persistence.AttributesToEnvvarMap(attrs, config.ENVVAR_PREFIX)

}

// Get the environmental variables for the workload for MS split workloads. The workload config
// record has a version range. The workload has to fall within that range inorder for us to apply
// the configuration to the workload. If there are multiple configs in the version range, we will
// use the most current config we have.
func (w *GovernanceWorker) GetWorkloadConfig(url string, version string) (map[string]string, error) {

	// Filter to return workload configs with versions less than or equal to the input workload version range
	OlderWorkloadWCFilter := func(workload_url string, version string) persistence.WCFilter {
		return func(e persistence.WorkloadConfig) bool {
			// if e.WorkloadURL == workload_url && strings.Compare(e.Version, version) <= 0 {
			if vExp, err := policy.Version_Expression_Factory(e.VersionExpression); err != nil {
				return false
			} else if inRange, err := vExp.Is_within_range(version); err != nil {
				return false
			} else if e.WorkloadURL == workload_url && inRange {
				return true
			} else {
				return false
			}
		}
	}

	// Find the eligible workload config objects
	cfgs, err := persistence.FindWorkloadConfigs(w.db, []persistence.WCFilter{OlderWorkloadWCFilter(url, version)})
	if err != nil {
		return nil, fmt.Errorf("Unable to fetch post split workload preferences. Err: %v", err)
	} else if len(cfgs) == 0 {
		return persistence.ConfigToEnvvarMap(w.db, nil, config.ENVVAR_PREFIX)
	}

	// Sort them by version, oldest to newest
	sort.Sort(persistence.WorkloadConfigByVersion(cfgs))

	// Configure the env map with the newest config that is within the version range.
	return persistence.ConfigToEnvvarMap(w.db, &cfgs[len(cfgs)-1], config.ENVVAR_PREFIX)

}

func recordProducerAgreementState(url string, deviceId string, token string, agreementId string, apiSpecs *policy.APISpecList, state string) error {

	glog.V(5).Infof(logString(fmt.Sprintf("setting agreement %v state to %v", agreementId, state)))

	as := new(exchange.PutAgreementState)
	as.Microservices = apiSpecs.AsStringArray()
	as.State = state
	var resp interface{}
	resp = new(exchange.PostDeviceResponse)
	targetURL := url + "devices/" + deviceId + "/agreements/" + agreementId
	for {
		if err, tpErr := exchange.InvokeExchange(&http.Client{Timeout: time.Duration(config.HTTPDEFAULTTIMEOUT * time.Millisecond)}, "PUT", targetURL, deviceId, token, &as, &resp); err != nil {
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
		if err, tpErr := exchange.InvokeExchange(&http.Client{Timeout: time.Duration(config.HTTPDEFAULTTIMEOUT * time.Millisecond)}, "DELETE", targetURL, deviceId, token, nil, &resp); err != nil {
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

// create microservice instance and loads the containers
func (w *GovernanceWorker) StartMicroservice(ms_key string) error {
	glog.V(5).Infof(logString(fmt.Sprintf("Starting microservice instance for %v", ms_key)))
	if msdef, err := persistence.FindMicroserviceDefWithKey(w.db, ms_key); err != nil {
		return fmt.Errorf(logString(fmt.Sprintf("Error finding microserivce definition from db for %v. %v", ms_key, err)))
	} else if msdef == nil {
		return fmt.Errorf(logString(fmt.Sprintf("No microserivce definition available for %v.", ms_key)))
	} else {

		wls := msdef.Workloads
		if wls == nil || len(wls) < 1 {
			glog.Infof(logString(fmt.Sprintf("No workload needed for microservice %v.", msdef.SpecRef)))
			return nil
		}

		for _, wl := range wls {
			// convert the torrent string to a structure
			var torrent policy.Torrent
			if err := json.Unmarshal([]byte(wl.Torrent), &torrent); err != nil {
				return fmt.Errorf(logString(fmt.Sprintf("The torrent definition for microservice %v has error: %v", msdef.SpecRef, err)))
			}

			// convert workload to policy workload structure
			var ms_workload policy.Workload
			ms_workload.Deployment = wl.Deployment
			ms_workload.DeploymentSignature = wl.DeploymentSignature
			ms_workload.Torrent = torrent
			ms_workload.WorkloadPassword = ""
			ms_workload.DeploymentUserInfo = ""

			// verify torrent url
			if url, err := url.Parse(torrent.Url); err != nil {
				return fmt.Errorf("ill-formed URL: %v, error %v", torrent.Url, err)
			} else {
				hashes := make(map[string]string, 0)
				signatures := make(map[string]string, 0)

				for _, image := range torrent.Images {
					bits := strings.Split(image.File, ".")
					if len(bits) < 2 {
						return fmt.Errorf("found ill-formed image filename: %v, no file suffix found", bits)
					} else {
						hashes[image.File] = bits[0]
					}
					signatures[image.File] = image.Signature
				}

				// Verify the deployment signature
				if err := ms_workload.HasValidSignature(w.Config.Edge.PublicKeyPath, w.Config.UserPublicKeyPath()); err != nil {
					return fmt.Errorf(logString(fmt.Sprintf("microservice container has invalid deployment signature %v for %v", ms_workload.DeploymentSignature, ms_workload.Deployment)))
				}

				// save the instance
				if ms_instance, err := persistence.NewMicroserviceInstance(w.db, msdef.SpecRef, msdef.Version); err != nil {
					return fmt.Errorf(logString(fmt.Sprintf("Error persisting microservice instance for %v.", ms_key)))
				} else {
					// Fire an event to the torrent worker so that it will download the container
					cc := events.NewContainerConfig(*url, hashes, signatures, ms_workload.Deployment, ms_workload.DeploymentSignature, ms_workload.DeploymentUserInfo)

					// convert the user input from the service attributes to env variables
					if attrs, err := persistence.FindApplicableAttributes(w.db, msdef.SpecRef); err != nil {
						return fmt.Errorf(logString(fmt.Sprintf("Unable to fetch microservice preferences for %v. Err: %v", msdef.SpecRef, err)))
					} else if envAdds, err := persistence.AttributesToEnvvarMap(attrs, config.ENVVAR_PREFIX); err != nil {
						return fmt.Errorf(logString(fmt.Sprintf("Failed to convert microservice preferences to environmental variables for %v. Err: %v", msdef.SpecRef, err)))
					} else {
						envAdds[config.ENVVAR_PREFIX+"DEVICE_ID"] = w.deviceId
						envAdds[config.ENVVAR_PREFIX+"EXCHANGE_URL"] = w.Config.Edge.ExchangeURL
						// Add in any default variables from the microservice userInputs that havent been overridden
						for _, ui := range msdef.UserInputs {
							if ui.DefaultValue != "" {
								if _, ok := envAdds[ui.Name]; !ok {
									envAdds[ui.Name] = ui.DefaultValue
								}
							}
						}
						lc := events.NewContainerLaunchContext(cc, &envAdds, events.BlockchainConfig{}, ms_instance.GetKey())
						w.Messages() <- events.NewLoadContainerMessage(events.LOAD_CONTAINER, lc)
					}
				}
			}
		}
	}
	return nil
}

// create microservice instance and loads the containers
func (w *GovernanceWorker) CleanupMicroservice(spec_ref string, version string, key string, restart bool) error {
	glog.V(5).Infof(logString(fmt.Sprintf("Deleting microservice instance %v", key)))

	// archive this microservice instance in the db
	if ms_inst, err := persistence.ArchiveMicroserviceInstance(w.db, key); err != nil {
		glog.Errorf(logString(fmt.Sprintf("Error archiving microservice instance %v. %v", key, err)))
	} else if ms_inst == nil {
		glog.Errorf(logString(fmt.Sprintf("Unable to find microservice instance %v.", key)))
		// remove all the containers for agreements associated with it so that new agreements can be created over the new microservice
	} else if agreements, err := w.FindEstablishedAgreementsWithIds(ms_inst.AssociatedAgreements); err != nil {
		glog.Errorf(logString(fmt.Sprintf("Error finding agreements %v from the db. %v", ms_inst.AssociatedAgreements, err)))
	} else if agreements != nil {
		glog.V(5).Infof(logString(fmt.Sprintf("Removing all the containers for associated agreements %v", ms_inst.AssociatedAgreements)))
		for _, ag := range agreements {
			// send the event to the container so that the workloads can be deleted
			w.Messages() <- events.NewGovernanceWorkloadCancelationMessage(events.AGREEMENT_ENDED, events.AG_TERMINATED, ag.AgreementProtocol, ag.CurrentAgreementId, ag.CurrentDeployment)

			// end the agreements
			glog.V(3).Infof("Ending the agreement: %v because microservice %v is deleted", ag.CurrentAgreementId, key)
			reason_code := w.producerPH[ag.AgreementProtocol].GetTerminationCode(producer.TERM_REASON_MICROSERVICE_FAILURE)
			reason_text := w.producerPH[ag.AgreementProtocol].GetTerminationReason(reason_code)
			w.cancelAgreement(ag.CurrentAgreementId, ag.AgreementProtocol, reason_code, reason_text)
		}

		// remove all the microservice containers
		glog.V(5).Infof(logString(fmt.Sprintf("Removing all the containers for %v", key)))
		w.Messages() <- events.NewMicroserviceCancellationMessage(events.CANCEL_MICROSERVICE, key)
	}

	// restart a new microservice instance
	if restart {
		if msdef, err := persistence.FindMicroserviceDef(w.db, spec_ref, version); err != nil {
			return fmt.Errorf(logString(fmt.Sprintf("Error finding microserivce definition fron db for %v version %v. %v", spec_ref, version, err)))
		} else if msdef == nil {
			return fmt.Errorf(logString(fmt.Sprintf("No microserivce definition record in db for %v version %v. %v", spec_ref, version, err)))
		} else if err := w.StartMicroservice(msdef.GetKey()); err != nil {
			return fmt.Errorf(logString(fmt.Sprintf("Error starting microservice for %v. %v", msdef.GetKey(), err)))
		}
	}

	return nil
}

// go through all the protocols and find the agreements with given agreement ids from the db
func (w *GovernanceWorker) FindEstablishedAgreementsWithIds(agreementIds []string) ([]persistence.EstablishedAgreement, error) {

	// filter for finding all
	multiIdFilter := func(ids []string) persistence.EAFilter {
		return func(e persistence.EstablishedAgreement) bool {
			for _, id := range ids {
				if e.CurrentAgreementId == id {
					return true
				}
			}
			return false
		}
	}

	if agreementIds == nil || len(agreementIds) == 0 {
		return make([]persistence.EstablishedAgreement, 0), nil
	}

	// find all agreements in db
	var filters []persistence.EAFilter
	filters = append(filters, persistence.UnarchivedEAFilter())
	filters = append(filters, multiIdFilter(agreementIds))
	return persistence.FindEstablishedAgreementsAllProtocols(w.db, policy.AllAgreementProtocols(), filters)
}
