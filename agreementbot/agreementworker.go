package agreementbot

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/abstractprotocol"
	"github.com/open-horizon/anax/agreementbot/persistence"
	"github.com/open-horizon/anax/agreementbot/secrets"
	"github.com/open-horizon/anax/common"
	"github.com/open-horizon/anax/compcheck"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/worker"
	"golang.org/x/text/message"
	"math/rand"
	"net/http"
	"strings"
)

// These structs are the event bodies that flow from the processor to the agreement workers
const INITIATE = "INITIATE_AGREEMENT"
const REPLY = "AGREEMENT_REPLY"
const CANCEL = "AGREEMENT_CANCEL"
const DATARECEIVEDACK = "AGREEMENT_DATARECEIVED_ACK"
const WORKLOAD_UPGRADE = "WORKLOAD_UPGRADE"
const ASYNC_CANCEL = "ASYNC_CANCEL"
const MMS_OBJECT_POLICY = "MMS_OBJECT_POLICY"
const STOP = "PROTOCOL_WORKER_STOP"

type AgreementWork interface {
	Type() string
	ShortString() string
}

type InitiateAgreement struct {
	workType               string
	ProducerPolicy         policy.Policy                            // the producer policy received from the exchange - demarshalled
	OriginalProducerPolicy string                                   // the producer policy received from the exchange - original in string form to be sent back
	ConsumerPolicy         policy.Policy                            // the consumer policy we're matched up with - this is a copy so that we can modify/augment it
	Org                    string                                   // the org from which the consumer policy originated
	Device                 exchange.SearchResultDevice              // the device entry in the exchange
	ConsumerPolicyName     string                                   // the name of the consumer policy in the exchange
	ServicePolicies        map[string]externalpolicy.ExternalPolicy // cached service polices, keyed by service id. it is a subset of the service versions in the consumer policy file
}

func NewInitiateAgreement(pPolicy policy.Policy, cPolicy policy.Policy, org string, device exchange.SearchResultDevice, cpName string, sPols map[string]externalpolicy.ExternalPolicy) AgreementWork {
	return InitiateAgreement{
		workType:           INITIATE,
		ProducerPolicy:     pPolicy,
		ConsumerPolicy:     cPolicy,
		Org:                org,
		Device:             device,
		ConsumerPolicyName: cpName,
		ServicePolicies:    sPols,
	}
}

func (c InitiateAgreement) String() string {
	res := ""
	res += fmt.Sprintf("Workitem: %v,  Org: %v\n", c.workType, c.Org)
	res += fmt.Sprintf("Producer Policy: %v\n", c.ProducerPolicy)
	res += fmt.Sprintf("Consumer Policy: %v\n", c.ConsumerPolicy)
	res += fmt.Sprintf("Device: %v", c.Device)
	res += fmt.Sprintf("ConsumerPolicyName: %v", c.ConsumerPolicyName)
	res += fmt.Sprintf("ServicePolicies: %v", c.ServicePolicies)
	return res
}

func (c InitiateAgreement) ShortString() string {
	servicePolKeys := []string{}
	if c.ServicePolicies != nil {
		for k, _ := range c.ServicePolicies {
			servicePolKeys = append(servicePolKeys, k)
		}
	}

	return fmt.Sprintf("Workitem: %v, Org: %v, Device: %v, ConsumerPolicyName: %v, Producer Policy: %v, Consumer Policy: %v, ServicePolicies: %v",
		c.workType, c.Org, c.Device, c.ConsumerPolicyName, c.ProducerPolicy.ShortString(), c.ConsumerPolicy.ShortString(), servicePolKeys)
}

func (c InitiateAgreement) Type() string {
	return c.workType
}

type HandleReply struct {
	workType     string
	Reply        abstractprotocol.ProposalReply
	From         string // deprecated whisper address
	SenderId     string // exchange Id of sender
	SenderPubKey []byte
	MessageId    int
}

func NewHandleReply(reply abstractprotocol.ProposalReply, senderId string, senderPubKey []byte, messageId int) AgreementWork {
	return HandleReply{
		workType:     REPLY,
		Reply:        reply,
		SenderId:     senderId,
		SenderPubKey: senderPubKey,
		MessageId:    messageId,
	}
}

func (c HandleReply) String() string {
	return fmt.Sprintf("Workitem: %v, SenderId: %v, MessageId: %v, From: %v, Reply: %v, SenderPubKey: %x", c.workType, c.SenderId, c.MessageId, c.From, c.Reply, c.SenderPubKey)
}

func (c HandleReply) ShortString() string {
	senderPubKey := ""
	if c.SenderPubKey != nil {
		senderPubKey = cutil.TruncateDisplayString(string(c.SenderPubKey), 10)
	}
	return fmt.Sprintf("Workitem: %v, SenderId: %v, MessageId: %v, From: %v, Reply: %v, SenderPubKey: %x", c.workType, c.SenderId, c.MessageId, c.From, c.Reply, senderPubKey)
}

func (c HandleReply) Type() string {
	return c.workType
}

type HandleDataReceivedAck struct {
	workType     string
	Ack          string
	From         string // deprecated whisper address
	SenderId     string // exchange Id of sender
	SenderPubKey []byte
	MessageId    int
}

func NewHandleDataReceivedAck(ack string, senderId string, senderPubKey []byte, messageId int) AgreementWork {
	return HandleDataReceivedAck{
		workType:     DATARECEIVEDACK,
		Ack:          ack,
		SenderId:     senderId,
		SenderPubKey: senderPubKey,
		MessageId:    messageId,
	}
}

func (c HandleDataReceivedAck) String() string {
	return fmt.Sprintf("Workitem: %v, SenderId: %v, MessageId: %v, From: %v, Ack: %v, SenderPubKey: %x", c.workType, c.SenderId, c.MessageId, c.From, c.Ack, c.SenderPubKey)
}

func (c HandleDataReceivedAck) ShortString() string {
	senderPubKey := ""
	if c.SenderPubKey != nil {
		senderPubKey = cutil.TruncateDisplayString(string(c.SenderPubKey), 10)
	}
	return fmt.Sprintf("Workitem: %v, SenderId: %v, MessageId: %v, From: %v, Ack: %v, SenderPubKey: %x", c.workType, c.SenderId, c.MessageId, c.From, c.Ack, senderPubKey)
}

func (c HandleDataReceivedAck) Type() string {
	return c.workType
}

type CancelAgreement struct {
	workType    string
	AgreementId string
	Protocol    string
	Reason      uint
	MessageId   int
}

func (c CancelAgreement) Type() string {
	return c.workType
}

func (c CancelAgreement) ShortString() string {
	return fmt.Sprintf("Workitem: %v, AgreementId: %v, Protocol: %v, Reason: %v, MessageId: %v", c.workType, c.AgreementId, c.Protocol, c.Reason, c.MessageId)
}

func NewCancelAgreement(agId string, protocol string, reason uint, messageId int) AgreementWork {
	return CancelAgreement{
		workType:    CANCEL,
		AgreementId: agId,
		Protocol:    protocol,
		Reason:      reason,
		MessageId:   messageId,
	}
}

type HandleWorkloadUpgrade struct {
	workType    string
	AgreementId string
	Protocol    string
	Device      string
	PolicyName  string
}

func NewHandleWorkloadUpgrade(agId string, protocol string, device string, policyName string) AgreementWork {
	return HandleWorkloadUpgrade{
		workType:    WORKLOAD_UPGRADE,
		AgreementId: agId,
		Device:      device,
		Protocol:    protocol,
		PolicyName:  policyName,
	}
}

func (c HandleWorkloadUpgrade) Type() string {
	return c.workType
}

func (c HandleWorkloadUpgrade) ShortString() string {
	return fmt.Sprintf("Workitem: %v, AgreementId: %v, Device: %v, Protocol: %v, PolicyName: %v", c.workType, c.AgreementId, c.Device, c.Protocol, c.PolicyName)
}

type AsyncCancelAgreement struct {
	workType    string
	AgreementId string
	Protocol    string
	Reason      uint
}

func (c AsyncCancelAgreement) Type() string {
	return c.workType
}

func (c AsyncCancelAgreement) ShortString() string {
	return fmt.Sprintf("Workitem: %v, AgreementId: %v, Protocol: %v, Reason: %v", c.workType, c.AgreementId, c.Protocol, c.Reason)
}

type ObjectPolicyChange struct {
	workType string
	Event    events.MMSObjectPolicyMessage
}

func NewObjectPolicyChange(event events.MMSObjectPolicyMessage) AgreementWork {
	return ObjectPolicyChange{
		workType: MMS_OBJECT_POLICY,
		Event:    event,
	}
}

func (c ObjectPolicyChange) Type() string {
	return c.workType
}

func (c ObjectPolicyChange) ShortString() string {
	return fmt.Sprintf("Workitem: %v, Event: %v", c.workType, c.Event)
}

type StopWorker struct {
	workType string
}

func NewStopWorker() AgreementWork {
	return StopWorker{
		workType: STOP,
	}
}

func (c StopWorker) String() string {
	return fmt.Sprintf("Workitem: %v", c.workType)
}

func (c StopWorker) ShortString() string {
	return fmt.Sprintf("Workitem: %v", c.workType)
}

func (c StopWorker) Type() string {
	return c.workType
}

type AgreementWorker interface {
	AgreementLockManager() *AgreementLockManager
}

type BaseAgreementWorker struct {
	pm         *policy.PolicyManager
	db         persistence.AgbotDatabase
	config     *config.HorizonConfig
	alm        *AgreementLockManager
	workerID   string
	httpClient *http.Client
	ec         *worker.BaseExchangeContext
	mmsObjMgr  *MMSObjectPolicyManager
	secretsMgr secrets.AgbotSecrets
}

// A local implementation of the ExchangeContext interface because Agbot agreement workers are not full featured workers.
func (b *BaseAgreementWorker) GetExchangeId() string {
	if b.ec != nil {
		return b.ec.Id
	} else {
		return ""
	}
}

func (b *BaseAgreementWorker) GetExchangeToken() string {
	if b.ec != nil {
		return b.ec.Token
	} else {
		return ""
	}
}

func (b *BaseAgreementWorker) GetExchangeURL() string {
	if b.ec != nil {
		return b.ec.URL
	} else {
		return ""
	}
}

func (b *BaseAgreementWorker) GetCSSURL() string {
	if b.ec != nil {
		return b.ec.CSSURL
	} else {
		return ""
	}
}

func (b *BaseAgreementWorker) GetHTTPFactory() *config.HTTPClientFactory {
	if b.ec != nil {
		return b.ec.HTTPFactory
	} else {
		return b.config.Collaborators.HTTPClientFactory
	}
}

func (b *BaseAgreementWorker) AgreementLockManager() *AgreementLockManager {
	return b.alm
}

func (b *BaseAgreementWorker) checkPolicyCompatibility(workerId string, wi *InitiateAgreement, nodePolicy *policy.Policy, businessPolicy *policy.Policy, mergedServicePolicy *externalpolicy.ExternalPolicy, nodeArch string, msgPrinter *message.Printer) (bool, *policy.Policy, error) {

	if compatible, reason, _, consumPol, err := compcheck.CheckPolicyCompatiblility(nodePolicy, businessPolicy, mergedServicePolicy, "", msgPrinter); err != nil {
		glog.Warning(BAWlogstring(workerId, fmt.Sprintf("error checking policy compatibility. %v.", err.Error())))
		return false, nil, err
	} else {
		if compatible {
			if glog.V(5) {
				glog.Infof(BAWlogstring(workerId, fmt.Sprintf("Node %v is compatible", wi.Device.Id)))
			}
			return true, consumPol, nil
		} else {
			glog.Warningf(BAWlogstring(workerId, fmt.Sprintf("failed matching node policy %v and %v, error: %v", wi.ProducerPolicy, wi.ConsumerPolicy, reason)))
			return false, nil, nil
		}
	}
}

func (b *BaseAgreementWorker) InitiateNewAgreement(cph ConsumerProtocolHandler, wi *InitiateAgreement, random *rand.Rand, workerId string) {

	msgPrinter := i18n.GetMessagePrinter()

	// get node policy
	nodePolicyHandler := exchange.GetHTTPNodePolicyHandler(b)
	_, nodePolicy, err := compcheck.GetNodePolicy(nodePolicyHandler, wi.Device.Id, msgPrinter)
	if err != nil {
		glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("%v", err)))
		return
	} else if nodePolicy == nil {
		glog.Warning(BAWlogstring(workerId, fmt.Sprintf("Cannot find node policy for this node %v.", wi.Device.Id)))
		return
	} else {
		if glog.V(5) {
			glog.Infof(BAWlogstring(workerId, fmt.Sprintf("retrieved node policy: %v", nodePolicy)))
		}
	}

	// If a deployment policy is being used, set wi.ProducerPolicy to the node policy
	if wi.ConsumerPolicy.PatternId == "" {
		// non pattern case

		// If a deployment policy is being used and multiple service versions are possible, do an initial check of just the policy constraints of the deployment policy
		// with the node properties to see if those match before we get too far invested in checking matches of all the different service versions.
		// In the case were have thousands of deployment policies, this can avoid lots of calls to check and create workload_usages in the DB if there isn't a match at this level
		//
		// If the node policy has constraints, we can't do the check without the service properties so we need to find the workload.
		// Also, if there is only 1 workload possibility, skip the check here and just do the check with the service included to avoid checking twice
		// for the case where the node does match the policy
		if len(nodePolicy.Constraints) == 0 && len(wi.ConsumerPolicy.Workloads) > 1 {
			EmptySvcPolicy := externalpolicy.ExternalPolicy{
				Properties:  []externalpolicy.Property{},
				Constraints: []string{},
			}

			compatible, _, _ := b.checkPolicyCompatibility(workerId, wi, nodePolicy, &wi.ConsumerPolicy, &EmptySvcPolicy, "", msgPrinter)
			if !compatible {
				// Not compatible with the constraints of the deployment policy so no need to continue checking with the service versions
				return
			}
		}
		wi.ProducerPolicy = *nodePolicy
	}

	// Generate an agreement ID
	agreementIdString, aerr := cutil.GenerateAgreementId()
	if aerr != nil {
		glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error generating agreement id %v", aerr)))
		return
	}
	if glog.V(5) {
		glog.Infof(BAWlogstring(workerId, fmt.Sprintf("using AgreementId %v", agreementIdString)))
	}

	bcType, bcName, bcOrg := (&wi.ProducerPolicy).RequiresKnownBC(cph.Name())

	// Use the blockchain name to choose the handler
	protocolHandler := cph.AgreementProtocolHandler(bcType, bcName, bcOrg)
	if protocolHandler == nil {
		glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("agreement protocol handler is not ready yet for %v %v", bcType, bcName)))
		return
	}

	// Get the agreement id lock to prevent any other thread from processing this same agreement.
	lock := b.alm.getAgreementLock(agreementIdString)
	lock.Lock()
	defer lock.Unlock()

	// The device object we're working with might not include the policies for the services needed by the
	// workload in the current consumer policy. If that's the case, query the exchange to get all the device
	// policies so we can merge them.
	var exchangeDev *exchange.Device
	if theDev, err := GetDevice(b.config.Collaborators.HTTPClientFactory.NewHTTPClient(nil), wi.Device.Id, b.config.AgreementBot.ExchangeURL, cph.GetExchangeId(), cph.GetExchangeToken()); err != nil {
		glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error getting device %v policies, error: %v", wi.Device.Id, err)))
		return
	} else {
		exchangeDev = theDev
	}

	// get the node type for later use
	nodeType := wi.Device.GetNodeType()

	// There could be more than 1 workload version in the consumer policy, and each version might NOT require the exact same
	// services/microservices (and versions), so we first need to choose a workload. Choosing a workload is based on the priority of
	// each workload and whether or not this workload has been tried before. Also, iterate the loop more than once if we choose
	// a workload entry that turns out to be unsupportable by the device.
	foundWorkload := false
	var workload, lastWorkload *policy.Workload
	svcIds := []string{} // stores the service ids for all the services, top level and dependent services
	found := true        // if the service policy can be found from the businesspol_manager
	var servicePol *externalpolicy.ExternalPolicy
	var wlUsage *persistence.WorkloadUsage = nil

	for !foundWorkload {

		//If number of workloads > 1, then we need to utilize the workload_usages to find the best workload
		if len(wi.ConsumerPolicy.Workloads) > 1 {
			if wlUsage, err = b.db.FindSingleWorkloadUsageByDeviceAndPolicyName(wi.Device.Id, wi.ConsumerPolicy.Header.Name); err != nil {
				glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error searching for persistent workload usage records for device %v with policy %v, error: %v", wi.Device.Id, wi.ConsumerPolicy.Header.Name, err)))
				return
			}
		}

		if wlUsage == nil {
			workload = wi.ConsumerPolicy.NextHighestPriorityWorkload(0, 0, 0)
		} else if wlUsage.DisableRetry {
			workload = wi.ConsumerPolicy.NextHighestPriorityWorkload(wlUsage.Priority, 0, wlUsage.FirstTryTime)
		} else if wlUsage != nil {
			workload = wi.ConsumerPolicy.NextHighestPriorityWorkload(wlUsage.Priority, wlUsage.RetryCount+1, wlUsage.FirstTryTime)
		}

		// If we chose the same workload 2 times in a row through this loop, then we need to exit out of here
		// Added second comparison in case the workload pointer got changed by the policy merger
		if (lastWorkload == workload) || (lastWorkload != nil && workload != nil && lastWorkload.IsSame(*workload)) {
			glog.Warningf(BAWlogstring(workerId, fmt.Sprintf("unable to find supported workload for %v within %v", wi.Device.Id, wi.ConsumerPolicy.Workloads)))

			if workload != nil && !workload.HasEmptyPriority() {
				// If we created a workload usage record during this process, get rid of it.
				if err := b.db.DeleteWorkloadUsage(wi.Device.Id, wi.ConsumerPolicy.Header.Name); err != nil {
					glog.Warningf(BAWlogstring(workerId, fmt.Sprintf("unable to delete workload usage record for %v with %v because %v", wi.Device.Id, wi.ConsumerPolicy.Header.Name, err)))
				}
			}
			return

		}

		// If the service is suspended, then do not make an agreement.
		if found, suspended := exchange.ServiceSuspended(exchangeDev.RegisteredServices, workload.WorkloadURL, workload.Org, workload.Version); found && suspended {
			glog.Infof(BAWlogstring(workerId, fmt.Sprintf("cannot make agreement with %v for policy %v because service %v version %v is suspended by the user.", wi.Device.Id, wi.ConsumerPolicy.Header.Name, cutil.FormOrgSpecUrl(workload.WorkloadURL, workload.Org), workload.Version)))
			// When the service's config state is resumed, the agent will update the node resource and the agbot will be returned this node
			// in a search result.
			return
		}

		if wi.ConsumerPolicy.PatternId != "" {
			// Check the arch of the top level service against the node. If it does not match, then do not make an agreement.
			// In the pattern case, the top level service spec, arch and version are also put in the node's registeredServices
			for _, ms_svc := range exchangeDev.RegisteredServices {
				if ms_svc.Url == cutil.FormOrgSpecUrl(workload.WorkloadURL, workload.Org) {
					for _, prop := range ms_svc.Properties {
						if prop.Name == "arch" {
							// convert the arch to GOARCH standard using synonyms defined in the config
							arch1 := prop.Value
							if arch1 != "" && b.config.ArchSynonyms.GetCanonicalArch(arch1) != "" {
								arch1 = b.config.ArchSynonyms.GetCanonicalArch(arch1)
							}
							arch2 := workload.Arch
							if arch2 != "" && b.config.ArchSynonyms.GetCanonicalArch(arch2) != "" {
								arch2 = b.config.ArchSynonyms.GetCanonicalArch(arch2)
							}

							if arch1 != arch2 {
								glog.Infof(BAWlogstring(workerId, fmt.Sprintf("workload arch %v does not match the device arch %v. Can not make agreement.", workload.Arch, prop.Value)))
								return
							}
						}
					}
				}
			}
		}

		// The workload in the consumer policy has a reference to the workload details. We need to get the details so that we
		// can verify that the device has the right version API specs (services) to run this workload. Then, we can store the workload details
		// into the consumer policy file. We have a copy of the consumer policy file that we can modify. If the device doesnt have the right
		// version API specs (services), then we will try the next workload.

		// The arch field in the workload could be empty or a '*' meaning any arch. If that's the case, we will use the device's arch
		// when we search for services.
		if workload.Arch == "" || workload.Arch == "*" {
			workload.Arch = exchangeDev.Arch
		}

		asl, workloadDetails, sIds, err := exchange.GetHTTPServiceResolverHandler(cph)(workload.WorkloadURL, workload.Org, workload.Version, workload.Arch)
		if err != nil {
			glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error searching for service details %v, error: %v", workload, err)))
			return
		}
		topSvcDef := compcheck.ServiceDefinition{Org: workload.Org, ServiceDefinition: *workloadDetails}
		sIdTop := sIds[0]

		// Do not make proposals for services without a deployment configuration got its node type.
		t_comp, t_reason := compcheck.CheckTypeCompatibility(nodeType, &topSvcDef, msgPrinter)
		if !t_comp {
			glog.Warningf(BAWlogstring(workerId, fmt.Sprintf("cannot make agreement with node %v for service %v/%v %v. %v", wi.Device.Id, workload.Org, workload.WorkloadURL, workload.Version, t_reason)))
			return
		}

		// zero out the dependent services for cluster type
		if nodeType == persistence.DEVICE_TYPE_CLUSTER {
			asl = new(policy.APISpecList)
		}

		// Canonicalize the arch field in the API spec
		for ix, apiSpec := range *asl {
			if apiSpec.Arch != "" && b.config.ArchSynonyms.GetCanonicalArch(apiSpec.Arch) != "" {
				(*asl)[ix].Arch = b.config.ArchSynonyms.GetCanonicalArch(apiSpec.Arch)
			}
		}

		policy_match := false

		// Check if the depended services are suspended. If any of them are suspended, then abort the agreement initialization process.
		for _, apiSpec := range *asl {
			for _, devMS := range exchangeDev.RegisteredServices {
				if devMS.ConfigState == exchange.SERVICE_CONFIGSTATE_SUSPENDED {
					if devMS.Url == cutil.FormOrgSpecUrl(apiSpec.SpecRef, apiSpec.Org) || devMS.Url == apiSpec.SpecRef {
						glog.Infof(BAWlogstring(workerId, fmt.Sprintf("cannot make agreement with %v for policy %v because service %v is suspended by the user.", wi.Device.Id, wi.ConsumerPolicy.Header.Name, devMS.Url)))
						// When the service's config state is resumed, the agent will update the node resource and the agbot will be returned this node
						// in a search result.
						return
					}
				}
			}
		}

		// get dependent service definitions for later use
		_, depServices, _, _, err := exchange.GetHTTPServiceDefResolverHandler(cph)(workload.WorkloadURL, workload.Org, workload.Version, workload.Arch)
		if err != nil {
			glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error searching for dependent service details for %v, error: %v", workload, err)))
			return
		}

		// check if merged producer policy matches the consumer policy
		if wi.ConsumerPolicy.PatternId != "" {
			mergedProducer, err := b.GetMergedProducerPolicyForPattern(wi.Device.Id, exchangeDev, *asl)
			if err != nil {
				glog.Errorf(BAWlogstring(workerId, fmt.Sprintf(err.Error())))
				return
			} else if mergedProducer != nil {
				wi.ProducerPolicy = *mergedProducer
			}
			svcDefResolverHandler := exchange.GetHTTPServiceDefResolverHandler(b)
			patternHandler := exchange.GetHTTPExchangePatternHandler(b)
			nodePolHandler := exchange.GetHTTPNodePolicyHandler(b)
			cc := compcheck.CompCheck{NodeId: wi.Device.Id, PatternId: wi.ConsumerPolicy.PatternId, Service: []common.AbstractServiceFile{&topSvcDef}}
			resourceCC := compcheck.CompCheckResource{DepServices: depServices, NodeArch: exchangeDev.Arch}
			ccOutput, err := compcheck.EvaluatePatternPrivilegeCompatability(svcDefResolverHandler, patternHandler, nodePolHandler, &cc, &resourceCC, msgPrinter, false, false)
			// If the device doesnt support the workload requirements, then remember that we rejected a higher priority workload because of
			// device requirements not being met. This will cause agreement cancellation to try the highest priority workload again
			// even if retries have been disabled.
			if err != nil {
				glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("%v", err)))
				return
			} else if ccOutput.Compatible {
				if err := wi.ProducerPolicy.APISpecs.Supports(*asl); err != nil {
					glog.Warningf(BAWlogstring(workerId, fmt.Sprintf("skipping workload %v because device %v cant support it: %v", workload, wi.Device.Id, err)))
				} else {
					policy_match = true
				}
			} else {
				policy_match = false
			}

		} else {
			// non pattern case

			// get service policy
			servicePolTemp, foundTemp := wi.ServicePolicies[sIdTop]
			if !foundTemp {
				var errTemp error
				serviceIdPolicyHandler := exchange.GetHTTPServicePolicyWithIdHandler(b)
				servicePol, errTemp = compcheck.GetServicePolicyWithId(serviceIdPolicyHandler, sIdTop, msgPrinter)
				if errTemp != nil {
					glog.Warning(BAWlogstring(workerId, fmt.Sprintf("error getting service policy for service %v. %v", sIdTop, errTemp)))
					return
				}
			} else {
				servicePol = &servicePolTemp
			}
			found = foundTemp

			//merge the service policy with the built-in service policy
			builtInSvcPol := externalpolicy.CreateServiceBuiltInPolicy(workload.WorkloadURL, workload.Org, workload.Version, workload.Arch)
			// add built-in service properties to the service policy
			mergedServicePol := compcheck.AddDefaultPropertiesToServicePolicy(servicePol, builtInSvcPol, nil)
			if mergedServicePol, err = b.SetServicePolicyPrivilege(mergedServicePol, &topSvcDef, sIdTop, depServices, msgPrinter); err != nil {
				glog.Warning(BAWlogstring(workerId, fmt.Sprintf("error setting the privilege for the merged service policy. %v.", err)))
				return
			}

			if compatible, consumPol, err := b.checkPolicyCompatibility(workerId, wi, nodePolicy, &wi.ConsumerPolicy, mergedServicePol, "", msgPrinter); err != nil {
				return
			} else {
				if compatible {
					if consumPol != nil {
						wi.ConsumerPolicy = *consumPol
					}
					policy_match = true
				}
			}
		}

		// Make sure the user inputs are all there
		userInput_match := true
		if policy_match {
			if compatible, reason, err := compcheck.VerifyUserInputForServiceCache(&topSvcDef, depServices, wi.ConsumerPolicy.UserInput, exchangeDev.UserInput, nodeType, msgPrinter); err != nil {
				glog.Warningf(BAWlogstring(workerId, fmt.Sprintf("Error validating the user input for service %v/%v %v %v: %v", workload.Org, workloadDetails.URL, workloadDetails.Version, workloadDetails.Arch, err)))
				userInput_match = false
			} else if !compatible {
				glog.Warningf(BAWlogstring(workerId, fmt.Sprintf("User input does not meet the requirement for service %v/%v %v %v: %v", workload.Org, workloadDetails.URL, workloadDetails.Version, workloadDetails.Arch, reason)))
				userInput_match = false
			}
		}

		// Make sure the deployment policy or pattern has all the right secret bindings in place and extract the secret details for the agent.
		secrets_match := true
		if policy_match && userInput_match && nodeType == persistence.DEVICE_TYPE_DEVICE {

			err := b.ValidateAndExtractSecrets(&wi.ConsumerPolicy, wi.Device.Id, &topSvcDef, depServices, workerId, msgPrinter)
			if err != nil {
				glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("Error processing secrets for policy %v, error: %v", wi.ConsumerPolicy.Header.Name, err)))
				secrets_match = false
			}
		}

		// All the error cases have been checked, now decide whether to propose this workload or try another version
		if !policy_match || !userInput_match || !secrets_match {
			if !workload.HasEmptyPriority() {
				// If this is not the first time through the loop, update the workload usage record, otherwise create it.
				if lastWorkload != nil {
					if _, err := b.db.UpdatePriority(wi.Device.Id, wi.ConsumerPolicy.Header.Name, workload.Priority.PriorityValue, workload.Priority.RetryDurationS, workload.Priority.VerifiedDurationS, agreementIdString); err != nil {
						glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error updating priority in persistent workload usage records for device %v with policy %v, error: %v", wi.Device.Id, wi.ConsumerPolicy.Header.Name, err)))
						return
					}
				} else if err := b.db.NewWorkloadUsage(wi.Device.Id, wi.ProducerPolicy.HAGroup.Partners, "", wi.ConsumerPolicy.Header.Name, workload.Priority.PriorityValue, workload.Priority.RetryDurationS, workload.Priority.VerifiedDurationS, true, agreementIdString); err != nil {
					glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error creating persistent workload usage records for device %v with policy %v, error: %v", wi.Device.Id, wi.ConsumerPolicy.Header.Name, err)))
					return
				}

				// Artificially bump up the retry count so that the loop will choose the next workload
				if _, err := b.db.UpdateRetryCount(wi.Device.Id, wi.ConsumerPolicy.Header.Name, workload.Priority.Retries+1, agreementIdString); err != nil {
					glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error updating retry count persistent workload usage records for device %v with policy %v, error: %v", wi.Device.Id, wi.ConsumerPolicy.Header.Name, err)))
					return
				}
			}
		} else {

			foundWorkload = true

			// copy all the service ids
			svcIds = make([]string, len(sIds))
			copy(svcIds, sIds)

			// if this service policy is not from the policy manager cache, then send a message to pass the service policy back
			// so that the business policy manager will save it.
			if !found {
				if servicePol == nil {
					// An empty policy means that the service does not have a policy.
					// An empty polcy is also tracked in the business policy manager, this way we know if there is
					// new service policy added later.
					// The business policy manager does not track all the service policies referenced by a business policy.
					// It only tracks the ones that have agreements with the nodes.
					servicePol = new(externalpolicy.ExternalPolicy)
				}
				polString, err := json.Marshal(servicePol)
				if err != nil {
					glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error marshaling service policy for service %v. %v", sIdTop, err)))
					return
				}
				cph.SendEventMessage(events.NewCacheServicePolicyMessage(events.CACHE_SERVICE_POLICY, wi.Org, wi.ConsumerPolicyName, sIdTop, string(polString)))
			}

			// The device seems to support the required API specs, so augment the consumer policy file with the workload
			// details that match what the producer can support.
			if wi.ConsumerPolicy.PatternId != "" {
				wi.ConsumerPolicy.APISpecs = (*asl)
			}

			// Save the deployment and implementation package details into the consumer policy so that the node knows how to run
			// the workload/service in the policy.
			if nodeType == persistence.DEVICE_TYPE_CLUSTER {
				workload.ClusterDeployment = workloadDetails.GetClusterDeploymentString()
				workload.ClusterDeploymentSignature = workloadDetails.GetClusterDeploymentSignature()
			} else {
				workload.Deployment = workloadDetails.GetDeploymentString()
				workload.DeploymentSignature = workloadDetails.GetDeploymentSignature()
			}

			if glog.V(5) {
				glog.V(5).Infof(BAWlogstring(workerId, fmt.Sprintf("workload %v is supported by device %v", workload, wi.Device.Id)))
			}
		}

		lastWorkload = workload
	}

	// get the node max heartbeat interval if it's set on the node
	nodeMaxHBInterval := exchangeDev.HeartbeatIntv.MaxInterval

	// if the node max heartbeat interval is not set on the node, then get if from the org
	if nodeMaxHBInterval == 0 {
		exchOrg, err := exchange.GetOrganization(b.config.Collaborators.HTTPClientFactory, exchange.GetOrg(wi.Device.Id), b.config.AgreementBot.ExchangeURL, cph.GetExchangeId(), cph.GetExchangeToken())
		if err != nil {
			glog.Errorf(BAWlogstring(workerId, fmt.Errorf("Unable to get org %v from exchange: %v", exchange.GetOrg(wi.Device.Id), err)))
		}
		nodeMaxHBInterval = exchOrg.HeartbeatIntv.MaxInterval
	}

	// Call the exchange to make sure that all partners are registered in the exchange. We can do this check now that we know
	// exactly what the merged producer policy looks like.
	if err := b.incompleteHAGroup(cph, &wi.ProducerPolicy); err != nil {
		glog.Warningf(BAWlogstring(workerId, fmt.Sprintf("received error checking HA group %v completeness for device %v, error: %v", wi.ProducerPolicy.HAGroup, wi.Device.Id, err)))
		return
	}

	// If this device is advertising a property that we are supposed to ignore, then skip it.
	if ignore, err := b.ignoreDevice(&wi.ProducerPolicy); err != nil {
		glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("received error checking for ignored device %v, error: %v", wi.Device.Id, err)))
		return
	} else if ignore {
		glog.V(3).Infof(BAWlogstring(workerId, fmt.Sprintf("skipping device %v, advertises ignored property", wi.Device.Id)))
		return
	}

	// Create pending agreement in database
	if err := b.db.AgreementAttempt(agreementIdString, wi.Org, wi.Device.Id, nodeType, wi.ConsumerPolicy.Header.Name, bcType, bcName, bcOrg, cph.Name(), wi.ConsumerPolicy.PatternId, svcIds, wi.ConsumerPolicy.NodeH, b.config.AgreementBot.GetProtocolTimeout(nodeMaxHBInterval), b.config.AgreementBot.GetAgreementTimeout(nodeMaxHBInterval)); err != nil {
		glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error persisting agreement attempt: %v", err)))

		// Decoding device publicKey to []byte
	} else if publicKeyBytes, err := base64.StdEncoding.DecodeString(wi.Device.PublicKey); err != nil {
		glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error decoding device publicKey for node: %s, %v", wi.Device.Id, err)))

		// Create message target for protocol message
	} else if mt, err := exchange.CreateMessageTarget(wi.Device.Id, nil, publicKeyBytes, ""); err != nil {
		glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error creating message target: %v", err)))

		// Initiate the protocol
	} else if proposal, err := protocolHandler.InitiateAgreement(agreementIdString, &wi.ProducerPolicy, &wi.ConsumerPolicy, wi.Org, cph.GetExchangeId(), mt, workload, b.config.AgreementBot.DefaultWorkloadPW, b.config.AgreementBot.NoDataIntervalS, cph.GetSendMessage()); err != nil {
		glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error initiating agreement: %v", err)))

		// Remove pending agreement from database
		if err := b.db.DeleteAgreement(agreementIdString, cph.Name()); err != nil {
			glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error deleting pending agreement: %v, error %v", agreementIdString, err)))
		}

		// TODO: Publish error on the message bus

		// Update the agreement in the DB with the proposal and policy
	} else if err := cph.PersistAgreement(wi, proposal, workerId); err != nil {
		glog.Errorf(err.Error())
	}

}

// get the merged producer policy. asl is the spec list for the dependent services for a top level service.
func (b *BaseAgreementWorker) GetMergedProducerPolicyForPattern(deviceId string, dev *exchange.Device, asl policy.APISpecList) (*policy.Policy, error) {
	var mergedProducer *policy.Policy

	// Run through all the services on the node that are required by this workload and merge the policies
	for _, apiSpec := range asl {
		for _, devMS := range dev.RegisteredServices {
			// Find the device's service definition based on the services needed by the top level service.
			if devMS.Url == cutil.FormOrgSpecUrl(apiSpec.SpecRef, apiSpec.Org) || devMS.Url == apiSpec.SpecRef {
				pol, err := policy.DemarshalPolicy(devMS.Policy)
				if err != nil {
					return nil, fmt.Errorf("error demarshalling device %v policy, error: %v", deviceId, err)
				}
				if mergedProducer == nil {
					mergedProducer = pol
				} else if newPolicy, err := policy.Are_Compatible_Producers(mergedProducer, pol, b.config.AgreementBot.NoDataIntervalS); err != nil {
					return nil, fmt.Errorf("error merging policies %v and %v, error: %v", mergedProducer, pol, err)
				} else {
					mergedProducer = newPolicy
				}
				break
			}
		}
	}
	return mergedProducer, nil
}

// This function sets a property on the service privilege that indicates if the service uses a workload that requires privileged mode or network=host
// This will not overwrite openhorizon.allowPrivileged=true if the service is found to not require privileged mode.
func (b *BaseAgreementWorker) SetServicePolicyPrivilege(svcPolicy *externalpolicy.ExternalPolicy, topSvc common.AbstractServiceFile, topSvcId string,
	depServiceDefs map[string]exchange.ServiceDefinition, msgPrinter *message.Printer) (*externalpolicy.ExternalPolicy, error) {

	runtimePriv, err, _ := compcheck.ServicesRequirePrivilege(topSvc, topSvcId, depServiceDefs, msgPrinter)
	if err != nil {
		return nil, err
	}

	svcPolicy.Properties.Add_Property(externalpolicy.Property_Factory(externalpolicy.PROP_SVC_PRIVILEGED, runtimePriv), true)

	if runtimePriv {
		svcPolicy.Constraints.Add_Constraint(fmt.Sprintf("%s = %t", externalpolicy.PROP_SVC_PRIVILEGED, runtimePriv))
	}
	return svcPolicy, nil
}

// When deploying a service that is configured with secrets, the agbot needs to revalidate that all the necessary secrets have been bound to
// secret manager secrets, and then extract those secrets and put them into the agreement proposal. This function returns true when
// all the required secrets ave been validated and extracted, otherwise and error is returned.
func (b *BaseAgreementWorker) ValidateAndExtractSecrets(consumerPolicy *policy.Policy, deviceId string, topSvcDef common.AbstractServiceFile,
	depServices map[string]exchange.ServiceDefinition, workerId string, msgPrinter *message.Printer) error {

	// When services and policies are published, the following validation is performed. Doing it here again in case something changed. The
	// exchange does not maintain referential integrity for these aspects of the user's metadata.

	glog.V(3).Infof(BAWlogstring(workerId, fmt.Sprintf("validating secret bindings for service %v/%v %v %v and dependencies, bindings: %v", topSvcDef.GetOrg(), topSvcDef.GetURL(), topSvcDef.GetVersion(), topSvcDef.GetArch(), consumerPolicy.SecretBinding)))

	// Validate that the secret bindings are still correct.

	compatible, reason, resMap, err := compcheck.VerifySecretBindingForServiceCache(
		topSvcDef,
		depServices,
		consumerPolicy.SecretBinding,
		nil,
		"",
		exchange.GetOrg(deviceId),
		msgPrinter)

	if err != nil {
		glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error validating secret bindings for policy %v, service %v/%v %v, error: %v", consumerPolicy.Header.Name, topSvcDef.GetOrg(), topSvcDef.GetURL(), topSvcDef.GetVersion(), err)))
		return err
	}

	if !compatible {
		err_msg := fmt.Sprintf("secret bindings for policy %v, service %v/%v %v not compatible, reason: %v", consumerPolicy.Header.Name, topSvcDef.GetOrg(), topSvcDef.GetURL(), topSvcDef.GetVersion(), reason)
		glog.Errorf(BAWlogstring(workerId, err_msg))
		return fmt.Errorf(err_msg)
	}

	if len(resMap) == 0 {
		glog.V(3).Infof(BAWlogstring(workerId, fmt.Sprintf("no secrets for service %v/%v %v", topSvcDef.GetOrg(), topSvcDef.GetURL(), topSvcDef.GetVersion())))
	}

	// Get the list of secret bindings that are needed so that we know which secrets to extract from the secret manager.
	neededSB, _ := compcheck.GroupSecretBindings(consumerPolicy.SecretBinding, resMap)

	// If the agreement needs secrets, make sure the secrets manager is available.
	if len(neededSB) != 0 && (b.secretsMgr == nil || !b.secretsMgr.IsReady()) {
		err := errors.New(fmt.Sprintf("secrets required for %v but the secret manager is not available, terminating agreement attempt", consumerPolicy.Header.Name))
		glog.Errorf(BAWlogstring(workerId, err))
		return err
	}

	newBindings := make([]exchangecommon.SecretBinding, 0)

	// Add the bound secrets with details into the consumer policy for transport to the agent in the proposal.
	for _, binding := range neededSB {

		// The secret details are transported within the proposal in the SecretBinding section where the secret provider secret name is
		// replaced with the secret details.
		sb := binding.MakeCopy()
		sb.Secrets = make([]exchangecommon.BoundSecret, 0)

		for _, boundSecret := range binding.Secrets {

			newBS := boundSecret.MakeCopy()

			// Iterate the bound secrets, extracting the details for each secret.
			for serviceSecretName, secretName := range boundSecret {

				glog.V(5).Infof(BAWlogstring(workerId, fmt.Sprintf("extracting secret details for %v:%v", serviceSecretName, secretName)))

				// The secret name might be a user private or org wide secret. Parse the name to determine which it is.
				secretUser, shortSecretName, err := compcheck.ParseVaultSecretName(secretName, msgPrinter)
				if err != nil {
					glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error parsing secret %v for policy %v, service %v/%v %v, error: %v", secretName, consumerPolicy.Header.Name, binding.ServiceOrgid, binding.ServiceUrl, binding.ServiceVersionRange, err)))
					return err
				}

				// Call the secret manager plugin to get the secret details.
				details, err := b.secretsMgr.GetSecretDetails(b.GetExchangeId(), b.GetExchangeToken(), exchange.GetOrg(deviceId), secretUser, shortSecretName)

				if err != nil {
					glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error retrieving secret %v for policy %v, service %v/%v %v, error: %v", secretName, consumerPolicy.Header.Name, binding.ServiceOrgid, binding.ServiceUrl, binding.ServiceVersionRange, err)))
					return err
				}

				detailBytes, err := json.Marshal(details)
				if err != nil {
					glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error marshalling secret details %v for policy %v, service %v/%v %v, error: %v", secretName, consumerPolicy.Header.Name, binding.ServiceOrgid, binding.ServiceUrl, binding.ServiceVersionRange, err)))
					return err
				}

				newBS[serviceSecretName] = base64.StdEncoding.EncodeToString(detailBytes)
			}
			sb.Secrets = append(sb.Secrets, newBS)

		}
		newBindings = append(newBindings, sb)

	}
	consumerPolicy.SecretDetails = newBindings

	return nil

}

func (b *BaseAgreementWorker) HandleAgreementReply(cph ConsumerProtocolHandler, wi *HandleReply, workerId string) bool {

	reply := wi.Reply
	protocolHandler := cph.AgreementProtocolHandler("", "", "") // Use the generic protocol handler

	// The reply message is usually deleted before recording on the blockchain. For now assume it will be deleted at the end. Early exit from
	// this function is NOT allowed.
	deletedMessage := false

	// Get the agreement id lock to prevent any other thread from processing this same agreement.
	lock := b.alm.getAgreementLock(wi.Reply.AgreementId())
	lock.Lock()

	// The lock is dropped at the end of this function or right before the blockchain write. Early exit from this function is NOT allowed.
	droppedLock := false

	// Assume we will ack negatively unless we find out that everything is ok.
	ackReplyAsValid := false
	sendReply := true

	if reply.ProposalAccepted() {

		// Find the saved agreement in the database. The returned agreement might be archived. If it's archived, then it is our agreement
		// so we will delete the protocol msg.
		if agreement, err := b.db.FindSingleAgreementByAgreementId(reply.AgreementId(), cph.Name(), []persistence.AFilter{}); err != nil {
			// A DB error occurred so we dont know if this is our agreement or not. Leave it alone until the agbot is restarted
			// or until the DB error is resolved.
			glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error querying pending agreement %v, error: %v", reply.AgreementId(), err)))
			sendReply = false
		} else if agreement != nil && agreement.Archived {
			glog.V(3).Infof(BAWlogstring(workerId, fmt.Sprintf("reply %v is for a cancelled agreement %v, deleting reply message.", wi.MessageId, reply.AgreementId())))
		} else if agreement == nil {
			// This protocol msg is not for this agbot, so ignore it.
			glog.Warningf(BAWlogstring(workerId, fmt.Sprintf("discarding reply %v, agreement id %v not in this agbot's database", wi.MessageId, reply.AgreementId())))
			// this will cause us to not send a reply, which is what we want in this case because this reply is not for us.
			sendReply = false
			deletedMessage = true // causes the msg to not be deleted, which is what we want so that another agbot will see the msg.
		} else if cph.AlreadyReceivedReply(agreement) {
			glog.Warningf(BAWlogstring(workerId, fmt.Sprintf("discarding reply %v, agreement id %v already received a reply", wi.MessageId, agreement.CurrentAgreementId)))
			// this will cause us to not send a reply ack, which is what we want in this case
			sendReply = false

			// Now we need to write the info to the exchange and the database
		} else if proposal, err := protocolHandler.DemarshalProposal(agreement.Proposal); err != nil {
			glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error validating proposal from pending agreement %v, error: %v", reply.AgreementId(), err)))
		} else if pol, err := policy.DemarshalPolicy(proposal.TsAndCs()); err != nil {
			glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error demarshalling tsandcs policy from pending agreement %v, error: %v", reply.AgreementId(), err)))

		} else if err := cph.PersistReply(reply, pol, workerId); err != nil {
			glog.Errorf(err.Error())

		} else if err := cph.RecordConsumerAgreementState(reply.AgreementId(), pol, agreement.Org, "Producer agreed", b.workerID); err != nil {
			glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error setting agreement state for %v", reply.AgreementId())))

			// We need to send a reply ack and write the info to the blockchain
		} else if consumerPolicy, err := policy.DemarshalPolicy(agreement.Policy); err != nil {
			glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("unable to demarshal policy for agreement %v, error %v", reply.AgreementId(), err)))
		} else {
			// Done handling the response successfully
			ackReplyAsValid = true

			// If we dont have a workload usage record for this device, then we need to create one. If there is already a
			// workload usage record and workload rollback retry counting is enabled, then check to see if the workload priority
			// has changed. If so, update the record and reset the retry count and time. Othwerwise just update the retry count.
			if wlUsage, err := b.db.FindSingleWorkloadUsageByDeviceAndPolicyName(wi.SenderId, consumerPolicy.Header.Name); err != nil {
				glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error searching for persistent workload usage records for device %v with policy %v, error: %v", wi.SenderId, consumerPolicy.Header.Name, err)))
			} else if wlUsage == nil {
				// There is no workload usage record. Make sure that the current workload chosen is the highest priority workload.
				// There could have been a change in the system such that the chosen workload is no longer the right choice. If this
				// is the case, then we need to reject the agreement and start over.

				workload := consumerPolicy.NextHighestPriorityWorkload(0, 0, 0)
				if !workload.Priority.IsSame(pol.Workloads[0].Priority) {
					// Need a new workload usage record but not the same as the highest priority. That can't be right.
					ackReplyAsValid = false
				} else if !pol.Workloads[0].HasEmptyPriority() {
					if err := b.db.NewWorkloadUsage(wi.SenderId, pol.HAGroup.Partners, agreement.Policy, consumerPolicy.Header.Name, pol.Workloads[0].Priority.PriorityValue, pol.Workloads[0].Priority.RetryDurationS, pol.Workloads[0].Priority.VerifiedDurationS, false, reply.AgreementId()); err != nil {
						glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error creating persistent workload usage records for device %v with policy %v, error: %v", wi.SenderId, consumerPolicy.Header.Name, err)))
					}
				}
			} else {
				if wlUsage.Policy == "" {
					if _, err := b.db.UpdatePolicy(wi.SenderId, consumerPolicy.Header.Name, agreement.Policy); err != nil {
						glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error updating policy in workload usage prioroty for device %v with policy %v, error: %v", wi.SenderId, consumerPolicy.Header.Name, err)))
					}
				}

				if !wlUsage.DisableRetry {
					if pol.Workloads[0].Priority.PriorityValue != wlUsage.Priority {
						if _, err := b.db.UpdatePriority(wi.SenderId, consumerPolicy.Header.Name, pol.Workloads[0].Priority.PriorityValue, pol.Workloads[0].Priority.RetryDurationS, pol.Workloads[0].Priority.VerifiedDurationS, reply.AgreementId()); err != nil {
							glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error updating workload usage prioroty for device %v with policy %v, error: %v", wi.SenderId, consumerPolicy.Header.Name, err)))
						}
					} else if _, err := b.db.UpdateRetryCount(wi.SenderId, consumerPolicy.Header.Name, wlUsage.RetryCount+1, reply.AgreementId()); err != nil {
						glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error updating workload usage retry count for device %v with policy %v, error: %v", wi.SenderId, consumerPolicy.Header.Name, err)))
					}
				}

				// Make sure the agreement id gets updated.
				if _, err := b.db.UpdateWUAgreementId(wi.SenderId, consumerPolicy.Header.Name, reply.AgreementId(), cph.Name()); err != nil {
					glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error updating agreement id %v in workload usage for %v for policy %v, error: %v", reply.AgreementId(), wi.SenderId, consumerPolicy.Header.Name, err)))
				}
			}

			// Both parties have agreed on the proposal, so now we need to scan the MMS object cache and find any objects that should be deployed
			// on this node.

			// For the purposes of compatibility, skip this function if the agbot config has not been updated to point to the CSS.
			// Only non-pattern based agreements can use MMS object policy.
			if agreement.GetDeviceType() == persistence.DEVICE_TYPE_DEVICE {
				if b.GetCSSURL() != "" && agreement.Pattern == "" {

					// Retrieve the node policy.
					nodePolicyHandler := exchange.GetHTTPNodePolicyHandler(b)
					msgPrinter := i18n.GetMessagePrinter()
					_, nodePolicy, err := compcheck.GetNodePolicy(nodePolicyHandler, agreement.DeviceId, msgPrinter)
					if err != nil {
						glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("%v", err)))
					} else if nodePolicy == nil {
						glog.Warning(BAWlogstring(workerId, fmt.Sprintf("cannot find node policy for this node %v.", agreement.DeviceId)))
					} else {
						glog.V(5).Infof(BAWlogstring(workerId, fmt.Sprintf("retrieved node policy: %v", nodePolicy)))
					}

					// Query the MMS cache to find objects with policies that refer to the agreed-to service(s). Service IDs are
					// a concatenation of org '/' service name, hardware architecture and version, separated by underscores. We need
					// all 3 pieces.
					for _, serviceId := range agreement.ServiceId {

						serviceNamePieces := strings.SplitN(serviceId, "_", 3)

						objPolicies := b.mmsObjMgr.GetObjectPolicies(agreement.Org, serviceNamePieces[0], serviceNamePieces[2], serviceNamePieces[1])

						destsToAddMap := make(map[string]*exchange.ObjectDestinationsToAdd, 0)
						if addedToList, _, err := AssignObjectToNodes(b, objPolicies, agreement.DeviceId, nodePolicy, destsToAddMap, false); err != nil {
							glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("unable to assign object(s) to node %v, error %v", agreement.DeviceId, err)))
						} else if addedToList {
							AddDestinationsForObjects(b, destsToAddMap)
						}
					}
				} else if b.GetCSSURL() == "" {
					glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("unable to evaluate object placement because there is no CSS URL configured in this agbot")))
				}
			}

			// Send the reply Ack if it's still valid.
			if ackReplyAsValid {
				if mt, err := exchange.CreateMessageTarget(wi.SenderId, nil, wi.SenderPubKey, wi.From); err != nil {
					glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error creating message target: %v", err)))
				} else if err := protocolHandler.Confirm(ackReplyAsValid, reply.AgreementId(), mt, cph.GetSendMessage()); err != nil {
					glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error trying to send reply ack for %v to %v, error: %v", reply.AgreementId(), mt, err)))
				}

				// Delete the original reply message
				if wi.MessageId != 0 {
					if err := cph.DeleteMessage(wi.MessageId); err != nil {
						glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error deleting message %v from exchange for agbot %v", wi.MessageId, cph.GetExchangeId())))
					}
				}

				deletedMessage = true
				droppedLock = true
				lock.Unlock()

				if err := cph.PostReply(reply.AgreementId(), proposal, reply, consumerPolicy, agreement.Org, workerId); err != nil {
					glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error trying to record agreement in blockchain, %v", err)))
					b.CancelAgreementWithLock(cph, reply.AgreementId(), cph.GetTerminationCode(TERM_REASON_CANCEL_BC_WRITE_FAILED), workerId)
					ackReplyAsValid = false
				}

			}
		}

		// Always send an ack for a reply with a positive decision in it
		if !ackReplyAsValid && sendReply {
			if mt, err := exchange.CreateMessageTarget(wi.SenderId, nil, wi.SenderPubKey, wi.From); err != nil {
				glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error creating message target: %v", err)))
			} else if err := protocolHandler.Confirm(ackReplyAsValid, reply.AgreementId(), mt, cph.GetSendMessage()); err != nil {
				glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error trying to send reply ack for %v to %v, error: %v", reply.AgreementId(), wi.From, err)))
			}
		}

	} else {
		glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("received rejection from producer %v", reply)))

		// Returns true if the protocol msg can be deleted.
		ok := b.CancelAgreement(cph, reply.AgreementId(), cph.GetTerminationCode(TERM_REASON_NEGATIVE_REPLY), workerId)
		deletedMessage = !ok
	}

	// Get rid of the lock
	if !droppedLock {
		lock.Unlock()
	}

	// Get rid of the exchange message if there is one
	if wi.MessageId != 0 && !deletedMessage {
		if err := cph.DeleteMessage(wi.MessageId); err != nil {
			glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error deleting message %v from exchange for agbot %v", wi.MessageId, cph.GetExchangeId())))
		}
	}

	return ackReplyAsValid

}

func (b *BaseAgreementWorker) HandleDataReceivedAck(cph ConsumerProtocolHandler, wi *HandleDataReceivedAck, workerId string) {

	protocolHandler := cph.AgreementProtocolHandler("", "", "") // Use the generic protocol handler

	deleteMessage := true

	if d, err := protocolHandler.ValidateDataReceivedAck(wi.Ack); err != nil {
		glog.Warningf(BAWlogstring(workerId, fmt.Sprintf("discarding message: %v", wi.Ack)))
	} else if drAck, ok := d.(*abstractprotocol.BaseDataReceivedAck); !ok {
		glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("unable to cast Data Received Ack %v to %v Proposal Reply, is %T", d, cph.Name(), d)))
	} else {

		// Get the agreement id lock to prevent any other thread from processing this same agreement.
		lock := b.alm.getAgreementLock(drAck.AgreementId())
		lock.Lock()

		// The agreement might be archived in this agbot's partition. If it's archived, then it is our agreement
		// so we will delete the protocol msg, but we will ignore the ack msg.
		if ag, err := b.db.FindSingleAgreementByAgreementId(drAck.AgreementId(), cph.Name(), []persistence.AFilter{}); err != nil {
			glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error querying agreement %v, error: %v", drAck.AgreementId(), err)))
			deleteMessage = false
		} else if ag != nil && ag.Archived {
			glog.V(3).Infof(BAWlogstring(workerId, fmt.Sprintf("Data received Ack is for a cancelled agreement %v, deleting ack message.", drAck.AgreementId())))
		} else if ag == nil {
			// This protocol msg is not for this agbot, so ignore it.
			glog.Warningf(BAWlogstring(workerId, fmt.Sprintf("discarding data received ack %v, agreement id %v not in this agbot's database", wi.MessageId, drAck.AgreementId())))
			deleteMessage = false // causes the msg to not be deleted, which is what we want so that another agbot will see the msg.
		} else if _, err := b.db.DataNotification(ag.CurrentAgreementId, cph.Name()); err != nil {
			glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("unable to record data notification, error: %v", err)))
		}

		// Drop the lock. The code block above must always flow through this point.
		lock.Unlock()

	}

	// Get rid of the exchange message if there is one
	if wi.MessageId != 0 && deleteMessage {
		if err := cph.DeleteMessage(wi.MessageId); err != nil {
			glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error deleting message %v from exchange for agbot %v", wi.MessageId, cph.GetExchangeId())))
		}
	}

}

func (b *BaseAgreementWorker) HandleWorkloadUpgrade(cph ConsumerProtocolHandler, wi *HandleWorkloadUpgrade, workerId string) {

	// Force an upgrade of a workload on a specific device, given a specific policy that delivered the workload.
	// The upgrade request will contain a specific device and policy name, but it might not contain an agreement
	// id. At this point we assume that the originator of the workload upgrade event validated that the agreement id
	// (if specified) matches the device and policy name. Further, the caller has also validated that the device does
	// (or did) have a workload running from the specified policy name.

	// If there is no agreement id specified then find one for the current device and policy name. If we find one,
	// grab the agreement id lock, cancel the agreement and delete the workload usage record.

	if wi.AgreementId == "" {
		if ags, err := b.db.FindAgreements([]persistence.AFilter{persistence.DevPolAFilter(wi.Device, wi.PolicyName)}, cph.Name()); err != nil {
			glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error finding agreement for device %v and policyName %v, error: %v", wi.Device, wi.PolicyName, err)))
		} else if len(ags) == 0 {
			// If there is no agreement found, is it a problem? We could have caught the system in a state where there is no
			// agreement, but there still might be a workload usage record for the device and policy name. It should be safe to
			// just delete the workload usage record. When an agreement reply is processed, the code will verify that the
			// highest priority workload is being used when creating a new workload usage record.
			glog.V(5).Infof(BAWlogstring(workerId, fmt.Sprintf("forced workload upgrade found no current agreement for device %v and policy name %v", wi.Device, wi.PolicyName)))
		} else {
			// Cancel all agreements
			for _, ag := range ags {
				// Terminate the agreement
				b.CancelAgreementWithLock(cph, ag.CurrentAgreementId, cph.GetTerminationCode(TERM_REASON_CANCEL_FORCED_UPGRADE), workerId)
			}
		}
	} else {
		// Terminate the agreement
		b.CancelAgreementWithLock(cph, wi.AgreementId, cph.GetTerminationCode(TERM_REASON_CANCEL_FORCED_UPGRADE), workerId)
	}

	// Find the workload usage record and delete it. This will cause any new agreement negotiations to start with the highest priority
	// workload.
	if err := b.db.DeleteWorkloadUsage(wi.Device, wi.PolicyName); err != nil {
		glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error deleting workload usage record for device %v and policyName %v, error: %v", wi.Device, wi.PolicyName, err)))
	}

}

func (b *BaseAgreementWorker) CancelAgreementWithLock(cph ConsumerProtocolHandler, agreementId string, reason uint, workerId string) bool {
	// Get the agreement id lock to prevent any other thread from processing this same agreement.
	lock := b.AgreementLockManager().getAgreementLock(agreementId)
	lock.Lock()

	// Terminate the agreement
	ok := b.CancelAgreement(cph, agreementId, reason, workerId)

	lock.Unlock()

	// Don't need the agreement lock anymore
	b.AgreementLockManager().deleteAgreementLock(agreementId)

	return ok
}

// Return true if the caller should delete the protocol message that initiated this cancel command.
func (b *BaseAgreementWorker) CancelAgreement(cph ConsumerProtocolHandler, agreementId string, reason uint, workerId string) bool {

	// Start timing out the agreement
	glog.V(3).Infof(BAWlogstring(workerId, fmt.Sprintf("terminating agreement %v reason: %v.", agreementId, cph.GetTerminationReason(reason))))

	// Update the database. Returns an error if the agreement is not found.
	ag, err := b.db.AgreementTimedout(agreementId, cph.Name())
	if err != nil {
		glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error marking agreement %v terminated: %v", agreementId, err)))
	} else if ag != nil && ag.Archived {
		// The agreement is not active and it is archived, so this message belongs to this agbot, but the cancel has already happened
		// so we should just get rid of the protocol msg.
		glog.V(3).Infof(BAWlogstring(workerId, fmt.Sprintf("cancel is for a cancelled agreement %v, deleting cancel message.", agreementId)))
		return true
	}

	// Safety check. AgreementTimedout returns an error for not found.
	if ag == nil {
		// The cancel is for an agreement that this agbot doesnt know anything about.
		glog.Warningf(BAWlogstring(workerId, fmt.Sprintf("discarding cancel for agreement id %v not in this agbot's database", agreementId)))
		// Tell the caller not to delete the exchange message if this is what initiated the cancel because this cancel is not for us.
		return false
	}

	// Update state in exchange
	if err := DeleteConsumerAgreement(b.config.Collaborators.HTTPClientFactory.NewHTTPClient(nil), b.config.AgreementBot.ExchangeURL, cph.GetExchangeId(), cph.GetExchangeToken(), agreementId); err != nil {
		glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error deleting agreement %v in exchange: %v", agreementId, err)))
	}

	// Update the workload usage record to clear the agreement. There might not be a workload usage record if there is no workload priority
	// specified in the workload section of the policy.
	if wlUsage, err := b.db.UpdateWUAgreementId(ag.DeviceId, ag.PolicyName, "", cph.Name()); err != nil {
		glog.Warningf(BAWlogstring(workerId, fmt.Sprintf("warning updating agreement id in workload usage for %v for policy %v, error: %v", ag.DeviceId, ag.PolicyName, err)))

	} else if wlUsage != nil && (wlUsage.ReqsNotMet || cph.IsTerminationReasonNodeShutdown(reason)) {
		// If the workload usage record indicates that it is not at the highest priority workload because the device cant meet the
		// requirements of the higher priority workload, then when an agreement gets cancelled, we will remove the record so that the
		// agbot always tries the next agreement starting with the highest priority workload again.
		// Or, we will remove the workload usage record if the device is cancelling the agreement because it is shutting down. A shut down
		// node that comes back and registers again, will start trying to run the highest priority workload. It should not remember the
		// workload priority in use at the time it was removed from the network.
		if err := b.db.DeleteWorkloadUsage(ag.DeviceId, ag.PolicyName); err != nil {
			glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error deleting workload usage record for device %v and policyName %v, error: %v", ag.DeviceId, ag.PolicyName, err)))
		}
	}

	// Remove the long blockchain cancel from the worker thread. It is important to give the protocol handler a chance to
	// do whatever cleanup and termination it needs to do so we should never skip calling this function.
	// If we can do the termination now, do it. Otherwise we will queue a command to do it later.

	if cph.CanCancelNow(ag) || ag.CounterPartyAddress == "" {
		if t := policy.RequiresBlockchainType(ag.AgreementProtocol); t != "" {
			b.DoAsyncCancel(cph, ag, reason, workerId)
		} else {
			cph.TerminateAgreement(ag, reason, workerId)
		}
	}

	if ag.BlockchainType != "" && !cph.IsBlockchainWritable(ag.BlockchainType, ag.BlockchainName, ag.BlockchainOrg) {
		// create deferred termination command
		glog.V(3).Infof(BAWlogstring(workerId, fmt.Sprintf("deferring blockchain cancel for %v", agreementId)))
		cph.DeferCommand(AsyncCancelAgreement{
			workType:    ASYNC_CANCEL,
			AgreementId: agreementId,
			Protocol:    cph.Name(),
			Reason:      reason,
		})
	}

	// Archive the record
	if _, err := b.db.ArchiveAgreement(ag.CurrentAgreementId, cph.Name(), reason, cph.GetTerminationReason(reason)); err != nil {
		glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error archiving terminated agreement: %v, error: %v", ag.CurrentAgreementId, err)))
	}

	return true
}

// This function is only called when the cancel is deferred due to blockchain unavailability.
func (b *BaseAgreementWorker) ExternalCancel(cph ConsumerProtocolHandler, agreementId string, reason uint, workerId string) {

	glog.V(3).Infof(BAWlogstring(workerId, fmt.Sprintf("starting deferred cancel for %v", agreementId)))

	// Find the agreement record
	if ag, err := b.db.FindSingleAgreementByAgreementId(agreementId, cph.Name(), []persistence.AFilter{}); err != nil {
		glog.Errorf(BAWlogstring(workerId, fmt.Sprintf("error querying agreement %v from database, error: %v", agreementId, err)))
	} else if ag == nil {
		glog.V(3).Infof(BAWlogstring(workerId, fmt.Sprintf("nothing to terminate for agreement %v, no database record.", agreementId)))
	} else {
		bcType, bcName, bcOrg := cph.GetKnownBlockchain(ag)
		if cph.IsBlockchainWritable(bcType, bcName, bcOrg) {
			b.DoAsyncCancel(cph, ag, reason, workerId)

		} else {
			glog.V(3).Infof(BAWlogstring(workerId, fmt.Sprintf("deferring blockchain cancel for %v", agreementId)))
			cph.DeferCommand(AsyncCancelAgreement{
				workType:    ASYNC_CANCEL,
				AgreementId: agreementId,
				Protocol:    cph.Name(),
				Reason:      reason,
			})
		}
	}
}

func (b *BaseAgreementWorker) DoAsyncCancel(cph ConsumerProtocolHandler, ag *persistence.Agreement, reason uint, workerId string) {

	glog.V(3).Infof(BAWlogstring(workerId, fmt.Sprintf("starting async cancel for %v", ag.CurrentAgreementId)))
	// This routine does not need to be a subworker because it will terminate on its own.
	go cph.TerminateAgreement(ag, reason, workerId)

}

var BAWlogstring = func(workerID string, v interface{}) string {
	return fmt.Sprintf("Base Agreement Worker (%v): %v", workerID, v)
}

// This function checks the Exchange for every declared HA partner to verify that the partner is registered in the
// exchange. As long as all partners are registered, agreements can be made. The partners dont have to be up and heart
// beating, they just have to be registered. If not all partners are registered then no agreements will be attempted
// with any of the registered partners.
func (b *BaseAgreementWorker) incompleteHAGroup(cph ConsumerProtocolHandler, producerPolicy *policy.Policy) error {

	// If the HA group specification is empty, there is nothing to check.
	if len(producerPolicy.HAGroup.Partners) == 0 {
		return nil
	} else {

		// Make sure all partners are in the exchange
		for _, partnerId := range producerPolicy.HAGroup.Partners {

			if _, err := GetDevice(b.config.Collaborators.HTTPClientFactory.NewHTTPClient(nil), partnerId, b.config.AgreementBot.ExchangeURL, cph.GetExchangeId(), cph.GetExchangeToken()); err != nil {
				return errors.New(fmt.Sprintf("could not obtain device %v from the exchange: %v", partnerId, err))
			}
		}
		return nil

	}
}

// Legacy function. Ignore devices that export specificly known configured properties.
func (b *BaseAgreementWorker) ignoreDevice(pol *policy.Policy) (bool, error) {

	for _, prop := range pol.Properties {
		if listContains(b.config.AgreementBot.IgnoreContractWithAttribs, prop.Name) {
			return true, nil
		}
	}
	return false, nil
}
