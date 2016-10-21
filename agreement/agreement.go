package agreement

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/citizenscientist"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/worker"
	"net/http"
	"net/url"
	"os"
	"reflect"
	gpolicy "repo.hovitos.engineering/MTN/go-policy"
	gwhisper "repo.hovitos.engineering/MTN/go-whisper"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// must be safely-constructed!!
type AgreementWorker struct {
	worker.Worker // embedded field
	db            *bolt.DB
	httpClient    *http.Client
	userId        string
	deviceId      string
	deviceToken   string
	protocols     map[string]bool
	pm            *gpolicy.PolicyManager
}

func NewAgreementWorker(config *config.HorizonConfig, db *bolt.DB) *AgreementWorker {
	messages := make(chan events.Message)
	commands := make(chan worker.Command, 100)

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
		w.Commands <- NewDeviceRegisteredCommand(msg.ID(), msg.Token())

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

		// If the message's topic is not one we care about then ignore the message
		// if len(w.protocols) != 0 {
		// if _, ok := w.protocols[msg.Topics()[0]]; ok {
		agCmd := NewReceivedProposalCommand(*msg)
		w.Commands <- agCmd
		// }
		// }

	default: //nothing
	}

	return
}

func (w *AgreementWorker) start() {

	glog.Info(logString(fmt.Sprintf("started")))

	w.Commands <- NewInitEdgeCommand()

	// Enter the command processing loop. Initialization is complete so wait for commands to
	// perform. Commands are created as the result of events that are triggered elsewhere
	// in the system.
	go func() {

		for {
			glog.V(2).Infof(logString(fmt.Sprintf("blocking for commands")))
			command := <-w.Commands
			glog.V(2).Infof(logString(fmt.Sprintf("received command: %T", command)))

			switch command.(type) {
			case *InitEdgeCommand:
				w.handleInitAnax()

			case *DeviceRegisteredCommand:
				cmd, _ := command.(*DeviceRegisteredCommand)
				w.handleDeviceRegistered(cmd)

			case *TerminateCommand:
				cmd, _ := command.(*TerminateCommand)
				glog.Errorf(logString(fmt.Sprintf("terminating, reason: %v", cmd.reason)))
				return

			case *AdvertisePolicyCommand:

				// Give the policy manager a chance to read in all the policies.
				if policyManager, err := gpolicy.Initialize(w.Worker.Manager.Config.Edge.PolicyPath, ""); err != nil {
					glog.Fatalf(logString(fmt.Sprintf("unable to inialize policy manager, error: %v", err)))
					return
				} else {
					w.pm = policyManager
				}

				// Capture all the agreement protocols that we might be using, and the tell the whisper subsystem to get ready
				w.protocols = w.pm.GetAllAgreementProtocols()
				for protocolName, _ := range w.protocols {
					w.Messages() <- events.NewWhisperSubscribeToMessage(events.SUBSCRIBE_TO, protocolName)
					// go w.InitiateAgreementProtocolHandler(protocolName)
				}

				// Publish what we have for the world to see
				if err := w.advertiseAllPolicies(w.Worker.Manager.Config.Edge.PolicyPath); err != nil {
					glog.Fatalf(logString(fmt.Sprintf("unable to advertise policies with exchange, error: %v", err)))
				}

			case *ReceivedProposalCommand:
				cmd, _ := command.(*ReceivedProposalCommand)

				protocolHandler := citizenscientist.NewProtocolHandler(w.Config.Edge.GethURL, w.pm)

				if proposal, err := protocolHandler.ValidateProposal(cmd.Msg.Payload()); err != nil {
					glog.Errorf(logString(fmt.Sprintf("discarding message: %v", cmd.Msg.Payload())))
				} else if _, err := persistence.NewEstablishedAgreement(w.db, proposal.AgreementId, cmd.Msg.Payload(), "Citizen Scientist"); err != nil {
					glog.Errorf(logString(fmt.Sprintf("persisting new pending agreement: %v", proposal.AgreementId)))
				} else if reply, err := protocolHandler.DecideOnProposal(proposal, cmd.Msg.From()); err != nil {
					glog.Errorf(logString(fmt.Sprintf("unable to respond to proposal, error: %v", err)))
				} else if err := w.RecordReply(proposal, reply); err != nil {
					glog.Errorf(logString(fmt.Sprintf("unable to record reply %v, error: %v", *reply, err)))
				}

				if proposal, err := protocolHandler.ValidateProposal(cmd.Msg.Payload()); err != nil {
					glog.Errorf(logString(fmt.Sprintf("discarding message: %v", cmd.Msg.Payload())))
				} else if _, err := persistence.NewEstablishedAgreement(w.db, proposal.AgreementId, cmd.Msg.Payload(), citizenscientist.PROTOCOL_NAME); err != nil {
					glog.Errorf(logString(fmt.Sprintf("persisting new pending agreement: %v", proposal.AgreementId)))
				} else if reply, err := protocolHandler.DecideOnProposal(proposal, cmd.Msg.From()); err != nil {
					glog.Errorf(logString(fmt.Sprintf("unable to respond to proposal, error: %v", err)))
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

func (w *AgreementWorker) handleInitAnax() {

	// If there is no edge config then there is nothing to init on the edge
	if w.Worker.Manager.Config.Edge == (config.Config{}) {
		return
	}

	// Make sure the policy file directories exist
	if err := os.MkdirAll(w.Worker.Manager.Config.Edge.PolicyPath, 0644); err != nil {
		reason := fmt.Sprintf("Cannot create anax policy file path %v", w.Worker.Manager.Config.Edge.PolicyPath)
		glog.Errorf(logString(fmt.Sprintf("terminating, %v", reason)))
		w.Commands <- NewTerminateCommand(reason)
		return
	}

}

func (w *AgreementWorker) handleDeviceRegistered(cmd *DeviceRegisteredCommand) {

	w.deviceId = cmd.Id
	w.deviceToken = cmd.Token

	// Start the go thread that heartbeats to the exchange
	targetURL := w.Manager.Config.Edge.ExchangeURL + "devices/" + w.deviceId + "/heartbeat?token=" + w.deviceToken
	go exchange.Heartbeat(w.httpClient, targetURL, w.Worker.Manager.Config.Edge.ExchangeHeartbeat)

}

func (w *AgreementWorker) RecordReply(proposal *citizenscientist.Proposal, reply *citizenscientist.ProposalReply, protocol string, cmd *ReceivedProposalCommand) error {

	if reply != nil {
		// Update the state in the database
		if _, err := persistence.AgreementStateAccepted(w.db, proposal.AgreementId, protocol, cmd.Msg.Payload(), proposal.Address, reply.Signature); err != nil {
			return errors.New(logString(fmt.Sprintf("received error updating database state, %v", err)))

			// Update the state in the exchange
		} else if tcPolicy, err := gpolicy.DemarshalPolicy(proposal.TsAndCs); err != nil {
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
				envAdds["MTN_AGREEMENTID"] = proposal.AgreementId
				envAdds["MTN_CONTRACT"] = tcPolicy.Header.Name
				lc.EnvironmentAdditions = &envAdds
				w.Worker.Manager.Messages <- events.NewAgreementMessage(events.AGREEMENT_REACHED, lc)
			}
		}

	} else {
		if err := persistence.DeleteEstablishedAgreement(w.db, proposal.AgreementId, protocol); err != nil {
			return errors.New(logString(fmt.Sprintf("received error deleting agreement from db %v", err)))
		} else if err := w.recordAgreementState(proposal.AgreementId, "", "Reject proposal"); err != nil {
			return errors.New(logString(fmt.Sprintf("received error setting state for agreement %v", err)))
		}
	}

	return nil
}

// ===============================================================================================
// Utility functions
//

// get the environmental variables for the workload
func (w *AgreementWorker) GetWorkloadPreference(url string) (map[string]string, error) {
	environmentAdditions := make(map[string]string, 0)

	if pcs, err := persistence.FindPendingContractByFilters(w.db, []persistence.PCFilter{persistence.SensorUrlPCFilter(url)}); err != nil {
		return nil, fmt.Errorf("Error getting workload preference: %v", err)
	} else {
		if len(pcs) > 0 {
			pc := pcs[0]
			// get common attributes
			environmentAdditions["MTN_NAME"] = *pc.Name
			environmentAdditions["MTN_ARCH"] = pc.Arch
			environmentAdditions["MTN_CPUS"] = strconv.Itoa(pc.CPUs)
			environmentAdditions["MTN_RAM"] = strconv.Itoa(*pc.RAM)
			environmentAdditions["MTN_IS_LOC_ENABLED"] = strconv.FormatBool(pc.IsLocEnabled)
			if pc.Lat != nil {
				environmentAdditions["MTN_LAT"] = *pc.Lat
			}
			if pc.Lon != nil {
				environmentAdditions["MTN_LON"] = *pc.Lon
			}

			// get public attributes
			if pc.AppAttributes != nil {
				for key, val := range *pc.AppAttributes {
					if val != "" {
						environmentAdditions[fmt.Sprintf("MTN_%s", strings.ToUpper(key))] = val
					}
				}
			}

			// get private attributes
			if pc.PrivateAppAttributes != nil {
				for key, val := range *pc.PrivateAppAttributes {
					if val != "" {
						environmentAdditions[fmt.Sprintf("MTN_%s", strings.ToUpper(key))] = val
					}
				}
			}
		}
	}
	return environmentAdditions, nil
}

func (w *AgreementWorker) advertiseAllPolicies(location string) error {

	var pType, pValue, pCompare string

	if policies, err := policy.GetAllPolicies(location); err != nil {
		return errors.New(fmt.Sprintf("AgreementWorker received error reading in policy files: %v", err))
	} else if len(policies) > 0 {
		ms := make([]exchange.Microservice, 0, 10)
		for _, p := range policies {
			newMS := new(exchange.Microservice)
			newMS.Url = p.APISpecs[0].SpecRef
			newMS.NumAgreements = 1

			if pBytes, err := json.Marshal(p); err != nil {
				return errors.New(fmt.Sprintf("AgreementWorker received error marshalling policy: %v", err))
			} else {
				newMS.Policy = string(pBytes)
			}

			if props, err := policy.RetrieveAllProperties(p); err != nil {
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

		pdr := exchange.CreateDevicePut(w.Config.Edge.GethURL)
		pdr.RegisteredMicroservices = ms
		var resp interface{}
		resp = new(exchange.PutDeviceResponse)
		targetURL := w.Manager.Config.Edge.ExchangeURL + "devices/" + w.deviceId + "?token=" + w.deviceToken

		for {
			if err, tpErr := exchange.InvokeExchange(w.httpClient, "PUT", targetURL, pdr, &resp); err != nil {
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
	targetURL := w.Manager.Config.Edge.ExchangeURL + "devices/" + w.deviceId + "/agreements/" + agreementId + "?token=" + w.deviceToken
	for {
		if err, tpErr := exchange.InvokeExchange(w.httpClient, "PUT", targetURL, as, &resp); err != nil {
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
