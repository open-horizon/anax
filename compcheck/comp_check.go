package compcheck

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/businesspolicy"
	"github.com/open-horizon/anax/common"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/i18n"
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
	COMPCHECK_CONVERTION_ERROR = 12
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
	return fmt.Sprintf("NodeId: %v, NodeArch: %v, NodePolicy: %v, NodeUserInput: %v, BusinessPolId: %v, BusinessPolicy: %v, PatternId: %v, Pattern: %v, ServicePolicy: %v, Service: %v",
		p.NodeId, p.NodeArch, p.NodePolicy, p.NodeUserInput, p.BusinessPolId, p.BusinessPolicy, p.PatternId, p.Pattern, p.ServicePolicy, p.Service)

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
	return fmt.Sprintf("NodeId: %v, NodeArch: %v, NodePolicy: %v, NodeUserInput: %v, BusinessPolId: %v, BusinessPolicy: %v, PatternId: %v, Pattern: %v, ServicePolicy: %v, Service: %v",
		p.NodeId, p.NodeArch, p.NodePolicy, p.NodeUserInput, p.BusinessPolId, p.BusinessPolicy, p.PatternId, p.Pattern, p.ServicePolicy, p.Service)

}

func NewCompCheckResourceFromUICheck(uiInput *UserInputCheck) *CompCheckResource {
	var rsrc CompCheckResource
	rsrc.NodeId = uiInput.NodeId
	rsrc.NodeArch = uiInput.NodeArch
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
	rsrc.NodePolicy = uiInput.NodePolicy
	rsrc.BusinessPolId = uiInput.BusinessPolId
	rsrc.BusinessPolicy = uiInput.BusinessPolicy

	// If user input a service policy, it will be applied to all the services defined in the bp or pattern
	rsrc.ServicePolicy = map[string]externalpolicy.ExternalPolicy{}
	if uiInput.ServicePolicy != nil {
		rsrc.ServicePolicy["AllServices"] = *uiInput.ServicePolicy
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
			return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Neither bussiness policy nor pattern is specified.")), COMPCHECK_INPUT_ERROR)
		}
	}

	// check policy first, for business policy case only
	policyCheckInput, err := convertToPolicyCheck(ccInput, msgPrinter)
	if err != nil {
		return nil, err
	}

	pcOutput := NewCompCheckOutput(true, map[string]string{}, NewCompCheckResourceFromPolicyCheck(policyCheckInput))
	if useBPol {
		var err1 error
		pcOutput, err1 = policyCompatible(getDeviceHandler, nodePolicyHandler, getBusinessPolicies, servicePolicyHandler, getSelectedServices, policyCheckInput, true, msgPrinter)
		if err1 != nil {
			return nil, err1
		}

		// if not compatible, then do not bother to do user input check
		if !pcOutput.Compatible {
			return pcOutput, nil
		}
	}

	// check user input for those services that are compatible
	uiCheckInput := createUserInputCheckInput(ccInput, pcOutput, msgPrinter)
	uiOutput, err := userInputCompatible(getDeviceHandler, getBusinessPolicies, getPatterns, getServiceHandler, serviceDefResolverHandler, getSelectedServices, uiCheckInput, checkAllSvcs, msgPrinter)
	if err != nil {
		return nil, err
	}

	// combine the output of policy check and user input check into one
	return createCompCheckOutput(pcOutput, uiOutput, checkAllSvcs, msgPrinter), nil
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
func createCompCheckOutput(pcOutput *CompCheckOutput, uiOutput *CompCheckOutput, checkAllSvcs bool, msgPrinter *message.Printer) *CompCheckOutput {
	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	var ccOutput CompCheckOutput

	// the last one take precedence for overall assesement
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
				} else {
					// add user input compatibility result
					for sId_ui, rs_ui := range uiOutput.Reason {
						if sId_ui == sId_pc {
							reason[sId_ui] = rs_ui
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
