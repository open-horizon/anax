// Package show displays various information about the Horizon edge node.
// The information is mostly obtained from the Horizon API, but in many cases massaged to be more human consumable.
package show

import (
	"encoding/json"
	"fmt"
	dockerclient "github.com/fsouza/go-dockerclient"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
)

const MUST_REGISTER_FIRST = "this command can not be run before running 'hzn register'"

//~~~~~~~~~~~~~~~~ show node ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

type Configstate struct {
	State          *string `json:"state"`
	LastUpdateTime string  `json:"last_update_time"` // removed omitempty
}

// This is a combo of anax's HorizonDevice and Info (status) structs
type NodeAndStatus struct {
	// from api.HorizonDevice
	Id                 *string     `json:"id"`
	Org                *string     `json:"organization"`
	Pattern            *string     `json:"pattern"`               // a simple name, not prefixed with the org
	Name               *string     `json:"name"`                  // removed omitempty
	Token              *string     `json:"token"`                 // removed omitempty
	TokenLastValidTime string      `json:"token_last_valid_time"` // removed omitempty
	TokenValid         *bool       `json:"token_valid"`           // removed omitempty
	HA                 *bool       `json:"ha"`                    // removed omitempty
	Config             Configstate `json:"configstate"`           // removed omitempty
	// from api.Info
	Geths         []api.Geth         `json:"geth"`
	Configuration *api.Configuration `json:"configuration"`
	Connectivity  map[string]bool    `json:"connectivity"`
}

// CopyNodeInto copies the node info into our output struct and converts times in the process
func (n *NodeAndStatus) CopyNodeInto(horDevice *api.HorizonDevice) {
	//todo: I don't like having to repeat all of these fields, hard to maintain. Maybe use reflection?
	n.Id = horDevice.Id
	n.Org = horDevice.Org
	n.Pattern = horDevice.Pattern
	n.Name = horDevice.Name
	n.Token = horDevice.Token
	if horDevice.TokenLastValidTime != nil {
		n.TokenLastValidTime = cliutils.ConvertTime(*horDevice.TokenLastValidTime)
	}
	n.TokenValid = horDevice.TokenValid
	n.HA = horDevice.HA
	n.Config.State = horDevice.Config.State
	if horDevice.Config.LastUpdateTime != nil {
		n.Config.LastUpdateTime = cliutils.ConvertTime(*horDevice.Config.LastUpdateTime)
	}
}

// CopyStatusInto copies the status info into our output struct
func (n *NodeAndStatus) CopyStatusInto(status *api.Info) {
	//todo: I don't like having to repeat all of these fields, hard to maintain. Maybe use reflection?
	n.Geths = status.Geths
	n.Configuration = status.Configuration
	n.Connectivity = status.Connectivity
}

func Node() {
	// Get the node info
	horDevice := api.HorizonDevice{}
	cliutils.HorizonGet("node", []int{200}, &horDevice)
	nodeInfo := NodeAndStatus{} // the structure we will output
	nodeInfo.CopyNodeInto(&horDevice)

	// Get the horizon status info
	status := api.Info{}
	cliutils.HorizonGet("status", []int{200}, &status)
	nodeInfo.CopyStatusInto(&status)

	// Output the combined info
	jsonBytes, err := json.MarshalIndent(nodeInfo, "", cliutils.JSON_INDENT)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'show node' output: %v", err)
	}
	fmt.Printf("%s\n", jsonBytes) //todo: is there a way to output with json syntax highlighting like jq does?
}

//~~~~~~~~~~~~~~~~ show agreements ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

type ActiveAgreement struct {
	Name                        string                   `json:"name"`
	CurrentAgreementId          string                   `json:"current_agreement_id"`
	ConsumerId                  string                   `json:"consumer_id"`
	AgreementCreationTime       string                   `json:"agreement_creation_time"`
	AgreementAcceptedTime       string                   `json:"agreement_accepted_time"`
	AgreementFinalizedTime      string                   `json:"agreement_finalized_time"`
	AgreementExecutionStartTime string                   `json:"agreement_execution_start_time"`
	AgreementDataReceivedTime   string                   `json:"agreement_data_received_time"`
	AgreementProtocol           string                   `json:"agreement_protocol"` // the agreement protocol being used. It is also in the proposal.
	Workload                    persistence.WorkloadInfo `json:"workload_to_run"`
}

// CopyAgreementInto copies the agreement info into our output struct
func (a *ActiveAgreement) CopyAgreementInto(agreement persistence.EstablishedAgreement) {
	//todo: I don't like having to repeat all of these fields, hard to maintain. Maybe use reflection?
	a.Name = agreement.Name
	a.CurrentAgreementId = agreement.CurrentAgreementId
	a.ConsumerId = agreement.ConsumerId

	a.AgreementCreationTime = cliutils.ConvertTime(agreement.AgreementCreationTime)
	a.AgreementAcceptedTime = cliutils.ConvertTime(agreement.AgreementAcceptedTime)
	a.AgreementFinalizedTime = cliutils.ConvertTime(agreement.AgreementFinalizedTime)
	a.AgreementExecutionStartTime = cliutils.ConvertTime(agreement.AgreementExecutionStartTime)
	a.AgreementDataReceivedTime = cliutils.ConvertTime(agreement.AgreementDataReceivedTime)

	a.AgreementProtocol = agreement.AgreementProtocol

	a.Workload = agreement.RunningWorkload
}

type ArchivedAgreement struct {
	Name                        string                   `json:"name"`
	CurrentAgreementId          string                   `json:"current_agreement_id"`
	ConsumerId                  string                   `json:"consumer_id"`
	AgreementCreationTime       string                   `json:"agreement_creation_time"`
	AgreementAcceptedTime       string                   `json:"agreement_accepted_time"`
	AgreementFinalizedTime      string                   `json:"agreement_finalized_time"`
	AgreementExecutionStartTime string                   `json:"agreement_execution_start_time"`
	AgreementDataReceivedTime   string                   `json:"agreement_data_received_time"`
	AgreementProtocol           string                   `json:"agreement_protocol"` // the agreement protocol being used. It is also in the proposal.
	Workload                    persistence.WorkloadInfo `json:"workload_to_run"`

	AgreementTerminatedTime string `json:"agreement_terminated_time"`
	TerminatedReason        uint64 `json:"terminated_reason"`      // the reason that the agreement was terminated
	TerminatedDescription   string `json:"terminated_description"` // a string form of the reason that the agreement was terminated
}

// CopyAgreementInto copies the agreement info into our output struct
func (a *ArchivedAgreement) CopyAgreementInto(agreement persistence.EstablishedAgreement) {
	//todo: what's the best way to make this part common with the active agreement copy? Interface? Anonymous struct?
	a.Name = agreement.Name
	a.CurrentAgreementId = agreement.CurrentAgreementId
	a.ConsumerId = agreement.ConsumerId
	a.AgreementCreationTime = cliutils.ConvertTime(agreement.AgreementCreationTime)
	a.AgreementAcceptedTime = cliutils.ConvertTime(agreement.AgreementAcceptedTime)
	a.AgreementFinalizedTime = cliutils.ConvertTime(agreement.AgreementFinalizedTime)
	a.AgreementExecutionStartTime = cliutils.ConvertTime(agreement.AgreementExecutionStartTime)
	a.AgreementDataReceivedTime = cliutils.ConvertTime(agreement.AgreementDataReceivedTime)
	a.AgreementProtocol = agreement.AgreementProtocol
	a.Workload = agreement.RunningWorkload

	a.AgreementTerminatedTime = cliutils.ConvertTime(agreement.AgreementTerminatedTime)
	a.TerminatedReason = agreement.TerminatedReason
	a.TerminatedDescription = agreement.TerminatedDescription
}

func Agreements(archivedAgreements bool) {
	// Get horizon api agreement output and drill down to the category we want
	apiOutput := make(map[string]map[string][]persistence.EstablishedAgreement, 0)
	cliutils.HorizonGet("agreement", []int{200}, &apiOutput)
	var ok bool
	if _, ok = apiOutput["agreements"]; !ok {
		cliutils.Fatal(cliutils.HTTP_ERROR, "horizon api agreement output did not include 'agreements' key")
	}
	whichAgreements := "active"
	if archivedAgreements {
		whichAgreements = "archived"
	}
	var apiAgreements []persistence.EstablishedAgreement
	if apiAgreements, ok = apiOutput["agreements"][whichAgreements]; !ok {
		cliutils.Fatal(cliutils.HTTP_ERROR, "horizon api agreement output did not include '%s' key", whichAgreements)
	}

	// Go thru the apiAgreements and convert into our output struct and then print
	if !archivedAgreements {
		agreements := make([]ActiveAgreement, len(apiAgreements))
		for i := range apiAgreements {
			agreements[i].CopyAgreementInto(apiAgreements[i])
		}
		jsonBytes, err := json.MarshalIndent(agreements, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'show agreements' output: %v", err)
		}
		fmt.Printf("%s\n", jsonBytes)
	} else {
		agreements := make([]ArchivedAgreement, len(apiAgreements))
		for i := range apiAgreements {
			agreements[i].CopyAgreementInto(apiAgreements[i])
		}
		jsonBytes, err := json.MarshalIndent(agreements, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'show agreements' output: %v", err)
		}
		fmt.Printf("%s\n", jsonBytes)
	}
}

//~~~~~~~~~~~~~~~~ show metering ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

type MeteringNotification struct {
	Amount                 uint64 `json:"amount"`                       // The number of tokens granted by this notification, rounded to the nearest minute
	StartTime              string `json:"start_time"`                   // The time when the agreement started, in seconds since 1970.
	CurrentTime            string `json:"current_time"`                 // The time when the notification was sent, in seconds since 1970.
	MissedTime             uint64 `json:"missed_time"`                  // The amount of time in seconds that the consumer detected missing data
	ConsumerMeterSignature string `json:"consumer_meter_signature"`     // The consumer's signature of the meter (amount, current time, agreement Id)
	AgreementHash          string `json:"agreement_hash"`               // The 32 byte SHA3 FIPS 202 hash of the proposal for the agreement.
	ConsumerSignature      string `json:"consumer_agreement_signature"` // The consumer's signature of the agreement hash.
	ConsumerAddress        string `json:"consumer_address"`             // The consumer's blockchain account/address.
	ProducerSignature      string `json:"producer_agreement_signature"` // The producer's signature of the agreement
	BlockchainType         string `json:"blockchain_type"`              // The type of the blockchain that this notification is intended to work with
}

func (m *MeteringNotification) CopyMeteringInto(metering persistence.MeteringNotification) {
	//todo: I don't like having to repeat all of these fields, hard to maintain. Maybe use reflection?
	m.Amount = metering.Amount
	m.StartTime = cliutils.ConvertTime(metering.StartTime)
	m.CurrentTime = cliutils.ConvertTime(metering.CurrentTime)
	m.MissedTime = metering.MissedTime
	m.ConsumerMeterSignature = metering.ConsumerMeterSignature
	m.AgreementHash = metering.AgreementHash
	m.ConsumerSignature = metering.ConsumerSignature
	m.ConsumerAddress = metering.ConsumerAddress
	m.ProducerSignature = metering.ProducerSignature
	m.BlockchainType = metering.BlockchainType
}

type ActiveMetering struct {
	Name                      string               `json:"name"`
	CurrentAgreementId        string               `json:"current_agreement_id"`
	ConsumerId                string               `json:"consumer_id"`
	AgreementFinalizedTime    string               `json:"agreement_finalized_time"`
	AgreementDataReceivedTime string               `json:"agreement_data_received_time"`
	AgreementProtocol         string               `json:"agreement_protocol"`              // the agreement protocol being used. It is also in the proposal.
	MeteringNotificationMsg   MeteringNotification `json:"metering_notification,omitempty"` // the most recent metering notification received
}

// CopyAgreementInto copies the agreement info into our output struct
func (a *ActiveMetering) CopyAgreementInto(agreement persistence.EstablishedAgreement) {
	//todo: I don't like having to repeat all of these fields, hard to maintain. Maybe use reflection?
	a.Name = agreement.Name
	a.CurrentAgreementId = agreement.CurrentAgreementId
	a.ConsumerId = agreement.ConsumerId
	a.AgreementFinalizedTime = cliutils.ConvertTime(agreement.AgreementFinalizedTime)
	a.AgreementDataReceivedTime = cliutils.ConvertTime(agreement.AgreementDataReceivedTime)
	a.AgreementProtocol = agreement.AgreementProtocol
	a.MeteringNotificationMsg.CopyMeteringInto(agreement.MeteringNotificationMsg)
}

type ArchivedMetering struct {
	Name                      string `json:"name"`
	CurrentAgreementId        string `json:"current_agreement_id"`
	ConsumerId                string `json:"consumer_id"`
	AgreementFinalizedTime    string `json:"agreement_finalized_time"`
	AgreementDataReceivedTime string `json:"agreement_data_received_time"`
	AgreementProtocol         string `json:"agreement_protocol"` // the agreement protocol being used. It is also in the proposal.

	AgreementTerminatedTime string `json:"agreement_terminated_time"`
	TerminatedReason        uint64 `json:"terminated_reason"`      // the reason that the agreement was terminated
	TerminatedDescription   string `json:"terminated_description"` // a string form of the reason that the agreement was terminated

	MeteringNotificationMsg MeteringNotification `json:"metering_notification,omitempty"` // the most recent metering notification received
}

// CopyAgreementInto copies the agreement info into our output struct
func (a *ArchivedMetering) CopyAgreementInto(agreement persistence.EstablishedAgreement) {
	//todo: I don't like having to repeat all of these fields, hard to maintain. Maybe use reflection?
	a.Name = agreement.Name
	a.CurrentAgreementId = agreement.CurrentAgreementId
	a.ConsumerId = agreement.ConsumerId
	a.AgreementFinalizedTime = cliutils.ConvertTime(agreement.AgreementFinalizedTime)
	a.AgreementDataReceivedTime = cliutils.ConvertTime(agreement.AgreementDataReceivedTime)
	a.AgreementProtocol = agreement.AgreementProtocol

	a.AgreementTerminatedTime = cliutils.ConvertTime(agreement.AgreementTerminatedTime)
	a.TerminatedReason = agreement.TerminatedReason
	a.TerminatedDescription = agreement.TerminatedDescription

	a.MeteringNotificationMsg.CopyMeteringInto(agreement.MeteringNotificationMsg)
}

func Metering(archivedMetering bool) {
	apiOutput := make(map[string]map[string][]persistence.EstablishedAgreement, 0)
	cliutils.HorizonGet("agreement", []int{200}, &apiOutput)
	var ok bool
	if _, ok = apiOutput["agreements"]; !ok {
		cliutils.Fatal(cliutils.HTTP_ERROR, "horizon api agreement output did not include 'agreements' key")
	}
	whichAgreements := "active"
	if archivedMetering {
		whichAgreements = "archived"
	}
	var apiAgreements []persistence.EstablishedAgreement
	if apiAgreements, ok = apiOutput["agreements"][whichAgreements]; !ok {
		cliutils.Fatal(cliutils.HTTP_ERROR, "horizon api agreement output did not include '%s' key", whichAgreements)
	}

	// Go thru the apiAgreements and convert into our output struct and then print
	if !archivedMetering {
		metering := make([]ActiveMetering, len(apiAgreements))
		for i := range apiAgreements {
			metering[i].CopyAgreementInto(apiAgreements[i])
		}
		jsonBytes, err := json.MarshalIndent(metering, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'show metering' output: %v", err)
		}
		fmt.Printf("%s\n", jsonBytes)
	} else {
		metering := make([]ArchivedMetering, len(apiAgreements))
		for i := range apiAgreements {
			metering[i].CopyAgreementInto(apiAgreements[i])
		}
		jsonBytes, err := json.MarshalIndent(metering, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'show metering' output: %v", err)
		}
		fmt.Printf("%s\n", jsonBytes)
	}
}

//~~~~~~~~~~~~~~~~ show keys ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

func Keys() {
	apiOutput := make(map[string][]string, 0)
	// Note: it is allowed to get /publickey before post /node is called, so we don't have to check for that error
	cliutils.HorizonGet("publickey", []int{200}, &apiOutput)
	var ok bool
	if _, ok = apiOutput["pem"]; !ok {
		cliutils.Fatal(cliutils.HTTP_ERROR, "horizon api publickey/x509 cert output did not include 'pem'-encoded block")
	}
	jsonBytes, err := json.MarshalIndent(apiOutput["pem"], "", cliutils.JSON_INDENT)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'show pem' output: %v", err)
	}
	fmt.Printf("%s\n", jsonBytes)
}

//~~~~~~~~~~~~~~~~ show attributes ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

const HTTPSBasicAuthAttributes = "HTTPSBasicAuthAttributes"

// Our form of the attributes output
type OurAttributes struct {
	Type       string                 `json:"type"`
	Label      string                 `json:"label"`
	SensorUrls []string               `json:"sensor_urls,omitempty"`
	Variables  map[string]interface{} `json:"variables"`
}

func Attributes() {
	// Get the attributes
	apiOutput := map[string][]api.Attribute{}
	httpCode := cliutils.HorizonGet("attribute", []int{200, cliutils.ANAX_NOT_CONFIGURED_YET}, &apiOutput)
	if httpCode == cliutils.ANAX_NOT_CONFIGURED_YET {
		cliutils.Fatal(cliutils.HTTP_ERROR, MUST_REGISTER_FIRST)
	}
	var ok bool
	if _, ok = apiOutput["attributes"]; !ok {
		cliutils.Fatal(cliutils.HTTP_ERROR, "horizon api attributes output did not include 'attributes' key")
	}
	apiAttrs := apiOutput["attributes"]

	// Only include interesting fields in our output
	var attrs []OurAttributes
	for _, a := range apiAttrs {
		if len(*a.SensorUrls) == 0 {
			attrs = append(attrs, OurAttributes{Type: *a.Type, Label: *a.Label, Variables: *a.Mappings})
		} else if *a.Type == HTTPSBasicAuthAttributes {
			attrs = append(attrs, OurAttributes{Type: *a.Type, Label: *a.Label, SensorUrls: *a.SensorUrls, Variables: *a.Mappings})
		}
	}

	// Convert to json and output
	jsonBytes, err := json.MarshalIndent(attrs, "", cliutils.JSON_INDENT)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'show attributes' output: %v", err)
	}
	fmt.Printf("%s\n", jsonBytes)
}

//~~~~~~~~~~~~~~~~ show services ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

type APIMicroservices struct {
	Config      []api.APIMicroserviceConfig `json:"config"`      // the microservice configurations
	Instances   map[string][]interface{}    `json:"instances"`   // the microservice instances that are running
	Definitions map[string][]interface{}    `json:"definitions"` // the definitions of microservices from the exchange
}

type OurService struct {
	APISpecs  policy.APISpecList     `json:"apiSpec"` // removed omitempty
	Variables map[string]interface{} `json:"variables"`
}

func Services() {
	// Get the services
	var apiOutput APIMicroservices
	// Note: intentionally querying /microservice, instead of just /microservice/config, because in the future we will probably want to mix in some key runtime info
	httpCode := cliutils.HorizonGet("microservice", []int{200, cliutils.ANAX_NOT_CONFIGURED_YET}, &apiOutput)
	if httpCode == cliutils.ANAX_NOT_CONFIGURED_YET {
		cliutils.Fatal(cliutils.HTTP_ERROR, MUST_REGISTER_FIRST)
	}
	apiServices := apiOutput.Config

	// Go thru the services and pull out interesting fields
	services := make([]OurService, 0)
	for _, s := range apiServices {
		serv := OurService{Variables: make(map[string]interface{})}
		asl := new(policy.APISpecList)
		asl.Add_API_Spec(policy.APISpecification_Factory(s.SensorUrl, s.SensorOrg, s.SensorVersion, cutil.ArchString()))
		serv.APISpecs = *asl
		// Copy all of the variables from each Mappings into our Variables map
		for _, a := range s.Attributes {
			if a.Mappings != nil {
				for k, v := range *a.Mappings {
					serv.Variables[k] = v
				}
			}
		}
		services = append(services, serv)
	}
	//todo: should we mix in any info from /microservice/policy?

	// Convert to json and output
	jsonBytes, err := json.MarshalIndent(services, "", cliutils.JSON_INDENT)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'show services' output: %v", err)
	}
	fmt.Printf("%s\n", jsonBytes)
}

//~~~~~~~~~~~~~~~~ show workloads ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

// Can't use the api and persistence structs because the Attributes type isn't detailed enough to drill down into it
type WorkloadConfigOnly struct {
	WorkloadURL       string                              `json:"workload_url"`
	Org               string                              `json:"organization"`
	VersionExpression string                              `json:"workload_version"` // This is a version range
	Attributes        []map[string]map[string]interface{} `json:"attributes"`
}

type APIWorkloads struct {
	Config     []WorkloadConfigOnly          `json:"config"`     // the workload configurations
	Containers *[]dockerclient.APIContainers `json:"containers"` // the docker info for a running container
}

// What we will output. Need our own structure because we want to pick and choose what we output.
type OurWorkload struct {
	WorkloadURL       string                 `json:"workload_url"`
	Org               string                 `json:"organization"`
	VersionExpression string                 `json:"workload_version"` // This is a version range
	Variables         map[string]interface{} `json:"variables"`
}

func Workloads() {
	// Get the workloads
	var apiOutput APIWorkloads
	// Note: intentionally querying /workload, instead of just /workload/config, because in the future we will probably want to mix in some key runtime info
	httpCode := cliutils.HorizonGet("workload", []int{200, cliutils.ANAX_NOT_CONFIGURED_YET}, &apiOutput)
	if httpCode == cliutils.ANAX_NOT_CONFIGURED_YET {
		cliutils.Fatal(cliutils.HTTP_ERROR, MUST_REGISTER_FIRST)
	}
	apiWorkloads := apiOutput.Config

	// Only include interesting fields in our output
	workloads := make([]OurWorkload, len(apiWorkloads))
	for i := range apiWorkloads {
		workloads[i].Org = apiWorkloads[i].Org
		workloads[i].WorkloadURL = apiWorkloads[i].WorkloadURL
		workloads[i].VersionExpression = apiWorkloads[i].VersionExpression
		// Copy all of the variables from each Mappings into our Variables map
		workloads[i].Variables = make(map[string]interface{})
		for _, a := range apiWorkloads[i].Attributes {
			if m, ok := a["mappings"]; ok {
				for k, v := range m {
					workloads[i].Variables[k] = v
				}
			}
		}
	}
	//todo: should we mix in any other info from /workload?

	// Convert to json and output
	jsonBytes, err := json.MarshalIndent(workloads, "", cliutils.JSON_INDENT)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'show workloads' output: %v", err)
	}
	fmt.Printf("%s\n", jsonBytes)
}
