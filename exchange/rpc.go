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
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/semanticversion"
	"github.com/open-horizon/edge-sync-service/common"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const PATTERN = "pattern"
const SERVICE = "service"

const NOHEARTBEAT_PARAM = "noheartbeat=true"

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
	Version       string   `json:"version"`
	Properties    []MSProp `json:"properties"`
	NumAgreements int      `json:"numAgreements"`
	Policy        string   `json:"policy"`
	ConfigState   string   `json:"configState"`
}

func (m Microservice) String() string {
	return fmt.Sprintf("URL: %v, Version: %v, Properties: %v, NumAgreements: %v, Policy: %v, ConfigState: %v", m.Url, m.Version, m.Properties, m.NumAgreements, m.Policy, m.ConfigState)
}

func (m Microservice) ShortString() string {
	return fmt.Sprintf("URL: %v, Version: %v, NumAgreements: %v, ConfigState: %v", m.Url, m.Version, m.NumAgreements, m.ConfigState)
}

func (m Microservice) DeepCopy() *Microservice {
	svcCopy := Microservice{Url: m.Url, Version: m.Version, NumAgreements: m.NumAgreements, Policy: m.Policy, ConfigState: m.ConfigState}
	for _, msprop := range m.Properties {
		svcCopy.Properties = append(svcCopy.Properties, msprop)
	}
	return &svcCopy
}

// structs and types for working with microservice based exchange searches
type SearchExchangeMSRequest struct {
	DesiredServices    []Microservice `json:"desiredServices"`
	SecondsStale       int            `json:"secondsStale"`
	PropertiesToReturn []string       `json:"propertiesToReturn"`
	StartIndex         int            `json:"startIndex"`
	NumEntries         int            `json:"numEntries"`
}

func (a SearchExchangeMSRequest) String() string {
	return fmt.Sprintf("Services: %v, SecondsStale: %v, PropertiesToReturn: %v, StartIndex: %v, NumEntries: %v", a.DesiredServices, a.SecondsStale, a.PropertiesToReturn, a.StartIndex, a.NumEntries)
}

type SearchResultDevice struct {
	Id        string `json:"id"`
	NodeType  string `json:"nodeType"`
	PublicKey string `json:"publicKey"`
}

func (d SearchResultDevice) String() string {
	return fmt.Sprintf("Id: %v, NodeType: %v", d.Id, d.NodeType)
}

func (d SearchResultDevice) ShortString() string {
	return d.String()
}

func (d *SearchResultDevice) GetNodeType() string {
	if d.NodeType == "" {
		return persistence.DEVICE_TYPE_DEVICE
	} else {
		return d.NodeType
	}
}

// This function creates the exchange search message body.
func CreateSearchMSRequest() *SearchExchangeMSRequest {

	ser := &SearchExchangeMSRequest{
		StartIndex: 0,
		NumEntries: 100,
	}

	return ser
}

type HeartbeatIntervals struct {
	MinInterval        int `json:"minInterval"`
	MaxInterval        int `json:"maxInterval"`
	IntervalAdjustment int `json:"intervalAdjustment"`
}

// Structs and types for interacting with the device (node) object in the exchange
type Device struct {
	Token              string             `json:"token"`
	Name               string             `json:"name"`
	Owner              string             `json:"owner"`
	NodeType           string             `json:"nodeType"`
	Pattern            string             `json:"pattern"`
	RegisteredServices []Microservice     `json:"registeredServices"`
	MsgEndPoint        string             `json:"msgEndPoint"`
	SoftwareVersions   SoftwareVersion    `json:"softwareVersions"`
	LastHeartbeat      string             `json:"lastHeartbeat"`
	PublicKey          string             `json:"publicKey"`
	Arch               string             `json:"arch"`
	UserInput          []policy.UserInput `json:"userInput"`
	HeartbeatIntv      HeartbeatIntervals `json:"heartbeatIntervals,omitempty"`
	LastUpdated        string             `json:"lastUpdated,omitempty"`
}

func (d Device) String() string {
	return fmt.Sprintf("Name: %v, Owner: %v, NodeType: %v, Pattern: %v, LastHeartbeat: %v, RegisteredServices: %v, MsgEndPoint: %v, Arch: %v, UserInput: %v, HeartbeatIntv: %v", d.Name, d.Owner, d.NodeType, d.Pattern, d.LastHeartbeat, d.RegisteredServices, d.MsgEndPoint, d.Arch, d.UserInput, d.HeartbeatIntv)
}

func (d Device) ShortString() string {
	str := fmt.Sprintf("Name: %v, Owner: %v, NodeType: %v, Pattern %v, LastHeartbeat: %v, MsgEndPoint: %v, Arch: %v, HeartbeatIntv: %v", d.Name, d.Owner, d.NodeType, d.Pattern, d.LastHeartbeat, d.MsgEndPoint, d.Arch, d.HeartbeatIntv)
	for _, ms := range d.RegisteredServices {
		str += fmt.Sprintf("%v,", ms.Url)
	}
	return str
}

func (n Device) DeepCopy() *Device {
	nodeCopy := Device{Token: n.Token, Name: n.Name, Owner: n.Owner, NodeType: n.NodeType, Pattern: n.Pattern, MsgEndPoint: n.MsgEndPoint, LastHeartbeat: n.LastHeartbeat, PublicKey: n.PublicKey, Arch: n.Arch, HeartbeatIntv: n.HeartbeatIntv, LastUpdated: n.LastUpdated}
	if n.RegisteredServices == nil {
		nodeCopy.RegisteredServices = nil
	} else {
		for _, svc := range n.RegisteredServices {
			nodeCopy.RegisteredServices = append(nodeCopy.RegisteredServices, *svc.DeepCopy())
		}
	}
	if n.SoftwareVersions == nil {
		nodeCopy.SoftwareVersions = nil
	} else {
		nodeCopy.SoftwareVersions = *n.SoftwareVersions.DeepCopy()
	}
	if n.UserInput == nil {
		nodeCopy.UserInput = nil
	} else {
		for _, userInput := range n.UserInput {
			nodeCopy.UserInput = append(nodeCopy.UserInput, *userInput.DeepCopy())
		}
	}
	return &nodeCopy
}

func (d *Device) GetNodeType() string {
	if d.NodeType == "" {
		return persistence.DEVICE_TYPE_DEVICE
	} else {
		return d.NodeType
	}
}

type GetDevicesResponse struct {
	Devices   map[string]Device `json:"nodes"`
	LastIndex int               `json:"lastIndex"`
}

func GetExchangeDevice(httpClientFactory *config.HTTPClientFactory, deviceId string, credId string, credPasswd string, exchangeUrl string) (*Device, error) {

	glog.V(3).Infof(rpclogString(fmt.Sprintf("retrieving device %v from exchange", deviceId)))

	var resp interface{}
	resp = new(GetDevicesResponse)
	targetURL := exchangeUrl + "orgs/" + GetOrg(deviceId) + "/nodes/" + GetId(deviceId)

	if cachedResource := GetNodeFromCache(GetOrg(deviceId), GetId(deviceId)); cachedResource != nil {
		return cachedResource, nil
	}

	retryCount := httpClientFactory.RetryCount
	retryInterval := httpClientFactory.GetRetryInterval()
	for {
		if err, tpErr := InvokeExchange(httpClientFactory.NewHTTPClient(nil), "GET", targetURL, credId, credPasswd, nil, &resp); err != nil {
			glog.Errorf(err.Error())
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
			devs := resp.(*GetDevicesResponse).Devices
			if dev, there := devs[deviceId]; !there {
				return nil, errors.New(fmt.Sprintf("device %v not in GET response %v as expected", deviceId, devs))
			} else {
				glog.V(3).Infof(rpclogString(fmt.Sprintf("retrieved device %v from exchange %v", deviceId, dev.ShortString())))
				glog.V(5).Infof(rpclogString(fmt.Sprintf("device details for %v: %v", deviceId, dev)))
				UpdateCache(NodeCacheMapKey(GetOrg(deviceId), GetId(deviceId)), NODE_DEF_TYPE_CACHE, dev)
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

	cachedNode := DeleteCacheNodeWriteThru(GetOrg(deviceId), GetId(deviceId))

	retryCount := httpClientFactory.RetryCount
	retryInterval := httpClientFactory.GetRetryInterval()
	for {
		if err, tpErr := InvokeExchange(httpClientFactory.NewHTTPClient(nil), "PUT", targetURL, deviceId, deviceToken, pdr, &resp); err != nil {
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
			glog.V(3).Infof(rpclogString(fmt.Sprintf("put device %v to exchange %v", deviceId, pdr)))
			if cachedNode != nil {
				UpdateCacheNodePutWriteThru(GetOrg(deviceId), GetId(deviceId), cachedNode, pdr)
			}
			return resp.(*PutDeviceResponse), nil
		}
	}
}

// patch the the device
func PatchExchangeDevice(httpClientFactory *config.HTTPClientFactory, deviceId string, deviceToken string, exchangeUrl string, pdr *PatchDeviceRequest) error {
	// create PUT body
	var resp interface{}
	resp = new(PostDeviceResponse)
	targetURL := exchangeUrl + "orgs/" + GetOrg(deviceId) + "/nodes/" + GetId(deviceId)

	cachedNode := DeleteCacheNodeWriteThru(GetOrg(deviceId), GetId(deviceId))

	retryCount := httpClientFactory.RetryCount
	retryInterval := httpClientFactory.GetRetryInterval()
	for {
		if err, tpErr := InvokeExchange(httpClientFactory.NewHTTPClient(nil), "PATCH", targetURL, deviceId, deviceToken, pdr, &resp); err != nil {
			return err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			if httpClientFactory.RetryCount == 0 {
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			} else if retryCount == 0 {
				return fmt.Errorf("Exceeded %v retries for error: %v", httpClientFactory.RetryCount, tpErr)
			} else {
				retryCount--
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			}
		} else {
			glog.V(3).Infof(rpclogString(fmt.Sprintf("patch device %v to exchange %v", deviceId, pdr.ShortString())))
			if cachedNode != nil {
				UpdateCacheNodePatchWriteThru(GetOrg(deviceId), GetId(deviceId), cachedNode, pdr)
			}
			return nil
		}
	}
}

type NodeStatus struct {
	RunningServices string `json:"runningServices,omitempty"`
}

func (w NodeStatus) String() string {
	return fmt.Sprintf(
		"Running Services: %v",
		w.RunningServices)
}

func GetNodeStatus(ec ExchangeContext, deviceId string) (*NodeStatus, error) {

	glog.V(3).Infof(rpclogString(fmt.Sprintf("getting node status for %v.", deviceId)))

	// Get the node status object. There should only be 1.
	var resp interface{}
	resp = new(NodeStatus)

	targetURL := fmt.Sprintf("%vorgs/%v/nodes/%v/status", ec.GetExchangeURL(), GetOrg(deviceId), GetId(deviceId))

	retryCount := ec.GetHTTPFactory().RetryCount
	retryInterval := ec.GetHTTPFactory().GetRetryInterval()
	for {
		if err, tpErr := InvokeExchange(ec.GetHTTPFactory().NewHTTPClient(nil), "GET", targetURL, ec.GetExchangeId(), ec.GetExchangeToken(), nil, &resp); err != nil {
			glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			if ec.GetHTTPFactory().RetryCount == 0 {
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			} else if retryCount == 0 {
				return nil, fmt.Errorf("Exceeded %v retries for error: %v", ec.GetHTTPFactory().RetryCount, tpErr)
			} else {
				retryCount--
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			}
		} else {
			glog.V(5).Infof(rpclogString(fmt.Sprintf("returning node status %v for %v.", resp, deviceId)))
			nodeStatus := resp.(*NodeStatus)
			return nodeStatus, nil
		}
	}
}

type DeviceAgreement struct {
	Service          []MSAgreementState `json:"services"`
	State            string             `json:"state"`
	AgreementService WorkloadAgreement  `json:"agrService"`
	LastUpdated      string             `json:"lastUpdated,omitempty"`
}

func (a DeviceAgreement) String() string {
	return fmt.Sprintf("AgreementService: %v, Service: %v, State: %v, LastUpdated: %v", a.AgreementService, a.Service, a.State, a.LastUpdated)
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
	Org     string `json:"orgid,omitempty"` // the org of the pattern
	Pattern string `json:"pattern"`         // pattern - without the org prefix on it
	URL     string `json:"url,omitempty"`   // workload URL
}

type MSAgreementState struct {
	Org string `json:"orgid"`
	URL string `json:"url"`
}

type PutAgreementState struct {
	State            string             `json:"state"`
	Services         []MSAgreementState `json:"services,omitempty"`
	AgreementService WorkloadAgreement  `json:"agreementService,omitempty"`
}

func (p PutAgreementState) String() string {
	return fmt.Sprintf("State: %v, Services: %v, AgreementService: %v", p.State, p.Services, p.AgreementService)
}

type SoftwareVersion map[string]string

func (s SoftwareVersion) DeepCopy() *SoftwareVersion {
	swVersCopy := make(SoftwareVersion, len(s))
	for key, val := range s {
		swVersCopy[key] = val
	}
	return &swVersCopy
}

type PutDeviceRequest struct {
	Token              string             `json:"token"`
	Name               string             `json:"name"`
	NodeType           string             `json:"nodeType"`
	Pattern            string             `json:"pattern"`
	RegisteredServices []Microservice     `json:"registeredServices"`
	MsgEndPoint        string             `json:"msgEndPoint"`
	SoftwareVersions   SoftwareVersion    `json:"softwareVersions"`
	PublicKey          []byte             `json:"publicKey"`
	Arch               string             `json:"arch"`
	UserInput          []policy.UserInput `json:"userInput"`
}

func (p PutDeviceRequest) String() string {
	return fmt.Sprintf("Token: %v, Name: %v, NodeType: %v, RegisteredServices %v, MsgEndPoint %v, SoftwareVersions %v, PublicKey %x, Arch: %v, UserInput: %v", "*****", p.Name, p.NodeType, p.RegisteredServices, p.MsgEndPoint, p.SoftwareVersions, p.PublicKey, p.Arch, p.UserInput)
}

func (p PutDeviceRequest) ShortString() string {
	str := fmt.Sprintf("Token: %v, Name: %v, NodeType: %v, MsgEndPoint %v, Arch: %v, SoftwareVersions %v", "*****", p.Name, p.NodeType, p.MsgEndPoint, p.Arch, p.SoftwareVersions)
	str += ", Service URLs: "
	for _, ms := range p.RegisteredServices {
		str += fmt.Sprintf("%v %v,", ms.Url, ms.Version)
	}
	return str
}

// Please patch one field at a time.
type PatchDeviceRequest struct {
	UserInput          *[]policy.UserInput `json:"userInput,omitempty"`
	Pattern            *string             `json:"pattern,omitempty"`
	Arch               *string             `json:"arch,omitempty"`
	RegisteredServices *[]Microservice     `json:"registeredServices,omitempty"`
}

func (p PatchDeviceRequest) String() string {
	pattern := "nil"
	if p.Pattern != nil {
		pattern = *p.Pattern
	}
	arch := "nil"
	if p.Arch != nil {
		arch = *p.Arch
	}
	return fmt.Sprintf("UserInput: %v, RegisteredServices: %v, Pattern: %v, Arch: %v", p.UserInput, p.RegisteredServices, pattern, arch)
}

func (p PatchDeviceRequest) ShortString() string {
	var registeredServices []string
	if p.RegisteredServices != nil {
		registeredServices = []string{}
		for _, ms := range *p.RegisteredServices {
			registeredServices = append(registeredServices, ms.ShortString())
		}
	}

	var userInput []string
	if p.UserInput != nil {
		userInput = []string{}
		for _, ui := range *p.UserInput {
			userInput = append(userInput, ui.ShortString())
		}
	}

	pattern := "nil"
	if p.Pattern != nil {
		pattern = *p.Pattern
	}
	arch := "nil"
	if p.Arch != nil {
		arch = *p.Arch
	}

	return fmt.Sprintf("UserInput: %v, RegisteredServices: %v, Pattern: %v, Arch: %v", userInput, registeredServices, pattern, arch)
}

type PostMessage struct {
	Message []byte `json:"message"`
	TTL     int    `json:"ttl"`
}

func (p PostMessage) String() string {
	return fmt.Sprintf("TTL: %v, Message: %x...", p.TTL, cutil.TruncateDisplayString(string(p.Message), 32))
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
	return fmt.Sprintf("MsgId: %v, AgbotId: %v, AgbotPubKey %v, Message %v, TimeSent %v", d.MsgId, d.AgbotId, d.AgbotPubKey, cutil.TruncateDisplayString(string(d.Message), 32), d.TimeSent)
}

type GetDeviceMessageResponse struct {
	Messages  []DeviceMessage `json:"messages"`
	LastIndex int             `json:"lastIndex"`
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
		Arch:             "",
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

func Heartbeat(httpClientFactory *config.HTTPClientFactory, url string, id string, token string) error {

	glog.V(5).Infof(rpclogString(fmt.Sprintf("Heartbeating to exchange: %v", url)))

	var resp interface{}
	resp = new(PostDeviceResponse)

	retryCount := httpClientFactory.RetryCount
	retryInterval := httpClientFactory.GetRetryInterval()

	for {
		if err, tpErr := InvokeExchange(httpClientFactory.NewHTTPClient(nil), "POST", url, id, token, nil, &resp); err != nil {
			glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
			return err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			if httpClientFactory.RetryCount == 0 {
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			} else if retryCount == 0 {
				return tpErr
			} else {
				retryCount--
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			}
		} else {
			glog.V(5).Infof(rpclogString(fmt.Sprintf("Sent heartbeat %v: %v", url, resp)))
			break
		}
	}
	return nil

}

func ConvertPropertyToExchangeFormat(prop *externalpolicy.Property) (*MSProp, error) {
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

// Functions and types for working with organizations in the exchange
type OrgLimits struct {
	MaxNodes int `json:"maxNodes"`
}

func (o OrgLimits) String() string {
	return fmt.Sprintf("MaxNodes: %v", o.MaxNodes)
}

type Organization struct {
	Label         string              `json:"label,omitempty"`
	Description   string              `json:"description,omitempty"`
	Tags          map[string]string   `json:"tags,omitempty"`
	HeartbeatIntv *HeartbeatIntervals `json:"heartbeatIntervals,omitempty"`
	Limits        *OrgLimits          `json:"limits,omitempty"`
	LastUpdated   string              `json:"lastUpdated,omitempty"`
}

func (o Organization) String() string {
	return fmt.Sprintf("Label: %v, Description: %v, Tags %v, HeartbeatIntv %v, Limits %v", o.Label, o.Description, o.Tags, o.HeartbeatIntv, o.Limits)
}

type GetOrganizationResponse struct {
	Orgs      map[string]Organization `json:"orgs"`
	LastIndex int                     `json:"lastIndex"`
}

// Get the metadata for a specific organization.
func GetOrganization(httpClientFactory *config.HTTPClientFactory, org string, exURL string, id string, token string) (*Organization, error) {

	glog.V(3).Infof(rpclogString(fmt.Sprintf("getting organization definition %v", org)))

	if orgDef := GetOrgDefFromCache(org); orgDef != nil {
		return orgDef, nil
	}

	var resp interface{}
	resp = new(GetOrganizationResponse)

	// Search the exchange for the organization definition
	targetURL := fmt.Sprintf("%vorgs/%v", exURL, org)

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
			orgs := resp.(*GetOrganizationResponse).Orgs
			if theOrg, ok := orgs[org]; !ok {
				return nil, errors.New(fmt.Sprintf("organization %v not found", org))
			} else {
				glog.V(3).Infof(rpclogString(fmt.Sprintf("found organization %v definition %v", org, theOrg)))
				UpdateCache(org, ORG_DEF_TYPE_CACHE, theOrg)
				return &theOrg, nil
			}
		}
	}

}

type UserDefinition struct {
	Password    string `json:"password"`
	Admin       bool   `json:"admin"`
	Email       string `json:"email"`
	LastUpdated string `json:"lastUpdated,omitempty"`
}

type GetUsersResponse struct {
	Users     map[string]UserDefinition `json:"users"`
	LastIndex int                       `json:"lastIndex"`
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
	NodeOrgIds []string `json:"nodeOrgids,omitempty"`
	LastCall   string   `json:"lastTime"`
}

// Return the current status of nodes in a given pattern. This function can return nil and no error if the exchange has no
// updated status to return.
func GetNodeHealthStatus(httpClientFactory *config.HTTPClientFactory, pattern string, org string, nodeOrgs []string, lastCallTime string, exURL string, id string, token string) (*NodeHealthStatus, error) {

	glog.V(3).Infof(rpclogString(fmt.Sprintf("getting node health status for %v", pattern)))

	// to save time, do not make a rpc call if the nodeOrgs is empty
	if len(nodeOrgs) == 0 {
		var nh NodeHealthStatus
		nh.Nodes = make(map[string]NodeInfo, 0)
		return &nh, nil
	}

	params := &NodeHealthStatusRequest{
		NodeOrgIds: nodeOrgs,
		LastCall:   lastCallTime,
	}

	var resp interface{}
	resp = new(NodeHealthStatus)

	// Search the exchange for the node health status
	targetURL := fmt.Sprintf("%vorgs/%v/search/nodehealth", exURL, org)
	if pattern != "" {
		targetURL = fmt.Sprintf("%vorgs/%v/patterns/%v/nodehealth", exURL, GetOrg(pattern), GetId(pattern))
	}

	retryCount := httpClientFactory.RetryCount
	retryInterval := httpClientFactory.GetRetryInterval()
	for {
		if err, tpErr := InvokeExchange(httpClientFactory.NewHTTPClient(nil), "POST", targetURL, id, token, &params, &resp); err != nil && !strings.Contains(err.Error(), "status: 404") {
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
			status := resp.(*NodeHealthStatus)
			glog.V(3).Infof(rpclogString(fmt.Sprintf("found nodehealth status for %v, status %v", pattern, status)))
			return status, nil
		}
	}

}

type ExchangeSurfaceError struct {
	ErrorList []persistence.SurfaceError `json:"errors"`
}

func GetSurfaceErrors(ec ExchangeContext, deviceId string) (*ExchangeSurfaceError, error) {
	var resp interface{}
	resp = new(ExchangeSurfaceError)

	targetURL := fmt.Sprintf("%vorgs/%v/nodes/%v/errors", ec.GetExchangeURL(), GetOrg(deviceId), GetId(deviceId))

	retryCount := ec.GetHTTPFactory().RetryCount
	retryInterval := ec.GetHTTPFactory().GetRetryInterval()
	for {
		if err, tpErr := InvokeExchange(ec.GetHTTPFactory().NewHTTPClient(nil), "GET", targetURL, ec.GetExchangeId(), ec.GetExchangeToken(), nil, &resp); err != nil {
			glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			if ec.GetHTTPFactory().RetryCount == 0 {
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			} else if retryCount == 0 {
				return nil, fmt.Errorf("Exceeded %v retries for error: %v", ec.GetHTTPFactory().RetryCount, tpErr)
			} else {
				retryCount--
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			}
		} else {
			glog.V(5).Infof(rpclogString(fmt.Sprintf("returning node surface errors %v for %v.", resp, deviceId)))
			surfaceErrors := resp.(*ExchangeSurfaceError)

			return surfaceErrors, nil
		}
	}
}

func PutSurfaceErrors(ec ExchangeContext, deviceId string, errorList *ExchangeSurfaceError) (*PutDeviceResponse, error) {
	var resp interface{}
	resp = new(PutDeviceResponse)

	targetURL := fmt.Sprintf("%vorgs/%v/nodes/%v/errors", ec.GetExchangeURL(), GetOrg(deviceId), GetId(deviceId))

	retryCount := ec.GetHTTPFactory().RetryCount
	retryInterval := ec.GetHTTPFactory().GetRetryInterval()
	for {
		if err, tpErr := InvokeExchange(ec.GetHTTPFactory().NewHTTPClient(nil), "PUT", targetURL, ec.GetExchangeId(), ec.GetExchangeToken(), errorList, &resp); err != nil {
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			if ec.GetHTTPFactory().RetryCount == 0 {
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			} else if retryCount == 0 {
				return nil, fmt.Errorf("Exceeded %v retries for error: %v", ec.GetHTTPFactory().RetryCount, tpErr)
			} else {
				retryCount--
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			}
		} else {
			glog.V(3).Infof(rpclogString(fmt.Sprintf("put node surface errors for %v to exchange %v", deviceId, errorList)))
			return resp.(*PutDeviceResponse), nil
		}
	}
}

func DeleteSurfaceErrors(ec ExchangeContext, deviceId string) error {
	var resp interface{}
	resp = new(PostDeviceResponse)

	targetURL := fmt.Sprintf("%vorgs/%v/nodes/%v/errors", ec.GetExchangeURL(), GetOrg(deviceId), GetId(deviceId))

	retryCount := ec.GetHTTPFactory().RetryCount
	retryInterval := ec.GetHTTPFactory().GetRetryInterval()

	for {
		if err, tpErr := InvokeExchange(ec.GetHTTPFactory().NewHTTPClient(nil), "DELETE", targetURL, ec.GetExchangeId(), ec.GetExchangeToken(), nil, &resp); err != nil && !strings.Contains(err.Error(), "status: 404") {
			return err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			if ec.GetHTTPFactory().RetryCount == 0 {
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			} else if retryCount == 0 {
				return fmt.Errorf("Exceeded %v retries for error: %v", ec.GetHTTPFactory().RetryCount, tpErr)
			} else {
				retryCount--
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			}
		} else {
			glog.V(3).Infof(rpclogString(fmt.Sprintf("deleted node surface errors for %v to exchange", deviceId)))
			return nil
		}
	}

}

// This function is used to invoke an exchange API
// For GET, the given resp parameter will be untouched when http returns code 404.
func InvokeExchange(httpClient *http.Client, method string, urlPath string, user string, pw string, params interface{}, resp *interface{}) (error, error) {

	if len(method) == 0 {
		return errors.New(fmt.Sprintf("Error invoking exchange, method name must be specified")), nil
	} else if len(urlPath) == 0 {
		return errors.New(fmt.Sprintf("Error invoking exchange, no URL to invoke")), nil
	} else if resp == nil {
		return errors.New(fmt.Sprintf("Error invoking exchange, response object must be specified")), nil
	}

	// encode the url so that it can accept unicode
	urlObj, err := url.Parse(urlPath)
	if err != nil {
		return errors.New(fmt.Sprintf("Error invoking exchange, malformed URL: %v. %v", urlPath, err)), nil
	}
	urlObj.RawQuery = urlObj.Query().Encode()

	if reflect.ValueOf(params).Kind() == reflect.Ptr {
		paramValue := reflect.Indirect(reflect.ValueOf(params))
		glog.V(3).Infof(rpclogString(fmt.Sprintf("Invoking exchange %v at %v with %v", method, urlPath, paramValue)))
		//fmt.Printf("Invoking exchange %v at %v with %v\n", method, urlPath, paramValue)
	} else {
		glog.V(3).Infof(rpclogString(fmt.Sprintf("Invoking exchange %v at %v with %v", method, urlPath, params)))
		//fmt.Printf("Invoking exchange %v at %v with %v\n", method, urlPath, params)
	}

	requestBody := bytes.NewBuffer(nil)
	if params != nil {
		if jsonBytes, err := json.Marshal(params); err != nil {
			return errors.New(fmt.Sprintf("Invocation of %v at %v with %v failed marshalling to json, error: %v", method, urlPath, params, err)), nil
		} else {
			requestBody = bytes.NewBuffer(jsonBytes)
		}
	}

	if req, err := http.NewRequest(method, urlObj.String(), requestBody); err != nil {
		return errors.New(fmt.Sprintf("Invocation of %v at %v with %v failed creating HTTP request, error: %v", method, urlPath, requestBody, err)), nil
	} else {
		req.Close = true // work around to ensure that Go doesn't get connections confused. Supposed to be fixed in Go 1.6.
		req.Header.Add("Accept", "application/json")
		if method != "GET" {
			req.Header.Add("Content-Type", "application/json")
		}
		if user != "" && pw != "" {
			req.Header.Add("Authorization", fmt.Sprintf("Basic %v", base64.StdEncoding.EncodeToString([]byte(user+":"+pw))))
		}

		// If the exchange is down, this call will return an error.
		httpResp, err := httpClient.Do(req)
		if httpResp != nil && httpResp.Body != nil {
			defer httpResp.Body.Close()
		}
		if IsTransportError(httpResp, err) {
			status := ""
			if httpResp != nil {
				status = httpResp.Status
			}
			return nil, errors.New(fmt.Sprintf("Invocation of %v at %v with %v failed invoking HTTP request, error: %v, HTTP Status: %v", method, urlPath, requestBody, err, status))
		} else if err != nil {
			return errors.New(fmt.Sprintf("Invocation of %v at %v with %v failed invoking HTTP request, error: %v", method, urlPath, requestBody, err)), nil
		} else {
			var outBytes []byte
			var readErr error
			if httpResp.Body != nil {
				if outBytes, readErr = ioutil.ReadAll(httpResp.Body); readErr != nil {
					return errors.New(fmt.Sprintf("Invocation of %v at %v failed reading response message, HTTP Status %v, error: %v", method, urlPath, httpResp.Status, readErr)), nil
				}
			}

			// Handle special case of server error
			if httpResp.StatusCode == http.StatusInternalServerError && strings.Contains(string(outBytes), "timed out") {
				return nil, errors.New(fmt.Sprintf("Invocation of %v at %v with %v failed invoking HTTP request, error: %v", method, urlPath, requestBody, err))
			}

			if method == "GET" && httpResp.StatusCode != http.StatusOK {
				if httpResp.StatusCode == http.StatusNotFound {
					glog.V(5).Infof(rpclogString(fmt.Sprintf("Got %v. Response to %v at %v is %v", httpResp.StatusCode, method, urlPath, string(outBytes))))
					return nil, nil
				} else {
					return errors.New(fmt.Sprintf("Invocation of %v at %v failed invoking HTTP request, status: %v, response: %v", method, urlPath, httpResp.StatusCode, string(outBytes))), nil
				}
			} else if (method == "PUT" || method == "POST" || method == "PATCH") && ((httpResp.StatusCode != http.StatusCreated && httpResp.StatusCode != http.StatusNoContent && httpResp.StatusCode != http.StatusConflict) || (httpResp.StatusCode == http.StatusConflict && !strings.Contains(urlPath, "business/policies/"))) {
				return errors.New(fmt.Sprintf("Invocation of %v at %v failed invoking HTTP request, status: %v, response: %v", method, urlPath, httpResp.StatusCode, string(outBytes))), nil
			} else if method == "DELETE" && httpResp.StatusCode != http.StatusNoContent {
				return errors.New(fmt.Sprintf("Invocation of %v at %v failed invoking HTTP request, status: %v, response: %v", method, urlPath, httpResp.StatusCode, string(outBytes))), nil
			} else if (method == "DELETE") || ((method == "PUT" || method == "POST" || method == "PATCH") && httpResp.StatusCode == http.StatusNoContent) {
				return nil, nil
			} else {
				out := string(outBytes)
				glog.V(6).Infof(rpclogString(fmt.Sprintf("Response to %v at %v is %v", method, urlPath, out)))

				// no need to Unmarshal the string output
				switch (*resp).(type) {
				case string:
					*resp = out
					return nil, nil
				}

				if err := json.Unmarshal(outBytes, resp); err != nil {
					return errors.New(fmt.Sprintf("Unable to demarshal response %v from invocation of %v at %v, error: %v", out, method, urlPath, err)), nil
				} else {
					if httpResp.StatusCode == http.StatusNotFound {
						glog.V(5).Infof(rpclogString(fmt.Sprintf("Got %v. Response to %v at %v is %v", httpResp.StatusCode, method, urlPath, *resp)))
					}
					switch (*resp).(type) {
					case *PutDeviceResponse:
						return nil, nil

					case *PostDeviceResponse:
						pdresp := (*resp).(*PostDeviceResponse)
						if pdresp.Code != "ok" {
							return errors.New(fmt.Sprintf("Invocation of %v at %v with %v returned error message: %v", method, urlPath, params, pdresp.Msg)), nil
						} else {
							return nil, nil
						}

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

					case *GetServicesResponse:
						return nil, nil

					case *GetOrganizationResponse:
						return nil, nil

					case *GetUsersResponse:
						return nil, nil

					case *GetPatternResponse:
						return nil, nil

					case *GetAgbotsPatternsResponse:
						return nil, nil

					case *NodeHealthStatus:
						return nil, nil

					case *ExchangePolicy:
						return nil, nil

					case *GetBusinessPolicyResponse:
						return nil, nil

					case *SearchExchBusinessPolResponse:
						return nil, nil

					case *GetAgbotsBusinessPolsResponse:
						return nil, nil

					case *ObjectDestinationPolicies:
						return nil, nil

					case *common.MetaData:
						return nil, nil

					case *ObjectDestinationStatuses:
						return nil, nil

					case *ExchangeSurfaceError:
						return nil, nil

					case *ExchangeChanges:
						return nil, nil

					case *ExchangeChangeIDResponse:
						return nil, nil

					case *NodeStatus:
						return nil, nil

					case *VaultSecretExistsResponse:
						return nil, nil

					default:
						return errors.New(fmt.Sprintf("Unknown type of response object %v (%T) passed to invocation of %v at %v with %v", *resp, *resp, method, urlPath, requestBody)), nil
					}
				}
			}
		}
	}
}

func IsTransportError(pResp *http.Response, err error) bool {
	if err != nil {
		if strings.Contains(err.Error(), ": EOF") {
			return true
		}

		l_error_string := strings.ToLower(err.Error())
		if strings.Contains(l_error_string, "time") && strings.Contains(l_error_string, "out") {
			return true
		} else if strings.Contains(l_error_string, "connection") && (strings.Contains(l_error_string, "refused") || strings.Contains(l_error_string, "reset")) {
			return true
		}
	}

	if pResp != nil {
		if pResp.StatusCode == http.StatusBadGateway {
			// 502: bad gateway error
			return true
		} else if pResp.StatusCode == http.StatusGatewayTimeout {
			// 504: gateway timeout
			return true
		} else if pResp.StatusCode == http.StatusServiceUnavailable {
			//503: service unavailable
			return true
		}
	}
	return false
}

var rpclogString = func(v interface{}) string {
	return fmt.Sprintf("Exchange RPC %v", v)
}

func GetExchangeVersion(httpClientFactory *config.HTTPClientFactory, exchangeUrl string, id string, token string) (string, error) {

	glog.V(3).Infof(rpclogString("Get exchange version."))

	var resp interface{}
	resp = ""
	targetURL := exchangeUrl + "admin/version"

	// remove trailing slash from the url for a consistent cache key
	cacheKey := strings.TrimSuffix(exchangeUrl, "/")
	if exchVers := GetExchangeVersionFromCache(cacheKey); strings.TrimSpace(exchVers) != "" {
		return exchVers, nil
	}

	retryCount := httpClientFactory.RetryCount
	retryInterval := httpClientFactory.GetRetryInterval()
	for {
		if err, tpErr := InvokeExchange(httpClientFactory.NewHTTPClient(nil), "GET", targetURL, id, token, nil, &resp); err != nil {
			glog.Errorf(err.Error())
			return "", err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			if httpClientFactory.RetryCount == 0 {
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			} else if retryCount == 0 {
				return "", fmt.Errorf("Exceeded %v retries for error: %v", httpClientFactory.RetryCount, tpErr)
			} else {
				retryCount--
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			}
		} else {
			// remove last return charactor if any
			v := resp.(string)
			if strings.HasSuffix(v, "\n") {
				v = v[:len(v)-1]
			}

			UpdateCache(cacheKey, EXCH_VERS_TYPE_CACHE, v)

			return v, nil
		}
	}
}

// This function gets the pattern/service signing key names and their contents. The oType is one of PATTERN, or SERVICE
// defined in the beginning of this file. When oType is PATTERN, the oURL is the pattern name and oVersion and oArch are ignored.
func GetObjectSigningKeys(ec ExchangeContext, oType string, oURL string, oOrg string, oVersion string, oArch string) (map[string]string, error) {

	glog.V(3).Infof(rpclogString(fmt.Sprintf("getting %v signing keys for %v %v %v %v", oType, oURL, oOrg, oVersion, oArch)))

	// get object id and key target url
	var oIndex string
	var targetURL string

	switch oType {
	case PATTERN:
		pat_resp, err := GetPatterns(ec.GetHTTPFactory(), oOrg, oURL, ec.GetExchangeURL(), ec.GetExchangeId(), ec.GetExchangeToken())
		if err != nil {
			return nil, errors.New(rpclogString(fmt.Sprintf("failed to get the pattern %v/%v.%v", oOrg, oURL, err)))
		} else if pat_resp == nil {
			return nil, errors.New(rpclogString(fmt.Sprintf("unable to find the pattern %v/%v.%v", oOrg, oURL, err)))
		}
		for id, _ := range pat_resp {
			oIndex = id
			targetURL = fmt.Sprintf("%vorgs/%v/patterns/%v/keys", ec.GetExchangeURL(), oOrg, GetId(oIndex))
			break
		}

	case SERVICE:
		if oVersion == "" || !semanticversion.IsVersionString(oVersion) {
			return nil, errors.New(rpclogString(fmt.Sprintf("GetObjectSigningKeys got wrong version string %v. The version string should be a non-empy single version string.", oVersion)))
		}
		ms_resp, ms_id, err := GetService(ec, oURL, oOrg, oVersion, oArch)
		if err != nil {
			return nil, errors.New(rpclogString(fmt.Sprintf("failed to get the service %v %v %v %v.%v", oURL, oOrg, oVersion, oArch, err)))
		} else if ms_resp == nil {
			return nil, errors.New(rpclogString(fmt.Sprintf("unable to find the service %v %v %v %v.", oURL, oOrg, oVersion, oArch)))
		}

		oIndex = ms_id
		cachedKeys := GetServiceKeysFromCache(oIndex)
		if cachedKeys != nil {
			return *cachedKeys, nil
		}

		targetURL = fmt.Sprintf("%vorgs/%v/services/%v/keys", ec.GetExchangeURL(), oOrg, GetId(oIndex))

	default:
		return nil, errors.New(rpclogString(fmt.Sprintf("GetObjectSigningKeys received wrong type parameter: %v. It should be one of %v, or %v.", oType, PATTERN, SERVICE)))
	}

	// get all the signing key names for the object
	var resp_KeyNames interface{}
	resp_KeyNames = ""

	key_names := make([]string, 0)

	retryCount := ec.GetHTTPFactory().RetryCount
	retryInterval := ec.GetHTTPFactory().GetRetryInterval()
	for {
		if err, tpErr := InvokeExchange(ec.GetHTTPFactory().NewHTTPClient(nil), "GET", targetURL, ec.GetExchangeId(), ec.GetExchangeToken(), nil, &resp_KeyNames); err != nil {
			glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			if ec.GetHTTPFactory().RetryCount == 0 {
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			} else if retryCount == 0 {
				return nil, fmt.Errorf("Exceeded %v retries for error: %v", ec.GetHTTPFactory().RetryCount, tpErr)
			} else {
				retryCount--
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			}
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

		retryCount := ec.GetHTTPFactory().RetryCount
		retryInterval := ec.GetHTTPFactory().GetRetryInterval()
		for {
			if err, tpErr := InvokeExchange(ec.GetHTTPFactory().NewHTTPClient(nil), "GET", fmt.Sprintf("%v/%v", targetURL, key), ec.GetExchangeId(), ec.GetExchangeToken(), nil, &resp_KeyContent); err != nil {
				glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
				return nil, err
			} else if tpErr != nil {
				glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
				if ec.GetHTTPFactory().RetryCount == 0 {
					time.Sleep(time.Duration(retryInterval) * time.Second)
					continue
				} else if retryCount == 0 {
					return nil, fmt.Errorf("Exceeded %v retries for error: %v", ec.GetHTTPFactory().RetryCount, tpErr)
				} else {
					retryCount--
					time.Sleep(time.Duration(retryInterval) * time.Second)
					continue
				}
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

	if oType == SERVICE {
		UpdateCache(oIndex, SVC_KEY_TYPE_CACHE, ret)
	}

	return ret, nil
}
