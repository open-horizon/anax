package exchange

import (
	"bytes"
	"crypto/rsa"
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

type SearchExchangeRequest struct {
	DesiredMicroservices []Microservice `json:"desiredMicroservices"`
	SecondsStale         int            `json:"secondsStale"`
	PropertiesToReturn   []string       `json:"propertiesToReturn"`
	StartIndex           int            `json:"startIndex"`
	NumEntries           int            `json:"numEntries"`
}

func (a SearchExchangeRequest) String() string {
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

type SearchExchangeResponse struct {
	Devices   []SearchResultDevice `json:"devices"`
	LastIndex int                  `json:"lastIndex"`
}

func (r SearchExchangeResponse) String() string {
	return fmt.Sprintf("Devices: %v, LastIndex: %v", r.Devices, r.LastIndex)
}

type Device struct {
	Token                   string          `json:"token"`
	Name                    string          `json:"name"`
	Owner                   string          `json:"owner"`
	RegisteredMicroservices []Microservice  `json:"registeredMicroservices"`
	MsgEndPoint             string          `json:"msgEndPoint"`
	SoftwareVersions        SoftwareVersion `json:"softwareVersions"`
	LastHeartbeat           string          `json:"lastHeartbeat"`
	PublicKey               []byte          `json:"publicKey"`
}

type GetDevicesResponse struct {
	Devices   map[string]Device `json:"devices"`
	LastIndex int               `json:"lastIndex"`
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
	Agbots map[string]Agbot `json:"agbots"`
}

type AgbotAgreement struct {
	Workload    string `json:"workload"`
	State       string `json:"state"`
	LastUpdated string `json:"lastUpdated"`
}

func (a AgbotAgreement) String() string {
	return fmt.Sprintf("Workload: %v, State: %v, LastUpdated: %v", a.Workload, a.State, a.LastUpdated)
}

type DeviceAgreement struct {
	Microservice string `json:"microservice"`
	State        string `json:"state"`
	LastUpdated  string `json:"lastUpdated"`
}

func (a DeviceAgreement) String() string {
	return fmt.Sprintf("Microservice: %v, State: %v, LastUpdated: %v", a.Microservice, a.State, a.LastUpdated)
}

type AllAgbotAgreementsResponse struct {
	Agreements map[string]AgbotAgreement `json:"agreements`
	LastIndex  int                       `json:"lastIndex"`
}

func (a AllAgbotAgreementsResponse) String() string {
	return fmt.Sprintf("Agreements: %v, LastIndex: %v", a.Agreements, a.LastIndex)
}

type AllDeviceAgreementsResponse struct {
	Agreements map[string]DeviceAgreement `json:"agreements`
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

type PutAgbotAgreementState struct {
	Workload string `json:"workload"`
	State    string `json:"state"`
}

type PutAgreementState struct {
	Microservice string `json:"microservice"`
	State        string `json:"state"`
}

type SoftwareVersion map[string]string

type PutDeviceRequest struct {
	Token                   string          `json:"token"`
	Name                    string          `json:"name"`
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
		PublicKey:        keyBytes(),
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
		TTL: theTTL,
	}

	return pm
}

type ExchangeMessageTarget struct {
	ReceiverExchangeId     string
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
	DeviceId     string `json:"deviceId"`
	DevicePubKey []byte `json:"devicePubKey"`
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
func CreateSearchRequest() *SearchExchangeRequest {

	ser := &SearchExchangeRequest{
		StartIndex: 0,
		NumEntries: 100,
	}

	return ser
}

// This function creates the device registration message body.
func CreateDevicePut(token string, name string) *PutDeviceRequest {

	keyBytes := func() []byte {
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

	pdr := &PutDeviceRequest{
		Token:            token,
		Name:             name,
		MsgEndPoint:      "",
		SoftwareVersions: make(map[string]string),
		PublicKey:        keyBytes(),
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

func Heartbeat(h *http.Client, url string, id string, token string, interval int) {

	for {
		glog.V(5).Infof(rpclogString(fmt.Sprintf("Heartbeating to exchange: %v", url)))

		var resp interface{}
		resp = new(PostDeviceResponse)
		for {
			if err, tpErr := InvokeExchange(h, "POST", url, id, token, nil, &resp); err != nil {
				glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
				break
			} else if tpErr != nil {
				glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
				time.Sleep(10 * time.Second)
				continue
			} else {
				glog.V(5).Infof(rpclogString(fmt.Sprintf("Sent heartbeat %v: %v", url, resp)))
				break
			}
		}

		time.Sleep(time.Duration(interval) * time.Second)
	}

}

func GetEthereumClient(url string, chainName string, chainType string, deviceId string, token string) (string, error) {

	glog.V(5).Infof(rpclogString(fmt.Sprintf("getting ethereum client metadata for chain %v", chainName)))

	var resp interface{}
	resp = new(GetEthereumClientResponse)
	targetURL := url + "bctypes/" + chainType + "/blockchains/" + chainName
	for {
		if err, tpErr := InvokeExchange(&http.Client{Timeout: time.Duration(config.HTTPDEFAULTTIMEOUT*time.Millisecond)}, "GET", targetURL, deviceId, token, nil, &resp); err != nil {
			glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
			return "", err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(3).Infof(rpclogString(fmt.Sprintf("found blockchain %v.", resp)))
			clientMetadata := resp.(*GetEthereumClientResponse).Blockchains[chainName].Details
			return clientMetadata, nil
		}
	}

}

func ConvertPropertyToExchangeFormat(prop *policy.Property) (*MSProp, error) {
	var pType, pValue, pCompare string

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

// This function is used to invoke an exchange API
func InvokeExchange(httpClient *http.Client, method string, url string, user string, pw string, params interface{}, resp *interface{}) (error, error) {

	if len(method) == 0 {
		return errors.New(fmt.Sprintf("Error invoking exchange, method name must be specified")), nil
	} else if len(url) == 0 {
		return errors.New(fmt.Sprintf("Error invoking exchange, no URL to invoke")), nil
	} else if resp == nil {
		return errors.New(fmt.Sprintf("Error invoking exchange, response object must be specified")), nil
	}

	glog.V(5).Infof(rpclogString(fmt.Sprintf("Invoking exchange %v at %v with %v", method, url, params)))

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
			req.Header.Add("Authorization", "Basic "+user+":"+pw)
		}
		glog.V(5).Infof(rpclogString(fmt.Sprintf("Invoking exchange with headers: %v", req.Header)))
		// If the exchange is down, this call will return an error.
		if httpResp, err := httpClient.Do(req); err != nil {
			if isTimeout(err) {
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
					if isTimeout(err) {
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

			if method == "GET" && (httpResp.StatusCode != http.StatusOK && httpResp.StatusCode != http.StatusNotFound) {
				return errors.New(fmt.Sprintf("Invocation of %v at %v failed invoking HTTP request, status: %v, response: %v", method, url, httpResp.StatusCode, string(outBytes))), nil
			} else if (method == "PUT" || method == "POST" || method == "PATCH") && httpResp.StatusCode != http.StatusCreated {
				return errors.New(fmt.Sprintf("Invocation of %v at %v failed invoking HTTP request, status: %v, response: %v", method, url, httpResp.StatusCode, string(outBytes))), nil
			} else if method == "DELETE" && httpResp.StatusCode != http.StatusNoContent {
				return errors.New(fmt.Sprintf("Invocation of %v at %v failed invoking HTTP request, status: %v, response: %v", method, url, httpResp.StatusCode, string(outBytes))), nil
			} else if method == "DELETE" {
				return nil, nil
			} else {
				out := string(outBytes)
				glog.V(5).Infof(rpclogString(fmt.Sprintf("Response to %v at %v is %v", method, url, out)))
				if err := json.Unmarshal(outBytes, resp); err != nil {
					return errors.New(fmt.Sprintf("Unable to demarshal response %v from invocation of %v at %v, error: %v", out, method, url, err)), nil
				} else {
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

					case *SearchExchangeResponse:
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

					default:
						return errors.New(fmt.Sprintf("Unknown type of response object passed to invocation of %v at %v with %v", *resp, method, url, requestBody)), nil
					}
				}
			}
		}
	}
}

func isTimeout(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "time") && strings.Contains(strings.ToLower(err.Error()), "out")
}

var rpclogString = func(v interface{}) string {
	return fmt.Sprintf("Exchange RPC %v", v)
}
