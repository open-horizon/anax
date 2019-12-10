package compcheck

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/businesspolicy"
	"github.com/open-horizon/anax/config"
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
	ServicePolicy  *externalpolicy.ExternalPolicy `json:"service_policy,omitempty"`
	Service        []exchange.ServiceDefinition   `json:"service,omitempty"`
}

func (p CompCheck) String() string {
	return fmt.Sprintf("NodeId: %v, NodeArch: %v, NodePolicy: %v, NodeUserInput: %v, BusinessPolId: %v, BusinessPolicy: %v, ServicePolicy: %v, Service: %v",
		p.NodeId, p.NodeArch, p.NodePolicy, p.NodeUserInput, p.BusinessPolId, p.BusinessPolicy, p.ServicePolicy, p.Service)

}

// The output format for the compatibility check
type CompCheckOutput struct {
	Compatible bool              `json:"compatible"`
	Reason     map[string]string `json:"reason"` // set when not compatible
	Input      *CompCheck        `json:"input,omitempty"`
}

func (p *CompCheckOutput) String() string {
	return fmt.Sprintf("Compatible: %v, Reason: %v, Input: %v",
		p.Compatible, p.Reason, p.Input)

}

func NewCompCheckOutput(compatible bool, reason map[string]string, input *CompCheck) *CompCheckOutput {
	return &CompCheckOutput{
		Compatible: compatible,
		Reason:     reason,
		Input:      input,
	}
}

// exchange context using user credential
type UserExchangeContext struct {
	UserId      string
	Password    string
	URL         string
	CSSURL      string
	HTTPFactory *config.HTTPClientFactory
}

func (u *UserExchangeContext) GetExchangeId() string {
	return u.UserId
}

func (u *UserExchangeContext) GetExchangeToken() string {
	return u.Password
}

func (u *UserExchangeContext) GetExchangeURL() string {
	return u.URL
}

func (u *UserExchangeContext) GetCSSURL() string {
	return u.CSSURL
}

func (u *UserExchangeContext) GetHTTPFactory() *config.HTTPClientFactory {
	return u.HTTPFactory
}

func DeployCompatible(ec exchange.ExchangeContext, ccInput *CompCheck, checkAllSvcs bool, msgPrinter *message.Printer) (*CompCheckOutput, error) {

	getDeviceHandler := exchange.GetHTTPDeviceHandler(ec)
	nodePolicyHandler := exchange.GetHTTPNodePolicyHandler(ec)
	getBusinessPolicies := exchange.GetHTTPBusinessPoliciesHandler(ec)
	servicePolicyHandler := exchange.GetHTTPServicePolicyHandler(ec)
	getServiceHandler := exchange.GetHTTPServiceHandler(ec)
	serviceDefResolverHandler := exchange.GetHTTPServiceDefResolverHandler(ec)
	getSelectedServices := exchange.GetHTTPSelectedServicesHandler(ec)

	return deployCompatible(getDeviceHandler, nodePolicyHandler, getBusinessPolicies, servicePolicyHandler, getServiceHandler, serviceDefResolverHandler, getSelectedServices, ccInput, checkAllSvcs, msgPrinter)
}

// Internal function for PolicyCompatible
func deployCompatible(getDeviceHandler exchange.DeviceHandler,
	nodePolicyHandler exchange.NodePolicyHandler,
	getBusinessPolicies exchange.BusinessPoliciesHandler,
	servicePolicyHandler exchange.ServicePolicyHandler,
	getServiceHandler exchange.ServiceHandler,
	serviceDefResolverHandler exchange.ServiceDefResolverHandler,
	getSelectedServices exchange.SelectedServicesHandler,
	ccInput *CompCheck, checkAllSvcs bool, msgPrinter *message.Printer) (*CompCheckOutput, error) {

	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	// check policy first
	policyCheckInput, err := convertToPolicyCheck(ccInput, msgPrinter)
	if err != nil {
		return nil, err
	}
	pcOutput, err := policyCompatible(getDeviceHandler, nodePolicyHandler, getBusinessPolicies, servicePolicyHandler, getSelectedServices, policyCheckInput, true, msgPrinter)
	if err != nil {
		return nil, err
	}

	// if not compatible, then do not bother to do user input check
	if !pcOutput.Compatible {
		return convertToCompCheckOutput(pcOutput, msgPrinter)
	}

	// check user input for those services that are compatible
	uiCheckInput := createUserInputCheckInput(ccInput, pcOutput, msgPrinter)
	uiOutput, err := userInputCompatible(getDeviceHandler, getBusinessPolicies, getServiceHandler, serviceDefResolverHandler, getSelectedServices, uiCheckInput, checkAllSvcs, msgPrinter)
	if err != nil {
		return nil, err
	}

	// combine the output of policy check and user input check into one
	return createCompCheckOutput(pcOutput, uiOutput, checkAllSvcs, msgPrinter), nil
}

// Use the result of policy check together with the original input to create a user input check input object.
func createUserInputCheckInput(ccInput *CompCheck, pcOutput *PolicyCheckOutput, msgPrinter *message.Printer) *UserInputCheck {
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

	// limit the check to the services that are compatible for policy check
	svcs := []string{}
	for sId, reason := range pcOutput.Reason {
		if !strings.Contains(reason, msgPrinter.Sprintf("Incompatible")) {
			svcs = append(svcs, sId)
		}
	}
	uiCheckInput.ServiceToCheck = svcs

	return &uiCheckInput
}

// combine the PolicyCheckOutput and UserInputCheckOutput into CompCheckOutput
func createCompCheckOutput(pcOutput *PolicyCheckOutput, uiOutput *UserInputCheckOutput, checkAllSvcs bool, msgPrinter *message.Printer) *CompCheckOutput {
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
	ccOutput.Reason = reason

	// combine the input part
	ccInput := CompCheck{}
	ccInput.NodeId = uiOutput.Input.NodeId
	ccInput.NodeArch = uiOutput.Input.NodeArch
	ccInput.NodeUserInput = uiOutput.Input.NodeUserInput
	ccInput.NodePolicy = pcOutput.Input.NodePolicy
	ccInput.BusinessPolId = uiOutput.Input.BusinessPolId
	ccInput.BusinessPolicy = uiOutput.Input.BusinessPolicy
	ccInput.Service = uiOutput.Input.Service
	ccInput.ServicePolicy = pcOutput.Input.ServicePolicy
	ccOutput.Input = &ccInput

	return &ccOutput
}

// convert PolicyCheckOutput or UserInputCheckOutput object to a CompCheckOutput object
func convertToCompCheckOutput(in interface{}, msgPrinter *message.Printer) (*CompCheckOutput, error) {
	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	var out CompCheckOutput
	if buf, err := json.Marshal(in); err != nil {
		return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Error marshaling object %v. %v", in)), COMPCHECK_GENERAL_ERROR)
	} else if err := json.Unmarshal(buf, &out); err != nil {
		return nil, fmt.Errorf(msgPrinter.Sprintf("Failed to convert input to CompCheckOutput object. %v", err))
	}
	return &out, nil
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
