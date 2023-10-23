package exchange

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/semanticversion"
	"github.com/open-horizon/edge-sync-service/common"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
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
		return id
	} else {
		return id[ix+1:]
	}
}

// swagger:model
type PutPostDeleteStandardResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
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

	if glog.V(5) {
		if reflect.ValueOf(params).Kind() == reflect.Ptr {
			paramValue := reflect.Indirect(reflect.ValueOf(params))

			// These 2 param types can contain very large payloads so must be cautious about trying to generate a log message with the entire payload
			payload_str := ""
			switch params.(type) {
			case *GetExchangeChangesRequest:
				gecr := params.(*GetExchangeChangesRequest)
				if len(gecr.Orgs) > 50 {
					payload_str = fmt.Sprintf("<exchange.GetExchangeChangesRequest for %d organizations>", len(gecr.Orgs))
				} else {
					payload_str = fmt.Sprintf("%v", paramValue)
				}
			case *PostDestsRequest:
				pdr := params.(*PostDestsRequest)
				if len(pdr.Destinations) > 50 {
					payload_str = fmt.Sprintf("<exchange.PostDestsRequest with %d destinations>", len(pdr.Destinations))
				} else {
					payload_str = fmt.Sprintf("%v", paramValue)
				}
			default:
				payload_str = fmt.Sprintf("%v", paramValue)
			}
			glog.Infof(rpclogString(fmt.Sprintf("Invoking exchange %v at %v with %v", method, urlPath, payload_str)))
		} else {
			glog.Infof(rpclogString(fmt.Sprintf("Invoking exchange %v at %v with %v", method, urlPath, params)))
		}
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
		/*
		 * "req.Close  = true" code commented out so that agbot can reuse connections on the management hub.
		 * In order to support 10's of thousands of agents though there is a different behavior between agbot and agent where the agbot sets
		 * a IdleConnTimeout in seconds and the agent sets the IdleConnTimeout in milliseconds so it frees up quickly after a request/response
		 */
		//req.Close = true // work around to ensure that Go doesn't get connections confused. Supposed to be fixed in Go 1.6.
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

			if (method == "GET" || method == "LIST") && httpResp.StatusCode != http.StatusOK {
				if httpResp.StatusCode == http.StatusNotFound {
					if glog.V(5) {
						glog.Infof(rpclogString(fmt.Sprintf("Got %v. Response to %v at %v is %v", httpResp.StatusCode, method, urlPath, string(outBytes))))
					}
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
				if glog.V(6) {
					glog.Infof(rpclogString(fmt.Sprintf("Response to %v at %v is %v", method, urlPath, out)))
				}

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
						if glog.V(5) {
							glog.Infof(rpclogString(fmt.Sprintf("Got %v. Response to %v at %v is %v", httpResp.StatusCode, method, urlPath, *resp)))
						}
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

					case *ExchangeNodePolicy:
						return nil, nil

					case *ExchangeServicePolicy:
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

					case *MetaDataList:
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

					case *DeviceStatus:
						return nil, nil

					case *VaultSecretExistsResponse:
						return nil, nil

					case *GetNMPResponse:
						return nil, nil

					case *ExchangeNodeManagementPolicyResponse:
						return nil, nil

					case *exchangecommon.AgentFileVersions:
						return nil, nil

					case *exchangecommon.AgentUpgradeVersions:
						return nil, nil

					case *exchangecommon.UpgradeManifest:
						return nil, nil

					case *PutPostDeleteStandardResponse:
						return nil, nil

					case *NodeManagementAllStatuses:
						return nil, nil

					case *exchangecommon.GetHAGroupResponse:
						return nil, nil

					case *exchangecommon.NodeManagementPolicyStatus:
						return nil, nil

					case *exchangecommon.ExchangeNMPStatus:
						return nil, nil
					default:
						return errors.New(fmt.Sprintf("Unknown type of response object %v (%T) passed to invocation of %v at %v with %v", *resp, *resp, method, urlPath, requestBody)), nil
					}
				}
			}
		}
	}
}

func InvokeExchangeRetryOnTransportError(httpClientFactory *config.HTTPClientFactory, method string, urlPath string, user string, pw string, params interface{}, resp *interface{}) error {
	retryCount := httpClientFactory.RetryCount
	retryInterval := httpClientFactory.GetRetryInterval()
	for {
		if err, tpErr := InvokeExchange(httpClientFactory.NewHTTPClient(nil), method, urlPath, user, pw, params, resp); err != nil {
			glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
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
			return nil
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
		} else if strings.Contains(l_error_string, "broken pipe") {
			return true
		} else if strings.Contains(l_error_string, "dial tcp") && strings.Contains(l_error_string, "server misbehaving") {
			// Could be from DNS like: dial tcp: lookup cp-console.cloud on 127.0.0.53:53: server misbehaving:
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
		} else if pResp.StatusCode == http.StatusRequestTimeout {
			// 408: request time out
			return true
		} else if pResp.StatusCode == http.StatusTooManyRequests {
			// 429: too many requests
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
			return nil, errors.New(rpclogString(fmt.Sprintf("GetObjectSigningKeys got wrong version string %v. The version string should be a non-empty single version string.", oVersion)))
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
				if glog.V(5) {
					glog.Infof(rpclogString(fmt.Sprintf("found object signing keys %v.", resp_KeyNames)))
				}
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
					if glog.V(5) {
						glog.Infof(rpclogString(fmt.Sprintf("found signing key content for key %v: %v.", key, resp_KeyContent)))
					}
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
