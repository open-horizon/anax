package agreementbot

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/abstractprotocol"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/worker"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// must be safely-constructed!!
type AgreementBotWorker struct {
	worker.Worker  // embedded field
	db             *bolt.DB
	httpClient     *http.Client // a shared HTTP client instance for this worker
	agbotId        string
	token          string
	pm             *policy.PolicyManager
	consumerPH     map[string]ConsumerProtocolHandler
	ready          bool
	PatternManager *PatternManager
}

func NewAgreementBotWorker(cfg *config.HorizonConfig, db *bolt.DB) *AgreementBotWorker {
	messages := make(chan events.Message, 100) // The channel for outbound messages to the anax wide bus
	commands := make(chan worker.Command, 100) // The channel for commands into the agreement bot worker

	worker := &AgreementBotWorker{
		Worker: worker.Worker{
			Manager: worker.Manager{
				Config:   cfg,
				Messages: messages,
			},

			Commands: commands,
		},

		db:             db,
		httpClient:     cfg.Collaborators.HTTPClientFactory.NewHTTPClient(nil),
		agbotId:        cfg.AgreementBot.ExchangeId,
		token:          cfg.AgreementBot.ExchangeToken,
		consumerPH:     make(map[string]ConsumerProtocolHandler),
		ready:          false,
		PatternManager: NewPatternManager(),
	}

	glog.Info("Starting AgreementBot worker")
	worker.start()
	return worker
}

func (w *AgreementBotWorker) Messages() chan events.Message {
	return w.Worker.Manager.Messages
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

	default: //nothing

	}

	return
}

func (w *AgreementBotWorker) start() {

	go func() {

		glog.Info("AgreementBot worker initializing")

		// If there is no Agbot config, we will terminate
		if w.Config.AgreementBot == (config.AGConfig{}) {
			glog.Errorf("AgreementBotWorker terminating, no AgreementBot config.")
			return
		} else if w.db == nil {
			glog.Errorf("AgreementBotWorker terminating, no AgreementBot database configured.")
			return
		}

		// Make sure the policy directory is in place
		if err := os.MkdirAll(w.Worker.Manager.Config.AgreementBot.PolicyPath, 0644); err != nil {
			glog.Errorf("AgreementBotWorker cannot create agreement bot policy file path %v, terminating.", w.Worker.Manager.Config.AgreementBot.PolicyPath)
			return
		}

		// Query the exchange for patterns that this agbot is supposed to serve and generate a policy for each one.
		if err := w.GeneratePolicyFromPatterns(0); err != nil {
			glog.Errorf("AgreementBotWorker cannot read patterns from the exchange, error %v, terminating.", err)
			return
		}

		// Give the policy manager a chance to read in all the policies. The agbot worker will not proceed past this point
		// until it has some policies to work with.
		for {
			if policyManager, err := policy.Initialize(w.Worker.Manager.Config.AgreementBot.PolicyPath, w.workloadResolver, false); err != nil {
				glog.Errorf("AgreementBotWorker unable to initialize policy manager, error: %v", err)
			} else if policyManager.NumberPolicies() != 0 {
				w.pm = policyManager
				break
			}
			glog.V(3).Infof("AgreementBotWorker waiting for policies to appear")
			time.Sleep(time.Duration(1) * time.Minute)
		}

		glog.Info("AgreementBot worker started")

		// Make sure that our public key is registered in the exchange so that other parties
		// can send us messages.
		if err := w.registerPublicKey(); err != nil {
			glog.Errorf("AgreementBotWorker unable to register public key, error: %v", err)
			return
		}

		// For each agreement protocol in the current list of configured policies, startup a processor
		// to initiate the protocol.
		for protocolName, _ := range w.pm.GetAllAgreementProtocols() {
			if policy.SupportedAgreementProtocol(protocolName) {
				cph := CreateConsumerPH(protocolName, w.Worker.Manager.Config, w.db, w.pm, w.Worker.Manager.Messages)
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
			return
		}

		// The agbot worker is now ready to handle incoming messages
		w.ready = true

		// Begin heartbeating with the exchange.
		targetURL := w.Manager.Config.AgreementBot.ExchangeURL + "orgs/" + exchange.GetOrg(w.agbotId) + "/agbots/" + exchange.GetId(w.agbotId) + "/heartbeat"
		go exchange.Heartbeat(w.Config.Collaborators.HTTPClientFactory.NewHTTPClient(nil), targetURL, w.agbotId, w.token, w.Worker.Manager.Config.AgreementBot.ExchangeHeartbeat)

		// Start the governance routines.
		go w.GovernAgreements()
		go w.GovernArchivedAgreements()
		go w.GovernBlockchainNeeds()
		if w.Config.AgreementBot.CheckUpdatedPolicyS != 0 {
			go policy.PolicyFileChangeWatcher(w.Config.AgreementBot.PolicyPath, w.pm.WatcherContent, w.changedPolicy, w.deletedPolicy, w.errorPolicy, w.workloadResolver, w.Config.AgreementBot.CheckUpdatedPolicyS)
			go w.GeneratePolicyFromPatterns(w.Config.AgreementBot.CheckUpdatedPolicyS)
		}

		// Enter the command processing loop. Initialization is complete so wait for commands to
		// perform. Commands are created as the result of events that are triggered elsewhere
		// in the system. This function also wakes up periodically and looks for messages on
		// its exchnage message queue.

		nonBlockDuration := w.Config.AgreementBot.NewContractIntervalS
		for {
			glog.V(2).Infof("AgreementBotWorker non-blocking for commands")
			select {
			case command := <-w.Commands:
				glog.V(2).Infof("AgreementBotWorker received command: %v", command)

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
								cph := CreateConsumerPH(agp.Name, w.Worker.Manager.Config, w.db, w.pm, w.Worker.Manager.Messages)
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

				default:
					glog.Errorf("AgreementBotWorker Unknown command (%T): %v", command, command)
				}

				glog.V(5).Infof("AgreementBotWorker handled command %v", command)

				glog.V(4).Infof("AgreementBotWorker queueing deferred commands")
				for _, cph := range w.consumerPH {
					cph.HandleDeferredCommands()
				}
				glog.V(4).Infof("AgreementBotWorker done queueing deferred commands")

			case <-time.After(time.Duration(nonBlockDuration) * time.Second):
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
							DeleteMessage(msg.MsgId, w.agbotId, w.token, w.Config.AgreementBot.ExchangeURL, w.httpClient)
						} else {
							cmd := NewNewProtocolMessageCommand(protocolMessage, msg.MsgId, msg.DeviceId, msg.DevicePubKey)
							if !w.consumerPH[msgProtocol].AcceptCommand(cmd) {
								glog.Infof(fmt.Sprintf("AgreementBotWorker protocol handler for %v not accepting exchange messages, deleting msg.", msgProtocol))
								DeleteMessage(msg.MsgId, w.agbotId, w.token, w.Config.AgreementBot.ExchangeURL, w.httpClient)
							} else if err := w.consumerPH[msgProtocol].DispatchProtocolMessage(cmd, w.consumerPH[msgProtocol]); err != nil {
								DeleteMessage(msg.MsgId, w.agbotId, w.token, w.Config.AgreementBot.ExchangeURL, w.httpClient)
							}
						}
					}
				}
				glog.V(5).Infof(fmt.Sprintf("AgreementBotWorker done processing messages"))

				glog.V(4).Infof("AgreementBotWorker Polling Exchange.")
				w.findAndMakeAgreements()
				glog.V(4).Infof("AgreementBotWorker Done Polling Exchange.")

				glog.V(4).Infof("AgreementBotWorker queueing deferred commands")
				for _, cph := range w.consumerPH {
					cph.HandleDeferredCommands()
				}
				glog.V(4).Infof("AgreementBotWorker done queueing deferred commands")

			}
			runtime.Gosched()
		}
	}()

	glog.Info("AgreementBotWorker waiting for commands.")

}

// Search the exchange and make agreements with any device that is eligible based on the policies we have and
// agreement protocols that we support.
func (w *AgreementBotWorker) findAndMakeAgreements() {

	// Get a list of all the orgs we are serving
	allOrgs := w.pm.GetAllPolicyOrgs()

	for _, org := range allOrgs {
		// Get a copy of all policies in the policy manager so that we can safely iterate the list
		policies := w.pm.GetAllAvailablePolicies(org)
		for _, consumerPolicy := range policies {

			if devices, err := w.searchExchange(&consumerPolicy, org); err != nil {
				glog.Errorf("AgreementBotWorker received error searching for %v, error: %v", &consumerPolicy, err)
			} else {

				for _, dev := range *devices {

					glog.V(3).Infof("AgreementBotWorker picked up %v", dev.ShortString())
					glog.V(5).Infof("AgreementBotWorker picked up %v", dev)

					// Check for agreements already in progress with this device
					if found, err := w.alreadyMakingAgreementWith(&dev, &consumerPolicy); err != nil {
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

					// The only reason for no microservices in the device search result is because the search was pattern based.
					// In this case there will not be any policies from the producer side to work with. The agbot assumes that
					// device side anax will not allow microservice registration that is incompatible with the pattern.

					// If there are no microservices in the returned device then we cant do any of the
					// producer side policy merge and compatibility checks until we get the node's policies from the
					// exchange. It is preferable to NOT call the exchange on the main agbot thread. So, make an
					// agreement protocol choice based solely on the consumer side policy. Once the new agreement
					// attempt gets on a worker thread, then we can perform the policy checks and merges.
					producerPolicy := policy.Policy_Factory("empty")
					err := error(nil)
					if len(dev.Microservices) != 0 {

						// For every microservice required by the workload, deserialize the JSON policy blob into a policy object and
						// then merge them all together.
						if producerPolicy, err = w.MergeAllProducerPolicies(&dev); err != nil {
							glog.Errorf("AgreementBotWorker unable to merge microservice policies, error: %v", err)
							continue
						} else if producerPolicy == nil {
							glog.Errorf("AgreementBotWorker unable to create merged policy from producer %v", dev)
							continue
						}

						// Check to see if the device's merged policy is compatible with the consumer
						if err := policy.Are_Compatible(producerPolicy, &consumerPolicy); err != nil {
							glog.Errorf("AgreementBotWorker received error comparing %v and %v, error: %v", *producerPolicy, consumerPolicy, err)
							continue
						}

					}

					// Select a worker pool based on the agreement protocol that will be used.
					protocol := policy.Select_Protocol(producerPolicy, &consumerPolicy)
					cmd := NewMakeAgreementCommand(*producerPolicy, consumerPolicy, org, dev)

					bcType, bcName, bcOrg := producerPolicy.RequiresKnownBC(protocol)

					if _, ok := w.consumerPH[protocol]; !ok {
						glog.Errorf("AgreementBotWorker unable to find protocol handler for %v.", protocol)
					} else if bcType != "" && !w.consumerPH[protocol].IsBlockchainWritable(bcType, bcName, bcOrg) {
						// Get that blockchain running if it isn't up.
						glog.V(5).Infof("AgreementBotWorker skipping device id %v, requires blockchain %v %v %v that isnt ready yet.", dev.Id, bcType, bcName, bcOrg)
						w.Worker.Manager.Messages <- events.NewNewBCContainerMessage(events.NEW_BC_CLIENT, bcType, bcName, bcOrg, w.Manager.Config.AgreementBot.ExchangeURL, w.agbotId, w.token)
						continue
					} else if !w.consumerPH[protocol].AcceptCommand(cmd) {
						glog.Errorf("AgreementBotWorker protocol handler for %v not accepting new agreement commands.", protocol)
					} else {
						w.consumerPH[protocol].HandleMakeAgreement(cmd, w.consumerPH[protocol])
						glog.V(5).Infof("AgreementBoWorker queued agreement attempt for policy %v and protocol %v", consumerPolicy.Header.Name, protocol)
					}

				}

			}
		}
	}
}

// Check all agreement protocol buckets to see if there are any agreements with this device.
func (w *AgreementBotWorker) alreadyMakingAgreementWith(dev *exchange.SearchResultDevice, consumerPolicy *policy.Policy) (bool, error) {

	// Check to see if we're already doing something with this device
	pendingAgreementFilter := func() AFilter {
		return func(a Agreement) bool {
			return a.DeviceId == dev.Id && a.PolicyName == consumerPolicy.Header.Name && a.AgreementTimedout == 0
		}
	}

	// Search all agreement protocol buckets
	for _, agp := range policy.AllAgreementProtocols() {
		// Find all agreements that are in progress. They might be waiting for a reply or not yet finalized on blockchain.
		// TODO: To support more than 1 agreement (maxagreements > 1) with this device for this policy, we need to adjust this logic.
		if agreements, err := FindAgreements(w.db, []AFilter{UnarchivedAFilter(), pendingAgreementFilter()}, agp); err != nil {
			glog.Errorf("AgreementBotWorker received error trying to find pending agreements for protocol %v: %v", agp, err)
		} else if len(agreements) != 0 {
			return true, nil
		}
	}
	return false, nil

}

// Merge all the producer policies into 1 so that they can collectively be checked for compatibility against a consumer policy.
// The list of microservices in a device object that comes back in a search only includes the microservices that we
// searched for.
func (w *AgreementBotWorker) MergeAllProducerPolicies(dev *exchange.SearchResultDevice) (*policy.Policy, error) {

	var producerPolicy *policy.Policy

	for _, msDef := range dev.Microservices {
		tempPolicy := new(policy.Policy)
		if len(msDef.Policy) == 0 {
			return nil, errors.New(fmt.Sprintf("empty policy blob for %v, skipping this device.", msDef.Url))
		} else if err := json.Unmarshal([]byte(msDef.Policy), tempPolicy); err != nil {
			return nil, errors.New(fmt.Sprintf("error demarshalling policy blob %v, error: %v", msDef.Policy, err))
		} else if producerPolicy == nil {
			producerPolicy = tempPolicy
		} else if newPolicy, err := policy.Are_Compatible_Producers(producerPolicy, tempPolicy, w.Config.AgreementBot.NoDataIntervalS); err != nil {
			return nil, errors.New(fmt.Sprintf("error merging policies %v and %v, error: %v", producerPolicy, tempPolicy, err))
		} else {
			producerPolicy = newPolicy
		}
	}

	return producerPolicy, nil
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
	targetURL := w.Manager.Config.AgreementBot.ExchangeURL + "orgs/" + exchange.GetOrg(w.agbotId) + "/agbots/" + exchange.GetId(w.agbotId) + "/msgs"
	for {
		if err, tpErr := exchange.InvokeExchange(w.httpClient, "GET", targetURL, w.agbotId, w.token, nil, &resp); err != nil {
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
func RetrieveAllProperties(version string, arch string, pol *policy.Policy) (*policy.PropertyList, error) {
	pl := new(policy.PropertyList)

	for _, p := range pol.Properties {
		*pl = append(*pl, p)
	}

	if version != "" {
		*pl = append(*pl, policy.Property{Name: "version", Value: version})
	}
	*pl = append(*pl, policy.Property{Name: "arch", Value: arch})

	if len(pol.AgreementProtocols) != 0 {
		*pl = append(*pl, policy.Property{Name: "agreementProtocols", Value: pol.AgreementProtocols.As_String_Array()})
	}

	return pl, nil
}

func DeleteConsumerAgreement(httpClient *http.Client, url string, agbotId string, token string, agreementId string) error {

	logString := func(v interface{}) string {
		return fmt.Sprintf("AgreementBot Governance: %v", v)
	}

	glog.V(5).Infof(logString(fmt.Sprintf("deleting agreement %v in exchange", agreementId)))

	var resp interface{}
	resp = new(exchange.PostDeviceResponse)
	targetURL := url + "orgs/" + exchange.GetOrg(agbotId) + "/agbots/" + exchange.GetId(agbotId) + "/agreements/" + agreementId
	for {
		if err, tpErr := exchange.InvokeExchange(httpClient, "DELETE", targetURL, agbotId, token, nil, &resp); err != nil && !strings.Contains(err.Error(), "not found") {
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
// There are 2 ways to search the exchange; (a) by pattern and workload URL, or (b) by list of microservices.
// If the agbot is working with a policy file that was generated from a pattern, then it will do searches by
// pattern. If the agbot is working with a manually created policy file, then it will do searches by list of
// microservices.
func (w *AgreementBotWorker) searchExchange(pol *policy.Policy, searchOrg string) (*[]exchange.SearchResultDevice, error) {

	// If it is a pattern based policy, search by worload URL and pattern.
	if pol.PatternId != "" {

		// Setup the search request body
		ser := exchange.CreateSearchPatternRequest()
		ser.SecondsStale = w.Config.AgreementBot.ActiveDeviceTimeoutS
		ser.WorkloadURL = pol.Workloads[0].WorkloadURL

		// Invoke the exchange
		var resp interface{}
		resp = new(exchange.SearchExchangePatternResponse)
		targetURL := w.Worker.Manager.Config.AgreementBot.ExchangeURL + "orgs/" + searchOrg + "/patterns/" + exchange.GetId(pol.PatternId) + "/search"
		for {
			if err, tpErr := exchange.InvokeExchange(w.httpClient, "POST", targetURL, w.agbotId, w.token, ser, &resp); err != nil {
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

		// Search the exchange based on a list of microservices.
		// Collect the API specs to search over into a map so that duplicates are automatically removed.
		msMap := make(map[string]*exchange.Microservice)

		// For policy files that point to the exchange for workload details, we need to get all the referred to API specs
		// from all workloads and search for devices that can satisfy all the workloads in the policy file. If a device
		// can't satisfy all the workloads then workload rollback cant work so we shouldnt make an agreement with this
		// device.
		for _, workload := range pol.Workloads {
			if workload, err := exchange.GetWorkload(w.Config.Collaborators.HTTPClientFactory, workload.WorkloadURL, workload.Org, workload.Version, workload.Arch, w.Config.AgreementBot.ExchangeURL, w.agbotId, w.token); err != nil {
				return nil, errors.New(fmt.Sprintf("AgreementBotWorker received error retrieving workload definition for %v, error: %v", workload, err))
			} else {
				for _, apiSpec := range workload.APISpecs {
					if newMS, err := w.makeNewMSSearchElement(apiSpec.SpecRef, apiSpec.Org, "", apiSpec.Arch, pol); err != nil {
						return nil, err
					} else {
						msMap[apiSpec.SpecRef] = newMS
					}
				}
			}
		}

		// Convert the collected API specs into an array for the search request body
		desiredMS := make([]exchange.Microservice, 0, 10)
		for _, ms := range msMap {
			desiredMS = append(desiredMS, *ms)
		}

		// Setup the search request body
		ser := exchange.CreateSearchMSRequest()
		ser.SecondsStale = w.Config.AgreementBot.ActiveDeviceTimeoutS
		ser.DesiredMicroservices = desiredMS

		// Invoke the exchange
		var resp interface{}
		resp = new(exchange.SearchExchangeMSResponse)
		targetURL := w.Worker.Manager.Config.AgreementBot.ExchangeURL + "orgs/" + searchOrg + "/search/nodes"
		for {
			if err, tpErr := exchange.InvokeExchange(w.httpClient, "POST", targetURL, w.agbotId, w.token, ser, &resp); err != nil {
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
				glog.V(3).Infof("AgreementBotWorker found %v devices in exchange.", len(resp.(*exchange.SearchExchangeMSResponse).Devices))
				dev := resp.(*exchange.SearchExchangeMSResponse).Devices
				return &dev, nil
			}
		}
	}
}

func (w *AgreementBotWorker) makeNewMSSearchElement(specRef string, org string, version string, arch string, pol *policy.Policy) (*exchange.Microservice, error) {
	newMS := new(exchange.Microservice)
	newMS.Url = specRef
	newMS.NumAgreements = 1

	if props, err := RetrieveAllProperties(version, arch, pol); err != nil {
		return nil, errors.New(fmt.Sprintf("AgreementBotWorker received error calculating properties: %v", err))
	} else {
		for _, prop := range *props {
			if newProp, err := exchange.ConvertPropertyToExchangeFormat(&prop); err != nil {
				return nil, errors.New(fmt.Sprintf("AgreementBotWorker got error converting properties %v to exchange format: %v", prop, err))
			} else {
				newMS.Properties = append(newMS.Properties, *newProp)
			}
		}
	}
	return newMS, nil
}

func (w *AgreementBotWorker) syncOnInit() error {
	glog.V(3).Infof(AWlogString("beginning sync up."))

	// Search all agreement protocol buckets
	for _, agp := range policy.AllAgreementProtocols() {

		// Loop through our database and check each record for accuracy with the exchange and the blockchain
		if agreements, err := FindAgreements(w.db, []AFilter{UnarchivedAFilter()}, agp); err == nil {

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
						if err := DeleteConsumerAgreement(w.Config.Collaborators.HTTPClientFactory.NewHTTPClient(nil), w.Config.AgreementBot.ExchangeURL, w.agbotId, w.token, ag.CurrentAgreementId); err != nil {
							glog.Errorf(AWlogString(fmt.Sprintf("error deleting agreement %v in exchange: %v", ag.CurrentAgreementId, err)))
						}
						// Remove any workload usage records so that a new agreement will be made starting from the highest priority workload
						if err := DeleteWorkloadUsage(w.db, ag.DeviceId, ag.PolicyName); err != nil {
							glog.Warningf(AWlogString(fmt.Sprintf("error deleting workload usage for %v using policy %v, error: %v", ag.DeviceId, ag.PolicyName, err)))
						}
						// Indicate that the agreement is timed out
						if _, err := AgreementTimedout(w.db, ag.CurrentAgreementId, agp); err != nil {
							glog.Errorf(AWlogString(fmt.Sprintf("error marking agreement %v terminated: %v", ag.CurrentAgreementId, err)))
						}
						w.consumerPH[agp].HandleAgreementTimeout(NewAgreementTimeoutCommand(ag.CurrentAgreementId, ag.AgreementProtocol, w.consumerPH[agp].GetTerminationCode(TERM_REASON_POLICY_CHANGED)), w.consumerPH[agp])
					} else if err := w.pm.MatchesMine(ag.Org, pol); err != nil {
						glog.Warningf(AWlogString(fmt.Sprintf("agreement %v has a policy %v that has changed: %v", ag.CurrentAgreementId, pol.Header.Name, err)))

						// Remove any workload usage records (non-HA) or mark for pending upgrade (HA). There might not be a workload usage record
						// if the consumer policy does not specify the workload priority section.
						if wlUsage, err := FindSingleWorkloadUsageByDeviceAndPolicyName(w.db, ag.DeviceId, ag.PolicyName); err != nil {
							glog.Warningf(AWlogString(fmt.Sprintf("error retreiving workload usage for %v using policy %v, error: %v", ag.DeviceId, ag.PolicyName, err)))
						} else if wlUsage != nil && len(wlUsage.HAPartners) != 0 && wlUsage.PendingUpgradeTime != 0 {
							// Skip this agreement, it is part of an HA group where another member is upgrading
							continue
						} else if wlUsage != nil && len(wlUsage.HAPartners) != 0 && wlUsage.PendingUpgradeTime == 0 {
							for _, partnerId := range wlUsage.HAPartners {
								if _, err := UpdatePendingUpgrade(w.db, partnerId, ag.PolicyName); err != nil {
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
						targetURL := w.Worker.Manager.Config.AgreementBot.ExchangeURL + "orgs/" + exchange.GetOrg(w.agbotId) + "/agbots/" + exchange.GetId(w.agbotId) + "/agreements/" + ag.CurrentAgreementId

						if err, tpErr := exchange.InvokeExchange(w.httpClient, "GET", targetURL, w.agbotId, w.token, nil, &resp); err != nil || tpErr != nil {
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
								w.recordConsumerAgreementState(ag.CurrentAgreementId, pol, ag.Org, state)
							}
						}
						glog.V(3).Infof(AWlogString(fmt.Sprintf("added agreement %v to policy agreement counter.", ag.CurrentAgreementId)))
					}

					// This state should never occur, but could if there was an error along the way. It means that a DB record
					// was created for this agreement but the record was never updated with the creation time, which is supposed to occur
					// immediately following creation of the record. Further, if this were to occur, then the exchange should not have been
					// updated, so there is no reason to try to clean that up. Same is true for the workload usage records.
				} else if ag.AgreementInceptionTime != 0 && ag.AgreementCreationTime == 0 {
					if err := DeleteAgreement(w.db, ag.CurrentAgreementId, agp); err != nil {
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
						w.Messages() <- events.NewNewBCContainerMessage(events.NEW_BC_CLIENT, typeName, instName, org, w.Config.AgreementBot.ExchangeURL, w.agbotId, w.token)
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

func (w *AgreementBotWorker) cleanupAgreement(ag *Agreement) {
	// Update state in exchange
	if err := DeleteConsumerAgreement(w.Config.Collaborators.HTTPClientFactory.NewHTTPClient(nil), w.Config.AgreementBot.ExchangeURL, w.agbotId, w.token, ag.CurrentAgreementId); err != nil {
		glog.Errorf(AWlogString(fmt.Sprintf("error deleting agreement %v in exchange: %v", ag.CurrentAgreementId, err)))
	}

	// Delete this workload usage record so that a new agreement will be made starting from the highest priority workload
	if err := DeleteWorkloadUsage(w.db, ag.DeviceId, ag.PolicyName); err != nil {
		glog.Warningf(AWlogString(fmt.Sprintf("error deleting workload usage for %v using policy %v, error: %v", ag.DeviceId, ag.PolicyName, err)))
	}

	// Indicate that the agreement is timed out
	if _, err := AgreementTimedout(w.db, ag.CurrentAgreementId, ag.AgreementProtocol); err != nil {
		glog.Errorf(AWlogString(fmt.Sprintf("error marking agreement %v terminated: %v", ag.CurrentAgreementId, err)))
	}

	w.consumerPH[ag.AgreementProtocol].HandleAgreementTimeout(NewAgreementTimeoutCommand(ag.CurrentAgreementId, ag.AgreementProtocol, w.consumerPH[ag.AgreementProtocol].GetTerminationCode(TERM_REASON_POLICY_CHANGED)), w.consumerPH[ag.AgreementProtocol])
}

func (w *AgreementBotWorker) recordConsumerAgreementState(agreementId string, pol *policy.Policy, org string, state string) error {

	workload := pol.Workloads[0].WorkloadURL

	glog.V(5).Infof(AWlogString(fmt.Sprintf("setting agreement %v for workload %v state to %v", agreementId, workload, state)))

	as := new(exchange.PutAgbotAgreementState)
	as.Workload = exchange.WorkloadAgreement{
		Org:     exchange.GetOrg(pol.PatternId),
		Pattern: exchange.GetId(pol.PatternId),
		URL:     workload,
	}
	as.State = state
	var resp interface{}
	resp = new(exchange.PostDeviceResponse)
	targetURL := w.Config.AgreementBot.ExchangeURL + "orgs/" + exchange.GetOrg(w.agbotId) + "/agbots/" + exchange.GetId(w.agbotId) + "/agreements/" + agreementId
	for {
		if err, tpErr := exchange.InvokeExchange(w.httpClient, "PUT", targetURL, w.agbotId, w.token, &as, &resp); err != nil {
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
	targetURL := w.Config.AgreementBot.ExchangeURL + "orgs/" + exchange.GetOrg(w.agbotId) + "/agbots/" + exchange.GetId(w.agbotId)
	for {
		if err, tpErr := exchange.InvokeExchange(w.httpClient, "PATCH", targetURL, w.agbotId, w.token, &as, &resp); err != nil {
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

func (w *AgreementBotWorker) workloadResolver(wURL string, wOrg string, wVersion string, wArch string) (*policy.APISpecList, error) {

	// TODO: do we need a dedicated HTTP client instance here or can we use the shared one?
	asl, err := exchange.WorkloadResolver(w.Config.Collaborators.HTTPClientFactory, wURL, wOrg, wVersion, wArch, w.Config.AgreementBot.ExchangeURL, w.Config.AgreementBot.ExchangeId, w.Config.AgreementBot.ExchangeToken)
	if err != nil {
		glog.Errorf(AWlogString(fmt.Sprintf("unable to resolve workload, error %v", err)))
	}
	return asl, err
}

// Generate policy files based on pattern metadata in the exchange. If checkInterval is zero,
// this function will perform a single pass through the exchange metadata, and generate policy files.
// A non-zero value will cause this function to remain in a loop checking every checkInterval seconds
// for new metadata and updating the policy file(s) accordingly.
func (w *AgreementBotWorker) GeneratePolicyFromPatterns(checkInterval int) error {
	for {

		glog.V(5).Infof(AWlogString(fmt.Sprintf("scanning patterns for updates")))
		err := w.internalGeneratePolicyFromPatterns()

		// Terminate now if necessary
		if checkInterval == 0 {
			return err
		} else {
			if err != nil {
				glog.Errorf(AWlogString(fmt.Sprintf("unable to process patterns, error %v", err)))
			}
			time.Sleep(time.Duration(checkInterval) * time.Second)
		}
	}
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

		// Query exchange for all patterns in the org
		if exchangePatternMetadata, err := exchange.GetPatterns(w.Config.Collaborators.HTTPClientFactory, org, "", w.Config.AgreementBot.ExchangeURL, w.agbotId, w.token); err != nil {
			return errors.New(fmt.Sprintf("unable to get patterns for org %v, error %v", org, err))

			// Check for pattern metadata changes and update policy files accordingly
		} else if err := w.PatternManager.UpdatePatternPolicies(org, exchangePatternMetadata, w.Config.AgreementBot.PolicyPath); err != nil {
			return errors.New(fmt.Sprintf("unable to update policies for org %v, error %v", org, err))
		}
	}

	return nil

}

func (w *AgreementBotWorker) getAgbotPatterns() (map[string]exchange.ServedPattern, error) {

	var resp interface{}
	resp = new(exchange.GetAgbotsPatternsResponse)
	targetURL := w.Config.AgreementBot.ExchangeURL + "orgs/" + exchange.GetOrg(w.agbotId) + "/agbots/" + exchange.GetId(w.agbotId) + "/patterns"
	for {
		if err, tpErr := exchange.InvokeExchange(w.httpClient, "GET", targetURL, w.agbotId, w.token, nil, &resp); err != nil {
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

// ==========================================================================================================
// Utility functions

var AWlogString = func(v interface{}) string {
	return fmt.Sprintf("AgreementBotWorker %v", v)
}
