package policy

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/i18n"
	"golang.org/x/text/message"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

// The purpose this file is to abstract the Policy struct and the files that it lives within as
// a serialized JSON document. Policy documents are complex structs that each live within
// their own file. There are functions in here to read and write those files as well as
// dynamically discover the existence of new files without the need to restart.

// Following is the structure of a policy document. One of the most important things
// to note is that the workload struct references payload types from the old payloads.json file.
// This is intentional so that there can be a smooth transition from that file to the new
// policy file structure.

const version1 = "1.0" // Policy document schema version (in case we need it)
const version2 = "2.0"
const CurrentVersion = version2 // Current schema version

type PolicyHeader struct {
	Name    string `json:"name"`    // Name assigned to this policy by its author
	Version string `json:"version"` // The schema version of this file
}

func (h PolicyHeader) IsSame(compare PolicyHeader) bool {
	return h.Name == compare.Name && h.Version == compare.Version
}

type ValueExchange struct {
	Type        string `json:"type,omitempty"`        // The type of value exchange
	Value       string `json:"value,omitempty"`       // The value being exchanged
	PaymentRate int    `json:"paymentRate,omitempty"` // The number of seconds between payments
	Token       string `json:"token,omitempty"`       // A token used to identify the user of the value - added in version 2
}

type ProposalRejection struct {
	Number   int `json:"number,omitempty"`   // Number of rejections before giving up on making agreements
	Duration int `json:"duration,omitempty"` // The length of time to wait before trying again
}

// This is the main struct that defines the Policy object.
type Policy struct {
	Header             PolicyHeader                        `json:"header"`
	PatternId          string                              `json:"patternId,omitempty"` // Manually created policy files should NOT use this field.
	APISpecs           APISpecList                         `json:"apiSpec,omitempty"`
	AgreementProtocols AgreementProtocolList               `json:"agreementProtocols,omitempty"`
	Workloads          WorkloadList                        `json:"workloads,omitempty"`
	DeviceType         string                              `json:"deviceType,omitempty"`
	ValueEx            ValueExchange                       `json:"valueExchange,omitempty"`
	DataVerify         DataVerification                    `json:"dataVerification,omitempty"`
	ProposalReject     ProposalRejection                   `json:"proposalRejection,omitempty"`
	MaxAgreements      int                                 `json:"maxAgreements,omitempty"`
	Properties         externalpolicy.PropertyList         `json:"properties,omitempty"`       // Version 2.0
	Constraints        externalpolicy.ConstraintExpression `json:"constraints,omitempty"`      // Version 2.0
	RequiredWorkload   string                              `json:"requiredWorkload,omitempty"` // Version 2.0
	HAGroup            HighAvailabilityGroup               `json:"ha_group,omitempty"`         // Version 2.0
	NodeH              NodeHealth                          `json:"nodeHealth,omitempty"`       // Version 2.0
	UserInput          []UserInput                         `json:"userInput,omitempty"`
}

// These functions are used to create Policy objects. You can create the base object
// and add features to it using the other functions
func Policy_Factory(name string) *Policy {
	p := new(Policy)
	p.Header.Name = name
	p.Header.Version = CurrentVersion

	return p
}

func (self *Policy) DeepCopy() *Policy {
	newPolicy := Policy_Factory(self.Header.Name)
	newPolicy.PatternId = self.PatternId
	newPolicy.APISpecs = make([]APISpecification, len(self.APISpecs))
	copy(newPolicy.APISpecs, self.APISpecs)

	newPolicy.AgreementProtocols = make([]AgreementProtocol, len(self.AgreementProtocols))
	copy(newPolicy.AgreementProtocols, self.AgreementProtocols)

	newPolicy.Workloads = make([]Workload, len(self.Workloads))
	copy(newPolicy.Workloads, self.Workloads)

	newPolicy.DeviceType = self.DeviceType
	newPolicy.ValueEx = self.ValueEx
	newPolicy.DataVerify = self.DataVerify
	newPolicy.ProposalReject = self.ProposalReject
	newPolicy.MaxAgreements = self.MaxAgreements

	newPolicy.Properties = make([]externalpolicy.Property, len(self.Properties))
	copy(newPolicy.Properties, self.Properties)

	newPolicy.Constraints = make([]string, len(self.Constraints))
	copy(newPolicy.Constraints, self.Constraints)

	newPolicy.RequiredWorkload = self.RequiredWorkload

	newPolicy.HAGroup = HighAvailabilityGroup{Partners: make([]string, len(self.HAGroup.Partners))}
	copy(newPolicy.HAGroup.Partners, self.HAGroup.Partners)
	newPolicy.NodeH = self.NodeH

	for _, ui := range self.UserInput {
		newUI := ui
		newUI.Inputs = make([]Input, len(ui.Inputs))
		copy(newUI.Inputs, ui.Inputs)
		newPolicy.UserInput = append(newPolicy.UserInput, newUI)
	}

	return newPolicy
}

func (self *Policy) Add_API_Spec(spec *APISpecification) error {
	if spec != nil {
		return self.APISpecs.Add_API_Spec(spec)
	} else {
		return errors.New(fmt.Sprintf("Add_API_Spec Error: input API Spec is nil."))
	}
}

func (self *Policy) Add_Agreement_Protocol(ap *AgreementProtocol) error {
	if ap != nil {
		return self.AgreementProtocols.Add_Agreement_Protocol(ap)
	} else {
		return errors.New(fmt.Sprintf("Add_Agreement_Protocol Error: input AgreementProtocol is nil."))
	}
}

func (self *Policy) Add_Property(p *externalpolicy.Property, replaceExisting bool) error {
	if p != nil {
		return self.Properties.Add_Property(p, replaceExisting)
	} else {
		return errors.New(fmt.Sprintf("Add_Property Error: input Property is nil."))
	}
}

func (self *Policy) Add_HAGroup(g *HighAvailabilityGroup) error {
	if g != nil {
		self.HAGroup = *g
		return nil
	} else {
		return errors.New(fmt.Sprintf("Add_HAGroup Error: input Group is nil."))
	}
}

func (self *Policy) Add_DataVerification(d *DataVerification) error {
	if d != nil {
		self.DataVerify = *d
		return nil
	} else {
		return errors.New(fmt.Sprintf("Add_DataVerification Error: input is nil."))
	}
}

func (self *Policy) Add_Constraints(c *externalpolicy.ConstraintExpression) error {
	if c != nil {
		if _, err := c.Validate(); err != nil {
			return errors.New(fmt.Sprintf("Add_Constraints validation error. %v.", err))
		} else {
			self.Constraints = *c
			return nil
		}
	} else {
		return errors.New(fmt.Sprintf("Add_Constraints Error: input is nil."))
	}
}

func (self *Policy) Add_Workload(w *Workload) error {
	if w != nil {
		return self.Workloads.Add_Workload(w)
	} else {
		return errors.New(fmt.Sprintf("Add_Workload Error: input is nil."))
	}
}

func (self *Policy) Add_NodeHealth(nh *NodeHealth) error {
	if nh != nil {
		self.NodeH = *nh
		return nil
	} else {
		return errors.New(fmt.Sprintf("Add_NodeHealth Error: input is nil."))
	}
}

// This is a function that compares two in-memory Policy objects to determine if they are compatible
// or not. If no error is returned, then the policies are compatible. The order of parameters is
// important. The first policy is the policy of the device that is offering itself for usage (aka
// the Producer's policy), the second parameter is the Consumer's policy that is trying to agree to
// the offer from the Producer. The data in the policy will be interpreted accordingly, as follows:
//
// Compatibility is defined as; a Consumer can make an agreement with a Producer when:
// 1) the Producer supports all the APISpecs that the Consumer's workload requires. Thus, the APISpecs
//    supported by the Producer MUST be a superset (or equal) of the API Specs that the workload requires.
// 2) the Producer advertises all the properties that the Consumer requires, and the Consumer
//    advertises all the properties that the Producer requires.
// 3) the Producer is offering enough resources for the Consumer's workload.
//

func Are_Compatible(producer_policy *Policy, consumer_policy *Policy, msgPrinter *message.Printer) *PolicyCompError {

	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	if !consumer_policy.Is_Version(producer_policy.Header.Version) {
		full_err := errors.New(msgPrinter.Sprintf("Compatibility Error: Schema versions are not the same, Consumer policy: %v, Producer policy %v", consumer_policy.Header.Version, producer_policy.Header.Version))
		return NewPolicyCompError1(full_err)
	} else if err := (&consumer_policy.Constraints).IsSatisfiedBy(producer_policy.Properties); err != nil {
		full_err := errors.New(msgPrinter.Sprintf("Compatibility Error: Node properties %v do not satisfy constraint requirements %v. Underlying error: %v", producer_policy.Properties, consumer_policy.Constraints, err))
		short_err_str := msgPrinter.Sprintf("Compatibility Error: Node properties do not satisfy constraint requirements. %v", err)
		return NewPolicyCompError(full_err, short_err_str)
	} else if err := (&producer_policy.Constraints).IsSatisfiedBy(consumer_policy.Properties); err != nil {
		full_err := errors.New(msgPrinter.Sprintf("Compatibility Error: Properties %v do not satisfy Node constraint  %v. Underlying error: %v", consumer_policy.Properties, producer_policy.Constraints, err))
		short_err_str := msgPrinter.Sprintf("Compatibility Error: Properties do not satisfy node constraint. %v", err)
		return NewPolicyCompError(full_err, short_err_str)
	} else if _, err := (&producer_policy.AgreementProtocols).Intersects_With(&consumer_policy.AgreementProtocols); err != nil {
		full_err := errors.New(msgPrinter.Sprintf("Compatibility Error: No common Agreement Protocols between %v and %v. Underlying error: %v", producer_policy.AgreementProtocols, consumer_policy.AgreementProtocols, err))
		return NewPolicyCompError1(full_err)
	} else if !producer_policy.DataVerify.IsCompatibleWith(consumer_policy.DataVerify) {
		full_err := errors.New(msgPrinter.Sprintf("Compatibility Error: Data verification must be compatible, producer has %v and consumer has %v.", producer_policy.DataVerify, consumer_policy.DataVerify))
		return NewPolicyCompError1(full_err)
	}

	return nil
}

// This function will select an agreement protocol to pursue based on the input policies. This function
// assumes that the input policies are compatible.
func Select_Protocol(producer_policy *Policy, consumer_policy *Policy) string {
	agpList, _ := (&producer_policy.AgreementProtocols).Intersects_With(&consumer_policy.AgreementProtocols)

	return (*agpList.Single_Element())[0].Name
}

// This function is used to check if 2 producer policies are compatible with each other. This is the means
// by which an agbot can make an agreement with a device that utilizes more than one API spec in the contract.
// Producers advertise API spec availability individually in their policy files. An agbot that wants to
// consume 2 API specs (microservices) in the same agreement should verify that the producer policies are
// compatible with each other before attempting a compatibility check with it's own policy file.
// If the policies are found to be compatible a merged policy will be returned. If the policies are not
// compatible then an error will be returned.
func Are_Compatible_Producers(producer_policy1 *Policy, producer_policy2 *Policy, defaultNoData uint64) (*Policy, error) {

	if producer_policy1 == nil {
		return producer_policy2, nil
	} else if !producer_policy1.Is_Version(producer_policy2.Header.Version) {
		return nil, errors.New(fmt.Sprintf("Compatibility Error: Schema versions are not the same, Policy1: %v, Policy2 %v", producer_policy1.Header.Version, producer_policy2.Header.Version))
	} else if _, err := (&producer_policy1.AgreementProtocols).Intersects_With(&producer_policy2.AgreementProtocols); err != nil {
		return nil, errors.New(fmt.Sprintf("Compatibility Error: No common Agreement Protocols between %v and %v. Underlying error: %v", producer_policy1.AgreementProtocols, producer_policy2.AgreementProtocols, err))
	} else if err := (&producer_policy1.Properties).Compatible_With(&producer_policy2.Properties, true); err != nil {
		return nil, errors.New(fmt.Sprintf("Compatibility Error: Common Properties between %v and %v. Underlying error: %v", producer_policy1.Properties, producer_policy2.Properties, err))
	} else if !producer_policy1.DataVerify.IsProducerCompatible(producer_policy2.DataVerify) {
		return nil, errors.New(fmt.Sprintf("Compatibility Error: Data verification must be compatible between %v and %v.", producer_policy1.DataVerify, producer_policy2.DataVerify))
	} else if !producer_policy1.HAGroup.Compatible_With(&producer_policy2.HAGroup) {
		return nil, errors.New(fmt.Sprintf("Compatibility Error: HAGroups must be compatible between %v and %v.", producer_policy1.HAGroup, producer_policy2.HAGroup))
	}

	merged_pol := new(Policy)
	merged_pol.Header.Name = producer_policy1.Header.Name + " merged with " + producer_policy2.Header.Name
	merged_pol.Header.Version = CurrentVersion
	merged_pol.APISpecs = (&producer_policy1.APISpecs).MergeWith(&producer_policy2.APISpecs)
	intersecting_agreement_protocols, _ := (&producer_policy1.AgreementProtocols).Intersects_With(&producer_policy2.AgreementProtocols)
	(&merged_pol.AgreementProtocols).Concatenate(intersecting_agreement_protocols)
	(&merged_pol.Properties).MergeWith(&producer_policy1.Properties, true)
	(&merged_pol.Properties).MergeWith(&producer_policy2.Properties, true)
	merged_pol.DataVerify = producer_policy1.DataVerify.ProducerMergeWith(producer_policy2.DataVerify, defaultNoData)

	// Merge constraints
	// TODO: implement comparison logic.
	// For now we will take the cowards way out and simply AND together the contraint expressions
	// from both policies.
	(&merged_pol.Constraints).MergeWith(&producer_policy1.Constraints)
	(&merged_pol.Constraints).MergeWith(&producer_policy2.Constraints)

	merged_pol.HAGroup = *((&producer_policy1.HAGroup).Merge(&producer_policy2.HAGroup))
	merged_pol.MaxAgreements = cutil.Min(producer_policy1.MaxAgreements, producer_policy2.MaxAgreements)

	if producer_policy1.UserInput != nil && len(producer_policy1.UserInput) != 0 {
		merged_pol.UserInput = make([]UserInput, len(producer_policy1.UserInput))
		copy(merged_pol.UserInput, producer_policy1.UserInput)
	}

	return merged_pol, nil
}

// This function creates a merged policy file from a producer policy and a consumer policy, which will eventually
// become the full terms and conditions of an agreement. If no error is returned, a merged policy object is returned.
// The order of parameters is important, just like in the Are_Compatible API.
func Create_Terms_And_Conditions(producer_policy *Policy, consumer_policy *Policy, workload *Workload, agreementId string, defaultPW string, defaultNoData uint64, agreementProtocolVersion int) (*Policy, error) {

	// Make sure the policies are compatible. If not an error will be returned.
	if err := Are_Compatible(producer_policy, consumer_policy, nil); err != nil {
		return nil, err
	} else {
		// Start making a new merged policy
		merged_pol := new(Policy)
		merged_pol.Header.Name = producer_policy.Header.Name + " merged with " + consumer_policy.Header.Name
		merged_pol.Header.Version = CurrentVersion

		// Propagate the pattern id
		merged_pol.PatternId = consumer_policy.PatternId

		// The consumer policy object has already been augmented with the microservices from the producer
		merged_pol.APISpecs = append(merged_pol.APISpecs, consumer_policy.APISpecs...)

		intersecting_agreement_protocols, _ := (&producer_policy.AgreementProtocols).Intersects_With(&consumer_policy.AgreementProtocols)
		agps := *intersecting_agreement_protocols.Single_Element()
		agps[0].ProtocolVersion = agreementProtocolVersion
		merged_pol.AgreementProtocols = agps
		merged_pol.Workloads = append(merged_pol.Workloads, *workload)
		if err := merged_pol.ObscureWorkloadPWs(agreementId, defaultPW); err != nil {
			return nil, errors.New(fmt.Sprintf("Error merging policies, error: %v", err))
		}
		merged_pol.ValueEx = consumer_policy.ValueEx
		merged_pol.DataVerify = producer_policy.DataVerify.MergeWith(consumer_policy.DataVerify, defaultNoData)

		// The properties from the consumer are provided, indicating that some or all of them meet
		// the constraints of the node.
		(&merged_pol.Properties).MergeWith(&consumer_policy.Properties, false)
		//(&merged_pol.Properties).Concatenate(&producer_policy.Properties)

		// The consumer's constraints are included indicating the requirements which are being met
		// by the producer policy (which is also included in the proposal).
		merged_pol.Constraints = consumer_policy.Constraints

		// Merge the remaining policy.
		// TODO: Can we get rid of any of these?
		merged_pol.RequiredWorkload = producer_policy.RequiredWorkload
		merged_pol.HAGroup = producer_policy.HAGroup
		merged_pol.NodeH = consumer_policy.NodeH

		// the user input is contained in pattern and business policy. they are consumer policies.
		if consumer_policy.UserInput != nil && len(consumer_policy.UserInput) != 0 {
			merged_pol.UserInput = make([]UserInput, len(consumer_policy.UserInput))
			copy(merged_pol.UserInput, consumer_policy.UserInput)
		}

		return merged_pol, nil
	}
}

// Merge a external policy into a policy
func MergePolicyWithExternalPolicy(pol *Policy, extPol *externalpolicy.ExternalPolicy) (*Policy, error) {
	if pol == nil {
		return nil, nil
	} else if extPol == nil {
		return pol, nil
	} else {
		// make a copy of the given policy
		merged_pol := Policy(*pol)
		if pol.UserInput != nil && len(pol.UserInput) != 0 {
			merged_pol.UserInput = make([]UserInput, len(pol.UserInput))
			copy(merged_pol.UserInput, pol.UserInput)
		}

		merged_pol.Properties.MergeWith(&(extPol.Properties), false)
		merged_pol.Constraints.MergeWith(&(extPol.Constraints))
		return &merged_pol, nil
	}
}

func (self *Policy) Is_Self_Consistent(keyFileNames []string,
	workloadOrServiceResolver func(wURL string, wOrg string, wVersion string, wArch string) (*APISpecList, error)) error {

	// Check validity of the Data verification section
	if ok, err := self.DataVerify.IsValid(); !ok {
		return errors.New(fmt.Sprintf("Data Verification section is not valid, error: %v", err))
	}

	// Check validity of the agreement protocol list
	for _, agp := range self.AgreementProtocols {
		if err := agp.IsValid(); err != nil {
			return errors.New(fmt.Sprintf("AgreementProtocol section of %v has error %v", self.Header.Name, err))
		}
	}

	// Check validity of the Workload section
	usedPriorities := make(map[int]bool)
	var referencedApiSpecRefs *APISpecList
	for ix, workload := range self.Workloads {
		if keyFileNames != nil {
			if err := workload.HasValidSignature(keyFileNames); err != nil {
				return err
			}
		}
		if len(self.Workloads) > 1 {
			if workload.Priority.PriorityValue == 0 {
				return errors.New(fmt.Sprintf("Missing required workload priority definition when there is more than 1 workload definition: %v", self.Workloads))
			} else if _, ok := usedPriorities[workload.Priority.PriorityValue]; ok {
				return errors.New(fmt.Sprintf("Duplicate workload priority value %v", workload))
			} else {
				usedPriorities[workload.Priority.PriorityValue] = true
			}
		}

		// If the policy file form is inconsistent, return the error
		if (self.Workloads[0].WorkloadURL == "" && workload.WorkloadURL != "") || (self.Workloads[0].WorkloadURL != "" && workload.WorkloadURL == "") {
			return errors.New(fmt.Sprintf("Workload section has mix of policy forms, element 0 has workloadUrl %v, element %v has %v", self.Workloads[0].WorkloadURL, ix, workload.WorkloadURL))
		}

		if (workload.Org != "" && workload.WorkloadURL == "") || (workload.Org == "" && workload.WorkloadURL != "") {
			return errors.New(fmt.Sprintf("Workload section has mix of policy forms, org is %v and workload URL is %v. Both must be specified or neither.", workload.Org, workload.WorkloadURL))
		}

		// If the policy file workloads dont reference the same org, return the error
		if self.Workloads[0].Org != workload.Org {
			return errors.New(fmt.Sprintf("Workload section has mix of organizations, element 0 has org %v, element %v has %v", self.Workloads[0].Org, ix, workload.Org))
		}

		// If the workloads use different API specs, return the error. API specs can differ by version from one workload to
		// another but they cant differ by architecture, nor can one workload require an API spec that is not required
		// by another workload in this policy file.
		if workloadOrServiceResolver != nil && workload.WorkloadURL != "" && workload.Deployment == "" && workload.ClusterDeployment == "" {
			if ix == 0 {
				if firstASRL, err := workloadOrServiceResolver(workload.WorkloadURL, workload.Org, workload.Version, workload.Arch); err == nil {
					referencedApiSpecRefs = firstASRL
				} else {
					return errors.New(fmt.Sprintf("Workload %v does not resolve, error: %v", workload, err))
				}
				// If the policy is not pattern based then it is a policy file that places workloads on nodes solely based on services that the node owner
				// has opted in to. In this case, the agreement services must have dependencies in order for them to be placed on a node. This is the not
				// the case for patterns (i.e. patterns can place agreement services on nodes where the agreement service has no dependencies).
				if self.PatternId == "" && (referencedApiSpecRefs == nil || len(*referencedApiSpecRefs) == 0) {
					return errors.New(fmt.Sprintf("Agreement services in non-pattern policies must have service dependencies, policy: %v", self.Header.Name))
				}

			} else {
				secondASRL, err := workloadOrServiceResolver(workload.WorkloadURL, workload.Org, workload.Version, workload.Arch)
				if err != nil {
					return errors.New(fmt.Sprintf("Workload %v does not resolve, error: %v", workload, err))
				}
				if !(*referencedApiSpecRefs).IsSame(*secondASRL, false) {
					return errors.New(fmt.Sprintf("Workload section has workloads that use different API specs %v and %v", *referencedApiSpecRefs, *secondASRL))
				}
			}

		}

	}

	return nil
}

// These are getter functions used to query attributes of a policy object
func (self *Policy) Get_DataVerification_enabled() bool {
	return self.DataVerify.Enabled
}

func (self *Policy) Is_Version(v string) bool {
	return self.Header.Version == v
}

func (self *Policy) String() string {
	res := ""
	res += fmt.Sprintf("Name: %v Version: %v, Pattern: %v\n", self.Header.Name, self.Header.Version, self.PatternId)
	res += "API Specifications\n"
	for _, apiSpec := range self.APISpecs {
		res += fmt.Sprintf("Ref: %v Org: %v Version: %v Exclusive: %v Arch: %v\n", apiSpec.SpecRef, apiSpec.Org, apiSpec.Version, apiSpec.ExclusiveAccess, apiSpec.Arch)
	}
	res += fmt.Sprintf("Agreement Protocol: %v\n", self.AgreementProtocols)
	res += "Workloads:\n"
	for _, wl := range self.Workloads {
		res += wl.ShortString() + "\n"
	}
	res += "Properties:\n"
	for _, p := range self.Properties {
		res += fmt.Sprintf("Name: %v Value: %v\n", p.Name, p.Value)
	}
	res += fmt.Sprintf("Constraints: %v\n", self.Constraints)
	res += fmt.Sprintf("Data Verification: %v\n", self.DataVerify)
	res += fmt.Sprintf("Node Health: %v\n", self.NodeH)

	return res
}

func (self *Policy) ShortString() string {
	res := ""
	res += fmt.Sprintf("Name: %v Version: %v, Pattern: %v", self.Header.Name, self.Header.Version, self.PatternId)
	res += ", Workloads: "
	for _, wl := range self.Workloads {
		res += wl.ShortString() + "\n"
	}

	propString := ""
	if self.Properties != nil {
		propString = self.Properties.ShortString()
	}
	res += fmt.Sprintf("Properties: %v\n", propString)
	res += fmt.Sprintf("Constraints: %v\n", self.Constraints)

	return res
}

func (self *Policy) IsSameWorkload(compare *Policy) bool {
	for _, wl := range self.Workloads {
		found := false
		for _, compareWL := range compare.Workloads {
			if wl.IsSame(compareWL) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (self *Policy) ObscureWorkloadPWs(agreementId string, defaultPW string) error {
	for ix, _ := range self.Workloads {
		if err := (&self.Workloads[ix]).Obscure(agreementId, defaultPW); err != nil {
			return err
		}
	}
	return nil
}

// Returns the next highest priority workload given a starting priority value, the number of retries so far and the
// starting time of the first try at this priority. If the caller passes in zero for the priority, then this routine will return
// the absolute highest priority workload. If there is no next highest priority, this function will return the lowest
// priority workload.
//
// Assumptions:
// (a) 1 is a higher priority workload than any priority value greater than 1, etc.
// (b) workload priorities dont have to be in order in the workload array.
// (c) workload priorities dont have to be sequential, i.e. you can have priority 5, 10 and 45.
// (d) there are no duplicate priority values in the array. This condition is checked by the Is_Self_Consistent() function
//     which is called by the agbot when it initializes and reads in policy files.
//
func (self *Policy) NextHighestPriorityWorkload(currentPriority int, retryCount int, retryStartTime uint64) *Workload {

	glog.V(3).Infof("Checking for next higher priority workload. Starting from priority %v, with %v retries at %v", currentPriority, retryCount, retryStartTime)

	if len(self.Workloads) == 1 {
		glog.V(3).Infof("Returning the only workload choice: %v", self.Workloads[0].ShortString())
		return &self.Workloads[0]
	} else {
		smallestDelta := 0
		foundSomething := false
		now := uint64(time.Now().Unix())
		nextWorkload := 0
		lowestPriorityWorkload := 0

		// Loop through each workload array element. The possible outcomes are:
		// (a) Pick the highest priority workload when the input currentPriority == 0
		// (b) Stay with the input priority workload because it hasnt used up its retries within the specified time limit
		// (c) Choose the next highest priority workload, which numerically is the next higher priority
		// (d) Choose the lowest priority workload because there are no other choices
		for ix, wl := range self.Workloads {

			// If/when we find the workload entry at the same priority as the input priority, check to see if the
			// retry count and time since first try indicate that we should stay with the current priority workload.
			if wl.Priority.PriorityValue == currentPriority {
				if wl.Priority.Retries > retryCount || now-retryStartTime > uint64(wl.Priority.RetryDurationS) {
					nextWorkload = ix
					foundSomething = true
					break
				}
				glog.V(5).Infof("Workload: %v has reached retry limit %v retries in %v seconds", self.Workloads[ix], wl.Priority.Retries, wl.Priority.RetryDurationS)
				lowestPriorityWorkload = ix
			}

			// Skip over workloads whose priority is higher (numerically less than) the input current priority
			if wl.Priority.PriorityValue <= currentPriority {
				continue
			}

			// If this workload is possibly the next highest priority workload, then remember it.
			if smallestDelta == 0 || wl.Priority.PriorityValue-currentPriority < smallestDelta {
				smallestDelta = wl.Priority.PriorityValue - currentPriority
				nextWorkload = ix
				foundSomething = true
				glog.V(5).Infof("Found new candidate workload: %v", self.Workloads[nextWorkload])
			}
		}

		// We might not find the next highest priority workload because the lowest priority workload might be
		// our only choice, even if it has exceeded its retry count.
		if !foundSomething {
			glog.V(3).Infof("Returning lowest priority workload choice: %v", self.Workloads[lowestPriorityWorkload].ShortString())
			return &self.Workloads[lowestPriorityWorkload]
		} else {
			glog.V(3).Infof("Returning workload choice: %v", self.Workloads[nextWorkload].ShortString())
			return &self.Workloads[nextWorkload]
		}
	}
}

func (p *Policy) MinimumProtocolVersion(name string, other *Policy, maxSupportedVersion int) int {
	pv := maxSupportedVersion
	if prodAGP := p.AgreementProtocols.FindByName(name); prodAGP == nil { // This should never happen
		return pv
	} else if conAGP := other.AgreementProtocols.FindByName(name); conAGP == nil { // This should never happen
		return pv
	} else {
		return prodAGP.MinimumProtocolVersion(conAGP, maxSupportedVersion)
	}

}

func (p *Policy) RequiresKnownBC(protocol string) (string, string, string) {
	if prodAGP := p.AgreementProtocols.FindByName(protocol); prodAGP == nil {
		return "", "", ""
	} else if len(prodAGP.Blockchains) != 0 {
		return prodAGP.Blockchains[0].Type, prodAGP.Blockchains[0].Name, prodAGP.Blockchains[0].Org
	}
	return "", "", ""
}

// convert the arch in the specref into GOARCH if it is a synonym of a GOARCH.
// the synonyms are defined in the configuration file
func (p *Policy) ConvertSpecRefArchToGOARCH(arch_synonymns config.ArchSynonyms) {
	if p.APISpecs != nil {
		for i := 0; i < len(p.APISpecs); i++ {
			api_spec := &p.APISpecs[i]
			if api_spec.Arch != "" && arch_synonymns.GetCanonicalArch(api_spec.Arch) != "" {
				api_spec.Arch = arch_synonymns.GetCanonicalArch(api_spec.Arch)
			}
		}
	}
}

// These are functions that operate on policy files in the file system.
//
// This function reads a file and demarshals it into a Policy struct, which is returned to
// the caller.
func ReadPolicyFile(name string, arch_synonymns config.ArchSynonyms) (*Policy, error) {

	if policyFile, err := os.Open(name); err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to open policy file %v, error: %v", name, err))
	} else if bytes, err := ioutil.ReadAll(policyFile); err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to read policy file %v, error: %v", name, err))
	} else {
		newPolicy := new(Policy)
		if err := json.Unmarshal(bytes, newPolicy); err != nil {
			return nil, errors.New(fmt.Sprintf("Unable to demarshal policy file %v, error: %v", name, err))
		} else {
			newPolicy.ConvertSpecRefArchToGOARCH(arch_synonymns)

			return newPolicy, nil
		}
	}
}

// This function writes a Policy object into a file. Note that the file is written formatted so
// that it is human readable.
func WritePolicyFile(newPolicy *Policy, name string) error {

	if bytes, err := json.MarshalIndent(newPolicy, "", "    "); err != nil {
		return errors.New(fmt.Sprintf("Unable to marshal policy %v to file, error: %v", newPolicy, err))
	} else if err := ioutil.WriteFile(name, bytes, 0644); err != nil {
		return errors.New(fmt.Sprintf("Unable to write policy file %v, error: %v", name, err))
	} else {
		return nil
	}
}

// This function deletes all the policy files for the given pattern of the given org.
func DeletePolicyFilesForPattern(policyPath string, org string, pattern string) error {

	// get all the policy files from the policy path and delete them
	orgPath := policyPath + "/" + org + "/"

	if _, err := os.Stat(orgPath); os.IsNotExist(err) {
		glog.Infof("The directory %v does not exist, do nothing.", orgPath)
		return nil
	}

	files, err := getPolicyFiles(orgPath)
	if err != nil {
		return fmt.Errorf("Unable to get list of policy files in %v, error: %v", orgPath, err)
	}

	// For each policy, if it is for this pattern, delete it.
	p_id := fmt.Sprintf("%v/%v", org, pattern)
	for _, fileInfo := range files {
		if policy, err := ReadPolicyFile(orgPath+fileInfo.Name(), config.NewArchSynonyms()); err != nil {
			return fmt.Errorf("Failed to read file %v, error: %v", orgPath+fileInfo.Name(), err)
		} else if policy.PatternId != "" && policy.PatternId == p_id {
			if err := DeletePolicyFile(orgPath + fileInfo.Name()); err != nil {
				return err
			}
		}
	}

	return nil
}

// This function deletes all the policy files for the given org.
// If patternBasedOnly is false, it deletes all policy file under the path.
// If patternBasedOnly is true, it only deletes the policy files that are pattern based.
func DeletePolicyFilesForOrg(policyPath string, org string, patternBasedOnly bool) error {

	// get all the policy files from the policy path and delete them
	orgPath := policyPath + "/" + org + "/"

	if _, err := os.Stat(orgPath); os.IsNotExist(err) {
		glog.Infof("The directory %v does not exist, do nothing.", orgPath)
		return nil
	}

	files, err := getPolicyFiles(orgPath)
	if err != nil {
		return fmt.Errorf("pattern manager unable to get list of policy files in %v, error: %v", orgPath, err)
	}

	// For each policy, delete it according to the patternBasedOnly setting
	for _, fileInfo := range files {

		if !patternBasedOnly {
			// just delete it
			if err := DeletePolicyFile(orgPath + fileInfo.Name()); err != nil {
				return err
			}
		} else if policy, err := ReadPolicyFile(orgPath+fileInfo.Name(), config.NewArchSynonyms()); err != nil {
			// this file could have error, just delete it
			glog.Errorf("Failed to read file %v, error: %v", orgPath+fileInfo.Name(), err)
			if err := DeletePolicyFile(orgPath + fileInfo.Name()); err != nil {
				return err
			}
		} else if policy.PatternId != "" {
			if err := DeletePolicyFile(orgPath + fileInfo.Name()); err != nil {
				return err
			}
		}
	}

	return nil
}

// The next section provides a function that can be used to dynamically discover the addition or removal
// of policy files from the system. It maintains a map of WatchEntry objects that represent every file in
// the policy file directory (from the config). The Watcher function calls back to inform the invoker of
// these events.

type WatchEntry struct {
	FInfo os.FileInfo
	Pol   *Policy
}

func newWatchEntry(fi os.FileInfo, p *Policy) *WatchEntry {
	return &WatchEntry{FInfo: fi, Pol: p}
}

func (w *WatchEntry) String() string {
	return fmt.Sprintf("Watch Entry, Filename: %v Policy Name: %v ", w.FInfo.Name(), w.Pol.Header.Name)
}

type Contents struct {
	AllWatches map[string]map[string]*WatchEntry
}

func NewContents() *Contents {
	return &Contents{
		AllWatches: make(map[string]map[string]*WatchEntry),
	}
}

func (c *Contents) String() string {
	res := "Policy Watch Contents: "
	for org, orgMap := range c.AllWatches {
		res += fmt.Sprintf("Org: %v ", org)
		for _, we := range orgMap {
			res += fmt.Sprintf("%v", we)
		}
	}
	return res
}

func (c *Contents) HasOrg(org string) bool {
	if _, ok := c.AllWatches[org]; !ok {
		return false
	}
	return true
}

func (c *Contents) HasFile(org string, filename string) bool {
	if !c.HasOrg(org) {
		return false
	} else if _, ok := c.AllWatches[org][filename]; !ok {
		return false
	}
	return true
}

func (c *Contents) AddWatchEntry(org string, fInfo os.FileInfo, pol *Policy) {
	if !c.HasOrg(org) {
		c.AllWatches[org] = make(map[string]*WatchEntry)
	}
	c.AllWatches[org][fInfo.Name()] = newWatchEntry(fInfo, pol)
}

func (c *Contents) UpdateWatchEntry(org string, fInfo os.FileInfo, pol *Policy) {
	if c.HasFile(org, fInfo.Name()) {
		c.AllWatches[org][fInfo.Name()] = newWatchEntry(fInfo, pol)
	}
}

func (c *Contents) RemoveWatchEntry(org string, filename string) {
	if c.HasFile(org, filename) {
		delete(c.AllWatches[org], filename)
	}
}

func (c *Contents) GetPolicyName(org, filename string) string {
	if c.HasFile(org, filename) {
		return c.AllWatches[org][filename].Pol.Header.Name
	}
	return ""
}

// check if there is already a tracked policy file with a policy name that is the same as the new policy that we're trying to add.
// It returns the name of the conflict file.
func (c *Contents) ConflictsWithAlreadyTracked(org string, pol *Policy) string {
	if !c.HasOrg(org) {
		return ""
	} else {
		for fn, we := range c.AllWatches[org] {
			if we.Pol.Header.Name == pol.Header.Name {
				return fn
			}
		}

	}
	return ""
}

func CreatePolicyFile(filepath string, org string, name string, p *Policy) (string, error) {

	// Store the policy on the filesystem in an org based hierarchy
	fullFilePath := fmt.Sprintf("%v%v/", filepath, org)
	fullFileName := fmt.Sprintf("%v%v.policy", fullFilePath, name)
	if err := os.MkdirAll(fullFilePath, 0764); err != nil {
		return "", errors.New(fmt.Sprintf("Error writing policy file, cannot create file path %v", fullFilePath))
	} else if err := WritePolicyFile(p, fullFileName); err != nil {
		return "", errors.New(fmt.Sprintf("Error writing out policy file %v, to %v, error: %v", *p, fullFileName, err))
	}
	return fullFileName, nil

}

func RenamePolicyFile(filepath string, org string, name string, newSuffix string) error {

	fullFilePath := fmt.Sprintf("%v%v/", filepath, org)
	fullFileName := fmt.Sprintf("%v%v.policy", fullFilePath, name)
	if err := os.Rename(fullFileName, fullFileName+newSuffix); err != nil {
		return fmt.Errorf("Failed to rename the policy file %v to %v, error %v", fullFileName, fullFileName+newSuffix, err)
	}
	return nil

}

func DeletePolicyFile(name string) error {
	if err := os.Remove(name); err != nil {
		return fmt.Errorf("Failed to remove the policy file %v, error %v", name, err)
	}
	return nil
}

// This is the policy file watcher function. It can be called once, to be notified of all policy files
// when the system starts up. Or, it can be dispatched as a go routine that wakes up on the invoker's
// interval to check for changes in the policy directory. The directory is subdivided into organizations
// by directories with the organization name. In each of these directories is where the policy files
// can be found.
//
// Humans are devious and can make all kinds of different changes to the policy files. Changes like renaming
// files, changing the contents of a file, changing a policy name within the file, adding new files, deleting
// files, etc. All of these potential changes need to be accounted for within this function.
//
// When the watcher observes a change in the policy directory it will call the appropriate callback function:
// - fileChanged is called when new files are added OR when an existing file is updated.
// - fileDeleted is called when a file is deleted
// - fileError is called when an error occurs trying to demarshal a file into a policy object

func PolicyFileChangeWatcher(homePath string,
	contents *Contents,
	arch_synonymns config.ArchSynonyms,
	fileChanged func(org string, fileName string, policy *Policy),
	fileDeleted func(org string, fileName string, policy *Policy),
	fileError func(org string, fileName string, err error),
	workloadOrServiceResolver func(wURL string, wOrg string, wVersion string, wArch string) (*APISpecList, error),
	checkInterval int) (*Contents, error) {

	// contents is the map that holds info on every policy file in every org in the policy directory

	// The main loop that monitors the policy directory.
	for {

		dirs, err := getPolicyDirectories(homePath)
		if err != nil {
			return contents, errors.New(fmt.Sprintf("Policy File Watcher unable to get list of policy directories in %v, error: %v", homePath, err))
		}

		// Get a list of all directories in the policy directory
		for _, dirInfo := range dirs {
			org := dirInfo.Name()
			glog.V(5).Infof("Policy File Watcher reading directory %v", dirInfo)

			// Get a list of all policy files in the directory
			orgPath := homePath + "/" + org + "/"
			files, err := getPolicyFiles(orgPath)
			if err != nil {
				return contents, errors.New(fmt.Sprintf("Policy File Watcher unable to get list of policy files in %v, error: %v", orgPath, err))
			}

			// For each file, if we dont have a record of it, read in the file and create an entry in the map.
			for _, fileInfo := range files {
				if !contents.HasFile(org, fileInfo.Name()) {
					if policy, err := ReadPolicyFile(orgPath+fileInfo.Name(), arch_synonymns); err != nil {
						fileError(org, orgPath+fileInfo.Name(), err)
					} else if err := policy.Is_Self_Consistent(nil, workloadOrServiceResolver); err != nil {
						fileError(org, orgPath+fileInfo.Name(), errors.New(fmt.Sprintf("Policy file not self consistent %v, error: %v", orgPath, err)))
					} else if fn := contents.ConflictsWithAlreadyTracked(org, policy); fn != "" {
						fileError(org, orgPath+fileInfo.Name(), errors.New(fmt.Sprintf("Policy File Watcher cannot add policy file %v/%v because it has the same policy header name with the policy file %v/%v.", org, fileInfo.Name(), org, fn)))
					} else {
						contents.AddWatchEntry(org, fileInfo, policy)
						fileChanged(org, orgPath+fileInfo.Name(), policy)
						glog.V(5).Infof("Policy File Watcher Adding file %v", orgPath+fileInfo.Name())
					}
				}
			}
		}

		// For each file that we know about (this includes any new files discovered above), check to see
		// if the file has changed or has been deleted.
		for org, orgMap := range contents.AllWatches {
			orgPath := homePath + "/" + org + "/"
			for _, we := range orgMap {
				if newStat, err := os.Stat(orgPath + we.FInfo.Name()); err != nil && !os.IsNotExist(err) {
					fileError(org, orgPath+we.FInfo.Name(), err)
				} else if err != nil && os.IsNotExist(err) {
					// A file that is deleted might actually have been renamed. To check this, we need to look at
					// all the other policies we captured to see if there is another file with our policy in it. If so,
					// we can skip the delete notification.
					found := false
					for key, val := range orgMap {
						if key == we.FInfo.Name() {
							continue
						} else if val.Pol.Header.Name == we.Pol.Header.Name {
							found = true
							break
						}
					}
					// If there is another file with our policy in it, then we can skip the delete event but we still have to
					// remove the file entry from the contents map.
					if !found {
						fileDeleted(org, orgPath+we.FInfo.Name(), we.Pol)
						glog.V(5).Infof("Policy File Watcher detected deleted file %v", orgPath+we.FInfo.Name())
					}
					contents.RemoveWatchEntry(org, we.FInfo.Name())

				} else if newStat.ModTime().After(we.FInfo.ModTime()) {
					// A changed file could be a new policy and a deleted policy if it's the policy name that was changed.
					if policy, err := ReadPolicyFile(orgPath+we.FInfo.Name(), arch_synonymns); err != nil {
						fileError(org, orgPath+we.FInfo.Name(), err)
					} else if err := policy.Is_Self_Consistent(nil, workloadOrServiceResolver); err != nil {
						fileError(org, orgPath+we.FInfo.Name(), errors.New(fmt.Sprintf("Policy file not self consistent %v, error: %v", orgPath+we.FInfo.Name(), err)))
					} else if policy.Header.Name != we.Pol.Header.Name {
						// Contents of the file changed the policy name, so this means we have a new policy and a deleted policy at the same time.
						// Inform the world about the deleted policy.
						fileDeleted(org, orgPath+we.FInfo.Name(), we.Pol)
						glog.V(5).Infof("Policy File Watcher detected deleted policy in existing file %v", orgPath+we.FInfo.Name())
						// Inform the world about the new policy and save a reference to it.
						fileChanged(org, orgPath+we.FInfo.Name(), policy)
						glog.V(5).Infof("Policy File Watcher Stats detected new policy in existing file %v", orgPath+we.FInfo.Name())
						contents.AddWatchEntry(org, newStat, policy)
					} else {
						fileChanged(org, orgPath+we.FInfo.Name(), policy)
						glog.V(5).Infof("Policy File Watcher Stats detected changed file %v", orgPath+we.FInfo.Name())
						contents.UpdateWatchEntry(org, newStat, policy)
					}
				}
			}
		}

		// Break out of the main loop if there is no check interval specified. This means that the caller
		// doesnt want us to monitor the directory.
		if checkInterval > 0 {
			time.Sleep(time.Duration(checkInterval) * time.Second)
		} else {
			break
		}
	}

	return contents, nil
}

// Delete all policy files. This function does not try to update the policy manager, it is a low level function
// that simply removes all the policy files.
func DeleteAllPolicyFiles(homePath string, patternBasedOnly bool) error {

	dirs, err := getPolicyDirectories(homePath)
	if err != nil {
		return errors.New(fmt.Sprintf("unable to get list of policy directories in %v, error: %v", homePath, err))
	}

	// Each directory can have policy files in it. On a node, there is only 1 policy directory.
	for _, dirInfo := range dirs {
		org := dirInfo.Name()
		glog.V(5).Infof("Deleting policies from directory %v", org)

		pDir := homePath + "/" + org
		if !patternBasedOnly {
			// Remove the org directory.
			if err := os.RemoveAll(pDir); err != nil {
				glog.Errorf("Error removing policy directory %v, error: %v", pDir, err)
			}
		} else {
			// remove policy files that are pattern based
			if err := DeletePolicyFilesForOrg(homePath, org, true); err != nil {
				return err
			}
		}
	}
	return nil
}

// This is an internal function used to find all policy files in the policy directory. Files that don't end in
// .policy are ignored. Directories are also ignored.
func getPolicyFiles(homePath string) ([]os.FileInfo, error) {
	res := make([]os.FileInfo, 0, 10)

	if files, err := ioutil.ReadDir(homePath); err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to get list of policy files in %v, error: %v", homePath, err))
	} else {
		for _, fileInfo := range files {
			if strings.HasSuffix(fileInfo.Name(), ".policy") && !fileInfo.IsDir() {
				res = append(res, fileInfo)
			}
		}
		return res, nil
	}
}

// This is an internal function used to find all directories in the policy directory. It only returns directories.
func getPolicyDirectories(homePath string) ([]os.FileInfo, error) {
	res := make([]os.FileInfo, 0, 10)

	if files, err := ioutil.ReadDir(homePath); err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to get list of policy directories in %v, error: %v", homePath, err))
	} else {
		for _, fileInfo := range files {
			if fileInfo.IsDir() {
				res = append(res, fileInfo)
			}
		}
		return res, nil
	}
}

// Error contains a full error string and a short error. The short error can be used for cli output.
// the full error can be used for internal logs.
// It implements the error interface
type PolicyCompError struct {
	Err      string
	ShortErr string
}

func (e *PolicyCompError) Error() string {
	if e == nil {
		return ""
	} else {
		return fmt.Sprintf("%v", e.Err)
	}
}

func (e *PolicyCompError) String() string {
	if e == nil {
		return ""
	} else {
		return e.Err
	}
}

// returns the short error.
// if the short error is empty, then return the long error.
func (e *PolicyCompError) ShortString() string {
	if e == nil {
		return ""
	} else if e.ShortErr != "" {
		return e.ShortErr
	} else {
		return e.Err
	}
}

func NewPolicyCompError(err error, shortErr string) *PolicyCompError {
	return &PolicyCompError{
		Err:      err.Error(),
		ShortErr: shortErr,
	}
}

// short error and long error is the same. we only save long error.
func NewPolicyCompError1(err error) *PolicyCompError {
	return &PolicyCompError{
		Err: err.Error(),
	}
}
