package compcheck

import (
	"fmt"
	"github.com/open-horizon/anax/businesspolicy"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/policy"
)

// **** Note: All the errors returned by the functions in this file are of type *CompCheckError,
// You can cast it by err.(*compcheck.CompCheckError) if you want to get the error code.

// error code for compatibility check error: CompCheckError
const (
	COMPCHECK_INPUT_ERROR      = 10
	COMPCHECK_VALIDATION_ERROR = 11
	COMPCHECK_CONVERTION_ERROR = 12
	COMPCHECK_MERGING_ERROR    = 13
	COMPCHECK_EXCHANGE_ERROR   = 14
)

const (
	COMPATIBLE   = "compatible"
	INCOMPATIBLE = "incompatible"
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

// The output format for the policy check
type PolicyCompOutput struct {
	Compatible bool              `json:"compatible"`
	Reason     map[string]string `json:"reason"` // set when not compatible
	Input      *PolicyCompInput  `json:"input,omitempty"`
}

func (p *PolicyCompOutput) String() string {
	return fmt.Sprintf("Compatible: %v, Reason: %v, Input: %v",
		p.Compatible, p.Reason, p.Input)

}

func NewPolicyCompOutput(compatible bool, reason map[string]string, input *PolicyCompInput) *PolicyCompOutput {
	return &PolicyCompOutput{
		Compatible: compatible,
		Reason:     reason,
		Input:      input,
	}
}

// The input format for the policy check
type PolicyCompInput struct {
	NodeId         string                         `json:"node_id,omitempty"`
	NodeArch       string                         `json:"node_arch,omitempty"`
	NodePolicy     *externalpolicy.ExternalPolicy `json:"node_policy,omitempty"`
	BusinessPolId  string                         `json:"business_policy_id,omitempty"`
	BusinessPolicy *businesspolicy.BusinessPolicy `json:"business_policy,omitempty"`
	ServicePolicy  *externalpolicy.ExternalPolicy `json:"service_policy,omitempty"`
}

func (p PolicyCompInput) String() string {
	return fmt.Sprintf("NodeId: %v, NodeArch: %v, NodePolicy: %v, BusinessPolId: %v, BusinessPolicy: %v, ServicePolicy: %v,",
		p.NodeId, p.NodeArch, p.NodePolicy, p.BusinessPolId, p.BusinessPolicy, p.ServicePolicy)

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

// This is the function that HZN and the agbot secure API calls.
// Given the PolicyCompInput, check if the policies are compatible.
// The required fields in PolicyCompInput are:
//  (NodeId or NodePolicy) and (BusinessPolId or BusinessPolicy)
//
// When checking whether the policies are compatible or not, we devide policies into two side:
//    Edge side: node policy (including the node built-in policy)
//    Agbot side: business policy + service policy + service built-in properties
func PolicyCompatible(ec exchange.ExchangeContext, pcInput *PolicyCompInput, checkAllSvcs bool) (*PolicyCompOutput, error) {

	getDeviceHandler := exchange.GetHTTPDeviceHandler(ec)
	nodePolicyHandler := exchange.GetHTTPNodePolicyHandler(ec)
	getBusinessPolicies := exchange.GetHTTPBusinessPoliciesHandler(ec)
	servicePolicyHandler := exchange.GetHTTPServicePolicyHandler(ec)
	getSelectedServices := exchange.GetHTTPSelectedServicesHandler(ec)

	return policyCompatible(getDeviceHandler, nodePolicyHandler, getBusinessPolicies, servicePolicyHandler, getSelectedServices, pcInput, checkAllSvcs)
}

// Internal function for PolicyCompatible
func policyCompatible(getDeviceHandler exchange.DeviceHandler,
	nodePolicyHandler exchange.NodePolicyHandler,
	getBusinessPolicies exchange.BusinessPoliciesHandler,
	servicePolicyHandler exchange.ServicePolicyHandler,
	getSelectedServices exchange.SelectedServicesHandler,
	pcInput *PolicyCompInput, checkAllSvcs bool) (*PolicyCompOutput, error) {
	if pcInput == nil {
		return nil, NewCompCheckError(fmt.Errorf("The input cannot be null"), COMPCHECK_INPUT_ERROR)
	}

	// make a copy of the input because the process will change it. The pointer to policies will stay the same.
	input_temp := PolicyCompInput(*pcInput)
	input := &input_temp

	// validate node policy and convert it to internal policy
	var nPolicy *policy.Policy
	nodeId := input.NodeId
	if input.NodePolicy != nil {
		if err := input.NodePolicy.Validate(); err != nil {
			return nil, NewCompCheckError(fmt.Errorf("Failed to validate the node policy. %v", err), COMPCHECK_VALIDATION_ERROR)
		} else {
			// give nodeId a default if it is empty
			if nodeId == "" {
				nodeId = "TempNodePolicyId"
			}

			var err1 error
			nPolicy, err1 = policy.GenPolicyFromExternalPolicy(input.NodePolicy, policy.MakeExternalPolicyHeaderName(nodeId))
			if err1 != nil {
				return nil, NewCompCheckError(fmt.Errorf("Failed to convert node policy to internal policy format for node %v: %v", nodeId, err1), COMPCHECK_CONVERTION_ERROR)
			}
		}
	} else if nodeId != "" {
		if node, err := GetExchangeNode(getDeviceHandler, nodeId); err != nil {
			return nil, err
		} else if node == nil {
			return nil, NewCompCheckError(fmt.Errorf("No node found for this node id %v.", nodeId), COMPCHECK_INPUT_ERROR)
		} else if input.NodeArch != "" {
			if node.Arch != "" && node.Arch != input.NodeArch {
				return nil, NewCompCheckError(fmt.Errorf("The input node architecture %v does not match the node architecture %v for %v.", input.NodeArch, node.Arch, nodeId), COMPCHECK_INPUT_ERROR)
			}
		} else {
			input.NodeArch = node.Arch
		}

		var err1 error
		input.NodePolicy, nPolicy, err1 = GetNodePolicy(nodePolicyHandler, nodeId)
		if err1 != nil {
			return nil, err1
		} else if nPolicy == nil {
			return nil, NewCompCheckError(fmt.Errorf("No node policy found for this node %v.", nodeId), COMPCHECK_INPUT_ERROR)
		}
	} else {
		return nil, NewCompCheckError(fmt.Errorf("Neither node policy nor node id is specified."), COMPCHECK_INPUT_ERROR)
	}

	// validate and convert the business policy to internal policy
	var bPolicy *policy.Policy
	bpId := input.BusinessPolId
	if input.BusinessPolicy != nil {
		// give bpId a default value if it is empty
		if bpId == "" {
			bpId = "TempBusinessPolicyId"
		}
		//convert busineess policy to internal policy, the validate is done in this function
		var err1 error
		bPolicy, err1 = input.BusinessPolicy.GenPolicyFromBusinessPolicy(bpId)
		if err1 != nil {
			return nil, NewCompCheckError(fmt.Errorf("Failed to convert business policy %v to internal policy format: %v", bpId, err1), COMPCHECK_CONVERTION_ERROR)
		}
	} else if bpId != "" {
		var err1 error
		input.BusinessPolicy, bPolicy, err1 = GetBusinessPolicy(getBusinessPolicies, bpId)
		if err1 != nil {
			return nil, err1
		} else if bPolicy == nil {
			return nil, NewCompCheckError(fmt.Errorf("No business policy found for this id %v.", bpId), COMPCHECK_INPUT_ERROR)
		}
	} else {
		return nil, NewCompCheckError(fmt.Errorf("Neither business policy nor business policy is are specified."), COMPCHECK_INPUT_ERROR)
	}

	if bPolicy.Workloads == nil || len(bPolicy.Workloads) == 0 {
		return nil, NewCompCheckError(fmt.Errorf("No services specified in the business policy %v.", bPolicy.Header.Name), COMPCHECK_VALIDATION_ERROR)
	}

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
					sId := cliutils.FormExchangeIdForService(workload.WorkloadURL, workload.Version, workload.Arch)
					sId = fmt.Sprintf("%v/%v", workload.Org, sId)
					messages[sId] = fmt.Sprintf("%v: %v", INCOMPATIBLE, "Architecture does not match.")
				}
				continue
			}
		}

		// get service policy + default properties
		if input.ServicePolicy == nil {
			if workload.Arch != "" {
				// get service policy with built-in properties
				if mergedServicePol, sPol, sId, err := GetServicePolicyWithDefaultProperties(servicePolicyHandler, workload.WorkloadURL, workload.Org, workload.Version, workload.Arch); err != nil {
					return nil, err
					// compatibility check
				} else {
					if compatible, reason, _, _, err := CheckPolicyCompatiblility(nPolicy, bPolicy, mergedServicePol, input.NodeArch); err != nil {
						return nil, err
					} else if compatible {
						overall_compatible = true
						if checkAllSvcs {
							servicePol_comp = sPol
							messages[sId] = COMPATIBLE
						} else {
							input.ServicePolicy = sPol
							return NewPolicyCompOutput(true, map[string]string{sId: COMPATIBLE}, input), nil
						}
					} else {
						servicePol_incomp = sPol
						messages[sId] = fmt.Sprintf("%v: %v", INCOMPATIBLE, reason)
					}
				}
			} else {
				if svcMeta, err := getSelectedServices(workload.WorkloadURL, workload.Org, workload.Version, workload.Arch); err != nil {
					return nil, NewCompCheckError(fmt.Errorf("Failed to get services for all archetctures for %v/%v version %v. %v", workload.Org, workload.WorkloadURL, workload.Version, err), COMPCHECK_EXCHANGE_ERROR)
				} else {
					// since workload arch is empty, need to go through all the arches
					for sId, svc := range svcMeta {
						// get service policy with built-in properties
						if mergedServicePol, sPol, _, err := GetServicePolicyWithDefaultProperties(servicePolicyHandler, workload.WorkloadURL, workload.Org, workload.Version, svc.Arch); err != nil {
							return nil, err
							// compatibility check
						} else {
							if compatible, reason, _, _, err := CheckPolicyCompatiblility(nPolicy, bPolicy, mergedServicePol, input.NodeArch); err != nil {
								return nil, err
							} else if compatible {
								overall_compatible = true
								if checkAllSvcs {
									servicePol_comp = sPol
									messages[sId] = COMPATIBLE
								} else {
									input.ServicePolicy = sPol
									return NewPolicyCompOutput(true, map[string]string{sId: COMPATIBLE}, input), nil
								}
							} else {
								servicePol_incomp = sPol
								messages[sId] = fmt.Sprintf("%v: %v", INCOMPATIBLE, reason)
							}
						}
					}
				}
			}
		} else {
			// get default service properties
			builtInSvcPol := externalpolicy.CreateServiceBuiltInPolicy(workload.WorkloadURL, workload.Org, workload.Version, workload.Arch)
			// add built-in service properties to the service policy
			mergedServicePol := AddDefaultPropertiesToServicePolicy(input.ServicePolicy, builtInSvcPol)
			// compatibility check
			sId := cliutils.FormExchangeIdForService(workload.WorkloadURL, workload.Version, workload.Arch)
			sId = fmt.Sprintf("%v/%v", workload.Org, sId)
			if compatible, reason, _, _, err := CheckPolicyCompatiblility(nPolicy, bPolicy, mergedServicePol, input.NodeArch); err != nil {
				return nil, err
			} else if compatible {
				overall_compatible = true
				if checkAllSvcs {
					messages[sId] = COMPATIBLE
				} else {
					return NewPolicyCompOutput(true, map[string]string{sId: COMPATIBLE}, input), nil
				}
			} else {
				messages[sId] = fmt.Sprintf("%v:%v", INCOMPATIBLE, reason)
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
		return NewPolicyCompOutput(overall_compatible, messages, input), nil
	} else {
		if input.NodeArch != "" {
			messages["general"] = fmt.Sprintf("%v: Service with 'arch' %v cannot be found in the business policy.", INCOMPATIBLE, input.NodeArch)
		} else {
			messages["general"] = fmt.Sprintf("%v: No services found in the bussiness policy.", INCOMPATIBLE)
		}

		return NewPolicyCompOutput(false, messages, input), nil
	}
}

// It does the policy compatibility check. node arch can be empty. It is called by agbot and PolicyCompatible function.
// The node arch is supposed to be already compared against the service arch before calling this function.
func CheckPolicyCompatiblility(nodePolicy *policy.Policy, businessPolicy *policy.Policy, mergedServicePolicy *externalpolicy.ExternalPolicy, nodeArch string) (bool, string, *policy.Policy, *policy.Policy, error) {
	// check node policy
	if nodePolicy == nil {
		return false, "", nil, nil, NewCompCheckError(fmt.Errorf("Node policy cannot be nil."), COMPCHECK_INPUT_ERROR)
	}

	// check bussiness policy
	if businessPolicy == nil {
		return false, "", nil, nil, NewCompCheckError(fmt.Errorf("Business policy cannot be nil."), COMPCHECK_INPUT_ERROR)
	}

	// check merged service policy
	if mergedServicePolicy == nil {
		return false, "", nil, nil, NewCompCheckError(fmt.Errorf("Merged service policy cannot be nil."), COMPCHECK_INPUT_ERROR)
	}

	// validate service policy
	if err := mergedServicePolicy.Validate(); err != nil {
		return false, "", nil, nil, NewCompCheckError(fmt.Errorf("Failed to validate the service policy. %v", err), COMPCHECK_VALIDATION_ERROR)
	}

	// merge the service policy, the default service properties
	mergedConsumerPol, err := MergeFullServicePolicyToBusinessPolicy(businessPolicy, mergedServicePolicy)
	if err != nil {
		return false, "", nil, nil, err
	}

	// Add node arch properties to the node policy
	mergedProducerPol, err := addNodeArchToPolicy(nodePolicy, nodeArch)
	if err != nil {
		return false, "", nil, nil, err
	}

	// check if the node policy and merged bp policy are compatible
	if err := policy.Are_Compatible(nodePolicy, mergedConsumerPol); err != nil {
		return false, err.Error(), mergedProducerPol, mergedConsumerPol, nil
	} else {
		// policy match
		return true, "", mergedProducerPol, mergedConsumerPol, nil
	}
}

// add node arch property to the node policy. node arch can be empty
func addNodeArchToPolicy(nodePolicy *policy.Policy, nodeArch string) (*policy.Policy, error) {
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
		return nil, NewCompCheckError(fmt.Errorf("error merging node policy with arch property. %v", err), COMPCHECK_MERGING_ERROR)
	} else {
		return pPolicy, nil
	}

}

// Get the exchange device
func GetExchangeNode(getDeviceHandler exchange.DeviceHandler, nodeId string) (*exchange.Device, error) {
	// check input
	if nodeId == "" {
		return nil, NewCompCheckError(fmt.Errorf("Node id is empty."), COMPCHECK_INPUT_ERROR)
	} else {
		nodeOrg := exchange.GetOrg(nodeId)
		if nodeOrg == "" {
			return nil, NewCompCheckError(fmt.Errorf("Organization is not specified in the node id: %v.", nodeId), COMPCHECK_INPUT_ERROR)
		}
	}

	if node, err := getDeviceHandler(nodeId, ""); err != nil {
		return nil, NewCompCheckError(fmt.Errorf("Error getting node %v from the exchange. %v", nodeId, err), COMPCHECK_EXCHANGE_ERROR)
	} else {
		return node, err
	}
}

// Get node policy from the exchange and convert it to internal policy.
// It returns (nil, nil) If there is no node policy found.
func GetNodePolicy(nodePolicyHandler exchange.NodePolicyHandler, nodeId string) (*externalpolicy.ExternalPolicy, *policy.Policy, error) {

	// check input
	if nodeId == "" {
		return nil, nil, NewCompCheckError(fmt.Errorf("Node id is empty."), COMPCHECK_INPUT_ERROR)
	} else {
		nodeOrg := exchange.GetOrg(nodeId)
		if nodeOrg == "" {
			return nil, nil, NewCompCheckError(fmt.Errorf("Organization is not specified in the node id: %v.", nodeId), COMPCHECK_INPUT_ERROR)
		}
	}

	// get node policy
	nodePolicy, err := nodePolicyHandler(nodeId)
	if err != nil {
		return nil, nil, NewCompCheckError(fmt.Errorf("Error trying to query node policy for %v: %v", nodeId, err), COMPCHECK_EXCHANGE_ERROR)
	}

	if nodePolicy == nil {
		return nil, nil, nil
	}

	// validate the policy
	extPolicy := nodePolicy.GetExternalPolicy()
	if err := extPolicy.Validate(); err != nil {
		return nil, nil, NewCompCheckError(fmt.Errorf("Failed to validate the node policy for node %v. %v", nodeId, err), COMPCHECK_VALIDATION_ERROR)
	}

	// convert the policy to internal policy format
	pPolicy, err := policy.GenPolicyFromExternalPolicy(&extPolicy, policy.MakeExternalPolicyHeaderName(nodeId))
	if err != nil {
		return nil, nil, NewCompCheckError(fmt.Errorf("Failed to convert node policy to internal policy format for node %v: %v", nodeId, err), COMPCHECK_CONVERTION_ERROR)
	}
	return &extPolicy, pPolicy, nil
}

// Get business policy from the exchange the convert it to internal policy
func GetBusinessPolicy(getBusinessPolicies exchange.BusinessPoliciesHandler, bpId string) (*businesspolicy.BusinessPolicy, *policy.Policy, error) {

	// check input
	if bpId == "" {
		return nil, nil, NewCompCheckError(fmt.Errorf("Business policy id is empty."), COMPCHECK_INPUT_ERROR)
	} else {
		bpOrg := exchange.GetOrg(bpId)
		if bpOrg == "" {
			return nil, nil, NewCompCheckError(fmt.Errorf("Organization is not specified in the business policy id: %v.", bpId), COMPCHECK_INPUT_ERROR)
		}
	}

	// get poicy from the exchange
	exchPols, err := getBusinessPolicies(exchange.GetOrg(bpId), exchange.GetId(bpId))
	if err != nil {
		return nil, nil, NewCompCheckError(fmt.Errorf("Unable to get business policy for %v, %v", bpId, err), COMPCHECK_EXCHANGE_ERROR)
	}
	if exchPols == nil || len(exchPols) == 0 {
		return nil, nil, nil
	}

	// convert the business policy to internal policy format
	for polId, exchPol := range exchPols {
		bPol := exchPol.GetBusinessPolicy()

		pPolicy, err := bPol.GenPolicyFromBusinessPolicy(polId)
		if err != nil {
			return nil, nil, NewCompCheckError(fmt.Errorf("Failed to convert business policy %v to internal policy format: %v", bpId, err), COMPCHECK_CONVERTION_ERROR)
		}
		return &bPol, pPolicy, nil
	}

	return nil, nil, nil
}

// Get service policy from the exchange
func GetServicePolicyWithId(serviceIdPolicyHandler exchange.ServicePolicyWithIdHandler, svcId string) (*externalpolicy.ExternalPolicy, error) {
	// check input
	if svcId == "" {
		return nil, NewCompCheckError(fmt.Errorf("Service policy id is empty."), COMPCHECK_INPUT_ERROR)
	} else {
		svcOrg := exchange.GetOrg(svcId)
		if svcOrg == "" {
			return nil, NewCompCheckError(fmt.Errorf("Organization is not specified in the service policy id: %v.", svcId), COMPCHECK_INPUT_ERROR)
		}
	}

	// get service policy from the exchange
	servicePolicy, err := serviceIdPolicyHandler(svcId)
	if err != nil {
		return nil, NewCompCheckError(fmt.Errorf("Error trying to query service policy for %v: %v", svcId, err), COMPCHECK_EXCHANGE_ERROR)
	} else if servicePolicy == nil {
		return nil, nil
	} else {
		// validate the policy
		extPolicy := servicePolicy.GetExternalPolicy()
		if err := extPolicy.Validate(); err != nil {
			return nil, NewCompCheckError(fmt.Errorf("Failed to validate the service policy %v. %v", svcId, err), COMPCHECK_VALIDATION_ERROR)
		}
		return &extPolicy, nil
	}
}

// Get service policy from the exchange,
func GetServicePolicy(servicePolicyHandler exchange.ServicePolicyHandler, svcUrl string, svcOrg string, svcVersion string, svcArch string) (*externalpolicy.ExternalPolicy, string, error) {
	// check input
	if svcUrl == "" {
		return nil, "", NewCompCheckError(fmt.Errorf("Service name is empty."), COMPCHECK_INPUT_ERROR)
	} else if svcOrg == "" {
		return nil, "", NewCompCheckError(fmt.Errorf("Service organization is empty."), COMPCHECK_INPUT_ERROR)
	}

	// get service policy fromt the exchange
	servicePolicy, sId, err := servicePolicyHandler(svcUrl, svcOrg, svcVersion, svcArch)
	if err != nil {
		return nil, "", NewCompCheckError(fmt.Errorf("Error trying to query service policy for %v/%v %v %v. %v", svcOrg, svcUrl, svcVersion, svcArch, err), COMPCHECK_EXCHANGE_ERROR)
	} else if servicePolicy == nil {
		return nil, sId, nil
	} else {
		extPolicy := servicePolicy.GetExternalPolicy()
		if err := extPolicy.Validate(); err != nil {
			return nil, sId, NewCompCheckError(fmt.Errorf("Failed to validate the service policy for %v/%v %v %v. %v", svcOrg, svcUrl, svcVersion, svcArch, err), COMPCHECK_VALIDATION_ERROR)
		}
		return &extPolicy, sId, nil
	}
}

// Get service policy from the exchange and then add the service defalt properties
func GetServicePolicyWithDefaultProperties(servicePolicyHandler exchange.ServicePolicyHandler, svcUrl string, svcOrg string, svcVersion string, svcArch string) (*externalpolicy.ExternalPolicy, *externalpolicy.ExternalPolicy, string, error) {

	servicePol, sId, err := GetServicePolicy(servicePolicyHandler, svcUrl, svcOrg, svcVersion, svcArch)
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
func MergeFullServicePolicyToBusinessPolicy(businessPolicy *policy.Policy, servicePolicy *externalpolicy.ExternalPolicy) (*policy.Policy, error) {
	if businessPolicy == nil {
		return nil, NewCompCheckError(fmt.Errorf("The given business policy should not be nil for MergeFullServicePolicyToBusinessPolicy."), COMPCHECK_INPUT_ERROR)
	}
	if servicePolicy == nil {
		return businessPolicy, nil
	}

	if pPolicy, err := policy.MergePolicyWithExternalPolicy(businessPolicy, servicePolicy); err != nil {
		return nil, NewCompCheckError(fmt.Errorf("error merging business policy with service policy. %v", err), COMPCHECK_MERGING_ERROR)
	} else {
		return pPolicy, nil
	}
}

// This function merges the given business policy with the given built-in properties of the service and the given service policy
// from the top level service, if any.
func MergeServicePolicyToBusinessPolicy(businessPol *policy.Policy, builtInSvcPol *externalpolicy.ExternalPolicy, servicePol *externalpolicy.ExternalPolicy) (*policy.Policy, error) {
	if businessPol == nil {
		return nil, NewCompCheckError(fmt.Errorf("The given business policy should not be nil for MergeServicePolicyToBusinessPolicy."), COMPCHECK_INPUT_ERROR)
	}

	// add built-in service properties to the service policy
	merged_pol1 := AddDefaultPropertiesToServicePolicy(servicePol, builtInSvcPol)

	//merge service policy
	if merged_pol2, err := MergeFullServicePolicyToBusinessPolicy(businessPol, merged_pol1); err != nil {
		return nil, err
	} else {
		return merged_pol2, nil
	}
}
