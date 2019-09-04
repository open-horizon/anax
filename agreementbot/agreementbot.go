package agreementbot

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/abstractprotocol"
	"github.com/open-horizon/anax/agreementbot/persistence"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/version"
	"github.com/open-horizon/anax/worker"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// for identifying the subworkers used by this worker
const HEARTBEAT = "AgbotHeartBeat"
const DATABASE_HEARTBEAT = "AgbotDatabaseHeartBeat"
const GOVERN_AGREEMENTS = "AgBotGovernAgreements"
const GOVERN_ARCHIVED_AGREEMENTS = "AgBotGovernArchivedAgreements"
const GOVERN_BC_NEEDS = "AgBotGovernBlockchain"
const POLICY_WATCHER = "AgBotPolicyWatcher"
const GENERATE_POLICY = "AgBotPolicyGenerator"
const STALE_PARTITIONS = "AgbotStaleDatabasePartition"

// Agreement governance timing state. Used in the GovernAgreements subworker.
type DVState struct {
	dvSkip uint64
	nhSkip uint64
}

// must be safely-constructed!!
type AgreementBotWorker struct {
	worker.BaseWorker  // embedded field
	db                 persistence.AgbotDatabase
	httpClient         *http.Client // a shared HTTP client instance for this worker
	pm                 *policy.PolicyManager
	consumerPH         map[string]ConsumerProtocolHandler
	ready              bool
	PatternManager     *PatternManager
	BusinessPolManager *BusinessPolicyManager
	NHManager          *NodeHealthManager
	GovTiming          DVState
	lastExchVerCheck   int64
	shutdownStarted    bool
	lastAgMakingTime   uint64 // the start time for the last agreement making cycle, only used by non-pattern case
	MMSObjectPM        *MMSObjectPolicyManager
	mmsObjectPollTime  int64 // the last time the MMS was polled for changes
}

func NewAgreementBotWorker(name string, cfg *config.HorizonConfig, db persistence.AgbotDatabase) *AgreementBotWorker {

	ec := worker.NewExchangeContext(cfg.AgreementBot.ExchangeId, cfg.AgreementBot.ExchangeToken, cfg.AgreementBot.ExchangeURL, cfg.AgreementBot.CSSURL, cfg.Collaborators.HTTPClientFactory)

	baseWorker := worker.NewBaseWorker(name, cfg, ec)
	worker := &AgreementBotWorker{
		BaseWorker:        baseWorker,
		db:                db,
		httpClient:        cfg.Collaborators.HTTPClientFactory.NewHTTPClient(nil),
		consumerPH:        make(map[string]ConsumerProtocolHandler),
		ready:             false,
		PatternManager:    NewPatternManager(),
		NHManager:         NewNodeHealthManager(),
		GovTiming:         DVState{},
		lastExchVerCheck:  0,
		shutdownStarted:   false,
		lastAgMakingTime:  0,
		mmsObjectPollTime: 0,
	}

	glog.Info("Starting AgreementBot worker")
	worker.Start(worker, int(cfg.AgreementBot.NewContractIntervalS))
	return worker
}

func (w *AgreementBotWorker) ShutdownStarted() bool {
	return w.shutdownStarted
}

func (w *AgreementBotWorker) Messages() chan events.Message {
	return w.BaseWorker.Manager.Messages
}

func (w *AgreementBotWorker) NewEvent(incoming events.Message) {

	if w.Config.AgreementBot == (config.AGConfig{}) {
		return
	}

	switch incoming.(type) {
	case *events.AccountFundedMessage:
		msg, _ := incoming.(*events.AccountFundedMessage)
		switch msg.Event().Id {
		case events.ACCOUNT_FUNDED:
			cmd := NewAccountFundedCommand(msg)
			w.Commands <- cmd
		}

	case *events.BlockchainClientInitializedMessage:
		msg, _ := incoming.(*events.BlockchainClientInitializedMessage)
		switch msg.Event().Id {
		case events.BC_CLIENT_INITIALIZED:
			cmd := NewClientInitializedCommand(msg)
			w.Commands <- cmd
		}

	case *events.BlockchainClientStoppingMessage:
		msg, _ := incoming.(*events.BlockchainClientStoppingMessage)
		switch msg.Event().Id {
		case events.BC_CLIENT_STOPPING:
			cmd := NewClientStoppingCommand(msg)
			w.Commands <- cmd
		}

	case *events.EthBlockchainEventMessage:
		if w.ready {
			msg, _ := incoming.(*events.EthBlockchainEventMessage)
			switch msg.Event().Id {
			case events.BC_EVENT:
				agCmd := NewBlockchainEventCommand(*msg)
				w.Commands <- agCmd
			}
		}

	case *events.ABApiAgreementCancelationMessage:
		if w.ready {
			msg, _ := incoming.(*events.ABApiAgreementCancelationMessage)
			switch msg.Event().Id {
			case events.AGREEMENT_ENDED:
				agCmd := NewAgreementTimeoutCommand(msg.AgreementId, msg.AgreementProtocol, w.consumerPH[msg.AgreementProtocol].GetTerminationCode(TERM_REASON_USER_REQUESTED))
				w.Commands <- agCmd
			}
		}

	case *events.PolicyChangedMessage:
		if w.ready {
			msg, _ := incoming.(*events.PolicyChangedMessage)
			switch msg.Event().Id {
			case events.CHANGED_POLICY:
				pcCmd := NewPolicyChangedCommand(*msg)
				w.Commands <- pcCmd
			}
		}

	case *events.PolicyDeletedMessage:
		if w.ready {
			msg, _ := incoming.(*events.PolicyDeletedMessage)
			switch msg.Event().Id {
			case events.DELETED_POLICY:
				pdCmd := NewPolicyDeletedCommand(*msg)
				w.Commands <- pdCmd
			}
		}

	case *events.ABApiWorkloadUpgradeMessage:
		if w.ready {
			msg, _ := incoming.(*events.ABApiWorkloadUpgradeMessage)
			switch msg.Event().Id {
			case events.WORKLOAD_UPGRADE:
				wuCmd := NewWorkloadUpgradeCommand(*msg)
				w.Commands <- wuCmd
			}
		}

	case *events.NodeShutdownCompleteMessage:
		msg, _ := incoming.(*events.NodeShutdownCompleteMessage)
		switch msg.Event().Id {
		case events.UNCONFIGURE_COMPLETE:
			w.Commands <- worker.NewBeginShutdownCommand()
			w.Commands <- worker.NewTerminateCommand("shutdown")
		case events.AGBOT_QUIESCE_COMPLETE:
			w.Commands <- worker.NewTerminateCommand("shutdown")
		}

	case *events.NodeShutdownMessage:
		msg, _ := incoming.(*events.NodeShutdownMessage)
		switch msg.Event().Id {
		case events.START_AGBOT_QUIESCE:
			w.Commands <- NewAgbotShutdownCommand(msg)
		}

	case *events.CacheServicePolicyMessage:
		msg, _ := incoming.(*events.CacheServicePolicyMessage)

		switch msg.Event().Id {
		case events.CACHE_SERVICE_POLICY:
			w.Commands <- NewCacheServicePolicyCommand(msg)
		}

	case *events.ServicePolicyChangedMessage:
		msg, _ := incoming.(*events.ServicePolicyChangedMessage)
		switch msg.Event().Id {
		case events.SERVICE_POLICY_CHANGED:
			w.Commands <- NewServicePolicyChangedCommand(msg)
		}

	case *events.ServicePolicyDeletedMessage:
		msg, _ := incoming.(*events.ServicePolicyDeletedMessage)
		switch msg.Event().Id {
		case events.SERVICE_POLICY_DELETED:
			w.Commands <- NewServicePolicyDeletedCommand(msg)
		}

	case *events.MMSObjectPolicyMessage:
		msg, _ := incoming.(*events.MMSObjectPolicyMessage)
		w.Commands <- NewMMSObjectPolicyEventCommand(msg)

	default: //nothing

	}

	return
}

// This function is used by Initialize to send the Agbot terminate message in the cases where Initialize fails such that the
// entire agbot process should also terminate.
func (w *AgreementBotWorker) fail() bool {
	w.Messages() <- events.NewNodeShutdownCompleteMessage(events.AGBOT_QUIESCE_COMPLETE, "")
	return false
}

func (w *AgreementBotWorker) Initialize() bool {

	glog.Info("AgreementBot worker initializing")

	// If there is no Agbot config, we will terminate. This is a normal condition when running on a node.
	if w.Config.AgreementBot == (config.AGConfig{}) {
		glog.Warningf("AgreementBotWorker terminating, no AgreementBot config.")
		return false
	} else if w.db == nil {
		glog.Warningf("AgreementBotWorker terminating, no AgreementBot database configured.")
		return false
	}

	// Log an error if the current exchange version does not meet the requirement.
	if err := version.VerifyExchangeVersion(w.Config.Collaborators.HTTPClientFactory, w.GetExchangeURL(), w.GetExchangeId(), w.GetExchangeToken(), false); err != nil {
		glog.Errorf(AWlogString(fmt.Sprintf("Error verifiying exchange version. error: %v", err)))
		return w.fail()
	}

	// Make sure the policy directory is in place so that we have a place to put the generated policy files.
	if err := os.MkdirAll(w.BaseWorker.Manager.Config.AgreementBot.PolicyPath, 0644); err != nil {
		glog.Errorf("AgreementBotWorker cannot create agreement bot policy file path %v, terminating.", w.BaseWorker.Manager.Config.AgreementBot.PolicyPath)
		return w.fail()
	}

	// To start clean, remove all left over pattern based policy files from the last time the agbot was started.
	// This is only called once at the agbot start up time.
	if err := policy.DeleteAllPolicyFiles(w.BaseWorker.Manager.Config.AgreementBot.PolicyPath, true); err != nil {
		glog.Errorf("AgreementBotWorker cannot clean up pattern based policy files under %v. %v", w.BaseWorker.Manager.Config.AgreementBot.PolicyPath, err)
		return w.fail()
	}

	// Start the go thread that heartbeats to the exchange
	w.DispatchSubworker(HEARTBEAT, w.heartBeat, w.BaseWorker.Manager.Config.AgreementBot.ExchangeHeartbeat, false)

	// Give the policy manager a chance to read in all the policies. The agbot worker will not proceed past this point
	// until it has some policies to work with.
	w.BusinessPolManager = NewBusinessPolicyManager(w.Messages())
	w.MMSObjectPM = NewMMSObjectPolicyManager(w.BaseWorker.Manager.Config)
	for {

		// Query the exchange for patterns that this agbot is supposed to serve and generate a policy for each one. If an error
		// occurs, it will be ignored. The Agbot should not proceed out of initialization until it has at least 1 policy/pattern
		// that it can serve.

		// generate policy files for patterns
		if err := w.internalGeneratePolicyFromPatterns(); err != nil {
			glog.Errorf(AWlogString(fmt.Sprintf("unable to process patterns, error %v", err)))
		}

		// let policy manager read it
		if filePolManager, err := policy.Initialize(w.BaseWorker.Manager.Config.AgreementBot.PolicyPath, w.Config.ArchSynonyms, w.serviceResolver, true, false); err != nil {
			glog.Errorf("AgreementBotWorker unable to initialize policy manager, error: %v", err)
		} else {
			// creating business policy cache and update the policy manager
			w.pm = filePolManager
			if err := w.internalGeneratePolicyFromBusinessPols(); err != nil {
				glog.Errorf(AWlogString(fmt.Sprintf("unable to process business policy, error %v", err)))
			} else if filePolManager.NumberPolicies() != 0 {
				// w.pm = filePolManager
				break
			}
		}
		glog.V(3).Infof("AgreementBotWorker waiting for policies to appear")
		time.Sleep(time.Duration(w.BaseWorker.Manager.Config.AgreementBot.CheckUpdatedPolicyS) * time.Second)

	}

	glog.Info("AgreementBot worker started")

	// Make sure that our public key is registered in the exchange so that other parties
	// can send us messages.
	if err := w.registerPublicKey(); err != nil {
		glog.Errorf("AgreementBotWorker unable to register public key, error: %v", err)
		return w.fail()
	}

	// For each agreement protocol in the current list of configured policies, startup a processor
	// to initiate the protocol.
	for protocolName, _ := range w.pm.GetAllAgreementProtocols() {
		if policy.SupportedAgreementProtocol(protocolName) {
			cph := CreateConsumerPH(protocolName, w.BaseWorker.Manager.Config, w.db, w.pm, w.BaseWorker.Manager.Messages, w.MMSObjectPM)
			cph.Initialize()
			w.consumerPH[protocolName] = cph
		} else {
			glog.Errorf("AgreementBotWorker ignoring agreement protocol %v, not supported.", protocolName)
		}
	}

	// Sync up between what's in our database versus what's in the exchange, and make sure that the policy manager's
	// agreement counts are correct. The governance routine will cancel any agreements whose state might have changed
	// while the agbot was down. We will also check to make sure that policies havent changed. If they have, then
	// we will cancel agreements and allow them to re-negotiate.
	if err := w.syncOnInit(); err != nil {
		glog.Errorf("AgreementBotWorker Terminating, unable to sync up, error: %v", err)
		return w.fail()
	}

	// The agbot worker is now ready to handle incoming messages
	w.ready = true

	// Start the go thread that heartbeats to the database and checks for stale partitions.
	w.DispatchSubworker(DATABASE_HEARTBEAT, w.databaseHeartBeat, int(w.BaseWorker.Manager.Config.GetPartitionStale()/3), false)
	w.DispatchSubworker(STALE_PARTITIONS, w.stalePartitions, int(w.BaseWorker.Manager.Config.GetPartitionStale()), false)

	// Start the governance routines using the subworker APIs.
	w.DispatchSubworker(GOVERN_AGREEMENTS, w.GovernAgreements, int(w.BaseWorker.Manager.Config.AgreementBot.ProcessGovernanceIntervalS), false)
	w.DispatchSubworker(GOVERN_ARCHIVED_AGREEMENTS, w.GovernArchivedAgreements, 1800, false)
	w.DispatchSubworker(GOVERN_BC_NEEDS, w.GovernBlockchainNeeds, 60, false)
	if w.Config.AgreementBot.CheckUpdatedPolicyS != 0 {
		// Use custom subworker APIs for the policy watcher because it is stateful and already does its own time management.
		ch := w.AddSubworker(POLICY_WATCHER)
		go w.policyWatcher(POLICY_WATCHER, ch)

		w.DispatchSubworker(GENERATE_POLICY, w.GeneratePolicies, int(w.Config.AgreementBot.CheckUpdatedPolicyS), false)
	}

	return true
}

func (w *AgreementBotWorker) CommandHandler(command worker.Command) bool {

	// Enter the command processing loop. Initialization is complete so wait for commands to
	// perform. Commands are created as the result of events that are triggered elsewhere
	// in the system. This function also wakes up periodically and looks for messages on
	// its exchange message queue.

	switch command.(type) {
	case *BlockchainEventCommand:
		cmd, _ := command.(*BlockchainEventCommand)
		// Put command on each protocol worker's command queue
		for _, ch := range w.consumerPH {
			if ch.AcceptCommand(cmd) {
				ch.HandleBlockchainEvent(cmd)
			}
		}

	case *PolicyChangedCommand:
		cmd := command.(*PolicyChangedCommand)

		if pol, err := policy.DemarshalPolicy(cmd.Msg.PolicyString()); err != nil {
			glog.Errorf(fmt.Sprintf("AgreementBotWorker error demarshalling change event policy %v, error: %v", cmd.Msg.PolicyString(), err))
		} else {
			// We know that all agreement protocols in the policy are supported by this runtime. If not, then this
			// event would not have occurred.

			glog.V(5).Infof("AgreementBotWorker about to update policy in PM.")
			// Update the policy in the policy manager.
			w.pm.UpdatePolicy(cmd.Msg.Org(), pol)
			glog.V(5).Infof("AgreementBotWorker updated policy in PM.")

			for _, agp := range pol.AgreementProtocols {
				// Update the protocol handler map and make sure there are workers available if the policy has a new protocol in it.
				if _, ok := w.consumerPH[agp.Name]; !ok {
					glog.V(3).Infof("AgreementBotWorker creating worker pool for new agreement protocol %v", agp.Name)
					cph := CreateConsumerPH(agp.Name, w.BaseWorker.Manager.Config, w.db, w.pm, w.BaseWorker.Manager.Messages, w.MMSObjectPM)
					cph.Initialize()
					w.consumerPH[agp.Name] = cph
				}
			}

			// Send the policy change command to all protocol handlers just in case an agreement protocol was
			// deleted from the new policy file.
			for agp, _ := range w.consumerPH {
				// Queue the command to the relevant protocol handler for further processing.
				if w.consumerPH[agp].AcceptCommand(cmd) {
					w.consumerPH[agp].HandlePolicyChanged(cmd, w.consumerPH[agp])
				}
			}

		}

	case *PolicyDeletedCommand:
		cmd := command.(*PolicyDeletedCommand)

		if pol, err := policy.DemarshalPolicy(cmd.Msg.PolicyString()); err != nil {
			glog.Errorf(fmt.Sprintf("AgreementBotWorker error demarshalling change event policy %v, error: %v", cmd.Msg.PolicyString(), err))
		} else {

			glog.V(5).Infof("AgreementBotWorker about to delete policy from PM.")
			// Update the policy in the policy manager.
			w.pm.DeletePolicy(cmd.Msg.Org(), pol)
			glog.V(5).Infof("AgreementBotWorker deleted policy from PM.")

			// Queue the command to the correct protocol worker pool(s) for further processing. The deleted policy
			// might not contain a supported protocol, so we need to check that first.
			for _, agp := range pol.AgreementProtocols {
				if _, ok := w.consumerPH[agp.Name]; ok {
					if w.consumerPH[agp.Name].AcceptCommand(cmd) {
						w.consumerPH[agp.Name].HandlePolicyDeleted(cmd, w.consumerPH[agp.Name])
					}
				} else {
					glog.Infof("AgreementBotWorker ignoring policy deleted command for unsupported agreement protocol %v", agp.Name)
				}
			}
		}

	case *CacheServicePolicyCommand:
		cmd, _ := command.(*CacheServicePolicyCommand)
		w.BusinessPolManager.AddMarshaledServicePolicy(cmd.Msg.BusinessPolOrg, cmd.Msg.BusinessPolName, cmd.Msg.ServiceId, cmd.Msg.ServicePolicy)

	case *ServicePolicyChangedCommand:
		cmd, _ := command.(*ServicePolicyChangedCommand)
		// Send the service policy changed command to all protocol handlers
		for agp, _ := range w.consumerPH {
			// Queue the command to the relevant protocol handler for further processing.
			if w.consumerPH[agp].AcceptCommand(cmd) {
				w.consumerPH[agp].HandleServicePolicyChanged(cmd, w.consumerPH[agp])
			}
		}

	case *ServicePolicyDeletedCommand:
		cmd, _ := command.(*ServicePolicyDeletedCommand)
		// Send the service policy deleted command to all protocol handlers
		for agp, _ := range w.consumerPH {
			// Queue the command to the relevant protocol handler for further processing.
			if w.consumerPH[agp].AcceptCommand(cmd) {
				w.consumerPH[agp].HandleServicePolicyDeleted(cmd, w.consumerPH[agp])
			}
		}

	case *AgreementTimeoutCommand:
		cmd, _ := command.(*AgreementTimeoutCommand)
		if _, ok := w.consumerPH[cmd.Protocol]; !ok {
			glog.Errorf(fmt.Sprintf("AgreementBotWorker unable to process agreement timeout command %v due to unknown agreement protocol", cmd))
		} else {
			if w.consumerPH[cmd.Protocol].AcceptCommand(cmd) {
				w.consumerPH[cmd.Protocol].HandleAgreementTimeout(cmd, w.consumerPH[cmd.Protocol])
			}
		}

	case *WorkloadUpgradeCommand:
		cmd, _ := command.(*WorkloadUpgradeCommand)
		// The workload upgrade request might not involve a specific agreement, so we can't know precisely which agreement
		// protocol might be relevant. Therefore we will send this upgrade to all protocol worker pools.
		for _, ch := range w.consumerPH {
			if ch.AcceptCommand(cmd) {
				ch.HandleWorkloadUpgrade(cmd, ch)
			}
		}

	case *AccountFundedCommand:
		cmd, _ := command.(*AccountFundedCommand)
		for _, cph := range w.consumerPH {
			cph.SetBlockchainWritable(&cmd.Msg)
		}

	case *ClientInitializedCommand:
		cmd, _ := command.(*ClientInitializedCommand)
		for _, cph := range w.consumerPH {
			cph.SetBlockchainClientAvailable(&cmd.Msg)
		}

	case *ClientStoppingCommand:
		cmd, _ := command.(*ClientStoppingCommand)
		for _, cph := range w.consumerPH {
			cph.SetBlockchainClientNotAvailable(&cmd.Msg)
		}

	case *MMSObjectPolicyEventCommand:
		cmd, _ := command.(*MMSObjectPolicyEventCommand)
		for _, ch := range w.consumerPH {
			if ch.AcceptCommand(cmd) {
				ch.HandleMMSObjectPolicy(cmd, ch)
			}
		}

	case *AgbotShutdownCommand:
		w.shutdownStarted = true
		glog.V(4).Infof("AgreementBotWorker received start shutdown command")

	default:
		return false
	}

	return true

}

func (w *AgreementBotWorker) NoWorkHandler() {

	glog.V(4).Infof("AgreementBotWorker queueing deferred commands")
	for _, cph := range w.consumerPH {
		cph.HandleDeferredCommands()
	}
	glog.V(4).Infof("AgreementBotWorker done queueing deferred commands")

	glog.V(5).Infof(fmt.Sprintf("AgreementBotWorker retrieving messages from the exchange"))

	if msgs, err := w.getMessages(); err != nil {
		glog.Errorf(fmt.Sprintf("AgreementBotWorker unable to retrieve exchange messages, error: %v", err))
	} else {
		// Loop through all the returned messages and process them
		for _, msg := range msgs {

			glog.V(3).Infof(fmt.Sprintf("AgreementBotWorker reading message %v from the exchange", msg.MsgId))
			// First get my own keys
			_, myPrivKey, _ := exchange.GetKeys(w.Config.AgreementBot.MessageKeyPath)

			// Deconstruct and decrypt the message. Then process it.
			if protocolMessage, receivedPubKey, err := exchange.DeconstructExchangeMessage(msg.Message, myPrivKey); err != nil {
				glog.Errorf(fmt.Sprintf("AgreementBotWorker unable to deconstruct exchange message %v, error %v", msg, err))
			} else if serializedPubKey, err := exchange.MarshalPublicKey(receivedPubKey); err != nil {
				glog.Errorf(fmt.Sprintf("AgreementBotWorker unable to marshal the key from the encrypted message %v, error %v", receivedPubKey, err))
			} else if bytes.Compare(msg.DevicePubKey, serializedPubKey) != 0 {
				glog.Errorf(fmt.Sprintf("AgreementBotWorker sender public key from exchange %x is not the same as the sender public key in the encrypted message %x", msg.DevicePubKey, serializedPubKey))
			} else if msgProtocol, err := abstractprotocol.ExtractProtocol(string(protocolMessage)); err != nil {
				glog.Errorf(fmt.Sprintf("AgreementBotWorker unable to extract agreement protocol name from message %v", protocolMessage))
			} else if _, ok := w.consumerPH[msgProtocol]; !ok {
				glog.Infof(fmt.Sprintf("AgreementBotWorker unable to direct exchange message %v to a protocol handler, deleting it.", protocolMessage))
				DeleteMessage(msg.MsgId, w.GetExchangeId(), w.GetExchangeToken(), w.GetExchangeURL(), w.httpClient)
			} else {
				cmd := NewNewProtocolMessageCommand(protocolMessage, msg.MsgId, msg.DeviceId, msg.DevicePubKey)
				if !w.consumerPH[msgProtocol].AcceptCommand(cmd) {
					glog.Infof(fmt.Sprintf("AgreementBotWorker protocol handler for %v not accepting exchange messages, deleting msg.", msgProtocol))
					DeleteMessage(msg.MsgId, w.GetExchangeId(), w.GetExchangeToken(), w.GetExchangeURL(), w.httpClient)
				} else if err := w.consumerPH[msgProtocol].DispatchProtocolMessage(cmd, w.consumerPH[msgProtocol]); err != nil {
					DeleteMessage(msg.MsgId, w.GetExchangeId(), w.GetExchangeToken(), w.GetExchangeURL(), w.httpClient)
				}
			}
		}
	}
	glog.V(5).Infof(fmt.Sprintf("AgreementBotWorker done processing messages"))

	// If shutdown has not started then keep looking for nodes to make agreements with.
	// If shutdown has started then we will stop making new agreements. Instead we will look for agreements that have not yet completed
	// the agreement protocol process. If there are any, then we will hold the quiesce from completing.
	if !w.ShutdownStarted() {
		glog.V(4).Infof("AgreementBotWorker Polling Exchange.")
		w.findAndMakeAgreements()
		glog.V(4).Infof("AgreementBotWorker Done Polling Exchange.")

	} else {
		// Find all agreements that are not yet finalized. This filter will return only agreements that are still in an agreement protocol
		// pending state.
		glog.V(4).Infof("AgreementBotWorker Looking for pending agreements before shutting down.")

		agreementPendingFilter := func() persistence.AFilter {
			return func(a persistence.Agreement) bool { return a.AgreementFinalizedTime == 0 && a.AgreementTimedout == 0 }
		}

		// Look at all agreements across all protocols,
		foundPending := false
		for _, agp := range policy.AllAgreementProtocols() {

			// Find all agreements that are in progress, agreements that are not archived and dont have either a finalized time or a timeeout time.
			if agreements, err := w.db.FindAgreements([]persistence.AFilter{agreementPendingFilter(), persistence.UnarchivedAFilter()}, agp); err != nil {
				glog.Errorf("AgreementBotWorker unable to read agreements from database, error: %v", err)
				w.Messages() <- events.NewNodeShutdownCompleteMessage(events.AGBOT_QUIESCE_COMPLETE, err.Error())
			} else if len(agreements) != 0 {
				foundPending = true
				break
			}

		}

		// If no pending agreements were found, then we can begin the shutdown.
		if !foundPending {

			glog.V(5).Infof("AgreementBotWorker shutdown beginning")

			w.SetWorkerShuttingDown()

			// Shutdown the protocol specific agreement workers for each supported protocol.
			for _, cph := range w.consumerPH {
				cph.HandleStopProtocol(cph)
			}

			// Shutdown the subworkers.
			w.TerminateSubworkers()

			// Shutdown the database partition.
			w.db.QuiescePartition()

			w.Messages() <- events.NewNodeShutdownCompleteMessage(events.AGBOT_QUIESCE_COMPLETE, "")

		}
	}

}

// Go through all the patterns and business polices and make agreements.
func (w *AgreementBotWorker) findAndMakeAgreements() {
	// current timestamp to be saved as the last agreement making cycle start time later.
	currentAgMakingStartTime := uint64(time.Now().Unix())

	// get a list of all the pattern orgs this agbot is serving
	allOrgs := w.pm.GetAllPolicyOrgs()
	for _, org := range allOrgs {
		// Get a copy of all policies in the policy manager that pulls from the policy files so that we can safely iterate the list
		// This is the pattern case
		patternPolicies := w.pm.GetAllAvailablePolicies(org)
		for _, consumerPolicy := range patternPolicies {
			if consumerPolicy.PatternId != "" {
				w.searchNodesAndMakeAgreements(&consumerPolicy, org, "", 0)
			} else if pBE := w.BusinessPolManager.GetBusinessPolicyEntry(org, &consumerPolicy); pBE != nil {
				_, polName := cutil.SplitOrgSpecUrl(consumerPolicy.Header.Name)
				w.searchNodesAndMakeAgreements(&consumerPolicy, org, polName, pBE.Updated)
			}
		}
	}

	// the current agreement making cycle is done, save the timestamp.
	w.lastAgMakingTime = currentAgMakingStartTime
}

// Search the exchange and make agreements with any device that is eligible based on the policies we have and
// agreement protocols that we support.
func (w *AgreementBotWorker) searchNodesAndMakeAgreements(consumerPolicy *policy.Policy, org string, polName string, polLastUpdateTime uint64) {

	if devices, err := w.searchExchange(consumerPolicy, org, polName, polLastUpdateTime); err != nil {
		glog.Errorf("AgreementBotWorker received error searching for %v, error: %v", consumerPolicy, err)
	} else {

		for _, dev := range *devices {

			glog.V(3).Infof("AgreementBotWorker picked up %v for policy %v.", dev.ShortString(), consumerPolicy.Header.Name)
			glog.V(5).Infof("AgreementBotWorker picked up %v", dev)

			// Check for agreements already in progress with this device
			if found, err := w.alreadyMakingAgreementWith(&dev, consumerPolicy); err != nil {
				glog.Errorf("AgreementBotWorker received error trying to find pending agreements: %v", err)
				continue
			} else if found {
				glog.V(5).Infof("AgreementBotWorker skipping device id %v, agreement attempt already in progress with %v", dev.Id, consumerPolicy.Header.Name)
				continue
			}

			// If the device is not ready to make agreements yet, then skip it.
			if len(dev.PublicKey) == 0 || string(dev.PublicKey) == "" {
				glog.V(5).Infof("AgreementBotWorker skipping device id %v, node is not ready to exchange messages", dev.Id)
				continue
			}

			producerPolicy := policy.Policy_Factory(consumerPolicy.Header.Name)

			// get the cached service policies from the business policy manager. The returned value
			// is a map keyed by the service id.
			// There could be many service versions defined in a businees policy.
			// The policy manager only caches the ones that are used by an old agreement for this business policy.
			// The cached ones may not be what the new agreement will use. If the new agreement chooses a
			// new service version, then the new service policy will be put into the cache.
			svcPolicies := make(map[string]externalpolicy.ExternalPolicy, 0)
			if consumerPolicy.PatternId == "" {
				svcPolicies = w.BusinessPolManager.GetServicePoliciesForPolicy(org, polName)
			}

			// Select a worker pool based on the agreement protocol that will be used. This is decided by the
			// consumer policy.
			protocol := policy.Select_Protocol(producerPolicy, consumerPolicy)
			cmd := NewMakeAgreementCommand(*producerPolicy, *consumerPolicy, org, polName, dev, svcPolicies)

			bcType, bcName, bcOrg := producerPolicy.RequiresKnownBC(protocol)

			if _, ok := w.consumerPH[protocol]; !ok {
				glog.Errorf("AgreementBotWorker unable to find protocol handler for %v.", protocol)
			} else if bcType != "" && !w.consumerPH[protocol].IsBlockchainWritable(bcType, bcName, bcOrg) {
				// Get that blockchain running if it isn't up.
				glog.V(5).Infof("AgreementBotWorker skipping device id %v, requires blockchain %v %v %v that isnt ready yet.", dev.Id, bcType, bcName, bcOrg)
				w.BaseWorker.Manager.Messages <- events.NewNewBCContainerMessage(events.NEW_BC_CLIENT, bcType, bcName, bcOrg, w.GetExchangeURL(), w.GetExchangeId(), w.GetExchangeToken())
				continue
			} else if !w.consumerPH[protocol].AcceptCommand(cmd) {
				glog.Errorf("AgreementBotWorker protocol handler for %v not accepting new agreement commands.", protocol)
			} else {
				w.consumerPH[protocol].HandleMakeAgreement(cmd, w.consumerPH[protocol])
				glog.V(5).Infof("AgreementBotWorker queued agreement attempt for policy %v and protocol %v", consumerPolicy.Header.Name, protocol)
			}
		}
	}
}

// Check all agreement protocol buckets to see if there are any agreements with this device.
func (w *AgreementBotWorker) alreadyMakingAgreementWith(dev *exchange.SearchResultDevice, consumerPolicy *policy.Policy) (bool, error) {

	// Check to see if we're already doing something with this device
	pendingAgreementFilter := func() persistence.AFilter {
		return func(a persistence.Agreement) bool {
			return a.DeviceId == dev.Id && a.PolicyName == consumerPolicy.Header.Name && a.AgreementTimedout == 0
		}
	}

	// Search all agreement protocol buckets
	for _, agp := range policy.AllAgreementProtocols() {
		// Find all agreements that are in progress. They might be waiting for a reply or not yet finalized.
		// TODO: To support more than 1 agreement (maxagreements > 1) with this device for this policy, we need to adjust this logic.
		if agreements, err := w.db.FindAgreements([]persistence.AFilter{persistence.UnarchivedAFilter(), pendingAgreementFilter()}, agp); err != nil {
			glog.Errorf("AgreementBotWorker received error trying to find pending agreements for protocol %v: %v", agp, err)
		} else if len(agreements) != 0 {

			ag := agreements[0]
			if ag.AgreementFinalizedTime != 0 {
				glog.V(5).Infof("AgreementBotWorker sending agreement verify for %v", ag.CurrentAgreementId)
				w.consumerPH[ag.AgreementProtocol].VerifyAgreement(&ag, w.consumerPH[ag.AgreementProtocol])
			}

			return true, nil
		}
	}
	return false, nil

}

func (w *AgreementBotWorker) policyWatcher(name string, quit chan bool) {

	worker.GetWorkerStatusManager().SetSubworkerStatus(w.GetName(), name, worker.STATUS_STARTED)

	// create a place for the policy watcher to save state between iterations.
	contents := w.pm.WatcherContent

	for {
		glog.V(5).Infof(fmt.Sprintf("AgreementBotWorker checking for new or updated policy files"))
		select {
		case <-quit:
			w.Commands <- worker.NewSubWorkerTerminationCommand(name)
			glog.V(3).Infof(fmt.Sprintf("AgreementBotWorker %v exiting the subworker", name))
			return

		case <-time.After(time.Duration(w.Config.AgreementBot.CheckUpdatedPolicyS) * time.Second):
			contents, _ = policy.PolicyFileChangeWatcher(w.Config.AgreementBot.PolicyPath, contents, w.Config.ArchSynonyms, w.changedPolicy, w.deletedPolicy, w.errorPolicy, w.serviceResolver, 0)
		}
	}

}

// Functions called by the policy watcher
func (w *AgreementBotWorker) changedPolicy(org string, fileName string, pol *policy.Policy) {
	glog.V(3).Infof(fmt.Sprintf("AgreementBotWorker detected changed policy file %v containing %v", fileName, pol))
	if policyString, err := policy.MarshalPolicy(pol); err != nil {
		glog.Errorf(fmt.Sprintf("AgreementBotWorker error trying to marshal policy %v error: %v", pol, err))
	} else {
		w.Messages() <- events.NewPolicyChangedMessage(events.CHANGED_POLICY, fileName, pol.Header.Name, org, policyString)
	}
}

func (w *AgreementBotWorker) deletedPolicy(org string, fileName string, pol *policy.Policy) {
	glog.V(3).Infof(fmt.Sprintf("AgreementBotWorker detected deleted policy file %v containing %v", fileName, pol))
	if policyString, err := policy.MarshalPolicy(pol); err != nil {
		glog.Errorf(fmt.Sprintf("AgreementBotWorker error trying to marshal policy %v error: %v", pol, err))
	} else {
		w.Messages() <- events.NewPolicyDeletedMessage(events.DELETED_POLICY, fileName, pol.Header.Name, org, policyString)
	}
}

func (w *AgreementBotWorker) errorPolicy(org string, fileName string, err error) {
	glog.Errorf(fmt.Sprintf("AgreementBotWorker tried to read policy file %v/%v, encountered error: %v", org, fileName, err))
}

func (w *AgreementBotWorker) getMessages() ([]exchange.AgbotMessage, error) {
	var resp interface{}
	resp = new(exchange.GetAgbotMessageResponse)
	targetURL := w.GetExchangeURL() + "orgs/" + exchange.GetOrg(w.GetExchangeId()) + "/agbots/" + exchange.GetId(w.GetExchangeId()) + "/msgs"
	for {
		if err, tpErr := exchange.InvokeExchange(w.httpClient, "GET", targetURL, w.GetExchangeId(), w.GetExchangeToken(), nil, &resp); err != nil {
			glog.Errorf(err.Error())
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(3).Infof(fmt.Sprintf("AgreementBotWorker retrieved %v messages", len(resp.(*exchange.GetAgbotMessageResponse).Messages)))
			msgs := resp.(*exchange.GetAgbotMessageResponse).Messages
			return msgs, nil
		}
	}
}

// This function runs through the agbot policy and builds a list of properties and values that
// it wants to search on.
func RetrieveAllProperties(version string, arch string, pol *policy.Policy) (*externalpolicy.PropertyList, error) {
	pl := new(externalpolicy.PropertyList)

	for _, p := range pol.Properties {
		*pl = append(*pl, p)
	}

	if version != "" {
		*pl = append(*pl, externalpolicy.Property{Name: "version", Value: version})
	}
	*pl = append(*pl, externalpolicy.Property{Name: "arch", Value: arch})

	if len(pol.AgreementProtocols) != 0 {
		*pl = append(*pl, externalpolicy.Property{Name: "agreementProtocols", Value: pol.AgreementProtocols.As_String_Array()})
	}

	return pl, nil
}

func DeleteConsumerAgreement(httpClient *http.Client, url string, agbotId string, token string, agreementId string) error {

	glog.V(5).Infof(AWlogString(fmt.Sprintf("deleting agreement %v in exchange", agreementId)))

	var resp interface{}
	resp = new(exchange.PostDeviceResponse)
	targetURL := url + "orgs/" + exchange.GetOrg(agbotId) + "/agbots/" + exchange.GetId(agbotId) + "/agreements/" + agreementId
	for {
		if err, tpErr := exchange.InvokeExchange(httpClient, "DELETE", targetURL, agbotId, token, nil, &resp); err != nil && !strings.Contains(err.Error(), "not found") {
			glog.Errorf(AWlogString(fmt.Sprintf(err.Error())))
			return err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(5).Infof(AWlogString(fmt.Sprintf("deleted agreement %v from exchange", agreementId)))
			return nil
		}
	}

}

func DeleteMessage(msgId int, agbotId, agbotToken, exchangeURL string, httpClient *http.Client) error {
	var resp interface{}
	resp = new(exchange.PostDeviceResponse)
	targetURL := exchangeURL + "orgs/" + exchange.GetOrg(agbotId) + "/agbots/" + exchange.GetId(agbotId) + "/msgs/" + strconv.Itoa(msgId)
	for {
		if err, tpErr := exchange.InvokeExchange(httpClient, "DELETE", targetURL, agbotId, agbotToken, nil, &resp); err != nil {
			glog.Errorf(err.Error())
			return err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(3).Infof("Deleted exchange message %v", msgId)
			return nil
		}
	}
}

// Search the exchange for devices to make agreements with. The system should be operating such that devices are
// not returned from the exchange (for any given set of search criteria) once an agreement which includes those
// criteria has been reached. This prevents the agbot from continually sending proposals to devices that are
// already in an agreement.
//
// There are 2 ways to search the exchange; (a) by pattern and service or workload URL, or (b) by business policy.
// If the agbot is working with a policy file that was generated from a pattern, then it will do searches
// by pattern. If the agbot is working with a business policy, then it will do searches by the business policy.
func (w *AgreementBotWorker) searchExchange(pol *policy.Policy, polOrg string, polName string, polLastUpdateTime uint64) (*[]exchange.SearchResultDevice, error) {

	// If it is a pattern based policy, search by workload URL and pattern.
	if pol.PatternId != "" {
		// Get a list of node orgs that the agbot is serving for this pattern.
		nodeOrgs := w.PatternManager.GetServedNodeOrgs(polOrg, exchange.GetId(pol.PatternId))
		if len(nodeOrgs) == 0 {
			glog.V(3).Infof("Policy file for pattern %v exists but currently the agbot is not serving this policy for any organizations.", pol.PatternId)
			empty := make([]exchange.SearchResultDevice, 0, 0)
			return &empty, nil
		}

		// Setup the search request body
		ser := exchange.CreateSearchPatternRequest()
		ser.SecondsStale = w.Config.AgreementBot.ActiveDeviceTimeoutS
		ser.NodeOrgIds = nodeOrgs
		ser.ServiceURL = cutil.FormOrgSpecUrl(pol.Workloads[0].WorkloadURL, pol.Workloads[0].Org)

		// Invoke the exchange
		var resp interface{}
		resp = new(exchange.SearchExchangePatternResponse)
		targetURL := w.GetExchangeURL() + "orgs/" + polOrg + "/patterns/" + exchange.GetId(pol.PatternId) + "/search"
		for {
			if err, tpErr := exchange.InvokeExchange(w.httpClient, "POST", targetURL, w.GetExchangeId(), w.GetExchangeToken(), ser, &resp); err != nil {
				if !strings.Contains(err.Error(), "status: 404") {
					return nil, err
				} else {
					empty := make([]exchange.SearchResultDevice, 0, 0)
					return &empty, nil
				}
			} else if tpErr != nil {
				glog.Warningf(tpErr.Error())
				time.Sleep(10 * time.Second)
				continue
			} else {
				glog.V(3).Infof("AgreementBotWorker found %v devices in exchange.", len(resp.(*exchange.SearchExchangePatternResponse).Devices))
				dev := resp.(*exchange.SearchExchangePatternResponse).Devices
				return &dev, nil
			}
		}

	} else {
		// Get a list of node orgs that the agbot is serving for this business policy.
		nodeOrgs := w.BusinessPolManager.GetServedNodeOrgs(polOrg, polName)
		if len(nodeOrgs) == 0 {
			glog.V(3).Infof("Business policy %v/%v exists but currently the agbot is not serving this policy for any organizations.", polOrg, polName)
			empty := make([]exchange.SearchResultDevice, 0, 0)
			return &empty, nil
		}

		// to make the search more efficient, the exchange only searchs the nodes what have been changed since bp_check_time.
		// if there is change for the business policy since last cycle, all nodes need to be checked again.
		bp_check_time := w.lastAgMakingTime
		if polLastUpdateTime > w.lastAgMakingTime {
			bp_check_time = 0
		}

		// Setup the search request body
		ser := exchange.SearchExchBusinessPolRequest{
			NodeOrgIds:   nodeOrgs,
			ChangedSince: bp_check_time,
		}

		// Invoke the exchange
		var resp interface{}
		resp = new(exchange.SearchExchBusinessPolResponse)
		targetURL := w.GetExchangeURL() + "orgs/" + polOrg + "/business/policies/" + polName + "/search"
		for {
			if err, tpErr := exchange.InvokeExchange(w.httpClient, "POST", targetURL, w.GetExchangeId(), w.GetExchangeToken(), ser, &resp); err != nil {
				if !strings.Contains(err.Error(), "status: 404") {
					return nil, err
				} else {
					empty := make([]exchange.SearchResultDevice, 0, 0)
					return &empty, nil
				}
			} else if tpErr != nil {
				glog.Warningf(tpErr.Error())
				time.Sleep(10 * time.Second)
				continue
			} else {
				glog.V(3).Infof("AgreementBotWorker found %v devices in exchange.", len(resp.(*exchange.SearchExchBusinessPolResponse).Devices))
				dev := resp.(*exchange.SearchExchBusinessPolResponse).Devices
				return &dev, nil
			}
		}
	}
}

func (w *AgreementBotWorker) syncOnInit() error {
	glog.V(3).Infof(AWlogString("beginning sync up."))

	// Search all agreement protocol buckets
	for _, agp := range policy.AllAgreementProtocols() {

		// Loop through our database and check each record for accuracy with the exchange and the blockchain
		if agreements, err := w.db.FindAgreements([]persistence.AFilter{persistence.UnarchivedAFilter()}, agp); err == nil {

			neededBCInstances := make(map[string]map[string]map[string]bool)

			for _, ag := range agreements {

				// Make a list of all blockchain instances in use by agreements in our DB so that we can make sure there is a
				// blockchain client running for each instance.
				bcType, bcName, bcOrg := w.consumerPH[ag.AgreementProtocol].GetKnownBlockchain(&ag)

				if len(neededBCInstances[bcOrg]) == 0 {
					neededBCInstances[bcOrg] = make(map[string]map[string]bool)
				}
				if len(neededBCInstances[bcOrg][bcType]) == 0 {
					neededBCInstances[bcOrg][bcType] = make(map[string]bool)
				}
				neededBCInstances[bcOrg][bcType][bcName] = true

				// If the agreement has received a reply then we just need to make sure that the policy manager's agreement counts
				// are correct. Even for already timedout agreements, the governance process will cleanup old and outdated agreements,
				// so we don't need to do anything here.
				if ag.AgreementCreationTime != 0 {
					if pol, err := policy.DemarshalPolicy(ag.Policy); err != nil {
						glog.Errorf(AWlogString(fmt.Sprintf("unable to demarshal policy for agreement %v, error %v", ag.CurrentAgreementId, err)))
					} else if existingPol := w.pm.GetPolicy(ag.Org, pol.Header.Name); existingPol == nil {
						glog.Errorf(AWlogString(fmt.Sprintf("agreement %v has a policy %v that doesn't exist anymore", ag.CurrentAgreementId, pol.Header.Name)))
						// Update state in exchange
						if err := DeleteConsumerAgreement(w.Config.Collaborators.HTTPClientFactory.NewHTTPClient(nil), w.GetExchangeURL(), w.GetExchangeId(), w.GetExchangeToken(), ag.CurrentAgreementId); err != nil {
							glog.Errorf(AWlogString(fmt.Sprintf("error deleting agreement %v in exchange: %v", ag.CurrentAgreementId, err)))
						}
						// Remove any workload usage records so that a new agreement will be made starting from the highest priority workload
						if err := w.db.DeleteWorkloadUsage(ag.DeviceId, ag.PolicyName); err != nil {
							glog.Warningf(AWlogString(fmt.Sprintf("error deleting workload usage for %v using policy %v, error: %v", ag.DeviceId, ag.PolicyName, err)))
						}
						// Indicate that the agreement is timed out
						if _, err := w.db.AgreementTimedout(ag.CurrentAgreementId, agp); err != nil {
							glog.Errorf(AWlogString(fmt.Sprintf("error marking agreement %v terminated: %v", ag.CurrentAgreementId, err)))
						}
						w.consumerPH[agp].HandleAgreementTimeout(NewAgreementTimeoutCommand(ag.CurrentAgreementId, ag.AgreementProtocol, w.consumerPH[agp].GetTerminationCode(TERM_REASON_POLICY_CHANGED)), w.consumerPH[agp])
					} else if err := w.pm.MatchesMine(ag.Org, pol); err != nil {
						glog.Warningf(AWlogString(fmt.Sprintf("agreement %v has a policy %v that has changed: %v", ag.CurrentAgreementId, pol.Header.Name, err)))

						// Remove any workload usage records (non-HA) or mark for pending upgrade (HA). There might not be a workload usage record
						// if the consumer policy does not specify the workload priority section.
						if wlUsage, err := w.db.FindSingleWorkloadUsageByDeviceAndPolicyName(ag.DeviceId, ag.PolicyName); err != nil {
							glog.Warningf(AWlogString(fmt.Sprintf("error retreiving workload usage for %v using policy %v, error: %v", ag.DeviceId, ag.PolicyName, err)))
						} else if wlUsage != nil && len(wlUsage.HAPartners) != 0 && wlUsage.PendingUpgradeTime != 0 {
							// Skip this agreement, it is part of an HA group where another member is upgrading
							continue
						} else if wlUsage != nil && len(wlUsage.HAPartners) != 0 && wlUsage.PendingUpgradeTime == 0 {
							for _, partnerId := range wlUsage.HAPartners {
								if _, err := w.db.UpdatePendingUpgrade(partnerId, ag.PolicyName); err != nil {
									glog.Warningf(AWlogString(fmt.Sprintf("could not update pending workload upgrade for %v using policy %v, error: %v", partnerId, ag.PolicyName, err)))
								}
							}
							// Choose this device's agreement within the HA group to start upgrading
							w.cleanupAgreement(&ag)
						} else {
							// Non-HA device or agrement without workload priority in the policy, re-make the agreement
							w.cleanupAgreement(&ag)
						}
					} else if err := w.pm.AttemptingAgreement([]policy.Policy{*existingPol}, ag.CurrentAgreementId, ag.Org); err != nil {
						glog.Errorf(AWlogString(fmt.Sprintf("cannot update agreement count for %v, error: %v", ag.CurrentAgreementId, err)))
					} else if err := w.pm.FinalAgreement([]policy.Policy{*existingPol}, ag.CurrentAgreementId, ag.Org); err != nil {
						glog.Errorf(AWlogString(fmt.Sprintf("cannot update agreement count for %v, error: %v", ag.CurrentAgreementId, err)))

						// There is a small window where an agreement might not have been recorded in the exchange. Let's just make sure.
					} else {

						var exchangeAgreement map[string]exchange.AgbotAgreement
						var resp interface{}
						resp = new(exchange.AllAgbotAgreementsResponse)
						targetURL := w.GetExchangeURL() + "orgs/" + exchange.GetOrg(w.GetExchangeId()) + "/agbots/" + exchange.GetId(w.GetExchangeId()) + "/agreements/" + ag.CurrentAgreementId

						if err, tpErr := exchange.InvokeExchange(w.httpClient, "GET", targetURL, w.GetExchangeId(), w.GetExchangeToken(), nil, &resp); err != nil || tpErr != nil {
							glog.Errorf(AWlogString(fmt.Sprintf("encountered error getting agbot info from exchange, error %v, transport error %v", err, tpErr)))
							continue
						} else {
							exchangeAgreement = resp.(*exchange.AllAgbotAgreementsResponse).Agreements
							glog.V(5).Infof(AWlogString(fmt.Sprintf("found agreements %v in the exchange.", exchangeAgreement)))

							if _, there := exchangeAgreement[ag.CurrentAgreementId]; !there {
								glog.V(3).Infof(AWlogString(fmt.Sprintf("agreement %v missing from exchange, adding it back in.", ag.CurrentAgreementId)))
								state := ""
								if ag.AgreementFinalizedTime != 0 {
									state = "Finalized Agreement"
								} else if ag.CounterPartyAddress != "" {
									state = "Producer Agreed"
								} else if ag.AgreementCreationTime != 0 {
									state = "Formed Proposal"
								} else {
									state = "unknown"
								}
								if err := w.recordConsumerAgreementState(ag.CurrentAgreementId, pol, ag.Org, state); err != nil {
									glog.Errorf(AWlogString(fmt.Sprintf("unable to record agreement %v state %v, error %v", ag.CurrentAgreementId, state, err)))
								}
							}
						}
						glog.V(3).Infof(AWlogString(fmt.Sprintf("added agreement %v to policy agreement counter.", ag.CurrentAgreementId)))
					}

					// This state should never occur, but could if there was an error along the way. It means that a DB record
					// was created for this agreement but the record was never updated with the creation time, which is supposed to occur
					// immediately following creation of the record. Further, if this were to occur, then the exchange should not have been
					// updated, so there is no reason to try to clean that up. Same is true for the workload usage records.
				} else if ag.AgreementInceptionTime != 0 && ag.AgreementCreationTime == 0 {
					if err := w.db.DeleteAgreement(ag.CurrentAgreementId, agp); err != nil {
						glog.Errorf(AWlogString(fmt.Sprintf("error deleting partially created agreement: %v, error: %v", ag.CurrentAgreementId, err)))
					}
				}

			}

			// Fire off start requests for each BC client that we need running. The blockchain worker and the container worker will tolerate
			// a start request for containers that are already running.
			glog.V(3).Infof(AWlogString(fmt.Sprintf("discovered BC instances in DB %v", neededBCInstances)))
			for org, typeMap := range neededBCInstances {
				for typeName, instMap := range typeMap {
					for instName, _ := range instMap {
						w.Messages() <- events.NewNewBCContainerMessage(events.NEW_BC_CLIENT, typeName, instName, org, w.GetExchangeURL(), w.GetExchangeId(), w.GetExchangeToken())
					}
				}
			}

		} else {
			return errors.New(AWlogString(fmt.Sprintf("error searching database: %v", err)))
		}
	}

	glog.V(3).Infof(AWlogString("sync up completed normally."))
	return nil
}

func (w *AgreementBotWorker) cleanupAgreement(ag *persistence.Agreement) {
	// Update state in exchange
	if err := DeleteConsumerAgreement(w.Config.Collaborators.HTTPClientFactory.NewHTTPClient(nil), w.GetExchangeURL(), w.GetExchangeId(), w.GetExchangeToken(), ag.CurrentAgreementId); err != nil {
		glog.Errorf(AWlogString(fmt.Sprintf("error deleting agreement %v in exchange: %v", ag.CurrentAgreementId, err)))
	}

	// Delete this workload usage record so that a new agreement will be made starting from the highest priority workload
	if err := w.db.DeleteWorkloadUsage(ag.DeviceId, ag.PolicyName); err != nil {
		glog.Warningf(AWlogString(fmt.Sprintf("error deleting workload usage for %v using policy %v, error: %v", ag.DeviceId, ag.PolicyName, err)))
	}

	// Indicate that the agreement is timed out
	if _, err := w.db.AgreementTimedout(ag.CurrentAgreementId, ag.AgreementProtocol); err != nil {
		glog.Errorf(AWlogString(fmt.Sprintf("error marking agreement %v terminated: %v", ag.CurrentAgreementId, err)))
	}

	w.consumerPH[ag.AgreementProtocol].HandleAgreementTimeout(NewAgreementTimeoutCommand(ag.CurrentAgreementId, ag.AgreementProtocol, w.consumerPH[ag.AgreementProtocol].GetTerminationCode(TERM_REASON_POLICY_CHANGED)), w.consumerPH[ag.AgreementProtocol])
}

func (w *AgreementBotWorker) recordConsumerAgreementState(agreementId string, pol *policy.Policy, org string, state string) error {

	workload := pol.Workloads[0].WorkloadURL

	glog.V(5).Infof(AWlogString(fmt.Sprintf("setting agreement %v for workload %v state to %v", agreementId, workload, state)))

	as := new(exchange.PutAgbotAgreementState)
	as.Service = exchange.WorkloadAgreement{
		Org:     exchange.GetOrg(pol.PatternId),
		Pattern: exchange.GetId(pol.PatternId),
		URL:     workload,
	}
	as.State = state

	var resp interface{}
	resp = new(exchange.PostDeviceResponse)
	targetURL := w.GetExchangeURL() + "orgs/" + exchange.GetOrg(w.GetExchangeId()) + "/agbots/" + exchange.GetId(w.GetExchangeId()) + "/agreements/" + agreementId
	for {
		if err, tpErr := exchange.InvokeExchange(w.httpClient, "PUT", targetURL, w.GetExchangeId(), w.GetExchangeToken(), &as, &resp); err != nil {
			glog.Errorf(err.Error())
			return err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(5).Infof(AWlogString(fmt.Sprintf("set agreement %v to state %v", agreementId, state)))
			return nil
		}
	}

}

func listContains(list string, target string) bool {
	ignoreAttribs := strings.Split(list, ",")
	for _, propName := range ignoreAttribs {
		if propName == target {
			return true
		}
	}
	return false
}

func (w *AgreementBotWorker) registerPublicKey() error {
	glog.V(5).Infof(AWlogString(fmt.Sprintf("registering agbot public key")))

	as := exchange.CreateAgbotPublicKeyPatch(w.Config.AgreementBot.MessageKeyPath)
	var resp interface{}
	resp = new(exchange.PostDeviceResponse)
	targetURL := w.GetExchangeURL() + "orgs/" + exchange.GetOrg(w.GetExchangeId()) + "/agbots/" + exchange.GetId(w.GetExchangeId())
	for {
		if err, tpErr := exchange.InvokeExchange(w.httpClient, "PATCH", targetURL, w.GetExchangeId(), w.GetExchangeToken(), &as, &resp); err != nil {
			glog.Errorf(err.Error())
			return err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(5).Infof(AWlogString(fmt.Sprintf("patched agbot public key %x", as)))
			return nil
		}
	}
}

func (w *AgreementBotWorker) serviceResolver(wURL string, wOrg string, wVersion string, wArch string) (*policy.APISpecList, error) {

	asl, _, _, err := exchange.GetHTTPServiceResolverHandler(w)(wURL, wOrg, wVersion, wArch)
	if err != nil {
		glog.Errorf(AWlogString(fmt.Sprintf("unable to resolve %v %v, error %v", wURL, wOrg, err)))
	}
	return asl, err
}

// Generate policies.
// 1. generate policy files based on pattern metadata in the exchange.
// 2. generate policies in memory on business metadata in the exchange.
// 3. capture object destination policies in memory from the MMS. This happens within #2.
// The return value from this function is
// the length of time the caller should wait before calling again. If -1 is returned, there was an error.
func (w *AgreementBotWorker) GeneratePolicies() int {

	glog.V(5).Infof(AWlogString(fmt.Sprintf("scanning patterns for updates")))
	if err := w.internalGeneratePolicyFromPatterns(); err != nil {
		glog.Errorf(AWlogString(fmt.Sprintf("unable to process patterns, error %v", err)))
		return -1
	}
	glog.V(5).Infof(AWlogString(fmt.Sprintf("pattern manager initialized: %v", w.PatternManager.ShortString())))

	glog.V(5).Infof(AWlogString(fmt.Sprintf("scanning businesses and object policies for updates")))
	if err := w.internalGeneratePolicyFromBusinessPols(); err != nil {
		glog.Errorf(AWlogString(fmt.Sprintf("unable to process business and object policies, error %v", err)))
		return -1
	}
	glog.V(5).Infof(AWlogString(fmt.Sprintf("scanning service policies for updates")))
	if err := w.internalUpdateServicePolicies(); err != nil {
		glog.Errorf(AWlogString(fmt.Sprintf("unable to process service policies, error %v", err)))
		return -1
	}
	glog.V(5).Infof(AWlogString(fmt.Sprintf("business policy manager initialized: %v", w.BusinessPolManager.ShortString())))

	return 0
}

// Generate policy files based on pattern metadata in the exchange. A list of orgs and patterns is
// configured for the agbot to serve. Policy files are created, updated and deleted based on this
// metadata and based on the pattern metadata itself. This function assumes that the
// PolicyFileChangeWatcher will observe changes to policy files made by this function and act as usual
// to make or cancel agreements.
func (w *AgreementBotWorker) internalGeneratePolicyFromPatterns() error {

	// Get the configured org/pattern pairs for this agbot.
	pats, err := w.getAgbotPatterns()
	if err != nil {
		return errors.New(fmt.Sprintf("unable to retrieve agbot pattern metadata, error %v", err))
	}

	// Consume the configured org/pattern pairs into the PatternManager
	if err := w.PatternManager.SetCurrentPatterns(pats, w.Config.AgreementBot.PolicyPath); err != nil {
		return errors.New(fmt.Sprintf("unable to process agbot served patterns metadata %v, error %v", pats, err))
	}

	// Iterate over each org in the PatternManager and process all the patterns in that org
	for org, _ := range w.PatternManager.OrgPatterns {

		var exchangePatternMetadata map[string]exchange.Pattern
		var err error

		// check if the org exists on the exchange or not
		if _, err = exchange.GetOrganization(w.Config.Collaborators.HTTPClientFactory, org, w.GetExchangeURL(), w.GetExchangeId(), w.GetExchangeToken()); err != nil {
			// org does not exist is returned as an error
			glog.V(5).Infof(AWlogString(fmt.Sprintf("unable to get organization %v: %v", org, err)))
			exchangePatternMetadata = make(map[string]exchange.Pattern)
		} else {
			// Query exchange for all patterns in the org
			if exchangePatternMetadata, err = exchange.GetPatterns(w.Config.Collaborators.HTTPClientFactory, org, "", w.GetExchangeURL(), w.GetExchangeId(), w.GetExchangeToken()); err != nil {
				return errors.New(fmt.Sprintf("unable to get patterns for org %v, error %v", org, err))
			}
		}

		// Check for pattern metadata changes and update policy files accordingly
		if err := w.PatternManager.UpdatePatternPolicies(org, exchangePatternMetadata, w.Config.AgreementBot.PolicyPath); err != nil {
			return errors.New(fmt.Sprintf("unable to update policies for org %v, error %v", org, err))
		}
	}

	return nil

}

func (w *AgreementBotWorker) getAgbotPatterns() (map[string]exchange.ServedPattern, error) {

	var resp interface{}
	resp = new(exchange.GetAgbotsPatternsResponse)
	targetURL := w.GetExchangeURL() + "orgs/" + exchange.GetOrg(w.GetExchangeId()) + "/agbots/" + exchange.GetId(w.GetExchangeId()) + "/patterns"
	for {
		if err, tpErr := exchange.InvokeExchange(w.httpClient, "GET", targetURL, w.GetExchangeId(), w.GetExchangeToken(), nil, &resp); err != nil {
			glog.Errorf(AWlogString(err.Error()))
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(AWlogString(tpErr.Error()))
			time.Sleep(10 * time.Second)
			continue
		} else {
			pats := resp.(*exchange.GetAgbotsPatternsResponse).Patterns
			glog.V(5).Infof(AWlogString(fmt.Sprintf("retrieved agbot patterns from exchange %v", pats)))
			return pats, nil
		}
	}

}

// Generate policy files based on business policy metadata in the exchange. A list of orgs and business policies is
// configured for the agbot to serve. Policies are created, updated and deleted based on this
// metadata and based on the business policy metadata itself.
func (w *AgreementBotWorker) internalGeneratePolicyFromBusinessPols() error {

	// Get the configured (policy org, business policy, node org) triplets for this agbot.
	pats, err := w.getAgbotBusinessPols()
	if err != nil {
		return errors.New(fmt.Sprintf("unable to retrieve served businees policy metadata for this agbot, error %v", err))
	}

	// Consume the configured (policy org, business policy, node org) triplets into the BusinessPolicyManager
	if err := w.BusinessPolManager.SetCurrentBusinessPolicies(pats, w.pm); err != nil {
		return errors.New(fmt.Sprintf("unable to process agbot served business policy metadata %v, error %v", pats, err))
	}

	// Consume the configured (policy org, business policy, node org) triplets into the ObjectPolicyManager
	if err := w.MMSObjectPM.SetCurrentPolicyOrgs(pats); err != nil {
		return errors.New(fmt.Sprintf("unable to process agbot served business policy metadata %v, error %v", pats, err))
	}

	// Iterate over each org in the BusinessPolManager and process all the business policies in that org
	oldTime := w.mmsObjectPollTime
	w.mmsObjectPollTime = time.Now().UTC().UnixNano()
	for _, org := range w.BusinessPolManager.GetAllPolicyOrgs() {

		var exchPolsMetadata map[string]exchange.ExchangeBusinessPolicy
		var mmsObjPolicies *exchange.ObjectDestinationPolicies
		var err error

		// check if the org exists on the exchange or not
		getOrganization := exchange.GetHTTPExchangeOrgHandler(w)
		if _, err = getOrganization(org); err != nil {
			// org does not exist is returned as an error
			glog.V(5).Infof(AWlogString(fmt.Sprintf("unable to get organization %v: %v", org, err)))
			exchPolsMetadata = make(map[string]exchange.ExchangeBusinessPolicy)
			mmsObjPolicies = new(exchange.ObjectDestinationPolicies)
		} else {
			// Query exchange for all business policies in the org
			getBusinessPolicies := exchange.GetHTTPBusinessPoliciesHandler(w)
			if exchPolsMetadata, err = getBusinessPolicies(org, ""); err != nil {
				return errors.New(fmt.Sprintf("unable to get business polices for org %v, error %v", org, err))
			}

			// Query MMS for all object policies in the org since the last time we called.
			getMMSObjectPolicies := exchange.GetHTTPObjectPolicyUpdatesQueryHandler(w)
			if mmsObjPolicies, err = getMMSObjectPolicies(org, oldTime); err != nil {
				return errors.New(fmt.Sprintf("unable to get object polices for org %v, error %v", org, err))
			}

		}

		// Check for business policy metadata changes and update policies accordingly
		if err := w.BusinessPolManager.UpdatePolicies(org, exchPolsMetadata, w.pm); err != nil {
			return errors.New(fmt.Sprintf("unable to update business policies for org %v, error %v", org, err))
		}

		// Check for policy metadata changes and update policies accordingly. Publish any status change events.
		if events, err := w.MMSObjectPM.UpdatePolicies(org, mmsObjPolicies, exchange.GetHTTPObjectQueryHandler(w)); err != nil {
			return errors.New(fmt.Sprintf("unable to update object policies for org %v, error %v", org, err))
		} else {
			for _, ev := range events {
				w.Messages() <- ev
			}
		}
	}

	return nil

}

// For each business policy in the BusinessPolicyManager, this function updates the service policies with
// the latest changes
func (w *AgreementBotWorker) internalUpdateServicePolicies() error {
	// map keyed  by the service keys
	updatedServicePols := make(map[string]int, 10)

	// Iterate over each org in the BusinessPolManager and process all the business policies in that org
	for _, org := range w.BusinessPolManager.GetAllPolicyOrgs() {
		orgMap := w.BusinessPolManager.GetAllBusinessPolicyEntriesForOrg(org)
		if orgMap != nil {
			for bpName, bPol := range orgMap {
				if bPol.ServicePolicies != nil {
					for svcKey, _ := range bPol.ServicePolicies {
						if _, ok := updatedServicePols[svcKey]; !ok {
							servicePolicy, err := w.getServicePolicy(svcKey)
							if err != nil {
								return errors.New(fmt.Sprintf("Error getting busibess policy for %v, %v", svcKey, err))
							} else if servicePolicy == nil {
								// delete the service policy from all the business policies that reference it.
								if err := w.BusinessPolManager.RemoveServicePolicy(org, bpName, svcKey); err != nil {
									glog.Errorf(AWlogString(fmt.Sprintf("Error deleting service policy %v in the business policy manager: %v", svcKey, err)))
								}
							} else {
								// update the service policy for all the business policies that reference it.
								if err := w.BusinessPolManager.AddServicePolicy(org, bpName, svcKey, servicePolicy); err != nil {
									glog.Errorf(AWlogString(fmt.Sprintf("Error updating service policy %v in the business policy manager: %v", svcKey, err)))
								}
							}
							updatedServicePols[svcKey] = 1
						}
					}
				}
			}
		}
	}
	return nil
}

// Get service policy
func (w *AgreementBotWorker) getServicePolicy(svcId string) (*externalpolicy.ExternalPolicy, error) {

	servicePolicyHandler := exchange.GetHTTPServicePolicyWithIdHandler(w)
	servicePolicy, err := servicePolicyHandler(svcId)
	if err != nil {
		return nil, fmt.Errorf("error trying to query service policy for %v: %v", svcId, err)
	} else if servicePolicy == nil {
		return nil, nil
	} else {
		extPolicy := servicePolicy.GetExternalPolicy()
		return &extPolicy, nil
	}
}

func (w *AgreementBotWorker) getAgbotBusinessPols() (map[string]exchange.ServedBusinessPolicy, error) {

	var resp interface{}
	resp = new(exchange.GetAgbotsBusinessPolsResponse)
	targetURL := w.GetExchangeURL() + "orgs/" + exchange.GetOrg(w.GetExchangeId()) + "/agbots/" + exchange.GetId(w.GetExchangeId()) + "/businesspols"
	for {
		if err, tpErr := exchange.InvokeExchange(w.httpClient, "GET", targetURL, w.GetExchangeId(), w.GetExchangeToken(), nil, &resp); err != nil {
			glog.Errorf(AWlogString(err.Error()))
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(AWlogString(tpErr.Error()))
			time.Sleep(10 * time.Second)
			continue
		} else {
			pats := resp.(*exchange.GetAgbotsBusinessPolsResponse).BusinessPols
			glog.V(5).Infof(AWlogString(fmt.Sprintf("retrieved agbot servced business policy names from exchange %v", pats)))
			return pats, nil
		}
	}
}

// Heartbeat to the exchange. This function is called by the heartbeat subworker.
func (w *AgreementBotWorker) heartBeat() int {

	if w.Config.AgreementBot.ExchangeVersionCheckIntervalM > 0 {
		// get the exchange version check interval and change to seconds
		check_interval := w.Manager.Config.AgreementBot.ExchangeVersionCheckIntervalM * 60

		// check the exchange version when time is right
		time_now := time.Now().Unix()
		if w.lastExchVerCheck == 0 || time_now-w.lastExchVerCheck >= int64(check_interval) {
			w.lastExchVerCheck = time_now

			// log error if the current exchange version does not meet the requirement
			if err := version.VerifyExchangeVersion(w.Config.Collaborators.HTTPClientFactory, w.GetExchangeURL(), w.GetExchangeId(), w.GetExchangeToken(), false); err != nil {
				glog.Errorf(AWlogString(fmt.Sprintf("Error verifiying exchange version. error: %v", err)))
			}
		}
	}

	// now do the hearbeat
	targetURL := w.GetExchangeURL() + "orgs/" + exchange.GetOrg(w.GetExchangeId()) + "/agbots/" + exchange.GetId(w.GetExchangeId()) + "/heartbeat"
	exchange.Heartbeat(w.Config.Collaborators.HTTPClientFactory.NewHTTPClient(nil), targetURL, w.GetExchangeId(), w.GetExchangeToken())

	return 0
}

// Heartbeat to the database. This function is called by the database heartbeat subworker.
func (w *AgreementBotWorker) databaseHeartBeat() int {

	if err := w.db.HeartbeatPartition(); err != nil {
		glog.Errorf(AWlogString(fmt.Sprintf("Error heartbeating to the database, error: %v", err)))
	}

	return 0
}

// Ask the database to check for stale partitions and move them into our partition if one is found.
func (w *AgreementBotWorker) stalePartitions() int {

	if err := w.db.MovePartition(w.Config.GetPartitionStale()); err != nil {
		glog.Errorf(AWlogString(fmt.Sprintf("Error claiming an unowned partition, error: %v", err)))
	}
	return 0
}

// ==========================================================================================================
// Utility functions

var AWlogString = func(v interface{}) string {
	return fmt.Sprintf("AgreementBotWorker %v", v)
}
