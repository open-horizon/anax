package cliutils

import (
	"strings"
	"fmt"
	"os"
	"net/http"
	"io/ioutil"
	"encoding/json"
	"time"
	"encoding/base64"
	"bytes"
	"github.com/open-horizon/anax/exchange"
)

const (
	HZN_API = "http://localhost"
)

// Holds the cmd line flags that were set so other pkgs can access
type Options struct {
	Verbose *bool
	ArchivedAgreements *bool
}
var Opts Options


func Verbose(msg string, args ...interface{}) {
	if !*Opts.Verbose { return }
	if !strings.HasSuffix(msg, "\n") { msg += "\n"}
	fmt.Fprintf(os.Stderr, "[verbose] "+msg, args...)	// send to stderr so it doesn't mess up stdout if they are piping that to jq or something like that
}


//todo: how should we handle exit codes? A different 1 for every error? (too complicated) Defined categories of errors?
func Fatal(exitCode int, msg string, args ...interface{}) {
	if !strings.HasSuffix(msg, "\n") { msg += "\n"}
	fmt.Fprintf(os.Stderr, "Error: "+msg, args...)
	os.Exit(exitCode)
}

/*
func GetShortBinaryName() string {
	return path.Base(os.Args[0])
}
*/


// GetHorizonUrlBase returns the base part of the horizon api url (which can be overridden by env var HORIZON_URL_BASE)
func GetHorizonUrlBase() string {
	envVar := os.Getenv("HORIZON_URL_BASE")
	if envVar != "" {
		return envVar
	}
	return HZN_API
}


// HorizonGet runs a GET on the anax api and fills in the specified json structure.
// If goodHttp is non-zero and does not match the actual http code, it will exit with an error. Otherwise the actual code is returned.
func HorizonGet(urlSuffix string, goodHttp int, structure interface{}) (httpCode int) {
	url := GetHorizonUrlBase() + "/" + urlSuffix
	apiMsg := "GET " + url
	Verbose(apiMsg)
	resp, err := http.Get(url)
	if err != nil { Fatal(3, "%s failed: %v", apiMsg, err) }
	defer resp.Body.Close()
	httpCode = resp.StatusCode
	Verbose("HTTP code: %d", httpCode)
	if goodHttp > 0 && httpCode != goodHttp { Fatal(3, "bad HTTP code from %s: %d", apiMsg, httpCode) }
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil { Fatal(3, "failed to read body response for %s: %v", apiMsg, err) }
	err = json.Unmarshal(bodyBytes, structure)
	if err != nil { Fatal(3, "failed to unmarshal body response for %s: %v", apiMsg, err) }
	return
}


// ExchangeGet runs a GET to the exchange api and fills in the specified json structure.
// If goodHttp is non-zero and does not match the actual http code, it will exit with an error. Otherwise the actual code is returned.
func ExchangeGet(urlBase string, urlSuffix string, credentials string, goodHttp int, structure interface{}) (httpCode int) {
	url := urlBase + "/" + urlSuffix
	apiMsg := "GET " + url
	Verbose(apiMsg)
	httpClient := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil { Fatal(3, "%s new request failed: %v", apiMsg, err) }
	req.Header.Add("Accept", "application/json")
	//req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Basic %v", base64.StdEncoding.EncodeToString([]byte(credentials))))
	resp, err := httpClient.Do(req)
	if err != nil { Fatal(3, "%s request failed: %v", apiMsg, err) }
	defer resp.Body.Close()
	httpCode = resp.StatusCode
	Verbose("HTTP code: %d", httpCode)
	if goodHttp > 0 && httpCode != goodHttp { Fatal(3, "bad HTTP code from %s: %d", apiMsg, httpCode) }
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil { Fatal(3, "failed to read body response for %s: %v", apiMsg, err) }
	err = json.Unmarshal(bodyBytes, structure)
	if err != nil { Fatal(3, "failed to unmarshal body response for %s: %v", apiMsg, err) }
	return
}


// ExchangePutPost runs a PUT or POST to the exchange api to create of update a resource.
// If goodHttp is non-zero and does not match the actual http code, it will exit with an error. Otherwise the actual code is returned.
func ExchangePutPost(method string, urlBase string, urlSuffix string, credentials string, goodHttp int, body interface{}) (httpCode int) {
	url := urlBase + "/" + urlSuffix
	apiMsg := method + " " + url
	Verbose(apiMsg)
	httpClient := &http.Client{}
	jsonBytes, err := json.Marshal(body)
	if err != nil { Fatal(3, "failed to marshal body for %s: %v", apiMsg, err) }
	requestBody := bytes.NewBuffer(jsonBytes)
	req, err := http.NewRequest(method, url, requestBody)
	if err != nil { Fatal(3, "%s new request failed: %v", apiMsg, err) }
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Basic %v", base64.StdEncoding.EncodeToString([]byte(credentials))))
	resp, err := httpClient.Do(req)
	if err != nil { Fatal(3, "%s request failed: %v", apiMsg, err) }
	defer resp.Body.Close()
	httpCode = resp.StatusCode
	Verbose("HTTP code: %d", httpCode)
	if goodHttp > 0 && httpCode != goodHttp {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil { Fatal(3, "failed to read body response for %s: %v", apiMsg, err) }
		respMsg := exchange.PostDeviceResponse{}
		err = json.Unmarshal(bodyBytes, &respMsg)
		if err != nil { Fatal(3, "failed to unmarshal body response for %s: %v", apiMsg, err) }
		Fatal(3, "bad HTTP code %d from %s: %s, %s", httpCode, apiMsg, respMsg.Code, respMsg.Msg)
	}
	return
}


func ConvertTime(unixSecond uint64) string {
	 return time.Unix(int64(unixSecond), 0).String()
}
