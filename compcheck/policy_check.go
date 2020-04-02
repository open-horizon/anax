package compcheck

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/businesspolicy"
	"github.com/open-horizon/anax/common"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/semanticversion"
	"golang.org/x/text/message"
)

// The input format for the policy check
type PolicyCheck struct {
	NodeId         string                         `json:"node_id,omitempty"`
	NodeArch       string                         `json:"node_arch,omitempty"`
	NodeType       string                         `json:"node_type,omitempty"` // can be omitted if node_id is specified
	NodePolicy     *externalpolicy.ExternalPolicy `json:"node_policy,omitempty"`
	BusinessPolId  string                         `json:"business_policy_id,omitempty"`
	BusinessPolicy *businesspolicy.BusinessPolicy `json:"business_policy,omitempty"`
	ServicePolicy  *externalpolicy.ExternalPolicy `json:"service_policy,omitempty"`
	Service        []common.ServiceFile           `json:"service,omitempty"` //only needed if the services are not in the exchange
}

func (p PolicyCheck) String() string {
	return fmt.Sprintf("NodeId: %v, NodeArch: %v, NodeType: %v, NodePolicy: %v, BusinessPolId: %v, BusinessPolicy: %v, ServicePolicy: %v, Service：%v",
		p.NodeId, p.NodeArch, p.NodeType, p.NodePolicy, p.BusinessPolId, p.BusinessPolicy, p.ServicePolicy, p.Service)
}

// This is the function that HZN and the agbot secure API calls.
// Given the PolicyCheck input, check if the policies are compatible.
// The required fields in PolicyCheck are:
//  (NodeId or NodePolicy) and (BusinessPolId or BusinessPolicy)
//
// When checking whether the policies are compatible or not, we devide policies into two side:
//    Edge side: node policy (including the node built-in policy)
//    Agbot side: business policy + service policy + service built-in properties
func PolicyCompatible(ec exchange.ExchangeContext, pcInput *PolicyCheck, checkAllSvcs bool, msgPrinter *message.Printer) (*CompCheckOutput, error) {

	getDeviceHandler := exchange.GetHTTPDeviceHandler(ec)
	nodePolicyHandler := exchange.GetHTTPNodePolicyHandler(ec)
	getBusinessPolicies := exchange.GetHTTPBusinessPoliciesHandler(ec)
	servicePolicyHandler := exchange.GetHTTPServicePolicyHandler(ec)
	getSelectedServices := exchange.GetHTTPSelectedServicesHandler(ec)
	getService := exchange.GetHTTPServiceHandler(ec)
	getServiceResolvedDef := exchange.GetHTTPServiceDefResolverHandler(ec)

	return policyCompatible(getDeviceHandler, nodePolicyHandler, getBusinessPolicies, servicePolicyHandler, getSelectedServices, getService, getServiceResolvedDef, pcInput, checkAllSvcs, msgPrinter)
}

// Internal function for PolicyCompatible
func policyCompatible(getDeviceHandler exchange.DeviceHandler,
	nodePolicyHandler exchange.NodePolicyHandler,
	getBusinessPolicies exchange.BusinessPoliciesHandler,
	servicePolicyHandler exchange.ServicePolicyHandler,
	getSelectedServices exchange.SelectedServicesHandler,
	getService exchange.ServiceHandler,
	getServiceResolvedDef exchange.ServiceDefResolverHandler,
	pcInput *PolicyCheck, checkAllSvcs bool, msgPrinter *message.Printer) (*CompCheckOutput, error) {

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

	resources := NewCompCheckResourceFromPolicyCheck(pcInput)

	nodeId := input.NodeId
	if nodeId != "" {
		node, err := GetExchangeNode(getDeviceHandler, nodeId, msgPrinter)
		if err != nil {
			return nil, err
		}
		if input.NodeArch != "" {
			if node.Arch != "" && node.Arch != input.NodeArch {
				return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("The input node architecture %v does not match the exchange node architecture %v for node %v.", input.NodeArch, node.Arch, nodeId)), COMPCHECK_INPUT_ERROR)
			}
		} else {
			resources.NodeArch = node.Arch
		}

		resources.NodeType = node.NodeType
	}

	// verify the input node type value and get the node type for the node from
	// node id or from the input.
	if nodeType, err := VerifyNodeType(input.NodeType, resources.NodeType, nodeId, msgPrinter); err != nil {
		return nil, err
	} else {
		resources.NodeType = nodeType
	}

	// validate node policy and convert it to internal policy
	var nPolicy *policy.Policy
	var err1 error
	resources.NodePolicy, nPolicy, err1 = processNodePolicy(nodePolicyHandler, nodeId, input.NodePolicy, msgPrinter)
	if err1 != nil {
		return nil, err1
	}

	// validate and convert the business policy to internal policy
	var bPolicy *policy.Policy
	bpId := input.BusinessPolId
	resources.BusinessPolicy, bPolicy, err1 = processBusinessPolicy(getBusinessPolicies, bpId, input.BusinessPolicy, true, msgPrinter)
	if err1 != nil {
		return nil, err1
	}

	msg_incompatible := msgPrinter.Sprintf("Policy Incompatible")
	msg_compatible := msgPrinter.Sprintf("Compatible")

	// go through all the workloads and check if compatible or not
	messages := map[string]string{}
	overall_compatible := false
	for _, workload := range bPolicy.Workloads {

		// make sure arch is correct
		if workload.Arch == "*" {
			workload.Arch = ""
		}

		// used for creating sId
		w_arch := workload.Arch
		if w_arch == "" {
			w_arch = "*"
		}

		if resources.NodeArch != "" {
			if workload.Arch == "" {
				workload.Arch = resources.NodeArch
			} else if resources.NodeArch != workload.Arch {
				if checkAllSvcs {
					sId := cutil.FormExchangeIdForService(workload.WorkloadURL, workload.Version, w_arch)
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
				if mergedServicePol, sPol, topSvcDef, sId, err := GetServicePolicyWithDefaultProperties(servicePolicyHandler, getServiceResolvedDef, getService, workload.WorkloadURL, workload.Org, workload.Version, workload.Arch, msgPrinter); err != nil {
					return nil, err
					// compatibility check
				} else {
					if sPol != nil {
						resources.ServicePolicy[sId] = *sPol
					}
					// node type and service type check
					var err1 error
					compatible := true
					reason := ""
					if topSvcDef != nil {
						compatible, reason = CheckTypeCompatibility(resources.NodeType, &ServiceDefinition{workload.Org, *topSvcDef}, msgPrinter)
					}
					if compatible {
						// policy compatibility check
						compatible, reason, _, _, err1 = CheckPolicyCompatiblility(nPolicy, bPolicy, mergedServicePol, resources.NodeArch, msgPrinter)
						if err1 != nil {
							return nil, err1
						}
					}
					if compatible {
						overall_compatible = true
						if checkAllSvcs {
							messages[sId] = msg_compatible
						} else {
							return NewCompCheckOutput(true, map[string]string{sId: msg_compatible}, resources), nil
						}
					} else {
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
						mergedServicePol, sPol, topSvcDef, _, err := GetServicePolicyWithDefaultProperties(servicePolicyHandler, getServiceResolvedDef, getService, workload.WorkloadURL, workload.Org, workload.Version, svc.Arch, msgPrinter)
						if err != nil {
							return nil, err
						} else {
							if sPol != nil {
								resources.ServicePolicy[sId] = *sPol
							}

							// node type and service type check
							compatible := true
							reason := ""
							if topSvcDef != nil {
								compatible, reason = CheckTypeCompatibility(resources.NodeType, &ServiceDefinition{workload.Org, *topSvcDef}, msgPrinter)
							}
							if compatible {
								// policy compatibility check
								compatible, reason, _, _, err = CheckPolicyCompatiblility(nPolicy, bPolicy, mergedServicePol, resources.NodeArch, msgPrinter)
								if err != nil {
									return nil, err
								}
							}
							if compatible {
								overall_compatible = true
								if checkAllSvcs {
									messages[sId] = msg_compatible
								} else {
									return NewCompCheckOutput(true, map[string]string{sId: msg_compatible}, resources), nil
								}
							} else {
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
			mergedServicePol := AddDefaultPropertiesToServicePolicy(input.ServicePolicy, builtInSvcPol, msgPrinter)
			mergedServicePol, topSvcDef, sId, err := SetServicePolicyPrivilege(getServiceResolvedDef, getService, workload, mergedServicePol, input.Service, msgPrinter)
			// node type and service type check
			var err1 error
			compatible := true
			reason := ""
			if err != nil {
				compatible = false
				reason = err.Error()
				sId = cutil.FormExchangeIdForService(workload.WorkloadURL, workload.Version, workload.Arch)
				sId = fmt.Sprintf("%v/%v", workload.Org, sId)
			} else {
				if topSvcDef != nil {
					compatible, reason = CheckTypeCompatibility(resources.NodeType, &ServiceDefinition{workload.Org, *topSvcDef}, msgPrinter)
				}
				if compatible {
					// policy compatibility check
					compatible, reason, _, _, err1 = CheckPolicyCompatiblility(nPolicy, bPolicy, mergedServicePol, resources.NodeArch, msgPrinter)
					if err1 != nil {
						return nil, err1
					}
				}
			}
			if compatible {
				overall_compatible = true
				if checkAllSvcs {
					messages[sId] = msg_compatible
				} else {
					return NewCompCheckOutput(true, map[string]string{sId: msg_compatible}, resources), nil
				}
			} else {
				messages[sId] = fmt.Sprintf("%v: %v", msg_incompatible, reason)
			}
		}
	}

	if messages != nil && len(messages) != 0 {
		return NewCompCheckOutput(overall_compatible, messages, resources), nil
	} else {
		// If we get here, it means that no workload is found in the bp that matches the required node arch.
		if resources.NodeArch != "" {
			messages["general"] = fmt.Sprintf("%v: %v", msg_incompatible, msgPrinter.Sprintf("Service with 'arch' %v cannot be found in the deployment policy.", resources.NodeArch))
		} else {
			messages["general"] = fmt.Sprintf("%v: %v", msg_incompatible, msgPrinter.Sprintf("No services found in the deployment policy."))
		}

		return NewCompCheckOutput(false, messages, resources), nil
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
		return false, "", nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Deployment policy cannot be null.")), COMPCHECK_INPUT_ERROR)
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
		return false, err.ShortString(), mergedProducerPol, mergedConsumerPol, nil
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
				return nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Failed to convert node policy to internal policy format for node %v: %v", nodeId, err)), COMPCHECK_CONVERSION_ERROR)
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
		return nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Failed to convert node policy to internal policy for node %v: %v", nodeId, err)), COMPCHECK_CONVERSION_ERROR)
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
				return nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Failed to convert deployment policy %v to internal policy: %v", bpId, err1)), COMPCHECK_CONVERSION_ERROR)
			}
			return inputBP, pPolicy, nil
		} else {
			// validate the input policy
			if err := inputBP.Validate(); err != nil {
				return nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Validation failure for deployment policy %v. %v", bpId, err)), COMPCHECK_VALIDATION_ERROR)
			}
			if inputBP.Service.ServiceVersions == nil || len(inputBP.Service.ServiceVersions) == 0 {
				return nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("No services specified in the given deployment policy %v.", bpId)), COMPCHECK_VALIDATION_ERROR)
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
			return nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("No services specified in the deployment policy %v.", pPolicy.Header.Name)), COMPCHECK_VALIDATION_ERROR)
		}
		return bPolicy, pPolicy, nil
	} else {
		return nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Neither deployment policy nor deployment policy id is specified.")), COMPCHECK_INPUT_ERROR)
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
		return nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Deployment policy id is empty.")), COMPCHECK_INPUT_ERROR)
	} else {
		bpOrg := exchange.GetOrg(bpId)
		if bpOrg == "" {
			return nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Organization is not specified in the deployment policy id: %v.", bpId)), COMPCHECK_INPUT_ERROR)
		}
	}

	// get poicy from the exchange
	exchPols, err := getBusinessPolicies(exchange.GetOrg(bpId), exchange.GetId(bpId))
	if err != nil {
		return nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Unable to get deployment policy for %v, %v", bpId, err)), COMPCHECK_EXCHANGE_ERROR)
	}
	if exchPols == nil || len(exchPols) == 0 {
		return nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("No deployment policy found for this id %v.", bpId)), COMPCHECK_INPUT_ERROR)
	}

	// convert the business policy to internal policy format
	for polId, exchPol := range exchPols {
		bPol := exchPol.GetBusinessPolicy()

		if convert {
			pPolicy, err := bPol.GenPolicyFromBusinessPolicy(polId)
			if err != nil {
				return nil, nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Failed to convert deployment policy %v to internal policy format: %v", bpId, err)), COMPCHECK_CONVERSION_ERROR)
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
func GetServicePolicyWithDefaultProperties(servicePolicyHandler exchange.ServicePolicyHandler,
	getServiceResolvedDef exchange.ServiceDefResolverHandler, getService exchange.ServiceHandler,
	svcUrl string, svcOrg string, svcVersion string, svcArch string,
	msgPrinter *message.Printer) (*externalpolicy.ExternalPolicy, *externalpolicy.ExternalPolicy, *exchange.ServiceDefinition, string, error) {
	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	servicePol, sId, err := GetServicePolicy(servicePolicyHandler, svcUrl, svcOrg, svcVersion, svcArch, msgPrinter)
	if err != nil {
		return nil, nil, nil, "", err
	}

	// get default service properties
	builtInSvcPol := externalpolicy.CreateServiceBuiltInPolicy(svcUrl, svcOrg, svcVersion, svcArch)

	if err != nil {
		return nil, nil, nil, "", err
	}

	// add built-in service properties to the service policy
	merged_pol := AddDefaultPropertiesToServicePolicy(servicePol, builtInSvcPol, msgPrinter)
	merged_pol, topSvcDef, _, err := SetServicePolicyPrivilege(getServiceResolvedDef, getService, policy.Workload{WorkloadURL: svcUrl, Org: svcOrg, Version: svcVersion, Arch: svcArch}, merged_pol, nil, msgPrinter)
	if err != nil {
		return nil, nil, nil, "", err
	}
	return merged_pol, servicePol, topSvcDef, sId, nil
}

// Add service default properties to the given service policy
func AddDefaultPropertiesToServicePolicy(servicePol, defaultSvcProps *externalpolicy.ExternalPolicy,
	msgPrinter *message.Printer) *externalpolicy.ExternalPolicy {
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
		return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("The given deployment policy should not be null.")), COMPCHECK_INPUT_ERROR)
	}
	if servicePolicy == nil {
		return businessPolicy, nil
	}

	if pPolicy, err := policy.MergePolicyWithExternalPolicy(businessPolicy, servicePolicy); err != nil {
		return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Error merging deployment policy with service policy. %v", err)), COMPCHECK_MERGING_ERROR)
	} else {
		return pPolicy, nil
	}
}

// This function merges the given business policy with the given built-in properties of the service and the given service policy
// from the top level service, if any.
func MergeServicePolicyToBusinessPolicy(getServiceResolvedDef exchange.ServiceDefResolverHandler, getService exchange.ServiceHandler,
	workload policy.Workload, businessPol *policy.Policy, builtInSvcPol *externalpolicy.ExternalPolicy, servicePol *externalpolicy.ExternalPolicy, msgPrinter *message.Printer) (*policy.Policy, error) {
	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	if businessPol == nil {
		return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("The deployment policy should not be null.")), COMPCHECK_INPUT_ERROR)
	}

	// add built-in service properties to the service policy
	merged_pol1 := AddDefaultPropertiesToServicePolicy(servicePol, builtInSvcPol, msgPrinter)
	var err error
	if merged_pol1, _, _, err = SetServicePolicyPrivilege(getServiceResolvedDef, getService, workload, merged_pol1, nil, msgPrinter); err != nil {
		return nil, err
	}

	if businessPol.Properties.HasProperty(externalpolicy.PROP_SVC_PRIVILEGED) {
		declPrivProp, err := businessPol.Properties.GetProperty(externalpolicy.PROP_SVC_PRIVILEGED)
		if err != nil {
			return nil, NewCompCheckError(err, COMPCHECK_GENERAL_ERROR)
		}
		declPriv := declPrivProp.Value.(bool)
		svcPrivProp, err := merged_pol1.Properties.GetProperty(externalpolicy.PROP_SVC_PRIVILEGED)
		if err != nil {
			return nil, NewCompCheckError(err, COMPCHECK_GENERAL_ERROR)
		}
		svcPriv := svcPrivProp.Value.(bool)
		if svcPriv && !declPriv {
			return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Deployment policy %s cannot have %s=true when referring to a service that requires privilege to run.", businessPol.Header, externalpolicy.PROP_NODE_PRIVILEGED)), COMPCHECK_INPUT_ERROR)
		}
	}
	//merge service policy
	if merged_pol2, err := MergeFullServicePolicyToBusinessPolicy(businessPol, merged_pol1, msgPrinter); err != nil {
		return nil, err
	} else {
		return merged_pol2, nil
	}
}

// SetServicePolicyPrivilege sets a property on the service privilege that indicates if the service uses a workload that requires privileged mode or network=host
// This will not overwrite openhorizon.allowPrivileged=true if the service is found to not require privileged mode.
func SetServicePolicyPrivilege(getServiceResolvedDef exchange.ServiceDefResolverHandler, getService exchange.ServiceHandler,
	workload policy.Workload, svcPolicy *externalpolicy.ExternalPolicy, svcDefs []common.ServiceFile,
	msgPrinter *message.Printer) (*externalpolicy.ExternalPolicy, *exchange.ServiceDefinition, string, error) {
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	svcList := map[string]exchange.ServiceDefinition{}
	var topSvcDef *exchange.ServiceDefinition
	var topSvcId string
	var sDefMap map[string]exchange.ServiceDefinition
	var err error
	if svcDefs == nil || len(svcDefs) == 0 {
		sDefMap, topSvcDef, topSvcId, err = getServiceResolvedDef(workload.WorkloadURL, workload.Org, workload.Version, workload.Arch)
		if err != nil {
			return nil, nil, "", NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Error retrieving service %v/%v %v %v and its dependents from the exchange. %v", workload.Org, workload.WorkloadURL, workload.Version, workload.Arch, err)), COMPCHECK_EXCHANGE_ERROR)
		}
		if sDefMap != nil {
			svcList = sDefMap
		}

		if topSvcDef != nil {
			svcList[topSvcId] = *topSvcDef
		}
	} else {
		sDefMap, topSvcDef, topSvcId, err = getServiceListFromInputDefs(getServiceResolvedDef, workload, svcDefs, msgPrinter)
		if err != nil {
			return nil, nil, "", err
		}
		if topSvcDef != nil {
			svcList[topSvcId] = *topSvcDef
		}
	}

	runtimePriv, err, _ := servicesRequirePrivilege(&svcList, msgPrinter)
	if err != nil {
		return nil, nil, "", err
	}

	svcPolicy.Properties.Add_Property(externalpolicy.Property_Factory(externalpolicy.PROP_SVC_PRIVILEGED, runtimePriv), true)

	if runtimePriv {
		svcPolicy.Constraints.Add_Constraint(fmt.Sprintf("%s = %t", externalpolicy.PROP_SVC_PRIVILEGED, runtimePriv))
	}
	return svcPolicy, topSvcDef, topSvcId, nil
}

// Given a list of service def files for top level services and a workload,
// get the service def from the list and go to the exchange and fetch all of its dependent services.
// It returns a list of all dependent services, the top level service and id.
func getServiceListFromInputDefs(getServiceResolvedDef exchange.ServiceDefResolverHandler,
	workload policy.Workload, svcDefs []common.ServiceFile,
	msgPrinter *message.Printer) (map[string]exchange.ServiceDefinition, *exchange.ServiceDefinition, string, error) {

	// find the top level service that matches the workload from the input services
	var sDefTop *exchange.ServiceDefinition
	sIdTop := ""
	found := false
	for _, in_svc := range svcDefs {
		if in_svc.GetURL() == workload.WorkloadURL && in_svc.GetVersion() == workload.Version &&
			(workload.Arch == "*" || workload.Arch == "" || in_svc.GetArch() == workload.Arch) &&
			(in_svc.GetOrg() == "" || in_svc.GetOrg() == workload.Org) {
			found = true
			sDefTop = &exchange.ServiceDefinition{Label: in_svc.Label, Description: in_svc.Description, Public: in_svc.Public, Documentation: in_svc.Documentation, URL: in_svc.URL, Version: in_svc.Version, Arch: in_svc.Arch, Sharable: in_svc.Sharable, MatchHardware: in_svc.MatchHardware, RequiredServices: in_svc.RequiredServices, UserInputs: in_svc.UserInputs}
			sIdTop = cutil.FormExchangeIdForService(sDefTop.URL, sDefTop.Version, sDefTop.Arch)
			sIdTop = fmt.Sprintf("%v/%v", workload.Org, sIdTop)
			sDefTop.Deployment = ""
			if in_svc.Deployment != nil && in_svc.Deployment != "" {
				if deployment, err := json.Marshal(in_svc.Deployment); err != nil {
					return nil, nil, "", NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Failed to marshal deployment configuration for service %v, error %v", sIdTop, err)), COMPCHECK_GENERAL_ERROR)
				} else {
					sDefTop.Deployment = string(deployment)
				}
			}
			sDefTop.ClusterDeployment = ""
			if in_svc.ClusterDeployment != nil && in_svc.ClusterDeployment != "" {
				if cluster_deployment, err := json.Marshal(in_svc.ClusterDeployment); err != nil {
					return nil, nil, "", NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Failed to marshal cluster deployment configuration for service %v, error %v", sIdTop, err)), COMPCHECK_GENERAL_ERROR)
				} else {
					sDefTop.ClusterDeployment = string(cluster_deployment)
				}
			}

			break
		}
	}
	if !found {
		return nil, nil, "", NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Unable to find service definition from the input services.")), COMPCHECK_GENERAL_ERROR)
	}

	// get all the service defs for the dependent services
	service_map := map[string]exchange.ServiceDefinition{}
	if sDefTop.RequiredServices != nil && len(sDefTop.RequiredServices) != 0 {
		for _, sDep := range sDefTop.RequiredServices {
			if vExp, err := semanticversion.Version_Expression_Factory(sDep.GetVersionRange()); err != nil {
				return nil, nil, "", NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Unable to create version expression from %v. %v", sDep.Version, err)), COMPCHECK_GENERAL_ERROR)
			} else {
				if s_map, s_def, s_id, err := getServiceResolvedDef(sDep.URL, sDep.Org, vExp.Get_expression(), sDep.Arch); err != nil {
					return nil, nil, "", NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Error retrieving dependent services from the exchange for %v. %v", sDep, err)), COMPCHECK_EXCHANGE_ERROR)
				} else {
					service_map[s_id] = *s_def
					for id, s := range s_map {
						service_map[id] = s
					}
				}
			}
		}
	}
	return service_map, sDefTop, sIdTop, nil
}
