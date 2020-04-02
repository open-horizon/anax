package compcheck

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/businesspolicy"
	"github.com/open-horizon/anax/common"
	"github.com/open-horizon/anax/containermessage"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
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
	NodePolicy     *externalpolicy.ExternalPolicy `json:"node_policy,omitempty"`
	NodeUserInput  []policy.UserInput             `json:"node_user_input,omitempty"`
	BusinessPolId  string                         `json:"business_policy_id,omitempty"`
	BusinessPolicy *businesspolicy.BusinessPolicy `json:"business_policy,omitempty"`
	PatternId      string                         `json:"pattern_id,omitempty"`
	Pattern        *common.PatternFile            `json:"pattern,omitempty"`
	ServicePolicy  *externalpolicy.ExternalPolicy `json:"service_policy,omitempty"`
	Service        []common.ServiceFile           `json:"service,omitempty"`
}

func (p CompCheck) String() string {
	return fmt.Sprintf("NodeId: %v, NodeArch: %v, NodeType: %v, NodePolicy: %v, NodeUserInput: %v, BusinessPolId: %v, BusinessPolicy: %v, PatternId: %v, Pattern: %v, ServicePolicy: %v, Service: %v",
		p.NodeId, p.NodeArch, p.NodeType, p.NodePolicy, p.NodeUserInput, p.BusinessPolId, p.BusinessPolicy, p.PatternId, p.Pattern, p.ServicePolicy, p.Service)

}

// The output format for the compatibility check
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
	NodePolicy     *externalpolicy.ExternalPolicy           `json:"node_policy,omitempty"`
	NodeUserInput  []policy.UserInput                       `json:"node_user_input,omitempty"`
	BusinessPolId  string                                   `json:"business_policy_id,omitempty"`
	BusinessPolicy *businesspolicy.BusinessPolicy           `json:"business_policy,omitempty"`
	PatternId      string                                   `json:"pattern_id,omitempty"`
	Pattern        common.AbstractPatternFile               `json:"pattern,omitempty"`
	ServicePolicy  map[string]externalpolicy.ExternalPolicy `json:"service_policy,omitempty"`
	Service        []common.AbstractServiceFile             `json:"service,omitempty"`
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

	// change the service type to from ServiceFile to AbstractServiceFile
	if uiInput.Service != nil {
		rsrc.Service = []common.AbstractServiceFile{}
		for _, svc := range uiInput.Service {
			rsrc.Service = append(rsrc.Service, &svc)
		}
	}
	return &rsrc
}

func NewCompCheckResourceFromPolicyCheck(uiInput *PolicyCheck) *CompCheckResource {
	var rsrc CompCheckResource
	rsrc.NodeId = uiInput.NodeId
	rsrc.NodeArch = uiInput.NodeArch
	rsrc.NodeType = uiInput.NodeType
	rsrc.NodePolicy = uiInput.NodePolicy
	rsrc.BusinessPolId = uiInput.BusinessPolId
	rsrc.BusinessPolicy = uiInput.BusinessPolicy

	// If user input a service policy, it will be applied to all the services defined in the bp or pattern
	rsrc.ServicePolicy = map[string]externalpolicy.ExternalPolicy{}
	if uiInput.ServicePolicy != nil {
		rsrc.ServicePolicy["AllServices"] = *uiInput.ServicePolicy
	}

	// change the service type to from ServiceFile to AbstractServiceFile
	if uiInput.Service != nil {
		rsrc.Service = []common.AbstractServiceFile{}
		for _, svc := range uiInput.Service {
			rsrc.Service = append(rsrc.Service, &svc)
		}
	}

	return &rsrc
}

func DeployCompatible(ec exchange.ExchangeContext, ccInput *CompCheck, checkAllSvcs bool, msgPrinter *message.Printer) (*CompCheckOutput, error) {

	getDeviceHandler := exchange.GetHTTPDeviceHandler(ec)
	nodePolicyHandler := exchange.GetHTTPNodePolicyHandler(ec)
	getBusinessPolicies := exchange.GetHTTPBusinessPoliciesHandler(ec)
	getPatterns := exchange.GetHTTPExchangePatternHandler(ec)
	servicePolicyHandler := exchange.GetHTTPServicePolicyHandler(ec)
	getServiceHandler := exchange.GetHTTPServiceHandler(ec)
	serviceDefResolverHandler := exchange.GetHTTPServiceDefResolverHandler(ec)
	getSelectedServices := exchange.GetHTTPSelectedServicesHandler(ec)

	return deployCompatible(getDeviceHandler, nodePolicyHandler, getBusinessPolicies, getPatterns, servicePolicyHandler, getServiceHandler, serviceDefResolverHandler, getSelectedServices, ccInput, checkAllSvcs, msgPrinter)
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
	ccInput *CompCheck, checkAllSvcs bool, msgPrinter *message.Printer) (*CompCheckOutput, error) {

	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	useBPol := false
	if ccInput.BusinessPolId != "" || ccInput.BusinessPolicy != nil {
		useBPol = true
		if ccInput.PatternId != "" || ccInput.Pattern != nil {
			return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Bussiness policy and pattern are mutually exclusive.")), COMPCHECK_INPUT_ERROR)
		}
	} else {
		if ccInput.PatternId == "" && ccInput.Pattern == nil {
			return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Neither deployment policy nor pattern is specified.")), COMPCHECK_INPUT_ERROR)
		}
	}

	// check policy first, for business policy case only
	policyCheckInput, err := convertToPolicyCheck(ccInput, msgPrinter)
	if err != nil {
		return nil, err
	}

	pcOutput := NewCompCheckOutput(true, map[string]string{}, NewCompCheckResourceFromPolicyCheck(policyCheckInput))
	privOutput := NewCompCheckOutput(true, map[string]string{}, &CompCheckResource{})
	if useBPol {
		var err1 error
		pcOutput, err1 = policyCompatible(getDeviceHandler, nodePolicyHandler, getBusinessPolicies, servicePolicyHandler, getSelectedServices, getServiceHandler, serviceDefResolverHandler, policyCheckInput, true, msgPrinter)
		if err1 != nil {
			return nil, err1
		}

		// if not compatible, then do not bother to do user input check
		if !pcOutput.Compatible {
			return pcOutput, nil
		}
	} else if ccInput.NodeId != "" || ccInput.NodePolicy != nil {
		privOutput, err := EvaluatePatternPrivilegeCompatability(serviceDefResolverHandler, getServiceHandler, getPatterns, nodePolicyHandler, ccInput, &CompCheckResource{NodeArch: "amd64"}, msgPrinter, checkAllSvcs, true)
		if err != nil {
			return nil, err
		}
		if !privOutput.Compatible {
			return privOutput, nil
		}
	}

	// check user input for those services that are compatible
	uiCheckInput := createUserInputCheckInput(ccInput, pcOutput, msgPrinter)
	uiOutput, err := userInputCompatible(getDeviceHandler, getBusinessPolicies, getPatterns, getServiceHandler, serviceDefResolverHandler, getSelectedServices, uiCheckInput, checkAllSvcs, msgPrinter)
	if err != nil {
		return nil, err
	}

	// combine the output of policy check and user input check into one
	return createCompCheckOutput(pcOutput, privOutput, uiOutput, checkAllSvcs, msgPrinter), nil
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
	uiCheckInput.Service = ccInput.Service
	uiCheckInput.NodeUserInput = ccInput.NodeUserInput
	uiCheckInput.PatternId = ccInput.PatternId
	uiCheckInput.Pattern = ccInput.Pattern

	// limit the check to the services that are compatible for policy check
	// empty means check all
	svcs := []string{}
	for sId, reason := range pcOutput.Reason {
		if !strings.Contains(reason, msgPrinter.Sprintf("Incompatible")) {
			svcs = append(svcs, sId)
		}
	}
	uiCheckInput.ServiceToCheck = svcs

	return &uiCheckInput
}

// combine the policy check output and user input output into CompCheckOutput
func createCompCheckOutput(pcOutput *CompCheckOutput, privOutput *CompCheckOutput, uiOutput *CompCheckOutput, checkAllSvcs bool, msgPrinter *message.Printer) *CompCheckOutput {
	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	var ccOutput CompCheckOutput

	// the last one takes precedence for overall assesement
	ccOutput.Compatible = uiOutput.Compatible

	// combine the reason from both
	msg_compatible := msgPrinter.Sprintf("Compatible")
	reason := map[string]string{}
	if !checkAllSvcs && uiOutput.Compatible {
		reason = uiOutput.Reason
	} else {
		// save the policy incompatibility reason.
		if pcOutput.Reason == nil || len(pcOutput.Reason) == 0 {
			// pattern case, only has userinput check
			for sId_ui, rs_ui := range uiOutput.Reason {
				reason[sId_ui] = rs_ui
			}
		} else {
			// business policy case, has policy and userinput checks
			for sId_pc, rs_pc := range pcOutput.Reason {
				if rs_pc != msg_compatible {
					reason[sId_pc] = rs_pc
				} else if rs_privc, ok := privOutput.Reason[sId_pc]; ok && rs_privc != msg_compatible {
					reason[sId_pc] = rs_privc
				} else {
					// add user input compatibility result
					for sId_ui, rs_ui := range uiOutput.Reason {
						if sId_ui == sId_pc {
							reason[sId_ui] = rs_ui
						} else if strings.HasSuffix(sId_pc, "_*") || strings.HasSuffix(sId_pc, "_") || strings.HasSuffix(sId_ui, "_*") || strings.HasSuffix(sId_ui, "_") {
							// remove the arch parts and compare
							sId_pc_na := cutil.RemoveArchFromServiceId(sId_pc)
							sId_ui_na := cutil.RemoveArchFromServiceId(sId_ui)
							if sId_pc_na == sId_ui_na {
								reason[sId_ui] = rs_ui
							}
						}
					}
				}
			}
		}
	}
	ccOutput.Reason = reason

	// combine the input part
	ccInput := CompCheckResource{}
	ccInput.NodeId = uiOutput.Input.NodeId
	ccInput.NodeArch = uiOutput.Input.NodeArch
	ccInput.NodeType = uiOutput.Input.NodeType
	ccInput.NodeUserInput = uiOutput.Input.NodeUserInput
	ccInput.NodePolicy = pcOutput.Input.NodePolicy
	ccInput.BusinessPolId = uiOutput.Input.BusinessPolId
	ccInput.BusinessPolicy = uiOutput.Input.BusinessPolicy
	ccInput.PatternId = uiOutput.Input.PatternId
	ccInput.Pattern = uiOutput.Input.Pattern
	ccInput.Service = uiOutput.Input.Service
	ccInput.ServicePolicy = pcOutput.Input.ServicePolicy
	ccOutput.Input = &ccInput

	return &ccOutput
}

// convert CompCheck object to PolicyCheckobject.
func convertToPolicyCheck(in interface{}, msgPrinter *message.Printer) (*PolicyCheck, error) {
	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	var out PolicyCheck
	if buf, err := json.Marshal(in); err != nil {
		return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Error marshaling object %v. %v", in)), COMPCHECK_GENERAL_ERROR)
	} else if err := json.Unmarshal(buf, &out); err != nil {
		return nil, fmt.Errorf(msgPrinter.Sprintf("Failed to convert input to PolicyCheck object. %v", err))
	}
	return &out, nil
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
		return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Error getting node %v from the exchange. %v", nodeId, err)), COMPCHECK_EXCHANGE_ERROR)
	} else if node == nil {
		return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("No node found for this node id %v.", nodeId)), COMPCHECK_INPUT_ERROR)
	} else {
		return node, nil
	}
}

// EvaluatePatternPrivilegeCompatability determines if a given node requires a workload that uses privileged mode or network=host.
// This function will recursively evaluate top-level services specified in the pattern.
func EvaluatePatternPrivilegeCompatability(getServiceResolvedDef exchange.ServiceDefResolverHandler, getService exchange.ServiceHandler,
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
			sId := fmt.Sprintf("%s/%s", svcRef.ServiceOrg, svcRef.ServiceURL)
			allSvcs, err := GetAllServices(getServiceResolvedDef, getService, policy.Workload{WorkloadURL: svcRef.ServiceURL, Org: svcRef.ServiceOrg, Arch: svcRef.ServiceArch, Version: workload.Version}, msgPrinter)
			if err != nil && !quiet {
				return nil, err
			}
			for _, inputSvcDef := range cc.Service {
				exchSvcDef := exchange.ServiceDefinition{URL: inputSvcDef.GetURL()}
				if _, ok := inputSvcDef.Deployment.(string); ok {
					exchSvcDef.Deployment = inputSvcDef.Deployment.(string)
				} else {
					depByte, err := json.Marshal(inputSvcDef.Deployment)
					if err != nil {
						return nil, err
					}
					exchSvcDef.Deployment = string(depByte)
				}
				if allSvcs == nil {
					allSvcs = &map[string]exchange.ServiceDefinition{fmt.Sprintf("%s/%s", inputSvcDef.GetOrg(), inputSvcDef.GetURL()): exchSvcDef}
				} else {
					(*allSvcs)[fmt.Sprintf("%s/%s", inputSvcDef.GetOrg(), inputSvcDef.GetURL())] = exchSvcDef
				}
			}
			workLoadPriv, err, privWorkloads := servicesRequirePrivilege(allSvcs, msgPrinter)
			if err != nil {
				return nil, err
			}
			for _, sId := range privWorkloads {
				exchSDef := (*allSvcs)[sId]
				sDef := &ServiceDefinition{exchange.GetOrg(sId), exchSDef}
				svcIncomp = append(svcIncomp, sDef)
			}

			if workLoadPriv && checkAll {
				svcPriv = true
				messages[sId] = fmt.Sprintf("Version %s of this service requires the following workloads that will only run on a privileged node. %v", workload.Version, privWorkloads)
			} else if workLoadPriv {
				ccResource.Service = svcIncomp
				return NewCompCheckOutput(false, map[string]string{sId: fmt.Sprintf("Version %s of this service requires the following workloads that will only run on a privileged node. %v", workload.Version, privWorkloads)}, nil), nil
			} else {
				sDef := &ServiceDefinition{svcRef.ServiceOrg, (*allSvcs)[sId]}
				svcComp = append(svcComp, sDef)
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

// GetAllServices returns a map of serviceIds to service definitions of the given service and any service it is dependent on evaluated recursively
func GetAllServices(getServiceResolvedDef exchange.ServiceDefResolverHandler, getService exchange.ServiceHandler,
	workload policy.Workload, msgPrinter *message.Printer) (*map[string]exchange.ServiceDefinition, error) {
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}
	sDefMap, topSvcDef, topSvcId, err := getServiceResolvedDef(workload.WorkloadURL, workload.Org, workload.Version, workload.Arch)
	if err != nil {
		// logging this message here so it gets translated. Will add quiet parameter to determine if this is an error or a cli warning later.
		return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Failed to find definition for dependent services of %s. Compatability of %s cannot be fully evaluated until all services are in the exchange.", topSvcId, externalpolicy.PROP_NODE_PRIVILEGED)), COMPCHECK_EXCHANGE_ERROR)
	}
	if topSvcDef != nil {
		sDefMap[topSvcId] = *topSvcDef
	}
	return &sDefMap, nil
}

// this function takes as a parameter a map of service ids to deployment strings
// Returns true if any of the service listed require privilge to run
// returns a slice of service ids of any services that require privilege to run
func servicesRequirePrivilege(serviceDefs *map[string]exchange.ServiceDefinition, msgPrinter *message.Printer) (bool, error, []string) {
	privSvcs := []string{}
	reqPriv := false

	if serviceDefs == nil {
		return false, nil, nil
	}

	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	for sId, sDef := range *serviceDefs {
		if priv, err := deploymentRequiresPrivilege(sDef.GetDeploymentString(), msgPrinter); err != nil {
			return false, err, nil
		} else if priv {
			reqPriv = true
			privSvcs = append(privSvcs, sId)
		}
	}
	return reqPriv, nil, privSvcs
}

// Check if the deployment string given uses the privileged flag or network=host
func deploymentRequiresPrivilege(deploymentString string, msgPrinter *message.Printer) (bool, error) {
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
