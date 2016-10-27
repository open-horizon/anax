package agreementbot

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
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
	"time"
)

// must be safely-constructed!!
type AgreementBotWorker struct {
	worker.Worker // embedded field
	db            *bolt.DB
	httpClient    *http.Client
	agbotId       string
	token         string
	protocols     map[string]bool
	bc            *ethblockchain.BaseContracts
	pm            *policy.PolicyManager
	pwcommands    chan worker.Command
}

func NewAgreementBotWorker(config *config.HorizonConfig, db *bolt.DB) *AgreementBotWorker {
	messages := make(chan events.Message, 100)   // The channel for outbound messages to the anax wide bus
	commands := make(chan worker.Command, 100)   // The channel for commands into the agreement bot worker
	pwcommands := make(chan worker.Command, 100) // The channel for commands into the agreement bot protocol workers

	worker := &AgreementBotWorker{
		Worker: worker.Worker{
			Manager: worker.Manager{
				Config:   config,
				Messages: messages,
			},

			Commands: commands,
		},

		db:         db,
		httpClient: &http.Client{},
		agbotId:    config.AgreementBot.ExchangeId,
		token:      config.AgreementBot.ExchangeToken,
		protocols:  make(map[string]bool),
		pwcommands: pwcommands,
	}

	glog.Info("Starting AgreementBot worker")
	worker.start()
	return worker
}

func (w *AgreementBotWorker) Messages() chan events.Message {
	return w.Worker.Manager.Messages
}

func (w *AgreementBotWorker) NewEvent(incoming events.Message) {

	switch incoming.(type) {
	case *events.WhisperReceivedMessage:
		msg, _ := incoming.(*events.WhisperReceivedMessage)

		// TODO: When we replace this with telehash, check to see if the protocol in the message
		// is already known to us. For now, whisper doesnt put the topic in the message so we have
		// now way of checking.
		agCmd := NewReceivedWhisperMessageCommand(*msg)
		w.Commands <- agCmd

	case *events.ABPolicyCreatedMessage:
		msg, _ := incoming.(*events.ABPolicyCreatedMessage)

		agCmd := NewNewPolicyCommand(msg.PolicyFile())
		w.Commands <- agCmd

	default: //nothing

	}

	return
}

func (w *AgreementBotWorker) start() {
	glog.Info("AgreementBot worker started")

	// If there is no Agbot config, we will terminate
	if w.Config.AgreementBot == (config.AGConfig{}) {
		glog.Errorf("AgreementBotWorker terminating, no AgreementBot config.")
		return
	} else if w.db == nil {
		glog.Errorf("AgreementBotWorker terminating, no AgreementBot database configured.")
		return
	}

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
		runtime.Gosched()
	}

	// For each agreement protocol in the current list of configured policies, startup a processor
	// to initiate the protocol and tell the whisper worker that it needs to listen on a specific
	// topic.

	w.protocols = w.pm.GetAllAgreementProtocols()
	for protocolName, _ := range w.protocols {
		w.Messages() <- events.NewWhisperSubscribeToMessage(events.SUBSCRIBE_TO, protocolName)
		go w.InitiateAgreementProtocolHandler(protocolName)
	}

	// Begin heartbeating with the exchange.
	targetURL := w.Manager.Config.AgreementBot.ExchangeURL + "agbots/" + w.agbotId + "/heartbeat?token=" + w.token
	go exchange.Heartbeat(&http.Client{}, targetURL, w.Worker.Manager.Config.AgreementBot.ExchangeHeartbeat)

	// Start the governance routine.
	go w.GovernAgreements()

	// Enter the command processing loop. Initialization is complete so wait for commands to
	// perform. Commands are created as the result of events that are triggered elsewhere
	// in the system.
	go func() {

		for {
			glog.V(2).Infof("AgreementBotWorker blocking for commands")
			command := <-w.Commands
			glog.V(2).Infof("AgreementBotWorker received command: %v", command)

			switch command.(type) {
			case *ReceivedWhisperMessageCommand:
				cmd := command.(*ReceivedWhisperMessageCommand)
				// TODO: Hack assume there is only one protocol handler
				w.pwcommands <- cmd

			case *NewPolicyCommand:
				cmd := command.(*NewPolicyCommand)
				if newPolicy, err := policy.ReadPolicyFile(cmd.PolicyFile); err != nil {
					glog.Errorf("AgreementBotWorker unable to read policy file %v into memory, error: %v", cmd.PolicyFile, err)
				} else {
					w.pm.AddPolicy(newPolicy) // We dont care if it's already there

					// Make sure the whisper subsystem is subscribed to messages for this policy's agreement protocol
					w.Messages() <- events.NewWhisperSubscribeToMessage(events.SUBSCRIBE_TO, newPolicy.AgreementProtocols[0].Name)
				}

			default:
				glog.Errorf("Unknown command (%T): %v", command, command)
			}

			glog.V(5).Infof("AgreementBotWorker handled command")
			runtime.Gosched()
		}

	}()

	glog.Info("AgreementBotWorker waiting for commands.")

}

// There is one of these running is a go routine for each agreement protocol that we support
func (w *AgreementBotWorker) InitiateAgreementProtocolHandler(protocol string) {

	if protocol == policy.CitizenScientist {

		// Set up random number gen. This is used to generate agreement id strings.
		random := rand.New(rand.NewSource(int64(time.Now().Nanosecond())))

		// Set up agreement worker pool based on the current technical config.
		work := make(chan CSAgreementWork)
		for ix := 0; ix < w.Worker.Manager.Config.AgreementBot.AgreementWorkers; ix++ {
			agw := NewCSAgreementWorker(w.pm, w.Worker.Manager.Config, w.db)
			go agw.start(work, random, w.bc)
		}

		// Main function of the agreement processor. Constantly search through the list of available
		// devices to ensure that we are contracting with as many devices as possible.
		go func() {
			for {

				glog.V(5).Infof("AgreementBot about to select command (non-blocking).")
				select {
				case command := <-w.pwcommands:
					switch command.(type) {
					case *ReceivedWhisperMessageCommand:
						glog.V(5).Infof("AgreementBot received inbound whisper message.")
						cmd := command.(*ReceivedWhisperMessageCommand)
						agreementWork := CSHandleReply{
							workType: REPLY,
							Reply:    cmd.Msg.Payload(),
						}
						work <- agreementWork
						glog.V(5).Infof("AgreementBot queued possible reply message")

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
							glog.Errorf("AgreementBot received error on searching for %v, error: %v", consumerPolicy, err)
						} else {

							for _, dev := range *devices {

								glog.V(5).Infof("AgreementBot picked up %v", dev)

								// Check to see if we're already doing something with this device
								pendingAgreementFilter := func() AFilter {
									return func(a Agreement) bool {
										return a.DeviceId == dev.Id && a.AgreementFinalizedTime == 0 && a.AgreementTimedout == 0
									}
								}

								// Find all agreements that are in progress. They might be waiting for a reply or not yet finalized on blockchain.
								if agreements, err := FindAgreements(w.db, []AFilter{pendingAgreementFilter()}); err != nil {
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
									} else {
										agreementWork := CSInitiateAgreement{
											workType:       INITIATE,
											ProducerPolicy: producerPolicy,
											ConsumerPolicy: &consumerPolicy,
											Device:         &dev,
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
	*pl = append(*pl, policy.Property{Name: "dataVerification", Value: pol.DataVerify.Enabled})
	*pl = append(*pl, policy.Property{Name: "agreementProtocols", Value: pol.AgreementProtocols.As_String_Array()})

	return pl, nil
}

func (w *AgreementBotWorker) searchExchange(pol *policy.Policy) (*[]exchange.Device, error) {

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
	ser.DesiredMicroservices = append(ms, *newMS)

	var resp interface{}
	resp = new(exchange.SearchExchangeResponse)
	targetURL := w.Worker.Manager.Config.AgreementBot.ExchangeURL + "search/devices?id=" + w.agbotId + "&token=" + w.token
	for {
		if err, tpErr := exchange.InvokeExchange(w.httpClient, "POST", targetURL, ser, &resp); err != nil {
			return nil, err
		} else if tpErr != nil {
			glog.V(5).Infof(err.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(3).Infof("AgreementBotWorker found %v devices in exchange.", len(resp.(*exchange.SearchExchangeResponse).Devices))
			dev := resp.(*exchange.SearchExchangeResponse).Devices
			return &dev, nil
		}
	}
}

// ==========================================================================================================
// Utility functions
