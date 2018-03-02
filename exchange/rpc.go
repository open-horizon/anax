package exchange

import (
	"bytes"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/policy"
	"io/ioutil"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// microservice sharing mode
const MS_SHARING_MODE_EXCLUSIVE = "exclusive"
const MS_SHARING_MODE_SINGLE = "single"
const MS_SHARING_MODE_MULTIPLE = "multiple"

const MICROSERVICE = "microservce"
const WORKLOAD = "workload"
const PATTERN = "pattern"

// Helper functions for dealing with exchangeIds that are already prefixed with the org name and then "/".
func GetOrg(id string) string {
	if ix := strings.Index(id, "/"); ix < 0 {
		return ""
	} else {
		return id[:ix]
	}
}

func GetId(id string) string {
	if ix := strings.Index(id, "/"); ix < 0 {
		return ""
	} else {
		return id[ix+1:]
	}
}

// Structs used to invoke the exchange API
type MSProp struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	PropType string `json:"propType"`
	Op       string `json:"op"`
}

func (p MSProp) String() string {
	return fmt.Sprintf("Property %v %v %v, Type: %v,", p.Name, p.Op, p.Value, p.PropType)
}

type Microservice struct {
	Url           string   `json:"url"`
	Properties    []MSProp `json:"properties"`
	NumAgreements int      `json:"numAgreements"`
	Policy        string   `json:"policy"`
}

func (m Microservice) String() string {
	return fmt.Sprintf("URL: %v, Properties: %v, NumAgreements: %v, Policy: %v", m.Url, m.Properties, m.NumAgreements, m.Policy)
}

func (m Microservice) ShortString() string {
	return fmt.Sprintf("URL: %v, NumAgreements: %v, Properties: %v", m.Url, m.NumAgreements, m.Properties)
}

// structs and types for working with microservice based exchange searches
type SearchExchangeMSRequest struct {
	DesiredMicroservices []Microservice `json:"desiredMicroservices"`
	SecondsStale         int            `json:"secondsStale"`
	PropertiesToReturn   []string       `json:"propertiesToReturn"`
	StartIndex           int            `json:"startIndex"`
	NumEntries           int            `json:"numEntries"`
}

func (a SearchExchangeMSRequest) String() string {
	return fmt.Sprintf("Microservices: %v, SecondsStale: %v, PropertiesToReturn: %v, StartIndex: %v, NumEntries: %v", a.DesiredMicroservices, a.SecondsStale, a.PropertiesToReturn, a.StartIndex, a.NumEntries)
}

type SearchResultDevice struct {
	Id            string         `json:"id"`
	Name          string         `json:"name"`
	Microservices []Microservice `json:"microservices"`
	MsgEndPoint   string         `json:"msgEndPoint"`
	PublicKey     []byte         `json:"publicKey"`
}

func (d SearchResultDevice) String() string {
	return fmt.Sprintf("Id: %v, Name: %v, Microservices: %v, MsgEndPoint: %v", d.Id, d.Name, d.Microservices, d.MsgEndPoint)
}

func (d SearchResultDevice) ShortString() string {
	str := fmt.Sprintf("Id: %v, Name: %v, MsgEndPoint: %v, Microservice URLs:", d.Id, d.Name, d.MsgEndPoint)
	for _, ms := range d.Microservices {
		str += fmt.Sprintf("%v,", ms.Url)
	}
	return str
}

type SearchExchangeMSResponse struct {
	Devices   []SearchResultDevice `json:"nodes"`
	LastIndex int                  `json:"lastIndex"`
}

func (r SearchExchangeMSResponse) String() string {
	return fmt.Sprintf("Devices: %v, LastIndex: %v", r.Devices, r.LastIndex)
}

// Structs and types for working with pattern based exchange searches
type SearchExchangePatternRequest struct {
	WorkloadURL  string `json:"workloadUrl"`
	SecondsStale int    `json:"secondsStale"`
	StartIndex   int    `json:"startIndex"`
	NumEntries   int    `json:"numEntries"`
}

func (a SearchExchangePatternRequest) String() string {
	return fmt.Sprintf("WorkloadURL: %v, SecondsStale: %v, StartIndex: %v, NumEntries: %v", a.WorkloadURL, a.SecondsStale, a.StartIndex, a.NumEntries)
}

type SearchExchangePatternResponse struct {
	Devices   []SearchResultDevice `json:"nodes"`
	LastIndex int                  `json:"lastIndex"`
}

func (r SearchExchangePatternResponse) String() string {
	return fmt.Sprintf("Devices: %v, LastIndex: %v", r.Devices, r.LastIndex)
}

// Structs and types for interacting with the device (node) object in the exchange
type Device struct {
	Token                   string          `json:"token"`
	Name                    string          `json:"name"`
	Owner                   string          `json:"owner"`
	Pattern                 string          `json:"pattern"`
	RegisteredMicroservices []Microservice  `json:"registeredMicroservices"`
	MsgEndPoint             string          `json:"msgEndPoint"`
	SoftwareVersions        SoftwareVersion `json:"softwareVersions"`
	LastHeartbeat           string          `json:"lastHeartbeat"`
	PublicKey               []byte          `json:"publicKey"`
}

type GetDevicesResponse struct {
	Devices   map[string]Device `json:"nodes"`
	LastIndex int               `json:"lastIndex"`
}

func GetExchangeDevice(httpClientFactory *config.HTTPClientFactory, deviceId string, deviceToken string, exchangeUrl string) (*Device, error) {

	glog.V(3).Infof(rpclogString(fmt.Sprintf("retrieving device %v from exchange", deviceId)))

	var resp interface{}
	resp = new(GetDevicesResponse)
	targetURL := exchangeUrl + "orgs/" + GetOrg(deviceId) + "/nodes/" + GetId(deviceId)
	for {
		if err, tpErr := InvokeExchange(httpClientFactory.NewHTTPClient(nil), "GET", targetURL, deviceId, deviceToken, nil, &resp); err != nil {
			glog.Errorf(err.Error())
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			devs := resp.(*GetDevicesResponse).Devices
			if dev, there := devs[deviceId]; !there {
				return nil, errors.New(fmt.Sprintf("device %v not in GET response %v as expected", deviceId, devs))
			} else {
				glog.V(3).Infof(rpclogString(fmt.Sprintf("retrieved device %v from exchange %v", deviceId, dev)))
				return &dev, nil
			}
		}
	}
}

// modify the the device
func PutExchangeDevice(httpClientFactory *config.HTTPClientFactory, deviceId string, deviceToken string, exchangeUrl string, pdr *PutDeviceRequest) (*PutDeviceResponse, error) {
	// create PUT body
	var resp interface{}
	resp = new(PutDeviceResponse)
	targetURL := exchangeUrl + "orgs/" + GetOrg(deviceId) + "/nodes/" + GetId(deviceId)

	for {
		if err, tpErr := InvokeExchange(httpClientFactory.NewHTTPClient(nil), "PUT", targetURL, deviceId, deviceToken, pdr, &resp); err != nil {
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(3).Infof(rpclogString(fmt.Sprintf("put device %v to exchange %v", deviceId, pdr)))
			return resp.(*PutDeviceResponse), nil
		}
	}
}

type ServedPattern struct {
	Org         string `json:"patternOrgid"`
	Pattern     string `json:"pattern"`
	LastUpdated string `json:"lastUpdated"`
}

type Agbot struct {
	Token         string `json:"token"`
	Name          string `json:"name"`
	Owner         string `json:"owner"`
	MsgEndPoint   string `json:"msgEndPoint"`
	LastHeartbeat string `json:"lastHeartbeat"`
	PublicKey     []byte `json:"publicKey"`
}

func (a Agbot) String() string {
	return fmt.Sprintf("Name: %v, Owner: %v, LastHeartbeat: %v, PublicKey: %x", a.Name, a.Owner, a.LastHeartbeat, a.PublicKey)
}

func (a Agbot) ShortString() string {
	return fmt.Sprintf("Name: %v, Owner: %v, LastHeartbeat: %v", a.Name, a.Owner, a.LastHeartbeat)
}

type GetAgbotsResponse struct {
	Agbots    map[string]Agbot `json:"agbots"`
	LastIndex int              `json:"lastIndex"`
}

type GetAgbotsPatternsResponse struct {
	Patterns map[string]ServedPattern `json:"patterns"`
}

type AgbotAgreement struct {
	Workload    WorkloadAgreement `json:"workload"`
	State       string            `json:"state"`
	LastUpdated string            `json:"lastUpdated"`
}

func (a AgbotAgreement) String() string {
	return fmt.Sprintf("Workload: %v, State: %v, LastUpdated: %v", a.Workload, a.State, a.LastUpdated)
}

type DeviceAgreement struct {
	Microservice []MSAgreementState `json:"microservices"`
	State        string             `json:"state"`
	Workload     WorkloadAgreement  `json:"workload"`
	LastUpdated  string             `json:"lastUpdated"`
}

func (a DeviceAgreement) String() string {
	return fmt.Sprintf("Microservice: %v, State: %v, LastUpdated: %v", a.Microservice, a.State, a.LastUpdated)
}

type AllAgbotAgreementsResponse struct {
	Agreements map[string]AgbotAgreement `json:"agreements"`
	LastIndex  int                       `json:"lastIndex"`
}

func (a AllAgbotAgreementsResponse) String() string {
	return fmt.Sprintf("Agreements: %v, LastIndex: %v", a.Agreements, a.LastIndex)
}

type AllDeviceAgreementsResponse struct {
	Agreements map[string]DeviceAgreement `json:"agreements"`
	LastIndex  int                        `json:"lastIndex"`
}

func (a AllDeviceAgreementsResponse) String() string {
	return fmt.Sprintf("Agreements: %v, LastIndex: %v", a.Agreements, a.LastIndex)
}

type PutDeviceResponse map[string]string

type PostDeviceResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
}

type WorkloadAgreement struct {
	Org     string `json:"orgid"`   // the org of the pattern
	Pattern string `json:"pattern"` // pattern - without the org prefix on it
	URL     string `json:"url"`     // workload URL
}

type PutAgbotAgreementState struct {
	Workload WorkloadAgreement `json:"workload"`
	State    string            `json:"state"`
}

type MSAgreementState struct {
	Org string `json:"orgid"`
	URL string `json:"url"`
}

type PutAgreementState struct {
	Microservices []MSAgreementState `json:"microservices"`
	State         string             `json:"state"`
	Workload      WorkloadAgreement  `json:"workload"`
}

type SoftwareVersion map[string]string

type PutDeviceRequest struct {
	Token                   string          `json:"token"`
	Name                    string          `json:"name"`
	Pattern                 string          `json:"pattern"`
	RegisteredMicroservices []Microservice  `json:"registeredMicroservices"`
	MsgEndPoint             string          `json:"msgEndPoint"`
	SoftwareVersions        SoftwareVersion `json:"softwareVersions"`
	PublicKey               []byte          `json:"publicKey"`
}

func (p PutDeviceRequest) String() string {
	return fmt.Sprintf("Token: %v, Name: %v, RegisteredMicroservices %v, MsgEndPoint %v, SoftwareVersions %v, PublicKey %x", p.Token, p.Name, p.RegisteredMicroservices, p.MsgEndPoint, p.SoftwareVersions, p.PublicKey)
}

func (p PutDeviceRequest) ShortString() string {
	str := fmt.Sprintf("Token: %v, Name: %v, MsgEndPoint %v, SoftwareVersions %v, Microservice URLs: ", p.Token, p.Name, p.MsgEndPoint, p.SoftwareVersions)
	for _, ms := range p.RegisteredMicroservices {
		str += fmt.Sprintf("%v,", ms.Url)
	}
	return str
}

type PatchAgbotPublicKey struct {
	PublicKey []byte `json:"publicKey"`
}

// This function creates the device registration message body.
func CreateAgbotPublicKeyPatch(keyPath string) *PatchAgbotPublicKey {

	keyBytes := func() []byte {
		if pubKey, _, err := GetKeys(keyPath); err != nil {
			glog.Errorf(rpclogString(fmt.Sprintf("Error getting keys %v", err)))
			return []byte(`none`)
		} else if b, err := MarshalPublicKey(pubKey); err != nil {
			glog.Errorf(rpclogString(fmt.Sprintf("Error marshalling agbot public key %v, error %v", pubKey, err)))
			return []byte(`none`)
		} else {
			return b
		}
	}

	pdr := &PatchAgbotPublicKey{
		PublicKey: keyBytes(),
	}

	return pdr
}

type PostMessage struct {
	Message []byte `json:"message"`
	TTL     int    `json:"ttl"`
}

func (p PostMessage) String() string {
	return fmt.Sprintf("TTL: %v, Message: %x...", p.TTL, p.Message[:32])
}

func CreatePostMessage(msg []byte, ttl int) *PostMessage {
	theTTL := 180
	if ttl != 0 {
		theTTL = ttl
	}

	pm := &PostMessage{
		Message: msg,
		TTL:     theTTL,
	}

	return pm
}

type ExchangeMessageTarget struct {
	ReceiverExchangeId     string // in the form org/id
	ReceiverPublicKeyObj   *rsa.PublicKey
	ReceiverPublicKeyBytes []byte
	ReceiverMsgEndPoint    string
}

func CreateMessageTarget(receiverId string, receiverPubKey *rsa.PublicKey, receiverPubKeySerialized []byte, receiverMessageEndpoint string) (*ExchangeMessageTarget, error) {
	if len(receiverMessageEndpoint) == 0 && receiverPubKey == nil && len(receiverPubKeySerialized) == 0 {
		return nil, errors.New(fmt.Sprintf("Must specify either one of the public key inputs OR the message endpoint input for the message receiver %v", receiverId))
	} else if len(receiverMessageEndpoint) != 0 && (receiverPubKey != nil || len(receiverPubKeySerialized) != 0) {
		return nil, errors.New(fmt.Sprintf("Specified message endpoint and at least one of the public key inputs for the message receiver %v, %v or %v", receiverId, receiverPubKey, receiverPubKeySerialized))
	} else {
		return &ExchangeMessageTarget{
			ReceiverExchangeId:     receiverId,
			ReceiverPublicKeyObj:   receiverPubKey,
			ReceiverPublicKeyBytes: receiverPubKeySerialized,
			ReceiverMsgEndPoint:    receiverMessageEndpoint,
		}, nil
	}
}

type DeviceMessage struct {
	MsgId       int    `json:"msgId"`
	AgbotId     string `json:"agbotId"`
	AgbotPubKey []byte `json:"agbotPubKey"`
	Message     []byte `json:"message"`
	TimeSent    string `json:"timeSent"`
}

func (d DeviceMessage) String() string {
	return fmt.Sprintf("MsgId: %v, AgbotId: %v, AgbotPubKey %v, Message %v, TimeSent %v", d.MsgId, d.AgbotId, d.AgbotPubKey, d.Message[:32], d.TimeSent)
}

type GetDeviceMessageResponse struct {
	Messages  []DeviceMessage `json:"messages"`
	LastIndex int             `json:"lastIndex"`
}

type AgbotMessage struct {
	MsgId        int    `json:"msgId"`
	DeviceId     string `json:"nodeId"`
	DevicePubKey []byte `json:"nodePubKey"`
	Message      []byte `json:"message"`
	TimeSent     string `json:"timeSent"`
	TimeExpires  string `json:"timeExpires"`
}

func (a AgbotMessage) String() string {
	return fmt.Sprintf("MsgId: %v, DeviceId: %v, TimeSent %v, TimeExpires %v, DevicePubKey %v, Message %v", a.MsgId, a.DeviceId, a.TimeSent, a.TimeExpires, a.DevicePubKey, a.Message[:32])
}

type GetAgbotMessageResponse struct {
	Messages  []AgbotMessage `json:"messages"`
	LastIndex int            `json:"lastIndex"`
}

type GetEthereumClientResponse struct {
	Blockchains map[string]BlockchainDef `json:"blockchains"`
	LastIndex   int                      `json:"lastIndex"`
}

type BlockchainDef struct {
	Description string `json:"description"`
	DefinedBy   string `json:"definedBy"`
	Details     string `json:"details"`
	LastUpdated string `json:"lastUpdated"`
}

// This is the structure of what is marshalled into the BlockchainDef.Details field of ethereum
// based blockchains.
type ChainInstance struct {
	BlocksURLs    string `json:"blocksURLs"`
	ChainDataDir  string `json:"chainDataDir"`
	DiscoveryURLs string `json:"discoveryURLs"`
	Port          string `json:"port"`
	HostName      string `json:"hostname"`
	Identity      string `json:"identity"`
	KDF           string `json:"kdf"`
	PingHost      string `json:"pingHost"`
	ColonusDir    string `json:"colonusDir"`
	EthDir        string `json:"ethDir"`
	MaxPeers      string `json:"maxPeers"`
	GethLog       string `json:"gethLog"`
}

type ChainDetails struct {
	Arch           string          `json:"arch"`
	DeploymentDesc policy.Workload `json:"deployment_description"`
	Instance       ChainInstance   `json:"instance"`
}

type BlockchainDetails struct {
	Chains []ChainDetails `json:"chains"`
}

// This function creates the exchange search message body.
func CreateSearchMSRequest() *SearchExchangeMSRequest {

	ser := &SearchExchangeMSRequest{
		StartIndex: 0,
		NumEntries: 100,
	}

	return ser
}

// This function creates the exchange search message body.
func CreateSearchPatternRequest() *SearchExchangePatternRequest {

	ser := &SearchExchangePatternRequest{
		StartIndex: 0,
		NumEntries: 100,
	}

	return ser
}

// This function will cause the messaging key to be created if it doesnt already exist.
func keyBytes() []byte {
	if pubKey, _, err := GetKeys(""); err != nil {
		glog.Errorf(rpclogString(fmt.Sprintf("Error getting keys %v", err)))
		return []byte(`none`)
	} else if b, err := MarshalPublicKey(pubKey); err != nil {
		glog.Errorf(rpclogString(fmt.Sprintf("Error marshalling device public key %v, error %v", pubKey, err)))
		return []byte(`none`)
	} else {
		return b
	}
}

// This function creates the device registration message body.
func CreateDevicePut(token string, name string) *PutDeviceRequest {

	// If we have a messaging key, pass it on the PUT.
	pkBytes := []byte("")
	if HasKeys() {
		pkBytes = keyBytes()
	}

	// Create the PUT node body.
	pdr := &PutDeviceRequest{
		Token:            token,
		Name:             name,
		MsgEndPoint:      "",
		Pattern:          "",
		SoftwareVersions: make(map[string]string),
		PublicKey:        pkBytes,
	}

	return pdr
}

// This function creates the device registration complete message body.
func CreatePatchDeviceKey() *PatchAgbotPublicKey {

	// Same request body structure for node and agbot.
	pdr := &PatchAgbotPublicKey{
		PublicKey: keyBytes(),
	}

	return pdr
}

func ConvertToString(a []string) string {
	r := ""
	for _, s := range a {
		r = r + s + ","
	}
	r = strings.TrimRight(r, ",")
	return r
}

func Heartbeat(h *http.Client, url string, id string, token string) error {

	glog.V(5).Infof(rpclogString(fmt.Sprintf("Heartbeating to exchange: %v", url)))

	var resp interface{}
	resp = new(PostDeviceResponse)
	for {
		if err, tpErr := InvokeExchange(h, "POST", url, id, token, nil, &resp); err != nil {
			glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
			return err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(5).Infof(rpclogString(fmt.Sprintf("Sent heartbeat %v: %v", url, resp)))
			break
		}
	}
	return nil

}

func GetEthereumClient(httpClientFactory *config.HTTPClientFactory, url string, org string, chainName string, chainType string, deviceId string, token string) (string, error) {

	glog.V(5).Infof(rpclogString(fmt.Sprintf("getting ethereum client metadata for chain %v/%v", org, chainName)))

	var resp interface{}
	resp = new(GetEthereumClientResponse)
	targetURL := url + "orgs/" + org + "/bctypes/" + chainType + "/blockchains/" + chainName
	for {
		if err, tpErr := InvokeExchange(httpClientFactory.NewHTTPClient(nil), "GET", targetURL, deviceId, token, nil, &resp); err != nil {
			glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
			return "", err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			time.Sleep(10 * time.Second)
			continue
		} else {
			if val, ok := resp.(*GetEthereumClientResponse).Blockchains[chainName]; ok {
				glog.V(3).Infof(rpclogString(fmt.Sprintf("found blockchain %v.", resp)))
				clientMetadata := val.Details
				return clientMetadata, nil
			} else {
				glog.V(3).Infof(rpclogString(fmt.Sprintf("not found blockchain %v.", chainName)))
				return "", nil
			}
		}
	}

}

func ConvertPropertyToExchangeFormat(prop *policy.Property) (*MSProp, error) {
	var pType, pValue, pCompare string

	// version is a special property, it has a special type.
	if prop.Name == "version" {
		newProp := &MSProp{
			Name:     prop.Name,
			Value:    prop.Value.(string),
			PropType: "version",
			Op:       "in",
		}
		return newProp, nil
	}

	switch prop.Value.(type) {
	case string:
		pType = "string"
		pValue = prop.Value.(string)
		pCompare = "in"
	case int:
		pType = "int"
		pValue = strconv.Itoa(prop.Value.(int))
		pCompare = ">="
	case bool:
		pType = "boolean"
		pValue = strconv.FormatBool(prop.Value.(bool))
		pCompare = "="
	case []string:
		pType = "list"
		pValue = ConvertToString(prop.Value.([]string))
		pCompare = "in"
	case float64:
		pType = "int"
		pValue = strconv.Itoa(int(prop.Value.(float64)))
		pCompare = ">="
	default:
		return nil, errors.New(fmt.Sprintf("Encountered unsupported property type: %v converting to exchange format.", reflect.TypeOf(prop.Value).String()))
	}
	// Now put the property together
	newProp := &MSProp{
		Name:     prop.Name,
		Value:    pValue,
		PropType: pType,
		Op:       pCompare,
	}
	return newProp, nil
}

// Functions related to working with workloads and microservices in the exchange
type APISpec struct {
	SpecRef string `json:"specRef"`
	Org     string `json:"org"`
	Version string `json:"version"`
	Arch    string `json:"arch"`
}

type UserInput struct {
	Name         string `json:"name"`
	Label        string `json:"label"`
	Type         string `json:"type"`
	DefaultValue string `json:"defaultValue"`
}

type WorkloadDeployment struct {
	Deployment          string `json:"deployment"`
	DeploymentSignature string `json:"deployment_signature"`
	Torrent             string `json:"torrent"`
}

type WorkloadDefinition struct {
	Owner       string               `json:"owner"`
	Label       string               `json:"label"`
	Description string               `json:"description"`
	Public      bool                 `json:"public"`
	WorkloadURL string               `json:"workloadUrl"`
	Version     string               `json:"version"`
	Arch        string               `json:"arch"`
	DownloadURL string               `json:"downloadUrl"`
	APISpecs    []APISpec            `json:"apiSpec"`
	UserInputs  []UserInput          `json:"userInput"`
	Workloads   []WorkloadDeployment `json:"workloads"`
	LastUpdated string               `json:"lastUpdated"`
}

func (w *WorkloadDefinition) String() string {
	return fmt.Sprintf("Owner: %v, "+
		"Label: %v, "+
		"Description: %v, "+
		"WorkloadURL: %v, "+
		"Version: %v, "+
		"Arch: %v, "+
		"DownloadURL: %v, "+
		"APISpecs: %v, "+
		"UserInputs: %v, "+
		"Workloads: %v, "+
		"LastUpdated: %v",
		w.Owner, w.Label, w.Description, w.WorkloadURL, w.Version, w.Arch, w.DownloadURL,
		w.APISpecs, w.UserInputs, w.Workloads, w.LastUpdated)
}

func (w *WorkloadDefinition) GetUserInputName(name string) *UserInput {
	for _, ui := range w.UserInputs {
		if ui.Name == name {
			return &ui
		}
	}
	return nil
}

func (w *WorkloadDefinition) NeedsUserInput() bool {
	for _, ui := range w.UserInputs {
		if ui.DefaultValue == "" {
			return true
		}
	}
	return false
}

type GetWorkloadsResponse struct {
	Workloads map[string]WorkloadDefinition `json:"workloads"`
	LastIndex int                           `json:"lastIndex"`
}

//todo: should use map[string]string instead of HardwareMatch struct, because MatchHardware is intended to be free-form
//		(contain whatever key/values that the microservice script needs to determine if the node can run it.)
type HardwareMatch struct {
	USBDeviceIds string `json:"usbDeviceIds"`
	Devfiles     string `json:"devFiles"`
}

type MicroserviceDefinition struct {
	Owner         string               `json:"owner"`
	Label         string               `json:"label"`
	Description   string               `json:"description"`
	Public        bool                 `json:"public"`
	SpecRef       string               `json:"specRef"`
	Version       string               `json:"version"`
	Arch          string               `json:"arch"`
	Sharable      string               `json:"sharable"`
	DownloadURL   string               `json:"downloadUrl"`
	MatchHardware HardwareMatch        `json:"matchHardware"`
	UserInputs    []UserInput          `json:"userInput"`
	Workloads     []WorkloadDeployment `json:"workloads"`
	LastUpdated   string               `json:"lastUpdated"`
}

func (w *MicroserviceDefinition) String() string {
	return fmt.Sprintf("Owner: %v, "+
		"Label: %v, "+
		"Description: %v, "+
		"SpecRef: %v, "+
		"Version: %v, "+
		"Arch: %v, "+
		"Sharable: %v, "+
		"DownloadURL: %v, "+
		"MatchHardware: %v, "+
		"UserInputs: %v, "+
		"Workloads: %v, "+
		"LastUpdated: %v",
		w.Owner, w.Label, w.Description, w.SpecRef, w.Version, w.Arch, w.Sharable, w.DownloadURL,
		w.MatchHardware, w.UserInputs, w.Workloads, w.LastUpdated)
}

type GetMicroservicesResponse struct {
	Microservices map[string]MicroserviceDefinition `json:"microservices"`
	LastIndex     int                               `json:"lastIndex"`
}

func getSearchVersion(version string) (string, error) {
	// The caller could pass a specific version or a version range, in the version parameter. If it's a version range
	// then it must be a full expression. That is, it must be expanded into the full syntax. For example; 1.2.3 is a specific
	// version, and [4.5.6, INFINITY) is the full expression corresponding to the shorthand form of "4.5.6".
	searchVersion := ""
	if version == "" || policy.IsVersionExpression(version) {
		// search for all versions
	} else if policy.IsVersionString(version) {
		// search for a specific version
		searchVersion = version
	} else {
		return "", errors.New(fmt.Sprintf("input version %v is not a valid version string", version))
	}
	return searchVersion, nil
}

// Get workload and its exchange id for the given org, url, version and arch. If the the version string is version range, then the highest available workload within the range will be returned.
func GetWorkload(httpClientFactory *config.HTTPClientFactory, wURL string, wOrg string, wVersion string, wArch string, exURL string, id string, token string) (*WorkloadDefinition, string, error) {

	glog.V(3).Infof(rpclogString(fmt.Sprintf("getting workload definition %v %v %v %v", wURL, wOrg, wVersion, wArch)))

	var resp interface{}
	resp = new(GetWorkloadsResponse)

	// Figure out which version to filter the search with. Could be "".
	searchVersion, err := getSearchVersion(wVersion)
	if err != nil {
		return nil, "", err
	}

	// Search the exchange for the workload definition
	targetURL := fmt.Sprintf("%vorgs/%v/workloads?workloadUrl=%v&arch=%v", exURL, wOrg, wURL, wArch)
	if searchVersion != "" {
		targetURL = fmt.Sprintf("%vorgs/%v/workloads?workloadUrl=%v&version=%v&arch=%v", exURL, wOrg, wURL, searchVersion, wArch)
	}

	for {
		if err, tpErr := InvokeExchange(httpClientFactory.NewHTTPClient(nil), "GET", targetURL, id, token, nil, &resp); err != nil {
			glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
			return nil, "", err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			time.Sleep(10 * time.Second)
			continue
		} else {
			workloadMetadata := resp.(*GetWorkloadsResponse).Workloads

			// If the caller wanted a specific version, check for 1 result.
			if searchVersion != "" {
				if len(workloadMetadata) != 1 {
					glog.Errorf(rpclogString(fmt.Sprintf("expecting 1 result in GET workloads response: %v", resp)))
					return nil, "", errors.New(fmt.Sprintf("expecting 1 result, got %v", len(workloadMetadata)))
				} else {
					for wlId, workloadDef := range workloadMetadata {
						glog.V(3).Infof(rpclogString(fmt.Sprintf("returning workload definition %v", &workloadDef)))
						return &workloadDef, wlId, nil
					}
				}
			} else {
				if len(workloadMetadata) == 0 {
					glog.V(3).Infof(rpclogString(fmt.Sprintf("no workload definition found for %v", wURL)))
					return nil, "", nil
				}

				// The caller wants the highest version in the input version range. If no range was specified then
				// they will get the highest of all available versions.
				vRange, _ := policy.Version_Expression_Factory("0.0.0")
				if wVersion != "" {
					vRange, _ = policy.Version_Expression_Factory(wVersion)
				}

				highest := ""
				// resWDef has to be the object instead of pointer to the object because onece the pointer points to &wDef,
				// the content of it will get changed when the content of wDef gets changed in the loop
				var resWDef WorkloadDefinition
				var resWId string
				for wlId, wDef := range workloadMetadata {
					if inRange, err := vRange.Is_within_range(wDef.Version); err != nil {
						return nil, "", errors.New(fmt.Sprintf("unable to verify that %v is within %v, error %v", wDef.Version, vRange, err))
					} else if inRange {
						glog.V(5).Infof(rpclogString(fmt.Sprintf("found workload version %v within acceptable range", wDef.Version)))

						// cannot pass in "" in the CompareVersions because it checks for invalid version strings.
						var c int
						var err error
						if highest == "" {
							c, err = policy.CompareVersions("0.0.0", wDef.Version)
						} else {
							c, err = policy.CompareVersions(highest, wDef.Version)
						}

						if err != nil {
							glog.Errorf(rpclogString(fmt.Sprintf("error compairing version %v with version %v. %v", highest, wDef.Version, err)))
						} else if c == -1 {
							highest = wDef.Version
							resWDef = wDef
							resWId = wlId
						}
					}
				}

				if highest == "" {
					// when highest is empty, it means that there were no data in workloadMetadata, hence return nil.
					glog.V(3).Infof(rpclogString(fmt.Sprintf("returning workload definition %v for %v", nil, wURL)))
					return nil, "", nil
				} else {
					glog.V(3).Infof(rpclogString(fmt.Sprintf("returning workload definition %v for %v", resWDef, wURL)))
					return &resWDef, resWId, nil
				}
			}
		}
	}
}

// Get microservice and its exchange id for the given org, url, version and arch. If the the version string is version range, then the highest available microservice within the range will be returned.
func GetMicroservice(httpClientFactory *config.HTTPClientFactory, mURL string, mOrg string, mVersion string, mArch string, exURL string, id string, token string) (*MicroserviceDefinition, string, error) {

	glog.V(3).Infof(rpclogString(fmt.Sprintf("getting microservice definition %v %v %v %v", mURL, mOrg, mVersion, mArch)))

	var resp interface{}
	resp = new(GetMicroservicesResponse)

	// Figure out which version to filter the search with. Could be "".
	searchVersion, err := getSearchVersion(mVersion)
	if err != nil {
		return nil, "", err
	}

	// Search the exchange for the microservice definition
	targetURL := fmt.Sprintf("%vorgs/%v/microservices?specRef=%v&arch=%v", exURL, mOrg, mURL, mArch)
	if searchVersion != "" {
		targetURL = fmt.Sprintf("%vorgs/%v/microservices?specRef=%v&version=%v&arch=%v", exURL, mOrg, mURL, searchVersion, mArch)
	}

	for {
		if err, tpErr := InvokeExchange(httpClientFactory.NewHTTPClient(nil), "GET", targetURL, id, token, nil, &resp); err != nil {
			glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
			return nil, "", err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(5).Infof(rpclogString(fmt.Sprintf("found microservice %v.", resp)))
			msMetadata := resp.(*GetMicroservicesResponse).Microservices

			// If the caller wanted a specific version, check for 1 result.
			if searchVersion != "" {
				if len(msMetadata) != 1 {
					glog.Errorf(rpclogString(fmt.Sprintf("expecting 1 microservice %v %v %v response: %v", mURL, mOrg, mVersion, resp)))
					return nil, "", errors.New(fmt.Sprintf("expecting 1 microservice %v %v %v, got %v", mURL, mOrg, mVersion, len(msMetadata)))
				} else {
					for msId, msDef := range msMetadata {
						glog.V(3).Infof(rpclogString(fmt.Sprintf("returning microservice definition %v", &msDef)))
						return &msDef, msId, nil
					}
				}

			} else {
				if len(msMetadata) == 0 {
					return nil, "", errors.New(fmt.Sprintf("expecting at least 1 microservce %v %v %v, got %v", mURL, mOrg, mVersion, len(msMetadata)))
				}
				// The caller wants the highest version in the input version range. If no range was specified then
				// they will get the highest of all available versions.
				vRange, _ := policy.Version_Expression_Factory("0.0.0")
				if mVersion != "" {
					vRange, _ = policy.Version_Expression_Factory(mVersion)
				}

				highest := ""
				// resMsDef has to be the object instead of pointer to the object because onece the pointer points to &msDef,
				// the content of it will get changed when the content of msDef gets changed in the loop
				var resMsDef MicroserviceDefinition
				var resMsId string
				for msId, msDef := range msMetadata {
					if inRange, err := vRange.Is_within_range(msDef.Version); err != nil {
						return nil, "", errors.New(fmt.Sprintf("unable to verify that %v is within %v, error %v", msDef.Version, vRange, err))
					} else if inRange {
						glog.V(5).Infof(rpclogString(fmt.Sprintf("found microservice version %v within acceptable range", msDef.Version)))

						// cannot pass in "" in the CompareVersions because it checks for invalid version strings.
						var c int
						var err error

						if highest == "" {
							c, err = policy.CompareVersions("0.0.0", msDef.Version)
						} else {
							c, err = policy.CompareVersions(highest, msDef.Version)
						}
						if err != nil {
							glog.Errorf(rpclogString(fmt.Sprintf("error compairing version %v with version %v. %v", highest, msDef.Version, err)))
						} else if c == -1 {
							highest = msDef.Version
							resMsDef = msDef
							resMsId = msId
						}
					}
				}

				if highest == "" {
					// when highest is empty, it means that there were no data in msMetadata, hence return nil.
					glog.V(3).Infof(rpclogString(fmt.Sprintf("returning microservice definition %v for %v", nil, mURL)))
					return nil, "", nil
				} else {
					glog.V(3).Infof(rpclogString(fmt.Sprintf("returning microservice definition %v for %v", resMsDef, mURL)))
					return &resMsDef, resMsId, nil
				}
			}
		}
	}
}

// The purpose of this function is to verify that a given workload URL, version and architecture, is defined in the exchange
// as well as all of its API spec dependencies. This function also returns the API dependencies converted into
// policy types so that the caller can use those types to do policy compatibility checks if they want to.
func WorkloadResolver(httpClientFactory *config.HTTPClientFactory, wURL string, wOrg string, wVersion string, wArch string, exURL string, id string, token string) (*policy.APISpecList, *WorkloadDefinition, error) {
	resolveMicroservices := true

	glog.V(5).Infof(rpclogString(fmt.Sprintf("resolving workload %v %v %v %v", wURL, wOrg, wVersion, wArch)))

	res := new(policy.APISpecList)
	// Get a version specific workload definition.
	workload, _, werr := GetWorkload(httpClientFactory, wURL, wOrg, wVersion, wArch, exURL, id, token)
	if werr != nil {
		return nil, nil, werr
	} else if workload == nil {
		return nil, nil, errors.New(fmt.Sprintf("unable to find workload %v %v %v %v on the exchange.", wURL, wOrg, wVersion, wArch))
	} else if len(workload.Workloads) != 1 {
		return nil, nil, errors.New(fmt.Sprintf("expecting 1 element in the workloads array of %v, have %v", workload, len(workload.Workloads)))
	} else {

		// We found the workload definition. Microservices are referred to within a workload definition by
		// URL, architecture, and version range. Microservice definitions in the exchange arent queryable by version range,
		// so we will have to do the version filtering.  We're looking for the highest version microservice definition that
		// is within the range defined by the workload.  See ./policy/version.go for an explanation of version syntax and
		// version ranges. The GetMicroservices() function is smart enough to return the microservice we're looking for as
		// long as we give it a range to search within.

		if resolveMicroservices {
			glog.V(5).Infof(rpclogString(fmt.Sprintf("resolving microservices for %v %v %v %v", wURL, wOrg, wVersion, wArch)))
			for _, apiSpec := range workload.APISpecs {

				// Make sure the microservice has the same arch as the workload
				// Convert version to a version range expression (if it's not already an expression) so that GetMicroservice()
				// will return us something in the range required by the workload.
				if apiSpec.Arch != wArch {
					return nil, nil, errors.New(fmt.Sprintf("microservice %v has a different architecture from the workload.", apiSpec))
				} else if vExp, err := policy.Version_Expression_Factory(apiSpec.Version); err != nil {
					return nil, nil, errors.New(fmt.Sprintf("unable to create version expression from %v, error %v", apiSpec.Version, err))
				} else if ms, _, err := GetMicroservice(httpClientFactory, apiSpec.SpecRef, apiSpec.Org, vExp.Get_expression(), apiSpec.Arch, exURL, id, token); err != nil {
					return nil, nil, err
				} else if ms == nil {
					return nil, nil, errors.New(fmt.Sprintf("unable to find microservice %v within version range %v in the exchange.", apiSpec, vExp))
				} else {
					newAPISpec := policy.APISpecification_Factory(ms.SpecRef, apiSpec.Org, ms.Version, ms.Arch)
					if ms.Sharable == MS_SHARING_MODE_SINGLE {
						newAPISpec.ExclusiveAccess = false
					}
					(*res) = append((*res), (*newAPISpec))
				}
			}
			glog.V(5).Infof(rpclogString(fmt.Sprintf("resolved microservices for %v %v %v %v", wURL, wOrg, wVersion, wArch)))
		}
		glog.V(5).Infof(rpclogString(fmt.Sprintf("resolved workload %v %v %v %v", wURL, wOrg, wVersion, wArch)))
		return res, workload, nil

	}

}

// Functions and types for working with organizations in the exchange
type Organization struct {
	Label       string `json:"label"`
	Description string `json:"description"`
	LastUpdated string `json:"lastUpdated"`
}

type GetOrganizationResponse struct {
	Orgs      map[string]Organization `json:"orgs"`
	LastIndex int                     `json:"lastIndex"`
}

// Get the metadata for a specific organization.
func GetOrganization(httpClientFactory *config.HTTPClientFactory, org string, exURL string, id string, token string) (*Organization, error) {

	glog.V(3).Infof(rpclogString(fmt.Sprintf("getting organization definition %v", org)))

	var resp interface{}
	resp = new(GetOrganizationResponse)

	// Search the exchange for the organization definition
	targetURL := fmt.Sprintf("%vorgs/%v", exURL, org)

	for {
		if err, tpErr := InvokeExchange(httpClientFactory.NewHTTPClient(nil), "GET", targetURL, id, token, nil, &resp); err != nil {
			glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			time.Sleep(10 * time.Second)
			continue
		} else {
			orgs := resp.(*GetOrganizationResponse).Orgs
			if theOrg, ok := orgs[org]; !ok {
				return nil, errors.New(fmt.Sprintf("organization %v not found", org))
			} else {
				glog.V(3).Infof(rpclogString(fmt.Sprintf("found organization %v definition %v", org, theOrg)))
				return &theOrg, nil
			}
		}
	}

}

// Function and types related to working with patterns

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

type WorkloadReference struct {
	WorkloadURL      string           `json:"workloadUrl,omitempty"`      // refers to a workload definition in the exchange
	WorkloadOrg      string           `json:"workloadOrgid,omitempty"`    // the org holding the workload definition
	WorkloadArch     string           `json:"workloadArch,omitempty"`     // the hardware architecture of the workload definition
	WorkloadVersions []WorkloadChoice `json:"workloadVersions,omitempty"` // a list of workload version for rollback
	DataVerify       DataVerification `json:"dataVerification"`           // policy for verifying that the node is sending data
	NodeH            NodeHealth       `json:"nodeHealth"`                 // policy for determining when a node's health is violating its agreements
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

type Pattern struct {
	Owner              string              `json:"owner"`
	Label              string              `json:"label"`
	Description        string              `json:"description"`
	Public             bool                `json:"public"`
	Workloads          []WorkloadReference `json:"workloads"`
	AgreementProtocols []AgreementProtocol `json:"agreementProtocols"`
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
		glog.V(3).Infof(rpclogString(fmt.Sprintf("getting pattern definitions for %v and %v", org, pattern)))
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

	for {
		if err, tpErr := InvokeExchange(httpClientFactory.NewHTTPClient(nil), "GET", targetURL, id, token, nil, &resp); err != nil {
			glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			time.Sleep(10 * time.Second)
			continue
		} else {
			pats := resp.(*GetPatternResponse).Patterns
			glog.V(3).Infof(rpclogString(fmt.Sprintf("found patterns for %v, %v", org, pats)))
			return pats, nil
		}
	}

}

// Create a name for the generated policy that should be unique within the org.
func makePolicyName(patternName string, workloadURL string, workloadOrg string, workloadArch string) string {

	url := ""
	pieces := strings.SplitN(workloadURL, "/", 3)
	if len(pieces) >= 3 {
		url = strings.TrimSuffix(pieces[2], "/")
		url = strings.Replace(url, "/", "-", -1)
	}

	return fmt.Sprintf("%v_%v_%v_%v", patternName, url, workloadOrg, workloadArch)

}

// Convert a pattern to a list of policy objects. Each pattern contains 1 or more workloads,
// which will each be translated to a policy.
func ConvertToPolicies(patternId string, p *Pattern) ([]*policy.Policy, error) {

	name := GetId(patternId)

	policies := make([]*policy.Policy, 0, 10)

	// Each pattern contains a list of workloads that needs to be converted to a policy
	for _, workload := range p.Workloads {

		// make sure required fields are not empty
		if workload.WorkloadURL == "" || workload.WorkloadOrg == "" || workload.WorkloadArch == "" {
			return nil, fmt.Errorf("workloadUrl, workloadOrgid or workloadArch is empty string in pattern %v.", name)
		} else if workload.WorkloadVersions == nil || len(workload.WorkloadVersions) == 0 {
			return nil, fmt.Errorf("The workloadVersions array is empty in pattern %v.", name)
		}

		policyName := makePolicyName(name, workload.WorkloadURL, workload.WorkloadOrg, workload.WorkloadArch)

		pol := policy.Policy_Factory(fmt.Sprintf("%v", policyName))

		// Copy Agreement protocol metadata into the policy
		for _, agp := range p.AgreementProtocols {
			newAGP := policy.AgreementProtocol_Factory(agp.Name)
			newAGP.Initialize()
			for _, bc := range agp.Blockchains {
				newBC := policy.Blockchain_Factory(bc.Type, bc.Name, bc.Org)
				(&newAGP.Blockchains).Add_Blockchain(newBC)
			}
			pol.Add_Agreement_Protocol(newAGP)
		}

		// Copy workload metadata into the policy
		for _, wl := range workload.WorkloadVersions {
			if wl.Version == "" {
				return nil, fmt.Errorf("The version for workload %v arch %v is empty in pattern %v.", workload.WorkloadURL, workload.WorkloadArch, name)
			}
			newWL := policy.Workload_Factory(workload.WorkloadURL, workload.WorkloadOrg, wl.Version, workload.WorkloadArch)
			newWL.Priority = (*policy.Workload_Priority_Factory(wl.Priority.PriorityValue, wl.Priority.Retries, wl.Priority.RetryDurationS, wl.Priority.VerifiedDurationS))
			newWL.DeploymentOverrides = wl.DeploymentOverrides
			newWL.DeploymentOverridesSignature = wl.DeploymentOverridesSignature
			pol.Add_Workload(newWL)
		}

		// Copy Data Verification metadata into the policy
		if workload.DataVerify.Enabled {
			mp := policy.Meter{
				Tokens:                workload.DataVerify.Metering.Tokens,
				PerTimeUnit:           workload.DataVerify.Metering.PerTimeUnit,
				NotificationIntervalS: workload.DataVerify.Metering.NotificationIntervalS,
			}
			d := policy.DataVerification_Factory(workload.DataVerify.URL, workload.DataVerify.URLUser, workload.DataVerify.URLPassword, workload.DataVerify.Interval, workload.DataVerify.CheckRate, mp)
			pol.Add_DataVerification(d)
		}

		// Indicate that this is a pattern based policy file. Manually created policy files should not use this field.
		pol.PatternId = patternId

		// Unlimited number of devices can get this workload
		pol.MaxAgreements = 0

		// Copy over the node health policy
		nh := policy.NodeHealth_Factory(workload.NodeH.MissingHBInterval, workload.NodeH.CheckAgreementStatus)
		pol.Add_NodeHealth(nh)

		glog.V(3).Infof(rpclogString(fmt.Sprintf("converted %v into %v", workload, pol)))
		policies = append(policies, pol)

	}

	return policies, nil

}

// This section is for types related to querying the exchange for node health

type AgreementObject struct {
}

type NodeInfo struct {
	LastHeartbeat string                     `json:"lastHeartbeat"`
	Agreements    map[string]AgreementObject `json:"agreements"`
}

func (n NodeInfo) String() string {
	return fmt.Sprintf("LastHeartbeat: %v, Agreements: %v", n.LastHeartbeat, n.Agreements)
}

type NodeHealthStatus struct {
	Nodes map[string]NodeInfo `json:"nodes"`
}

type NodeHealthStatusRequest struct {
	LastCall string `json:"lastTime"`
}

// Return the current status of nodes in a given pattern. This function can return nil and no error if the exchange has no
// updated status to return.
func GetNodeHealthStatus(httpClientFactory *config.HTTPClientFactory, pattern string, org string, lastCallTime string, exURL string, id string, token string) (*NodeHealthStatus, error) {

	glog.V(3).Infof(rpclogString(fmt.Sprintf("getting node health status for %v", pattern)))

	params := &NodeHealthStatusRequest{
		LastCall: lastCallTime,
	}

	var resp interface{}
	resp = new(NodeHealthStatus)

	// Search the exchange for the node health status
	targetURL := fmt.Sprintf("%vorgs/%v/search/nodehealth", exURL, org)
	if pattern != "" {
		targetURL = fmt.Sprintf("%vorgs/%v/patterns/%v/nodehealth", exURL, GetOrg(pattern), GetId(pattern))
	}

	for {
		if err, tpErr := InvokeExchange(httpClientFactory.NewHTTPClient(nil), "POST", targetURL, id, token, &params, &resp); err != nil && !strings.Contains(err.Error(), "status: 404") {
			glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			time.Sleep(10 * time.Second)
			continue
		} else {
			status := resp.(*NodeHealthStatus)
			glog.V(3).Infof(rpclogString(fmt.Sprintf("found nodehealth status for %v, status %v", pattern, status)))
			return status, nil
		}
	}

}

// This function is used to invoke an exchange API
// For GET, the given resp parameter will be untouched when http returns code 404.
func InvokeExchange(httpClient *http.Client, method string, url string, user string, pw string, params interface{}, resp *interface{}) (error, error) {

	if len(method) == 0 {
		return errors.New(fmt.Sprintf("Error invoking exchange, method name must be specified")), nil
	} else if len(url) == 0 {
		return errors.New(fmt.Sprintf("Error invoking exchange, no URL to invoke")), nil
	} else if resp == nil {
		return errors.New(fmt.Sprintf("Error invoking exchange, response object must be specified")), nil
	}

	if reflect.ValueOf(params).Kind() == reflect.Ptr {
		paramValue := reflect.Indirect(reflect.ValueOf(params))
		glog.V(5).Infof(rpclogString(fmt.Sprintf("Invoking exchange %v at %v with %v", method, url, paramValue)))
	} else {
		glog.V(5).Infof(rpclogString(fmt.Sprintf("Invoking exchange %v at %v with %v", method, url, params)))
	}

	requestBody := bytes.NewBuffer(nil)
	if params != nil {
		if jsonBytes, err := json.Marshal(params); err != nil {
			return errors.New(fmt.Sprintf("Invocation of %v at %v with %v failed marshalling to json, error: %v", method, url, params, err)), nil
		} else {
			requestBody = bytes.NewBuffer(jsonBytes)
		}
	}
	if req, err := http.NewRequest(method, url, requestBody); err != nil {
		return errors.New(fmt.Sprintf("Invocation of %v at %v with %v failed creating HTTP request, error: %v", method, url, requestBody, err)), nil
	} else {
		req.Close = true // work around to ensure that Go doesn't get connections confused. Supposed to be fixed in Go 1.6.
		req.Header.Add("Accept", "application/json")
		if method != "GET" {
			req.Header.Add("Content-Type", "application/json")
		}
		if user != "" && pw != "" {
			req.Header.Add("Authorization", fmt.Sprintf("Basic %v", base64.StdEncoding.EncodeToString([]byte(user+":"+pw))))
		}
		glog.V(5).Infof(rpclogString(fmt.Sprintf("Invoking exchange with headers: %v", req.Header)))
		// If the exchange is down, this call will return an error.

		if httpResp, err := httpClient.Do(req); err != nil {
			if isTransportError(err) {
				return nil, errors.New(fmt.Sprintf("Invocation of %v at %v with %v failed invoking HTTP request, error: %v", method, url, requestBody, err))
			} else {
				return errors.New(fmt.Sprintf("Invocation of %v at %v with %v failed invoking HTTP request, error: %v", method, url, requestBody, err)), nil
			}
		} else {
			defer httpResp.Body.Close()

			var outBytes []byte
			var readErr error
			if httpResp.Body != nil {
				if outBytes, readErr = ioutil.ReadAll(httpResp.Body); err != nil {
					if isTransportError(err) {
						return nil, errors.New(fmt.Sprintf("Invocation of %v at %v failed reading response message, HTTP Status %v, error: %v", method, url, httpResp.StatusCode, readErr))
					} else {
						return errors.New(fmt.Sprintf("Invocation of %v at %v failed reading response message, HTTP Status %v, error: %v", method, url, httpResp.StatusCode, readErr)), nil
					}
				}
			}

			// Handle special case of server error
			if httpResp.StatusCode == http.StatusInternalServerError && strings.Contains(string(outBytes), "timed out") {
				return nil, errors.New(fmt.Sprintf("Invocation of %v at %v with %v failed invoking HTTP request, error: %v", method, url, requestBody, err))
			}

			if method == "GET" && httpResp.StatusCode != http.StatusOK {
				if httpResp.StatusCode == http.StatusNotFound {
					glog.V(5).Infof(rpclogString(fmt.Sprintf("Got %v. Response to %v at %v is %v", httpResp.StatusCode, method, url, string(outBytes))))
					return nil, nil
				} else {
					return errors.New(fmt.Sprintf("Invocation of %v at %v failed invoking HTTP request, status: %v, response: %v", method, url, httpResp.StatusCode, string(outBytes))), nil
				}
			} else if (method == "PUT" || method == "POST" || method == "PATCH") && httpResp.StatusCode != http.StatusCreated {
				return errors.New(fmt.Sprintf("Invocation of %v at %v failed invoking HTTP request, status: %v, response: %v", method, url, httpResp.StatusCode, string(outBytes))), nil
			} else if method == "DELETE" && httpResp.StatusCode != http.StatusNoContent {
				return errors.New(fmt.Sprintf("Invocation of %v at %v failed invoking HTTP request, status: %v, response: %v", method, url, httpResp.StatusCode, string(outBytes))), nil
			} else if method == "DELETE" {
				return nil, nil
			} else {
				out := string(outBytes)
				glog.V(5).Infof(rpclogString(fmt.Sprintf("Response to %v at %v is %v", method, url, out)))

				// no need to Unmarshal the string output
				switch (*resp).(type) {
				case string:
					*resp = out
					return nil, nil
				}

				if err := json.Unmarshal(outBytes, resp); err != nil {
					return errors.New(fmt.Sprintf("Unable to demarshal response %v from invocation of %v at %v, error: %v", out, method, url, err)), nil
				} else {
					if httpResp.StatusCode == http.StatusNotFound {
						glog.V(5).Infof(rpclogString(fmt.Sprintf(" ---- Got %v. Response to %v at %v is %v", httpResp.StatusCode, method, url, *resp)))
					}
					switch (*resp).(type) {
					case *PutDeviceResponse:
						return nil, nil

					case *PostDeviceResponse:
						pdresp := (*resp).(*PostDeviceResponse)
						if pdresp.Code != "ok" {
							return errors.New(fmt.Sprintf("Invocation of %v at %v with %v returned error message: %v", method, url, params, pdresp.Msg)), nil
						} else {
							return nil, nil
						}

					case *SearchExchangeMSResponse:
						return nil, nil

					case *SearchExchangePatternResponse:
						return nil, nil

					case *GetDevicesResponse:
						return nil, nil

					case *GetAgbotsResponse:
						return nil, nil

					case *AllDeviceAgreementsResponse:
						return nil, nil

					case *AllAgbotAgreementsResponse:
						return nil, nil

					case *GetDeviceMessageResponse:
						return nil, nil

					case *GetAgbotMessageResponse:
						return nil, nil

					case *GetEthereumClientResponse:
						return nil, nil

					case *GetWorkloadsResponse:
						return nil, nil

					case *GetMicroservicesResponse:
						return nil, nil

					case *GetOrganizationResponse:
						return nil, nil

					case *GetPatternResponse:
						return nil, nil

					case *GetAgbotsPatternsResponse:
						return nil, nil

					case *NodeHealthStatus:
						return nil, nil

					default:
						return errors.New(fmt.Sprintf("Unknown type of response object %v passed to invocation of %v at %v with %v", *resp, method, url, requestBody)), nil
					}
				}
			}
		}
	}
}

func isTransportError(err error) bool {
	l_error_string := strings.ToLower(err.Error())
	if strings.Contains(l_error_string, "time") && strings.Contains(l_error_string, "out") {
		return true
	} else if strings.Contains(l_error_string, "connection") && (strings.Contains(l_error_string, "refused") || strings.Contains(l_error_string, "reset")) {
		return true
	}
	return false
}

var rpclogString = func(v interface{}) string {
	return fmt.Sprintf("Exchange RPC %v", v)
}

func GetExchangeVersion(httpClientFactory *config.HTTPClientFactory, exchangeUrl string) (string, error) {

	glog.V(3).Infof(rpclogString("Get exchange version."))

	return "1.46.0", nil

	//var resp interface{}
	//resp = ""
	//targetURL := exchangeUrl + "admin/version"
	//for {
	//	if err, tpErr := InvokeExchange(httpClientFactory.NewHTTPClient(nil), "GET", targetURL, "", "", nil, &resp); err != nil {
	//		glog.Errorf(err.Error())
	//		return "", err
	//	} else if tpErr != nil {
	//		glog.Warningf(tpErr.Error())
	//		time.Sleep(10 * time.Second)
	//		continue
	//	} else {
	// remove last return charactor if any
	//		v := resp.(string)
	//		if strings.HasSuffix(v, "\n") {
	//			v = v[:len(v)-1]
	//		}

	//		return v, nil
	//	}
	//}
}

// This function gets the pattern/workload/microservice signing key names and their contents.
// The oType is one of PATTERN, MICROSERVICE, WORKLOAD defined in the begining of this file.
// When oType is PATTERN, the oURL is the pattern name and oVersion and oArch are ignored.
func GetObjectSigningKeys(httpClientFactory *config.HTTPClientFactory, oType string, oURL string, oOrg string, oVersion string, oArch string, exURL string, id string, token string) (map[string]string, error) {

	glog.V(3).Infof(rpclogString(fmt.Sprintf("getting %v signing keys for %v %v %v %v", oType, oURL, oOrg, oVersion, oArch)))

	// get object id and key target url
	var oIndex string
	var targetURL string

	switch oType {
	case PATTERN:
		pat_resp, err := GetPatterns(httpClientFactory, oOrg, oURL, exURL, id, token)
		if err != nil {
			return nil, errors.New(rpclogString(fmt.Sprintf("failed to get the pattern %v/%v.%v", oOrg, oURL, err)))
		} else if pat_resp == nil {
			return nil, errors.New(rpclogString(fmt.Sprintf("unable to find the pattern %v/%v.%v", oOrg, oURL, err)))
		}
		for id, _ := range pat_resp {
			oIndex = id
			targetURL = fmt.Sprintf("%vorgs/%v/patterns/%v/keys", exURL, oOrg, GetId(oIndex))
			break
		}

	case MICROSERVICE:
		if oVersion == "" || !policy.IsVersionString(oVersion) {
			return nil, errors.New(rpclogString(fmt.Sprintf("GetObjectSigningKeys got wrong version string %v. The version string should be a non-empy single version string.", oVersion)))
		}
		ms_resp, ms_id, err := GetMicroservice(httpClientFactory, oURL, oOrg, oVersion, oArch, exURL, id, token)
		if err != nil {
			return nil, errors.New(rpclogString(fmt.Sprintf("failed to get the microservice %v %v %v %v.%v", oURL, oOrg, oVersion, oArch, err)))
		} else if ms_resp == nil {
			return nil, errors.New(rpclogString(fmt.Sprintf("unable to find the microservice %v %v %v %v.", oURL, oOrg, oVersion, oArch)))
		}
		oIndex = ms_id
		targetURL = fmt.Sprintf("%vorgs/%v/microservices/%v/keys", exURL, oOrg, GetId(oIndex))

	case WORKLOAD:
		if oVersion == "" || !policy.IsVersionString(oVersion) {
			return nil, errors.New(rpclogString(fmt.Sprintf("GetObjectSigningKeys got wrong version string %v. The version string should be a non-empy single version string.", oVersion)))
		}
		wl_resp, wl_id, err := GetWorkload(httpClientFactory, oURL, oOrg, oVersion, oArch, exURL, id, token)
		if err != nil {
			return nil, errors.New(rpclogString(fmt.Sprintf("failed to get the workload %v %v %v %v. %v", oURL, oOrg, oVersion, oArch, err)))
		} else if wl_resp == nil {
			return nil, errors.New(rpclogString(fmt.Sprintf("unable to find the workload %v %v %v %v.", oURL, oOrg, oVersion, oArch)))
		}
		oIndex = wl_id
		targetURL = fmt.Sprintf("%vorgs/%v/workloads/%v/keys", exURL, oOrg, GetId(oIndex))

	default:
		return nil, errors.New(rpclogString(fmt.Sprintf("GetObjectSigningKeys received wrong type parameter: %v. It should be one of %v, %v and %v.", oType, PATTERN, MICROSERVICE, WORKLOAD)))
	}

	// get all the singining key names for the object
	var resp_KeyNames interface{}
	resp_KeyNames = ""

	key_names := make([]string, 0)

	for {
		if err, tpErr := InvokeExchange(httpClientFactory.NewHTTPClient(nil), "GET", targetURL, id, token, nil, &resp_KeyNames); err != nil {
			glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			time.Sleep(10 * time.Second)
			continue
		} else {
			if resp_KeyNames.(string) != "" {
				glog.V(5).Infof(rpclogString(fmt.Sprintf("found object signing keys %v.", resp_KeyNames)))
				if err := json.Unmarshal([]byte(resp_KeyNames.(string)), &key_names); err != nil {
					return nil, errors.New(fmt.Sprintf("Unable to demarshal pattern key list %v to string array, error: %v", resp_KeyNames, err))
				}
			}
			break
		}
	}

	// get the key contents
	ret := make(map[string]string)

	for _, key := range key_names {
		var resp_KeyContent interface{}
		resp_KeyContent = ""
		for {
			if err, tpErr := InvokeExchange(httpClientFactory.NewHTTPClient(nil), "GET", fmt.Sprintf("%v/%v", targetURL, key), id, token, nil, &resp_KeyContent); err != nil {
				glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
				return nil, err
			} else if tpErr != nil {
				glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
				time.Sleep(10 * time.Second)
				continue
			} else {
				if resp_KeyContent.(string) != "" {
					glog.V(5).Infof(rpclogString(fmt.Sprintf("found signing key content for key %v: %v.", key, resp_KeyContent)))
					ret[key] = resp_KeyContent.(string)
				} else {
					glog.Warningf(rpclogString(fmt.Sprintf("could not find key content for key %v", key)))
				}
				break
			}
		}
	}

	return ret, nil
}
