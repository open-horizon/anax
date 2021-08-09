package compcheck

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/businesspolicy"
	"github.com/open-horizon/anax/common"
	"github.com/open-horizon/anax/containermessage"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/semanticversion"
	"golang.org/x/text/message"
	"strings"
)

// **** Note: All the errors returned by the functions in this component are of type *CompCheckError,
// You can cast it by err.(*compcheck.CompCheckError) if you want to get the error code.

// error code for compatibility check error: CompCheckError
const (
	COMPCHECK_INPUT_ERROR      = 10
	COMPCHECK_VALIDATION_ERROR = 11
	COMPCHECK_CONVERSION_ERROR = 12
	COMPCHECK_MERGING_ERROR    = 13
	COMPCHECK_EXCHANGE_ERROR   = 14
	COMPCHECK_GENERAL_ERROR    = 15
)

// Error for policy compatibility check
type CompCheckError struct {
	Err     string `json:"error"`
	ErrCode int    `json:"error_code,omitempty"`
}

func (e *CompCheckError) Error() string {
	if e == nil {
		return ""
	} else {
		return fmt.Sprintf("%v", e.Err)
	}
}

func (e *CompCheckError) String() string {
	if e == nil {
		return ""
	} else {
		return fmt.Sprintf("ErrCode: %v, Error: %v", e.ErrCode, e.Err)
	}
}

func NewCompCheckError(err error, errCode int) *CompCheckError {
	return &CompCheckError{
		Err:     err.Error(),
		ErrCode: errCode,
	}
}

// The input format for the comptible check
type CompCheck struct {
	NodeId         string                         `json:"node_id,omitempty"`
	NodeArch       string                         `json:"node_arch,omitempty"`
	NodeType       string                         `json:"node_type,omitempty"` // can be omitted if node_id is specified
	NodeOrg        string                         `json:"node_org,omitempty"`  // can be omitted if node_id is specified
	NodePolicy     *externalpolicy.ExternalPolicy `json:"node_policy,omitempty"`
	NodeUserInput  []policy.UserInput             `json:"node_user_input,omitempty"`
	BusinessPolId  string                         `json:"business_policy_id,omitempty"`
	BusinessPolicy *businesspolicy.BusinessPolicy `json:"business_policy,omitempty"`
	PatternId      string                         `json:"pattern_id,omitempty"`
	Pattern        common.AbstractPatternFile     `json:"pattern,omitempty"`
	ServicePolicy  *externalpolicy.ExternalPolicy `json:"service_policy,omitempty"`
	Service        []common.AbstractServiceFile   `json:"service,omitempty"`
}

func (p CompCheck) String() string {
	return fmt.Sprintf("NodeId: %v, NodeArch: %v, NodeType: %v, NodePolicy: %v, NodeUserInput: %v, BusinessPolId: %v, BusinessPolicy: %v, PatternId: %v, Pattern: %v, ServicePolicy: %v, Service: %v",
		p.NodeId, p.NodeArch, p.NodeType, p.NodePolicy, p.NodeUserInput, p.BusinessPolId, p.BusinessPolicy, p.PatternId, p.Pattern, p.ServicePolicy, p.Service)

}

// stucture for the readining of the inital input for agbot secure APIs. It
// is used by the unmarshal handers (UnmarshalJSON) of CompCheck, PolicyCheck, UserInputCheck
// and SecretBindingCheck that only the agbot secure APIs will use.
// This is because the original structures contain interfaces AbstractPatternFile and
// AbstractServiceFile that cannot be demarshaled into.
type CompCheck_NoAbstract struct {
	NodeId         string                         `json:"node_id,omitempty"`
	NodeArch       string                         `json:"node_arch,omitempty"`
	NodeType       string                         `json:"node_type,omitempty"` // can be omitted if node_id is specified
	NodeOrg        string                         `json:"node_org,omitempty"`  // can be omitted if node_id is specified
	NodePolicy     *externalpolicy.ExternalPolicy `json:"node_policy,omitempty"`
	NodeUserInput  []policy.UserInput             `json:"node_user_input,omitempty"`
	BusinessPolId  string                         `json:"business_policy_id,omitempty"`
	BusinessPolicy *businesspolicy.BusinessPolicy `json:"business_policy,omitempty"`
	PatternId      string                         `json:"pattern_id,omitempty"`
	Pattern        *common.PatternFile            `json:"pattern,omitempty"`
	ServicePolicy  *externalpolicy.ExternalPolicy `json:"service_policy,omitempty"`
	Service        []common.ServiceFile           `json:"service,omitempty"`
}

// unmashal handler for CompCheck object to handle AbstractPatternFile and AbstractServiceFile
func (p *CompCheck) UnmarshalJSON(b []byte) error {

	var cc CompCheck_NoAbstract
	if err := json.Unmarshal(b, &cc); err != nil {
		return err
	}

	p.NodeId = cc.NodeId
	p.NodeArch = cc.NodeArch
	p.NodeType = cc.NodeType
	p.NodeOrg = cc.NodeOrg
	p.NodePolicy = cc.NodePolicy
	p.NodeUserInput = cc.NodeUserInput
	p.BusinessPolId = cc.BusinessPolId
	p.BusinessPolicy = cc.BusinessPolicy
	p.PatternId = cc.PatternId
	p.ServicePolicy = cc.ServicePolicy

	if cc.Pattern != nil {
		p.Pattern = cc.Pattern
	}

	if cc.Service != nil && len(cc.Service) != 0 {
		p.Service = []common.AbstractServiceFile{}
		for index, _ := range cc.Service {
			p.Service = append(p.Service, &cc.Service[index])
		}
	}

	return nil
}

// CompCheckOutput The output format for the compatibility check
// swagger:model
type CompCheckOutput struct {
	Compatible bool               `json:"compatible"`
	Reason     map[string]string  `json:"reason"` // set when not compatible
	Input      *CompCheckResource `json:"input,omitempty"`
}

func (p *CompCheckOutput) String() string {
	return fmt.Sprintf("Compatible: %v, Reason: %v, Input: %v",
		p.Compatible, p.Reason, p.Input)

}

func NewCompCheckOutput(compatible bool, reason map[string]string, input *CompCheckResource) *CompCheckOutput {
	return &CompCheckOutput{
		Compatible: compatible,
		Reason:     reason,
		Input:      input,
	}
}

// To store the resource (pattern, bp, services etc) used for compatibility check
type CompCheckResource struct {
	NodeId         string                                   `json:"node_id,omitempty"`
	NodeArch       string                                   `json:"node_arch,omitempty"`
	NodeType       string                                   `json:"node_type,omitempty"`
	NodeOrg        string                                   `json:"node_org,omitempty"`
	NodePolicy     *externalpolicy.ExternalPolicy           `json:"node_policy,omitempty"`
	NodeUserInput  []policy.UserInput                       `json:"node_user_input,omitempty"`
	BusinessPolId  string                                   `json:"business_policy_id,omitempty"`
	BusinessPolicy *businesspolicy.BusinessPolicy           `json:"business_policy,omitempty"`
	PatternId      string                                   `json:"pattern_id,omitempty"`
	Pattern        common.AbstractPatternFile               `json:"pattern,omitempty"`
	ServicePolicy  map[string]externalpolicy.ExternalPolicy `json:"service_policy,omitempty"`
	Service        []common.AbstractServiceFile             `json:"service,omitempty"`
	DepServices    map[string]exchange.ServiceDefinition    `json:"dependent_services,omitempty"` // for internal use for performance. A map of service definition keyed by id.
	// It is either empty or provides ALL the dependent services needed. It is expected the top level service definitions are provided
	// in the 'Service' attribute when this attribute is not empty.
	NeededSB     []exchangecommon.SecretBinding `json:"needed_secret_binding,omitempty"`
	ExtraneousSB []exchangecommon.SecretBinding `json:"extraneous_secret_binding,omitempty"`
}

func (p CompCheckResource) String() string {
	return fmt.Sprintf("NodeId: %v, NodeArch: %v, NodeType: %v, NodePolicy: %v, NodeUserInput: %v, BusinessPolId: %v, BusinessPolicy: %v, PatternId: %v, Pattern: %v, ServicePolicy: %v, Service: %v",
		p.NodeId, p.NodeArch, p.NodeType, p.NodePolicy, p.NodeUserInput, p.BusinessPolId, p.BusinessPolicy, p.PatternId, p.Pattern, p.ServicePolicy, p.Service)

}

func NewCompCheckResourceFromUICheck(uiInput *UserInputCheck) *CompCheckResource {
	var rsrc CompCheckResource
	rsrc.NodeId = uiInput.NodeId
	rsrc.NodeArch = uiInput.NodeArch
	rsrc.NodeType = uiInput.NodeType
	rsrc.NodeUserInput = uiInput.NodeUserInput
	rsrc.BusinessPolId = uiInput.BusinessPolId
	rsrc.BusinessPolicy = uiInput.BusinessPolicy
	rsrc.PatternId = uiInput.PatternId

	// change the type from PatternFile to AbstractPatternFile
	rsrc.Pattern = uiInput.Pattern

	// copy the top level services if any
	if uiInput.Service != nil {
		rsrc.Service = []common.AbstractServiceFile{}
		for _, svc := range uiInput.Service {
			rsrc.Service = append(rsrc.Service, svc)
		}
	}

	// copy the dependent services if any
	if uiInput.DepServices != nil {
		rsrc.DepServices = map[string]exchange.ServiceDefinition{}
		for sId, svc := range uiInput.DepServices {
			rsrc.DepServices[sId] = svc
		}
	}

	return &rsrc
}

func NewCompCheckRsrcFromSecretBindingCheck(sbInput *SecretBindingCheck) *CompCheckResource {
	var rsrc CompCheckResource
	rsrc.NodeId = sbInput.NodeId
	rsrc.NodeArch = sbInput.NodeArch
	rsrc.NodeType = sbInput.NodeType
	rsrc.NodeOrg = sbInput.NodeOrg
	rsrc.BusinessPolId = sbInput.BusinessPolId
	rsrc.BusinessPolicy = sbInput.BusinessPolicy
	rsrc.PatternId = sbInput.PatternId

	// change the type from PatternFile to AbstractPatternFile
	rsrc.Pattern = sbInput.Pattern

	// copy the top level services if any
	if sbInput.Service != nil {
		rsrc.Service = []common.AbstractServiceFile{}
		for _, svc := range sbInput.Service {
			rsrc.Service = append(rsrc.Service, svc)
		}
	}

	// copy the dependent services if any
	if sbInput.DepServices != nil {
		rsrc.DepServices = map[string]exchange.ServiceDefinition{}
		for sId, svc := range sbInput.DepServices {
			rsrc.DepServices[sId] = svc
		}
	}
	return &rsrc
}

func NewCompCheckResourceFromPolicyCheck(pcInput *PolicyCheck) *CompCheckResource {
	var rsrc CompCheckResource
	rsrc.NodeId = pcInput.NodeId
	rsrc.NodeArch = pcInput.NodeArch
	rsrc.NodeType = pcInput.NodeType
	rsrc.NodePolicy = pcInput.NodePolicy
	rsrc.BusinessPolId = pcInput.BusinessPolId
	rsrc.BusinessPolicy = pcInput.BusinessPolicy

	// If user input a service policy, it will be applied to all the services defined in the bp or pattern
	rsrc.ServicePolicy = map[string]externalpolicy.ExternalPolicy{}
	if pcInput.ServicePolicy != nil {
		rsrc.ServicePolicy["AllServices"] = *pcInput.ServicePolicy
	}

	// copy the top level services if any
	if pcInput.Service != nil {
		rsrc.Service = []common.AbstractServiceFile{}
		for _, svc := range pcInput.Service {
			rsrc.Service = append(rsrc.Service, svc)
		}
	}

	// copy the dependent services if any
	if pcInput.DepServices != nil {
		rsrc.DepServices = map[string]exchange.ServiceDefinition{}
		for sId, svc := range pcInput.DepServices {
			rsrc.DepServices[sId] = svc
		}
	}

	return &rsrc
}

func DeployCompatible(ec exchange.ExchangeContext, agbotUrl string, ccInput *CompCheck, checkAllSvcs bool, msgPrinter *message.Printer) (*CompCheckOutput, error) {

	getDeviceHandler := exchange.GetHTTPDeviceHandler(ec)
	nodePolicyHandler := exchange.GetHTTPNodePolicyHandler(ec)
	getBusinessPolicies := exchange.GetHTTPBusinessPoliciesHandler(ec)
	getPatterns := exchange.GetHTTPExchangePatternHandler(ec)
	servicePolicyHandler := exchange.GetHTTPServicePolicyHandler(ec)
	getServiceHandler := exchange.GetHTTPServiceHandler(ec)
	serviceDefResolverHandler := exchange.GetHTTPServiceDefResolverHandler(ec)
	getSelectedServices := exchange.GetHTTPSelectedServicesHandler(ec)
	vaultSecretExists := exchange.GetHTTPVaultSecretExistsHandler(ec)

	return deployCompatible(getDeviceHandler, nodePolicyHandler, getBusinessPolicies, getPatterns, servicePolicyHandler, getServiceHandler, serviceDefResolverHandler, getSelectedServices, vaultSecretExists, agbotUrl, ccInput, checkAllSvcs, msgPrinter)
}

// Internal function for PolicyCompatible
func deployCompatible(getDeviceHandler exchange.DeviceHandler,
	nodePolicyHandler exchange.NodePolicyHandler,
	getBusinessPolicies exchange.BusinessPoliciesHandler,
	getPatterns exchange.PatternHandler,
	servicePolicyHandler exchange.ServicePolicyHandler,
	getServiceHandler exchange.ServiceHandler,
	serviceDefResolverHandler exchange.ServiceDefResolverHandler,
	getSelectedServices exchange.SelectedServicesHandler,
	vaultSecretExists exchange.VaultSecretExistsHandler, agbotUrl string,
	ccInput *CompCheck, checkAllSvcs bool, msgPrinter *message.Printer) (*CompCheckOutput, error) {

	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	useBPol := false
	if ccInput.BusinessPolId != "" || ccInput.BusinessPolicy != nil {
		useBPol = true
		if ccInput.PatternId != "" || ccInput.Pattern != nil {
			return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Deployment policy and pattern are mutually exclusive.")), COMPCHECK_INPUT_ERROR)
		}
	} else {
		if ccInput.PatternId == "" && ccInput.Pattern == nil {
			return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Neither deployment policy nor pattern is specified.")), COMPCHECK_INPUT_ERROR)
		}
	}

	// check policy first, for business policy case only
	policyCheckInput := convertToPolicyCheck(ccInput)

	pcOutput := NewCompCheckOutput(true, map[string]string{}, NewCompCheckResourceFromPolicyCheck(policyCheckInput))
	privOutput := NewCompCheckOutput(true, map[string]string{}, &CompCheckResource{})
	if useBPol {
		var err1 error
		pcOutput, err1 = policyCompatible(getDeviceHandler, nodePolicyHandler, getBusinessPolicies, servicePolicyHandler, getSelectedServices, serviceDefResolverHandler, policyCheckInput, true, msgPrinter)
		if err1 != nil {
			return nil, err1
		}

		// if not compatible, then do not bother to do user input check
		if !pcOutput.Compatible {
			return pcOutput, nil
		}
	} else if ccInput.NodeId != "" || ccInput.NodePolicy != nil {
		privOutput, err := EvaluatePatternPrivilegeCompatability(serviceDefResolverHandler, getPatterns, nodePolicyHandler, ccInput, privOutput.Input, msgPrinter, true, true)
		if err != nil {
			return nil, err
		}
		if !privOutput.Compatible {
			return createCompCheckOutput(pcOutput, privOutput, nil, nil, checkAllSvcs, msgPrinter), nil
		}
	}

	// check user input for those services that are compatible
	uiCheckInput := createUserInputCheckInput(ccInput, pcOutput, msgPrinter)
	uiOutput, err := userInputCompatible(getDeviceHandler, getBusinessPolicies, getPatterns, serviceDefResolverHandler, getSelectedServices, uiCheckInput, true, msgPrinter)
	if err != nil {
		return nil, err
	}
	if !uiOutput.Compatible {
		return createCompCheckOutput(pcOutput, privOutput, uiOutput, nil, checkAllSvcs, msgPrinter), nil
	}

	var sbOutput *CompCheckOutput
	if uiOutput.Input.NodeType == persistence.DEVICE_TYPE_DEVICE {
		var err error
		// check the secret bindings for those that are compatible
		sbCheckInput := createSecretBindingCheckInput(ccInput, uiOutput, msgPrinter)
		sbOutput, err = secretBindingCompatible(getDeviceHandler, getBusinessPolicies, getPatterns, serviceDefResolverHandler, getSelectedServices, vaultSecretExists, agbotUrl, sbCheckInput, checkAllSvcs, msgPrinter)
		if err != nil {
			return nil, err
		}
	}

	// combine the output of policy check, user input check, secretbinding check into one
	return createCompCheckOutput(pcOutput, privOutput, uiOutput, sbOutput, checkAllSvcs, msgPrinter), nil
}

// Use the result of policy check together with the original input to create a user input check input object.
func createUserInputCheckInput(ccInput *CompCheck, pcOutput *CompCheckOutput, msgPrinter *message.Printer) *UserInputCheck {
	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	uiCheckInput := UserInputCheck{}

	// get some policies from the policy check output so that we do not need to get to the exchange to get them again.
	uiCheckInput.NodeId = pcOutput.Input.NodeId
	uiCheckInput.NodeArch = pcOutput.Input.NodeArch
	uiCheckInput.NodeType = pcOutput.Input.NodeType
	uiCheckInput.BusinessPolId = pcOutput.Input.BusinessPolId
	uiCheckInput.BusinessPolicy = pcOutput.Input.BusinessPolicy

	// these are from the original input
	uiCheckInput.NodeUserInput = ccInput.NodeUserInput
	uiCheckInput.PatternId = ccInput.PatternId
	uiCheckInput.Pattern = ccInput.Pattern
	if ccInput.Service != nil && len(ccInput.Service) != 0 {
		uiCheckInput.Service = ccInput.Service
	} else {
		uiCheckInput.Service = pcOutput.Input.Service
	}

	// limit the check to the services that are compatible for policy check
	// empty means check all
	svcs := []string{}
	for sId, reason := range pcOutput.Reason {
		if !strings.Contains(reason, msgPrinter.Sprintf("Incompatible")) {
			svcs = append(svcs, sId)
		}
	}
	uiCheckInput.ServiceToCheck = svcs

	// get service cache
	uiCheckInput.DepServices = pcOutput.Input.DepServices

	return &uiCheckInput
}

// Use the result of userinput check together with the original input to create a secret binding check input object.
func createSecretBindingCheckInput(ccInput *CompCheck, uiOutput *CompCheckOutput, msgPrinter *message.Printer) *SecretBindingCheck {
	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	sbCheckInput := SecretBindingCheck{}

	// get policies or patten from the userinput check output so that we do not need to get to the exchange to get them again.
	sbCheckInput.NodeId = uiOutput.Input.NodeId
	sbCheckInput.NodeArch = uiOutput.Input.NodeArch
	sbCheckInput.NodeType = uiOutput.Input.NodeType
	sbCheckInput.BusinessPolId = uiOutput.Input.BusinessPolId
	sbCheckInput.BusinessPolicy = uiOutput.Input.BusinessPolicy
	sbCheckInput.PatternId = uiOutput.Input.PatternId
	sbCheckInput.Pattern = uiOutput.Input.Pattern

	// these are from the original input
	sbCheckInput.NodeOrg = ccInput.NodeOrg

	// limit the check to the services that are compatible for previous checks
	// empty means check all
	svcs := []string{}
	for sId, reason := range uiOutput.Reason {
		if !strings.Contains(reason, msgPrinter.Sprintf("Incompatible")) {
			svcs = append(svcs, sId)
		}
	}
	sbCheckInput.ServiceToCheck = svcs

	// get the service cache
	sbCheckInput.DepServices = uiOutput.Input.DepServices
	sbCheckInput.Service = uiOutput.Input.Service

	return &sbCheckInput
}

// update the reason map from the deployment check output
func updateReasonMap(cco *CompCheckOutput, reason map[string]string, msgPrinter *message.Printer) {
	if cco.Reason == nil || len(cco.Reason) == 0 {
		return
	}

	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}
	msg_compatible := msgPrinter.Sprintf("Compatible")
	for sId, rs := range cco.Reason {
		if strings.HasSuffix(sId, "_*") || strings.HasSuffix(sId, "_") {
			sId = cutil.RemoveArchFromServiceId(sId)
		}
		if _, ok := reason[sId]; !ok {
			// add it in if it is not in the map
			reason[sId] = rs
		} else if rs != msg_compatible {
			// keep the latest in comptible code
			reason[sId] = rs
		}
	}
}

// combine the policy check output and user input output into CompCheckOutput
func createCompCheckOutput(pcOutput *CompCheckOutput, privOutput *CompCheckOutput, uiOutput *CompCheckOutput, sbOutput *CompCheckOutput, checkAllSvcs bool, msgPrinter *message.Printer) *CompCheckOutput {
	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	var ccOutput CompCheckOutput
	var lastOutput *CompCheckOutput

	// the last one takes precedence for overall assesement.
	// sbOutput can be nil when node type is cluster because
	// scretebinding check is not supported
	if sbOutput != nil {
		lastOutput = sbOutput
	} else if uiOutput != nil {
		lastOutput = uiOutput
	} else {
		lastOutput = privOutput
	}
	ccOutput.Compatible = lastOutput.Compatible

	// combine the reason from all
	reason := map[string]string{}
	if !checkAllSvcs && lastOutput.Compatible {
		reason = lastOutput.Reason
	} else {
		updateReasonMap(pcOutput, reason, msgPrinter)
		updateReasonMap(privOutput, reason, msgPrinter)
		if uiOutput != nil {
			updateReasonMap(uiOutput, reason, msgPrinter)
		}
		if sbOutput != nil {
			updateReasonMap(sbOutput, reason, msgPrinter)
		}
	}

	ccOutput.Reason = reason

	// combine the input part
	ccInput := CompCheckResource{}
	ccInput.NodeId = lastOutput.Input.NodeId
	ccInput.NodeArch = lastOutput.Input.NodeArch
	ccInput.NodeType = lastOutput.Input.NodeType
	ccInput.NodeOrg = lastOutput.Input.NodeOrg
	if uiOutput != nil {
		ccInput.NodeUserInput = uiOutput.Input.NodeUserInput
		ccInput.PatternId = uiOutput.Input.PatternId
		ccInput.Pattern = uiOutput.Input.Pattern
	}
	ccInput.NodePolicy = pcOutput.Input.NodePolicy
	ccInput.BusinessPolId = lastOutput.Input.BusinessPolId
	ccInput.BusinessPolicy = lastOutput.Input.BusinessPolicy
	ccInput.Service = lastOutput.Input.Service
	ccInput.ServicePolicy = pcOutput.Input.ServicePolicy
	ccOutput.Input = &ccInput

	if sbOutput != nil {
		ccOutput.Input.NeededSB = sbOutput.Input.NeededSB
		ccOutput.Input.ExtraneousSB = sbOutput.Input.ExtraneousSB
	}

	return &ccOutput
}

// convert CompCheck object to PolicyCheckobject.
func convertToPolicyCheck(in *CompCheck) *PolicyCheck {
	var out PolicyCheck

	out.NodeId = in.NodeId
	out.NodeArch = in.NodeArch
	out.NodeType = in.NodeType
	out.NodePolicy = in.NodePolicy
	out.BusinessPolId = in.BusinessPolId
	out.BusinessPolicy = in.BusinessPolicy
	out.ServicePolicy = in.ServicePolicy
	out.Service = in.Service

	return &out
}

// Get the exchange device
func GetExchangeNode(getDeviceHandler exchange.DeviceHandler, nodeId string, msgPrinter *message.Printer) (*exchange.Device, error) {
	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	// check input
	if nodeId == "" {
		return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("The given node id is empty.")), COMPCHECK_INPUT_ERROR)
	} else {
		nodeOrg := exchange.GetOrg(nodeId)
		if nodeOrg == "" {
			return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Organization is not specified in the given node id: %v.", nodeId)), COMPCHECK_INPUT_ERROR)
		}
	}

	if node, err := getDeviceHandler(nodeId, ""); err != nil {
		return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Error getting node %v from the Exchange. %v", nodeId, err)), COMPCHECK_EXCHANGE_ERROR)
	} else if node == nil {
		return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("No node found for this node id %v.", nodeId)), COMPCHECK_INPUT_ERROR)
	} else {
		return node, nil
	}
}

// EvaluatePatternPrivilegeCompatability determines if a given node requires a workload that uses privileged mode or network=host.
// This function will recursively evaluate top-level services specified in the pattern.
func EvaluatePatternPrivilegeCompatability(getServiceResolvedDef exchange.ServiceDefResolverHandler,
	getPatterns exchange.PatternHandler, nodePolicyHandler exchange.NodePolicyHandler,
	cc *CompCheck, ccResource *CompCheckResource, msgPrinter *message.Printer, checkAll bool, quiet bool) (*CompCheckOutput, error) {

	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	if cc.NodeId == "" && cc.NodePolicy == nil {
		return nil, nil
	}
	nodePol, _, err := processNodePolicy(nodePolicyHandler, cc.NodeId, cc.NodePolicy, msgPrinter)
	if err != nil {
		return nil, err
	}
	nodePriv := false
	if nodePol.Properties.HasProperty(externalpolicy.PROP_NODE_PRIVILEGED) {
		privProv, err := nodePol.Properties.GetProperty(externalpolicy.PROP_NODE_PRIVILEGED)
		if err != nil {
			return nil, err
		}
		nodePriv = privProv.Value.(bool)
	}
	if nodePriv {
		return NewCompCheckOutput(true, nil, nil), nil
	}

	patternDef, err := processPattern(getPatterns, cc.PatternId, cc.Pattern, msgPrinter)
	if err != nil {
		return nil, err
	}

	svcComp := []common.AbstractServiceFile{}
	svcIncomp := []common.AbstractServiceFile{}
	messages := map[string]string{}
	overallPriv := false
	for _, svcRef := range getWorkloadsFromPattern(patternDef, ccResource.NodeArch) {
		svcPriv := false
		for _, workload := range svcRef.ServiceVersions {
			topSvc, topId, depSvcs, err := GetServiceAndDeps(svcRef.ServiceURL, svcRef.ServiceOrg, workload.Version, svcRef.ServiceArch, cc.Service,
				ccResource.DepServices, getServiceResolvedDef, msgPrinter)
			if err != nil && !quiet {
				return nil, err
			}

			// check the compatibilit of the privileged settings
			workLoadPriv, err, privWorkloads := ServicesRequirePrivilege(topSvc, topId, depSvcs, msgPrinter)
			if err != nil {
				return nil, err
			}

			// save the incompatibles
			for _, sId := range privWorkloads {
				if topId == sId {
					svcIncomp = append(svcIncomp, topSvc)
				} else {
					sDef := &ServiceDefinition{exchange.GetOrg(sId), depSvcs[sId]}
					svcIncomp = append(svcIncomp, sDef)
				}
			}

			if workLoadPriv && checkAll {
				svcPriv = true
				messages[topId] = fmt.Sprintf("Version %s of this service requires the following workloads that will only run on a privileged node. %v", workload.Version, privWorkloads)
			} else if workLoadPriv {
				ccResource.Service = svcIncomp
				return NewCompCheckOutput(false, map[string]string{topId: fmt.Sprintf("Version %s of this service requires the following workloads that will only run on a privileged node. %v", workload.Version, privWorkloads)}, nil), nil
			} else {
				svcComp = append(svcComp, topSvc)
			}
		}
		if svcPriv {
			overallPriv = true
		}
	}
	if overallPriv {
		ccResource.Service = svcComp
	} else {
		ccResource.Service = svcIncomp
	}

	if !nodePriv && overallPriv {
		return NewCompCheckOutput(false, messages, ccResource), nil
	}
	return NewCompCheckOutput(true, nil, ccResource), nil
}

// this function takes as a parameter a map of service ids to deployment strings
// Returns true if any of the service listed require privilge to run
// returns a slice of service ids of any services that require privilege to run
func ServicesRequirePrivilege(topSvc common.AbstractServiceFile, topSvcId string, depServiceDefs map[string]exchange.ServiceDefinition, msgPrinter *message.Printer) (bool, error, []string) {
	privSvcs := []string{}
	reqPriv := false

	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	// handle top level service
	if topSvc != nil {
		depstring := ""
		if _, ok := topSvc.GetDeployment().(string); ok {
			depstring = topSvc.GetDeployment().(string)
		} else {
			depByte, err := json.Marshal(topSvc.GetDeployment())
			if err != nil {
				return false, err, nil
			}
			depstring = string(depByte)
		}

		if priv, err := DeploymentRequiresPrivilege(depstring, msgPrinter); err != nil {
			return false, err, nil
		} else if priv {
			reqPriv = true
			privSvcs = append(privSvcs, topSvcId)
		}
	}

	// handle dependent services
	if depServiceDefs != nil {
		for sId, sDef := range depServiceDefs {
			if priv, err := DeploymentRequiresPrivilege(sDef.GetDeploymentString(), msgPrinter); err != nil {
				return false, err, nil
			} else if priv {
				reqPriv = true
				privSvcs = append(privSvcs, sId)
			}
		}
	}
	return reqPriv, nil, privSvcs
}

// Check if the deployment string given uses the privileged flag or network=host
func DeploymentRequiresPrivilege(deploymentString string, msgPrinter *message.Printer) (bool, error) {
	if deploymentString == "" {
		return false, nil
	}
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}
	deploymentStruct := &containermessage.DeploymentDescription{}
	err := json.Unmarshal([]byte(deploymentString), deploymentStruct)
	if err != nil {
		return false, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Error unmarshaling deployment string to internal deployment structure: %v", err)), COMPCHECK_CONVERSION_ERROR)
	}
	for _, topSvc := range deploymentStruct.Services {
		if topSvc != nil {
			if topSvc.Privileged || topSvc.Network == "host" {
				return true, nil
			}
		}
	}
	return false, nil
}

// verifies the input node type has valid value and it matches the exchange node type.
func VerifyNodeType(nodeType string, exchNodeType string, nodeId string, msgPrinter *message.Printer) (string, error) {
	if nodeType != "" {
		if nodeType != persistence.DEVICE_TYPE_DEVICE && nodeType != persistence.DEVICE_TYPE_CLUSTER {
			return "", NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Invalid node type: %v. It must be 'device' or 'cluster'.", nodeType)), COMPCHECK_INPUT_ERROR)
		} else if exchNodeType != "" && nodeType != exchNodeType {
			return "", NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("The input node type '%v' does not match the node type '%v' from the node %v.", nodeType, exchNodeType, nodeId)), COMPCHECK_INPUT_ERROR)
		}
		return nodeType, nil
	} else {
		if exchNodeType != "" {
			return exchNodeType, nil
		} else {
			return persistence.DEVICE_TYPE_DEVICE, nil
		}
	}
}

// Check if the node type is compatible with the serivce
func CheckTypeCompatibility(nodeType string, serviceDef common.AbstractServiceFile, msgPrinter *message.Printer) (bool, string) {
	if (nodeType == "" || nodeType == persistence.DEVICE_TYPE_DEVICE) && common.DeploymentIsEmpty(serviceDef.GetDeployment()) {
		return false, msgPrinter.Sprintf("Service does not have deployment configuration for node type 'device'.")
	}
	if nodeType == persistence.DEVICE_TYPE_CLUSTER && common.DeploymentIsEmpty(serviceDef.GetClusterDeployment()) {
		return false, msgPrinter.Sprintf("Service does not have cluster deployment configuration for node type 'cluster'.")
	}

	return true, ""
}

// Get the dependent services for the given service.
// It goes to the dependentServices to find a dependent first. If not found
// it will go to the exchange to get the dependents.
func GetServiceDependentDefs(sDef common.AbstractServiceFile,
	dependentServices map[string]exchange.ServiceDefinition, // can be nil
	serviceDefResolverHandler exchange.ServiceDefResolverHandler,
	msgPrinter *message.Printer) (map[string]exchange.ServiceDefinition, error) {

	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	// nothing to check
	if sDef == nil {
		return nil, nil
	}

	// get all the service defs for the dependent services for device type node
	service_map := map[string]exchange.ServiceDefinition{}
	if sDef.GetRequiredServices() != nil && len(sDef.GetRequiredServices()) != 0 {
		for _, sDep := range sDef.GetRequiredServices() {
			var s_map map[string]exchange.ServiceDefinition
			var s_def *exchange.ServiceDefinition
			var err error
			var s_id string
			svcSpec := NewServiceSpec(sDep.URL, sDep.Org, sDep.VersionRange, sDep.Arch)
			if s_map, err = FindServiceDefs(svcSpec, dependentServices, msgPrinter); err != nil {
				return nil, NewCompCheckError(err, COMPCHECK_GENERAL_ERROR)
			} else if vExp, err := semanticversion.Version_Expression_Factory(sDep.VersionRange); err != nil {
				return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Unable to create version expression from %v. %v", sDep.VersionRange, err)), COMPCHECK_GENERAL_ERROR)
			} else if s_map, s_def, s_id, err = serviceDefResolverHandler(sDep.URL, sDep.Org, vExp.Get_expression(), sDep.Arch); err != nil {
				return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Error retrieving dependent services from the Exchange for %v. %v", sDep, err)), COMPCHECK_EXCHANGE_ERROR)
			} else {
				service_map[s_id] = *s_def
			}
			for id, s := range s_map {
				service_map[id] = s
			}
		}
	}

	return service_map, nil
}

// Find the a service and all of its dependent services from the given service map. The version given in the
// svcSpec is a version range.
func FindServiceDefs(svcSpec *ServiceSpec, sDefsIn map[string]exchange.ServiceDefinition, msgPrinter *message.Printer) (map[string]exchange.ServiceDefinition, error) {
	if sDefsIn == nil || len(sDefsIn) == 0 {
		return nil, nil
	}

	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	found := true
	all := map[string]exchange.ServiceDefinition{}
	for sID, sDef := range sDefsIn {
		if sDef.URL == svcSpec.ServiceUrl && exchange.GetOrg(sID) == svcSpec.ServiceOrgid && sDef.Arch == svcSpec.ServiceArch {
			if vExp, err := semanticversion.Version_Expression_Factory(svcSpec.ServiceVersionRange); err != nil {
				return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Unable to create version expression from %v. %v", svcSpec.ServiceVersionRange, err)), COMPCHECK_GENERAL_ERROR)
			} else if ok, err := vExp.Is_within_range(sDef.Version); err != nil {
				return nil, NewCompCheckError(err, COMPCHECK_GENERAL_ERROR)
			} else if ok {
				found = true
				all[sID] = sDefsIn[sID]

				if sDef.RequiredServices != nil && len(sDef.RequiredServices) != 0 {
					for _, sDep := range sDef.RequiredServices {
						svcSpecDep := NewServiceSpec(sDep.URL, sDep.Org, sDep.VersionRange, sDep.Arch)
						if defs, err := FindServiceDefs(svcSpecDep, sDefsIn, msgPrinter); err != nil || defs == nil {
							return nil, err
						} else {
							for id, s := range defs {
								all[id] = s
							}
						}
					}
				}
			}
		}
	}

	if !found {
		return nil, nil
	} else {
		return all, nil
	}
}

// Get a specific service from the given array of services
func GetServiceFromInput(svcUrl, svcOrg, svcVersion, svcArch string, inServices []common.AbstractServiceFile) common.AbstractServiceFile {
	for _, in_svc := range inServices {
		if in_svc.GetURL() == svcUrl && in_svc.GetVersion() == svcVersion &&
			(svcArch == "*" || svcArch == "" || in_svc.GetArch() == svcArch) &&
			(in_svc.GetOrg() == "" || in_svc.GetOrg() == svcOrg) {
			return in_svc
		}
	}

	return nil
}

// Given the a list of top level services and dependent servces, find the
// top level service and its dependent services. If not found, get from the exchange.
func GetServiceAndDeps(svcUrl, svcOrg, svcVersion, svcArch string,
	inServices []common.AbstractServiceFile, inDepServices map[string]exchange.ServiceDefinition,
	getServiceResolvedDef exchange.ServiceDefResolverHandler,
	msgPrinter *message.Printer) (common.AbstractServiceFile, string, map[string]exchange.ServiceDefinition, error) {

	var topSvc common.AbstractServiceFile
	var exchTopSvc *exchange.ServiceDefinition
	topId := ""
	depSvcs := map[string]exchange.ServiceDefinition{}
	var err error

	// get services
	topSvc = GetServiceFromInput(svcUrl, svcOrg, svcVersion, svcArch, inServices)
	if topSvc != nil {
		// found in input or exchange
		topId = cutil.FormExchangeIdForService(svcUrl, svcVersion, svcArch)
		topId = fmt.Sprintf("%v/%v", svcOrg, topId)
		depSvcs, err = GetServiceDependentDefs(topSvc, inDepServices, getServiceResolvedDef, msgPrinter)
		if err != nil {
			return nil, "", nil, err
		}
	} else {
		// not found, get it and dependents from the exchange
		depSvcs, exchTopSvc, topId, err = getServiceResolvedDef(svcUrl, svcOrg, svcVersion, svcArch)
		if err != nil {
			return nil, "", nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Failed to find definition for dependent services of %s. Compatability of %s cannot be fully evaluated until all services are in the Exchange.", topId, externalpolicy.PROP_NODE_PRIVILEGED)), COMPCHECK_EXCHANGE_ERROR)
		}
		topSvc = &ServiceDefinition{exchange.GetOrg(topId), *exchTopSvc}
	}

	return topSvc, topId, depSvcs, nil
}

func FormatReasonMessage(reason string, type_error bool, msg_prefix string, type_prefix string) string {
	if type_error {
		return fmt.Sprintf("%v: %v", type_prefix, reason)
	} else {
		return fmt.Sprintf("%v: %v", msg_prefix, reason)
	}
}
