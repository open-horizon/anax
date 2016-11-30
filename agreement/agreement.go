package agreement

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/citizenscientist"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/device"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/governance"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/worker"
	gwhisper "github.com/open-horizon/go-whisper"
	"net/http"
	"net/url"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// must be safely-constructed!!
type AgreementWorker struct {
	worker.Worker       // embedded field
	db                  *bolt.DB
	httpClient          *http.Client
	userId              string
	deviceId            string
	deviceToken         string
	protocols           map[string]bool
	pm                  *policy.PolicyManager
	bcClientInitialized bool
}

func NewAgreementWorker(config *config.HorizonConfig, db *bolt.DB, pm *policy.PolicyManager) *AgreementWorker {
	messages := make(chan events.Message)
	commands := make(chan worker.Command, 100)

	id, _ := device.Id()

	token := ""
	if dev, _ := persistence.FindExchangeDevice(db); dev != nil {
		token = dev.Token
	}

	worker := &AgreementWorker{
		Worker: worker.Worker{
			Manager: worker.Manager{
				Config:   config,
				Messages: messages,
			},

			Commands: commands,
		},

		db:         db,
		httpClient: &http.Client{},
		protocols:  make(map[string]bool),
		pm:         pm,
		bcClientInitialized: false,
		deviceId:   id,
		deviceToken: token,
	}

	glog.Info("Starting Agreement worker")
	worker.start()
	return worker
}

func (w *AgreementWorker) Messages() chan events.Message {
	return w.Worker.Manager.Messages
}

func (w *AgreementWorker) NewEvent(incoming events.Message) {

	switch incoming.(type) {
	case *events.EdgeRegisteredExchangeMessage:
		msg, _ := incoming.(*events.EdgeRegisteredExchangeMessage)
		w.Commands <- NewDeviceRegisteredCommand(msg.Token())

	case *events.PolicyCreatedMessage:
		msg, _ := incoming.(*events.PolicyCreatedMessage)

		switch msg.Event().Id {
		case events.NEW_POLICY:
			w.Commands <- NewAdvertisePolicyCommand(msg.PolicyFile())
		default:
			glog.Errorf("AgreementWorker received Unsupported event: %v", incoming.Event().Id)
		}

	case *events.WhisperReceivedMessage:
		msg, _ := incoming.(*events.WhisperReceivedMessage)

		// TODO: When we replace this with telehash, check to see if the protocol in the message
		// is already known to us. For now, whisper doesnt put the topic in the message so we have
		// now way of checking.
		agCmd := NewReceivedProposalCommand(*msg)
		w.Commands <- agCmd

	case *events.BlockchainClientInitilizedMessage:
		msg, _ := incoming.(*events.BlockchainClientInitilizedMessage)
		switch msg.Event().Id {
		case events.BC_CLIENT_INITIALIZED:
			w.bcClientInitialized = true
		}

	default: //nothing
	}

	return
}

func (w *AgreementWorker) start() {

	glog.Info(logString(fmt.Sprintf("started")))

	// If there is no edge config then there is nothing to init on the edge
	if w.Worker.Manager.Config.Edge == (config.Config{}) {
		return
	}

	// Enter the command processing loop. Initialization is complete so wait for commands to
	// perform. Commands are created as the result of events that are triggered elsewhere
	// in the system.
	go func() {

		if w.deviceToken != "" {
			// Sync up between what's in our database versus what's in the exchange, and make sure that the policy manager's
			// agreement counts are correct. The governance routine will cancel any agreements whose state might have changed
			// while the agbot was down. We will also check to make sure that policies havent changed. If they have, then
			// we will cancel agreements and allow them to re-negotiate.
			if err := w.syncOnInit(); err != nil {
				glog.Errorf(logString(fmt.Sprintf("Terminating, unable to sync up, error: %v", err)))
				return
			}

			// If the device is registered, start heartbeating. If the device isn't registered yet, then we will
			// start heartbeating when the registration event comes in.
			targetURL := w.Manager.Config.Edge.ExchangeURL + "devices/" + w.deviceId + "/heartbeat"
			go exchange.Heartbeat(w.httpClient, targetURL, w.deviceId, w.deviceToken, w.Worker.Manager.Config.Edge.ExchangeHeartbeat)
		}

		// Publish what we have for the world to see
		if err := w.advertiseAllPolicies(w.Worker.Manager.Config.Edge.PolicyPath); err != nil {
			glog.Errorf(logString(fmt.Sprintf("unable to advertise policies with exchange, error: %v", err)))
		}

		// Start the go routine that makes sure the whisper id is correct in the exchange
		go w.maintainWhisperId()

		// Handle agreement processor commands
		for {
			glog.V(2).Infof(logString(fmt.Sprintf("blocking for commands")))
			command := <-w.Commands
			glog.V(2).Infof(logString(fmt.Sprintf("received command: %T", command)))

			switch command.(type) {
			case *DeviceRegisteredCommand:
				cmd, _ := command.(*DeviceRegisteredCommand)
				w.handleDeviceRegistered(cmd)

			case *TerminateCommand:
				cmd, _ := command.(*TerminateCommand)
				glog.Errorf(logString(fmt.Sprintf("terminating, reason: %v", cmd.reason)))
				return

			case *AdvertisePolicyCommand:
				cmd, _ := command.(*AdvertisePolicyCommand)

				if newPolicy, err := policy.ReadPolicyFile(cmd.PolicyFile); err != nil {
					glog.Errorf(logString(fmt.Sprintf("unable to read policy file %v into memory, error: %v", cmd.PolicyFile, err)))
				} else if err := w.pm.AddPolicy(newPolicy); err != nil {
					glog.Errorf(logString(fmt.Sprintf("policy name is a duplicate, not added, error: %v", err)))
				} else {

					// Publish what we have for the world to see
					if err := w.advertiseAllPolicies(w.Worker.Manager.Config.Edge.PolicyPath); err != nil {
						glog.Errorf(logString(fmt.Sprintf("unable to advertise policies with exchange, error: %v", err)))
					}
				}

			case *ReceivedProposalCommand:
				cmd, _ := command.(*ReceivedProposalCommand)

				protocolHandler := citizenscientist.NewProtocolHandler(w.Config.Edge.GethURL, w.pm)

				// Proposal messages could be duplicates or fakes if there is an agbot imposter in the network. The sequence
				// of checks here is very important to prevent duplicates from causing chaos, especially if there are duplicates
				// on agreement worker threads at the same time.
				if proposal, err := protocolHandler.ValidateProposal(cmd.Msg.Payload()); err != nil {
					glog.Errorf(logString(fmt.Sprintf("discarding message: %v due to %v", cmd.Msg.Payload(), err)))
				} else if agAlreadyExists, err := persistence.FindEstablishedAgreements(w.db, citizenscientist.PROTOCOL_NAME, []persistence.EAFilter{persistence.UnarchivedEAFilter(),persistence.IdEAFilter(proposal.AgreementId)}); err != nil {
					glog.Errorf(logString(fmt.Sprintf("unable to retrieve agreements from database, error %v", err)))
				} else if len(agAlreadyExists) != 0 {
					glog.Errorf(logString(fmt.Sprintf("agreement %v already exists, ignoring proposal: %v", proposal.AgreementId, proposal.ShortString())))
				} else if tcPolicy, err := policy.DemarshalPolicy(proposal.TsAndCs); err != nil {
					glog.Errorf(logString(fmt.Sprintf("received error demarshalling TsAndCs, %v", err)))
				} else if reply, err := protocolHandler.DecideOnProposal(proposal, cmd.Msg.From(), w.deviceId); err != nil {
					glog.Errorf(logString(fmt.Sprintf("respond to proposal with error: %v", err)))
				} else if _, err := persistence.NewEstablishedAgreement(w.db, tcPolicy.Header.Name, proposal.AgreementId, proposal.ConsumerId, cmd.Msg.Payload(), citizenscientist.PROTOCOL_NAME, tcPolicy.APISpecs[0].SpecRef); err != nil {
					glog.Errorf(logString(fmt.Sprintf("error persisting new agreement: %v, error: %v", proposal.AgreementId, err)))
				} else if err := w.RecordReply(proposal, reply, citizenscientist.PROTOCOL_NAME, cmd); err != nil {
					glog.Errorf(logString(fmt.Sprintf("unable to record reply %v, error: %v", *reply, err)))
				}

			default:
				glog.Errorf("Unknown command (%T): %v", command, command)
			}

			glog.V(5).Infof(logString(fmt.Sprintf("handled command")))
			runtime.Gosched()
		}

	}()

	glog.Info(logString(fmt.Sprintf("waiting for commands.")))

}

func (w *AgreementWorker) maintainWhisperId() {
	// The device side whisper id can change if geth fails on us. If this happens, we need to
	// update the whisper id in the exchange.

	getWhisperId := func() string {
		if wId, err := gwhisper.AccountId(w.Config.Edge.GethURL); err != nil {
			glog.Errorf(logStringww(fmt.Sprintf("encountered error reading whisper id, error %v",err)))
			return ""
		} else {
			return wId
		}
	}

	for {
		glog.V(5).Infof(logStringww(fmt.Sprintf("checking on whisper id")))

		newId := getWhisperId()
		if newId != "" {
			var resp interface{}
			resp = new(exchange.GetDevicesResponse)
			targetURL := w.Worker.Manager.Config.Edge.ExchangeURL + "devices/" + w.deviceId
			if err, tpErr := exchange.InvokeExchange(w.httpClient, "GET", targetURL, w.deviceId, w.deviceToken, nil, &resp); err != nil || tpErr != nil {
				glog.Errorf(logStringww(fmt.Sprintf("encountered error getting device info from exchange, error %v",err)))
			} else if dev, there := resp.(*exchange.GetDevicesResponse).Devices[w.deviceId]; there {
				glog.V(5).Infof(logStringww(fmt.Sprintf("found device %v in the exchange.", w.deviceId)))
				if dev.MsgEndPoint != newId {
					if err := w.advertiseAllPolicies(w.Worker.Manager.Config.Edge.PolicyPath); err != nil {
						glog.Errorf(logStringww(fmt.Sprintf("unable to update whisper id with exchange, error: %v", err)))
					} else {
						glog.V(3).Infof(logStringww(fmt.Sprintf("updated exchange for %v with new whisper id %v", w.deviceId, newId)))
					}
				} else {
					glog.V(3).Infof(logStringww(fmt.Sprintf("whisper id for %v has not changed", w.deviceId)))
				}
			} else {
				glog.Errorf(logStringww(fmt.Sprintf("did not find device %v in the exchange", w.deviceId)))
			}
		}

		time.Sleep(30 * time.Second)
	}

}


func (w *AgreementWorker) handleDeviceRegistered(cmd *DeviceRegisteredCommand) {

	w.deviceToken = cmd.Token

	// Start the go thread that heartbeats to the exchange
	targetURL := w.Manager.Config.Edge.ExchangeURL + "devices/" + w.deviceId + "/heartbeat"
	go exchange.Heartbeat(w.httpClient, targetURL, w.deviceId, w.deviceToken, w.Worker.Manager.Config.Edge.ExchangeHeartbeat)

}

func (w *AgreementWorker) RecordReply(proposal *citizenscientist.Proposal, reply *citizenscientist.ProposalReply, protocol string, cmd *ReceivedProposalCommand) error {
	if reply != nil {
		// Update the state in the database
		if _, err := persistence.AgreementStateAccepted(w.db, proposal.AgreementId, protocol, cmd.Msg.Payload(), proposal.Address, reply.Signature); err != nil {
			return errors.New(logString(fmt.Sprintf("received error updating database state, %v", err)))

			// Update the state in the exchange
		} else if tcPolicy, err := policy.DemarshalPolicy(proposal.TsAndCs); err != nil {
			return errors.New(logString(fmt.Sprintf("received error demarshalling TsAndCs, %v", err)))
		} else if err := w.recordAgreementState(proposal.AgreementId, tcPolicy.APISpecs[0].SpecRef, "Agree to proposal"); err != nil {
			return errors.New(logString(fmt.Sprintf("received error setting state for agreement %v", err)))
		} else {

			// Publish the "agreement reached" event to the message bus so that torrent can start downloading the workload
			// hash is same as filename w/out extension
			hashes := make(map[string]string, 0)
			signatures := make(map[string]string, 0)
			for _, image := range tcPolicy.Workloads[0].Torrent.Images {
				bits := strings.Split(image.File, ".")
				if len(bits) < 2 {
					return errors.New(fmt.Sprintf("Ill-formed image filename: %v", bits))
				} else {
					hashes[image.File] = bits[0]
				}
				signatures[image.File] = image.Signature
			}
			if url, err := url.Parse(tcPolicy.Workloads[0].Torrent.Url); err != nil {
				return errors.New(fmt.Sprintf("Ill-formed URL: %v", tcPolicy.Workloads[0].Torrent.Url))
			} else {
				wc := gwhisper.NewConfigure("", *url, hashes, signatures, tcPolicy.Workloads[0].Deployment, tcPolicy.Workloads[0].DeploymentSignature, tcPolicy.Workloads[0].DeploymentUserInfo)
				lc := new(events.AgreementLaunchContext)
				lc.Configure = wc
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
				if tcPolicy.Workloads[0].WorkloadPassword == "" {
					envAdds[config.ENVVAR_PREFIX+"CONFIGURE_NONCE"] = proposal.AgreementId
					envAdds[config.COMPAT_ENVVAR_PREFIX+"CONFIGURE_NONCE"] = proposal.AgreementId
				} else {
					envAdds[config.ENVVAR_PREFIX+"CONFIGURE_NONCE"] = tcPolicy.Workloads[0].WorkloadPassword
					envAdds[config.COMPAT_ENVVAR_PREFIX+"CONFIGURE_NONCE"] = tcPolicy.Workloads[0].WorkloadPassword
				}
				envAdds[config.ENVVAR_PREFIX+"HASH"] = tcPolicy.Workloads[0].WorkloadPassword
				// For workload compatibility, the DEVICE_ID env var is passed with and without the prefix. We would like to drop
				// the env var without prefix once all the workloads have ben updated.
				envAdds["DEVICE_ID"] = w.deviceId
				envAdds[config.ENVVAR_PREFIX+"DEVICE_ID"] = w.deviceId
				envAdds[config.COMPAT_ENVVAR_PREFIX+"DEVICE_ID"] = w.deviceId

				lc.EnvironmentAdditions = &envAdds
				lc.AgreementProtocol = citizenscientist.PROTOCOL_NAME
				w.Worker.Manager.Messages <- events.NewAgreementMessage(events.AGREEMENT_REACHED, lc)
			}
		}

	} else {

		if _, err := persistence.ArchiveEstablishedAgreement(w.db, proposal.AgreementId, protocol); err != nil {
			return errors.New(logString(fmt.Sprintf("received error deleting agreement from db %v", err)))
		}

	}

	return nil
}

func (w *AgreementWorker) syncOnInit() error {

	glog.V(3).Infof(logString("beginning sync up."))

	protocolHandler := citizenscientist.NewProtocolHandler(w.Config.Edge.GethURL, w.pm)

	// Loop through our database and check each record for accuracy with the exchange and the blockchain
	if agreements, err := persistence.FindEstablishedAgreements(w.db, citizenscientist.PROTOCOL_NAME, []persistence.EAFilter{persistence.UnarchivedEAFilter()}); err == nil {

		// If there are agreemens in the database then we will assume that the device is already registered
		for _, ag := range agreements {

			// If the agreement has received a reply then we just need to make sure that the policy manager's agreement counts
			// are correct. Even for already timedout agreements, the governance process will cleanup old and outdated agreements,
			// so we don't need to do anything here.
			if ag.AgreementAcceptedTime != 0 {
				if proposal, err := protocolHandler.DemarshalProposal(ag.Proposal); err != nil {
					glog.Errorf(logString(fmt.Sprintf("unable to demarshal proposal for agreement %v, error %v", ag.CurrentAgreementId, err)))
				} else if pol, err := policy.DemarshalPolicy(proposal.ProducerPolicy); err != nil {
					glog.Errorf(logString(fmt.Sprintf("unable to demarshal policy for agreement %v, error %v", ag.CurrentAgreementId, err)))
				} else if existingPol := w.pm.GetPolicy(pol.Header.Name); existingPol == nil {
					glog.Errorf(logString(fmt.Sprintf("agreement %v has a policy %v that doesn't exist anymore", ag.CurrentAgreementId, pol.Header.Name)))
					w.Messages() <- events.NewInitAgreementCancelationMessage(events.AGREEMENT_ENDED, governance.CANCEL_POLICY_CHANGED, citizenscientist.PROTOCOL_NAME, ag.CurrentAgreementId, ag.CurrentDeployment)

				} else if err := w.pm.MatchesMine(pol); err != nil {
					glog.Errorf(logString(fmt.Sprintf("agreement %v has a policy %v that has changed.", ag.CurrentAgreementId, pol.Header.Name)))
					w.Messages() <- events.NewInitAgreementCancelationMessage(events.AGREEMENT_ENDED, governance.CANCEL_POLICY_CHANGED, citizenscientist.PROTOCOL_NAME, ag.CurrentAgreementId, ag.CurrentDeployment)

				} else if err := w.pm.AttemptingAgreement(existingPol, ag.CurrentAgreementId); err != nil {
					glog.Errorf(logString(fmt.Sprintf("cannot update agreement count for %v, error: %v", ag.CurrentAgreementId, err)))
				} else if err := w.pm.FinalAgreement(existingPol, ag.CurrentAgreementId); err != nil {
					glog.Errorf(logString(fmt.Sprintf("cannot update agreement count for %v, error: %v", ag.CurrentAgreementId, err)))

					// There is a small window where an agreement might not have been recorded in the exchange. Let's just make sure.
				} else {

					var exchangeAgreement map[string]exchange.DeviceAgreement
					var resp interface{}
					resp = new(exchange.AllDeviceAgreementsResponse)
					targetURL := w.Worker.Manager.Config.Edge.ExchangeURL + "devices/" + w.deviceId + "/agreements/" + ag.CurrentAgreementId
					for {
						if err, tpErr := exchange.InvokeExchange(w.httpClient, "GET", targetURL, w.deviceId, w.deviceToken, nil, &resp); err != nil {
							return err
						} else if tpErr != nil {
							glog.V(5).Infof(err.Error())
							time.Sleep(10 * time.Second)
							continue
						} else {
							exchangeAgreement = resp.(*exchange.AllDeviceAgreementsResponse).Agreements
							glog.V(5).Infof(logString(fmt.Sprintf("found agreements %v in the exchange.", exchangeAgreement)))
							break
						}
					}

					if _, there := exchangeAgreement[ag.CurrentAgreementId]; !there {
						glog.V(3).Infof(logString(fmt.Sprintf("agreement %v missing from exchange, adding it back in.", ag.CurrentAgreementId)))
						state := ""
						if ag.AgreementFinalizedTime != 0 {
							state = "Finalized Agreement"
						} else if ag.AgreementAcceptedTime != 0 {
							state = "Agree to proposal"
						} else {
							state = "unknown"
						}
						w.recordAgreementState(ag.CurrentAgreementId, pol.APISpecs[0].SpecRef, state)
					}
					glog.V(3).Infof(logString(fmt.Sprintf("added agreement %v to policy agreement counter.", ag.CurrentAgreementId)))
				}

				// This state should never occur, but could if there was an error along the way. It means that a DB record
				// was created for this agreement but the record was never updated with the accepted time, which is supposed to occur
				// immediately following creation of the record. A record in this state needs to be deleted.
			} else if ag.AgreementCreationTime != 0 && ag.AgreementAcceptedTime == 0 {
				if _, err := persistence.ArchiveEstablishedAgreement(w.db, ag.CurrentAgreementId, citizenscientist.PROTOCOL_NAME); err != nil {
					glog.Errorf(logString(fmt.Sprintf("error deleting partially created agreement: %v, error: %v", ag.CurrentAgreementId, err)))
				}
			}

		}
	} else {
		return errors.New(logString(fmt.Sprintf("error searching database: %v", err)))
	}

	glog.V(3).Infof(logString("sync up completed normally."))
	return nil

}

// ===============================================================================================
// Utility functions
//

// get the environmental variables for the workload (this is about launching)
func (w *AgreementWorker) GetWorkloadPreference(url string) (map[string]string, error) {
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

func (w *AgreementWorker) advertiseAllPolicies(location string) error {

	// Wait for blockchain client fully initialized before advertising the policies
	for {
		if w.bcClientInitialized == false {
			time.Sleep(time.Duration(5) * time.Second)
			glog.V(3).Infof("AgreementWorker waiting for blockchain client to be fully initialized.")
		} else {
			break
		}
	}

	var pType, pValue, pCompare string

	policies := w.pm.GetAllPolicies()

	if len(policies) > 0 {
		ms := make([]exchange.Microservice, 0, 10)
		for _, p := range policies {
			newMS := new(exchange.Microservice)
			newMS.Url = p.APISpecs[0].SpecRef
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

			// Make sure whisper is listening for message in all agreement protocols
			for _, agp := range p.AgreementProtocols {
				w.Messages() <- events.NewWhisperSubscribeToMessage(events.SUBSCRIBE_TO, agp.Name)
			}

			ms = append(ms, *newMS)

		}

		pdr := exchange.CreateDevicePut(w.Config.Edge.GethURL, w.deviceToken)
		pdr.RegisteredMicroservices = ms
		var resp interface{}
		resp = new(exchange.PutDeviceResponse)
		targetURL := w.Manager.Config.Edge.ExchangeURL + "devices/" + w.deviceId

		glog.V(3).Infof("AgreementWorker Registering microservices: %v at %v", pdr.ShortString(), targetURL)

		for {
			if err, tpErr := exchange.InvokeExchange(w.httpClient, "PUT", targetURL, w.deviceId, w.deviceToken, pdr, &resp); err != nil {
				return err
			} else if tpErr != nil {
				glog.V(5).Infof(err.Error())
				time.Sleep(10 * time.Second)
				continue
			} else {
				glog.V(3).Infof(logString(fmt.Sprintf("advertised policies for device %v in exchange: %v", w.deviceId, resp)))
				return nil
			}
		}
	}

	return nil
}

func (w *AgreementWorker) recordAgreementState(agreementId string, microservice string, state string) error {

	glog.V(5).Infof(logString(fmt.Sprintf("setting agreement %v state to %v", agreementId, state)))

	as := new(exchange.PutAgreementState)
	as.Microservice = microservice
	as.State = state
	var resp interface{}
	resp = new(exchange.PostDeviceResponse)
	targetURL := w.Manager.Config.Edge.ExchangeURL + "devices/" + w.deviceId + "/agreements/" + agreementId
	for {
		if err, tpErr := exchange.InvokeExchange(w.httpClient, "PUT", targetURL, w.deviceId, w.deviceToken, as, &resp); err != nil {
			glog.Errorf(err.Error())
			return err
		} else if tpErr != nil {
			glog.Warningf(err.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(5).Infof(logString(fmt.Sprintf("set agreement %v to state %v", agreementId, state)))
			return nil
		}
	}

}

var logString = func(v interface{}) string {
	return fmt.Sprintf("AgreementWorker %v", v)
}

var logStringww = func(v interface{}) string {
	return fmt.Sprintf("AgreementWorker whisper maintenance %v", v)
}
