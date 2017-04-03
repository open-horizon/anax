package policy

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
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
	Type        string `json:"type"`        // The type of value exchange
	Value       string `json:"value"`       // The value being exchanged
	PaymentRate int    `json:"paymentRate"` // The number of seconds between payments
	Token       string `json:"token"`       // A token used to identify the user of the value - added in version 2
}

type ProposalRejection struct {
	Number   int `json:"number"`   // Number of rejections before giving up on making agreements
	Duration int `json:"duration"` // The length of time to wait before trying again
}

// This is the main struct that defines the Policy object
type Policy struct {
	Header                 PolicyHeader          `json:"header"`
	APISpecs               APISpecList           `json:"apiSpec"`
	AgreementProtocols     AgreementProtocolList `json:"agreementProtocols"`
	Workloads              []Workload            `json:"workloads,omitempty"`
	DeviceType             string                `json:"deviceType,omitempty"`
	ValueEx                ValueExchange         `json:"valueExchange,omitempty"`
	ResourceLimits         ResourceLimit         `json:"resourceLimits,omitempty"`
	DataVerify             DataVerification      `json:"dataVerification,omitempty"`
	ProposalReject         ProposalRejection     `json:"proposalRejection,omitempty"`
	MaxAgreements          int                   `json:"maxAgreements,omitempty"`
	Properties             PropertyList          `json:"properties,omitempty"`             // Version 2.0
	CounterPartyProperties RequiredProperty      `json:"counterPartyProperties,omitempty"` // Version 2.0
	Blockchains            BlockchainList        `json:"blockchains,omitempty"`            // Version 2.0
	RequiredWorkload       string                `json:"requiredWorkload,omitempty"`       // Version 2.0
	HAGroup                HighAvailabilityGroup `json:"ha_group,omitempty"`               // Version 2.0
}

// These functions are used to create Policy objects. You can create the base object
// and add features to it using the other functions
func Policy_Factory(name string) *Policy {
	p := new(Policy)
	p.Header.Name = name
	p.Header.Version = CurrentVersion

	return p
}

func (self *Policy) Add_API_Spec(spec *APISpecification) error {
	if spec != nil {
		return self.APISpecs.Add_API_Spec(spec)
	} else {
		return errors.New(fmt.Sprintf("Add_API_Spec Error: input API Spec is nil."))
	}
}

func (self *Policy) Add_Blockchain(bc *Blockchain) error {
	if bc != nil {
		return self.Blockchains.Add_Blockchain(bc)
	} else {
		return errors.New(fmt.Sprintf("Add_Blockchain Error: input Blockchain is nil."))
	}
}

func (self *Policy) Add_Agreement_Protocol(ap *AgreementProtocol) error {
	if ap != nil {
		return self.AgreementProtocols.Add_Agreement_Protocol(ap)
	} else {
		return errors.New(fmt.Sprintf("Add_Agreement_Protocol Error: input AgreementProtocol is nil."))
	}
}

func (self *Policy) Add_Property(p *Property) error {
	if p != nil {
		return self.Properties.Add_Property(p)
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

// This is a function that compares two in-memory Policy objects to determine if they are compatible
// or not. If no error is returned, then the policies are compatible. The order of parameters is
// important. The first policy is the policy of the device that is offering itself for usage (aka
// the Producer's policy), the second parameter is the Consumer's policy that is trying to agree to
// the offer from the Producer. The data in the policy will be interpreted accordingly, as follows:
//
// Compatibility is defined as; a Consumer can make an agreement with a Producer when:
// 1) the Producer supports all the APISpecs that the Consumer's workload requires. Thus, the APISpecs
//    supported by the Producer MUST be a superset (or equal) of the API Specs that the workload requires.
// 2) the Producer advertises all the properties that the Consumer requires, and when the Consumer
//    advertises all the properties that the Producer requires.
// 3) the Producer is offering enough resources for the Consumer's workload.
//

func Are_Compatible(producer_policy *Policy, consumer_policy *Policy) error {

	if !consumer_policy.Is_Version(producer_policy.Header.Version) {
		return errors.New(fmt.Sprintf("Compatibility Error: Schema versions are not the same, Consumer policy: %v, Producer policy %v", consumer_policy.Header.Version, producer_policy.Header.Version))
	} else if err := (&consumer_policy.APISpecs).Is_Subset_Of(&producer_policy.APISpecs); err != nil {
		return errors.New(fmt.Sprintf("Compatibility Error: Producer policy APISpecs %v do not support Consumer APISpec requirements %v. Underlying error: %v", producer_policy.APISpecs, consumer_policy.APISpecs, err))
	} else if err := (&consumer_policy.CounterPartyProperties).IsSatisfiedBy(producer_policy.Properties); err != nil {
		return errors.New(fmt.Sprintf("Compatibility Error: Producer properties %v do not satisfy Consumer property requirements %v. Underlying error: %v", producer_policy.Properties, consumer_policy.CounterPartyProperties, err))
	} else if err := (&producer_policy.CounterPartyProperties).IsSatisfiedBy(consumer_policy.Properties); err != nil {
		return errors.New(fmt.Sprintf("Compatibility Error: Consumer properties %v do not satisfy Producer property requirements %v. Underlying error: %v", consumer_policy.Properties, producer_policy.CounterPartyProperties, err))
	} else if _, err := (&producer_policy.Blockchains).Intersects_With(&consumer_policy.Blockchains); err != nil {
		return errors.New(fmt.Sprintf("Compatibility Error: Producer policy Blockchains %v are not supported by Consumer Blockchain options %v. Underlying error: %v", producer_policy.Blockchains, consumer_policy.Blockchains, err))
	} else if _, err := (&producer_policy.AgreementProtocols).Intersects_With(&consumer_policy.AgreementProtocols); err != nil {
		return errors.New(fmt.Sprintf("Compatibility Error: No common Agreement Protocols between %v and %v. Underlying error: %v", producer_policy.AgreementProtocols, consumer_policy.AgreementProtocols, err))
	} else if !(&consumer_policy.ResourceLimits).IsSatisfiedBy(&producer_policy.ResourceLimits) {
		return errors.New(fmt.Sprintf("Compatibility Error: Producer resource limits %v do not satisfy consumer resource requirements %v. Underlying error: %v", producer_policy.ResourceLimits, consumer_policy.ResourceLimits, err))
	} else if (producer_policy.DataVerify != DataVerification{}) && producer_policy.DataVerify.IsSame(consumer_policy.DataVerify) {
		return errors.New(fmt.Sprintf("Compatibility Error: Data verification must be identical or absent on one side, producer has %v and consumer has %v.", producer_policy.DataVerify, consumer_policy.DataVerify, err))
	}

	return nil
}

// This function is used to check if 2 producer policies are compatible with each other. This is the means
// by which an agbot can make an agreement with a device that utilizes more than one API spec in the contract.
// Producers advertise API spec availability individually in their policy files. An agbot that wants to
// consume 2 API specs (microservices) in the same agreement should verify that the producer policies are
// compatible with each other before attempting a compatibility check with it's own policy file.
// If the policies are found to be compatible a merged policy will be returned. If the policies are not
// compatible then an error will be returned.
func Are_Compatible_Producers(producer_policy1 *Policy, producer_policy2 *Policy) (*Policy, error) {

	if !producer_policy1.Is_Version(producer_policy2.Header.Version) {
		return nil, errors.New(fmt.Sprintf("Compatibility Error: Schema versions are not the same, Policy1: %v, Policy2 %v", producer_policy1.Header.Version, producer_policy2.Header.Version))
	} else if _, err := (&producer_policy1.Blockchains).Intersects_With(&producer_policy2.Blockchains); err != nil {
		return nil, errors.New(fmt.Sprintf("Compatibility Error: No Common Blockchains between %v and %v. Underlying error: %v", producer_policy1.Blockchains, producer_policy2.Blockchains, err))
	} else if _, err := (&producer_policy1.AgreementProtocols).Intersects_With(&producer_policy2.AgreementProtocols); err != nil {
		return nil, errors.New(fmt.Sprintf("Compatibility Error: No common Agreement Protocols between %v and %v. Underlying error: %v", producer_policy1.AgreementProtocols, producer_policy2.AgreementProtocols, err))
	} else if err := (&producer_policy1.Properties).Compatible_With(&producer_policy2.Properties); err != nil {
		return nil, errors.New(fmt.Sprintf("Compatibility Error: Common Properties between %v and %v. Underlying error: %v", producer_policy1.Properties, producer_policy2.Properties, err))
	} else if !producer_policy1.DataVerify.IsSame(producer_policy2.DataVerify) {
		return nil, errors.New(fmt.Sprintf("Compatibility Error: Data verification must be identical between %v and %v.", producer_policy1.DataVerify, producer_policy2.DataVerify, err))
	}

	merged_pol := new(Policy)
	merged_pol.Header.Name = producer_policy1.Header.Name + " merged with " + producer_policy2.Header.Name
	merged_pol.Header.Version = CurrentVersion
	(&merged_pol.APISpecs).Concatenate(&producer_policy1.APISpecs)
	(&merged_pol.APISpecs).Concatenate(&producer_policy2.APISpecs)
	intersecting_blockchains, _ := (&producer_policy1.Blockchains).Intersects_With(&producer_policy2.Blockchains)
	(&merged_pol.Blockchains).Concatenate(intersecting_blockchains)
	intersecting_agreement_protocols, _ := (&producer_policy1.AgreementProtocols).Intersects_With(&producer_policy2.AgreementProtocols)
	(&merged_pol.AgreementProtocols).Concatenate(intersecting_agreement_protocols)
	(&merged_pol.Properties).Concatenate(&producer_policy1.Properties)
	(&merged_pol.Properties).Concatenate(&producer_policy2.Properties)
	merged_pol.DataVerify = producer_policy1.DataVerify

	// Merge counterpartyProperties
	// 2 CounterPartyProperty specifications could be incompatible, and this could be detected in some cases.
	// TODO: implement comparison logic.
	// For now we will take the cowards way out and simply AND together the Counter Party Property expressions
	// from both policies.
	merged_pol.CounterPartyProperties = *((&producer_policy1.CounterPartyProperties).Merge(&producer_policy2.CounterPartyProperties))
	merged_pol.ResourceLimits = *((&producer_policy1.ResourceLimits).MergeProducers(&producer_policy2.ResourceLimits))

	return merged_pol, nil
}

// This function creates a merged policy file from a producer policy and a consumer policy, which will eventually
// become the full terms and conditions of an agreement. If no error is returned, a merged policy object is returned.
// The order of parameters is important, just like in the Are_Compatible API.
func Create_Terms_And_Conditions(producer_policy *Policy, consumer_policy *Policy, workload *Workload, agreementId string, defaultPW string) (*Policy, error) {

	// Make sure the policies are compatible. If not an error will be returned.
	if err := Are_Compatible(producer_policy, consumer_policy); err != nil {
		return nil, err
	} else {
		// Start making a new merged policy
		merged_pol := new(Policy)
		merged_pol.Header.Name = producer_policy.Header.Name + " merged with " + consumer_policy.Header.Name
		merged_pol.Header.Version = CurrentVersion
		merged_pol.APISpecs = append(merged_pol.APISpecs, consumer_policy.APISpecs...)
		intersecting_agreement_protocols, _ := (&producer_policy.AgreementProtocols).Intersects_With(&consumer_policy.AgreementProtocols)
		merged_pol.AgreementProtocols = *intersecting_agreement_protocols.Single_Element()
		merged_pol.Workloads = append(merged_pol.Workloads, *workload)
		if err := merged_pol.ObscureWorkloadPWs(agreementId, defaultPW); err != nil {
			return nil, errors.New(fmt.Sprintf("Error merging policies, error: %v", err))
		}
		merged_pol.ValueEx = consumer_policy.ValueEx
		merged_pol.ResourceLimits = consumer_policy.ResourceLimits
		merged_pol.DataVerify = consumer_policy.DataVerify
		merged_pol.DataVerify.Obscure()
		intersecting_blockchains, _ := (&producer_policy.Blockchains).Intersects_With(&consumer_policy.Blockchains)
		merged_pol.Blockchains = *intersecting_blockchains.Single_Element()
		(&merged_pol.Properties).Concatenate(&consumer_policy.Properties)
		(&merged_pol.Properties).Concatenate(&producer_policy.Properties)
		merged_pol.RequiredWorkload = producer_policy.RequiredWorkload
		merged_pol.HAGroup = producer_policy.HAGroup

		return merged_pol, nil
	}
}

func (self *Policy) Is_Self_Consistent(keyPath string) error {
	usedPriorities := make(map[int]bool)
	for _, workload := range self.Workloads {
		if len(keyPath) != 0 {
			if err := workload.HasValidSignature(keyPath); err != nil {
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
	res += fmt.Sprintf("Name: %v Version: %v\n", self.Header.Name, self.Header.Version)
	res += "API Specifications\n"
	for _, apiSpec := range self.APISpecs {
		res += fmt.Sprintf("Ref: %v Version: %v Exclusive: %v Arch: %v\n", apiSpec.SpecRef, apiSpec.Version, apiSpec.ExclusiveAccess, apiSpec.Arch)
	}
	res += fmt.Sprintf("Agreement Protocol: %v\n", self.AgreementProtocols)
	res += "Workloads:\n"
	for _, wl := range self.Workloads {
		res += fmt.Sprintf("Deployment: %v DeploymentSignature: %v DeploymentUserInfo: %v Torrent: %v\n", wl.Deployment, wl.DeploymentSignature, wl.DeploymentUserInfo, wl.Torrent)
	}
	res += "Properties:\n"
	for _, p := range self.Properties {
		res += fmt.Sprintf("Name: %v Value: %v\n", p.Name, p.Value)
	}
	res += fmt.Sprintf("Resource Limits: %v\n", self.ResourceLimits)
	res += fmt.Sprintf("Data Verification: %v\n", self.DataVerify)
	res += fmt.Sprintf("Blockchains: %v\n", self.Blockchains)

	return res
}

func (self *Policy) ShortString() string {
	res := ""
	res += fmt.Sprintf("Name: %v Version: %v", self.Header.Name, self.Header.Version)
	res += ", API Specifications "
	for _, apiSpec := range self.APISpecs {
		res += fmt.Sprintf("Ref: %v Version: %v Exclusive: %v Arch: %v", apiSpec.SpecRef, apiSpec.Version, apiSpec.ExclusiveAccess, apiSpec.Arch)
	}
	res += fmt.Sprintf(", Agreement Protocol: %v", self.AgreementProtocols)
	res += ", Workloads: "
	for _, wl := range self.Workloads {
		res += fmt.Sprintf("Deployment: %v", wl.Deployment)
	}
	res += ", Properties: "
	for _, p := range self.Properties {
		res += fmt.Sprintf("Name: %v Value: %v", p.Name, p.Value)
	}
	res += fmt.Sprintf(", Resource Limits: %v", self.ResourceLimits)
	res += fmt.Sprintf(", Data Verification: %v", self.DataVerify)

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
				if wl.Priority.Retries > retryCount || now - retryStartTime > uint64(wl.Priority.RetryDurationS) {
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
			if smallestDelta == 0 || wl.Priority.PriorityValue - currentPriority < smallestDelta {
				smallestDelta = wl.Priority.PriorityValue - currentPriority
				nextWorkload = ix
				foundSomething = true
				glog.V(5).Infof("Found new candidate workload: %v", self.Workloads[nextWorkload])
			}
		}

		// We might not find the next highest priority workload because the lowest priority workload might be
		// our only choice, even if it has exceeded its retry count.
		if !foundSomething {
			glog.V(3).Infof("Returning lowest priority workload choice: %v", self.Workloads[lowestPriorityWorkload])
			return &self.Workloads[lowestPriorityWorkload]
		} else {
			glog.V(3).Infof("Returning workload choice: %v", self.Workloads[nextWorkload])
			return &self.Workloads[nextWorkload]
		}
	}
}


// These are functions that operate on policy files in the file system.
//
// This function reads a file and demarshals it into a Policy struct, which is returned to
// the caller.
func ReadPolicyFile(name string) (*Policy, error) {

	if policyFile, err := os.Open(name); err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to open policy file %v, error: %v", name, err))
	} else if bytes, err := ioutil.ReadAll(policyFile); err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to read policy file %v, error: %v", name, err))
	} else {
		newPolicy := new(Policy)
		if err := json.Unmarshal(bytes, newPolicy); err != nil {
			return nil, errors.New(fmt.Sprintf("Unable to demarshal policy file %v, error: %v", name, err))
		} else {
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

// This is the policy file watcher function. It can be called once, to be notified of all policy files
// when the system starts up. Or, it can be dispatched as a go routine that wakes up on the invoker's
// interval to check for changes in the policy directory.
//
// When the watcher observes a change in the policy directory it will call the appropriate callback function:
// - fileChanged is called when new files are added OR when an existing file is updated.
// - fileDeleted is called when a file is deleted
// - fileError is called when an error occurs trying to demarshal a file into a policy object

func PolicyFileChangeWatcher(homePath string, fileChanged func(fileName string, policy *Policy), fileDeleted func(fileName string, policy *Policy), fileError func(fileName string, err error), checkInterval int) error {

	// The map that holds info on every policy document in the policy directory
	var contents = make(map[string]*WatchEntry)

	// The main loop that monitors the policy directory.
	for {
		// Get a list of all policy files in the directory
		if files, err := getPolicyFiles(homePath); err != nil {
			return errors.New(fmt.Sprintf("Policy File Watcher unable to get list of policy files in %v, error: %v", homePath, err))
		} else {
			// For each file, if we dont have a record of it, read in the file and create an entry in the map.
			for _, fileInfo := range files {
				if _, ok := contents[fileInfo.Name()]; !ok {
					if policy, err := ReadPolicyFile(homePath + fileInfo.Name()); err != nil {
						fileError(homePath+fileInfo.Name(), err)
					} else if err := policy.Is_Self_Consistent(""); err != nil {
						fileError(homePath+fileInfo.Name(), errors.New(fmt.Sprintf("Policy file not self consistent %v, error: %v", homePath, err)))
					} else {
						contents[fileInfo.Name()] = newWatchEntry(fileInfo, policy)
						fileChanged(homePath+fileInfo.Name(), policy)
						glog.V(5).Infof("Policy File Watcher Adding file %v", homePath+fileInfo.Name())
					}
				}
			}
		}

		// For each file that we know about (this includes any new files discovered above), check to see
		// the file has changed or has been deleted.
		for _, we := range contents {
			if newStat, err := os.Stat(homePath + we.FInfo.Name()); err != nil && !os.IsNotExist(err) {
				fileError(homePath+we.FInfo.Name(), err)
			} else if err != nil && os.IsNotExist(err) {
				fileDeleted(homePath+we.FInfo.Name(), we.Pol)
				glog.V(5).Infof("Policy File Watcher detected deleted file %v", homePath+we.FInfo.Name())
				key := we.FInfo.Name()
				delete(contents, key)
			} else if newStat.ModTime().After(we.FInfo.ModTime()) {
				fileChanged(homePath+we.FInfo.Name(), we.Pol)
				glog.V(5).Infof("Policy File Watcher Stats detected changed file %v", homePath+we.FInfo.Name())
				contents[we.FInfo.Name()] = newWatchEntry(newStat, we.Pol)
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
