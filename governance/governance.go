package governance

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/abstractprotocol"
	"github.com/open-horizon/anax/cache"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/eventlog"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/metering"
	"github.com/open-horizon/anax/microservice"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/producer"
	"github.com/open-horizon/anax/worker"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// TODO: make this module more aware of long-running setup operations like image downloading and dockerfile loading
// the max time we'll let a contract remain unconfigured by the provider
const MAX_CONTRACT_UNCONFIGURED_TIME_M = 20

const MAX_CONTRACT_PRELAUNCH_TIME_M = 10

const MAX_MICROPAYMENT_UNPAID_RUN_DURATION_M = 60

// enforced only after the workloads are running
const MAX_AGREEMENT_ACCEPTANCE_WAIT_TIME_M = 20

// related to agreement cleanup status
const STATUS_WORKLOAD_DESTROYED = 500
const STATUS_AG_PROTOCOL_TERMINATED = 501

// for identifying the subworkers used by this worker
const CONTAINER_GOVERNOR = "ContainerGovernor"
const MICROSERVICE_GOVERNOR = "MicroserviceGovernor"
const BC_GOVERNOR = "BlockchainGovernor"
const SURFACEERRORS = "SurfaceExchErrors"

// Keys for the exchange errors cache in the worker
const EXCHANGE_ERRORS = "ExchangeErrors"

type GovernanceWorker struct {
	worker.BaseWorker // embedded field
	db                *bolt.DB
	devicePattern     string
	pm                *policy.PolicyManager
	producerPH        map[string]producer.ProducerProtocolHandler
	deviceStatus      *DeviceStatus
	ShuttingDownCmd   *NodeShutdownCommand
	patternChange     ChangePattern
	limitedRetryEC    exchange.ExchangeContext
	exchErrors        cache.Cache
}

func NewGovernanceWorker(name string, cfg *config.HorizonConfig, db *bolt.DB, pm *policy.PolicyManager) *GovernanceWorker {

	var ec *worker.BaseExchangeContext
	var lrec exchange.ExchangeContext
	pattern := ""
	if dev, _ := persistence.FindExchangeDevice(db); dev != nil {
		ec = worker.NewExchangeContext(fmt.Sprintf("%v/%v", dev.Org, dev.Id), dev.Token, cfg.Edge.ExchangeURL, cfg.GetCSSURL(), cfg.Collaborators.HTTPClientFactory)
		pattern = dev.Pattern
		lrec = newLimitedRetryExchangeContext(ec)
	}

	worker := &GovernanceWorker{
		BaseWorker:      worker.NewBaseWorker(name, cfg, ec),
		db:              db,
		pm:              pm,
		devicePattern:   pattern,
		producerPH:      make(map[string]producer.ProducerProtocolHandler),
		deviceStatus:    NewDeviceStatus(),
		ShuttingDownCmd: nil,
		limitedRetryEC:  lrec,
		exchErrors:      cache.NewSimpleMapCache(),
	}

	// Start the worker and set the no work interval to 10 seconds.
	worker.Start(worker, 10)
	return worker
}

func newLimitedRetryExchangeContext(baseEC *worker.BaseExchangeContext) exchange.ExchangeContext {
	limitedRetryHTTPFactory := &config.HTTPClientFactory{
		NewHTTPClient: baseEC.HTTPFactory.NewHTTPClient,
		RetryCount:    1,
		RetryInterval: 5,
	}

	return exchange.NewCustomExchangeContext(baseEC.Id, baseEC.Token, baseEC.URL, baseEC.CSSURL, limitedRetryHTTPFactory)
}

func (w *GovernanceWorker) Messages() chan events.Message {
	return w.BaseWorker.Manager.Messages
}

func (w *GovernanceWorker) NewEvent(incoming events.Message) {

	switch incoming.(type) {
	case *events.EdgeRegisteredExchangeMessage:
		msg, _ := incoming.(*events.EdgeRegisteredExchangeMessage)
		w.EC = worker.NewExchangeContext(fmt.Sprintf("%v/%v", msg.Org(), msg.DeviceId()), msg.Token(), w.Config.Edge.ExchangeURL, w.Config.GetCSSURL(), w.Config.Collaborators.HTTPClientFactory)
		w.devicePattern = msg.Pattern()
		w.limitedRetryEC = newLimitedRetryExchangeContext(w.EC)

	case *events.EdgeConfigCompleteMessage:
		// Start any services that run without needing an agreement.
		cmd := w.NewStartAgreementLessServicesCommand()
		w.Commands <- cmd

	case *events.WorkloadMessage:
		msg, _ := incoming.(*events.WorkloadMessage)

		switch msg.Event().Id {
		case events.EXECUTION_BEGUN:
			glog.Infof(logString(fmt.Sprintf("Begun execution of containers according to agreement %v", msg.AgreementId)))

			cmd := w.NewStartGovernExecutionCommand(msg.Deployment, msg.AgreementProtocol, msg.AgreementId)
			w.Commands <- cmd
		case events.EXECUTION_FAILED:
			cmd := w.NewCleanupExecutionCommand(msg.AgreementProtocol, msg.AgreementId, w.producerPH[msg.AgreementProtocol].GetTerminationCode(producer.TERM_REASON_CONTAINER_FAILURE), msg.Deployment)
			w.Commands <- cmd
		case events.IMAGE_LOAD_FAILED:
			cmd := w.NewCleanupExecutionCommand(msg.AgreementProtocol, msg.AgreementId, w.producerPH[msg.AgreementProtocol].GetTerminationCode(producer.TERM_REASON_WL_IMAGE_LOAD_FAILURE), msg.Deployment)
			w.Commands <- cmd
		case events.WORKLOAD_DESTROYED:
			cmd := w.NewCleanupStatusCommand(msg.AgreementProtocol, msg.AgreementId, STATUS_WORKLOAD_DESTROYED)
			w.Commands <- cmd
		}

		cmd := w.NewReportDeviceStatusCommand()
		w.Commands <- cmd

	case *events.ImageFetchMessage:
		msg, _ := incoming.(*events.ImageFetchMessage)

		switch msg.LaunchContext.(type) {
		case *events.AgreementLaunchContext:
			var reason uint
			lc := msg.LaunchContext.(*events.AgreementLaunchContext)

			// get reason code from different image fetch error
			switch msg.Event().Id {
			case events.IMAGE_FETCHED:
				reason = 0
			case events.IMAGE_DATA_ERROR:
				reason = w.producerPH[lc.AgreementProtocol].GetTerminationCode(producer.TERM_REASON_IMAGE_DATA_ERROR)
			case events.IMAGE_FETCH_ERROR:
				reason = w.producerPH[lc.AgreementProtocol].GetTerminationCode(producer.TERM_REASON_IMAGE_FETCH_FAILURE)
			case events.IMAGE_FETCH_AUTH_ERROR:
				reason = w.producerPH[lc.AgreementProtocol].GetTerminationCode(producer.TERM_REASON_IMAGE_FETCH_AUTH_FAILURE)
			case events.IMAGE_SIG_VERIF_ERROR:
				reason = w.producerPH[lc.AgreementProtocol].GetTerminationCode(producer.TERM_REASON_IMAGE_SIG_VERIF_FAILURE)
			default:
				reason = w.producerPH[lc.AgreementProtocol].GetTerminationCode(producer.TERM_REASON_IMAGE_FETCH_FAILURE)
			}

			if ags, err := persistence.FindEstablishedAgreements(w.db, lc.AgreementProtocol, []persistence.EAFilter{persistence.UnarchivedEAFilter(), persistence.IdEAFilter(lc.AgreementId)}); err != nil {
				glog.Errorf(logString(fmt.Sprintf("unable to retrieve agreement %v from database, error %v", lc.AgreementId, err)))
				eventlog.LogDatabaseEvent(w.db, persistence.SEVERITY_ERROR,
					persistence.NewMessageMeta(EL_GOV_ERR_RETRIEVE_AG_FROM_DB, lc.AgreementId, err.Error()),
					persistence.EC_DATABASE_ERROR)
			} else if len(ags) != 1 {
				glog.Warningf(logString(fmt.Sprintf("unable to retrieve single agreement %v from database.", lc.AgreementId)))
			} else {
				if reason == 0 {
					eventlog.LogAgreementEvent(
						w.db,
						persistence.SEVERITY_INFO,
						persistence.NewMessageMeta(EL_GOV_IMAGE_LOADED, ags[0].RunningWorkload.Org, ags[0].RunningWorkload.URL),
						fmt.Sprintf(persistence.EC_IMAGE_LOADED),
						ags[0])
				} else {
					eventlog.LogAgreementEvent(
						w.db,
						persistence.SEVERITY_ERROR,
						persistence.NewMessageMeta(EL_GOV_ERR_LOADING_IMG, ags[0].RunningWorkload.Org, ags[0].RunningWorkload.URL),
						persistence.EC_ERROR_IMAGE_LOADE,
						ags[0])
					cmd := w.NewCleanupExecutionCommand(lc.AgreementProtocol, lc.AgreementId, reason, nil)
					w.Commands <- cmd
				}
			}
		case *events.ContainerLaunchContext:
			lc := msg.LaunchContext.(*events.ContainerLaunchContext)

			if msg.Event().Id == events.IMAGE_FETCHED {
				eventlog.LogServiceEvent2(
					w.db,
					persistence.SEVERITY_INFO,
					persistence.NewMessageMeta(EL_GOV_IMAGE_LOADED_FOR_SVC, lc.ServicePathElement.Org, lc.ServicePathElement.URL),
					persistence.EC_IMAGE_LOADED,
					"", lc.ServicePathElement.URL, "", lc.ServicePathElement.Version, "", lc.AgreementIds)
			} else {
				eventlog.LogServiceEvent2(
					w.db,
					persistence.SEVERITY_ERROR,
					persistence.NewMessageMeta(EL_GOV_ERR_LOADING_IMG_FOR_SVC, lc.ServicePathElement.Org, lc.ServicePathElement.URL),
					persistence.EC_ERROR_IMAGE_LOADE,
					"", lc.ServicePathElement.URL, "", lc.ServicePathElement.Version, "", lc.AgreementIds)
				cmd := w.NewUpdateMicroserviceCommand(lc.Name, false, microservice.MS_IMAGE_FETCH_FAILED, microservice.DecodeReasonCode(microservice.MS_IMAGE_FETCH_FAILED))
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

	case *events.ContainerMessage:
		msg, _ := incoming.(*events.ContainerMessage)
		if msg.LaunchContext.Blockchain.Name == "" { // microservice case
			switch msg.Event().Id {
			case events.EXECUTION_BEGUN:
				cmd := w.NewUpdateMicroserviceCommand(msg.LaunchContext.Name, true, 0, "")
				w.Commands <- cmd
			case events.EXECUTION_FAILED:
				cmd := w.NewUpdateMicroserviceCommand(msg.LaunchContext.Name, false, microservice.MS_EXEC_FAILED, microservice.DecodeReasonCode(microservice.MS_EXEC_FAILED))
				w.Commands <- cmd
			case events.IMAGE_LOAD_FAILED:
				cmd := w.NewUpdateMicroserviceCommand(msg.LaunchContext.Name, false, microservice.MS_IMAGE_LOAD_FAILED, microservice.DecodeReasonCode(microservice.MS_IMAGE_LOAD_FAILED))
				w.Commands <- cmd
			}

			cmd := w.NewReportDeviceStatusCommand()
			w.Commands <- cmd
		}
	case *events.MicroserviceContainersDestroyedMessage:
		msg, _ := incoming.(*events.MicroserviceContainersDestroyedMessage)

		switch msg.Event().Id {
		case events.CONTAINER_DESTROYED:
			cmd := w.NewUpdateMicroserviceCommand(msg.MsInstKey, false, 0, "")
			w.Commands <- cmd
		}

		cmd := w.NewReportDeviceStatusCommand()
		w.Commands <- cmd

	case *events.NodeShutdownMessage:

		msg, _ := incoming.(*events.NodeShutdownMessage)
		cmd := w.NewNodeShutdownCommand(msg)
		w.Commands <- cmd

	case *events.NodeShutdownCompleteMessage:
		msg, _ := incoming.(*events.NodeShutdownCompleteMessage)
		switch msg.Event().Id {
		case events.UNCONFIGURE_COMPLETE:
			w.Commands <- worker.NewTerminateCommand("shutdown")
		}

	case *events.NodeHeartbeatStateChangeMessage:
		msg, _ := incoming.(*events.NodeHeartbeatStateChangeMessage)
		switch msg.Event().Id {
		case events.NODE_HEARTBEAT_RESTORED:
			cmd := w.NewNodeHeartbeatRestoredCommand()
			w.Commands <- cmd

			// Make sure device status is up to date since heartbeating is now restored. It means connectivity to
			// the exchange has been out but is now working again.
			w.Commands <- w.NewReportDeviceStatusCommand()

			// Now that heartbeating is restored, fire the functions to check on exchange state changes. If the node
			// was offline long enough, the exchange might have pruned changes we needed to see, which means we will
			// never see them now. So, assume there were some changes we care about.
			w.Commands <- NewNodeErrorChangeCommand()
			w.Commands <- NewServiceChangeCommand()

		}

	case *events.ServiceConfigStateChangeMessage:
		msg, _ := incoming.(*events.ServiceConfigStateChangeMessage)
		switch msg.Event().Id {
		case events.SERVICE_SUSPENDED:
			cmd := w.NewServiceSuspendedCommand(msg.ServiceConfigState)
			w.Commands <- cmd
		}

	case *events.UpdatePolicyMessage:
		msg, _ := incoming.(*events.UpdatePolicyMessage)
		switch msg.Event().Id {
		case events.UPDATE_POLICY:
			// Ignore these update events until the node config is complete
			if w.GetExchangeToken() != "" {
				cmd := w.NewUpdatePolicyCommand(msg)
				w.Commands <- cmd
			}
		}

	case *events.NodePolicyMessage:
		msg, _ := incoming.(*events.NodePolicyMessage)
		switch msg.Event().Id {
		case events.UPDATE_POLICY, events.DELETED_POLICY:
			w.Commands <- NewNodePolicyChangedCommand(msg)
		}

	case *events.NodeUserInputMessage:
		msg, _ := incoming.(*events.NodeUserInputMessage)
		switch msg.Event().Id {
		case events.UPDATE_NODE_USERINPUT:
			w.Commands <- NewNodeUserInputChangedCommand(msg)
		}

	case *events.NodePatternMessage:
		msg, _ := incoming.(*events.NodePatternMessage)
		switch msg.Event().Id {
		case events.NODE_PATTERN_CHANGE_SHUTDOWN, events.NODE_PATTERN_CHANGE_REREG:
			w.Commands <- NewNodePatternChangedCommand(msg)
		}

	case *events.ExchangeChangeMessage:
		msg, _ := incoming.(*events.ExchangeChangeMessage)
		switch msg.Event().Id {
		case events.CHANGE_NODE_ERROR_TYPE:
			w.Commands <- NewNodeErrorChangeCommand()
		case events.CHANGE_SERVICE_TYPE:
			w.Commands <- NewServiceChangeCommand()
		}

	default: //nothing
	}

	glog.V(4).Infof(logString(fmt.Sprintf("command channel length %v added", len(w.Commands))))

	return
}

// Make sure that every agreement we have is in a valid state or proceeding to valid states in a timely fashion. If not,
// cancel the agreement and allow the agbots to re-make them if necessary.
func (w *GovernanceWorker) governAgreements() {

	glog.V(3).Infof(logString(fmt.Sprintf("governing pending agreements")))

	// Create a new filter for unfinalized agreements
	notYetFinalFilter := func() persistence.EAFilter {
		return func(a persistence.EstablishedAgreement) bool {
			return a.AgreementCreationTime != 0 && a.AgreementTerminatedTime == 0
		}
	}

	if establishedAgreements, err := persistence.FindEstablishedAgreementsAllProtocols(w.db, policy.AllAgreementProtocols(), []persistence.EAFilter{persistence.UnarchivedEAFilter(), notYetFinalFilter()}); err != nil {
		glog.Errorf(logString(fmt.Sprintf("Unable to retrieve not yet final agreements from database. Error: %v", err)))
	} else {

		// If there are agreements in the database then we will assume that the device is already registered
		for _, ag := range establishedAgreements {
			bcType, bcName, bcOrg := w.producerPH[ag.AgreementProtocol].GetKnownBlockchain(&ag)
			protocolHandler := w.producerPH[ag.AgreementProtocol].AgreementProtocolHandler(bcType, bcName, bcOrg)
			if ag.AgreementFinalizedTime == 0 { // TODO: might need to change this to be a protocol specific check

				// Cancel the agreement if finalization doesn't occur before the timeout
				glog.V(5).Infof(logString(fmt.Sprintf("checking agreement %v for finalization.", ag.CurrentAgreementId)))

				// Check to see if we need to update the consumer with our blockchain specific pieces of the agreement
				if w.producerPH[ag.AgreementProtocol].IsBlockchainClientAvailable(bcType, bcName, bcOrg) && ag.AgreementBCUpdateAckTime == 0 {
					w.producerPH[ag.AgreementProtocol].UpdateConsumer(&ag)
				}

				// Check to see if the agreement is valid. For agreement on the blockchain, we check the blockchain directly. This call to the blockchain
				// should be very fast if the client is up and running. For other agreements, send a message to the agbot to get the agbot's opinion
				// on the agreement.
				// Remember, the device might have been down for some time and/or restarted, causing it to miss events on the blockchain.
				if w.producerPH[ag.AgreementProtocol].IsBlockchainClientAvailable(bcType, bcName, bcOrg) && w.producerPH[ag.AgreementProtocol].IsAgreementVerifiable(&ag) {

					if recorded, err := w.producerPH[ag.AgreementProtocol].VerifyAgreement(&ag); err != nil {
						glog.Errorf(logString(fmt.Sprintf("encountered error verifying agreement %v, error %v", ag.CurrentAgreementId, err)))
						eventlog.LogAgreementEvent(w.db, persistence.SEVERITY_ERROR,
							persistence.NewMessageMeta(EL_GOV_ERR_AG_VERIFICATION, ag.RunningWorkload.URL, err.Error()),
							persistence.EC_ERROR_AGREEMENT_VERIFICATION,
							ag)
					} else {
						if recorded {
							if err := w.finalizeAgreement(ag, protocolHandler); err != nil {
								glog.Errorf(logString(err.Error()))
							} else {
								continue
							}
						}
					}
				}
				// If we fall through to here, then the agreement is Not finalized yet, check for a timeout.
				now := uint64(time.Now().Unix())
				if ag.AgreementCreationTime+w.BaseWorker.Manager.Config.Edge.AgreementTimeoutS < now {
					// Start timing out the agreement
					glog.V(3).Infof(logString(fmt.Sprintf("detected agreement %v timed out.", ag.CurrentAgreementId)))

					reason := w.producerPH[ag.AgreementProtocol].GetTerminationCode(producer.TERM_REASON_NOT_FINALIZED_TIMEOUT)
					event_code := persistence.EC_CANCEL_AGREEMENT_EXECUTION_TIMEOUT
					if ag.AgreementAcceptedTime == 0 {
						reason = w.producerPH[ag.AgreementProtocol].GetTerminationCode(producer.TERM_REASON_NO_REPLY_ACK)
						event_code = persistence.EC_CANCEL_AGREEMENT_NO_REPLYACK
					}

					eventlog.LogAgreementEvent(w.db, persistence.SEVERITY_INFO,
						persistence.NewMessageMeta(EL_GOV_START_TERM_AG_WITH_REASON, ag.RunningWorkload.URL, w.producerPH[ag.AgreementProtocol].GetTerminationReason(reason)),
						event_code, ag)
					w.cancelGovernedAgreement(&ag, reason)
				}

			} else {
				// For finalized agreements, make sure the workload has been started in time.
				if ag.AgreementExecutionStartTime == 0 {
					// workload not started yet and in an agreement ...
					if (int64(ag.AgreementAcceptedTime) + (MAX_CONTRACT_PRELAUNCH_TIME_M * 60)) < time.Now().Unix() {
						glog.Infof(logString(fmt.Sprintf("terminating agreement %v because it hasn't been launched in max allowed time. This could be because of a workload failure.", ag.CurrentAgreementId)))
						reason := w.producerPH[ag.AgreementProtocol].GetTerminationCode(producer.TERM_REASON_NOT_EXECUTED_TIMEOUT)
						eventlog.LogAgreementEvent(w.db, persistence.SEVERITY_INFO,
							persistence.NewMessageMeta(EL_GOV_START_TERM_AG_WITH_REASON, ag.RunningWorkload.URL, w.producerPH[ag.AgreementProtocol].GetTerminationReason(reason)),
							persistence.EC_CANCEL_AGREEMENT_EXECUTION_TIMEOUT, ag)
						w.cancelGovernedAgreement(&ag, reason)
					}
				} else {
					// Finalized agreements could become out of policy if the policy changes on the node. Verify that the existing agreement
					// is still in policy. To check this we have to get the original proposal and compare it for compatibility against the policies
					// as they currently exist on the node.

					if proposal, err := protocolHandler.DemarshalProposal(ag.Proposal); err != nil {
						glog.Errorf(logString(fmt.Sprintf("encountered error demarshalling proposal for agreement %v, error %v", ag.CurrentAgreementId, err)))

					} else if tcPolicy, err := policy.DemarshalPolicy(proposal.TsAndCs()); err != nil {
						glog.Errorf(logString(fmt.Sprintf("unable to  demarshal TsAndCs of agreement %v, error %v", ag.CurrentAgreementId, err)))

					} else if tcPolicy.PatternId != "" {
						// Agreements that are based on patterns cannot become "out of policy" because there is no policy compatibility defined
						// between nodes and agbots.
						continue

					} else if pol, err := policy.DemarshalPolicy(proposal.ProducerPolicy()); err != nil {
						glog.Errorf(logString(fmt.Sprintf("unable to demarshal policy for agreement %v, error %v", ag.CurrentAgreementId, err)))

					} else if policies, err := w.pm.GetPolicyList(exchange.GetOrg(w.GetExchangeId()), pol); err != nil {
						glog.Errorf(logString(fmt.Sprintf("unable to get policy list for producer policy in agreement %v, error %v", ag.CurrentAgreementId, err)))

					} else if mergedPolicy, err := w.pm.MergeAllProducers(&policies, pol); err != nil {
						glog.Errorf(logString(fmt.Sprintf("unable to merge producer policies for agreement %v, error %v", ag.CurrentAgreementId, err)))

					} else if mergedPolicy == nil {
						// When patterns are in use the producer policy is empty.
						glog.Errorf(logString(fmt.Sprintf("for %v, merged policy was based on %v, but results in a nil merged policy", ag.CurrentAgreementId, policies)))
						continue

					} else if err := policy.Are_Compatible(mergedPolicy, tcPolicy, nil); err != nil {

						glog.V(5).Infof(logString(fmt.Sprintf("TsAndCs: %v", tcPolicy.ShortString())))
						glog.V(5).Infof(logString(fmt.Sprintf("Merged Policy: %v", mergedPolicy.ShortString())))

						// The proposal for this agreement is no longer compatible with the node's policy, so cancel the agreement.
						glog.V(3).Infof(logString(fmt.Sprintf("current proposal for %v is out of policy: %v", ag.CurrentAgreementId, err)))

						reason := w.producerPH[ag.AgreementProtocol].GetTerminationCode(producer.TERM_REASON_POLICY_CHANGED)
						eventlog.LogAgreementEvent(w.db, persistence.SEVERITY_INFO,
							persistence.NewMessageMeta(EL_GOV_START_TERM_AG_WITH_REASON, ag.RunningWorkload.URL, w.producerPH[ag.AgreementProtocol].GetTerminationReason(reason)),
							persistence.EC_CANCEL_AGREEMENT_POLICY_CHANGED, ag)
						w.cancelGovernedAgreement(&ag, reason)

					} else {
						glog.V(5).Infof(logString(fmt.Sprintf("agreement %v is still in policy.", ag.CurrentAgreementId)))
					}
				}
			}
		}
	}
}

// Perform the common agreement cancelation steps.
// TODO: consolidate every place that does the same thing as this function to call this function instead.
func (w *GovernanceWorker) cancelGovernedAgreement(ag *persistence.EstablishedAgreement, reason uint) {

	w.cancelAgreement(ag.CurrentAgreementId, ag.AgreementProtocol, reason, w.producerPH[ag.AgreementProtocol].GetTerminationReason(reason))

	// cleanup workloads
	w.Messages() <- events.NewGovernanceWorkloadCancelationMessage(events.AGREEMENT_ENDED, events.AG_TERMINATED, ag.AgreementProtocol, ag.CurrentAgreementId, ag.GetDeploymentConfig())

	// clean up microservice instances if needed
	w.handleMicroserviceInstForAgEnded(ag.CurrentAgreementId, false)
}

// Make sure the workload containers are all running, by asking the container worker to verify.
func (w *GovernanceWorker) governContainers() int {

	// go govern
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
			w.Messages() <- events.NewGovernanceMaintenanceMessage(events.CONTAINER_MAINTAIN, ag.AgreementProtocol, ag.CurrentAgreementId, ag.GetDeploymentConfig())

		}
	}
	return 0
}

func (w *GovernanceWorker) reportBlockchains() int {

	// go govern
	glog.Info(logString(fmt.Sprintf("started blockchain need governance")))

	// Find all agreements that need a blockchain by searching through all the agreement protocol DB buckets
	for _, agp := range policy.AllAgreementProtocols() {

		// If the agreement protocol doesnt require a blockchain then we can skip it.
		if bcType := policy.RequiresBlockchainType(agp); bcType == "" {
			continue
		} else {

			// Make a map of all blockchain orgs and names that we need to have running
			neededBCs := make(map[string]map[string]bool)
			if agreements, err := persistence.FindEstablishedAgreements(w.db, agp, []persistence.EAFilter{persistence.UnarchivedEAFilter()}); err == nil {
				for _, ag := range agreements {
					_, bcName, bcOrg := w.producerPH[agp].GetKnownBlockchain(&ag)
					if bcName != "" {
						if _, ok := neededBCs[bcOrg]; !ok {
							neededBCs[bcOrg] = make(map[string]bool)
						}
						neededBCs[bcOrg][bcName] = true
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
	return 0
}

// It cancels the given agreement. Please take note that the system is very asynchronous. It is
// possible for multiple cancellations to occur in the time it takes to actually stop workloads and
// cancel on the blockchain, therefore this code needs to be prepared to run multiple times for the
// same agreement id.
func (w *GovernanceWorker) cancelAgreement(agreementId string, agreementProtocol string, reason uint, desc string) {

	var ag *persistence.EstablishedAgreement

	// This function could be called the first time an agreement is cancelled and it could be called from
	// the deferred queue after the agreement has been archived. Find the agreement using the find API
	// so that we will find it even if it has already been archived.
	filters := make([]persistence.EAFilter, 0)
	filters = append(filters, persistence.IdEAFilter(agreementId))
	if agreements, err := persistence.FindEstablishedAgreements(w.db, agreementProtocol, filters); err != nil {
		glog.Errorf(logString(fmt.Sprintf("error getting agreement %v from db: %v.", agreementId, err)))
		eventlog.LogDatabaseEvent(w.db, persistence.SEVERITY_ERROR,
			persistence.NewMessageMeta(EL_GOV_ERR_RETRIEVE_AG_FROM_DB, agreementId, err.Error()),
			persistence.EC_DATABASE_ERROR)
	} else if len(agreements) == 0 {
		glog.Errorf(logString(fmt.Sprintf("no record found for agreement: %v in db.", agreementId)))
	} else {
		ag = &agreements[0]

		if !ag.Archived && ag.AgreementTerminatedTime == 0 {
			// Update the database
			if _, err := persistence.AgreementStateTerminated(w.db, agreementId, uint64(reason), desc, agreementProtocol); err != nil {
				glog.Errorf(logString(fmt.Sprintf("error marking agreement %v terminated: %v.", agreementId, err)))
				eventlog.LogDatabaseEvent(w.db, persistence.SEVERITY_ERROR,
					persistence.NewMessageMeta(EL_GOV_ERR_MARK_AG_TERMINATED_IN_DB, agreementId, err.Error()),
					persistence.EC_DATABASE_ERROR)
			}
		}

		// update the exchange
		if ag.AgreementAcceptedTime != 0 {
			if err := w.deleteProducerAgreement(w.GetExchangeURL(), w.GetExchangeId(), w.GetExchangeToken(), agreementId); err != nil {
				glog.Errorf(logString(fmt.Sprintf("error deleting agreement %v in exchange: %v. Will retry.", agreementId, err)))
				eventlog.LogAgreementEvent(
					w.db,
					persistence.SEVERITY_ERROR,
					persistence.NewMessageMeta(EL_GOV_ERR_DEL_AG_IN_EXCH, ag.RunningWorkload.URL, err.Error()),
					persistence.EC_ERROR_DELETE_AGREEMENT_IN_EXCHANGE,
					*ag)

				// create deferred agreement cancelation command
				if !w.IsWorkerShuttingDown() {
					w.AddDeferredCommand(NewCancelAgreementCommand(agreementId, agreementProtocol, reason, desc))
					return
				}
			}
		}

		// If we can do the termination now, do it. Otherwise we will queue a command to do it later.
		w.externalTermination(ag, agreementId, agreementProtocol, reason)
		if !w.producerPH[agreementProtocol].IsBlockchainWritable(ag) {
			// create deferred external termination command
			w.Commands <- NewAsyncTerminationCommand(agreementId, agreementProtocol, reason)
		} else {
			eventlog.LogAgreementEvent(
				w.db,
				persistence.SEVERITY_INFO,
				persistence.NewMessageMeta(EL_GOV_COMPLETE_TERM_AG_WITH_REASON, ag.RunningWorkload.URL, desc),
				persistence.EC_AGREEMENT_CANCELED,
				*ag)
		}
	}
}

func (w *GovernanceWorker) externalTermination(ag *persistence.EstablishedAgreement, agreementId string, agreementProtocol string, reason uint) {

	// Put the rest of the cancel processing into it's own go routine. In some agreement protocols, this go
	// routine will be waiting for a blockchain cancel to run. In general it will take around 30 seconds, but could be
	// double or triple that time. This will free up the governance thread to handle other protocol messages.

	// This routine does not need to be a subworker because it will terminate on its own.
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

	// This routine does not need to be a subworker because it will terminate on its own.
	go func() {
		// If there are metering notifications, write them onto the blockchain also
		if ag.MeteringNotificationMsg != (persistence.MeteringNotification{}) && !ag.Archived {
			glog.V(3).Infof(logString(fmt.Sprintf("Writing Metering Notification %v to the blockchain for %v.", ag.MeteringNotificationMsg, agreementId)))
			bcType, bcName, bcOrg := w.producerPH[agreementProtocol].GetKnownBlockchain(ag)
			if mn := metering.ConvertFromPersistent(ag.MeteringNotificationMsg, agreementId); mn == nil {
				glog.Errorf(logString(fmt.Sprintf("error converting from persistent Metering Notification %v for %v, returned nil.", ag.MeteringNotificationMsg, agreementId)))
			} else if aph := w.producerPH[agreementProtocol].AgreementProtocolHandler(bcType, bcName, bcOrg); aph == nil {
				glog.Warningf(logString(fmt.Sprintf("cannot write meter record for %v, agreement protocol handler is not ready.", agreementId)))
			} else if err := aph.RecordMeter(agreementId, mn); err != nil {
				glog.Errorf(logString(fmt.Sprintf("error writing meter %v for agreement %v on the blockchain: %v", ag.MeteringNotificationMsg, agreementId, err)))
			}
		}
	}()

}

func (w *GovernanceWorker) Initialize() bool {

	// Wait for the device to be registered.
	for {
		if w.GetExchangeToken() != "" {
			break
		} else {
			glog.V(3).Infof(logString(fmt.Sprintf("GovernanceWorker command processor waiting for device registration")))
			time.Sleep(time.Duration(5) * time.Second)
		}
	}

	// Establish agreement protocol handlers
	for _, protocolName := range policy.AllAgreementProtocols() {
		pph := producer.CreateProducerPH(protocolName, w.BaseWorker.Manager.Config, w.db, w.pm, w)
		pph.Initialize()
		w.producerPH[protocolName] = pph
	}

	// report the device status to the exchange
	w.ReportDeviceStatus()

	// start checking for issues closed by agreements and putting updated surface errors in the exchange
	w.DispatchSubworker(SURFACEERRORS, w.surfaceErrors, w.BaseWorker.Manager.Config.Edge.SurfaceErrorCheckIntervalS, false)

	// Fire up the container governor
	w.DispatchSubworker(CONTAINER_GOVERNOR, w.governContainers, 60, false)

	// Fire up the blockchain reporter. Disabled for now.
	//w.DispatchSubworker(BC_GOVERNOR, w.reportBlockchains, 60, false)

	// Fire up the microservice governor
	w.DispatchSubworker(MICROSERVICE_GOVERNOR, w.governMicroservices, 60, false)

	// for the policy case update the exchange with the latest registeredServices
	if w.devicePattern == "" {
		w.UpdateRegisteredServicesWithAgreement()
	}

	return true

}

func (w *GovernanceWorker) CommandHandler(command worker.Command) bool {

	// Handle the domain specific commands
	// TODO: consolidate DB update cases
	switch command.(type) {
	case *StartGovernExecutionCommand:
		// TODO: update db start time and tc so it can be governed
		cmd, _ := command.(*StartGovernExecutionCommand)
		glog.V(3).Infof(logString(fmt.Sprintf("Starting governance on resources in agreement: %v", cmd.AgreementId)))

		if ag, err := persistence.AgreementStateExecutionStarted(w.db, cmd.AgreementId, cmd.AgreementProtocol); err != nil {
			glog.Errorf(logString(fmt.Sprintf("Failed to update local contract record to start governing Agreement: %v. Error: %v", cmd.AgreementId, err)))
		} else {
			eventlog.LogAgreementEvent(
				w.db,
				persistence.SEVERITY_INFO,
				persistence.NewMessageMeta(EL_GOV_WL_CONTAINER_UP, ag.RunningWorkload.Org, ag.RunningWorkload.URL),
				persistence.EC_CONTAINER_RUNNING,
				*ag)
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
			glog.V(3).Infof(logString(fmt.Sprintf("Ending the agreement: %v", agreementId)))

			eventlog.LogAgreementEvent(
				w.db,
				persistence.SEVERITY_INFO,
				persistence.NewMessageMeta(EL_GOV_START_TERM_AG_WITH_REASON, ags[0].RunningWorkload.URL, w.producerPH[cmd.AgreementProtocol].GetTerminationReason(cmd.Reason)),
				persistence.EC_CANCEL_AGREEMENT,
				ags[0])

			w.cancelAgreement(agreementId, cmd.AgreementProtocol, cmd.Reason, w.producerPH[cmd.AgreementProtocol].GetTerminationReason(cmd.Reason))

			// send the event to the container in case it has started the workloads.
			w.Messages() <- events.NewGovernanceWorkloadCancelationMessage(events.AGREEMENT_ENDED, events.AG_TERMINATED, cmd.AgreementProtocol, agreementId, cmd.Deployment)
			// clean up microservice instances if needed
			w.handleMicroserviceInstForAgEnded(agreementId, false)
		}

	case *producer.ExchangeMessageCommand:
		cmd, _ := command.(*producer.ExchangeMessageCommand)

		exchangeMsg := new(exchange.DeviceMessage)
		if err := json.Unmarshal(cmd.Msg.ExchangeMessage(), &exchangeMsg); err != nil {
			glog.Errorf(logString(fmt.Sprintf("unable to demarshal exchange device message %v, error %v", cmd.Msg.ExchangeMessage(), err)))
			return true
		} else if there, err := w.messageInExchange(exchangeMsg.MsgId); err != nil {
			glog.Errorf(logString(fmt.Sprintf("unable to get messages from the exchange, error %v", err)))
			return true
		} else if !there {
			glog.V(3).Infof(logString(fmt.Sprintf("ignoring message %v, already deleted from the exchange.", exchangeMsg.MsgId)))
			return true
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
			protocolHandler := w.producerPH[msgProtocol].AgreementProtocolHandler("", "", "")

			// ReplyAck messages could indicate that the agbot has decided not to pursue the agreement any longer.
			if replyAck, err := protocolHandler.ValidateReplyAck(protocolMsg); err == nil {
				err_log_msg := ""
				ags := []persistence.EstablishedAgreement{}
				var err error

				if ags, err = persistence.FindEstablishedAgreements(w.db, msgProtocol, []persistence.EAFilter{persistence.UnarchivedEAFilter(), persistence.IdEAFilter(replyAck.AgreementId())}); err != nil {
					glog.Errorf(logString(fmt.Sprintf("unable to retrieve agreement %v from database, error %v", replyAck.AgreementId(), err)))
					eventlog.LogDatabaseEvent(w.db, persistence.SEVERITY_ERROR,
						persistence.NewMessageMeta(EL_GOV_ERR_RETRIEVE_AG_FROM_DB_FOR_RAM, replyAck.AgreementId(), err.Error()),
						persistence.EC_DATABASE_ERROR)
				} else if len(ags) != 1 {
					glog.Warningf(logString(fmt.Sprintf("unable to retrieve single agreement %v from database.", replyAck.AgreementId())))
					err_log_msg = fmt.Sprintf("Unable to retrieve single agreement %v from database for ReplyAck message.", replyAck.AgreementId())
					deleteMessage = true
				} else {

					if replyAck.ReplyAgreementStillValid() {
						if ags[0].AgreementAcceptedTime != 0 || ags[0].AgreementTerminatedTime != 0 {
							glog.V(5).Infof(logString(fmt.Sprintf("ignoring replyack for %v because we already received one or are cancelling", replyAck.AgreementId())))
							deleteMessage = true
						} else if proposal, err := protocolHandler.DemarshalProposal(ags[0].Proposal); err != nil {
							glog.Errorf(logString(fmt.Sprintf("unable to demarshal proposal for agreement %v from database", replyAck.AgreementId())))
							err_log_msg = fmt.Sprintf("Unable to demarshal proposal for agreement %v from database", replyAck.AgreementId())
						} else if err := w.RecordReply(proposal, msgProtocol); err != nil {
							glog.Errorf(logString(fmt.Sprintf("unable to record reply %v, error: %v", replyAck, err)))
							err_log_msg = fmt.Sprintf("Unable to record reply %v, error: %v", replyAck, err)
						} else {
							deleteMessage = true
						}
					} else {
						deleteMessage = true

						eventlog.LogAgreementEvent(
							w.db,
							persistence.SEVERITY_INFO,
							persistence.NewMessageMeta(EL_GOV_REPLYACK_WILL_CANCEL_AG, ags[0].RunningWorkload.URL),
							persistence.EC_CANCEL_AGREEMENT_PER_AGBOT,
							ags[0])

						w.Messages() <- events.NewGovernanceWorkloadCancelationMessage(events.AGREEMENT_ENDED, events.AG_TERMINATED, ags[0].AgreementProtocol, ags[0].CurrentAgreementId, ags[0].GetDeploymentConfig())
						reason := w.producerPH[msgProtocol].GetTerminationCode(producer.TERM_REASON_AGBOT_REQUESTED)
						w.cancelAgreement(replyAck.AgreementId(), msgProtocol, reason, w.producerPH[msgProtocol].GetTerminationReason(reason))
						// clean up microservice instances if needed
						w.handleMicroserviceInstForAgEnded(replyAck.AgreementId(), false)
					}
				}

				if err_log_msg != "" {
					if len(ags) == 1 {
						eventlog.LogAgreementEvent(w.db, persistence.SEVERITY_ERROR,
							persistence.NewMessageMeta(EL_GOV_ERR_HANDLE_REPLYACK_MSG_FOR_AG, ags[0].RunningWorkload.URL, err_log_msg),
							persistence.EC_ERROR_PROCESSING_REPLYACT_MESSAGE, ags[0])
					} else {
						eventlog.LogAgreementEvent2(w.db, persistence.SEVERITY_ERROR,
							persistence.NewMessageMeta(EL_GOV_ERR_HANDLE_REPLYACK_MSG, err_log_msg),
							persistence.EC_ERROR_PROCESSING_REPLYACT_MESSAGE, replyAck.AgreementId(), persistence.WorkloadInfo{}, []persistence.ServiceSpec{}, "", replyAck.Protocol())
					}
				}

			} else if dataReceived, err := protocolHandler.ValidateDataReceived(protocolMsg); err == nil {

				// Data notification message indicates that the agbot has found that data is being received from the workload.
				err_log_msg := ""
				ags := []persistence.EstablishedAgreement{}
				var err error

				if ags, err = persistence.FindEstablishedAgreements(w.db, msgProtocol, []persistence.EAFilter{persistence.UnarchivedEAFilter(), persistence.IdEAFilter(dataReceived.AgreementId())}); err != nil {
					glog.Errorf(logString(fmt.Sprintf("unable to retrieve agreement %v from database, error %v", dataReceived.AgreementId(), err)))
					eventlog.LogDatabaseEvent(w.db, persistence.SEVERITY_ERROR,
						persistence.NewMessageMeta(EL_GOV_ERR_RETRIEVE_AG_FROM_DB_FOR_DRM, dataReceived.AgreementId(), err.Error()),
						persistence.EC_DATABASE_ERROR)
				} else if len(ags) != 1 {
					glog.Warningf(logString(fmt.Sprintf("unable to retrieve single agreement %v from database, error %v", dataReceived.AgreementId(), err)))
					deleteMessage = true
					err_log_msg = fmt.Sprintf("Unable to retrieve single agreement %v from database for DataReceived message.", dataReceived.AgreementId())
				} else if _, err := persistence.AgreementStateDataReceived(w.db, dataReceived.AgreementId(), msgProtocol); err != nil {
					glog.Errorf(logString(fmt.Sprintf("unable to update data received time for %v, error: %v", dataReceived.AgreementId(), err)))
					err_log_msg = fmt.Sprintf("Unable to update data received time for %v, error: %v", dataReceived.AgreementId(), err)
				} else if messageTarget, err := exchange.CreateMessageTarget(exchangeMsg.AgbotId, nil, exchangeMsg.AgbotPubKey, ""); err != nil {
					glog.Errorf(logString(fmt.Sprintf("error creating message target: %v", err)))
					err_log_msg = fmt.Sprintf("Error creating message target: %v", err)
				} else if err := protocolHandler.NotifyDataReceiptAck(dataReceived.AgreementId(), messageTarget, w.producerPH[msgProtocol].GetSendMessage()); err != nil {
					glog.Errorf(logString(fmt.Sprintf("unable to send data received ack for %v, error: %v", dataReceived.AgreementId(), err)))
					err_log_msg = fmt.Sprintf("Unable to send data received ack for %v, error: %v", dataReceived.AgreementId(), err)
				} else {
					deleteMessage = true
				}

				if err_log_msg != "" {
					if len(ags) == 1 {
						eventlog.LogAgreementEvent(w.db, persistence.SEVERITY_ERROR,
							persistence.NewMessageMeta(EL_GOV_ERR_HANDLE_DATARECEIVED_MSG_FOR_AG, ags[0].RunningWorkload.URL, err_log_msg),
							persistence.EC_ERROR_PROCESSING_DATARECEIVED_MESSAGE, ags[0])
					} else {
						eventlog.LogAgreementEvent2(w.db, persistence.SEVERITY_ERROR,
							persistence.NewMessageMeta(EL_GOV_ERR_HANDLE_DATARECEIVED_MSG, err_log_msg),
							persistence.EC_ERROR_PROCESSING_DATARECEIVED_MESSAGE, dataReceived.AgreementId(), persistence.WorkloadInfo{}, []persistence.ServiceSpec{}, "", dataReceived.Protocol())
					}
				}

			} else if mnReceived, err := protocolHandler.ValidateMeterNotification(protocolMsg); err == nil {
				// Metering notification messages indicate that the agbot is metering data sent to the data ingest.

				err_log_msg := ""
				ags := []persistence.EstablishedAgreement{}
				var err error

				if ags, err = persistence.FindEstablishedAgreements(w.db, msgProtocol, []persistence.EAFilter{persistence.UnarchivedEAFilter(), persistence.IdEAFilter(mnReceived.AgreementId())}); err != nil {
					glog.Errorf(logString(fmt.Sprintf("unable to retrieve agreement %v from database, error %v", mnReceived.AgreementId(), err)))
					eventlog.LogDatabaseEvent(w.db, persistence.SEVERITY_ERROR,
						persistence.NewMessageMeta(EL_GOV_ERR_RETRIEVE_AG_FROM_DB_FOR_MNM, mnReceived.AgreementId(), err.Error()),
						persistence.EC_DATABASE_ERROR)
				} else if len(ags) != 1 {
					glog.Warningf(logString(fmt.Sprintf("unable to retrieve single agreement %v from database, error %v", mnReceived.AgreementId(), err)))
					deleteMessage = true
					err_log_msg = fmt.Sprintf("Unable to retrieve single agreement %v from database for MeteringNotification message.", mnReceived.AgreementId())
				} else if ags[0].AgreementTerminatedTime != 0 {
					glog.V(5).Infof(logString(fmt.Sprintf("ignoring metering notification, agreement %v is terminating", mnReceived.AgreementId())))
					deleteMessage = true
					err_log_msg = fmt.Sprintf("Ignoring metering notification, agreement %v is terminating", mnReceived.AgreementId())
				} else if mn, err := metering.ConvertToPersistent(mnReceived.Meter()); err != nil {
					glog.Errorf(logString(fmt.Sprintf("unable to convert metering notification string %v to persistent metering notification for %v, error: %v", mnReceived.Meter(), mnReceived.AgreementId(), err)))
					deleteMessage = true
					err_log_msg = fmt.Sprintf("Unable to convert metering notification string %v to persistent metering notification for %v, error: %v", mnReceived.Meter(), mnReceived.AgreementId(), err)
				} else if _, err := persistence.MeteringNotificationReceived(w.db, mnReceived.AgreementId(), *mn, msgProtocol); err != nil {
					glog.Errorf(logString(fmt.Sprintf("unable to update metering notification for %v, error: %v", mnReceived.AgreementId(), err)))
					deleteMessage = true
					err_log_msg = fmt.Sprintf("unable to update metering notification for %v, error: %v", mnReceived.AgreementId(), err)
				} else {
					deleteMessage = true
				}

				if err_log_msg != "" {
					if len(ags) == 1 {
						eventlog.LogAgreementEvent(w.db, persistence.SEVERITY_ERROR,
							persistence.NewMessageMeta(EL_GOV_ERR_HANDLE_METERING_MSG_FOR_AG, ags[0].RunningWorkload.URL, err_log_msg),
							persistence.EC_ERROR_PROCESSING_METERING_NOTIFY_MESSAGE, ags[0])
					} else {
						eventlog.LogAgreementEvent2(w.db, persistence.SEVERITY_ERROR,
							persistence.NewMessageMeta(EL_GOV_ERR_HANDLE_METERING_MSG, err_log_msg),
							persistence.EC_ERROR_PROCESSING_METERING_NOTIFY_MESSAGE,
							mnReceived.AgreementId(), persistence.WorkloadInfo{}, []persistence.ServiceSpec{}, "", mnReceived.Protocol())
					}
				}

			} else if canReceived, err := protocolHandler.ValidateCancel(protocolMsg); err == nil {
				// Cancel messages indicate that the agbot wants to get rid of the agreement.

				err_log_msg := ""
				ags := []persistence.EstablishedAgreement{}
				var err error

				if ags, err = persistence.FindEstablishedAgreements(w.db, msgProtocol, []persistence.EAFilter{persistence.UnarchivedEAFilter(), persistence.IdEAFilter(canReceived.AgreementId())}); err != nil {
					glog.Errorf(logString(fmt.Sprintf("unable to retrieve agreement %v from database, error %v", canReceived.AgreementId(), err)))
					eventlog.LogDatabaseEvent(w.db, persistence.SEVERITY_ERROR,
						persistence.NewMessageMeta(EL_GOV_ERR_RETRIEVE_AG_FROM_DB_FOR_CANM, canReceived.AgreementId(), err.Error()),
						persistence.EC_DATABASE_ERROR)
				} else if len(ags) != 1 {
					glog.Warningf(logString(fmt.Sprintf("unable to retrieve single agreement %v from database, agreement not found", canReceived.AgreementId())))
					deleteMessage = true
				} else {
					eventlog.LogAgreementEvent(
						w.db,
						persistence.SEVERITY_INFO,
						persistence.NewMessageMeta(EL_GOV_NODE_RECEIVED_CANCEL_MSG, ags[0].RunningWorkload.Org, ags[0].RunningWorkload.URL, exchangeMsg.AgbotId),
						persistence.EC_RECEIVED_CANCEL_AGREEMENT_MESSAGE, ags[0])

					if exchangeMsg.AgbotId != ags[0].ConsumerId {
						glog.Warningf(logString(fmt.Sprintf("cancel ignored, cancel message for %v came from id %v but agreement is with %v", canReceived.AgreementId(), exchangeMsg.AgbotId, ags[0].ConsumerId)))
						deleteMessage = true
						err_log_msg = fmt.Sprintf("Cancel ignored, cancel message for %v came from id %v but agreement is with %v", canReceived.AgreementId(), exchangeMsg.AgbotId, ags[0].ConsumerId)
					} else if ags[0].AgreementTerminatedTime != 0 {
						glog.V(5).Infof(logString(fmt.Sprintf("ignoring cancel, agreement %v is terminating", canReceived.AgreementId())))
						deleteMessage = true
						err_log_msg = fmt.Sprintf("ignoring cancel, agreement %v is terminating", canReceived.AgreementId())
					} else {
						w.cancelAgreement(canReceived.AgreementId(), msgProtocol, canReceived.Reason(), w.producerPH[msgProtocol].GetTerminationReason(canReceived.Reason()))
						// cleanup workloads if needed
						w.Messages() <- events.NewGovernanceWorkloadCancelationMessage(events.AGREEMENT_ENDED, events.AG_TERMINATED, ags[0].AgreementProtocol, ags[0].CurrentAgreementId, ags[0].GetDeploymentConfig())
						// clean up microservice instances if needed
						w.handleMicroserviceInstForAgEnded(ags[0].CurrentAgreementId, false)
						deleteMessage = true
					}
				}

				if err_log_msg != "" {
					if len(ags) == 1 {
						eventlog.LogAgreementEvent(w.db, persistence.SEVERITY_ERROR,
							persistence.NewMessageMeta(EL_GOV_ERR_HANDLE_CANCEL_MSG_FOR_AG, ags[0].RunningWorkload.URL, err_log_msg),
							persistence.EC_ERROR_PROCESSING_CANCEL_AGREEMENT_MESSAGE, ags[0])
					} else {
						eventlog.LogAgreementEvent2(w.db, persistence.SEVERITY_ERROR,
							persistence.NewMessageMeta(EL_GOV_ERR_HANDLE_CANCEL_MSG, err_log_msg),
							persistence.EC_ERROR_PROCESSING_CANCEL_AGREEMENT_MESSAGE,
							canReceived.AgreementId(), persistence.WorkloadInfo{}, []persistence.ServiceSpec{}, "", canReceived.Protocol())
					}
				}

			} else {

				// Allow the message extension handler to see the message
				handled, cancel, agid, err := w.producerPH[msgProtocol].HandleExtensionMessages(&cmd.Msg, exchangeMsg)
				if err != nil {
					glog.Errorf(logString(fmt.Sprintf("unable to handle message %v , error: %v", protocolMsg, err)))
				} else if cancel {
					reason := w.producerPH[msgProtocol].GetTerminationCode(producer.TERM_REASON_AGBOT_REQUESTED)

					if ags, err := persistence.FindEstablishedAgreements(w.db, msgProtocol, []persistence.EAFilter{persistence.UnarchivedEAFilter(), persistence.IdEAFilter(agid)}); err != nil {
						glog.Errorf(logString(fmt.Sprintf("unable to retrieve agreement %v from database, error %v", agid, err)))
						eventlog.LogDatabaseEvent(
							w.db,
							persistence.SEVERITY_ERROR,
							persistence.NewMessageMeta(EL_GOV_ERR_RETRIEVE_AG_FROM_DB, agid, err.Error()),
							persistence.EC_DATABASE_ERROR)
					} else if len(ags) != 1 {
						glog.Warningf(logString(fmt.Sprintf("unable to retrieve single agreement %v from database, error %v", agid, err)))
						deleteMessage = true
					} else {
						eventlog.LogAgreementEvent(
							w.db,
							persistence.SEVERITY_INFO,
							persistence.NewMessageMeta(EL_GOV_AG_NOT_VALID, ags[0].RunningWorkload.URL),
							persistence.EC_CANCEL_AGREEMENT_PER_AGBOT,
							ags[0])
						w.cancelAgreement(agid, msgProtocol, reason, w.producerPH[msgProtocol].GetTerminationReason(reason))
						// cleanup workloads if needed
						w.Messages() <- events.NewGovernanceWorkloadCancelationMessage(events.AGREEMENT_ENDED, events.AG_TERMINATED, msgProtocol, agid, ags[0].GetDeploymentConfig())
						// clean up microservice instances if needed
						w.handleMicroserviceInstForAgEnded(agid, false)
					}
				}

				if handled {
					deleteMessage = handled
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

			if agreementId, termination, reason, creation, err := w.producerPH[protocol].HandleBlockchainEventMessage(cmd); err != nil {
				glog.Errorf(logString(err.Error()))
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
					w.Messages() <- events.NewGovernanceWorkloadCancelationMessage(events.AGREEMENT_ENDED, events.AG_TERMINATED, ags[0].AgreementProtocol, ags[0].CurrentAgreementId, ags[0].GetDeploymentConfig())
					// clean up microservice instances if needed
					w.handleMicroserviceInstForAgEnded(ags[0].CurrentAgreementId, false)
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
				} else if err := w.finalizeAgreement(ags[0], w.producerPH[protocol].AgreementProtocolHandler(ags[0].BlockchainType, ags[0].BlockchainName, ags[0].BlockchainOrg)); err != nil {
					glog.Errorf(logString(err.Error()))
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
				} else {
					eventlog.LogAgreementEvent(
						w.db,
						persistence.SEVERITY_INFO,
						persistence.NewMessageMeta(EL_GOV_WORKLOAD_DESTROYED, agreement.RunningWorkload.URL),
						persistence.EC_CONTAINER_STOPPED,
						*agreement)

					if agreement.AgreementProtocolTerminatedTime != 0 {
						archive = true
					}
				}
			case STATUS_AG_PROTOCOL_TERMINATED:
				if agreement, err := persistence.AgreementStateAgreementProtocolTerminated(w.db, cmd.AgreementId, cmd.AgreementProtocol); err != nil {
					glog.Errorf(logString(fmt.Sprintf("error marking agreement %v agreement protocol terminated: %v", cmd.AgreementId, err)))
				} else {
					if agreement.WorkloadTerminatedTime != 0 {
						archive = true
					}
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

				// for the policy case update the exchange with the latest registeredServices
				if w.devicePattern == "" {
					w.UpdateRegisteredServicesWithAgreement()
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
			eventlog.LogAgreementEvent(
				w.db,
				persistence.SEVERITY_INFO,
				persistence.NewMessageMeta(EL_GOV_COMPLETE_TERM_AG_WITH_REASON, ags[0].RunningWorkload.URL, cmd.Reason),
				persistence.EC_AGREEMENT_CANCELED,
				ags[0])
		} else {
			w.AddDeferredCommand(cmd)
		}
	case *CancelAgreementCommand:
		cmd, _ := command.(*CancelAgreementCommand)
		w.cancelAgreement(cmd.AgreementId, cmd.AgreementProtocol, cmd.Reason, cmd.ReasonDescription)
	case *UpdateMicroserviceCommand:
		cmd, _ := command.(*UpdateMicroserviceCommand)

		glog.V(5).Infof(logString(fmt.Sprintf("Updating service execution status %v", cmd)))

		if cmd.ExecutionStarted == false && cmd.ExecutionFailureCode == 0 {
			// the miceroservice containers were destroyed, just archive the ms instance it if it not already done
			// this part is from the CONTAINER_DESTROYED event id which was originally
			msi, err := persistence.ArchiveMicroserviceInstance(w.db, cmd.MsInstKey)
			if err != nil {
				glog.Errorf(logString(fmt.Sprintf("Error archiving service instance %v. %v", cmd.MsInstKey, err)))
			} else {
				eventlog.LogServiceEvent(w.db, persistence.SEVERITY_INFO,
					persistence.NewMessageMeta(EL_GOV_COMPLETE_CLEANUP_SVC, msi.GetKey()),
					persistence.EC_COMPLETE_CLEANUP_SERVICE,
					*msi)
			}
		} else {

			// microservice execution started or failed
			// this part is from EXECUTION_FAILED or EXECUTION_BEGUN event id

			// update the execution status for microservice instance
			if msinst, err := persistence.UpdateMSInstanceExecutionState(w.db, cmd.MsInstKey, cmd.ExecutionStarted, cmd.ExecutionFailureCode, cmd.ExecutionFailureDesc); err != nil {
				glog.Errorf(logString(fmt.Sprintf("Error updating service execution status. %v", err)))
			} else if msinst != nil {
				if msdef, err := persistence.FindMicroserviceDefWithKey(w.db, msinst.MicroserviceDefId); err != nil {
					glog.Errorf(logString(fmt.Sprintf("Error finding service definition fron db for %v version %v key %v. %v", cutil.FormOrgSpecUrl(msinst.SpecRef, msinst.Org), msinst.Version, msinst.MicroserviceDefId, err)))
				} else if msdef == nil {
					glog.Errorf(logString(fmt.Sprintf("No service definition record in db for %v version %v key %v. %v", cutil.FormOrgSpecUrl(msinst.SpecRef, msinst.Org), msinst.Version, msinst.MicroserviceDefId, err)))
				} else {
					if cmd.ExecutionStarted {
						eventlog.LogServiceEvent(w.db, persistence.SEVERITY_INFO,
							persistence.NewMessageMeta(EL_GOV_SVC_CONTAINER_STARTED, cutil.FormOrgSpecUrl(msinst.SpecRef, msinst.Org)),
							persistence.EC_COMPLETE_DEPENDENT_SERVICE,
							*msinst)
					} else {
						if msinst.CleanupStartTime == 0 { // if this is not part of the ms instance cleanup process
							// this is the case where agreement are made but microservice containers are failed
							w.handleMicroserviceExecFailure(msdef, cmd.MsInstKey)
						}
					}
				}
			}
		}
	case *UpgradeMicroserviceCommand:
		cmd, _ := command.(*UpgradeMicroserviceCommand)

		glog.V(5).Infof(logString(fmt.Sprintf("Upgrade service if needed. %v", cmd)))

		if !w.IsWorkerShuttingDown() {
			w.handleMicroserviceUpgrade(cmd.MsDefId)
		}

	case *ReportDeviceStatusCommand:
		cmd, _ := command.(*ReportDeviceStatusCommand)

		glog.V(5).Infof(logString(fmt.Sprintf("Report device status command %v", cmd)))
		if !w.IsWorkerShuttingDown() {
			w.ReportDeviceStatus()
		}

	case *NodeShutdownCommand:
		cmd, _ := command.(*NodeShutdownCommand)
		glog.V(5).Infof(logString(fmt.Sprintf("Node shutdown command %v", cmd)))

		// Remember the command until we need it again.
		shutdownHTTPRetries := 1
		shutdownHTTPInterval := 5
		w.SetWorkerShuttingDown(shutdownHTTPRetries, shutdownHTTPInterval)
		w.ShuttingDownCmd = cmd

		// Shutdown the governance subworkers. We do this to ensure that none of them wake up to do
		// something when we're shutting down (which could cause problems) because we dont need them
		// to complete the shutdown procedure.
		w.TerminateSubworkers()

	case *StartAgreementLessServicesCommand:
		cmd, _ := command.(*StartAgreementLessServicesCommand)
		glog.V(5).Infof(logString(fmt.Sprintf("%v", cmd)))

		w.startAgreementLessServices()

	case *NodeHeartbeatRestoredCommand:
		cmd, _ := command.(*NodeHeartbeatRestoredCommand)
		glog.V(5).Infof(logString(fmt.Sprintf("%v", cmd)))

		w.handleNodeHeartbeatRestored()

	case *ServiceSuspendedCommand:
		cmd, _ := command.(*ServiceSuspendedCommand)
		glog.V(5).Infof(logString(fmt.Sprintf("%v", cmd)))

		w.handleServiceSuspended(cmd.ServiceConfigState)

	case *UpdatePolicyCommand:
		cmd, _ := command.(*UpdatePolicyCommand)
		glog.V(5).Infof(logString(fmt.Sprintf("%v", cmd)))

		w.handleUpdatePolicy(cmd)

	case *NodePolicyChangedCommand:
		cmd, _ := command.(*NodePolicyChangedCommand)
		glog.V(5).Infof(logString(fmt.Sprintf("%v", cmd)))

		w.handleNodePolicyUpdated()

	case *NodeUserInputChangedCommand:
		cmd, _ := command.(*NodeUserInputChangedCommand)
		glog.V(5).Infof(logString(fmt.Sprintf("%v", cmd)))

		w.handleNodeUserInputUpdated(cmd.Msg.ServiceSpecs)

	case *NodePatternChangedCommand:
		cmd, _ := command.(*NodePatternChangedCommand)
		glog.V(5).Infof(logString(fmt.Sprintf("%v", cmd)))
		if cmd.Msg.Event().Id == events.NODE_PATTERN_CHANGE_SHUTDOWN {
			w.handleNodeExchPatternChanged(true, cmd.Msg.Pattern)
		} else if cmd.Msg.Event().Id == events.NODE_PATTERN_CHANGE_REREG {
			w.handleNodeExchPatternChanged(false, cmd.Msg.Pattern)
		}

	case *NodeErrorChangeCommand:
		exchErrors, err := exchange.GetHTTPSurfaceErrorsHandler(w.limitedRetryEC)(w.GetExchangeId())
		if err != nil {
			glog.Errorf(logString(fmt.Sprintf("Error reading surfaced errors from the exchange: %v", err)))
			w.exchErrors.Put(EXCHANGE_ERRORS, nil)
		} else {
			w.exchErrors.Put(EXCHANGE_ERRORS, exchErrors)
		}

	case *ServiceChangeCommand:
		w.governMicroserviceVersions()

	default:
		return false
	}
	return true
}

func (w *GovernanceWorker) NoWorkHandler() {

	// Make sure that all known agreements are maintained, if we're not shutting down.
	if !w.IsWorkerShuttingDown() {
		w.governAgreements()
	}

	// When all subworkers are down, start the shutdown process.
	if w.IsWorkerShuttingDown() && w.ShuttingDownCmd != nil {
		if w.AreAllSubworkersTerminated() {
			cmd := w.ShuttingDownCmd
			// This is one of the few go routines that should NOT be abstracted as a subworker.
			go w.nodeShutdown(cmd)
			w.ShuttingDownCmd = nil
		} else {
			glog.V(5).Infof(logString(fmt.Sprintf("GovernanceWorker waiting for subworkers to terminate.")))
		}
	}

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

	// for the policy case update the exchange with the latest registeredServices
	if w.devicePattern == "" {
		w.UpdateRegisteredServicesWithAgreement()
	}

	// Update state in exchange
	if proposal, err := protocolHandler.DemarshalProposal(agreement.Proposal); err != nil {
		return errors.New(logString(fmt.Sprintf("could not hydrate proposal, error: %v", err)))
	} else if tcPolicy, err := policy.DemarshalPolicy(proposal.TsAndCs()); err != nil {
		return errors.New(logString(fmt.Sprintf("error demarshalling TsAndCs policy for agreement %v, error %v", agreement.CurrentAgreementId, err)))
	} else if err := recordProducerAgreementState(w.Config.Collaborators.HTTPClientFactory.NewHTTPClient(nil), w.GetExchangeURL(), w.GetExchangeId(), w.GetExchangeToken(), w.devicePattern, agreement.CurrentAgreementId, tcPolicy, "Finalized Agreement"); err != nil {
		return errors.New(logString(fmt.Sprintf("error setting agreement %v finalized state in exchange: %v", agreement.CurrentAgreementId, err)))
	}

	return nil
}

func (w *GovernanceWorker) RecordReply(proposal abstractprotocol.Proposal, protocol string) error {

	// Update the agreement state in the database and in the exchange.
	if ag, err := persistence.AgreementStateAccepted(w.db, proposal.AgreementId(), protocol); err != nil {
		return errors.New(logString(fmt.Sprintf("received error updating database state, %v", err)))
	} else if tcPolicy, err := policy.DemarshalPolicy(proposal.TsAndCs()); err != nil {
		return errors.New(logString(fmt.Sprintf("received error demarshalling TsAndCs, %v", err)))
	} else if err := recordProducerAgreementState(w.Config.Collaborators.HTTPClientFactory.NewHTTPClient(nil), w.GetExchangeURL(), w.GetExchangeId(), w.GetExchangeToken(), w.devicePattern, proposal.AgreementId(), tcPolicy, "Agree to proposal"); err != nil {
		return errors.New(logString(fmt.Sprintf("received error setting state for agreement %v", err)))
	} else {

		eventlog.LogAgreementEvent(
			w.db,
			persistence.SEVERITY_INFO,
			persistence.NewMessageMeta(EL_GOV_AG_REACHED, ag.RunningWorkload.URL, ag.CurrentAgreementId),
			persistence.EC_AGREEMENT_REACHED,
			*ag)

		// Publish the "agreement reached" event to the message bus so that imagefetch can start downloading the workload.
		workload := tcPolicy.NextHighestPriorityWorkload(0, 0, 0)

		// get service image auths from the exchange
		img_auths := make([]events.ImageDockerAuth, 0)
		if w.Config.Edge.TrustDockerAuthFromOrg {
			if ias, err := exchange.GetHTTPServiceDockerAuthsHandler(w)(workload.WorkloadURL, workload.Org, workload.Version, workload.Arch); err != nil {
				return errors.New(logString(fmt.Sprintf("received error querying exchange for service image auths: %v, error %v", workload, err)))
			} else {
				if ias != nil {
					for _, iau_temp := range ias {
						username := iau_temp.UserName
						if username == "" {
							username = "token"
						}
						img_auths = append(img_auths, events.ImageDockerAuth{Registry: iau_temp.Registry, UserName: username, Password: iau_temp.Token})
					}
				}
			}
		}

		cc := events.NewContainerConfig(workload.Deployment, workload.DeploymentSignature, workload.DeploymentUserInfo,
			workload.ClusterDeployment, workload.ClusterDeploymentSignature, workload.DeploymentOverrides, img_auths)

		lc := new(events.AgreementLaunchContext)
		lc.Configure = *cc
		lc.AgreementId = proposal.AgreementId()
		lc.AgreementProtocol = protocol

		// get environmental settings for the workload

		// The service config variables are stored in the device's attributes.
		envAdds, err := w.GetServicePreference(workload.WorkloadURL, workload.Org, tcPolicy)
		if err != nil {
			glog.Errorf(logString(fmt.Sprintf("Error getting environment variables from node settings for %v %v: %v", workload.WorkloadURL, workload.Org, err)))
			return err
		}

		// The workload config we have might be from a lower version of the workload. Go to the exchange and
		// get the metadata for the version we are running and then add in any unset default user inputs.
		var serviceDef *exchange.ServiceDefinition
		if _, sDef, _, err := exchange.GetHTTPServiceResolverHandler(w)(workload.WorkloadURL, workload.Org, workload.Version, workload.Arch); err != nil {
			return fmt.Errorf("Received error querying exchange for service metadata: %v/%v, error %v", workload.Org, workload.WorkloadURL, err)
		} else if sDef == nil {
			return fmt.Errorf("Cound not find service metadata for %v/%v.", workload.Org, workload.WorkloadURL)
		} else {
			serviceDef = sDef
			sDef.PopulateDefaultUserInput(envAdds)
		}

		cutil.SetPlatformEnvvars(envAdds,
			config.ENVVAR_PREFIX,
			proposal.AgreementId(),
			exchange.GetId(w.GetExchangeId()),
			exchange.GetOrg(w.GetExchangeId()),
			workload.WorkloadPassword,
			w.GetExchangeURL(),
			w.devicePattern,
			w.BaseWorker.Manager.Config.GetFileSyncServiceProtocol(),
			w.BaseWorker.Manager.Config.GetFileSyncServiceAPIListen(),
			strconv.Itoa(int(w.BaseWorker.Manager.Config.GetFileSyncServiceAPIPort())))

		lc.EnvironmentAdditions = &envAdds

		// Make a list of service dependencies for this workload. For sevices, it is just the top level dependencies.
		deps := serviceDef.GetServiceDependencies()

		// Create the service instance dependency path with the workload as the root.
		instancePath := []persistence.ServiceInstancePathElement{*persistence.NewServiceInstancePathElement(workload.WorkloadURL, workload.Org, workload.Version)}

		eventlog.LogAgreementEvent(w.db, persistence.SEVERITY_INFO,
			persistence.NewMessageMeta(EL_GOV_START_DEPENDENT_SVC, ag.RunningWorkload.Org, ag.RunningWorkload.URL),
			persistence.EC_START_DEPENDENT_SERVICE,
			*ag)

		if ms_specs, err := w.processDependencies(instancePath, deps, proposal.AgreementId(), protocol); err != nil {
			eventlog.LogAgreementEvent(
				w.db,
				persistence.SEVERITY_ERROR,
				persistence.NewMessageMeta(EL_GOV_ERR_START_DEPENDENT_SVC, ag.RunningWorkload.Org, ag.RunningWorkload.URL, err.Error()),
				persistence.EC_ERROR_START_DEPENDENT_SERVICE,
				*ag)
			return err
		} else {
			// Save the list of services/microservices associated with this agreement and store them in the AgreementLaunchContext. These are
			// the services that are going to be network accessible to the workload container(s).
			lc.Microservices = ms_specs
		}

		eventlog.LogAgreementEvent(w.db, persistence.SEVERITY_INFO,
			persistence.NewMessageMeta(EL_GOV_START_WORKLOAD_SVC, ag.RunningWorkload.Org, ag.RunningWorkload.URL),
			persistence.EC_START_SERVICE,
			*ag)

		w.BaseWorker.Manager.Messages <- events.NewAgreementMessage(events.AGREEMENT_REACHED, lc)

		// Tell the BC worker to start the BC client container(s) if we need to.
		if ag.BlockchainType != "" && ag.BlockchainName != "" && ag.BlockchainOrg != "" {
			w.BaseWorker.Manager.Messages <- events.NewNewBCContainerMessage(events.NEW_BC_CLIENT, ag.BlockchainType, ag.BlockchainName, ag.BlockchainOrg, w.GetExchangeURL(), w.GetExchangeId(), w.GetExchangeToken())
		}
	}

	return nil
}

// Run through the list of service dependencies and start each one. This function is used recursively to start leaf nodes first,
// and then their parents.
func (w *GovernanceWorker) processDependencies(dependencyPath []persistence.ServiceInstancePathElement, deps *[]exchange.ServiceDependency, agreementId string, protocol string) ([]events.MicroserviceSpec, error) {
	ms_specs := []events.MicroserviceSpec{}

	glog.V(5).Infof(logString(fmt.Sprintf("processDependencies %v for agreement %v. The dependency path is: %v", *deps, agreementId, dependencyPath)))

	for _, sDep := range *deps {

		msdef, err := microservice.FindOrCreateMicroserviceDef(w.db, sDep.URL, sDep.Org, sDep.Version, sDep.Arch, exchange.GetHTTPServiceHandler(w))
		if err != nil {
			return ms_specs, fmt.Errorf(logString(fmt.Sprintf("failed to get or create service definition for dependent service for agreement %v. %v", agreementId, err)))
		}

		msspec := events.MicroserviceSpec{SpecRef: msdef.SpecRef, Org: msdef.Org, Version: msdef.Version, MsdefId: msdef.Id}
		ms_specs = append(ms_specs, msspec)

		// Recursively work down the dependency tree, starting leaf node dependencies first and then start their parents.
		fullPath := append(dependencyPath, *persistence.NewServiceInstancePathElement(msdef.SpecRef, msdef.Org, msdef.Version))
		if err := w.startDependentService(fullPath, msdef, agreementId, protocol); err != nil {
			eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_ERROR,
				persistence.NewMessageMeta(EL_GOV_ERR_START_DEPENDENT_SVC_FOR_AG, msdef.Org, msdef.SpecRef, msdef.Version, agreementId, err.Error()),
				persistence.EC_ERROR_START_DEPENDENT_SERVICE,
				"", msdef.SpecRef, msdef.Org, msdef.Version, msdef.Arch, []string{agreementId})
		}
	}

	return ms_specs, nil
}

// Function that starts leaf node service dependencies before starting parents.
func (w *GovernanceWorker) startDependentService(dependencyPath []persistence.ServiceInstancePathElement, msdef *persistence.MicroserviceDefinition, agreementId string, protocol string) error {

	// If the service has dependencies, process those before starting itself.
	if msdef.HasRequiredServices() {
		deps := microservice.ConvertRequiredServicesToExchange(msdef)
		if _, err := w.processDependencies(dependencyPath, deps, agreementId, protocol); err != nil {
			return err
		}
	}

	glog.V(5).Infof(logString(fmt.Sprintf("starting dependency: %v with def %v", dependencyPath, msdef)))
	return w.startMicroserviceInstForAgreement(msdef, agreementId, dependencyPath, protocol)
}

// Start all the agreement-less services. This function is only called when the node is running in service mode.
func (w *GovernanceWorker) startAgreementLessServices() {

	// A node that is not using a pattern cannot have agreement-less services.
	if w.devicePattern == "" {
		return
	}

	// Get the pattern definition from the exchange.
	pattern_org, pattern_name, pat := persistence.GetFormatedPatternString(w.devicePattern, "")
	patternDef, err := exchange.GetHTTPExchangePatternHandler(w)(pattern_org, pattern_name)
	if err != nil {
		eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_ERROR,
			persistence.NewMessageMeta(EL_GOV_ERR_START_AGLESS_SVC_ERR_SEARCH_PATTERN, w.devicePattern, err.Error()),
			persistence.EC_ERROR_START_AGREEMENTLESS_SERVICE,
			"", "", "", "", "", []string{})
		glog.Errorf(logString(fmt.Sprintf("Unable to start agreement-less services, error searching for pattern %v in exchange, error: %v", w.devicePattern, err)))
		return
	}

	// There should only be 1 pattern in the response.
	if _, ok := patternDef[pat]; !ok {
		eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_ERROR,
			persistence.NewMessageMeta(EL_GOV_ERR_START_AGLESS_SVC_ERR_PATTERN_NOT_FOUND, pat),
			persistence.EC_ERROR_START_AGREEMENTLESS_SERVICE,
			"", "", "", "", "", []string{})
		glog.Errorf(logString(fmt.Sprintf("Unable to start agreement-less services, pattern %v not found in exchange", pat)))
		return
	}

	glog.V(3).Infof(logString(fmt.Sprintf("Starting agreement-less services")))

	// Loop through all the services and start the ones that are agreement-less.
	for _, service := range patternDef[pat].Services {
		if service.AgreementLess {

			// get a versions to string for eventlog.
			a_versions := []string{}
			for _, v := range service.ServiceVersions {
				a_versions = append(a_versions, v.Version)
			}
			versions := strings.Join(a_versions, ",")

			eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_INFO,
				persistence.NewMessageMeta(EL_GOV_START_AGLESS_SVC, service.ServiceOrg, service.ServiceURL),
				persistence.EC_START_AGREEMENTLESS_SERVICE,
				"", service.ServiceURL, service.ServiceOrg, versions, service.ServiceArch, []string{})

			// Find the microservice definition for this service.
			if msdefs, err := persistence.FindUnarchivedMicroserviceDefs(w.db, service.ServiceURL, service.ServiceOrg); err != nil {
				eventlog.LogDatabaseEvent(w.db, persistence.SEVERITY_ERROR,
					persistence.NewMessageMeta(EL_GOV_ERR_START_AGLESS_SVC, service.ServiceOrg, service.ServiceURL, err.Error()),
					persistence.EC_DATABASE_ERROR)
				glog.Errorf(logString(fmt.Sprintf("Unable to start agreement-less service %v/%v, error %v", service.ServiceOrg, service.ServiceURL, err)))
				return
			} else if msdefs == nil || len(msdefs) == 0 {
				eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_ERROR,
					persistence.NewMessageMeta(EL_GOV_ERR_START_AGLESS_SVC_ERR_SDEF_NOT_FOUND, service.ServiceOrg, service.ServiceURL),
					persistence.EC_ERROR_START_AGREEMENTLESS_SERVICE,
					"", service.ServiceURL, service.ServiceOrg, versions, service.ServiceArch, []string{})
				glog.Errorf(logString(fmt.Sprintf("Unable to start agreement-less service %v/%v, local service definition not found", service.ServiceOrg, service.ServiceURL)))
				return
			} else {

				// Create the service instance dependency path with the agreement-less service as the root.
				instancePath := []persistence.ServiceInstancePathElement{*persistence.NewServiceInstancePathElement(msdefs[0].SpecRef, msdefs[0].Org, msdefs[0].Version)}

				if err := w.startDependentService(instancePath, &msdefs[0], "", policy.BasicProtocol); err != nil {
					eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_ERROR,
						persistence.NewMessageMeta(EL_GOV_ERR_START_AGLESS_SVC, service.ServiceOrg, service.ServiceURL, err.Error()),
						persistence.EC_ERROR_START_AGREEMENTLESS_SERVICE,
						"", service.ServiceURL, service.ServiceOrg, versions, service.ServiceArch, []string{})

					glog.Errorf(logString(fmt.Sprintf("Unable to start agreement-less service %v/%v, error %v", service.ServiceOrg, service.ServiceURL, err)))
				} else {
					eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_INFO,
						persistence.NewMessageMeta(EL_GOV_COMPLETE_START_AGLESS_SVC, service.ServiceOrg, service.ServiceURL),
						persistence.EC_COMPLETE_AGREEMENTLESS_SERVICE_STARTUP,
						"", service.ServiceURL, service.ServiceOrg, versions, service.ServiceArch, []string{})
				}

			}
		}
	}

	glog.V(3).Infof(logString(fmt.Sprintf("Started agreement-less services")))

}

// Get the environmental variables for a service (this is about launching).
func (w *GovernanceWorker) GetServicePreference(url string, org string, tcPolicy *policy.Policy) (map[string]string, error) {

	envAdds := make(map[string]string)

	// get user input from the node. This does not inclue the UserInputAttributes.
	attrs, err := persistence.FindApplicableAttributes(w.db, url, org)
	if err != nil {
		return nil, fmt.Errorf("Unable to fetch service preferences for service %v/%v. Err: %v", org, url, err)
	}
	nodePol, err := persistence.FindNodePolicy(w.db)
	if err != nil {
		return nil, err
	}
	envAdds, err = persistence.AttributesToEnvvarMap(attrs, make(map[string]string), config.ENVVAR_PREFIX, w.Config.Edge.DefaultServiceRegistrationRAM, nodePol)
	if err != nil {
		return nil, fmt.Errorf("Failed to convert attrributes to env map for service %v/%v. Err: %v", org, url, err)
	}

	// add node user input
	userInput, err := persistence.FindNodeUserInput(w.db)
	if err != nil {
		return nil, fmt.Errorf("Failed get user input from local db. %v", err)
	}
	envAdds, err = policy.UpdateSettingsWithUserInputs(userInput, envAdds, url, org)
	if err != nil {
		return nil, fmt.Errorf("Error getting environmental variable settings from node user input for %v/%v: %v", org, url, err)
	}

	// Add settings from business policy or pattern that comes with the proposal.
	if tcPolicy != nil {
		envAdds, err = policy.UpdateSettingsWithUserInputs(tcPolicy.UserInput, envAdds, url, org)
		if err != nil {
			return nil, fmt.Errorf("Error getting environmental variable settings from policy for %v/%v: %v", org, url, err)
		}
	}

	return envAdds, nil
}

func recordProducerAgreementState(httpClient *http.Client, url string, deviceId string, token string, pattern string, agreementId string, pol *policy.Policy, state string) error {

	glog.V(5).Infof(logString(fmt.Sprintf("setting agreement %v state to %v", agreementId, state)))

	// Gather up the service and workload info about this agreement.
	as := new(exchange.PutAgreementState)
	services := make([]exchange.MSAgreementState, 0, 5)

	for _, apiSpec := range pol.APISpecs {
		services = append(services, exchange.MSAgreementState{
			Org: apiSpec.Org,
			URL: apiSpec.SpecRef,
		})
	}

	workload := exchange.WorkloadAgreement{}
	workload.Org = exchange.GetOrg(deviceId)
	workload.Pattern = pattern
	workload.URL = cutil.FormOrgSpecUrl(pol.Workloads[0].WorkloadURL, pol.Workloads[0].Org) // This is always 1 workload array element

	// Configure the input object based on the service model or on the older workload model.
	as.State = state
	as.Services = services
	as.AgreementService = workload

	// Call the exchange API to set the agreement state.
	var resp interface{}
	resp = new(exchange.PostDeviceResponse)
	targetURL := url + "orgs/" + exchange.GetOrg(deviceId) + "/nodes/" + exchange.GetId(deviceId) + "/agreements/" + agreementId
	for {
		if err, tpErr := exchange.InvokeExchange(httpClient, "PUT", targetURL, deviceId, token, &as, &resp); err != nil {
			glog.Errorf(logString(fmt.Sprintf(err.Error())))
			return err
		} else if tpErr != nil {
			glog.Warningf(logString(tpErr.Error()))
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(5).Infof(logString(fmt.Sprintf("set agreement %v to state %v", agreementId, state)))
			return nil
		}
	}

}

func (w *GovernanceWorker) deleteProducerAgreement(url string, deviceId string, token string, agreementId string) error {

	glog.V(5).Infof(logString(fmt.Sprintf("deleting agreement %v in exchange", agreementId)))

	httpClientFactory := w.GetHTTPFactory()
	retryCount := httpClientFactory.RetryCount
	retryInterval := httpClientFactory.GetRetryInterval()

	var resp interface{}
	resp = new(exchange.PostDeviceResponse)
	targetURL := url + "orgs/" + exchange.GetOrg(deviceId) + "/nodes/" + exchange.GetId(deviceId) + "/agreements/" + agreementId
	for {
		if err, tpErr := exchange.InvokeExchange(httpClientFactory.NewHTTPClient(nil), "DELETE", targetURL, deviceId, token, nil, &resp); err != nil && !strings.Contains(err.Error(), "status: 404") {
			glog.Errorf(logString(fmt.Sprintf(err.Error())))
			return err
		} else if tpErr != nil {
			glog.Warningf(logString(tpErr.Error()))
			if httpClientFactory.RetryCount == 0 {
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			} else if retryCount == 0 {
				return errors.New(fmt.Sprintf("exceeded %v retries trying to delete node for %v", httpClientFactory.RetryCount, tpErr))
			} else {
				retryCount--
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			}
		} else {
			glog.V(5).Infof(logString(fmt.Sprintf("deleted agreement %v from exchange", agreementId)))
			return nil
		}
	}
}

func (w *GovernanceWorker) deleteMessage(msg *exchange.DeviceMessage) error {
	var resp interface{}
	resp = new(exchange.PostDeviceResponse)
	targetURL := w.GetExchangeURL() + "orgs/" + exchange.GetOrg(w.GetExchangeId()) + "/nodes/" + exchange.GetId(w.GetExchangeId()) + "/msgs/" + strconv.Itoa(msg.MsgId)
	for {
		if err, tpErr := exchange.InvokeExchange(w.Config.Collaborators.HTTPClientFactory.NewHTTPClient(nil), "DELETE", targetURL, w.GetExchangeId(), w.GetExchangeToken(), nil, &resp); err != nil {
			glog.Errorf(logString(err.Error()))
			return err
		} else if tpErr != nil {
			glog.Warningf(logString(tpErr.Error()))
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
	targetURL := w.GetExchangeURL() + "orgs/" + exchange.GetOrg(w.GetExchangeId()) + "/nodes/" + exchange.GetId(w.GetExchangeId()) + "/msgs"
	for {
		if err, tpErr := exchange.InvokeExchange(w.Config.Collaborators.HTTPClientFactory.NewHTTPClient(nil), "GET", targetURL, w.GetExchangeId(), w.GetExchangeToken(), nil, &resp); err != nil {
			glog.Errorf(logString(err.Error()))
			return false, err
		} else if tpErr != nil {
			glog.Warningf(logString(tpErr.Error()))
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

// Check if the agreement uses any of the given services
func (w *GovernanceWorker) agreementRequiresService(ag persistence.EstablishedAgreement, svcSpecs persistence.ServiceSpecs) (bool, error) {
	if svcSpecs == nil || len(svcSpecs) == 0 {
		return false, nil
	}

	workload := ag.RunningWorkload
	if workload.URL == "" || workload.Org == "" {
		return false, nil
	}

	asl, _, _, err := exchange.GetHTTPServiceResolverHandler(w)(workload.URL, workload.Org, workload.Version, workload.Arch)
	if err != nil {
		return false, fmt.Errorf(logString(fmt.Sprintf("error searching for service details %v, error: %v", workload, err)))
	}

	for _, sp := range svcSpecs {
		if workload.URL == sp.Url && workload.Org == sp.Org {
			return true, nil
		}
		if asl != nil {
			for _, s := range *asl {
				if s.SpecRef == sp.Url && s.Org == sp.Org {
					return true, nil
				}

			}
		}
	}

	return false, nil
}
