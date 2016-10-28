package exchange

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/policy"
	gwhisper "github.com/open-horizon/go-whisper"
	"io/ioutil"
	"net/http"
	"reflect"
	"runtime"
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

type Microservice struct {
	Url           string   `json:"url"`
	Properties    []MSProp `json:"properties"`
	NumAgreements int      `json:"numAgreements"`
	Policy        string   `json:"policy"`
}

type SearchExchangeRequest struct {
	DesiredMicroservices []Microservice `json:"desiredMicroservices"`
	DaysStale            int            `json:"daysStale"`
	PropertiesToReturn   []string       `json:"propertiesToReturn"`
	LastIndex            int            `json:"startIndex"`
	NumEntries           int            `json:"numEntries"`
}

type Device struct {
	Id            string         `json:"id"`
	Name          string         `json:"name"`
	Microservices []Microservice `json:"microservices"`
	MsgEndPoint   string         `json:"msgEndPoint"`
}

type SearchExchangeResponse struct {
	Devices   []Device `json:"devices"`
	LastIndex int      `json:"lastIndex"`
}

type GetDevicesResponse struct {
	Devices   map[string]Device `json:"devices"`
	LastIndex int               `json:"lastIndex"`
}

type AgbotAgreement struct {
	Workload    string `json:"workload"`
	State       string `json:"state"`
	LastUpdated string `json:"lastUpdated"`
}

type DeviceAgreement struct {
	Microservice string `json:"microservice"`
	State        string `json:"state"`
	LastUpdated  string `json:"lastUpdated"`
}

type AllAgbotAgreementsResponse struct {
	Agreements map[string]AgbotAgreement `json:"agreements`
	LastIndex  int                       `json:"lastIndex"`
}

type AllDeviceAgreementsResponse struct {
	Agreements map[string]DeviceAgreement `json:"agreements`
	LastIndex  int                        `json:"lastIndex"`
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
}

// This function creates the exchange search message body.
func CreateSearchRequest() *SearchExchangeRequest {

	ser := &SearchExchangeRequest{
		DaysStale:  0,
		LastIndex:  0,
		NumEntries: 10,
	}

	return ser
}

// This function creates the device registration message body.
func CreateDevicePut(gethURL string) *PutDeviceRequest {

	getWhisperId := func() string {
		if wId, err := gwhisper.AccountId(gethURL); err != nil {
			glog.Error(err)
			return ""
		} else {
			return wId
		}
	}

	pdr := &PutDeviceRequest{
		Name:             "anaxdev",
		MsgEndPoint:      getWhisperId(),
		SoftwareVersions: make(map[string]string),
	}

	return pdr
}

func ConvertToString(a []string) string {
	r := ""
	for _, s := range a {
		r = r + s + ", "
	}
	r = strings.TrimRight(r, ", ")
	return r
}

func Heartbeat(h *http.Client, url string, interval int) {

	for {
		glog.V(5).Infof("Heartbeating to exchange: %v", url)

		var resp interface{}
		resp = new(PostDeviceResponse)
		for {
			if err, tpErr := InvokeExchange(h, "POST", url, nil, &resp); err != nil {
				glog.Errorf(err.Error())
				break
			} else if tpErr != nil {
				glog.Warningf(err.Error())
				time.Sleep(10 * time.Second)
				continue
			} else {
				glog.V(5).Infof("Sent heartbeat %v: %v", url, resp)
				break
			}
		}

		time.Sleep(time.Duration(interval) * time.Second)
		runtime.Gosched()
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
func InvokeExchange(httpClient *http.Client, method string, url string, params interface{}, resp *interface{}) (error, error) {

	if len(method) == 0 {
		return errors.New(fmt.Sprintf("Error invoking exchange, method name must be specified")), nil
	} else if len(url) == 0 {
		return errors.New(fmt.Sprintf("Error invoking exchange, no URL to invoke")), nil
	} else if resp == nil {
		return errors.New(fmt.Sprintf("Error invoking exchange, response object must be specified")), nil
	}

	glog.V(5).Infof("Invoking exchange %v at %v with %v", method, url, params)

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
		if httpResp, err := httpClient.Do(req); err != nil {
			return nil, errors.New(fmt.Sprintf("Invocation of %v at %v with %v failed invoking HTTP request, error: %v", method, url, requestBody, err))
		} else {
			defer httpResp.Body.Close()

			var outBytes []byte
			var readErr error
			if httpResp.Body != nil {
				if outBytes, readErr = ioutil.ReadAll(httpResp.Body); err != nil {
					return errors.New(fmt.Sprintf("Invocation of %v at %v failed reading response message, HTTP Status %v, error: %v", method, url, httpResp.StatusCode, readErr)), nil
				}
			}

			// Handle special case of server error
			if httpResp.StatusCode == http.StatusInternalServerError && strings.Contains(string(outBytes), "timed out") {
				return nil, errors.New(fmt.Sprintf("Invocation of %v at %v with %v failed invoking HTTP request, error: %v", method, url, requestBody, err))
			}

			if method == "GET" && (httpResp.StatusCode != http.StatusOK && httpResp.StatusCode != http.StatusNotFound) {
				return errors.New(fmt.Sprintf("Invocation of %v at %v failed invoking HTTP request, status: %v, response: %v", method, url, httpResp.StatusCode, string(outBytes))), nil
			} else if (method == "PUT" || method == "POST") && httpResp.StatusCode != http.StatusCreated {
				return errors.New(fmt.Sprintf("Invocation of %v at %v failed invoking HTTP request, status: %v, response: %v", method, url, httpResp.StatusCode, string(outBytes))), nil
			} else if method == "DELETE" && httpResp.StatusCode != http.StatusNoContent {
				return errors.New(fmt.Sprintf("Invocation of %v at %v failed invoking HTTP request, status: %v, response: %v", method, url, httpResp.StatusCode, string(outBytes))), nil
			} else if method == "DELETE" {
				return nil, nil
			} else {
				out := string(outBytes)
				glog.V(5).Infof("Response to %v at %v is %v", method, url, out)
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

					case *AllDeviceAgreementsResponse:
						return nil, nil

					case *AllAgbotAgreementsResponse:
						return nil, nil

					default:
						return errors.New(fmt.Sprintf("Unknown type of response object passed to invocation of %v at %v with %v", *resp, method, url, requestBody)), nil
					}
				}
			}
		}
	}
}
