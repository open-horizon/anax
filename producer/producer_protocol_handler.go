package producer

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/abstractprotocol"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/eventlog"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/worker"
	"strings"
	"time"
)

const (
	EL_PROD_AG_EXISTS_IGNORE_PROPOSAL  = "Agreement %v already exists, ignoring proposal: %v"
	EL_PROD_ERR_DEMARSH_TC_FOR_AG      = "received error demarshalling TsAndCs for agrement %v, %v"
	EL_PROD_NODE_REJECTED_PROPOSAL_MSG = "Node received Proposal message using agreement %v for service %v/%v from the agbot %v."
	EL_PROD_NODE_REJECTED_PROPOSAL     = "Node rejected the proposal for service %v/%v."
	EL_PROD_ERR_HANDLE_PROPOSAL        = "Error handling proposal for service %v/%v. Error: %v"
)

// This is does nothing useful at run time.
// This code is only used in compileing time to make the eventlog messages gets into the catalog so that
// they can be translated.
// The event log messages will be saved in English. But the CLI can request them in different languages.
func MarkI18nMessages() {
	// get message printer. anax default language is English
	msgPrinter := i18n.GetMessagePrinter()

	// from api_node.go
	msgPrinter.Sprintf(EL_PROD_AG_EXISTS_IGNORE_PROPOSAL)
	msgPrinter.Sprintf(EL_PROD_ERR_DEMARSH_TC_FOR_AG)
	msgPrinter.Sprintf(EL_PROD_NODE_REJECTED_PROPOSAL_MSG)
	msgPrinter.Sprintf(EL_PROD_NODE_REJECTED_PROPOSAL)
	msgPrinter.Sprintf(EL_PROD_ERR_HANDLE_PROPOSAL)
}

func CreateProducerPH(name string, cfg *config.HorizonConfig, db *bolt.DB, pm *policy.PolicyManager, ec exchange.ExchangeContext) ProducerProtocolHandler {
	if handler := NewBasicProtocolHandler(name, cfg, db, pm, ec); handler != nil {
		return handler
	} // Add new producer side protocol handlers here
	return nil
}

type ProducerProtocolHandler interface {
	Initialize()
	Name() string
	AcceptCommand(cmd worker.Command) bool
	AgreementProtocolHandler(typeName string, name string, org string) abstractprotocol.ProtocolHandler
	HandleProposalMessage(proposal abstractprotocol.Proposal, protocolMsg string, exchangeMsg *exchange.DeviceMessage) bool
	HandleBlockchainEventMessage(cmd *BlockchainEventCommand) (string, bool, uint64, bool, error)
	TerminateAgreement(agreement *persistence.EstablishedAgreement, reason uint)
	GetSendMessage() func(mt interface{}, pay []byte) error
	GetTerminationCode(reason string) uint
	GetTerminationReason(code uint) string
	SetBlockchainClientAvailable(cmd *BCInitializedCommand)
	SetBlockchainClientNotAvailable(cmd *BCStoppingCommand)
	IsBlockchainClientAvailable(typeName string, name string, org string) bool
	SetBlockchainWritable(cmd *BCWritableCommand)
	IsBlockchainWritable(agreement *persistence.EstablishedAgreement) bool
	IsAgreementVerifiable(agreement *persistence.EstablishedAgreement) bool
	HandleExtensionMessages(msg *events.ExchangeDeviceMessage, exchangeMsg *exchange.DeviceMessage) (bool, bool, string, error)
	UpdateConsumer(ag *persistence.EstablishedAgreement)
	UpdateConsumers()
	GetKnownBlockchain(ag *persistence.EstablishedAgreement) (string, string, string)
	VerifyAgreement(ag *persistence.EstablishedAgreement) (bool, error)
}

type BaseProducerProtocolHandler struct {
	name   string
	pm     *policy.PolicyManager
	db     *bolt.DB
	config *config.HorizonConfig
	ec     exchange.ExchangeContext
}

func (w *BaseProducerProtocolHandler) GetSendMessage() func(mt interface{}, pay []byte) error {
	return w.sendMessage
}

func (w *BaseProducerProtocolHandler) Name() string {
	return w.name
}

func (w *BaseProducerProtocolHandler) sendMessage(mt interface{}, pay []byte) error {
	// The mt parameter is an abstract message target object that is passed to this routine
	// by the agreement protocol. It's an interface{} type so that we can avoid the protocol knowing
	// about non protocol types.

	var messageTarget *exchange.ExchangeMessageTarget
	switch mt.(type) {
	case *exchange.ExchangeMessageTarget:
		messageTarget = mt.(*exchange.ExchangeMessageTarget)
	default:
		return errors.New(fmt.Sprintf("input message target is %T, expecting exchange.MessageTarget", mt))
	}

	// Grab the exchange ID of the message receiver
	glog.V(3).Infof(BPPHlogString(w.Name(), fmt.Sprintf("Sending exchange message to: %v, message %v", messageTarget.ReceiverExchangeId, string(pay))))

	// Get my own keys
	myPubKey, myPrivKey, _ := exchange.GetKeys("")

	// Demarshal the receiver's public key if we need to
	if messageTarget.ReceiverPublicKeyObj == nil {
		if mtpk, err := exchange.DemarshalPublicKey(messageTarget.ReceiverPublicKeyBytes); err != nil {
			return errors.New(fmt.Sprintf("Unable to demarshal device's public key %x, error %v", messageTarget.ReceiverPublicKeyBytes, err))
		} else {
			messageTarget.ReceiverPublicKeyObj = mtpk
		}
	}

	// Create an encrypted message
	if encryptedMsg, err := exchange.ConstructExchangeMessage(pay, myPubKey, myPrivKey, messageTarget.ReceiverPublicKeyObj); err != nil {
		return errors.New(fmt.Sprintf("Unable to construct encrypted message from %v, error %v", pay, err))
		// Marshal it into a byte array
	} else if msgBody, err := json.Marshal(encryptedMsg); err != nil {
		return errors.New(fmt.Sprintf("Unable to marshal exchange message %v, error %v", encryptedMsg, err))
		// Send it to the device's message queue
	} else {
		pm := exchange.CreatePostMessage(msgBody, w.config.Edge.ExchangeMessageTTL)
		var resp interface{}
		resp = new(exchange.PostDeviceResponse)
		targetURL := w.config.Edge.ExchangeURL + "orgs/" + exchange.GetOrg(messageTarget.ReceiverExchangeId) + "/agbots/" + exchange.GetId(messageTarget.ReceiverExchangeId) + "/msgs"

		httpClientFactory := w.ec.GetHTTPFactory()
		retryCount := httpClientFactory.RetryCount
		retryInterval := httpClientFactory.GetRetryInterval()

		for {
			if err, tpErr := exchange.InvokeExchange(httpClientFactory.NewHTTPClient(nil), "POST", targetURL, w.ec.GetExchangeId(), w.ec.GetExchangeToken(), pm, &resp); err != nil {
				return err
			} else if tpErr != nil {
				glog.Warningf(tpErr.Error())
				if httpClientFactory.RetryCount == 0 {
					time.Sleep(time.Duration(retryInterval) * time.Second)
					continue
				} else if retryCount == 0 {
					return errors.New(fmt.Sprintf("exceeded %v retries trying to retrieve agbot for %v", httpClientFactory.RetryCount, tpErr))
				} else {
					retryCount--
					time.Sleep(time.Duration(retryInterval) * time.Second)
					continue
				}
			} else {
				glog.V(5).Infof(BPPHlogString(w.Name(), fmt.Sprintf("Sent message for %v to exchange.", messageTarget.ReceiverExchangeId)))
				return nil
			}
		}
	}
}

func (w *BaseProducerProtocolHandler) GetServiceResolver() func(wURL string, wOrg string, wVersion string, wArch string) (*policy.APISpecList, error) {
	return w.serviceResolver
}

func (w *BaseProducerProtocolHandler) serviceResolver(wURL string, wOrg string, wVersion string, wArch string) (*policy.APISpecList, error) {

	asl, _, _, err := exchange.GetHTTPServiceResolverHandler(w.ec)(wURL, wOrg, wVersion, wArch)
	if err != nil {
		glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("unable to resolve %v %v, error %v", wURL, wOrg, err)))
	}
	return asl, err
}

func (w *BaseProducerProtocolHandler) HandleProposal(ph abstractprotocol.ProtocolHandler, proposal abstractprotocol.Proposal, protocolMsg string, runningBCs []map[string]string, exchangeMsg *exchange.DeviceMessage) (bool, abstractprotocol.ProposalReply, *policy.Policy) {

	handled := false

	if agAlreadyExists, err := persistence.FindEstablishedAgreements(w.db, w.Name(), []persistence.EAFilter{persistence.UnarchivedEAFilter(), persistence.IdEAFilter(proposal.AgreementId())}); err != nil {
		glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("unable to retrieve agreements from database, error %v", err)))
	} else if len(agAlreadyExists) != 0 {
		glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("Agreement %v already exists, ignoring proposal: %v", proposal.AgreementId(), proposal.ShortString())))
		eventlog.LogAgreementEvent(
			w.db,
			persistence.SEVERITY_INFO,
			persistence.NewMessageMeta(EL_PROD_AG_EXISTS_IGNORE_PROPOSAL, proposal.AgreementId(), proposal.ShortString()),
			persistence.EC_IGNORE_PROPOSAL,
			agAlreadyExists[0])

		handled = true
	} else if tcPolicy, err := policy.DemarshalPolicy(proposal.TsAndCs()); err != nil {
		glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("received error demarshalling TsAndCs, %v", err)))
		eventlog.LogAgreementEvent2(
			w.db,
			persistence.SEVERITY_ERROR,
			persistence.NewMessageMeta(EL_PROD_ERR_DEMARSH_TC_FOR_AG, proposal.AgreementId(), err.Error()),
			persistence.EC_ERROR_IN_PROPOSAL,
			proposal.AgreementId(),
			persistence.WorkloadInfo{},
			[]persistence.ServiceSpec{},
			proposal.ConsumerId(),
			proposal.Protocol())

	} else {
		var wls, wversion, warch, worg string
		if len(tcPolicy.Workloads) > 0 {
			wls = tcPolicy.Workloads[0].WorkloadURL
			wversion = tcPolicy.Workloads[0].Version
			worg = tcPolicy.Workloads[0].Org
			warch = tcPolicy.Workloads[0].Arch
		}
		eventlog.LogAgreementEvent2(
			w.db,
			persistence.SEVERITY_INFO,
			persistence.NewMessageMeta(EL_PROD_NODE_REJECTED_PROPOSAL_MSG, proposal.AgreementId(), worg, wls, proposal.ConsumerId()),
			persistence.EC_RECEIVED_PROPOSAL,
			proposal.AgreementId(),
			persistence.WorkloadInfo{URL: wls, Org: worg, Version: wversion, Arch: warch},
			ConvertToServiceSpecs(tcPolicy.APISpecs),
			proposal.ConsumerId(),
			proposal.Protocol())

		err_log_event := ""

		if err := w.saveSigningKeys(tcPolicy); err != nil {
			glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("received error handling signing keys from the exchange: %v", err)))
			err_log_event = fmt.Sprintf("Received error handling signing keys from the exchange: %v", err)
			handled = true
		} else if pemFiles, err := w.config.Collaborators.KeyFileNamesFetcher.GetKeyFileNames(w.config.Edge.PublicKeyPath, w.config.UserPublicKeyPath()); err != nil {
			glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("received error getting pem key files: %v", err)))
			err_log_event = fmt.Sprintf("Received error getting pem key files: %v", err)
			handled = true
		} else if err := tcPolicy.Is_Self_Consistent(pemFiles, w.GetServiceResolver()); err != nil {
			glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("received error checking self consistency of TsAndCs, %v", err)))
			err_log_event = fmt.Sprintf("Received error checking self consistency of TsAndCs: %v", err)
			handled = true
		} else if dev, err := persistence.FindExchangeDevice(w.db); err != nil {
			glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("device is not configured to accept agreement yet.")))
			err_log_event = fmt.Sprintf("Device is not configured to accept agreement yet.")
			handled = true
		} else if dev == nil {
			glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("error retrieving device from db: %v", err)))
			err_log_event = fmt.Sprintf("Received error retrieving device from db: %v", err)
			handled = true
		} else if nmatch, err := w.MatchNodeType(tcPolicy, dev); err != nil {
			glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("received error checking node type match, %v", err)))
			err_log_event = fmt.Sprintf("Received error checking node type match, %v", err)
			handled = true
		} else if !nmatch {
			glog.Errorf(BPPHlogString(w.Name(), "node type matching failed, ignoring proposal"))
			err_log_event = "Node type matching failed, ignoring proposal"
			handled = true
		} else if pmatch, err := w.MatchPattern(tcPolicy, dev); err != nil {
			glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("received error checking pattern name match, %v", err)))
			err_log_event = fmt.Sprintf("Received error checking pattern name match, %v", err)
			handled = true
		} else if !pmatch {
			glog.Errorf(BPPHlogString(w.Name(), "pattern name matching failed, ignoring proposal"))
			err_log_event = "Pattern name matching failed, ignoring proposal"
			handled = true
		} else if ag, found, err := w.FindAgreementWithSameWorkload(ph, tcPolicy.Header.Name); err != nil {
			glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("error finding agreement with TsAndCs name '%v', error %v", tcPolicy.Header.Name, err)))
			err_log_event = fmt.Sprintf("Error finding agreement with TsAndCs (Terms And Conditions) name '%v', error %v", tcPolicy.Header.Name, err)
			handled = true
		} else if found {
			glog.Warningf(BPPHlogString(w.Name(), fmt.Sprintf("agreement with TsAndCs name '%v' exists, ignoring proposal: %v", tcPolicy.Header.Name, proposal.ShortString())))
			eventlog.LogAgreementEvent2(
				w.db,
				persistence.SEVERITY_WARN,
				persistence.NewMessageMeta(EL_PROD_AG_EXISTS_IGNORE_PROPOSAL, ag.CurrentAgreementId, wls),
				persistence.EC_IGNORE_PROPOSAL,
				proposal.AgreementId(),
				persistence.WorkloadInfo{URL: wls, Org: worg, Version: wversion, Arch: warch},
				ConvertToServiceSpecs(tcPolicy.APISpecs),
				proposal.ConsumerId(),
				proposal.Protocol())
			handled = true
		} else if messageTarget, err := exchange.CreateMessageTarget(exchangeMsg.AgbotId, nil, exchangeMsg.AgbotPubKey, ""); err != nil {
			glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("error creating message target: %v", err)))
			err_log_event = fmt.Sprintf("Error creating message target: %v", err)
		} else {
			handled = true
			producerPol, err := persistence.FindNodePolicy(w.db)
			if err != nil {
				glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("Error getting node policy from db: %v", err)))
			}
			if r, err := ph.DecideOnProposal(proposal, producerPol, w.ec.GetExchangeId(), exchange.GetOrg(w.ec.GetExchangeId()), runningBCs, messageTarget, w.sendMessage); err != nil {
				glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("respond to proposal with error: %v", err)))
				err_log_event = fmt.Sprintf("Respond to proposal with error: %v", err)
			} else {
				if !r.ProposalAccepted() {
					eventlog.LogAgreementEvent2(
						w.db,
						persistence.SEVERITY_INFO,
						persistence.NewMessageMeta(EL_PROD_NODE_REJECTED_PROPOSAL, worg, wls),
						persistence.EC_REJECT_PROPOSAL,
						proposal.AgreementId(),
						persistence.WorkloadInfo{URL: wls, Org: worg, Version: wversion, Arch: warch},
						ConvertToServiceSpecs(tcPolicy.APISpecs),
						proposal.ConsumerId(),
						proposal.Protocol())
				}
				return handled, r, tcPolicy
			}
		}

		if err_log_event != "" {
			eventlog.LogAgreementEvent2(
				w.db,
				persistence.SEVERITY_ERROR,
				persistence.NewMessageMeta(EL_PROD_ERR_HANDLE_PROPOSAL, worg, wls, err_log_event),
				persistence.EC_ERROR_PROCESSING_PROPOSAL,
				proposal.AgreementId(),
				persistence.WorkloadInfo{URL: wls, Org: worg, Version: wversion, Arch: warch},
				ConvertToServiceSpecs(tcPolicy.APISpecs),
				proposal.ConsumerId(),
				proposal.Protocol())
		}
	}
	return handled, nil, nil
}

// This function gets the pattern and workload's signing keys and save them to anax
func (w *BaseProducerProtocolHandler) saveSigningKeys(pol *policy.Policy) error {
	// do nothing if the config does not allow using the certs from the org on the exchange
	if !w.config.Edge.TrustCertUpdatesFromOrg {
		return nil
	}

	objSigningHandler := exchange.GetHTTPObjectSigningKeysHandler(w.ec)
	errHandler := func(keyname string) api.ErrorHandler {
		return func(err error) bool {
			glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("received error when saving the signing key file %v to anax. %v", keyname, err)))
			return true
		}
	}

	// save signing keys for pattern
	if pol.PatternId != "" {
		if key_map, err := objSigningHandler(exchange.PATTERN, exchange.GetId(pol.PatternId), exchange.GetOrg(pol.PatternId), "", ""); err != nil {
			return fmt.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("received error getting signing keys for pattern from the exchange: %v. %v", pol.PatternId, err)))
		} else if key_map != nil {
			for key, content := range key_map {
				//add .pem the end of the keyname if it does not have none.
				fn := key
				if !strings.HasSuffix(key, ".pem") {
					fn = fmt.Sprintf("%v.pem", key)
				}

				api.UploadPublicKey(fn, []byte(content), w.config, errHandler(fn))
			}
		}
	}

	// save signing keys for services
	if pol.Workloads != nil {
		for _, wl := range pol.Workloads {
			if key_map, err := objSigningHandler(exchange.SERVICE, wl.WorkloadURL, wl.Org, wl.Version, wl.Arch); err != nil {
				return fmt.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("received error getting signing keys for workload from the exchange: %v %v %v %v. %v", wl.WorkloadURL, wl.Org, wl.Version, wl.Arch, err)))
			} else if key_map != nil {
				for key, content := range key_map {
					//add .pem the end of the keyname if it does not have none.
					fn := key
					if !strings.HasSuffix(key, ".pem") {
						fn = fmt.Sprintf("%v.pem", key)
					}

					api.UploadPublicKey(fn, []byte(content), w.config, errHandler(fn))
				}
			}
		}
	}

	return nil
}

// check if the proposal the correct deployment configuration provided for this node
func (w *BaseProducerProtocolHandler) MatchNodeType(tcPolicy *policy.Policy, dev *persistence.ExchangeDevice) (bool, error) {
	if dev == nil {
		return false, fmt.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("device is not configured to accept agreement yet.")))
	} else if tcPolicy.Workloads == nil || len(tcPolicy.Workloads) == 0 {
		glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("no workload is supplied in the proposal.")))
		return false, nil
	} else {
		// make sure that the deployment configuration match the node type
		nodeType := dev.GetNodeType()
		workload := tcPolicy.Workloads[0]
		if nodeType == persistence.DEVICE_TYPE_DEVICE {
			if workload.Deployment == "" {
				if workload.ClusterDeployment == "" {
					glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("no deployment configuration is provided.")))
				} else {
					glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("wrong deployment configuration is provided for the node type '%v'.", nodeType)))
				}
				return false, nil
			}
		} else if nodeType == persistence.DEVICE_TYPE_CLUSTER {
			if workload.ClusterDeployment == "" {
				if workload.Deployment == "" {
					glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("no cluster deployment configuration is provided.")))
				} else {
					glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("wrong deployment configuration is provided for the node type '%v'.", nodeType)))
				}
			}
		}

		glog.V(5).Infof(BPPHlogString(w.Name(), fmt.Sprintf("workload has the correct deployment for the node type '%v'", nodeType)))
		return true, nil
	}
}

// check if the proposal has the same pattern
func (w *BaseProducerProtocolHandler) MatchPattern(tcPolicy *policy.Policy, dev *persistence.ExchangeDevice) (bool, error) {
	if dev == nil {
		return false, fmt.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("device is not configured to accept agreement yet.")))
	} else {
		// the patter id from the proposal is in the format of org/pattern,
		// we need to compose the same thing from device in order to compare
		device_pattern := dev.Pattern

		// compare the patterns from the propodal and device
		if tcPolicy.PatternId != device_pattern {
			glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("pattern from the proposal: '%v' does not match the pattern on the device: '%v'.", tcPolicy.PatternId, device_pattern)))
			return false, nil
		} else {
			glog.V(5).Infof(BPPHlogString(w.Name(), fmt.Sprintf("pattern from the proposal: '%v' matches the pattern on the device: '%v'.", tcPolicy.PatternId, device_pattern)))
			return true, nil
		}
	}
}

// Check if there are current unarchived agreements that have the same workload.
func (w *BaseProducerProtocolHandler) FindAgreementWithSameWorkload(ph abstractprotocol.ProtocolHandler, tcpol_name string) (*persistence.EstablishedAgreement, bool, error) {

	notTerminated := func() persistence.EAFilter {
		return func(a persistence.EstablishedAgreement) bool {
			return a.AgreementTerminatedTime == 0
		}
	}

	if ags, err := persistence.FindEstablishedAgreements(w.db, w.Name(), []persistence.EAFilter{notTerminated(), persistence.UnarchivedEAFilter()}); err != nil {
		return nil, false, fmt.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("error retrieving unarchived agreements from db: %v", err)))
	} else {
		for _, ag := range ags {
			if proposal, err := ph.DemarshalProposal(ag.Proposal); err != nil {
				return nil, false, fmt.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("error demarshalling agreement %v proposal: %v", ag, err)))
			} else if tcPolicy, err := policy.DemarshalPolicy(proposal.TsAndCs()); err != nil {
				return nil, false, fmt.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("error demarshalling agreement %v Producer Policy: %v", ag.CurrentAgreementId, err)))
			} else if tcPolicy.Header.Name == tcpol_name {
				return &ag, true, nil
			}
		}
	}

	return nil, false, nil
}

func (w *BaseProducerProtocolHandler) PersistProposal(proposal abstractprotocol.Proposal, reply abstractprotocol.ProposalReply, tcPolicy *policy.Policy, protocolMsg string) {
	if wi, err := persistence.NewWorkloadInfo(tcPolicy.Workloads[0].WorkloadURL, tcPolicy.Workloads[0].Org, tcPolicy.Workloads[0].Version, tcPolicy.Workloads[0].Arch); err != nil {
		glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("error creating workload info object from %v, error: %v", tcPolicy.Workloads[0], err)))
	} else if _, err := persistence.NewEstablishedAgreement(w.db, tcPolicy.Header.Name, proposal.AgreementId(), proposal.ConsumerId(), protocolMsg, w.Name(), proposal.Version(), ConvertToServiceSpecs(tcPolicy.APISpecs), "", proposal.ConsumerId(), "", "", "", wi); err != nil {
		glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("error persisting new agreement: %v, error: %v", proposal.AgreementId(), err)))
	}
}

func (w *BaseProducerProtocolHandler) TerminateAgreement(ag *persistence.EstablishedAgreement, reason uint, mt interface{}, pph ProducerProtocolHandler) {
	if proposal, err := pph.AgreementProtocolHandler("", "", "").DemarshalProposal(ag.Proposal); err != nil {
		glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("error demarshalling agreement %v proposal: %v", ag.CurrentAgreementId, err)))
	} else if pPolicy, err := policy.DemarshalPolicy(proposal.ProducerPolicy()); err != nil {
		glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("error demarshalling agreement %v Producer Policy: %v", ag.CurrentAgreementId, err)))
	} else {
		bcType, bcName, bcOrg := pph.GetKnownBlockchain(ag)
		if aph := pph.AgreementProtocolHandler(bcType, bcName, bcOrg); aph == nil {
			glog.Warningf(BPPHlogString(w.Name(), fmt.Sprintf("cannot terminate agreement %v, agreement protocol handler doesnt exist yet.", ag.CurrentAgreementId)))
		} else if policies, err := w.pm.GetPolicyList(exchange.GetOrg(w.ec.GetExchangeId()), pPolicy); err != nil {
			glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("agreement %v error getting policy list: %v", ag.CurrentAgreementId, err)))
		} else if err := aph.TerminateAgreement(policies, ag.CounterPartyAddress, ag.CurrentAgreementId, exchange.GetOrg(w.ec.GetExchangeId()), reason, mt, pph.GetSendMessage()); err != nil {
			glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("error terminating agreement %v on the blockchain: %v", ag.CurrentAgreementId, err)))
		}
	}
}

func (w *BaseProducerProtocolHandler) GetAgbotMessageEndpoint(agbotId string) (string, []byte, error) {

	glog.V(5).Infof(BPPHlogString(w.Name(), fmt.Sprintf("retrieving agbot %v msg endpoint from exchange", agbotId)))

	if ag, err := w.getAgbot(agbotId, w.ec.GetExchangeURL(), w.ec.GetExchangeId(), w.ec.GetExchangeToken()); err != nil {
		return "", nil, err
	} else {
		glog.V(5).Infof(BPPHlogString(w.Name(), fmt.Sprintf("retrieved agbot %v msg endpoint from exchange %v", agbotId, ag.MsgEndPoint)))
		return ag.MsgEndPoint, ag.PublicKey, nil
	}

}

func (w *BaseProducerProtocolHandler) getAgbot(agbotId string, url string, deviceId string, token string) (*exchange.Agbot, error) {

	glog.V(5).Infof(BPPHlogString(w.Name(), fmt.Sprintf("retrieving agbot %v from exchange", agbotId)))

	var resp interface{}
	resp = new(exchange.GetAgbotsResponse)
	targetURL := url + "orgs/" + exchange.GetOrg(agbotId) + "/agbots/" + exchange.GetId(agbotId)

	httpClientFactory := w.ec.GetHTTPFactory()
	retryCount := httpClientFactory.RetryCount
	retryInterval := httpClientFactory.GetRetryInterval()

	for {
		if err, tpErr := exchange.InvokeExchange(httpClientFactory.NewHTTPClient(nil), "GET", targetURL, deviceId, token, nil, &resp); err != nil {
			glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf(err.Error())))
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(BPPHlogString(w.Name(), tpErr.Error()))
			if httpClientFactory.RetryCount == 0 {
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			} else if retryCount == 0 {
				return nil, errors.New(fmt.Sprintf("exceeded %v retries trying to retrieve agbot for %v", httpClientFactory.RetryCount, tpErr))
			} else {
				retryCount--
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			}
		} else {
			ags := resp.(*exchange.GetAgbotsResponse).Agbots
			if ag, there := ags[agbotId]; !there {
				return nil, errors.New(fmt.Sprintf("agbot %v not in GET response %v as expected", agbotId, ags))
			} else {
				glog.V(5).Infof(BPPHlogString(w.Name(), fmt.Sprintf("retrieved agbot %v from exchange %v", agbotId, ag)))
				return &ag, nil
			}
		}
	}

}

func (b *BaseProducerProtocolHandler) HandleExtensionMessages(msg *events.ExchangeDeviceMessage, exchangeMsg *exchange.DeviceMessage) (bool, bool, string, error) {
	return false, false, "", nil
}

func (b *BaseProducerProtocolHandler) UpdateConsumer(ag *persistence.EstablishedAgreement) {}

func (b *BaseProducerProtocolHandler) UpdateConsumers() {}

func (c *BaseProducerProtocolHandler) SetBlockchainClientAvailable(cmd *BCInitializedCommand) {
	return
}

func (c *BaseProducerProtocolHandler) SetBlockchainClientNotAvailable(cmd *BCStoppingCommand) {
	return
}

func (c *BaseProducerProtocolHandler) GetKnownBlockchain(ag *persistence.EstablishedAgreement) (string, string, string) {
	return "", "", ""
}

// The list of termination reasons that should be supported by all agreement protocols. The caller can pass these into
// the GetTerminationCode API to get a protocol specific reason code for that termination reason.
const TERM_REASON_POLICY_CHANGED = "PolicyChanged"
const TERM_REASON_AGBOT_REQUESTED = "ConsumerCancelled"
const TERM_REASON_CONTAINER_FAILURE = "ContainerFailure"

const TERM_REASON_USER_REQUESTED = "UserRequested"
const TERM_REASON_NOT_FINALIZED_TIMEOUT = "NotFinalized"
const TERM_REASON_NO_REPLY_ACK = "NoReplyAck"
const TERM_REASON_NOT_EXECUTED_TIMEOUT = "NotExecuted"
const TERM_REASON_MICROSERVICE_FAILURE = "MicroserviceFailure"
const TERM_REASON_WL_IMAGE_LOAD_FAILURE = "WorkloadImageLoadFailure"
const TERM_REASON_MS_IMAGE_LOAD_FAILURE = "MicroserviceImageLoadFailure"
const TERM_REASON_MS_IMAGE_FETCH_FAILURE = "MicroserviceImageFetchFailure"
const TERM_REASON_MS_UPGRADE_REQUIRED = "MicroserviceUpgradeRequired"
const TERM_REASON_MS_DOWNGRADE_REQUIRED = "MicroserviceDowngradeRequired"
const TERM_REASON_IMAGE_DATA_ERROR = "ImageDataError"
const TERM_REASON_IMAGE_FETCH_FAILURE = "ImageFetchFailure"
const TERM_REASON_IMAGE_FETCH_AUTH_FAILURE = "ImageFetchAuthorizationFailure"
const TERM_REASON_IMAGE_SIG_VERIF_FAILURE = "ImageSignatureVerificationFailure"
const TERM_REASON_NODE_SHUTDOWN = "NodeShutdown"
const TERM_REASON_SERVICE_SUSPENDED = "ServiceSuspended"
const TERM_REASON_NODE_USERINPUT_CHANGED = "NodeUserInputChanged"
const TERM_REASON_NODE_PATTERN_CHANGED = "NodePatternChanged"

// ==============================================================================================================
type ExchangeMessageCommand struct {
	Msg events.ExchangeDeviceMessage
}

func (e ExchangeMessageCommand) String() string {
	return e.Msg.ShortString()
}

func (e ExchangeMessageCommand) ShortString() string {
	return e.Msg.ShortString()
}

func NewExchangeMessageCommand(msg events.ExchangeDeviceMessage) *ExchangeMessageCommand {
	return &ExchangeMessageCommand{
		Msg: msg,
	}
}

// ==============================================================================================================
type BlockchainEventCommand struct {
	Msg events.EthBlockchainEventMessage
}

func (e BlockchainEventCommand) ShortString() string {
	return e.Msg.ShortString()
}

func NewBlockchainEventCommand(msg events.EthBlockchainEventMessage) *BlockchainEventCommand {
	return &BlockchainEventCommand{
		Msg: msg,
	}
}

// ==============================================================================================================
type BCInitializedCommand struct {
	Msg *events.BlockchainClientInitializedMessage
}

func (c BCInitializedCommand) ShortString() string {

	return fmt.Sprintf("BCInitializedCommand: Msg %v", c.Msg)
}

func NewBCInitializedCommand(msg *events.BlockchainClientInitializedMessage) *BCInitializedCommand {
	return &BCInitializedCommand{
		Msg: msg,
	}
}

// ==============================================================================================================
type BCStoppingCommand struct {
	Msg *events.BlockchainClientStoppingMessage
}

func (c BCStoppingCommand) ShortString() string {

	return fmt.Sprintf("BCStoppingCommand: Msg %v", c.Msg)
}

func NewBCStoppingCommand(msg *events.BlockchainClientStoppingMessage) *BCStoppingCommand {
	return &BCStoppingCommand{
		Msg: msg,
	}
}

// ==============================================================================================================
type BCWritableCommand struct {
	Msg events.AccountFundedMessage
}

func (c BCWritableCommand) ShortString() string {

	return fmt.Sprintf("BCWritableCommand: Msg %v", c.Msg)
}

func NewBCWritableCommand(msg *events.AccountFundedMessage) *BCWritableCommand {
	return &BCWritableCommand{
		Msg: *msg,
	}
}

// ==========================================================================================================
// Utility functions

var BPPHlogString = func(p string, v interface{}) string {
	return fmt.Sprintf("Base Producer Protocol Handler (%v): %v", p, v)
}

// This function converts an array of APISpecification to an array of ServiceSpec
func ConvertToServiceSpecs(apiSpecs policy.APISpecList) persistence.ServiceSpecs {
	sps := make([]persistence.ServiceSpec, 0)
	if apiSpecs != nil && len(apiSpecs) > 0 {
		for _, as := range apiSpecs {
			sps = append(sps, persistence.ServiceSpec{Url: as.SpecRef, Org: as.Org})
		}
	}
	return sps
}
