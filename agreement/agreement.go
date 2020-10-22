package agreement

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/abstractprotocol"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/eventlog"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangesync"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/producer"
	"github.com/open-horizon/anax/worker"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// messages for eventlog
const (
	EL_AG_UNABLE_READ_POL_FILE                   = "Unable to read policy file %v for service %v, error: %v"
	EL_AG_START_ADVERTISE_POL                    = "Start policy advertising with the Exchange for service %v/%v."
	EL_AG_UNABLE_ADVERTISE_POL                   = "Unable to advertise policies with Exchange for service %v/%v, error: %v"
	EL_AG_COMPLETE_ADVERTISE_POL                 = "Complete policy advertising with the Exchange for service %v/%v."
	EL_AG_UNABLE_READ_NODE_POL_FROM_DB           = "unable to read node policy from the local database. %v"
	EL_AG_UNABLE_READ_NODE_FROM_DB               = "Unable to read node object from the local database. %v"
	EL_AG_UNABLE_SYNC_NODE_POL_WITH_EXCH         = "Unable to sync the local node policy with the Exchange copy. Error: %v"
	EL_AG_NODE_POL_SYNCED_WITH_EXCH              = "Node policy updated with the Exchange copy: %v"
	EL_AG_UNABLE_SYNC_NODE_UI_WITH_EXCH          = "Unable to sync the local node user input with the Exchange copy. Error: %v"
	EL_AG_NODE_UI_SYNCED_WITH_EXCH               = "Node user input updated with the Exchange copy. The changed user inputs are: %v"
	EL_AG_NODE_CANNOT_VERIFY_AG                  = "Node could not verify the agreement %v with the consumer. Will cancel it"
	EL_AG_NODE_IS_OFFLINE                        = "Node is offline. Logging of periodic offline error messages will be curtailed until connection is restored"
	EL_AG_UNABLE_SYNC_NODE_WITH_EXCH             = "Unable to sync the node with the Exchange copy. Error: %v"
	EL_AG_ERR_RETRIEVE_SVC_CONFIGSTATE_FROM_EXCH = "Unable to retrieve the service configuration state for node resource %v from the Exchange, error %v"
	EL_AG_UNABLE_READ_NODE_EXCH_PATTERN_FROM_DB  = "Unable to retrieve the saved node exchange pattern from the local database. %v"
	EL_AG_UNABLE_WRITE_NODE_EXCH_PATTERN_TO_DB   = "Unable to save the new node exchange pattern %v to the local database. Error: %v"
	EL_AG_TERM_UNABLE_SYNC_CONTAINERS            = "anax terminating, unable to sync up containers."
	EL_AG_TERM_UNABLE_SYNC_AGS                   = "anax terminating, unable to complete agreement sync up. %v"
)

// This is does nothing useful at run time.
// This code is only used at compile time to make the eventlog messages get into the catalog so that
// they can be translated.
// The event log messages will be saved in English. But the CLI can request them in different languages.
func MarkI18nMessages() {
	// get message printer. anax default language is English
	msgPrinter := i18n.GetMessagePrinter()

	msgPrinter.Sprintf(EL_AG_UNABLE_READ_POL_FILE)
	msgPrinter.Sprintf(EL_AG_START_ADVERTISE_POL)
	msgPrinter.Sprintf(EL_AG_UNABLE_ADVERTISE_POL)
	msgPrinter.Sprintf(EL_AG_COMPLETE_ADVERTISE_POL)
	msgPrinter.Sprintf(EL_AG_UNABLE_READ_NODE_POL_FROM_DB)
	msgPrinter.Sprintf(EL_AG_UNABLE_READ_NODE_FROM_DB)
	msgPrinter.Sprintf(EL_AG_UNABLE_SYNC_NODE_POL_WITH_EXCH)
	msgPrinter.Sprintf(EL_AG_NODE_POL_SYNCED_WITH_EXCH)
	msgPrinter.Sprintf(EL_AG_UNABLE_SYNC_NODE_UI_WITH_EXCH)
	msgPrinter.Sprintf(EL_AG_NODE_UI_SYNCED_WITH_EXCH)
	msgPrinter.Sprintf(EL_AG_NODE_CANNOT_VERIFY_AG)
	msgPrinter.Sprintf(EL_AG_NODE_IS_OFFLINE)
	msgPrinter.Sprintf(EL_AG_UNABLE_SYNC_NODE_WITH_EXCH)
	msgPrinter.Sprintf(EL_AG_ERR_RETRIEVE_SVC_CONFIGSTATE_FROM_EXCH)
	msgPrinter.Sprintf(EL_AG_UNABLE_READ_NODE_EXCH_PATTERN_FROM_DB)
	msgPrinter.Sprintf(EL_AG_UNABLE_WRITE_NODE_EXCH_PATTERN_TO_DB)
	msgPrinter.Sprintf(EL_AG_TERM_UNABLE_SYNC_CONTAINERS)
	msgPrinter.Sprintf(EL_AG_TERM_UNABLE_SYNC_AGS)
}

// must be safely-constructed!!
type AgreementWorker struct {
	worker.BaseWorker        // embedded field
	db                       *bolt.DB
	devicePattern            string
	protocols                map[string]bool
	pm                       *policy.PolicyManager
	containerSyncUpEvent     bool
	containerSyncUpSucessful bool
	producerPH               map[string]producer.ProducerProtocolHandler
	lastExchVerCheck         int64
	hznOffline               bool
	limitedRetryEC           exchange.ExchangeContext
}

func NewAgreementWorker(name string, cfg *config.HorizonConfig, db *bolt.DB, pm *policy.PolicyManager) *AgreementWorker {

	var ec *worker.BaseExchangeContext
	var lrec exchange.ExchangeContext
	pattern := ""
	if dev, _ := persistence.FindExchangeDevice(db); dev != nil {
		ec = worker.NewExchangeContext(fmt.Sprintf("%v/%v", dev.Org, dev.Id), dev.Token, cfg.Edge.ExchangeURL, cfg.GetCSSURL(), cfg.Collaborators.HTTPClientFactory)
		pattern = dev.Pattern
		lrec = newLimitedRetryExchangeContext(ec)
	}

	worker := &AgreementWorker{
		BaseWorker:       worker.NewBaseWorker(name, cfg, ec),
		db:               db,
		devicePattern:    pattern,
		protocols:        make(map[string]bool),
		pm:               pm,
		producerPH:       make(map[string]producer.ProducerProtocolHandler),
		lastExchVerCheck: 0,
		hznOffline:       false,
		limitedRetryEC:   lrec,
	}

	glog.Info("Starting Agreement worker")
	worker.Start(worker, 0)
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

func (w *AgreementWorker) Messages() chan events.Message {
	return w.BaseWorker.Manager.Messages
}

func (w *AgreementWorker) NewEvent(incoming events.Message) {

	switch incoming.(type) {
	case *events.EdgeRegisteredExchangeMessage:
		msg, _ := incoming.(*events.EdgeRegisteredExchangeMessage)
		w.Commands <- NewDeviceRegisteredCommand(msg)

	case *events.PolicyCreatedMessage:
		msg, _ := incoming.(*events.PolicyCreatedMessage)

		switch msg.Event().Id {
		case events.NEW_POLICY:
			w.Commands <- NewAdvertisePolicyCommand(msg.PolicyFile())
		default:
			glog.Errorf("AgreementWorker received Unsupported event: %v", incoming.Event().Id)
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

	case *events.ExchangeDeviceMessage:
		msg, _ := incoming.(*events.ExchangeDeviceMessage)
		switch msg.Event().Id {
		case events.RECEIVED_EXCHANGE_DEV_MSG:
			w.Commands <- producer.NewExchangeMessageCommand(*msg)
		}

	case *events.DeviceContainersSyncedMessage:
		msg, _ := incoming.(*events.DeviceContainersSyncedMessage)
		switch msg.Event().Id {
		case events.DEVICE_CONTAINERS_SYNCED:
			w.containerSyncUpSucessful = msg.IsCompleted()
			w.containerSyncUpEvent = true
		}

	case *events.EdgeConfigCompleteMessage:
		msg, _ := incoming.(*events.EdgeConfigCompleteMessage)
		switch msg.Event().Id {
		case events.NEW_DEVICE_CONFIG_COMPLETE:
			w.Commands <- NewEdgeConfigCompleteCommand(msg)
		}

	case *events.NodeShutdownMessage:
		msg, _ := incoming.(*events.NodeShutdownMessage)
		switch msg.Event().Id {
		case events.START_UNCONFIGURE:
			w.Commands <- worker.NewBeginShutdownCommand()
		}

	case *events.NodeShutdownCompleteMessage:
		msg, _ := incoming.(*events.NodeShutdownCompleteMessage)
		switch msg.Event().Id {
		case events.UNCONFIGURE_COMPLETE:
			w.Commands <- worker.NewTerminateCommand("shutdown")
		}

	case *events.NodePolicyMessage:
		msg, _ := incoming.(*events.NodePolicyMessage)
		switch msg.Event().Id {
		case events.UPDATE_POLICY, events.DELETED_POLICY:
			w.Commands <- NewNodePolicyChangedCommand(msg)
		}

	case *events.ExchangeChangeMessage:
		msg, _ := incoming.(*events.ExchangeChangeMessage)
		switch msg.Event().Id {
		case events.CHANGE_NODE_TYPE:
			w.Commands <- NewNodeChangeCommand()
		case events.CHANGE_NODE_POLICY_TYPE:
			w.Commands <- NewNodePolicyChangeCommand()
		}

	case *events.NodeHeartbeatStateChangeMessage:
		msg, _ := incoming.(*events.NodeHeartbeatStateChangeMessage)
		switch msg.Event().Id {
		case events.NODE_HEARTBEAT_RESTORED:

			// Now that heartbeating is restored, fire the functions to check on exchange state changes. If the node
			// was offline long enough, the exchange might have pruned changes we needed to see, which means we will
			// never see them now. So, assume there were some changes we care about.
			w.Commands <- NewNodeChangeCommand()
			w.Commands <- NewNodePolicyChangeCommand()

		}

	default: //nothing
	}

	return
}

// Initialize the agreement worker before it begins processing commands.
func (w *AgreementWorker) Initialize() bool {

	glog.Info(logString(fmt.Sprintf("started")))

	// Only check for container sync up when DockerEndpoint is set.
	// If it is not set, the docker client could not be initialized.
	if w.Config.Edge.DockerEndpoint != "" {
		// Block for the container syncup message, to make sure the docker state matches our local DB.
		for {
			if w.containerSyncUpEvent == false {
				time.Sleep(time.Duration(5) * time.Second)
				glog.V(3).Infof("AgreementWorker waiting for container syncup to be done.")
			} else if w.containerSyncUpSucessful {
				break
			} else {
				glog.Errorf(logString(fmt.Sprintf("Terminating, unable to sync up containers.")))
				eventlog.LogNodeEvent(w.db, persistence.SEVERITY_FATAL,
					persistence.NewMessageMeta(EL_AG_TERM_UNABLE_SYNC_CONTAINERS),
					persistence.EC_ERROR_CONTAINER_SYNC_ON_INIT,
					w.GetExchangeId(), exchange.GetOrg(w.GetExchangeId()), w.devicePattern, "")
				panic(logString(fmt.Sprintf("Terminating, unable to sync up containers")))
			}
		}
	}

	if w.GetExchangeToken() != "" {
		// populate the privFileName and pubFileName variables in the exchange.messaging.go
		if _, _, err := exchange.GetKeys(""); err != nil {
			glog.Errorf(logString(fmt.Sprintf("failed to get the messaging keys. %v", err)))
		}

		// Establish agreement protocol handlers
		for _, protocolName := range policy.AllAgreementProtocols() {
			pph := producer.CreateProducerPH(protocolName, w.BaseWorker.Manager.Config, w.db, w.pm, w)
			pph.Initialize()
			w.producerPH[protocolName] = pph
		}

		if err := w.syncNode(); err != nil {
			glog.Errorf(logString(fmt.Sprintf("Terminating, unable to sync up node. %v", err)))
			eventlog.LogNodeEvent(w.db, persistence.SEVERITY_FATAL,
				persistence.NewMessageMeta(EL_AG_TERM_UNABLE_SYNC_AGS, err.Error()),
				persistence.EC_ERROR_AGREEMENT_SYNC_ON_INIT,
				w.GetExchangeId(), exchange.GetOrg(w.GetExchangeId()), w.devicePattern, "")
			panic(logString(fmt.Sprintf("Terminating, unable to complete node sync up. %v", err)))
		}

		// Sync up between what's in our database versus what's in the exchange, and make sure that the policy manager's
		// agreement counts are correct. This function will cancel any agreements whose state might have changed
		// while the device was down. We will also check to make sure that policies havent changed. If they have, then
		// we will cancel agreements and allow them to re-negotiate.
		if err := w.syncOnInit(); err != nil {
			glog.Errorf(logString(fmt.Sprintf("Terminating, unable to sync up. %v", err)))
			eventlog.LogNodeEvent(w.db, persistence.SEVERITY_FATAL,
				persistence.NewMessageMeta(EL_AG_TERM_UNABLE_SYNC_AGS, err.Error()),
				persistence.EC_ERROR_AGREEMENT_SYNC_ON_INIT,
				w.GetExchangeId(), exchange.GetOrg(w.GetExchangeId()), w.devicePattern, "")
			panic(logString(fmt.Sprintf("Terminating, unable to complete agreement sync up. %v", err)))
		} else {
			w.Messages() <- events.NewDeviceAgreementsSyncedMessage(events.DEVICE_AGREEMENTS_SYNCED, true)
		}
	}

	// Publish what we have for the world to see
	if err := w.advertiseAllPolicies(); err != nil {
		glog.Warningf(logString(fmt.Sprintf("unable to advertise policies with exchange, error: %v", err)))
	}

	glog.Info(logString(fmt.Sprintf("waiting for commands.")))

	return true

}

func (w *AgreementWorker) syncNode() error {

	glog.V(3).Infof(logString("beginning sync up node."))

	// get the exchange node and save it locally
	if err := exchangesync.NodeInitalSetup(w.db, exchange.GetHTTPDeviceHandler(w)); err != nil {
		return errors.New(logString(fmt.Sprintf("Failed to initially set up local copy of the exchange node. %v. If the exchange url is changed, please run 'hzn unregister -D' command to clean up the local node before starting horizon service.", err)))
	}

	// setup the user input. exchange is the master.
	// However, if the exchange does not have user input, convert the old UserInputAttributes into UserInput format
	if err := exchangesync.NodeUserInputInitalSetup(w.db, exchange.GetHTTPPatchDeviceHandler(w)); err != nil {
		return errors.New(logString(fmt.Sprintf("Failed to initially set up node user input. %v", err)))
	}

	// setup the node policy. If neither node nor exchange has node policy, setup the default.
	// Otherwise, use the one from the exchange.
	if _, err := exchangesync.NodePolicyInitalSetup(w.db, w.Config, exchange.GetHTTPNodePolicyHandler(w), exchange.GetHTTPPutNodePolicyHandler(w)); err != nil {
		return errors.New(logString(fmt.Sprintf("Failed to initially set up node policy. %v", err)))
	}

	glog.V(3).Infof(logString("node sync up completed normally."))
	return nil
}

// Enter the command processing loop. Initialization is complete so wait for commands to
// perform. Commands are created as the result of events that are triggered elsewhere
// in the system. This function returns ture if the command was handled, false if not.
func (w *AgreementWorker) CommandHandler(command worker.Command) bool {

	// Handle the domain specific commands
	switch command.(type) {
	case *DeviceRegisteredCommand:
		cmd, _ := command.(*DeviceRegisteredCommand)
		w.handleDeviceRegistered(cmd)

	case *AdvertisePolicyCommand:
		cmd, _ := command.(*AdvertisePolicyCommand)

		a_tmp := strings.Split(cmd.PolicyFile, "/")
		svcName := a_tmp[len(a_tmp)-1]

		if newPolicy, err := policy.ReadPolicyFile(cmd.PolicyFile, w.Config.ArchSynonyms); err != nil {
			eventlog.LogAgreementEvent2(w.db, persistence.SEVERITY_ERROR,
				persistence.NewMessageMeta(EL_AG_UNABLE_READ_POL_FILE, cmd.PolicyFile, svcName, err.Error()),
				persistence.EC_ERROR_POLICY_ADVERTISING,
				"", persistence.WorkloadInfo{}, []persistence.ServiceSpec{}, "", "")

			glog.Errorf(logString(fmt.Sprintf("unable to read policy file %v into memory, error: %v", cmd.PolicyFile, err)))
		} else {
			w.pm.UpdatePolicy(exchange.GetOrg(w.GetExchangeId()), newPolicy)

			a_protocols := []string{}
			for _, p := range newPolicy.AgreementProtocols {
				a_protocols = append(a_protocols, p.Name)
			}
			protocols := strings.Join(a_protocols, ",")

			eventlog.LogAgreementEvent2(w.db, persistence.SEVERITY_INFO,
				persistence.NewMessageMeta(EL_AG_START_ADVERTISE_POL, newPolicy.APISpecs[0].Org, newPolicy.APISpecs[0].SpecRef),
				persistence.EC_START_POLICY_ADVERTISING,
				"", persistence.WorkloadInfo{}, producer.ConvertToServiceSpecs(newPolicy.APISpecs), "", protocols)
			// Publish what we have for the world to see
			if err := w.advertiseAllPolicies(); err != nil {
				eventlog.LogAgreementEvent2(w.db, persistence.SEVERITY_ERROR,
					persistence.NewMessageMeta(EL_AG_UNABLE_ADVERTISE_POL, newPolicy.APISpecs[0].Org, newPolicy.APISpecs[0].SpecRef, err.Error()),
					persistence.EC_ERROR_POLICY_ADVERTISING,
					"", persistence.WorkloadInfo{}, producer.ConvertToServiceSpecs(newPolicy.APISpecs), "", protocols)

				glog.Warningf(logString(fmt.Sprintf("unable to advertise policies with exchange, error: %v", err)))
			} else {
				eventlog.LogAgreementEvent2(w.db, persistence.SEVERITY_INFO,
					persistence.NewMessageMeta(EL_AG_COMPLETE_ADVERTISE_POL, newPolicy.APISpecs[0].Org, newPolicy.APISpecs[0].SpecRef),
					persistence.EC_COMPLETE_POLICY_ADVERTISING,
					"", persistence.WorkloadInfo{}, producer.ConvertToServiceSpecs(newPolicy.APISpecs), "", protocols)
			}
		}

	case *producer.ExchangeMessageCommand:
		cmd, _ := command.(*producer.ExchangeMessageCommand)
		exchangeMsg := new(exchange.DeviceMessage)
		if err := json.Unmarshal(cmd.Msg.ExchangeMessage(), &exchangeMsg); err != nil {
			glog.Errorf(logString(fmt.Sprintf("unable to demarshal exchange device message %v, error %v", cmd.Msg.ExchangeMessage(), err)))
		} else if there, err := w.messageInExchange(exchangeMsg.MsgId); err != nil {
			glog.Errorf(logString(fmt.Sprintf("unable to get messages from the exchange, error %v", err)))
			w.AddDeferredCommand(cmd)
			return true
		} else if !there {
			glog.V(3).Infof(logString(fmt.Sprintf("ignoring message %v, already deleted from the exchange.", exchangeMsg.MsgId)))
			return true
		}

		protocolMsg := cmd.Msg.ProtocolMessage()

		glog.V(3).Infof(logString(fmt.Sprintf("received message %v from the exchange", exchangeMsg.MsgId)))

		// Process the message if it's a proposal.
		deleteMessage := true
		proposalAccepted := false

		if msgProtocol, err := abstractprotocol.ExtractProtocol(protocolMsg); err != nil {
			glog.Errorf(logString(fmt.Sprintf("unable to extract agreement protocol name from message %v", protocolMsg)))
		} else if _, ok := w.producerPH[msgProtocol]; !ok {
			glog.Infof(logString(fmt.Sprintf("unable to direct exchange message %v to a protocol handler, deleting it.", protocolMsg)))
		} else if p, err := w.producerPH[msgProtocol].AgreementProtocolHandler("", "", "").ValidateProposal(protocolMsg); err != nil {
			glog.V(5).Infof(logString(fmt.Sprintf("Proposal handler ignoring non-proposal message: %s due to %v", cmd.Msg.ShortProtocolMessage(), err)))
			deleteMessage = false
		} else if pDevice, err := persistence.FindExchangeDevice(w.db); err != nil {
			glog.Errorf(logString(fmt.Sprintf("unable to get device from the local database. %v", err)))
		} else if pDevice != nil && pDevice.Config.State == persistence.CONFIGSTATE_CONFIGURED {

			deleteMessage, proposalAccepted = w.producerPH[msgProtocol].HandleProposalMessage(p, protocolMsg, exchangeMsg)

			if proposalAccepted {
				// send a message to let the changes worker know that we have received a proposal message
				w.Messages() <- events.NewProposalAcceptedMessage(events.PROPOSAL_ACCEPTED)
			}
		} else if pDevice != nil && pDevice.Config.State == persistence.CONFIGSTATE_CONFIGURING {
			w.AddDeferredCommand(cmd)
			return true
		} else {
			// Nothing to do, the message will be deleted because the node is going away and will not be able to send messages.
			// At this point, the agbot has an outstanding agreement that has to timeout before it will try to contact this node again.
			glog.Warningf(logString(fmt.Sprintf("node is shutting down, deleting proposal %v message %v", p, exchangeMsg.MsgId)))
		}

		if deleteMessage {

			if err := w.deleteMessage(exchangeMsg); err != nil {
				glog.Errorf(logString(fmt.Sprintf("error deleting exchange message %v, error %v", exchangeMsg.MsgId, err)))
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
		}

	case *EdgeConfigCompleteCommand:
		if w.GetExchangeToken() == "" {
			glog.Warningf(logString(fmt.Sprintf("ignoring config complete, device not registered: %v", w.GetExchangeId())))
		} else {
			// Write registered services and pattern (if any) to the exchange node.
			if err := w.patchPattern(); err != nil {
				glog.Errorf(logString(fmt.Sprintf("error setting registered services and the pattern, error: %v", err)))
			}
			// Setting the node's key into its exchange object enables an agbot to send proposal messages to it. Until this is
			// set, the node will not receive any proposals.
			w.patchNodeKey()
		}

	case *NodePolicyChangedCommand:
		cmd, _ := command.(*NodePolicyChangedCommand)
		switch cmd.Msg.Event().Id {
		case events.UPDATE_POLICY:
			w.NodePolicyUpdated()
		case events.DELETED_POLICY:
			w.NodePolicyDeleted()
		}

	case *NodeChangeCommand:
		w.checkNodeChanges()

	case *NodePolicyChangeCommand:
		w.checkNodePolicyChanges()

	default:
		// Unexpected commands are not handled.
		return false
	}

	// Assume the command was handled.
	return true

}

func (w *AgreementWorker) handleDeviceRegistered(cmd *DeviceRegisteredCommand) {

	w.EC = worker.NewExchangeContext(fmt.Sprintf("%v/%v", cmd.Msg.Org(), cmd.Msg.DeviceId()), cmd.Msg.Token(), w.Config.Edge.ExchangeURL, w.Config.GetCSSURL(), w.Config.Collaborators.HTTPClientFactory)
	w.devicePattern = cmd.Msg.Pattern()
	w.limitedRetryEC = newLimitedRetryExchangeContext(w.EC)

	// There is no need for agreement tracking on a node that is using patterns.
	if cmd.Msg.Pattern() != "" {
		w.pm.SetNoAgreementTracking()
	}

	if len(w.producerPH) == 0 {
		// Establish agreement protocol handlers
		for _, protocolName := range policy.AllAgreementProtocols() {
			pph := producer.CreateProducerPH(protocolName, w.BaseWorker.Manager.Config, w.db, w.pm, w)
			pph.Initialize()
			w.producerPH[protocolName] = pph
		}
	}

	// Do a quick sync to clean out old agreements in the exchange.
	if err := w.syncOnInit(); err != nil {
		glog.Errorf(logString(fmt.Sprintf("error during sync up of agreements, error: %v", err)))
	}
}

// This function is only called when anax device side initializes. The agbot has it's own initialization checking.
// This function is responsible for reconciling the agreements in our local DB with the agreements recorded in the exchange
// and the blockchain, as well as looking for agreements that need to change based on changes to policy files. This function
// handles agreements that exist in the exchange for which we have no DB records, it handles DB records for which
// the state in the exchange is missing, and it handles agreements whose state has changed in the blockchain.
func (w *AgreementWorker) syncOnInit() error {

	glog.V(3).Infof(logString("beginning sync up."))

	if nodePolicy, err := persistence.FindNodePolicy(w.db); err != nil {
		return errors.New(logString(fmt.Sprintf("unable to read node policy from the local database. %v", err)))
	} else if nodePolicy != nil {
		// add the node policy to the policy manager
		newPolicy, err := policy.GenPolicyFromExternalPolicy(nodePolicy, policy.MakeExternalPolicyHeaderName(w.GetExchangeId()))
		if err != nil {
			return errors.New(logString(fmt.Sprintf("Failed to convert node policy to policy file format: %v", err)))
		}
		w.pm.UpdatePolicy(exchange.GetOrg(w.GetExchangeId()), newPolicy)
	}

	// check if this is the pattern change case
	new_pattern, err := persistence.FindSavedNodeExchPattern(w.db)
	if err != nil {
		return errors.New(logString(fmt.Sprintf("received error reading node exchange pattern from local database: %v", err)))
	} else if new_pattern != "" {
		// send a message to re-reg the node wiht the new pattern
		glog.V(3).Infof(logString(fmt.Sprintf("the device is brought up to handle pattern change. THe new pattern is %v.", new_pattern)))
		w.Messages() <- events.NewNodePatternMessage(events.NODE_PATTERN_CHANGE_REREG, new_pattern)
	}

	// Reconcile the set of agreements recorded in the exchange for this device with the agreements in the local DB.
	// First get all the agreements for this device from the exchange.

	exchangeDeviceAgreements, err := w.getAllAgreements()
	if err != nil {
		return errors.New(logString(fmt.Sprintf("encountered error getting device agreement list from exchange, error %v", err)))
	} else {

		// Loop through each agreement in the exchange and search for that agreement in our DB. If it should not
		// be in the exchange, then we have to delete it from the exchange because its presence in the exchange
		// prevents an agbot from making an agreement with our device. It is posible to have a DB record for
		// an agreement that is not yet recorded in the exchange (this case is handled later in this function),
		// but the reverse should not occur normally. Agreements in the exchange must have a record on our local DB.
		for exchangeAg, _ := range exchangeDeviceAgreements {
			if agreements, err := persistence.FindEstablishedAgreementsAllProtocols(w.db, policy.AllAgreementProtocols(), []persistence.EAFilter{persistence.IdEAFilter(exchangeAg), persistence.UnarchivedEAFilter()}); err != nil {
				glog.Errorf(logString(fmt.Sprintf("error searching for agreement %v from exchange agreements. %v", exchangeAg, err)))
			} else if len(agreements) == 0 {
				glog.V(3).Infof(logString(fmt.Sprintf("found agreement %v in the exchange that is not in our DB.", exchangeAg)))
				// Delete the agreement from the exchange.
				if err := deleteProducerAgreement(w.GetHTTPFactory().NewHTTPClient(nil), w.GetExchangeURL(), w.GetExchangeId(), w.GetExchangeToken(), exchangeAg); err != nil {
					glog.Errorf(logString(fmt.Sprintf("error deleting agreement %v in exchange: %v", exchangeAg, err)))
				}
			}
		}

	}

	// Now perform the reverse set of checks, looping through our database and checking each record for accuracy with the exchange
	// and the blockchain.
	if agreements, err := persistence.FindEstablishedAgreementsAllProtocols(w.db, policy.AllAgreementProtocols(), []persistence.EAFilter{persistence.UnarchivedEAFilter()}); err == nil {

		neededBCInstances := make(map[string]map[string]map[string]bool)

		// If there are agreemens in the database then we will assume that the device is already registered
		for _, ag := range agreements {

			// Make a list of all blockchain instances in use by agreements in our DB so that we can make sure there is a
			// blockchain client running for each instance.
			bcType, bcName, bcOrg := w.producerPH[ag.AgreementProtocol].GetKnownBlockchain(&ag)

			if len(neededBCInstances[bcOrg]) == 0 {
				neededBCInstances[bcOrg] = make(map[string]map[string]bool)
			}
			if len(neededBCInstances[bcOrg][bcType]) == 0 {
				neededBCInstances[bcOrg][bcType] = make(map[string]bool)
			}
			neededBCInstances[bcOrg][bcType][bcName] = true

			// If there is an active agreement that is marked as terminated, then anax was restarted in the middle of
			// termination processing, and therefore we dont know how far it got. Initiate a cancel again to clean it up.
			if ag.AgreementTerminatedTime != 0 {
				reason := uint(ag.TerminatedReason)
				if _, err := persistence.AgreementStateForceTerminated(w.db, ag.CurrentAgreementId, ag.AgreementProtocol); err != nil {
					glog.Errorf(logString(fmt.Sprintf("unable to set force termination for agreement %v, error %v", ag.CurrentAgreementId, err)))
				}
				w.Messages() <- events.NewInitAgreementCancelationMessage(events.AGREEMENT_ENDED, reason, ag.AgreementProtocol, ag.CurrentAgreementId, ag.GetDeploymentConfig())

				// If the agreement's protocol requires that it is recorded externally in some way, verify that it is present there (e.g. a blockchain).
				// Make sure the external state agrees with our local DB state for this agreement. If not, then we might need to cancel the agreement.
				// Anax could have been down for a long time (or inoperable), and the external state may have changed.
			} else if ok, err := w.verifyAgreement(&ag, w.producerPH[ag.AgreementProtocol], bcType, bcName, bcOrg); err != nil {
				return errors.New(logString(fmt.Sprintf("unable to check for agreement %v in blockchain, error %v", ag.CurrentAgreementId, err)))
			} else if !ok {
				eventlog.LogAgreementEvent(
					w.db,
					persistence.SEVERITY_INFO,
					persistence.NewMessageMeta(EL_AG_NODE_CANNOT_VERIFY_AG, ag.CurrentAgreementId),
					persistence.EC_CANCEL_AGREEMENT,
					ag)
				w.Messages() <- events.NewInitAgreementCancelationMessage(events.AGREEMENT_ENDED, w.producerPH[ag.AgreementProtocol].GetTerminationCode(producer.TERM_REASON_AGBOT_REQUESTED), ag.AgreementProtocol, ag.CurrentAgreementId, ag.GetDeploymentConfig())

				// If the agreement has been started then we just need to make sure that the policy manager's agreement counts
				// are correct. Even for already timedout agreements, the governance process will cleanup old and outdated agreements,
				// so we don't need to do anything here.

			} else if proposal, err := w.producerPH[ag.AgreementProtocol].AgreementProtocolHandler("", "", "").DemarshalProposal(ag.Proposal); err != nil {
				glog.Errorf(logString(fmt.Sprintf("unable to demarshal proposal for agreement %v, error %v", ag.CurrentAgreementId, err)))
			} else if pol, err := policy.DemarshalPolicy(proposal.ProducerPolicy()); err != nil {
				glog.Errorf(logString(fmt.Sprintf("unable to demarshal policy for agreement %v, error %v", ag.CurrentAgreementId, err)))

			} else if policies, err := w.pm.GetPolicyList(exchange.GetOrg(w.GetExchangeId()), pol); err != nil {
				glog.Errorf(logString(fmt.Sprintf("unable to get policy list for producer policy in agreement %v, error: %v", ag.CurrentAgreementId, err)))
				w.Messages() <- events.NewInitAgreementCancelationMessage(events.AGREEMENT_ENDED, w.producerPH[ag.AgreementProtocol].GetTerminationCode(producer.TERM_REASON_POLICY_CHANGED), ag.AgreementProtocol, ag.CurrentAgreementId, ag.GetDeploymentConfig())

			} else if mergedPolicy, err := w.pm.MergeAllProducers(&policies, pol); err != nil {
				glog.Errorf(logString(fmt.Sprintf("unable to merge producer policies for agreement %v, error: %v", ag.CurrentAgreementId, err)))
				w.Messages() <- events.NewInitAgreementCancelationMessage(events.AGREEMENT_ENDED, w.producerPH[ag.AgreementProtocol].GetTerminationCode(producer.TERM_REASON_POLICY_CHANGED), ag.AgreementProtocol, ag.CurrentAgreementId, ag.GetDeploymentConfig())

			} else if _, err := policy.Are_Compatible_Producers(mergedPolicy, pol, uint64(pol.DataVerify.Interval)); err != nil {
				glog.Errorf(logString(fmt.Sprintf("unable to verify merged policy %v and %v for agreement %v, error: %v", mergedPolicy, pol, ag.CurrentAgreementId, err)))
				w.Messages() <- events.NewInitAgreementCancelationMessage(events.AGREEMENT_ENDED, w.producerPH[ag.AgreementProtocol].GetTerminationCode(producer.TERM_REASON_POLICY_CHANGED), ag.AgreementProtocol, ag.CurrentAgreementId, ag.GetDeploymentConfig())

			} else if err := w.pm.AttemptingAgreement(policies, ag.CurrentAgreementId, exchange.GetOrg(w.GetExchangeId())); err != nil {
				glog.Errorf(logString(fmt.Sprintf("cannot update agreement count for %v, error: %v", ag.CurrentAgreementId, err)))
			} else if err := w.pm.FinalAgreement(policies, ag.CurrentAgreementId, exchange.GetOrg(w.GetExchangeId())); err != nil {
				glog.Errorf(logString(fmt.Sprintf("cannot update agreement count for %v, error: %v", ag.CurrentAgreementId, err)))

				// There is a small window where an agreement might not have been recorded in the exchange. Let's just make sure.
			} else if ag.AgreementAcceptedTime != 0 && ag.AgreementTerminatedTime == 0 {

				if _, there := exchangeDeviceAgreements[ag.CurrentAgreementId]; !there {
					glog.Warningf(logString(fmt.Sprintf("agreement %v missing from exchange, adding it back in.", ag.CurrentAgreementId)))
					if cpol, err := policy.DemarshalPolicy(proposal.TsAndCs()); err != nil {
						glog.Errorf(logString(fmt.Sprintf("unable to demarshal consumer policy for agreement %v, error %v", ag.CurrentAgreementId, err)))
					} else {
						state := ""
						if ag.AgreementFinalizedTime != 0 {
							state = "Finalized Agreement"
						} else if ag.AgreementAcceptedTime != 0 {
							state = "Agree to proposal"
						} else {
							state = "unknown"
						}
						if err := w.recordAgreementState(ag.CurrentAgreementId, cpol, state); err != nil {
							glog.Errorf(logString(fmt.Sprintf("cannot record agreement %v state %v, error: %v", ag.CurrentAgreementId, state, err)))
						}
					}
				}
				glog.V(3).Infof(logString(fmt.Sprintf("added agreement %v to policy agreement counter.", ag.CurrentAgreementId)))
			}

		}

		// Fire off start requests for each BC client that we need running. The blockchain worker and the container worker will tolerate
		// a start request for containers that are already running.
		for org, typeMap := range neededBCInstances {
			for typeName, instMap := range typeMap {
				for instName, _ := range instMap {
					w.Messages() <- events.NewNewBCContainerMessage(events.NEW_BC_CLIENT, typeName, instName, org, w.GetExchangeURL(), w.GetExchangeId(), w.GetExchangeToken())
				}
			}
		}

	} else {
		return errors.New(logString(fmt.Sprintf("error searching database: %v", err)))
	}

	glog.V(3).Infof(logString("sync up completed normally."))
	return nil

}

// This function verifies that an agreement is present in the blockchain. An agreement might not be present for a variety of reasons,
// some of which are legitimate. The purpose of this routine is to figure out whether or not an agreement cancellation
// has occurred. It returns false if the agreement needs to be cancelled, or if there was an error.
func (w *AgreementWorker) verifyAgreement(ag *persistence.EstablishedAgreement, pph producer.ProducerProtocolHandler, bcType string, bcName string, bcOrg string) (bool, error) {

	// Agreements that havent been accepted yet by the device will not be in any external store so it's ok if they aren't there,
	// so return true.
	if ag.AgreementAcceptedTime == 0 {
		return true, nil
	} else if !pph.IsBlockchainClientAvailable(bcType, bcName, bcOrg) || !pph.IsAgreementVerifiable(ag) {
		glog.Warningf(logString(fmt.Sprintf("for %v unable to verify agreement, agreement protocol handler is not ready", ag.CurrentAgreementId)))
		return true, nil
	}

	// Check to see if the agreement is in an external store.
	if pph.AgreementProtocolHandler(bcType, bcName, bcOrg) == nil {
		glog.Errorf(logString(fmt.Sprintf("for %v unable to verify agreement, agreement protocol handler is not ready", ag.CurrentAgreementId)))
	} else if recorded, err := pph.VerifyAgreement(ag); err != nil {
		return false, errors.New(logString(fmt.Sprintf("encountered error verifying agreement %v, error %v", ag.CurrentAgreementId, err)))
	} else {
		if !recorded {
			// A finalized agreement should be in the external store.
			if ag.AgreementFinalizedTime != 0 && ag.AgreementTerminatedTime == 0 {
				glog.V(3).Infof(logString(fmt.Sprintf("agreement %v is not known externally, cancelling.", ag.CurrentAgreementId)))
				return false, nil
			}
		}
	}
	return true, nil

}

func (w *AgreementWorker) getAllAgreements() (map[string]exchange.DeviceAgreement, error) {

	var exchangeDeviceAgreements map[string]exchange.DeviceAgreement
	var resp interface{}
	resp = new(exchange.AllDeviceAgreementsResponse)

	targetURL := w.GetExchangeURL() + "orgs/" + exchange.GetOrg(w.GetExchangeId()) + "/nodes/" + exchange.GetId(w.GetExchangeId()) + "/agreements"
	for {
		if err, tpErr := exchange.InvokeExchange(w.GetHTTPFactory().NewHTTPClient(nil), "GET", targetURL, w.GetExchangeId(), w.GetExchangeToken(), nil, &resp); err != nil {
			glog.Errorf(err.Error())
			return exchangeDeviceAgreements, err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			exchangeDeviceAgreements = resp.(*exchange.AllDeviceAgreementsResponse).Agreements
			glog.V(5).Infof(logString(fmt.Sprintf("found agreements %v in the exchange.", exchangeDeviceAgreements)))
			return exchangeDeviceAgreements, nil
		}
	}

}

// ===============================================================================================
// Utility functions
//

// Update the registeredServices and Pattern for the node.
func (w *AgreementWorker) registerNode(ms *[]exchange.Microservice) error {

	pdr := exchange.PatchDeviceRequest{}
	if ms != nil {
		pdr.RegisteredServices = ms
	} else {
		tmp := make([]exchange.Microservice, 0)
		pdr.RegisteredServices = &tmp
	}

	glog.V(3).Infof(logString(fmt.Sprintf("Registering services and pattern: %v.", pdr.ShortString())))

	patchDevice := exchange.GetHTTPPatchDeviceHandler(w)
	if err := patchDevice(w.GetExchangeId(), w.GetExchangeToken(), &pdr); err != nil {
		return err
	} else {
		glog.V(3).Infof(logString(fmt.Sprintf("advertised policies for device %v in exchange.", w.GetExchangeId())))
	}

	return nil
}

// Update the Pattern for the node.
func (w *AgreementWorker) patchPattern() error {

	dev, err := persistence.FindExchangeDevice(w.db)
	if err != nil {
		return errors.New(fmt.Sprintf("received error getting device name: %v", err))
	} else if dev == nil {
		return errors.New("could not get device name because no device was registered yet.")
	}

	pdr := exchange.PatchDeviceRequest{}
	if dev.Pattern != "" {
		tmpPattern := dev.Pattern
		pdr.Pattern = &tmpPattern
		if err := exchange.GetHTTPPatchDeviceHandler(w)(w.GetExchangeId(), w.GetExchangeToken(), &pdr); err != nil {
			return err
		} else {
			glog.V(3).Infof(logString(fmt.Sprintf("updated the pattern to %v for device %v in exchange.", dev.Pattern, w.GetExchangeId())))
		}
	}

	return nil
}

func (w *AgreementWorker) patchNodeKey() error {

	pdr := exchange.CreatePatchDeviceKey()

	var resp interface{}
	resp = new(exchange.PutDeviceResponse)
	targetURL := w.GetExchangeURL() + "orgs/" + exchange.GetOrg(w.GetExchangeId()) + "/nodes/" + exchange.GetId(w.GetExchangeId())

	glog.V(3).Infof(logString(fmt.Sprintf("patching messaging key to node entry: %v at %v", pdr.ShortString(), targetURL)))

	for {
		if err, tpErr := exchange.InvokeExchange(w.GetHTTPFactory().NewHTTPClient(nil), "PATCH", targetURL, w.GetExchangeId(), w.GetExchangeToken(), pdr, &resp); err != nil {
			return err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(3).Infof(logString(fmt.Sprintf("patched node key for device %v in exchange: %v", w.GetExchangeId(), resp)))
			return nil
		}
	}
}

func (w *AgreementWorker) advertiseAllPolicies() error {

	var pType, pValue, pCompare string

	// Advertise the microservices that this device is offering
	policies := w.pm.GetAllPolicies(exchange.GetOrg(w.GetExchangeId()))

	glog.V(5).Infof(logString(fmt.Sprintf("current policies: %v", policies)))

	if len(policies) > 0 {
		ms := make([]exchange.Microservice, 0, 10)
		for _, p := range policies {
			newMS := new(exchange.Microservice)
			// skip the node policy which does not have APISpecs
			if len(p.APISpecs) == 0 {
				continue
			} else {
				newMS.Url = cutil.FormOrgSpecUrl(p.APISpecs[0].SpecRef, p.APISpecs[0].Org)
			}

			// The version property needs special handling
			newProp := &exchange.MSProp{
				Name:     "version",
				Value:    p.APISpecs[0].Version,
				PropType: "version",
				Op:       "in",
			}
			newMS.Properties = append(newMS.Properties, *newProp)

			newMS.NumAgreements = p.MaxAgreements

			p.DataVerify.Obscure()

			if pBytes, err := json.Marshal(p); err != nil {
				return errors.New(fmt.Sprintf("AgreementWorker received error marshalling policy: %v", err))
			} else {
				newMS.Policy = string(pBytes)
			}

			if props, err := policy.RetrieveAllProperties(&p); err != nil {
				return errors.New(fmt.Sprintf("AgreementWorker received error calculating properties: %v", err))
			} else {
				for _, prop := range *props {
					switch prop.Value.(type) {
					case string:
						pType = "string"
						pValue = prop.Value.(string)
						pCompare = "in"
					case int:
						pType = "int"
						pValue = strconv.Itoa(prop.Value.(int))
						pCompare = ">="
					case bool:
						pType = "boolean"
						pValue = strconv.FormatBool(prop.Value.(bool))
						pCompare = "="
					case []string:
						pType = "list"
						pValue = exchange.ConvertToString(prop.Value.([]string))
						pCompare = "in"
					default:
						return errors.New(fmt.Sprintf("AgreementWorker encountered unsupported property type: %v", reflect.TypeOf(prop.Value).String()))
					}
					// Now put the property together
					newProp := &exchange.MSProp{
						Name:     prop.Name,
						Value:    pValue,
						PropType: pType,
						Op:       pCompare,
					}
					newMS.Properties = append(newMS.Properties, *newProp)
				}
			}

			ms = append(ms, *newMS)

		}

		// Register the node's microservices and policies in the exchange so that the agbot can see
		// what's available on the node. However, the node's messaging key is published later, after the
		// configstate API is called to complete configuration on the node. It is the presence of this key
		// in the exchange that tells the agbot it can make an agreement with this device.
		if err := w.registerNode(&ms); err != nil {
			return err
		} else if err := w.patchPattern(); err != nil {
			return err
		}

	}

	return nil
}

func (w *AgreementWorker) recordAgreementState(agreementId string, pol *policy.Policy, state string) error {

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
	if w.devicePattern != "" {
		workload.Org = exchange.GetOrg(w.GetExchangeId())
		workload.Pattern = w.devicePattern
		workload.URL = cutil.FormOrgSpecUrl(pol.Workloads[0].WorkloadURL, pol.Workloads[0].Org) // This is always 1 workload array element
	}

	// Configure the input object based on the service model or on the older workload model.
	as.State = state
	as.Services = services
	as.AgreementService = workload

	var resp interface{}
	resp = new(exchange.PostDeviceResponse)
	targetURL := w.GetExchangeURL() + "orgs/" + exchange.GetOrg(w.GetExchangeId()) + "/nodes/" + exchange.GetId(w.GetExchangeId()) + "/agreements/" + agreementId
	for {
		if err, tpErr := exchange.InvokeExchange(w.GetHTTPFactory().NewHTTPClient(nil), "PUT", targetURL, w.GetExchangeId(), w.GetExchangeToken(), as, &resp); err != nil {
			glog.Errorf(err.Error())
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

func deleteProducerAgreement(httpClient *http.Client, url string, deviceId string, token string, agreementId string) error {

	glog.V(5).Infof(logString(fmt.Sprintf("deleting agreement %v in exchange", agreementId)))

	var resp interface{}
	resp = new(exchange.PostDeviceResponse)
	targetURL := url + "orgs/" + exchange.GetOrg(deviceId) + "/nodes/" + exchange.GetId(deviceId) + "/agreements/" + agreementId
	for {
		if err, tpErr := exchange.InvokeExchange(httpClient, "DELETE", targetURL, deviceId, token, nil, &resp); err != nil {
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

func (w *AgreementWorker) deleteMessage(msg *exchange.DeviceMessage) error {
	var resp interface{}
	resp = new(exchange.PostDeviceResponse)
	targetURL := w.GetExchangeURL() + "orgs/" + exchange.GetOrg(w.GetExchangeId()) + "/nodes/" + exchange.GetId(w.GetExchangeId()) + "/msgs/" + strconv.Itoa(msg.MsgId)
	for {
		if err, tpErr := exchange.InvokeExchange(w.GetHTTPFactory().NewHTTPClient(nil), "DELETE", targetURL, w.GetExchangeId(), w.GetExchangeToken(), nil, &resp); err != nil {
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

func (w *AgreementWorker) messageInExchange(msgId int) (bool, error) {
	var resp interface{}
	resp = new(exchange.GetDeviceMessageResponse)
	targetURL := w.GetExchangeURL() + "orgs/" + exchange.GetOrg(w.GetExchangeId()) + "/nodes/" + exchange.GetId(w.GetExchangeId()) + "/msgs/" + strconv.Itoa(msgId)
	for {
		if err, tpErr := exchange.InvokeExchange(w.GetHTTPFactory().NewHTTPClient(nil), "GET", targetURL, w.GetExchangeId(), w.GetExchangeToken(), nil, &resp); err != nil {
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
	return fmt.Sprintf("AgreementWorker %v", v)
}
