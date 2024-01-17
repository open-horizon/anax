package exchange

import (
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"strings"
	"time"
)

// Structs and types for interacting with the device (node) object in the exchange
type Device struct {
	Token              string             `json:"token"`
	Name               string             `json:"name"`
	Owner              string             `json:"owner"`
	NodeType           string             `json:"nodeType"`
	ClusterNamespace   string             `json:"clusterNamespace,omitempty"`
	IsNamespaceScoped  bool               `json:"isNamespaceScoped"`
	Pattern            string             `json:"pattern"`
	RegisteredServices []Microservice     `json:"registeredServices"`
	MsgEndPoint        string             `json:"msgEndPoint"`
	SoftwareVersions   SoftwareVersion    `json:"softwareVersions"`
	LastHeartbeat      string             `json:"lastHeartbeat"`
	PublicKey          string             `json:"publicKey"`
	Arch               string             `json:"arch"`
	UserInput          []policy.UserInput `json:"userInput"`
	HeartbeatIntv      HeartbeatIntervals `json:"heartbeatIntervals,omitempty"`
	HAGroup            string             `json:"ha_group"` // the name of the hagroup this node is in
	LastUpdated        string             `json:"lastUpdated,omitempty"`
}

func (d Device) String() string {
	return fmt.Sprintf("Name: %v, Owner: %v, NodeType: %v, ClusterNamespace: %v, IsNamespaceScoped: %v, HAGroup: %v, Pattern: %v, SoftwareVersions: %v, LastHeartbeat: %v, RegisteredServices: %v, MsgEndPoint: %v, Arch: %v, UserInput: %v, HeartbeatIntv: %v", d.Name, d.Owner, d.NodeType, d.ClusterNamespace, d.IsNamespaceScoped, d.HAGroup, d.Pattern, d.SoftwareVersions, d.LastHeartbeat, d.RegisteredServices, d.MsgEndPoint, d.Arch, d.UserInput, d.HeartbeatIntv)
}

func (d Device) ShortString() string {
	str := fmt.Sprintf("Name: %v, Owner: %v, NodeType: %v, ClusterNamespace: %v, HAGroup: %v, Pattern %v, LastHeartbeat: %v, MsgEndPoint: %v, Arch: %v, HeartbeatIntv: %v", d.Name, d.Owner, d.NodeType, d.ClusterNamespace, d.HAGroup, d.Pattern, d.LastHeartbeat, d.MsgEndPoint, d.Arch, d.HeartbeatIntv)
	for _, ms := range d.RegisteredServices {
		str += fmt.Sprintf("%v,", ms.Url)
	}
	return str
}

func (n Device) DeepCopy() *Device {
	nodeCopy := Device{Token: n.Token, Name: n.Name, Owner: n.Owner, NodeType: n.NodeType, ClusterNamespace: n.ClusterNamespace, IsNamespaceScoped: n.IsNamespaceScoped, HAGroup: n.HAGroup, Pattern: n.Pattern, MsgEndPoint: n.MsgEndPoint, LastHeartbeat: n.LastHeartbeat, PublicKey: n.PublicKey, Arch: n.Arch, HeartbeatIntv: n.HeartbeatIntv, LastUpdated: n.LastUpdated}
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

const AGENT_VERSION = "horizon"
const CERT_VERSION = "certificate"
const CONFIG_VERSION = "config"

type SoftwareVersion map[string]string

func (s SoftwareVersion) DeepCopy() *SoftwareVersion {
	swVersCopy := make(SoftwareVersion, len(s))
	for key, val := range s {
		swVersCopy[key] = val
	}
	return &swVersCopy
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
				if glog.V(5) {
					glog.Infof(rpclogString(fmt.Sprintf("device details for %v: %v", deviceId, dev)))
				}
				UpdateCache(NodeCacheMapKey(GetOrg(deviceId), GetId(deviceId)), NODE_DEF_TYPE_CACHE, dev)
				return &dev, nil
			}
		}
	}
}

func GetExchangeOrgDevices(httpClientFactory *config.HTTPClientFactory, orgId string, credId string, credPasswd string, exchangeUrl string) (map[string]Device, error) {
	glog.V(3).Infof(rpclogString(fmt.Sprintf("retrieving devices from org %v from exchange", orgId)))

	var resp interface{}
	resp = new(GetDevicesResponse)
	targetURL := exchangeUrl + "orgs/" + orgId + "/nodes"

	if err := InvokeExchangeRetryOnTransportError(httpClientFactory, "GET", targetURL, credId, credPasswd, nil, &resp); err != nil {
		glog.Errorf(err.Error())
		return nil, err
	}
	return resp.(*GetDevicesResponse).Devices, nil
}

type PutDeviceResponse map[string]string

type PostDeviceResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
}

type PutDeviceRequest struct {
	Token              string             `json:"token"`
	Name               string             `json:"name"`
	NodeType           string             `json:"nodeType"`
	ClusterNamespace   *string            `json:"clusterNamespace,omitempty"`
	IsNamespaceScoped  bool               `json:"isNamespaceScoped"`
	Pattern            string             `json:"pattern"`
	RegisteredServices []Microservice     `json:"registeredServices"`
	MsgEndPoint        string             `json:"msgEndPoint"`
	SoftwareVersions   SoftwareVersion    `json:"softwareVersions"`
	PublicKey          []byte             `json:"publicKey"`
	Arch               string             `json:"arch"`
	UserInput          []policy.UserInput `json:"userInput"`
}

func (p PutDeviceRequest) String() string {
	return fmt.Sprintf("Token: %v, Name: %v, NodeType: %v, ClusterNamespace: %v, IsNamespaceScoped: %v, RegisteredServices %v, MsgEndPoint %v, SoftwareVersions %v, PublicKey %x, Arch: %v, UserInput: %v", "*****", p.Name, p.NodeType, p.ClusterNamespace, p.IsNamespaceScoped, p.RegisteredServices, p.MsgEndPoint, p.SoftwareVersions, p.PublicKey, p.Arch, p.UserInput)
}

func (p PutDeviceRequest) ShortString() string {
	str := fmt.Sprintf("Token: %v, Name: %v, NodeType: %v, ClusterNamespace: %v, MsgEndPoint %v, Arch: %v, SoftwareVersions %v", "*****", p.Name, p.NodeType, p.ClusterNamespace, p.MsgEndPoint, p.Arch, p.SoftwareVersions)
	str += ", Service URLs: "
	for _, ms := range p.RegisteredServices {
		str += fmt.Sprintf("%v %v,", ms.Url, ms.Version)
	}
	return str
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

// Please patch one field at a time.
type PatchDeviceRequest struct {
	UserInput          *[]policy.UserInput `json:"userInput,omitempty"`
	Pattern            *string             `json:"pattern,omitempty"`
	Arch               *string             `json:"arch,omitempty"`
	ClusterNamespace   *string             `json:"clusterNamespace,omitempty"`
	IsNamespaceScoped  *bool               `json:"isNamespaceScoped,omitempty"`
	RegisteredServices *[]Microservice     `json:"registeredServices,omitempty"`
	SoftwareVersions   SoftwareVersion     `json:"softwareVersions"`
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
	clusterNs := "nil"
	if p.ClusterNamespace != nil {
		clusterNs = *p.ClusterNamespace
	}
	isNs := "nil"
	if p.IsNamespaceScoped != nil {
		isNs = fmt.Sprintf("%v", *p.IsNamespaceScoped)
	}
	return fmt.Sprintf("UserInput: %v, RegisteredServices: %v, Pattern: %v, Arch: %v, ClusterNamespace: %v, IsNamespaceScoped: %v, SoftwareVersions: %v", p.UserInput, p.RegisteredServices, pattern, arch, clusterNs, isNs, p.SoftwareVersions)
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

	clusterNs := "nil"
	if p.ClusterNamespace != nil {
		clusterNs = *p.ClusterNamespace
	}

	isNs := "nil"
	if p.IsNamespaceScoped != nil {
		isNs = fmt.Sprintf("%v", *p.IsNamespaceScoped)
	}

	return fmt.Sprintf("UserInput: %v, RegisteredServices: %v, Pattern: %v, Arch: %v, ClusterNamespace: %v, IsNamespaceScoped: %v, SoftwareVersions: %v", userInput, registeredServices, pattern, arch, clusterNs, isNs, p.SoftwareVersions)
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

// This function creates the device registration complete message body.
func CreatePatchDeviceKey() *PatchAgbotPublicKey {

	// Same request body structure for node and agbot.
	pdr := &PatchAgbotPublicKey{
		PublicKey: keyBytes(),
	}

	return pdr
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

// ----------- for node status ---------------------- //

type ContainerStatus struct {
	Name    string `json:"name"`
	Image   string `json:"image"`
	Created int64  `json:"created"`
	State   string `json:"state"`
}

func (w ContainerStatus) String() string {
	return fmt.Sprintf("Name: %v, "+
		"Image: %v, "+
		"Created: %v, "+
		"State: %v",
		w.Name, w.Image, w.Created, w.State)
}

type WorkloadStatus struct {
	AgreementId    string            `json:"agreementId"`
	ServiceURL     string            `json:"serviceUrl,omitempty"`
	Org            string            `json:"orgid,omitempty"`
	Version        string            `json:"version,omitempty"`
	Arch           string            `json:"arch,omitempty"`
	Containers     []ContainerStatus `json:"containerStatus"`
	OperatorStatus interface{}       `json:"operatorStatus,omitempty"`
	ConfigState    string            `json:"configState,omitempty"`
}

func (w WorkloadStatus) String() string {
	return fmt.Sprintf("AgreementId: %v, "+
		"ServiceURL: %v, "+
		"Org: %v, "+
		"Version: %v, "+
		"Arch: %v, "+
		"Containers: %v"+
		"OperatorStatus: %v"+
		"ConfigState: %v",
		w.AgreementId, w.ServiceURL, w.Org, w.Version, w.Arch, w.Containers, w.OperatorStatus, w.ConfigState)
}

type DeviceStatus struct {
	Connectivity    map[string]bool  `json:"connectivity,omitempty"` //  hosts and whether this device can reach them or not
	Services        []WorkloadStatus `json:"services"`
	RunningServices *string          `json:"runningServices,omitempty"`
	LastUpdated     *string          `json:"lastUpdated,omitempty"`
}

func (w DeviceStatus) String() string {
	return fmt.Sprintf(
		"Connectivity: %v, "+
			"Services: %v,"+
			"RunningServices: %v,"+
			"LastUpdated: %v",
		w.Connectivity, w.Services, w.RunningServices, w.LastUpdated)
}

func NewDeviceStatus() *DeviceStatus {
	return &DeviceStatus{}
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
			if glog.V(5) {
				glog.Infof(rpclogString(fmt.Sprintf("returning node status %v for %v.", resp, deviceId)))
			}
			nodeStatus := resp.(*NodeStatus)
			return nodeStatus, nil
		}
	}
}

func GetNodeFullStatus(ec ExchangeContext, deviceId string) (*DeviceStatus, error) {

	glog.V(3).Infof(rpclogString(fmt.Sprintf("getting node full status for %v.", deviceId)))

	// Get the node status object. There should only be 1.
	var resp interface{}
	resp = new(DeviceStatus)

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
			if glog.V(5) {
				glog.Infof(rpclogString(fmt.Sprintf("returning node status %v for %v.", resp, deviceId)))
			}
			nodeStatus := resp.(*DeviceStatus)
			return nodeStatus, nil
		}
	}
}

// ------------ for node agreements ------------------//

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

// ------------ for node surface errors ------------------//

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
			if glog.V(5) {
				glog.Infof(rpclogString(fmt.Sprintf("returning node surface errors %v for %v.", resp, deviceId)))
			}
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
