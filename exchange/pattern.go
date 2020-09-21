package exchange

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/policy"
	"strings"
	"time"
)

type Pattern struct {
	Owner              string              `json:"owner"`
	Label              string              `json:"label"`
	Description        string              `json:"description"`
	Public             bool                `json:"public"`
	Services           []ServiceReference  `json:"services"`
	AgreementProtocols []AgreementProtocol `json:"agreementProtocols"`
	UserInput          []policy.UserInput  `json:"userInput,omitempty"`
}

func (w Pattern) String() string {
	return fmt.Sprintf("Owner: %v, Label: %v, Description: %v, Public: %v, Services: %v, AgreementProtocols: %v, UserInput: %v",
		w.Owner,
		w.Label,
		w.Description,
		w.Public,
		w.Services,
		w.AgreementProtocols,
		w.UserInput)
}

func (w Pattern) ShortString() string {
	// get the short string for each service version
	svc_a := make([]string, len(w.Services))
	for i, wl := range w.Services {
		svc_a[i] = wl.ShortString()
	}

	return fmt.Sprintf("Owner: %v, Label: %v, Description: %v, Public: %v, Services: %v, AgreementProtocols: %v",
		w.Owner,
		w.Label,
		w.Description,
		w.Public,
		svc_a,
		w.AgreementProtocols)
}

// return a pointer to a copy of Pattern
func (w Pattern) DeepCopy() *Pattern {
	newPattern := Pattern{Owner: w.Owner, Label: w.Label, Description: w.Description, Public: w.Public}

	if w.Services != nil {
		newServices := make([]ServiceReference, len(w.Services))
		copy(newServices, w.Services)
		newPattern.Services = newServices
	}

	if w.AgreementProtocols != nil {
		newAgPro := make([]AgreementProtocol, len(w.AgreementProtocols))
		copy(newAgPro, w.AgreementProtocols)
		newPattern.AgreementProtocols = newAgPro
	}

	if w.UserInput != nil {
		newUserInput := make([]policy.UserInput, len(w.UserInput))
		copy(newUserInput, w.UserInput)
		newPattern.UserInput = newUserInput
	}

	return &newPattern
}

type WorkloadPriority struct {
	PriorityValue     int `json:"priority_value,omitempty"`     // The priority of the workload
	Retries           int `json:"retries,omitempty"`            // The number of retries before giving up and moving to the next priority
	RetryDurationS    int `json:"retry_durations,omitempty"`    // The number of seconds in which the specified number of retries must occur in order for the next priority workload to be attempted.
	VerifiedDurationS int `json:"verified_durations,omitempty"` // The number of second in which verified data must exist before the rollback retry feature is turned off
}

type UpgradePolicy struct {
	Lifecycle string `json:"lifecycle,omitempty"` // immediate, never, agreement
	Time      string `json:"time,omitempty"`      // the time of the upgrade
}

type WorkloadChoice struct {
	Version                      string           `json:"version,omitempty"`  // the version of the workload
	Priority                     WorkloadPriority `json:"priority,omitempty"` // the highest priority workload is tried first for an agreement, if it fails, the next priority is tried. Priority 1 is the highest, priority 2 is next, etc.
	Upgrade                      UpgradePolicy    `json:"upgradePolicy,omitempty"`
	DeploymentOverrides          string           `json:"deployment_overrides"`           // env var overrides for the workload
	DeploymentOverridesSignature string           `json:"deployment_overrides_signature"` // signature of env var overrides
}

func (w WorkloadChoice) String() string {
	return fmt.Sprintf("Version: %v, Priority: %v, Upgrade: %v, DeploymentOverrides: %v, DeploymentOverridesSignature: %v",
		w.Version,
		w.Priority,
		w.Upgrade,
		w.DeploymentOverrides,
		w.DeploymentOverridesSignature)
}

func (w WorkloadChoice) ShortString() string {
	return fmt.Sprintf("Version: %v, Priority: %v, Upgrade: %v, DeploymentOverrides: %v, DeploymentOverridesSignature: %v",
		w.Version,
		w.Priority,
		w.Upgrade,
		w.DeploymentOverrides,
		cutil.TruncateDisplayString(w.DeploymentOverridesSignature, 5))
}

type ServiceReference struct {
	ServiceURL      string           `json:"serviceUrl,omitempty"`      // refers to a service definition in the exchange
	ServiceOrg      string           `json:"serviceOrgid,omitempty"`    // the org holding the service definition
	ServiceArch     string           `json:"serviceArch,omitempty"`     // the hardware architecture of the service definition
	ServiceVersions []WorkloadChoice `json:"serviceVersions,omitempty"` // a list of service version for rollback
	DataVerify      DataVerification `json:"dataVerification"`          // policy for verifying that the node is sending data
	NodeH           NodeHealth       `json:"nodeHealth"`                // policy for determining when a node's health is violating its agreements
	AgreementLess   bool             `json:"agreementLess"`             // This service should get started on the node without an agreement to start it
}

func (w ServiceReference) String() string {
	return fmt.Sprintf("ServiceURL: %v, ServiceOrg: %v, ServiceArch: %v, ServiceVersions: %v, DataVerify: %v, NodeH: %v",
		w.ServiceURL,
		w.ServiceOrg,
		w.ServiceArch,
		w.ServiceVersions,
		w.DataVerify,
		w.NodeH)
}

func (w ServiceReference) ShortString() string {
	// get the short string for each service version
	wl_a := make([]string, len(w.ServiceVersions))
	for i, wl := range w.ServiceVersions {
		wl_a[i] = wl.ShortString()
	}
	return fmt.Sprintf("ServiceURL: %v, ServiceOrg: %v, ServiceArch: %v, ServiceVersions: %v, DataVerify: %v, NodeH: %v",
		w.ServiceURL,
		w.ServiceOrg,
		w.ServiceArch,
		wl_a,
		w.DataVerify,
		w.NodeH)
}

type Meter struct {
	Tokens                uint64 `json:"tokens,omitempty"`                // The number of tokens per time_unit
	PerTimeUnit           string `json:"per_time_unit,omitempty"`         // The per time units: min, hour and day are supported
	NotificationIntervalS int    `json:"notification_interval,omitempty"` // The number of seconds between metering notifications
}

type DataVerification struct {
	Enabled     bool   `json:"enabled,omitempty"`    // Whether or not data verification is enabled
	URL         string `json:"URL,omitempty"`        // The URL to be used for data receipt verification
	URLUser     string `json:"user,omitempty"`       // The user id to use when calling the verification URL
	URLPassword string `json:"password,omitempty"`   // The password to use when calling the verification URL
	Interval    int    `json:"interval,omitempty"`   // The number of seconds to check for data before deciding there isnt any data
	CheckRate   int    `json:"check_rate,omitempty"` // The number of seconds between checks for valid data being received
	Metering    Meter  `json:"metering,omitempty"`   // The metering configuration
}

type NodeHealth struct {
	MissingHBInterval    int `json:"missing_heartbeat_interval,omitempty"` // How long a heartbeat can be missing until it is considered missing (in seconds)
	CheckAgreementStatus int `json:"check_agreement_status,omitempty"`     // How often to check that the node agreement entry still exists in the exchange (in seconds)
}

type Blockchain struct {
	Type string `json:"type,omitempty"`         // The type of blockchain
	Name string `json:"name,omitempty"`         // The name of the blockchain instance in the exchange,it is specific to the value of the type
	Org  string `json:"organization,omitempty"` // The organization that owns the blockchain definition
}

type BlockchainList []Blockchain

type AgreementProtocol struct {
	Name            string         `json:"name,omitempty"`            // The name of the agreement protocol to be used
	ProtocolVersion int            `json:"protocolVersion,omitempty"` // The max protocol version supported
	Blockchains     BlockchainList `json:"blockchains,omitempty"`     // The blockchain to be used if the protocol requires one.
}

type GetPatternResponse struct {
	Patterns  map[string]Pattern `json:"patterns,omitempty"` // map of all defined patterns
	LastIndex int                `json:"lastIndex.omitempty"`
}

// Get all the pattern metadata for a specific organization, and pattern if specified.
func GetPatterns(httpClientFactory *config.HTTPClientFactory, org string, pattern string, exURL string, id string, token string) (map[string]Pattern, error) {

	if pattern == "" {
		glog.V(3).Infof(rpclogString(fmt.Sprintf("getting pattern definitions for %v", org)))
	} else {
		glog.V(3).Infof(rpclogString(fmt.Sprintf("getting pattern definitions for %v/%v", org, pattern)))
	}

	var resp interface{}
	resp = new(GetPatternResponse)

	// Search the exchange for the pattern definitions
	targetURL := ""
	if pattern == "" {
		targetURL = fmt.Sprintf("%vorgs/%v/patterns", exURL, org)
	} else {
		targetURL = fmt.Sprintf("%vorgs/%v/patterns/%v", exURL, org, pattern)
	}

	retryCount := httpClientFactory.RetryCount
	retryInterval := httpClientFactory.GetRetryInterval()
	for {
		if err, tpErr := InvokeExchange(httpClientFactory.NewHTTPClient(nil), "GET", targetURL, id, token, nil, &resp); err != nil {
			glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			if httpClientFactory.RetryCount == 0 {
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			} else if retryCount == 0 {
				return nil, fmt.Errorf("Exceeded %v retries for error: %v", httpClientFactory.RetryCount, tpErr)
			} else {
				retryCount--
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			}
		} else {
			var pats map[string]Pattern
			if resp != nil {
				pats = resp.(*GetPatternResponse).Patterns
			}

			if pattern != "" {
				pat0 := ""
				for _, pat := range pats {
					// log the pat with signatures truncated
					pat0 = pat.ShortString()
					break
				}
				glog.V(3).Infof(rpclogString(fmt.Sprintf("found pattern for %v, %v", org, pat0)))
			} else {
				glog.V(3).Infof(rpclogString(fmt.Sprintf("found %v patterns for %v.", len(pats), org)))
			}

			return pats, nil
		}
	}
}

// Create a name for the generated policy that should be unique within the org.
func makePolicyName(patternName string, workloadURL string, workloadOrg string, workloadArch string) string {

	url := workloadURL

	// for the old url style, get the part after '*://'
	pieces := strings.SplitN(workloadURL, "/", 3)
	if len(pieces) >= 3 {
		url = strings.TrimSuffix(pieces[2], "/")
	}

	// convert slash to under line
	url = strings.Replace(url, "/", "-", -1)

	return fmt.Sprintf("%v_%v_%v_%v", patternName, url, workloadOrg, workloadArch)
}

// Convert a pattern to a list of policy objects. Each pattern contains 1 or more workloads or services,
// which will each be translated to a policy.
func ConvertToPolicies(patternId string, p *Pattern) ([]*policy.Policy, error) {

	name := GetId(patternId)

	policies := make([]*policy.Policy, 0, 10)

	// Each pattern contains a list of services that need to be converted to a policy

	for _, service := range p.Services {

		// Don't generate policies on the agbot for agreement-less services because the service will be started
		// on the node as soon as the node is configured.
		if service.AgreementLess {
			continue
		}

		// make sure required fields are not empty
		if service.ServiceURL == "" || service.ServiceOrg == "" || service.ServiceArch == "" {
			return nil, fmt.Errorf("serviceUrl, serviceOrgid or serviceArch is empty string in pattern %v.", name)
		} else if service.ServiceVersions == nil || len(service.ServiceVersions) == 0 {
			return nil, fmt.Errorf("The serviceVersions array is empty in pattern %v.", name)
		}

		policyName := makePolicyName(name, service.ServiceURL, service.ServiceOrg, service.ServiceArch)

		pol := policy.Policy_Factory(fmt.Sprintf("%v", policyName))

		// Copy service metadata into the policy
		for _, wl := range service.ServiceVersions {
			if wl.Version == "" {
				return nil, fmt.Errorf("The version for service %v arch %v is empty in pattern %v.", service.ServiceURL, service.ServiceArch, name)
			}
			ConvertChoice(wl, service.ServiceURL, service.ServiceOrg, service.ServiceArch, pol)
		}

		ConvertCommon(p, patternId, service.DataVerify, service.NodeH, pol)

		glog.V(3).Infof(rpclogString(fmt.Sprintf("converted %v into policy %v", service.ShortString(), policyName)))
		policies = append(policies, pol)

	}

	return policies, nil

}

func ConvertChoice(wl WorkloadChoice, url string, org string, arch string, pol *policy.Policy) {
	newWL := policy.Workload_Factory(url, org, wl.Version, arch)
	newWL.Priority = (*policy.Workload_Priority_Factory(wl.Priority.PriorityValue, wl.Priority.Retries, wl.Priority.RetryDurationS, wl.Priority.VerifiedDurationS))
	newWL.DeploymentOverrides = wl.DeploymentOverrides
	newWL.DeploymentOverridesSignature = wl.DeploymentOverridesSignature
	pol.Add_Workload(newWL)
}

func ConvertDataVerify(dv DataVerification, pol *policy.Policy) {
	// Copy Data Verification metadata into the policy
	if dv.Enabled {
		mp := policy.Meter{
			Tokens:                dv.Metering.Tokens,
			PerTimeUnit:           dv.Metering.PerTimeUnit,
			NotificationIntervalS: dv.Metering.NotificationIntervalS,
		}
		d := policy.DataVerification_Factory(dv.URL, dv.URLUser, dv.URLPassword, dv.Interval, dv.CheckRate, mp)
		pol.Add_DataVerification(d)
	}
}

func ConvertNodeHealth(nodeh NodeHealth, pol *policy.Policy) {
	// Copy over the node health policy
	nh := policy.NodeHealth_Factory(nodeh.MissingHBInterval, nodeh.CheckAgreementStatus)
	pol.Add_NodeHealth(nh)
}

// Copy Agreement protocol metadata into the policy
func ConvertAgreementProtocol(p *Pattern, pol *policy.Policy) {
	if p.AgreementProtocols == nil || len(p.AgreementProtocols) == 0 {
		// add default agreement protocol
		newAGP := policy.AgreementProtocol_Factory(policy.BasicProtocol)
		newAGP.Initialize()
		pol.Add_Agreement_Protocol(newAGP)
	} else {
		for _, agp := range p.AgreementProtocols {
			newAGP := policy.AgreementProtocol_Factory(agp.Name)
			newAGP.Initialize()
			for _, bc := range agp.Blockchains {
				newBC := policy.Blockchain_Factory(bc.Type, bc.Name, bc.Org)
				(&newAGP.Blockchains).Add_Blockchain(newBC)
			}
			pol.Add_Agreement_Protocol(newAGP)
		}
	}
}

// Common conversion function calls
func ConvertCommon(p *Pattern, patternId string, dv DataVerification, nodeh NodeHealth, pol *policy.Policy) {

	ConvertDataVerify(dv, pol)

	ConvertNodeHealth(nodeh, pol)

	ConvertAgreementProtocol(p, pol)

	// Indicate that this is a pattern based policy file. Manually created policy files should not use this field.
	pol.PatternId = patternId

	// Unlimited number of devices can get this service
	pol.MaxAgreements = 0

	// make a copy of the user input
	pol.UserInput = make([]policy.UserInput, len(p.UserInput))
	copy(pol.UserInput, p.UserInput)

}

// Structs and types for working with pattern based exchange searches
type SearchExchangePatternRequest struct {
	ServiceURL   string   `json:"serviceUrl,omitempty"`
	NodeOrgIds   []string `json:"nodeOrgids,omitempty"`
	SecondsStale int      `json:"secondsStale"`
	NumEntries   int      `json:"numEntries"`
}

func (a SearchExchangePatternRequest) String() string {
	return fmt.Sprintf("ServiceURL: %v, SecondsStale: %v, NumEntries: %v", a.ServiceURL, a.SecondsStale, a.NumEntries)
}

type SearchExchangePatternResponse struct {
	Devices   []SearchResultDevice `json:"nodes"`
	LastIndex int                  `json:"lastIndex"`
}

func (r SearchExchangePatternResponse) String() string {
	return fmt.Sprintf("Devices: %v, LastIndex: %v", r.Devices, r.LastIndex)
}

// This function creates the exchange search message body.
func CreateSearchPatternRequest() *SearchExchangePatternRequest {

	ser := &SearchExchangePatternRequest{
		NumEntries: 0,
	}

	return ser
}

func GetPatternNodes(ec ExchangeContext, policyOrg string, patternId string, req *SearchExchangePatternRequest) (*[]SearchResultDevice, error) {
	// Invoke the exchange
	var resp interface{}
	resp = new(SearchExchangePatternResponse)
	targetURL := ec.GetExchangeURL() + "orgs/" + policyOrg + "/patterns/" + GetId(patternId) + "/search"
	for {
		if err, tpErr := InvokeExchange(ec.GetHTTPFactory().NewHTTPClient(nil), "POST", targetURL, ec.GetExchangeId(), ec.GetExchangeToken(), *req, &resp); err != nil {
			if !strings.Contains(err.Error(), "status: 404") {
				return nil, err
			} else {
				empty := make([]SearchResultDevice, 0, 0)
				return &empty, nil
			}
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			dev := resp.(*SearchExchangePatternResponse).Devices
			return &dev, nil
		}
	}
}
