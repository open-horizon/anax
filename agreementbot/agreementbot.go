package agreementbot

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/citizenscientist"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/ethblockchain"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/worker"
	"math/rand"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

// must be safely-constructed!!
type AgreementBotWorker struct {
	worker.Worker   // embedded field
	db              *bolt.DB
	httpClient      *http.Client
	agbotId         string
	token           string
	protocols       map[string]bool
	bc              *ethblockchain.BaseContracts
	pm              *policy.PolicyManager
	pwcommands      map[string]chan worker.Command
	bcWritesEnabled bool
	ready           bool
}

func NewAgreementBotWorker(config *config.HorizonConfig, db *bolt.DB) *AgreementBotWorker {
	messages := make(chan events.Message, 100)   // The channel for outbound messages to the anax wide bus
	commands := make(chan worker.Command, 100)   // The channel for commands into the agreement bot worker
	pwcommands := make(map[string]chan worker.Command) // The map of channels for commands into the agreement protocol workers

	worker := &AgreementBotWorker{
		Worker: worker.Worker{
			Manager: worker.Manager{
				Config:   config,
				Messages: messages,
			},

			Commands: commands,
		},

		db:              db,
		httpClient:      &http.Client{},
		agbotId:         config.AgreementBot.ExchangeId,
		token:           config.AgreementBot.ExchangeToken,
		protocols:       make(map[string]bool),
		pwcommands:      pwcommands,
		bcWritesEnabled: false,
		ready:           false,
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
			w.bcWritesEnabled = true
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
				agCmd := NewAgreementTimeoutCommand(msg.AgreementId, msg.AgreementProtocol, uint(msg.Reason))
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

		// Tell the eth worker to start the ethereum client container
		if w.agbotId != "" && w.token != "" {
			w.Worker.Manager.Messages <- events.NewNewEthContainerMessage(events.NEW_ETH_CLIENT, w.Manager.Config.AgreementBot.ExchangeURL, w.agbotId, w.token)
		}

		// Hold the agbot functions until we have blockchain funding. If there are events occurring that
		// we need to react to, they will queue up on the command queue while we wait here.
		for {
			if w.bcWritesEnabled == false {
				time.Sleep(time.Duration(5) * time.Second)
				glog.V(3).Infof("AgreementBotWorker command processor waiting for funding")
			} else {
				break
			}
		}

		glog.Info("AgreementBot worker started")

		// Establish the go objects that are used to interact with the ethereum blockchain.
		// This code should probably be in the protocol library.
		acct, _ := ethblockchain.AccountId()
		dir, _ := ethblockchain.DirectoryAddress()
		if bc, err := ethblockchain.InitBaseContracts(acct, w.Worker.Manager.Config.AgreementBot.GethURL, dir); err != nil {
			glog.Errorf("AgreementBotWorker unable to initialize platform contracts, error: %v", err)
			return
		} else {
			w.bc = bc
		}

		// Make sure the policy directory is in place
		if err := os.MkdirAll(w.Worker.Manager.Config.AgreementBot.PolicyPath, 0644); err != nil {
			glog.Errorf("AgreementBotWorker cannot create agreement bot policy file path %v, terminating.", w.Worker.Manager.Config.AgreementBot.PolicyPath)
			return
		}

		// Give the policy manager a chance to read in all the policies. The agbot worker will not proceed past this point
		// until it has some policies to work with.
		for {
			if policyManager, err := policy.Initialize(w.Worker.Manager.Config.AgreementBot.PolicyPath); err != nil {
				glog.Errorf("AgreementBotWorker unable to initialize policy manager, error: %v", err)
			} else if policyManager.NumberPolicies() != 0 {
				w.pm = policyManager
				break
			}
			glog.V(3).Infof("AgreementBotWorker waiting for policies to appear")
			time.Sleep(time.Duration(1) * time.Minute)
		}

		// As soon as policies appear, the agbot worker function should start doing agbot work. That means
		// we need to make sure that our public key is registered in the exchange so that other parties
		// can send us messages.
		if err := w.registerPublicKey(); err != nil {
			glog.Errorf("AgreementBotWorker unable to register public key, error: %v", err)
			return
		}

		// For each agreement protocol in the current list of configured policies, startup a processor
		// to initiate the protocol.

		w.protocols = w.pm.GetAllAgreementProtocols()
		for protocolName, _ := range w.protocols {
			w.pwcommands[protocolName] = make(chan worker.Command, 100)
			go w.InitiateAgreementProtocolHandler(protocolName)
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
		targetURL := w.Manager.Config.AgreementBot.ExchangeURL + "agbots/" + w.agbotId + "/heartbeat"
		go exchange.Heartbeat(&http.Client{}, targetURL, w.agbotId, w.token, w.Worker.Manager.Config.AgreementBot.ExchangeHeartbeat)

		// Start the governance routines.
		go w.GovernAgreements()
		go w.GovernArchivedAgreements()
		if w.Config.AgreementBot.CheckUpdatedPolicyS != 0 {
			go policy.PolicyFileChangeWatcher(w.Config.AgreementBot.PolicyPath, w.pm.WatcherContent, w.changedPolicy, w.deletedPolicy, w.errorPolicy, w.Config.AgreementBot.CheckUpdatedPolicyS)
		}

		// Enter the command processing loop. Initialization is complete so wait for commands to
		// perform. Commands are created as the result of events that are triggered elsewhere
		// in the system. This function also wakes up periodically and looks for messages on
		// its exchnage message queue.

		nonBlockDuration := 10
		for {
			glog.V(2).Infof("AgreementBotWorker non-blocking for commands")
			select {
			case command := <-w.Commands:
				glog.V(2).Infof("AgreementBotWorker received command: %v", command)

				switch command.(type) {
				case *BlockchainEventCommand:
					cmd, _ := command.(*BlockchainEventCommand)
					// Put command on each protocol worker's command queue
					for _, ch := range w.pwcommands {
						ch <- cmd
					}

				case *PolicyChangedCommand:
					cmd := command.(*PolicyChangedCommand)

					if pol, err := policy.DemarshalPolicy(cmd.Msg.PolicyString()); err != nil {
						glog.Errorf(fmt.Sprintf("AgreementBotWorker error demarshalling change event policy %v, error: %v", cmd.Msg.PolicyString(), err))
					} else {
						protocolName := pol.AgreementProtocols[0].Name

						glog.V(5).Infof("AgreementBotWorker about to update policy in PM.")
						// Update the policy in the policy manager.
						w.pm.UpdatePolicy(pol)
						glog.V(5).Infof("AgreementBotWorker updated policy in PM.")

						// Update the protocol handler map and make sure there are workers available if the policy has a new protocol in it.
						if _, ok := w.pwcommands[protocolName]; !ok {
							glog.V(3).Infof("AgreementBotWorker creating worker pool for new agreement protocol %v", protocolName)
							w.pwcommands[protocolName] = make(chan worker.Command, 100)
							go w.InitiateAgreementProtocolHandler(protocolName)
						}

						// Queue the command to the correct protocol worker pool for further processing. In odd corner cases
						// the policy file might contain an unsupported protocol, so be defensive.
						if _, ok := w.pwcommands[protocolName]; ok {
							w.pwcommands[protocolName] <- cmd
						} else {
							glog.Errorf("AgreementBotWorker unable to queue up policy change command because the policy %v contains an unsupported agreement protocol %v", pol.Header.Name, protocolName)
						}

					}

				case *PolicyDeletedCommand:
					cmd := command.(*PolicyDeletedCommand)

					if pol, err := policy.DemarshalPolicy(cmd.Msg.PolicyString()); err != nil {
						glog.Errorf(fmt.Sprintf("AgreementBotWorker error demarshalling change event policy %v, error: %v", cmd.Msg.PolicyString(), err))
					} else {
						protocolName := pol.AgreementProtocols[0].Name

						glog.V(5).Infof("AgreementBotWorker about to delete policy from PM.")
						// Update the policy in the policy manager.
						w.pm.DeletePolicy(pol)
						glog.V(5).Infof("AgreementBotWorker deleted policy from PM.")

						// Queue the command to the correct protocol worker pool for further processing. The deleted policy
						// might not contain a supported protocol, so we need to check that first.
						if _, ok := w.pwcommands[protocolName]; ok {
							w.pwcommands[protocolName] <- cmd
						} else {
							glog.Errorf("AgreementBotWorker unable to queue up policy deleted command because the policy %v contains an unsupported agreement protocol %v", pol.Header.Name, protocolName)
						}
					}

				case *AgreementTimeoutCommand:
					cmd, _ := command.(*AgreementTimeoutCommand)
					// Put command on each protocol worker's command queue
					for _, ch := range w.pwcommands {
						ch <- cmd
					}

				default:
					glog.Errorf("AgreementBotWorker Unknown command (%T): %v", command, command)
				}

				glog.V(5).Infof("AgreementBotWorker handled command")

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
						} else {
							cmd := NewNewProtocolMessageCommand(protocolMessage, msg.MsgId, msg.DeviceId, msg.DevicePubKey)
							// Put command on each protocol worker's command queue
							for _, ch := range w.pwcommands {
								ch <- cmd
							}
						}
					}
				}

				glog.V(5).Infof(fmt.Sprintf("AgreementBotWorker done processing messages"))
			}
			runtime.Gosched()
		}
	}()

	glog.Info("AgreementBotWorker waiting for commands.")

}

func (w *AgreementBotWorker) changedPolicy(fileName string, pol *policy.Policy) {
	glog.V(3).Infof(fmt.Sprintf("AgreementBotWorker detected changed policy file %v containing %v", fileName, pol))
	if policyString, err := policy.MarshalPolicy(pol); err != nil {
		glog.Errorf(fmt.Sprintf("AgreementBotWorker error trying to marshal policy %v error: %v", pol, err))
	} else {
		w.Messages() <- events.NewPolicyChangedMessage(events.CHANGED_POLICY, fileName, pol.Header.Name, policyString)
	}
}

func (w *AgreementBotWorker) deletedPolicy(fileName string, pol *policy.Policy) {
	glog.V(3).Infof(fmt.Sprintf("AgreementBotWorker detected deleted policy file %v containing %v", fileName, pol))
	if policyString, err := policy.MarshalPolicy(pol); err != nil {
		glog.Errorf(fmt.Sprintf("AgreementBotWorker error trying to marshal policy %v error: %v", pol, err))
	} else {
		w.Messages() <- events.NewPolicyDeletedMessage(events.DELETED_POLICY, fileName, pol.Header.Name, policyString)
	}
}

func (w *AgreementBotWorker) errorPolicy(fileName string, err error) {
	glog.Errorf(fmt.Sprintf("AgreementBotWorker tried to read policy file %v, encountered error: %v", fileName, err))
}

func (w *AgreementBotWorker) getMessages() ([]exchange.AgbotMessage, error) {
	var resp interface{}
	resp = new(exchange.GetAgbotMessageResponse)
	targetURL := w.Manager.Config.AgreementBot.ExchangeURL + "agbots/" + w.agbotId + "/msgs"
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

// There is one of these running for each agreement protocol that we support
func (w *AgreementBotWorker) InitiateAgreementProtocolHandler(protocol string) {

	unarchived := []AFilter{UnarchivedAFilter()}

	if protocol == citizenscientist.PROTOCOL_NAME {

		// Set up random number gen. This is used to generate agreement id strings.
		random := rand.New(rand.NewSource(int64(time.Now().Nanosecond())))

		// Setup a lock to protect the agreement mutex map
		agreementLockMgr := NewAgreementLockManager()

		// Set up agreement worker pool based on the current technical config.
		work := make(chan CSAgreementWork)
		for ix := 0; ix < w.Worker.Manager.Config.AgreementBot.AgreementWorkers; ix++ {
			agw := NewCSAgreementWorker(w.pm, w.Worker.Manager.Config, w.db, agreementLockMgr)
			go agw.start(work, random, w.bc)
		}

		// Main function of the agreement processor. Constantly search through the list of available
		// devices to ensure that we are contracting with as many devices as possible.
		go func() {

			protocolHandler := citizenscientist.NewProtocolHandler(w.Config.AgreementBot.GethURL, w.pm)

			for {

				glog.V(5).Infof("AgreementBot about to select command (non-blocking).")
				select {
				case command := <-w.pwcommands[protocol]:
					switch command.(type) {
					case *NewProtocolMessageCommand:
						glog.V(5).Infof("AgreementBot received inbound exchange message.")
						cmd := command.(*NewProtocolMessageCommand)
						// Figure out what kind of message this is
						if _, err := protocolHandler.ValidateReply(string(cmd.Message)); err == nil {
							agreementWork := CSHandleReply{
								workType:     REPLY,
								Reply:        string(cmd.Message),
								SenderId:     cmd.From,
								SenderPubKey: cmd.PubKey,
								MessageId:    cmd.MessageId,
							}
							work <- agreementWork
							glog.V(5).Infof("AgreementBot queued reply message")
						} else if _, err := protocolHandler.ValidateDataReceivedAck(string(cmd.Message)); err == nil {
							agreementWork := CSHandleDataReceivedAck{
								workType:     DATARECEIVEDACK,
								Ack:          string(cmd.Message),
								SenderId:     cmd.From,
								SenderPubKey: cmd.PubKey,
								MessageId:    cmd.MessageId,
							}
							work <- agreementWork
							glog.V(5).Infof("AgreementBot queued data received ack message")
						} else {
							glog.Warningf(AWlogString(fmt.Sprintf("ignoring  message: %v", string(cmd.Message))))
						}

					case *AgreementTimeoutCommand:
						glog.V(5).Infof("AgreementBot received agreement cancellation.")
						cmd := command.(*AgreementTimeoutCommand)
						agreementWork := CSCancelAgreement{
							workType:    CANCEL,
							AgreementId: cmd.AgreementId,
							Protocol:    cmd.Protocol,
							Reason:      cmd.Reason,
						}
						work <- agreementWork
						glog.V(5).Infof("AgreementBot queued agreement cancellation")

					case *BlockchainEventCommand:
						glog.V(5).Infof("AgreementBot received blockchain event.")
						cmd, _ := command.(*BlockchainEventCommand)

						// Unmarshal the raw event
						if rawEvent, err := protocolHandler.DemarshalEvent(cmd.Msg.RawEvent()); err != nil {
							glog.Errorf("AgreementBotWorker unable to demarshal raw event %v, error: %v", cmd.Msg.RawEvent(), err)
						} else if !protocolHandler.AgreementCreated(rawEvent) && !protocolHandler.ProducerTermination(rawEvent) && !protocolHandler.ConsumerTermination(rawEvent) {
							glog.V(5).Infof(AWlogString(fmt.Sprintf("ignoring the blockchain event because it is not agreement creation or termination event.")))
						} else {
							agreementId := protocolHandler.GetAgreementId(rawEvent)

							if ag, err := FindSingleAgreementByAgreementId(w.db, agreementId, protocol, unarchived); err != nil {
								glog.Errorf(AWlogString(fmt.Sprintf("error querying agreement %v from database, error: %v", agreementId, err)))
							} else if ag == nil {
								glog.V(3).Infof(AWlogString(fmt.Sprintf("ignoring the blockchain event, no database record for for agreement %v with protocol %v.", agreementId, protocol)))

								// if the event is agreement recorded event
							} else if protocolHandler.AgreementCreated(rawEvent) {
								agreementWork := CSHandleBCRecorded{
									workType:    BC_RECORDED,
									AgreementId: agreementId,
									Protocol:    protocol,
								}
								work <- agreementWork
								glog.V(5).Infof("AgreementBot queued blockchain agreement recorded event: %v", agreementWork)

								// If the event is a agreement terminated event
							} else if protocolHandler.ProducerTermination(rawEvent) || protocolHandler.ConsumerTermination(rawEvent) {
								agreementWork := CSHandleBCTerminated{
									workType:    BC_TERMINATED,
									AgreementId: agreementId,
									Protocol:    protocol,
								}
								work <- agreementWork
								glog.V(5).Infof("AgreementBot queued agreement cancellation due to blockchain termination event: %v", agreementWork)
							}
						}

					case *PolicyChangedCommand:
						glog.V(5).Infof("AgreementBot received policy changed command.")
						cmd, _ := command.(*PolicyChangedCommand)

						if eventPol, err := policy.DemarshalPolicy(cmd.Msg.PolicyString()); err != nil {
							glog.Errorf(fmt.Sprintf("AgreementBot error demarshalling change event policy %v, error: %v", cmd.Msg.PolicyString(), err))
						} else {
							protocolName := eventPol.AgreementProtocols[0].Name

							InProgress := func() AFilter {
								return func(e Agreement) bool { return e.AgreementCreationTime != 0 && e.AgreementTimedout == 0}
							}

							if agreements, err := FindAgreements(w.db, []AFilter{UnarchivedAFilter(),InProgress()}, protocolName); err == nil {
								for _, ag := range agreements {

									if pol, err := policy.DemarshalPolicy(ag.Policy); err != nil {
										glog.Errorf(AWlogString(fmt.Sprintf("unable to demarshal policy for agreement %v, error %v", ag.CurrentAgreementId, err)))

									} else if eventPol.Header.Name != pol.Header.Name {
										// This agreement is using a policy different from the one that changed.
										glog.V(5).Infof("AgreementBot policy change handler skipping agreement %v because it is using a policy that did not chnage.", ag.CurrentAgreementId)
										continue
									} else if err := w.pm.MatchesMine(pol); err != nil {
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
											// Choose this device's agreement within the HA group to start upgrading.
											// Delete this workload usage record so that a new agreement will be made starting from the highest priority workload
											if err := DeleteWorkloadUsage(w.db, ag.DeviceId, ag.PolicyName); err != nil {
												glog.Warningf(AWlogString(fmt.Sprintf("error deleting workload usage for %v using policy %v, error: %v", ag.DeviceId, ag.PolicyName, err)))
											}
											agreementWork := CSCancelAgreement{
												workType:    CANCEL,
												AgreementId: ag.CurrentAgreementId,
												Protocol:    ag.AgreementProtocol,
												Reason:      citizenscientist.AB_CANCEL_POLICY_CHANGED,
											}
											work <- agreementWork
										} else {
											// Non-HA device or agrement without workload priority in the policy, re-make the agreement
											// Delete this workload usage record so that a new agreement will be made starting from the highest priority workload
											if err := DeleteWorkloadUsage(w.db, ag.DeviceId, ag.PolicyName); err != nil {
												glog.Warningf(AWlogString(fmt.Sprintf("error deleting workload usage for %v using policy %v, error: %v", ag.DeviceId, ag.PolicyName, err)))
											}
											agreementWork := CSCancelAgreement{
												workType:    CANCEL,
												AgreementId: ag.CurrentAgreementId,
												Protocol:    ag.AgreementProtocol,
												Reason:      citizenscientist.AB_CANCEL_POLICY_CHANGED,
											}
											work <- agreementWork
										}
									} else {
										glog.V(5).Infof(AWlogString(fmt.Sprintf("for agreement %v, no policy content differences detected", ag.CurrentAgreementId)))
									}

								}
							} else {
								glog.Errorf(AWlogString(fmt.Sprintf("error searching database: %v", err)))
							}
						}

					case *PolicyDeletedCommand:
						glog.V(5).Infof("AgreementBot received policy deleted command.")
						cmd, _ := command.(*PolicyDeletedCommand)

						if eventPol, err := policy.DemarshalPolicy(cmd.Msg.PolicyString()); err != nil {
							glog.Errorf(fmt.Sprintf("AgreementBot error demarshalling policy %v from deleted event, error: %v", cmd.Msg.PolicyString(), err))
						} else {
							protocolName := eventPol.AgreementProtocols[0].Name

							InProgress := func() AFilter {
								return func(e Agreement) bool { return e.AgreementCreationTime != 0 && e.AgreementTimedout == 0}
							}

							if agreements, err := FindAgreements(w.db, []AFilter{UnarchivedAFilter(),InProgress()}, protocolName); err == nil {
								for _, ag := range agreements {

									if pol, err := policy.DemarshalPolicy(ag.Policy); err != nil {
										glog.Errorf(AWlogString(fmt.Sprintf("unable to demarshal policy for agreement %v, error %v", ag.CurrentAgreementId, err)))
									} else if existingPol := w.pm.GetPolicy(pol.Header.Name); existingPol == nil {
										glog.Errorf(AWlogString(fmt.Sprintf("agreement %v has a policy %v that doesn't exist anymore", ag.CurrentAgreementId, pol.Header.Name)))

										// Remove any workload usage records so that a new agreement will be made starting from the highest priority workload.
										if err := DeleteWorkloadUsage(w.db, ag.DeviceId, ag.PolicyName); err != nil {
											glog.Warningf(AWlogString(fmt.Sprintf("error deleting workload usage for %v using policy %v, error: %v", ag.DeviceId, ag.PolicyName, err)))
										}

										// Queue up a cancellation command for this agreement.
										agreementWork := CSCancelAgreement{
											workType:    CANCEL,
											AgreementId: ag.CurrentAgreementId,
											Protocol:    ag.AgreementProtocol,
											Reason:      citizenscientist.AB_CANCEL_POLICY_CHANGED,
										}
										work <- agreementWork

									}

								}
							} else {
								glog.Errorf(AWlogString(fmt.Sprintf("error searching database: %v", err)))
							}
						}

					default:
						glog.Errorf("Unknown command (%T): %v", command, command)
					}

				case <-time.After(time.Duration(w.Worker.Manager.Config.AgreementBot.NewContractIntervalS) * time.Second):

					glog.V(4).Infof("AgreementBot for %v protocol Polling Exchange.", protocol)

					// Get a copy of all policies in the policy manager so that we can safely iterate it
					policies := w.pm.GetAllAvailablePolicies()
					for _, consumerPolicy := range policies {

						agp := policy.AgreementProtocol_Factory(protocol)
						agpl := new(policy.AgreementProtocolList)
						*agpl = append(*agpl, *agp)
						if _, err := consumerPolicy.AgreementProtocols.Intersects_With(agpl); err != nil {
							continue
						} else if devices, err := w.searchExchange(&consumerPolicy); err != nil {
							glog.Errorf("AgreementBot received error on searching for %v, error: %v", &consumerPolicy, err)
						} else {

							for _, dev := range *devices {

								glog.V(3).Infof("AgreementBot picked up %v", dev.ShortString())
								glog.V(5).Infof("AgreementBot picked up %v", dev)

								// If this device is advertising a property that we are supposed to ignore, then skip it.
								if ignore, err := w.ignoreDevice(dev); err != nil {
									glog.Errorf("AgreementBot received error checking for ignored device %v, error: %v", dev, err)
								} else if ignore {
									glog.V(5).Infof("AgreementBot skipping device %v, advertises ignored property", dev)
									continue
								}

								// Check to see if we're already doing something with this device
								pendingAgreementFilter := func() AFilter {
									return func(a Agreement) bool {
										return a.DeviceId == dev.Id && a.PolicyName == consumerPolicy.Header.Name && a.AgreementFinalizedTime == 0 && a.AgreementTimedout == 0
									}
								}

								// Find all agreements that are in progress. They might be waiting for a reply or not yet finalized on blockchain.
								if agreements, err := FindAgreements(w.db, []AFilter{UnarchivedAFilter(), pendingAgreementFilter()}, citizenscientist.PROTOCOL_NAME); err != nil {
									glog.Errorf("AgreementBot received error trying to find pending agreements: %v", err)
								} else if len(agreements) != 0 {
									glog.V(5).Infof("AgreementBot skipping device id %v, agreement attempt already in progress", dev.Id)
									continue
								} else {

									// Deserialize the JSON policy blob into a policy object
									producerPolicy := new(policy.Policy)
									if len(dev.Microservices[0].Policy) == 0 {
										glog.Errorf("AgreementBot received empty policy blob, skipping this microservice.")
									} else if err := json.Unmarshal([]byte(dev.Microservices[0].Policy), producerPolicy); err != nil {
										glog.Errorf("AgreementBot received error demarshalling policy blob %v, error: %v", dev.Microservices[0].Policy, err)

										// Check to see if the device's policy is compatible
									} else if err := policy.Are_Compatible(producerPolicy, &consumerPolicy); err != nil {
										glog.Errorf("AgreementBot received error comparing %v and %v, error: %v", *producerPolicy, consumerPolicy, err)
									} else if err := w.incompleteHAGroup(dev, producerPolicy); err != nil {
										glog.Warningf("AgreementBot received error checking HA group %v completeness for device %v, error: %v", producerPolicy.HAGroup, dev.Id, err)
									} else {
										agreementWork := CSInitiateAgreement{
											workType:       INITIATE,
											ProducerPolicy: *producerPolicy,
											ConsumerPolicy: consumerPolicy,
											Device:         dev,
										}

										work <- agreementWork
										glog.V(5).Infof("AgreementBot queued agreement attempt")
									}
								}
							}

						}
					}
				}

			}
		}()

	} else {
		glog.Errorf("AgreementBot encountered unknown agreement protocol %v, agreement processor terminating.", protocol)
		delete(w.pwcommands, protocol)
	}

}

// This function runs through the agbot policy and builds a list of properties and values that
// it wants to search on.
func RetrieveAllProperties(pol *policy.Policy) (*policy.PropertyList, error) {
	pl := new(policy.PropertyList)

	for _, p := range pol.Properties {
		*pl = append(*pl, p)
	}

	*pl = append(*pl, policy.Property{Name: "version", Value: pol.APISpecs[0].Version})
	*pl = append(*pl, policy.Property{Name: "arch", Value: pol.APISpecs[0].Arch})
	*pl = append(*pl, policy.Property{Name: "agreementProtocols", Value: pol.AgreementProtocols.As_String_Array()})

	return pl, nil
}

func DeleteConsumerAgreement(url string, agbotId string, token string, agreementId string) error {

	logString := func(v interface{}) string {
		return fmt.Sprintf("AgreementBot Governance: %v", v)
	}

	glog.V(5).Infof(logString(fmt.Sprintf("deleting agreement %v in exchange", agreementId)))

	var resp interface{}
	resp = new(exchange.PostDeviceResponse)
	targetURL := url + "agbots/" + agbotId + "/agreements/" + agreementId
	for {
		if err, tpErr := exchange.InvokeExchange(&http.Client{}, "DELETE", targetURL, agbotId, token, nil, &resp); err != nil {
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

func (w *AgreementBotWorker) searchExchange(pol *policy.Policy) (*[]exchange.SearchResultDevice, error) {

	// Convert the policy into a microservice object that the exchange can search on
	ms := make([]exchange.Microservice, 0, 10)

	newMS := new(exchange.Microservice)
	newMS.Url = pol.APISpecs[0].SpecRef
	newMS.NumAgreements = 1

	if props, err := RetrieveAllProperties(pol); err != nil {
		return nil, errors.New(fmt.Sprintf("AgreementBotWorker received error calculating properties: %v", err))
	} else {
		for _, prop := range *props {
			if newProp, err := exchange.ConvertPropertyToExchangeFormat(&prop); err != nil {
				return nil, errors.New(fmt.Sprintf("AgreementBotWorker got error searching exchange: %v", err))
			} else {
				newMS.Properties = append(newMS.Properties, *newProp)
			}
		}
	}

	ser := exchange.CreateSearchRequest()
	ser.SecondsStale = w.Config.AgreementBot.ActiveDeviceTimeoutS
	ser.DesiredMicroservices = append(ms, *newMS)

	var resp interface{}
	resp = new(exchange.SearchExchangeResponse)
	targetURL := w.Worker.Manager.Config.AgreementBot.ExchangeURL + "search/devices"
	for {
		if err, tpErr := exchange.InvokeExchange(w.httpClient, "POST", targetURL, w.agbotId, w.token, ser, &resp); err != nil {
			return nil, err
		} else if tpErr != nil {
			glog.V(5).Infof(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(3).Infof("AgreementBotWorker found %v devices in exchange.", len(resp.(*exchange.SearchExchangeResponse).Devices))
			dev := resp.(*exchange.SearchExchangeResponse).Devices
			return &dev, nil
		}
	}
}

func (w *AgreementBotWorker) syncOnInit() error {
	glog.V(3).Infof(AWlogString("beginning sync up."))

	// Loop through our database and check each record for accuracy with the exchange and the blockchain
	if agreements, err := FindAgreements(w.db, []AFilter{UnarchivedAFilter()}, citizenscientist.PROTOCOL_NAME); err == nil {
		for _, ag := range agreements {

			// If the agreement has received a reply then we just need to make sure that the policy manager's agreement counts
			// are correct. Even for already timedout agreements, the governance process will cleanup old and outdated agreements,
			// so we don't need to do anything here.
			if ag.AgreementCreationTime != 0 {
				if pol, err := policy.DemarshalPolicy(ag.Policy); err != nil {
					glog.Errorf(AWlogString(fmt.Sprintf("unable to demarshal policy for agreement %v, error %v", ag.CurrentAgreementId, err)))
				} else if existingPol := w.pm.GetPolicy(pol.Header.Name); existingPol == nil {
					glog.Errorf(AWlogString(fmt.Sprintf("agreement %v has a policy %v that doesn't exist anymore", ag.CurrentAgreementId, pol.Header.Name)))
					// Update state in exchange
					if err := DeleteConsumerAgreement(w.Config.AgreementBot.ExchangeURL, w.agbotId, w.token, ag.CurrentAgreementId); err != nil {
						glog.Errorf(AWlogString(fmt.Sprintf("error deleting agreement %v in exchange: %v", ag.CurrentAgreementId, err)))
					}
					// Remove any workload usage records so that a new agreement will be made starting from the highest priority workload
					if err := DeleteWorkloadUsage(w.db, ag.DeviceId, ag.PolicyName); err != nil {
						glog.Warningf(AWlogString(fmt.Sprintf("error deleting workload usage for %v using policy %v, error: %v", ag.DeviceId, ag.PolicyName, err)))
					}
					// Indicate that the agreement is timed out
					if _, err := AgreementTimedout(w.db, ag.CurrentAgreementId, citizenscientist.PROTOCOL_NAME); err != nil {
						glog.Errorf(AWlogString(fmt.Sprintf("error marking agreement %v terminated: %v", ag.CurrentAgreementId, err)))
					}
					w.pwcommands[citizenscientist.PROTOCOL_NAME] <- NewAgreementTimeoutCommand(ag.CurrentAgreementId, ag.AgreementProtocol, citizenscientist.AB_CANCEL_POLICY_CHANGED)
				} else if err := w.pm.MatchesMine(pol); err != nil {
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
				} else if err := w.pm.AttemptingAgreement(existingPol, ag.CurrentAgreementId); err != nil {
					glog.Errorf(AWlogString(fmt.Sprintf("cannot update agreement count for %v, error: %v", ag.CurrentAgreementId, err)))
				} else if err := w.pm.FinalAgreement(existingPol, ag.CurrentAgreementId); err != nil {
					glog.Errorf(AWlogString(fmt.Sprintf("cannot update agreement count for %v, error: %v", ag.CurrentAgreementId, err)))

					// There is a small window where an agreement might not have been recorded in the exchange. Let's just make sure.
				} else {

					var exchangeAgreement map[string]exchange.AgbotAgreement
					var resp interface{}
					resp = new(exchange.AllAgbotAgreementsResponse)
					targetURL := w.Worker.Manager.Config.AgreementBot.ExchangeURL + "agbots/" + w.agbotId + "/agreements/" + ag.CurrentAgreementId

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
							w.recordConsumerAgreementState(ag.CurrentAgreementId, pol.APISpecs[0].SpecRef, state)
						}
					}
					glog.V(3).Infof(AWlogString(fmt.Sprintf("added agreement %v to policy agreement counter.", ag.CurrentAgreementId)))
				}

				// This state should never occur, but could if there was an error along the way. It means that a DB record
				// was created for this agreement but the record was never updated with the creation time, which is supposed to occur
				// immediately following creation of the record. Further, if this were to occur, then the exchange should not have been
				// updated, so there is no reason to try to clean that up. Same is true for the workload usage records.
			} else if ag.AgreementInceptionTime != 0 && ag.AgreementCreationTime == 0 {
				if err := DeleteAgreement(w.db, ag.CurrentAgreementId, citizenscientist.PROTOCOL_NAME); err != nil {
					glog.Errorf(AWlogString(fmt.Sprintf("error deleting partially created agreement: %v, error: %v", ag.CurrentAgreementId, err)))
				}
			}

		}
	} else {
		return errors.New(AWlogString(fmt.Sprintf("error searching database: %v", err)))
	}

	glog.V(3).Infof(AWlogString("sync up completed normally."))
	return nil
}

func (w *AgreementBotWorker) cleanupAgreement(ag *Agreement) {
	// Update state in exchange
	if err := DeleteConsumerAgreement(w.Config.AgreementBot.ExchangeURL, w.agbotId, w.token, ag.CurrentAgreementId); err != nil {
		glog.Errorf(AWlogString(fmt.Sprintf("error deleting agreement %v in exchange: %v", ag.CurrentAgreementId, err)))
	}

	// Delete this workload usage record so that a new agreement will be made starting from the highest priority workload
	if err := DeleteWorkloadUsage(w.db, ag.DeviceId, ag.PolicyName); err != nil {
		glog.Warningf(AWlogString(fmt.Sprintf("error deleting workload usage for %v using policy %v, error: %v", ag.DeviceId, ag.PolicyName, err)))
	}

	// Indicate that the agreement is timed out
	if _, err := AgreementTimedout(w.db, ag.CurrentAgreementId, citizenscientist.PROTOCOL_NAME); err != nil {
		glog.Errorf(AWlogString(fmt.Sprintf("error marking agreement %v terminated: %v", ag.CurrentAgreementId, err)))
	}

	w.pwcommands[citizenscientist.PROTOCOL_NAME] <- NewAgreementTimeoutCommand(ag.CurrentAgreementId, ag.AgreementProtocol, citizenscientist.AB_CANCEL_POLICY_CHANGED)
}

func (w *AgreementBotWorker) recordConsumerAgreementState(agreementId string, workloadID string, state string) error {

	glog.V(5).Infof(AWlogString(fmt.Sprintf("setting agreement %v state to %v", agreementId, state)))

	as := new(exchange.PutAgbotAgreementState)
	as.Workload = workloadID
	as.State = state
	var resp interface{}
	resp = new(exchange.PostDeviceResponse)
	targetURL := w.Config.AgreementBot.ExchangeURL + "agbots/" + w.agbotId + "/agreements/" + agreementId
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

// This function checks the Exchange for every declared HA partner to verify that the partner is registered in the
// exchange. As long as all partners are registered, agreements can be made. The partners dont have to be up and heart
// beating, they just have to be registered. If not all partners are registered then no agreements will be attempted
// with any of the registered partners.
func (w *AgreementBotWorker) incompleteHAGroup(dev exchange.SearchResultDevice, producerPolicy *policy.Policy) error {

	// If the HA group specification is empty, there is nothing to check.
	if len(producerPolicy.HAGroup.Partners) == 0  {
		return nil
	} else {

		// Make sure all partners are in the exchange
		for _, partnerId := range producerPolicy.HAGroup.Partners {

			if _, err := getTheDevice(partnerId, w.Config.AgreementBot.ExchangeURL, w.agbotId, w.token); err != nil {
				glog.Warningf(AWlogString(fmt.Sprintf("could not obtain device %v from the exchange: %v", partnerId, err)))
				return err
			}
		}
		return nil

	}
}

func getTheDevice(deviceId string, url string, agbotId string, token string) (*exchange.Device, error) {

	glog.V(5).Infof(AWlogString(fmt.Sprintf("retrieving device %v from exchange", deviceId)))

	var resp interface{}
	resp = new(exchange.GetDevicesResponse)
	targetURL := url + "devices/" + deviceId
	for {
		if err, tpErr := exchange.InvokeExchange(&http.Client{}, "GET", targetURL, agbotId, token, nil, &resp); err != nil {
			glog.Errorf(AWlogString(fmt.Sprintf(err.Error())))
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			devs := resp.(*exchange.GetDevicesResponse).Devices
			if dev, there := devs[deviceId]; !there {
				return nil, errors.New(fmt.Sprintf("device %v not in GET response %v as expected", deviceId, devs))
			} else {
				glog.V(5).Infof(AWlogString(fmt.Sprintf("retrieved device %v from exchange %v", deviceId, dev)))
				return &dev, nil
			}
		}
	}

}

func (w *AgreementBotWorker) ignoreDevice(dev exchange.SearchResultDevice) (bool, error) {
	for _, prop := range dev.Microservices[0].Properties {
		if listContains(w.Config.AgreementBot.IgnoreContractWithAttribs, prop.Name) {
			return true, nil
		}
	}
	return false, nil
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
	targetURL := w.Config.AgreementBot.ExchangeURL + "agbots/" + w.agbotId
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

// ==========================================================================================================
// Utility functions

var AWlogString = func(v interface{}) string {
	return fmt.Sprintf("AgreementBotWorker %v", v)
}
