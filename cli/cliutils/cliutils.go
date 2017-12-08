package cliutils

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/exchange"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	HZN_API             = "http://localhost"
	JSON_INDENT         = "  "
	MUST_REGISTER_FIRST = "this command can not be run before running 'hzn register'"

	// Exit Codes
	CLI_INPUT_ERROR    = 1 // we actually don't have control over the usage exit code that kingpin returns, so use the same code for input errors we catch ourselves
	JSON_PARSING_ERROR = 3
	READ_FILE_ERROR    = 4
	HTTP_ERROR         = 5
	//EXEC_CMD_ERROR = 6
	INTERNAL_ERROR = 99

	// Anax API HTTP Codes
	ANAX_ALREADY_CONFIGURED = 409
	ANAX_NOT_CONFIGURED_YET = 424
)

// Holds the cmd line flags that were set so other pkgs can access
type GlobalOptions struct {
	Verbose *bool
}

var Opts GlobalOptions

type UserExchangeReq struct {
	Password string `json:"password"`
	Admin    bool   `json:"admin"`
	Email    string `json:"email"`
}

func Verbose(msg string, args ...interface{}) {
	if !*Opts.Verbose {
		return
	}
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}
	fmt.Fprintf(os.Stderr, "[verbose] "+msg, args...) // send to stderr so it doesn't mess up stdout if they are piping that to jq or something like that
}

func Fatal(exitCode int, msg string, args ...interface{}) {
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}
	fmt.Fprintf(os.Stderr, "Error: "+msg, args...)
	os.Exit(exitCode)
}

/*
func GetShortBinaryName() string {
	return path.Base(os.Args[0])
}
*/

// SplitIdToken splits an id:token or user:pw and return the parts.
func SplitIdToken(idToken string) (id, token string) {
	parts := strings.SplitN(idToken, ":", 2)
	id = parts[0] // SplitN will always at least return 1 element
	token = ""
	if len(parts) >= 2 {
		token = parts[1]
	}
	return
}

// GetHorizonUrlBase returns the base part of the horizon api url (which can be overridden by env var HORIZON_URL_BASE)
func GetHorizonUrlBase() string {
	envVar := os.Getenv("HORIZON_URL_BASE")
	if envVar != "" {
		return envVar
	}
	return HZN_API
}

// GetRespBodyAsString converts an http response body to a string
func GetRespBodyAsString(responseBody io.ReadCloser) string {
	buf := new(bytes.Buffer)
	buf.ReadFrom(responseBody)
	return buf.String()
}

func isGoodCode(actualHttpCode int, goodHttpCodes []int) bool {
	if len(goodHttpCodes) == 0 {
		return true // passing in an empty list of good codes means anything is ok
	}
	for _, code := range goodHttpCodes {
		if code == actualHttpCode {
			return true
		}
	}
	return false
}

// HorizonGet runs a GET on the anax api and fills in the specified structure with the json.
// If the list of goodHttpCodes is not empty and none match the actual http code, it will exit with an error. Otherwise the actual code is returned.
// Only if the actual code matches the 1st element in goodHttpCodes, will it parse the body into the specified structure.
func HorizonGet(urlSuffix string, goodHttpCodes []int, structure interface{}) (httpCode int) {
	url := GetHorizonUrlBase() + "/" + urlSuffix
	apiMsg := http.MethodGet + " " + url
	Verbose(apiMsg)
	resp, err := http.Get(url)
	if err != nil {
		Fatal(HTTP_ERROR, "%s failed: %v", apiMsg, err)
	}
	defer resp.Body.Close()
	httpCode = resp.StatusCode
	Verbose("HTTP code: %d", httpCode)
	if !isGoodCode(httpCode, goodHttpCodes) {
		Fatal(HTTP_ERROR, "bad HTTP code from %s: %d", apiMsg, httpCode)
	}
	if httpCode == goodHttpCodes[0] {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			Fatal(HTTP_ERROR, "failed to read body response for %s: %v", apiMsg, err)
		}
		err = json.Unmarshal(bodyBytes, structure)
		if err != nil {
			Fatal(JSON_PARSING_ERROR, "failed to unmarshal body response for %s: %v", apiMsg, err)
		}
	}
	return
}

// HorizonDelete runs a DELETE on the anax api.
// If the list of goodHttpCodes is not empty and none match the actual http code, it will exit with an error. Otherwise the actual code is returned.
func HorizonDelete(urlSuffix string, goodHttpCodes []int) (httpCode int) {
	url := GetHorizonUrlBase() + "/" + urlSuffix
	apiMsg := http.MethodDelete + " " + url
	Verbose(apiMsg)
	httpClient := &http.Client{}
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		Fatal(HTTP_ERROR, "%s new request failed: %v", apiMsg, err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		Fatal(HTTP_ERROR, "%s request failed: %v", apiMsg, err)
	}
	defer resp.Body.Close()
	httpCode = resp.StatusCode
	Verbose("HTTP code: %d", httpCode)
	if !isGoodCode(httpCode, goodHttpCodes) {
		Fatal(HTTP_ERROR, "bad HTTP code %d from %s: %s", httpCode, apiMsg, GetRespBodyAsString(resp.Body))
	}
	return
}

// HorizonPutPost runs a PUT or POST to the anax api to create of update a resource.
// If the list of goodHttpCodes is not empty and none match the actual http code, it will exit with an error. Otherwise the actual code is returned.
func HorizonPutPost(method string, urlSuffix string, goodHttpCodes []int, body interface{}) (httpCode int) {
	url := GetHorizonUrlBase() + "/" + urlSuffix
	apiMsg := method + " " + url
	Verbose(apiMsg)
	httpClient := &http.Client{}
	jsonBytes, err := json.Marshal(body)
	if err != nil {
		Fatal(JSON_PARSING_ERROR, "failed to marshal body for %s: %v", apiMsg, err)
	}
	requestBody := bytes.NewBuffer(jsonBytes)
	req, err := http.NewRequest(method, url, requestBody)
	if err != nil {
		Fatal(HTTP_ERROR, "%s new request failed: %v", apiMsg, err)
	}
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		Fatal(HTTP_ERROR, "%s request failed: %v", apiMsg, err)
	}
	defer resp.Body.Close()
	httpCode = resp.StatusCode
	Verbose("HTTP code: %d", httpCode)
	if !isGoodCode(httpCode, goodHttpCodes) {
		Fatal(HTTP_ERROR, "bad HTTP code %d from %s: %s", httpCode, apiMsg, GetRespBodyAsString(resp.Body))
	}
	return
}

// GetExchangeUrl returns the exchange url from the anax api
func GetExchangeUrl() string {
	status := api.Info{}
	HorizonGet("status", []int{200}, &status)
	return strings.TrimSuffix(status.Configuration.ExchangeAPI, "/")
}

// ExchangeGet runs a GET to the exchange api and fills in the specified json structure. If the structure is just a string, fill in the raw json.
// If the list of goodHttpCodes is not empty and none match the actual http code, it will exit with an error. Otherwise the actual code is returned.
func ExchangeGet(urlBase string, urlSuffix string, credentials string, goodHttpCodes []int, structure interface{}) (httpCode int) {
	url := urlBase + "/" + urlSuffix
	apiMsg := http.MethodGet + " " + url
	Verbose(apiMsg)
	httpClient := &http.Client{}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		Fatal(HTTP_ERROR, "%s new request failed: %v", apiMsg, err)
	}
	req.Header.Add("Accept", "application/json")
	//req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Basic %v", base64.StdEncoding.EncodeToString([]byte(credentials))))
	resp, err := httpClient.Do(req)
	if err != nil {
		Fatal(HTTP_ERROR, "%s request failed: %v", apiMsg, err)
	}
	defer resp.Body.Close()
	httpCode = resp.StatusCode
	Verbose("HTTP code: %d", httpCode)
	if !isGoodCode(httpCode, goodHttpCodes) {
		Fatal(HTTP_ERROR, "bad HTTP code from %s: %d", apiMsg, httpCode)
	}
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		Fatal(HTTP_ERROR, "failed to read body response for %s: %v", apiMsg, err)
	}

	switch s := structure.(type) {
	case *string:
		// If the structure to fill in is just a string, unmarshal/remarshal it to get it in json indented form, and then return as a string
		var jsonStruct interface{}
		err = json.Unmarshal(bodyBytes, &jsonStruct)
		if err != nil {
			Fatal(JSON_PARSING_ERROR, "failed to unmarshal body response for %s: %v", apiMsg, err)
		}
		jsonBytes, err := json.MarshalIndent(jsonStruct, "", JSON_INDENT)
		if err != nil {
			Fatal(JSON_PARSING_ERROR, "failed to marshal 'show pem' output: %v", err)
		}
		*s = string(jsonBytes)
	default:
		err = json.Unmarshal(bodyBytes, structure)
		if err != nil {
			Fatal(JSON_PARSING_ERROR, "failed to unmarshal body response for %s: %v", apiMsg, err)
		}
	}
	return
}

// ExchangePutPost runs a PUT or POST to the exchange api to create of update a resource.
// If the list of goodHttpCodes is not empty and none match the actual http code, it will exit with an error. Otherwise the actual code is returned.
func ExchangePutPost(method string, urlBase string, urlSuffix string, credentials string, goodHttpCodes []int, body interface{}) (httpCode int) {
	url := urlBase + "/" + urlSuffix
	apiMsg := method + " " + url
	Verbose(apiMsg)
	httpClient := &http.Client{}
	jsonBytes, err := json.Marshal(body)
	if err != nil {
		Fatal(JSON_PARSING_ERROR, "failed to marshal body for %s: %v", apiMsg, err)
	}
	requestBody := bytes.NewBuffer(jsonBytes)
	req, err := http.NewRequest(method, url, requestBody)
	if err != nil {
		Fatal(HTTP_ERROR, "%s new request failed: %v", apiMsg, err)
	}
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	if credentials != "" {
		req.Header.Add("Authorization", fmt.Sprintf("Basic %v", base64.StdEncoding.EncodeToString([]byte(credentials))))
	} // else it is an anonymous call
	resp, err := httpClient.Do(req)
	if err != nil {
		Fatal(HTTP_ERROR, "%s request failed: %v", apiMsg, err)
	}
	defer resp.Body.Close()
	httpCode = resp.StatusCode
	Verbose("HTTP code: %d", httpCode)
	if !isGoodCode(httpCode, goodHttpCodes) {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			Fatal(HTTP_ERROR, "failed to read body response for %s: %v", apiMsg, err)
		}
		respMsg := exchange.PostDeviceResponse{}
		err = json.Unmarshal(bodyBytes, &respMsg)
		if err != nil {
			Fatal(JSON_PARSING_ERROR, "failed to unmarshal body response for %s: %v", apiMsg, err)
		}
		Fatal(HTTP_ERROR, "bad HTTP code %d from %s: %s, %s", httpCode, apiMsg, respMsg.Code, respMsg.Msg)
	}
	return
}

func ConvertTime(unixSeconds uint64) string {
	if unixSeconds == 0 {
		return ""
	}
	return time.Unix(int64(unixSeconds), 0).String()
}

/* Do not need at the moment, but keeping for reference...
// Run a command with optional stdin and args, and return stdout, stderr
func RunCmd(stdinBytes []byte, commandString string, args ...string) ([]byte, []byte) {
	// For debug, build the full cmd string
	cmdStr := commandString
	for _, a := range args {
		cmdStr += " " + a
	}
	if stdinBytes != nil { cmdStr += " < stdin" }
	Verbose("running: %v\n", cmdStr)

	// Create the command object with its args
	cmd := exec.Command(commandString, args...)
	if cmd == nil { Fatal(EXEC_CMD_ERROR, "did not get a command object") }

	var stdin io.WriteCloser
	//var jInbytes []byte
	var err error
	if stdinBytes != nil {
		// Create the std in pipe
		stdin, err = cmd.StdinPipe()
		if err != nil { Fatal(EXEC_CMD_ERROR, "Could not get Stdin pipe, error: %v", err) }
		// Read the input file
		//jInbytes, err = ioutil.ReadFile(stdinFilename)
		//if err != nil { Fatal(EXEC_CMD_ERROR,"Unable to read " + stdinFilename + " file, error: %v", err) }
	}
	// Create the stdout pipe to hold the output from the command
	stdout, err := cmd.StdoutPipe()
	if err != nil { Fatal(EXEC_CMD_ERROR,"could not retrieve output from command, error: %v", err) }
	// Create the stderr pipe to hold the errors from the command
	stderr, err := cmd.StderrPipe()
	if err != nil { Fatal(EXEC_CMD_ERROR,"could not retrieve stderr from command, error: %v", err) }

	// Start the command, which will block for input from stdin if the cmd reads from it
	err = cmd.Start()
	if err != nil { Fatal(EXEC_CMD_ERROR,"Unable to start command, error: %v", err) }

	if stdinBytes != nil {
		// Send in the std in bytes
		_, err = stdin.Write(stdinBytes)
		if err != nil { Fatal(EXEC_CMD_ERROR, "Unable to write to stdin of command, error: %v", err) }
		// Close std in so that the command will begin to execute
		err = stdin.Close()
		if err != nil { Fatal(EXEC_CMD_ERROR, "Unable to close stdin, error: %v", err) }
	}

	err = error(nil)
	// Read the output from stdout and stderr into byte arrays
	// stdoutBytes, err := readPipe(stdout)
	stdoutBytes, err := ioutil.ReadAll(stdout)
	if err != nil { Fatal(EXEC_CMD_ERROR,"could not read stdout, error: %v", err) }
	// stderrBytes, err := readPipe(stderr)
	stderrBytes, err := ioutil.ReadAll(stderr)
	if err != nil { Fatal(EXEC_CMD_ERROR,"could not read stderr, error: %v", err) }

	// Now block waiting for the command to complete
	err = cmd.Wait()
	if err != nil { Fatal(EXEC_CMD_ERROR, "command failed: %v, stderr: %s", err, string(stderrBytes)) }

	return stdoutBytes, stderrBytes
}
*/

/* Will probably need this....
func getString(v interface{}) string {
	if reflect.ValueOf(v).IsNil() { return "" }
	return fmt.Sprintf("%v", reflect.Indirect(reflect.ValueOf(v)))
}
*/
