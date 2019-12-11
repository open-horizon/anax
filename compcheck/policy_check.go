package compcheck

import (
	"fmt"
	"github.com/open-horizon/anax/businesspolicy"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/policy"
	"golang.org/x/text/message"
)

// The input format for the policy check
type PolicyCheck struct {
	NodeId         string                         `json:"node_id,omitempty"`
	NodeArch       string                         `json:"node_arch,omitempty"`
	NodePolicy     *externalpolicy.ExternalPolicy `json:"node_policy,omitempty"`
	BusinessPolId  string                         `json:"business_policy_id,omitempty"`
	BusinessPolicy *businesspolicy.BusinessPolicy `json:"business_policy,omitempty"`
	ServicePolicy  *externalpolicy.ExternalPolicy `json:"service_policy,omitempty"`
}

func (p PolicyCheck) String() string {
	return fmt.Sprintf("NodeId: %v, NodeArch: %v, NodePolicy: %v, BusinessPolId: %v, BusinessPolicy: %v, ServicePolicy: %v,",
		p.NodeId, p.NodeArch, p.NodePolicy, p.BusinessPolId, p.BusinessPolicy, p.ServicePolicy)

}

// The output format for the policy check
type PolicyCheckOutput struct {
	Compatible bool              `json:"compatible"`
	Reason     map[string]string `json:"reason"` // set when not compatible
	Input      *PolicyCheck      `json:"input,omitempty"`
}

func (p *PolicyCheckOutput) String() string {
	return fmt.Sprintf("Compatible: %v, Reason: %v, Input: %v",
		p.Compatible, p.Reason, p.Input)

}

func NewPolicyCheckOutput(compatible bool, reason map[string]string, input *PolicyCheck) *PolicyCheckOutput {
	return &PolicyCheckOutput{
		Compatible: compatible,
		Reason:     reason,
		Input:      input,
	}
}

// This is the function that HZN and the agbot secure API calls.
// Given the PolicyCheck input, check if the policies are compatible.
// The required fields in PolicyCheck are:
//  (NodeId or NodePolicy) and (BusinessPolId or BusinessPolicy)
//
// When checking whether the policies are compatible or not, we devide policies into two side:
//    Edge side: node policy (including the node built-in policy)
//    Agbot side: business policy + service policy + service built-in properties
func PolicyCompatible(ec exchange.ExchangeContext, pcInput *PolicyCheck, checkAllSvcs bool, msgPrinter *message.Printer) (*PolicyCheckOutput, error) {

	getDeviceHandler := exchange.GetHTTPDeviceHandler(ec)
	nodePolicyHandler := exchange.GetHTTPNodePolicyHandler(ec)
	getBusinessPolicies := exchange.GetHTTPBusinessPoliciesHandler(ec)
	servicePolicyHandler := exchange.GetHTTPServicePolicyHandler(ec)
	getSelectedServices := exchange.GetHTTPSelectedServicesHandler(ec)

	return policyCompatible(getDeviceHandler, nodePolicyHandler, getBusinessPolicies, servicePolicyHandler, getSelectedServices, pcInput, checkAllSvcs, msgPrinter)
}

// Internal function for PolicyCompatible
func policyCompatible(getDeviceHandler exchange.DeviceHandler,
	nodePolicyHandler exchange.NodePolicyHandler,
	getBusinessPolicies exchange.BusinessPoliciesHandler,
	servicePolicyHandler exchange.ServicePolicyHandler,
	getSelectedServices exchange.SelectedServicesHandler,
	pcInput *PolicyCheck, checkAllSvcs bool, msgPrinter *message.Printer) (*PolicyCheckOutput, error) {

	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	if pcInput == nil {
		return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("The PolicyCheck input cannot be null")), COMPCHECK_INPUT_ERROR)
	}

	// make a copy of the input because the process will change it. The pointer to policies will stay the same.
	input_temp := PolicyCheck(*pcInput)
	input := &input_temp

	nodeId := input.NodeId
	if nodeId != "" {
		if node, err := GetExchangeNode(getDeviceHandler, nodeId, msgPrinter); err != nil {
			return nil, err
		} else if input.NodeArch != "" {
			if node.Arch != "" && node.Arch != input.NodeArch {
				return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("The input node architecture %v does not match the exchange node architecture %v for node %v.", input.NodeArch, node.Arch, nodeId)), COMPCHECK_INPUT_ERROR)
			}
		} else {
			input.NodeArch = node.Arch
		}
	}

	// validate node policy and convert it to internal policy
	var nPolicy *policy.Policy
	var err1 error
	input.NodePolicy, nPolicy, err1 = processNodePolicy(nodePolicyHandler, nodeId, input.NodePolicy, msgPrinter)
	if err1 != nil {
		return nil, err1
	}

	// validate and convert the business policy to internal policy
	var bPolicy *policy.Policy
	bpId := input.BusinessPolId
	input.BusinessPolicy, bPolicy, err1 = processBusinessPolicy(getBusinessPolicies, bpId, input.BusinessPolicy, true, msgPrinter)
	if err1 != nil {
		return nil, err1
	}

	msg_incompatible := msgPrinter.Sprintf("Policy Incompatible")
	msg_compatible := msgPrinter.Sprintf("Compatible")

	// go through all the workloads and check if compatible or not
	messages := map[string]string{}
	overall_compatible := false
	var servicePol_comp, servicePol_incomp *externalpolicy.ExternalPolicy
	for _, workload := range bPolicy.Workloads {

		// make sure arch is correct
		if workload.Arch == "*" {
			workload.Arch = ""
		}
		if input.NodeArch != "" {
			if workload.Arch == "" {
				workload.Arch = input.NodeArch
			} else if input.NodeArch != workload.Arch {
				if checkAllSvcs {
					sId := cutil.FormExchangeIdForService(workload.WorkloadURL, workload.Version, workload.Arch)
					sId = fmt.Sprintf("%v/%v", workload.Org, sId)
					messages[sId] = fmt.Sprintf("%v: %v", msg_incompatible, msgPrinter.Sprintf("Architecture does not match."))
				}
				continue
			}
		}

		// get service policy + default properties
		if input.ServicePolicy == nil {
			if workload.Arch != "" {
				// get service policy with built-in properties
				if mergedServicePol, sPol, sId, err := GetServicePolicyWithDefaultProperties(servicePolicyHandler, workload.WorkloadURL, workload.Org, workload.Version, workload.Arch, msgPrinter); err != nil {
					return nil, err
					// compatibility check
				} else {
					if compatible, reason, _, _, err := CheckPolicyCompatiblility(nPolicy, bPolicy, mergedServicePol, input.NodeArch, msgPrinter); err != nil {
						return nil, err
					} else if compatible {
						overall_compatible = true
						if checkAllSvcs {
							servicePol_comp = sPol
							messages[sId] = msg_compatible
						} else {
							input.ServicePolicy = sPol
							return NewPolicyCheckOutput(true, map[string]string{sId: msg_compatible}, input), nil
						}
					} else {
						servicePol_incomp = sPol
						messages[sId] = fmt.Sprintf("%v: %v", msg_incompatible, reason)
					}
				}
			} else {
				if svcMeta, err := getSelectedServices(workload.WorkloadURL, workload.Org, workload.Version, workload.Arch); err != nil {
					return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Failed to get services for all archetctures for %v/%v version %v. %v", workload.Org, workload.WorkloadURL, workload.Version, err)), COMPCHECK_EXCHANGE_ERROR)
				} else {
					// since workload arch is empty, need to go through all the arches
					for sId, svc := range svcMeta {
						// get service policy with built-in properties
						if mergedServicePol, sPol, _, err := GetServicePolicyWithDefaultProperties(servicePolicyHandler, workload.WorkloadURL, workload.Org, workload.Version, svc.Arch, msgPrinter); err != nil {
							return nil, err
							// compatibility check
						} else {
							if compatible, reason, _, _, err := CheckPolicyCompatiblility(nPolicy, bPolicy, mergedServicePol, input.NodeArch, msgPrinter); err != nil {
								return nil, err
							} else if compatible {
								overall_compatible = true
								if checkAllSvcs {
									servicePol_comp = sPol
									messages[sId] = msg_compatible
								} else {
									input.ServicePolicy = sPol
									return NewPolicyCheckOutput(true, map[string]string{sId: msg_compatible}, input), nil
								}
							} else {
								servicePol_incomp = sPol
								messages[sId] = fmt.Sprintf("%v: %v", msg_incompatible, reason)
							}
						}
					}
				}
			}
		} else {
			// validate the service policy
			if err := input.ServicePolicy.Validate(); err != nil {
				return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Failed to validate the service policy. %v", err)), COMPCHECK_VALIDATION_ERROR)
			}

			// get default service properties
			builtInSvcPol := externalpolicy.CreateServiceBuiltInPolicy(workload.WorkloadURL, workload.Org, workload.Version, workload.Arch)
			// add built-in service properties to the service policy
			mergedServicePol := AddDefaultPropertiesToServicePolicy(input.ServicePolicy, builtInSvcPol)
			// compatibility check
			sId := cutil.FormExchangeIdForService(workload.WorkloadURL, workload.Version, workload.Arch)
			sId = fmt.Sprintf("%v/%v", workload.Org, sId)
			if compatible, reason, _, _, err := CheckPolicyCompatiblility(nPolicy, bPolicy, mergedServicePol, input.NodeArch, msgPrinter); err != nil {
				return nil, err
			} else if compatible {
				overall_compatible = true
				if checkAllSvcs {
					messages[sId] = msg_compatible
				} else {
					return NewPolicyCheckOutput(true, map[string]string{sId: msg_compatible}, input), nil
				}
			} else {
				messages[sId] = fmt.Sprintf("%v: %v", msg_incompatible, reason)
			}
		}
	}

	// If we get here, it means that no workload is found in the bp that matches the required node arch.
	if messages != nil && len(messages) != 0 {
		if overall_compatible {
			input.ServicePolicy = servicePol_comp
		} else {
			input.ServicePolicy = servicePol_incomp
		}
		return NewPolicyCheckOutput(overall_compatible, messages, input), nil
	} else {
		if input.NodeArch != "" {
			messages["general"] = fmt.Sprintf("%v: %v", msg_incompatible, msgPrinter.Sprintf("Service with 'arch' %v cannot be found in the business policy.", input.NodeArch))
		} else {
			messages["general"] = fmt.Sprintf("%v: %v", msg_incompatible, msgPrinter.Sprintf("No services found in the business policy."))
		}

		return NewPolicyCheckOutput(false, messages, input), nil
	}
}

// It does the policy compatibility check. node arch can be empty. It is called by agbot and PolicyCompatible function.
// The node arch is supposed to be already compared against the service arch before calling this function.
func CheckPolicyCompatiblility(nodePolicy *policy.Policy, businessPolicy *policy.Policy, mergedServicePolicy *externalpolicy.ExternalPolicy, nodeArch string, msgPrinter *message.Printer) (bool, string, *policy.Policy, *policy.Policy, error) {

	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	// check node policy
	if nodePolicy == nil {
		return false, "", nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Node policy cannot be null.")), COMPCHECK_INPUT_ERROR)
	}

	// check business policy
	if businessPolicy == nil {
		return false, "", nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Business policy cannot be null.")), COMPCHECK_INPUT_ERROR)
	}

	// check merged service policy
	if mergedServicePolicy == nil {
		return false, "", nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Merged service policy cannot be null.")), COMPCHECK_INPUT_ERROR)
	}

	// merge the service policy, the default service properties
	mergedConsumerPol, err := MergeFullServicePolicyToBusinessPolicy(businessPolicy, mergedServicePolicy, msgPrinter)
	if err != nil {
		return false, "", nil, nil, err
	}

	// Add node arch properties to the node policy
	mergedProducerPol, err := addNodeArchToPolicy(nodePolicy, nodeArch, msgPrinter)
	if err != nil {
		return false, "", nil, nil, err
	}

	// check if the node policy and merged bp policy are compatible
	if err := policy.Are_Compatible(nodePolicy, mergedConsumerPol, msgPrinter); err != nil {
		return false, err.Error(), mergedProducerPol, mergedConsumerPol, nil
	} else {
		// policy match
		return true, "", mergedProducerPol, mergedConsumerPol, nil
	}
}

// add node arch property to the node policy. node arch can be empty
func addNodeArchToPolicy(nodePolicy *policy.Policy, nodeArch string, msgPrinter *message.Printer) (*policy.Policy, error) {
	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	if nodePolicy == nil {
		return nil, nil
	}

	if nodeArch == "" {
		return nodePolicy, nil
	}

	nodeBuiltInProps := new(externalpolicy.PropertyList)
	nodeBuiltInProps.Add_Property(externalpolicy.Property_Factory(externalpolicy.PROP_NODE_ARCH, nodeArch), false)

	buitInPol := externalpolicy.ExternalPolicy{
		Properties:  *nodeBuiltInProps,
		Constraints: []string{},
	}

	if pPolicy, err := policy.MergePolicyWithExternalPolicy(nodePolicy, &buitInPol); err != nil {
		return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Error merging node policy with arch property. %v", err)), COMPCHECK_MERGING_ERROR)
	} else {
		return pPolicy, nil
	}

}

// If inputNP is given, validate it and generate internal policy from it.
// If not, user node id to get the node policy from the exchange and generate internal policy from it.
func processNodePolicy(nodePolicyHandler exchange.NodePolicyHandler,
	nodeId string, inputNP *externalpolicy.ExternalPolicy,
	msgPrinter *message.Printer) (*externalpolicy.ExternalPolicy, *policy.Policy, error) {

	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	if inputNP != nil {
		if err := inputNP.Validate(); err != nil {
			return nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Failed to validate the node policy. %v", err)), COMPCHECK_VALIDATION_ERROR)
		} else {
			// give nodeId a default if it is empty
			if nodeId == "" {
				nodeId = "TempNodePolicyId"
			}

			if nPolicy, err := policy.GenPolicyFromExternalPolicy(inputNP, policy.MakeExternalPolicyHeaderName(nodeId)); err != nil {
				return nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Failed to convert node policy to internal policy format for node %v: %v", nodeId, err)), COMPCHECK_CONVERTION_ERROR)
			} else {
				return inputNP, nPolicy, nil
			}
		}
	} else if nodeId != "" {
		if extNPolicy, nPolicy, err := GetNodePolicy(nodePolicyHandler, nodeId, msgPrinter); err != nil {
			return nil, nil, err
		} else if nPolicy == nil {
			return nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("No node policy found for this node %v.", nodeId)), COMPCHECK_INPUT_ERROR)
		} else {
			return extNPolicy, nPolicy, nil
		}
	} else {
		return nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Neither node policy nor node id is specified.")), COMPCHECK_INPUT_ERROR)
	}
}

// Get node policy from the exchange and convert it to internal policy.
// It returns (nil, nil) If there is no node policy found.
func GetNodePolicy(nodePolicyHandler exchange.NodePolicyHandler, nodeId string, msgPrinter *message.Printer) (*externalpolicy.ExternalPolicy, *policy.Policy, error) {
	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	// check input
	if nodeId == "" {
		return nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Node id is empty.")), COMPCHECK_INPUT_ERROR)
	} else {
		nodeOrg := exchange.GetOrg(nodeId)
		if nodeOrg == "" {
			return nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Organization is not specified in the node id: %v.", nodeId)), COMPCHECK_INPUT_ERROR)
		}
	}

	// get node policy
	nodePolicy, err := nodePolicyHandler(nodeId)
	if err != nil {
		return nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Error trying to query node policy for %v: %v", nodeId, err)), COMPCHECK_EXCHANGE_ERROR)
	}

	if nodePolicy == nil {
		return nil, nil, nil
	}

	// validate the policy
	extPolicy := nodePolicy.GetExternalPolicy()
	if err := extPolicy.Validate(); err != nil {
		return nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Failed to validate the node policy for node %v. %v", nodeId, err)), COMPCHECK_VALIDATION_ERROR)
	}

	// convert the policy to internal policy format
	pPolicy, err := policy.GenPolicyFromExternalPolicy(&extPolicy, policy.MakeExternalPolicyHeaderName(nodeId))
	if err != nil {
		return nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Failed to convert node policy to internal policy for node %v: %v", nodeId, err)), COMPCHECK_CONVERTION_ERROR)
	}
	return &extPolicy, pPolicy, nil
}

// If the inputBP is given, then validate it and convert it to internal policy.
// If not, get business policy from the exchange the convert it to internal policy
func processBusinessPolicy(getBusinessPolicies exchange.BusinessPoliciesHandler, bpId string, inputBP *businesspolicy.BusinessPolicy, convert bool, msgPrinter *message.Printer) (*businesspolicy.BusinessPolicy, *policy.Policy, error) {
	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	var pPolicy *policy.Policy
	var bPolicy *businesspolicy.BusinessPolicy
	if inputBP != nil {
		// give bpId a default value if it is empty
		if bpId == "" {
			bpId = "TempBusinessPolicyId"
		}
		if convert {
			//convert busineess policy to internal policy, the validate is done in this function
			var err1 error
			pPolicy, err1 = inputBP.GenPolicyFromBusinessPolicy(bpId)
			if err1 != nil {
				return nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Failed to convert business policy %v to internal policy: %v", bpId, err1)), COMPCHECK_CONVERTION_ERROR)
			}
			return inputBP, pPolicy, nil
		} else {
			// validate the input policy
			if err := inputBP.Validate(); err != nil {
				return nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Validation failure for business policy %v. %v", bpId, err)), COMPCHECK_VALIDATION_ERROR)
			}
			if inputBP.Service.ServiceVersions == nil || len(inputBP.Service.ServiceVersions) == 0 {
				return nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("No services specified in the given business policy %v.", bpId)), COMPCHECK_VALIDATION_ERROR)
			}
			return inputBP, nil, nil
		}
	} else if bpId != "" {
		var err1 error
		bPolicy, pPolicy, err1 = GetBusinessPolicy(getBusinessPolicies, bpId, convert, msgPrinter)
		if err1 != nil {
			return nil, nil, err1
		}
		if bPolicy.Service.ServiceVersions == nil || len(bPolicy.Service.ServiceVersions) == 0 {
			return nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("No services specified in the business policy %v.", pPolicy.Header.Name)), COMPCHECK_VALIDATION_ERROR)
		}
		return bPolicy, pPolicy, nil
	} else {
		return nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Neither business policy nor business policy id is specified.")), COMPCHECK_INPUT_ERROR)
	}
}

// get business policy from the exchange.
func GetBusinessPolicy(getBusinessPolicies exchange.BusinessPoliciesHandler, bpId string, convert bool, msgPrinter *message.Printer) (*businesspolicy.BusinessPolicy, *policy.Policy, error) {
	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	// check input
	if bpId == "" {
		return nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Business policy id is empty.")), COMPCHECK_INPUT_ERROR)
	} else {
		bpOrg := exchange.GetOrg(bpId)
		if bpOrg == "" {
			return nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Organization is not specified in the business policy id: %v.", bpId)), COMPCHECK_INPUT_ERROR)
		}
	}

	// get poicy from the exchange
	exchPols, err := getBusinessPolicies(exchange.GetOrg(bpId), exchange.GetId(bpId))
	if err != nil {
		return nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Unable to get business policy for %v, %v", bpId, err)), COMPCHECK_EXCHANGE_ERROR)
	}
	if exchPols == nil || len(exchPols) == 0 {
		return nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("No business policy found for this id %v.", bpId)), COMPCHECK_INPUT_ERROR)
	}

	// convert the business policy to internal policy format
	for polId, exchPol := range exchPols {
		bPol := exchPol.GetBusinessPolicy()

		if convert {
			pPolicy, err := bPol.GenPolicyFromBusinessPolicy(polId)
			if err != nil {
				return nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Failed to convert business policy %v to internal policy format: %v", bpId, err)), COMPCHECK_CONVERTION_ERROR)
			}
			return &bPol, pPolicy, nil
		} else {
			return &bPol, nil, nil
		}
	}

	return nil, nil, nil
}

// Get service policy from the exchange
func GetServicePolicyWithId(serviceIdPolicyHandler exchange.ServicePolicyWithIdHandler, svcId string, msgPrinter *message.Printer) (*externalpolicy.ExternalPolicy, error) {
	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	// check input
	if svcId == "" {
		return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Service policy id is empty.")), COMPCHECK_INPUT_ERROR)
	} else {
		svcOrg := exchange.GetOrg(svcId)
		if svcOrg == "" {
			return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Organization is not specified in the service policy id: %v.", svcId)), COMPCHECK_INPUT_ERROR)
		}
	}

	// get service policy from the exchange
	servicePolicy, err := serviceIdPolicyHandler(svcId)
	if err != nil {
		return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Error trying to query service policy for service id %v: %v", svcId, err)), COMPCHECK_EXCHANGE_ERROR)
	} else if servicePolicy == nil {
		return nil, nil
	} else {
		// validate the policy
		extPolicy := servicePolicy.GetExternalPolicy()
		if err := extPolicy.Validate(); err != nil {
			return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Error validating the service policy %v. %v", svcId, err)), COMPCHECK_VALIDATION_ERROR)
		}
		return &extPolicy, nil
	}
}

// Get service policy from the exchange,
func GetServicePolicy(servicePolicyHandler exchange.ServicePolicyHandler, svcUrl string, svcOrg string, svcVersion string, svcArch string, msgPrinter *message.Printer) (*externalpolicy.ExternalPolicy, string, error) {
	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	// check input
	if svcUrl == "" {
		return nil, "", NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Service name is empty.")), COMPCHECK_INPUT_ERROR)
	} else if svcOrg == "" {
		return nil, "", NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Service organization is empty.")), COMPCHECK_INPUT_ERROR)
	}

	// get service policy fromt the exchange
	servicePolicy, sId, err := servicePolicyHandler(svcUrl, svcOrg, svcVersion, svcArch)
	if err != nil {
		return nil, "", NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Error trying to query service policy for %v/%v %v %v. %v", svcOrg, svcUrl, svcVersion, svcArch, err)), COMPCHECK_EXCHANGE_ERROR)
	} else if servicePolicy == nil {
		return nil, sId, nil
	} else {
		extPolicy := servicePolicy.GetExternalPolicy()
		if err := extPolicy.Validate(); err != nil {
			return nil, sId, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Failed to validate the service policy for %v/%v %v %v. %v", svcOrg, svcUrl, svcVersion, svcArch, err)), COMPCHECK_VALIDATION_ERROR)
		}
		return &extPolicy, sId, nil
	}
}

// Get service policy from the exchange and then add the service defalt properties
func GetServicePolicyWithDefaultProperties(servicePolicyHandler exchange.ServicePolicyHandler, svcUrl string, svcOrg string, svcVersion string, svcArch string, msgPrinter *message.Printer) (*externalpolicy.ExternalPolicy, *externalpolicy.ExternalPolicy, string, error) {
	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	servicePol, sId, err := GetServicePolicy(servicePolicyHandler, svcUrl, svcOrg, svcVersion, svcArch, msgPrinter)
	if err != nil {
		return nil, nil, "", err
	}

	// get default service properties
	builtInSvcPol := externalpolicy.CreateServiceBuiltInPolicy(svcUrl, svcOrg, svcVersion, svcArch)

	// add built-in service properties to the service policy
	merged_pol := AddDefaultPropertiesToServicePolicy(servicePol, builtInSvcPol)

	return merged_pol, servicePol, sId, nil
}

// Add service default properties to the given service policy
func AddDefaultPropertiesToServicePolicy(servicePol, defaultSvcProps *externalpolicy.ExternalPolicy) *externalpolicy.ExternalPolicy {
	var merged_pol externalpolicy.ExternalPolicy
	if servicePol != nil {
		merged_pol = externalpolicy.ExternalPolicy(*servicePol)
		if defaultSvcProps != nil {
			(&merged_pol).MergeWith(defaultSvcProps, false)
		}
	} else {
		if defaultSvcProps != nil {
			merged_pol = externalpolicy.ExternalPolicy(*defaultSvcProps)
		}
	}

	return &merged_pol
}

// Merge a service policy into a business policy. The service policy
func MergeFullServicePolicyToBusinessPolicy(businessPolicy *policy.Policy, servicePolicy *externalpolicy.ExternalPolicy, msgPrinter *message.Printer) (*policy.Policy, error) {
	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	if businessPolicy == nil {
		return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("The given business policy should not be null.")), COMPCHECK_INPUT_ERROR)
	}
	if servicePolicy == nil {
		return businessPolicy, nil
	}

	if pPolicy, err := policy.MergePolicyWithExternalPolicy(businessPolicy, servicePolicy); err != nil {
		return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Error merging business policy with service policy. %v", err)), COMPCHECK_MERGING_ERROR)
	} else {
		return pPolicy, nil
	}
}

// This function merges the given business policy with the given built-in properties of the service and the given service policy
// from the top level service, if any.
func MergeServicePolicyToBusinessPolicy(businessPol *policy.Policy, builtInSvcPol *externalpolicy.ExternalPolicy, servicePol *externalpolicy.ExternalPolicy, msgPrinter *message.Printer) (*policy.Policy, error) {
	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	if businessPol == nil {
		return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("The business policy should not be null.")), COMPCHECK_INPUT_ERROR)
	}

	// add built-in service properties to the service policy
	merged_pol1 := AddDefaultPropertiesToServicePolicy(servicePol, builtInSvcPol)

	//merge service policy
	if merged_pol2, err := MergeFullServicePolicyToBusinessPolicy(businessPol, merged_pol1, msgPrinter); err != nil {
		return nil, err
	} else {
		return merged_pol2, nil
	}
}
