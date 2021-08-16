package cliutils

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	dockerclient "github.com/fsouza/go-dockerclient"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/rsapss-tool/sign"
	"github.com/open-horizon/rsapss-tool/verify"
	"golang.org/x/text/language"
)

const (
	HZN_API             = "http://localhost:" + config.AnaxAPIPortDefault
	HZN_API_MAC         = "http://localhost:8081"
	JSON_INDENT         = "  "
	MUST_REGISTER_FIRST = "this command can not be run before running 'hzn register'"

	// Exit Codes
	CLI_INPUT_ERROR    = 1 // we actually don't have control over the usage exit code that kingpin returns, so use the same code for input errors we catch ourselves
	JSON_PARSING_ERROR = 3
	FILE_IO_ERROR      = 4
	HTTP_ERROR         = 5
	//EXEC_CMD_ERROR = 6
	CLI_GENERAL_ERROR = 7
	NOT_FOUND         = 8
	SIGNATURE_INVALID = 9
	EXEC_CMD_ERROR    = 10
	INTERNAL_ERROR    = 99

	// Anax API HTTP Codes
	ANAX_ALREADY_CONFIGURED = 409
	ANAX_NOT_CONFIGURED_YET = 424

	//anax configuration files
	ANAX_OVERWRITE_FILE = "/etc/default/horizon"
	ANAX_CONFIG_FILE    = "/etc/horizon/anax.json"

	// default keys will be prepended with $HOME
	DEFAULT_PRIVATE_KEY_FILE = ".hzn/keys/service.private.key"
	DEFAULT_PUBLIC_KEY_FILE  = ".hzn/keys/service.public.pem"

	// http request body types
	HTTP_REQ_BODYTYPE_DEFAULT = 0
	HTTP_REQ_BODYTYPE_BYTES   = 1
	HTTP_REQ_BODYTYPE_FILE    = 2

	// set on heartbeating api endpoints since cli calls are not coming from an agent
	NOHEARTBEAT_PARAM = "noheartbeat=true"
)

// Holds the cmd line flags that were set so other pkgs can access
type GlobalOptions struct {
	Verbose     *bool
	IsDryRun    *bool
	UsingApiKey bool // should go away soon
}

var Opts GlobalOptions

// stores the verbose messages before the GlobalOptions.Verbose is set
var TempVerboseCache = []string{}

type UserExchangeReq struct {
	Password string `json:"password"`
	Admin    bool   `json:"admin"`
	HubAdmin bool   `json:"hubAdmin"`
	Email    string `json:"email"`
}

var dockerDriversWithTagSupport = []string{"syslog", "journald", "gelf", "fluentd", "awslogs", "splunk"}
var dockerDriversWithLoggingSupport = []string{"syslog", "journald", "local", "json-file"}

func Verbose(msg string, args ...interface{}) {
	if Opts.Verbose == nil {
		// This happens before the command arguments are parsed. It saves the verbose message to a cache
		if !strings.HasSuffix(msg, "\n") {
			msg += "\n"
		}
		TempVerboseCache = append(TempVerboseCache, fmt.Sprintf(msg, args...))
		return
	} else if len(TempVerboseCache) > 0 {
		// now the command line is parsed and we know if the user wants verbose messages or not.
		// It prints out the saved verbose messages from the cache if verbose is set.
		if *Opts.Verbose {
			for _, m := range TempVerboseCache {
				if !strings.HasSuffix(m, "\n") {
					m += "\n"
				}
				fmt.Fprintf(os.Stderr, i18n.GetMessagePrinter().Sprintf("[verbose] %s", m))
			}
		}
		// flush the cache
		TempVerboseCache = []string{}
	}

	// now do the print of the current message.
	if *Opts.Verbose {
		if !strings.HasSuffix(msg, "\n") {
			msg += "\n"
		}
		fmt.Fprintf(os.Stderr, i18n.GetMessagePrinter().Sprintf("[verbose] %s", msg), args...) // send to stderr so it doesn't mess up stdout if they are piping that to jq or something like that
	}
}

func Fatal(exitCode int, msg string, args ...interface{}) {
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}
	fmt.Fprintf(os.Stderr, i18n.GetMessagePrinter().Sprintf("Error: %s", msg), args...)
	os.Exit(exitCode)
}

func Warning(msg string, args ...interface{}) {
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}
	fmt.Fprintf(os.Stderr, i18n.GetMessagePrinter().Sprintf("Warning: %s", msg), args...)
}

func IsDryRun() bool {
	return *Opts.IsDryRun
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

// Unmarshal simply calls json.Unmarshal and handles any errors
func Unmarshal(data []byte, v interface{}, errMsg string) {
	err := json.Unmarshal(data, v)
	if err != nil {
		Fatal(JSON_PARSING_ERROR, i18n.GetMessagePrinter().Sprintf("failed to unmarshal bytes from %s: %v", errMsg, err))
	}
}

// MarshalIndent calls json.MarshalIndent and handles any errors
func MarshalIndent(v interface{}, errMsg string) string {
	jsonBytes, err := json.MarshalIndent(v, "", JSON_INDENT)
	if err != nil {
		Fatal(JSON_PARSING_ERROR, i18n.GetMessagePrinter().Sprintf("failed to marshal data type from %s: %v", errMsg, err))
	}
	return string(jsonBytes)
}

//todo: this function should be removed because it was for WIoTP keys that shouldn't have the org prepended.
//		The name is also very misleading because it doesn't apply to Cloud IAM api keys.
// SetWhetherUsingApiKey is a hack because some api keys are global and shouldn't be prepended by the org
// an api key or device id/token.
func SetWhetherUsingApiKey(creds string) {
	if os.Getenv("USING_API_KEY") == "0" {
		return // this is their way of telling us that even though the creds look like an api key it isn't
	}
	// Some API keys start with: a-<6charorgid>-
	if matched, err := regexp.MatchString(`^a-[A-Za-z0-9]{6}-`, creds); err != nil {
		Fatal(INTERNAL_ERROR, i18n.GetMessagePrinter().Sprintf("problem testing api key match: %v", err))
	} else if matched {
		Opts.UsingApiKey = true
		Verbose(i18n.GetMessagePrinter().Sprintf("Using API key"))
	}
}

func NewDockerClient() (client *dockerclient.Client) {
	var err error
	dockerEndpoint := "unix:///var/run/docker.sock" // if we need this to be user configurable someday, we can get it from an env var
	if client, err = dockerclient.NewClient(dockerEndpoint); err != nil {
		Fatal(CLI_GENERAL_ERROR, i18n.GetMessagePrinter().Sprintf("unable to create docker client: %v", err))
	}
	return
}

// GetDockerAuth finds the docker credentials for this registry in ~/.docker/config.json.
// It also will try to obtains credentials from a docker credential store if it's in use.
func GetDockerAuth(domain string) (auth dockerclient.AuthConfiguration, err error) {
	var auths *dockerclient.AuthConfigurations
	if auths, err = dockerclient.NewAuthConfigurationsFromDockerCfg(); err != nil {
		return
	}

	for domainName, creds := range auths.Configs {
		if (domainName == domain) || (domain == "" && strings.Contains(domainName, "docker.io")) {
			Verbose(i18n.GetMessagePrinter().Sprintf("docker auth domainName: %v", domainName))
			auth = creds
			return
		}
	}

	// try to load cred from configured credential store
	var authPrt *dockerclient.AuthConfiguration
	authPrt, err = dockerclient.NewAuthConfigurationsFromCredsHelpers(domain)
	if err != nil {
		err = errors.New(i18n.GetMessagePrinter().Sprintf("unable to load docker credentials for %s: %v", domain, err))
		return
	}
	if authPrt != nil {
		return *authPrt, nil
	}
	return dockerclient.AuthConfiguration{}, errors.New(i18n.GetMessagePrinter().Sprintf("unable to find docker credentials for %v", domain))
}

// PushDockerImage pushes the image to its docker registry, outputting progress to stdout. It returns the repo digest. If there is an error, it prints the error and exits.
// We don't have to handle the case of a digest in the image name, because in that case we assume the image has already been pushed (that is the way to get the digest).
func PushDockerImage(client *dockerclient.Client, domain, path, tag string) (digest string) {
	var repository string // for PushImageOptions later on
	if domain == "" {
		repository = path
	} else {
		repository = domain + "/" + path
	}
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	imageName := fmt.Sprintf("%s:%s", repository, tag) // Note: tag can be the empty string
	msgPrinter.Printf("Pushing %v...", imageName)
	msgPrinter.Println()

	// Get the docker client object for this registry, and set the push options and creds
	var buf bytes.Buffer
	multiWriter := io.MultiWriter(os.Stdout, &buf)                                               // we want output of the push to go 2 places: stdout (for the user to see progess) and a variable (so we can get the digest value)
	opts := dockerclient.PushImageOptions{Name: repository, Tag: tag, OutputStream: multiWriter} // do not set InactivityTimeout because the user will ctrl-c if they think something is wrong

	var auth dockerclient.AuthConfiguration
	var err error
	if auth, err = GetDockerAuth(domain); err != nil {
		Fatal(CLI_INPUT_ERROR, msgPrinter.Sprintf("could not get docker credentials from ~/.docker/config.json: %v. Maybe you need to run 'docker login ...' to provide credentials for the image registry.", err))
	}

	// Now actually push the image
	if err = client.PushImage(opts, auth); err != nil {
		Fatal(CLI_GENERAL_ERROR, msgPrinter.Sprintf("unable to push docker image %v: %v", repository+":"+tag, err))
	}

	// Get the digest value from the docker output or the image itself.
	digest = retrieveDigest(client, buf, repository, imageName)
	return
}

//PullDockerImage pulls the image from the docker registry. Progress is written to stdout. Function returns the image digest.
//If an error occurs the error is printed then the function exits.
func PullDockerImage(client *dockerclient.Client, domain, path, tag string) (digest string, err error) {
	var repository string // for PullImageOptions later on
	if domain == "" {
		repository = path
	} else {
		repository = domain + "/" + path
	}
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	imageName := fmt.Sprintf("%s:%s", repository, tag) // Note: tag can be the empty string
	msgPrinter.Printf("Pulling %v...", imageName)
	i18n.GetMessagePrinter().Println()

	// Get the docker client object for this registry, and set the push options and creds
	var buf bytes.Buffer
	multiWriter := io.MultiWriter(os.Stdout, &buf)
	opts := dockerclient.PullImageOptions{Repository: repository, Tag: tag, OutputStream: multiWriter}

	var auth dockerclient.AuthConfiguration
	var loginErr error
	loggedIn := true
	if auth, loginErr = GetDockerAuth(domain); loginErr != nil {
		Verbose(i18n.GetMessagePrinter().Sprintf("unable to get docker auth for docker.io or %s domain: %v", domain, err))
		loggedIn = false
	}

	//Pull the image
	if err := client.PullImage(opts, auth); err != nil {
		if !loggedIn {
			err = errors.New(msgPrinter.Sprintf("unable to pull docker image %v. Docker credentials were not found. Maybe you need to run 'docker login ...' if the image registry is private. Error: %v: %v", repository+":"+tag, err, loginErr))
		}
		err = errors.New(msgPrinter.Sprintf("unable to pull docker image %v: %v", repository+":"+tag, err))
		Verbose(err.Error())
	}

	// Get the digest value from the docker output or the image itself.
	digest = retrieveDigest(client, buf, repository, imageName)

	return

}

// Get the image digest so that it can be set into the published service definition. The digest will be in
// the stdout from the docker pull/push that was done previously, or it can be retrieved from the image itself.
func retrieveDigest(client *dockerclient.Client, buf bytes.Buffer, repository string, imageName string) (digest string) {

	msgPrinter := i18n.GetMessagePrinter()

	// Get the digest value that docker calculated for the image
	reDigest := regexp.MustCompile(`\s+digest:\s+(\S+)\s+size:`)
	var matches []string
	if matches = reDigest.FindStringSubmatch(buf.String()); len(matches) < 2 {
		Verbose(msgPrinter.Sprintf("Could not find the image digest in the docker output, retrieving image digest directly from the image."))
	} else {
		digest = matches[1]
		return
	}

	// The digest was not in the stdout response, try to find it in the image's metadata.
	if image, err := client.InspectImage(imageName); err != nil {
		Fatal(CLI_GENERAL_ERROR, msgPrinter.Sprintf("could not inspect image %v: %v.", imageName, err))
	} else {
		for _, rDigest := range image.RepoDigests {
			if strings.Contains(rDigest, repository) {
				_, _, _, digest = cutil.ParseDockerImagePath(rDigest)
				return
			}
		}
		Fatal(CLI_GENERAL_ERROR, msgPrinter.Sprintf("could not find digest for image %v.", imageName))
	}
	return
}

// OrgAndCreds prepends the org to creds (separated by /) unless creds already has an org prepended
func OrgAndCreds(org, creds string) string {
	// org is the org of the resource being accessed, so if they want to use creds from a different org, the prepend that org to creds before calling this
	if Opts.UsingApiKey || os.Getenv("USING_API_KEY") == "1" { //todo: remove because this was for WIoTP keys that shouldn't have the org prepended
		return creds
	}
	id, _ := SplitIdToken(creds) // only look for the / in the id, because the token is more likely to have special chars
	if strings.Contains(id, "/") {
		return creds // already has the org at the beginning
	}
	return org + "/" + creds
}

// AddSlash prepends "/" to the id if it is not the empty string and returns it. This is useful when id is the last thing in the route.
func AddSlash(id string) string {
	if id == "" {
		return id
	}
	return "/" + id
}

// TrimOrg returns id with the leading "<org>/" removed, if it was there. This is useful because in list sub-cmds id is shown with
// the org prepended, but when the id is put in routes it can not have the org prepended, because org is already earlier in the route.
func TrimOrg(org, id string) (string, string) {
	substrings := strings.Split(id, "/")
	if len(substrings) <= 1 { // this means id was empty, or did not contain '/'
		return org, id
	} else if len(substrings) == 2 {
		return substrings[0], substrings[1] // in this case the org the prepended to the id will override the org they may have specified thru the -o flag or env var
	} else {
		Fatal(CLI_INPUT_ERROR, i18n.GetMessagePrinter().Sprintf("the id can not contain more than 1 '/'"))
	}
	return "", "" // will never get here
}

// Add the given org to the id if the id does not already contain an org
func AddOrg(org, id string) string {
	substrings := strings.Split(id, "/")
	if len(substrings) <= 1 { // this means id was empty, or did not contain '/'
		return fmt.Sprintf("%v/%v", org, id)
	} else if len(substrings) == 2 {
		return id
	} else {
		Fatal(CLI_INPUT_ERROR, i18n.GetMessagePrinter().Sprintf("the id can not contain more than 1 '/'"))
	}
	return "" // will never get here
}

// ReadStdin reads from stdin, and returns it as a byte array.
func ReadStdin() []byte {
	fileBytes, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		Fatal(FILE_IO_ERROR, i18n.GetMessagePrinter().Sprintf("reading stdin failed: %v", err))
	}
	return fileBytes
}

// ReadFile reads from a file or stdin, and returns it as a byte array.
func ReadFile(filePath string) []byte {
	var fileBytes []byte
	var err error
	if filePath == "-" {
		fileBytes, err = ioutil.ReadAll(os.Stdin)
	} else {
		fileBytes, err = ioutil.ReadFile(filePath)
	}
	if err != nil {
		Fatal(FILE_IO_ERROR, i18n.GetMessagePrinter().Sprintf("reading %s failed: %v", filePath, err))
	}
	return fileBytes
}

// ExpandMapping is used in ExpandEnv() to print a warning if the env var is not defined.
func ExpandMapping(envVarName string) string {
	envVarValue := os.Getenv(envVarName)
	if envVarValue == "" {
		i18n.GetMessagePrinter().Printf("Warning: environment variable '%s' is referenced in input file, but not defined in the environment.", envVarName)
		i18n.GetMessagePrinter().Println()
	}
	return envVarValue
}

// ExpandEnv is equivalent to os.ExpandEnv(), except prints a warning when an env var is not defined
func ExpandEnv(s string) string {
	return os.Expand(s, ExpandMapping)
}

// ReadJsonFile reads json from a file or stdin, eliminates comments, substitutes env vars, and returns it.
func ReadJsonFile(filePath string) []byte {
	var fileBytes []byte
	var err error
	if filePath == "-" {
		fileBytes, err = ioutil.ReadAll(os.Stdin)
	} else {
		fileBytes, err = ioutil.ReadFile(filePath)
	}
	if err != nil {
		Fatal(FILE_IO_ERROR, i18n.GetMessagePrinter().Sprintf("reading %s failed: %v", filePath, err))
	}

	// Remove /* */ comments
	re := regexp.MustCompile(`(?s)/\*.*?\*/`)
	newBytes := re.ReplaceAll(fileBytes, nil)

	// Replace env vars
	if os.Getenv("HZN_DONT_SUBST_ENV_VARS") == "1" {
		return newBytes
	}
	str := ExpandEnv(string(newBytes))
	return []byte(str)
}

// ConfirmRemove prompts the user to confirm they want to run the destructive cmd
func ConfirmRemove(question string) {
	// Prompt the user to make sure he/she wants to do this
	fmt.Print(question + " [y/N]: ")
	var response string

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		Fatal(CLI_INPUT_ERROR, i18n.GetMessagePrinter().Sprintf("Error reading input, error %v", err))
	}
	response = strings.TrimSuffix(response, "\n")
	response = strings.ToLower(response)

	if strings.TrimSpace(response) != "y" {
		i18n.GetMessagePrinter().Printf("Exiting.")
		i18n.GetMessagePrinter().Println()
		os.Exit(0)
	}
}

// WithDefaultKeyFile returns the keyFile path if it has a non-blank value, or the default keys.
func WithDefaultKeyFile(keyFile string, isPublic bool) string {
	var err error

	if keyFile != "" {
		return VerifySigningKeyInput(keyFile, isPublic)
	}
	// get default file names if input is empty
	if keyFile, err = GetDefaultSigningKeyFile(isPublic); err != nil {
		Fatal(CLI_GENERAL_ERROR, err.Error())
	// convert to absolute path
	} else if keyFile, err = filepath.Abs(keyFile); err != nil {
		Fatal(CLI_GENERAL_ERROR, i18n.GetMessagePrinter().Sprintf("Failed to get absolute path for file %v. %v", keyFile, err))
	// check file exist
	} else if _, err := os.Stat(keyFile); err != nil {
		if os.IsNotExist(err) {
			return ""
		} else {
			Fatal(CLI_GENERAL_ERROR, i18n.GetMessagePrinter().Sprintf("Error checking absolute path for file %v. %v", keyFile, err))
		}
	}
	return keyFile
}

// WithDefaultEnvVar returns the specified flag ptr if it has a non-blank value, or the env var value.
func WithDefaultEnvVar(flag *string, envVarName string) *string {
	if *flag != "" {
		return flag
	}
	newFlag := os.Getenv(envVarName)
	if newFlag != "" {
		return &newFlag
	}
	return flag // it is empty, but we did not find an env var value
}

// RequiredWithDefaultEnvVar returns the specified flag ptr if it has a non-blank value, or the env var value.
func RequiredWithDefaultEnvVar(flag *string, envVarName, errMsg string) *string {
	if *flag != "" {
		return flag
	}
	newFlag := os.Getenv(envVarName)
	if newFlag != "" {
		return &newFlag
	}
	Fatal(CLI_INPUT_ERROR, errMsg)
	return flag // won't ever happen, here just to make intellij happy
}

// GetHorizonUrlBase returns the base part of the horizon api url (which can be overridden by env var HORIZON_URL)
func GetHorizonUrlBase() string {
	envVar := os.Getenv("HORIZON_URL")
	if envVar != "" {
		return envVar
	}
	if runtime.GOOS == "darwin" {
		return HZN_API_MAC
	} else {
		return HZN_API
	}
}

// GetHorizonContainerIndex returns expected horizon container index based on the HORIZON_URL port binding
// e.g. if horizon container is running on 8081 port it's index would be 1 and expected container name is horizon1
func GetHorizonContainerIndex() (int, error) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	horizonUrl, err := url.Parse(GetHorizonUrlBase())
	if err != nil {
		return -1, fmt.Errorf(msgPrinter.Sprintf("Error parsing HORIZON_URL: %v", err))
	}
	_, port, err := net.SplitHostPort(horizonUrl.Host)
	if err != nil {
		return -1, fmt.Errorf(msgPrinter.Sprintf("Error parsing host of the HORIZON_URL: %v", err))
	}
	portInt, err := strconv.Atoi(port)
	if err != nil {
		return -1, fmt.Errorf(msgPrinter.Sprintf("Error parsing port of the HORIZON_URL: %v", err))
	}
	if portInt < 8080 {
		return -1, fmt.Errorf(msgPrinter.Sprintf("Unexpected port of the HORIZON_URL: %v", portInt))
	}
	return portInt - 8080, nil
}

// Returns the agbot native url. If HZN_AGBOT_API not set, use HORIZON_URL
func GetAgbotUrlBase() string {
	envVar := os.Getenv("HZN_AGBOT_API")
	if envVar != "" {
		return envVar
	}

	return GetHorizonUrlBase()
}

// Returns the url for the agbot secure API.
// It first gets it from the HZN_AGBOT_URL env variable.
// The HZN_AGBOT_URL is available if it is in /etc/default/horizon file.
func GetAgbotSecureAPIUrlBase() string {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	envVar := os.Getenv("HZN_AGBOT_URL")
	Verbose(msgPrinter.Sprintf("The agbot url: %v", envVar))
	return envVar
}

// GetRespBodyAsString converts an http response body to a string
func GetRespBodyAsString(responseBody io.ReadCloser) string {
	if responseBody == nil {
		return ""
	}

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(responseBody); err != nil {
		Fatal(HTTP_ERROR, i18n.GetMessagePrinter().Sprintf("Error reading HTTP response, error %v", err))
	}
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

func printHorizonRestError(apiMethod string, err error) {
	msg := ""
	if os.Getenv("HORIZON_URL") == "" {
		statusCommand := "systemctl status horizon"
		statusURL := "curl http://localhost:8081/status"
		if runtime.GOOS == "darwin" {
			statusCommand = "docker ps | grep horizon"
		}
		msg = i18n.GetMessagePrinter().Sprintf("Can't connect to the Horizon REST API to run %s. Run '%s' to check if the Horizon agent is running. Or run '%s' to check the Horizon agent status. Or set HORIZON_URL to connect to another local port that is connected to a remote Horizon agent via a ssh tunnel. Specific error is: %v", apiMethod, statusCommand, statusURL, err)
	} else {
		msg = i18n.GetMessagePrinter().Sprintf("Can't connect to the Horizon REST API to run %s. Maybe the ssh tunnel associated with that port is down? Or maybe the remote Horizon agent at the other end of that tunnel is down. Specific error is: %v", apiMethod, err)
	}
	Fatal(HTTP_ERROR, msg)
}

// HorizonGet runs a GET on the anax api and fills in the specified structure with the json.
// If the list of goodHttpCodes is not empty and none match the actual http code, it will exit with an error. Otherwise the actual code is returned.
// Only if the actual code matches the 1st element in goodHttpCodes, will it parse the body into the specified structure.
// If quiet is true, then the error will be returned, the function returns back to the caller instead of exiting out.
func HorizonGet(urlSuffix string, goodHttpCodes []int, structure interface{}, quiet bool) (httpCode int, retError error) {
	retError = nil

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	httpClient := GetHTTPClient(0)

	url := GetHorizonUrlBase() + "/" + urlSuffix
	apiMsg := http.MethodGet + " " + url
	Verbose(apiMsg)
	// Create the request and run it
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		Fatal(HTTP_ERROR, msgPrinter.Sprintf("%s new request failed: %v", apiMsg, err))
	}
	req.Close = true
	req.Header.Add("Accept", "application/json")

	// add the language request to the http header
	localeTag, err := i18n.GetLocale()
	if err != nil {
		localeTag = language.English
	}
	req.Header.Add("Accept-Language", localeTag.String())

	resp, err := httpClient.Do(req)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		if quiet {
			if os.Getenv("HORIZON_URL") == "" {
				statusCommand := "systemctl status horizon"
				statusURL := "curl http://localhost:8081/status"
				if runtime.GOOS == "darwin" {
					statusCommand = "docker ps | grep horizon"
				}
				retError = fmt.Errorf(msgPrinter.Sprintf("Can't connect to the Horizon REST API to run %s. Run '%s' to check if the Horizon agent is running. Or run '%s' to check the Horizon agent status. Or set HORIZON_URL to connect to another local port that is connected to a remote Horizon agent via a ssh tunnel. Specific error is: %v", apiMsg, statusCommand, statusURL, err))
			} else {
				retError = fmt.Errorf(msgPrinter.Sprintf("Can't connect to the Horizon REST API to run %s. Maybe the ssh tunnel associated with that port is down? Or maybe the remote Horizon agent at the other end of that tunnel is down. Specific error is: %v", apiMsg, err))
			}
			return
		} else {
			printHorizonRestError(apiMsg, err)
		}
	}
	httpCode = resp.StatusCode
	Verbose(msgPrinter.Sprintf("HTTP code: %d", httpCode))
	if !isGoodCode(httpCode, goodHttpCodes) {
		if quiet {
			retError = fmt.Errorf(msgPrinter.Sprintf("Bad HTTP code from %s: %d", apiMsg, httpCode))
			return
		} else {
			Fatal(HTTP_ERROR, msgPrinter.Sprintf("bad HTTP code from %s: %d", apiMsg, httpCode))
		}
	}
	if httpCode == goodHttpCodes[0] {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			if quiet {
				retError = fmt.Errorf(msgPrinter.Sprintf("Failed to read body response from %s: %v", apiMsg, err))
				return
			} else {
				Fatal(HTTP_ERROR, msgPrinter.Sprintf("failed to read body response from %s: %v", apiMsg, err))
			}
		}
		switch s := structure.(type) {
		case *string:
			// Just return the unprocessed response body
			*s = string(bodyBytes)
		default:
			// Put the response body in the specified struct
			err = json.Unmarshal(bodyBytes, structure)
			if err != nil {
				if quiet {
					retError = fmt.Errorf(msgPrinter.Sprintf("Failed to unmarshal body response from %s: %v", apiMsg, err))
					return
				} else {
					Fatal(JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal body response from %s: %v", apiMsg, err))
				}
			}
		}
	}
	return
}

// HorizonDelete runs a DELETE on the anax api.
// If the list of goodHttpCodes is not empty and none match the actual http code, it will exit with an error. Otherwise the actual code is returned.
func HorizonDelete(urlSuffix string, goodHttpCodes []int, expectedHttpErrorCodes []int, quiet bool) (httpCode int, retError error) {
	url := GetHorizonUrlBase() + "/" + urlSuffix
	apiMsg := http.MethodDelete + " " + url

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	Verbose(apiMsg)
	if IsDryRun() {
		return 204, nil
	}
	httpClient := GetHTTPClient(0)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		if quiet {
			retError = fmt.Errorf(msgPrinter.Sprintf("%s new request failed: %v", apiMsg, err))
			return
		} else {
			Fatal(HTTP_ERROR, msgPrinter.Sprintf("%s new request failed: %v", apiMsg, err))
		}
	}
	req.Close = true

	resp, err := httpClient.Do(req)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		if quiet {
			if os.Getenv("HORIZON_URL") == "" {
				statusCommand := "systemctl status horizon"
				statusURL := "curl http://localhost:8081/status"
				if runtime.GOOS == "darwin" {
					statusCommand = "docker ps | grep horizon"
				}
				retError = fmt.Errorf(msgPrinter.Sprintf("Can't connect to the Horizon REST API to run %s. Run '%s' to check if the Horizon agent is running. Or run '%s' to check the Horizon agent status. Or set HORIZON_URL to connect to another local port that is connected to a remote Horizon agent via a ssh tunnel. Specific error is: %v", apiMsg, statusCommand, statusURL, err))
			} else {
				retError = fmt.Errorf(msgPrinter.Sprintf("Can't connect to the Horizon REST API to run %s. Maybe the ssh tunnel associated with that port is down? Or maybe the remote Horizon agent at the other end of that tunnel is down. Specific error is: %v", apiMsg, err))
			}
			return
		} else {
			printHorizonRestError(apiMsg, err)
		}
	}
	httpCode = resp.StatusCode
	Verbose(msgPrinter.Sprintf("HTTP code: %d", httpCode))
	if isGoodCode(httpCode, goodHttpCodes) {
		return
	} else if isGoodCode(httpCode, expectedHttpErrorCodes) {
		err_msg := GetRespBodyAsString(resp.Body)
		retError = fmt.Errorf(err_msg)
		return
	} else {
		err_msg := msgPrinter.Sprintf("bad HTTP code %d from %s: %s", httpCode, apiMsg, GetRespBodyAsString(resp.Body))
		if quiet {
			retError = fmt.Errorf(err_msg)
			return
		} else {
			Fatal(HTTP_ERROR, err_msg)
		}
	}
	return
}

// HorizonPutPost runs a PUT or POST to the anax api to create or update a resource.
// If the list of goodHttpCodes is not empty and none match the actual http code, it will exit with an error. Otherwise the actual code is returned.
func HorizonPutPost(method string, urlSuffix string, goodHttpCodes []int, body interface{}, exitOnErr bool) (httpCode int, resp_body string, err error) {
	url := GetHorizonUrlBase() + "/" + urlSuffix
	apiMsg := method + " " + url
	Verbose(apiMsg)
	if IsDryRun() {
		return 201, "", nil
	}
	httpClient := GetHTTPClient(0)

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Prepare body
	var jsonBytes []byte
	bodyIsBytes := false
	switch b := body.(type) {
	// If the body is a byte array or string, we treat it like a file being uploaded (not multi-part)
	case []byte:
		jsonBytes = b
		bodyIsBytes = true
	case string:
		jsonBytes = []byte(b)
		bodyIsBytes = true
	// Else it is a struct so assume it should be sent as json
	default:
		var err error
		jsonBytes, err = json.Marshal(body)
		if exitOnErr && err != nil {
			Fatal(JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal body for %s: %v", apiMsg, err))
		} else if err != nil {
			return 0, "", err
		}
	}
	requestBody := bytes.NewBuffer(jsonBytes)

	// Create the request and run it
	req, err := http.NewRequest(method, url, requestBody)
	if exitOnErr && err != nil {
		Fatal(HTTP_ERROR, msgPrinter.Sprintf("%s new request failed: %v", apiMsg, err))
	} else if err != nil {
		return 0, "", err
	}
	req.Close = true
	req.Header.Add("Accept", "application/json")
	if bodyIsBytes {
		req.Header.Add("Content-Length", strconv.Itoa(len(jsonBytes)))
	} else {
		req.Header.Add("Content-Type", "application/json")
	}
	resp, err := httpClient.Do(req)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil && exitOnErr {
		printHorizonRestError(apiMsg, err)
	} else if err != nil {
		return 0, "", err
	}

	// Process the response
	httpCode = resp.StatusCode
	Verbose(msgPrinter.Sprintf("HTTP code: %d", httpCode))

	resp_body = GetRespBodyAsString(resp.Body)
	if !isGoodCode(httpCode, goodHttpCodes) {
		if exitOnErr {
			Fatal(HTTP_ERROR, msgPrinter.Sprintf("bad HTTP code %d from %s: %s", httpCode, apiMsg, resp_body))
		} else {
			return 0, "", fmt.Errorf(msgPrinter.Sprintf("bad HTTP code %d from %s: %s", httpCode, apiMsg, resp_body))
		}
	}
	return
}

// Runs a GET to the agbot secure API and fills in the specified json structure. if the structure is just a string, fill in the raw json.
// If the list of goodHttpCodes is non-empty and none match the actual http code, it will exit with an error; otherwise, the actual code is returned
func AgbotGet(urlSuffix, credentials string, goodHttpCodes []int, structure interface{}) (httpCode int) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// check the agbot url
	agbot_url := GetAgbotSecureAPIUrlBase()
	if agbot_url == "" {
		Fatal(HTTP_ERROR, msgPrinter.Sprintf("HZN_AGBOT_URL is not defined"))
	}

	// query the agbot secure api
	httpCode = ExchangeGet("Agbot", agbot_url, urlSuffix, credentials, goodHttpCodes, structure)

	// ExchangeGet checks the http code, so we can just directly return
	return httpCode
}

// Runs a LIST to the agbot secure API and fills in the specified json structure. if the structure is just a string, fill in the raw json.
// If the list of goodHttpCodes is non-empty and none match the actual http code, it will exit with an error; otherwise, the actual code is returned
func AgbotList(urlSuffix, credentials string, goodHttpCodes []int, structure interface{}) (httpCode int) {

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// check the agbot url
	agbot_url := GetAgbotSecureAPIUrlBase()
	if agbot_url == "" {
		Fatal(HTTP_ERROR, msgPrinter.Sprintf("HZN_AGBOT_URL is not defined"))
	}

	// query the agbot secure api
	url := agbot_url + "/" + urlSuffix
	apiMsg := "LIST " + url

	Verbose(apiMsg)

	httpClient := GetHTTPClient(config.HTTPRequestTimeoutS)

	resp := InvokeRestApi(httpClient, "LIST", url, credentials, nil, "Agbot", apiMsg)
	if resp.Body != nil {
		defer resp.Body.Close()
	}

	respBody := io.Reader(resp.Body)
	bodyBytes, err := ioutil.ReadAll(respBody)
	if err != nil {
		Fatal(HTTP_ERROR, msgPrinter.Sprintf("failed to read body response from %s: %v", apiMsg, err))
	}
	httpCode = resp.StatusCode
	Verbose(msgPrinter.Sprintf("HTTP code: %d", httpCode))
	if !isGoodCode(httpCode, goodHttpCodes) {
		Fatal(HTTP_ERROR, msgPrinter.Sprintf("bad HTTP code %d from %s, output: %s", httpCode, apiMsg, string(bodyBytes)))
	}

	if len(bodyBytes) > 0 && structure != nil { // the DP front-end of exchange will return nothing when auth problem
		switch s := structure.(type) {
		case *[]byte:
			// This is the signal that they want the raw body back
			*s = bodyBytes
		case *string:
			// If the structure to fill in is just a string, unmarshal/remarshal it to get it in json indented form, and then return as a string
			//todo: this gets it in json indented form, but also returns the fields in random order (because they were interpreted as a map)
			var jsonStruct interface{}
			err = json.Unmarshal(bodyBytes, &jsonStruct)
			if err != nil {
				Fatal(JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal exchange body response from %s: %v", apiMsg, err))
			}
			jsonBytes, err := json.MarshalIndent(jsonStruct, "", JSON_INDENT)
			if err != nil {
				Fatal(JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal exchange output from %s: %v", apiMsg, err))
			}
			*s = string(jsonBytes)
		default:
			err = json.Unmarshal(bodyBytes, structure)
			if err != nil {
				Fatal(JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal exchange body response from %s: %v", apiMsg, err))
			}
		}
	}
	return
}

// Runs a PUT, POST, or PATCH to the agbot secure API to create or update a resource. If body is a string, it will be given to the exhcnage
// as json. Otherwise, the struct will be marshaled to json.
// If the list of goodHttpCodes is non-empty and none match the actual http code, it will exit with an error; otherwise, the actual code is returned
func AgbotPutPost(method, urlSuffix, credentials string, goodHttpCodes []int, body interface{}, structure interface{}) (httpCode int) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// check the agbot url
	agbot_url := GetAgbotSecureAPIUrlBase()
	if agbot_url == "" {
		Fatal(HTTP_ERROR, msgPrinter.Sprintf("HZN_AGBOT_URL is not defined"))
	}

	// query the agbot secure api
	httpCode = ExchangePutPost("Agbot", method, agbot_url, urlSuffix, credentials, goodHttpCodes, body, structure)

	// ExchangePutPost checks the http code, so we can just directly return
	return httpCode
}

// Runs a DELETE to the agbot secure API to delete a resource.
// If the list of goodHttpCodes is non-empty and none match the actual http code, it will exit with an error; otherwise, the actual code is returned
func AgbotDelete(urlSuffix, credentials string, goodHttpCodes []int) (httpCode int) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// check the agbot url
	agbot_url := GetAgbotSecureAPIUrlBase()
	if agbot_url == "" {
		Fatal(HTTP_ERROR, msgPrinter.Sprintf("HZN_AGBOT_URL is not defined"))
	}

	// query the agbot secure api
	httpCode = ExchangeDelete("Agbot", agbot_url, urlSuffix, credentials, goodHttpCodes)

	// ExchangeDelete checks the http code, so we can just directly return
	return httpCode
}

// get a value keyed by key in a file. The file contains key=value for each line.
func GetEnvVarFromFile(filename string, key string) (string, error) {
	fHandle, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		} else {
			return "", err
		}
	}
	defer fHandle.Close()

	scanner := bufio.NewScanner(fHandle)
	for scanner.Scan() {
		lineContent := string(scanner.Bytes())
		if strings.Contains(lineContent, key) {
			key_value := strings.Split(lineContent, "=")
			// comment line
			if strings.Contains(key_value[0], "#") {
				continue
			} else if len(key_value) > 1 {
				// trim the leading and trailing space, single quote and double quotes
				s := key_value[1]
				s = strings.TrimSpace(s)
				s = strings.Trim(s, "'")
				s = strings.Trim(s, "\"")
				return s, nil
			} else {
				return "", nil
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", nil
}

// Get the anax configuration from the given configuration file.
func GetAnaxConfig(configFile string) (*config.HorizonConfig, error) {
	_, err := os.Stat(configFile)
	if os.IsNotExist(err) {
		return nil, nil
	}

	if byteValue, err := ioutil.ReadFile(configFile); err != nil {
		return nil, err
	} else {
		var anaxConfig config.HorizonConfig
		if err := json.Unmarshal(byteValue, &anaxConfig); err != nil {
			return nil, fmt.Errorf(i18n.GetMessagePrinter().Sprintf("Failed to unmarshal bytes. %v", err))
		} else {
			return &anaxConfig, nil
		}
	}
}

// GetIcpCertPath gets the path of the certificate for the exchange and CSS (collectively referred to as the
// management hub). The variable name changed, so it could be the old one or the new one from the '/etc/default/horizon' file.
// If the env var is not set or the field is not found an empty string is returned.
func GetIcpCertPath() string {
	if value := os.Getenv(config.OldMgmtHubCertPath); value != "" {
		return value
	} else if value := os.Getenv(config.ManagementHubCertPath); value != "" {
		return value
	} else if value, err := GetEnvVarFromFile(ANAX_OVERWRITE_FILE, config.OldMgmtHubCertPath); err != nil {
		Verbose(i18n.GetMessagePrinter().Sprintf("Error getting %v from %v: %v", config.OldMgmtHubCertPath, ANAX_OVERWRITE_FILE, err))
	} else if value != "" {
		return value
	} else if value, err := GetEnvVarFromFile(ANAX_OVERWRITE_FILE, config.ManagementHubCertPath); err != nil {
		Verbose(i18n.GetMessagePrinter().Sprintf("Error getting %v from %v: %v", config.ManagementHubCertPath, ANAX_OVERWRITE_FILE, err))
	} else {
		return value
	}
	return ""
}

//TrustIcpCert adds the icp cert file to be trusted in calls made by the given http client
func TrustIcpCert(httpClient *http.Client) error {
	icpCertPath := GetIcpCertPath()
	if icpCertPath != "" {
		icpCert, err := ioutil.ReadFile(icpCertPath)
		if err != nil {
			return fmt.Errorf(i18n.GetMessagePrinter().Sprintf("Encountered error reading ICP cert file %v: %v", icpCertPath, err))
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(icpCert)

		transport := httpClient.Transport.(*http.Transport)
		transport.TLSClientConfig.RootCAs = caCertPool

	}
	return nil
}

// Get exchange url from /etc/default/horizon file. if not set, check /etc/horizon/anax.json file
func GetExchangeUrlFromAnax() string {
	if value, err := GetEnvVarFromFile(ANAX_OVERWRITE_FILE, "HZN_EXCHANGE_URL"); err != nil {
		Verbose(i18n.GetMessagePrinter().Sprintf("Error getting HZN_EXCHANGE_URL from %v. %v", ANAX_OVERWRITE_FILE, err))
	} else if value != "" {
		return value
	}

	if anaxConfig, err := GetAnaxConfig(ANAX_CONFIG_FILE); err != nil {
		Verbose(i18n.GetMessagePrinter().Sprintf("Error getting ExchangeUrl from %v. %v", ANAX_CONFIG_FILE, err))
	} else if anaxConfig != nil {
		return anaxConfig.Edge.ExchangeURL
	}

	return ""
}

// GetExchangeUrlFromAnax returns a string with the file or envvar that GetExchangeUrlFromAnax is getting the exchange url from
func GetExchangeUrlLocationFromAnax() string {
	if value, err := GetEnvVarFromFile(ANAX_OVERWRITE_FILE, "HZN_EXCHANGE_URL"); err != nil {
		Verbose(i18n.GetMessagePrinter().Sprintf("Error getting HZN_EXCHANGE_URL from %v. %v", ANAX_OVERWRITE_FILE, err))
	} else if value != "" {
		return ANAX_OVERWRITE_FILE
	}

	if anaxConfig, err := GetAnaxConfig(ANAX_CONFIG_FILE); err != nil {
		Verbose(i18n.GetMessagePrinter().Sprintf("Error getting ExchangeUrl from %v. %v", ANAX_CONFIG_FILE, err))
	} else if anaxConfig != nil {
		return ANAX_CONFIG_FILE
	}

	return ""
}

// GetExchangeUrl returns the exchange url from the env var or anax api
func GetExchangeUrl() string {
	exchUrl := os.Getenv("HZN_EXCHANGE_URL")

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if exchUrl == "" {
		Verbose(msgPrinter.Sprintf("HZN_EXCHANGE_URL is not set, get it from horizon agent configuration on the node."))
		value := GetExchangeUrlFromAnax()
		if value != "" {
			exchUrl = value
		} else {
			Fatal(CLI_GENERAL_ERROR, msgPrinter.Sprintf("Could not get the Exchange url from environment variable HZN_EXCHANGE_URL or the horizon agent"))
		}
	}

	exchUrl = strings.TrimSuffix(exchUrl, "/")                 // anax puts a trailing slash on it
	if Opts.UsingApiKey || os.Getenv("USING_API_KEY") == "1" { //todo: remove because this was for WIoTP keys that shouldn't have the org prepended
		re := regexp.MustCompile(`edgenode$`)
		exchUrl = re.ReplaceAllLiteralString(exchUrl, "edge")
	}

	Verbose(msgPrinter.Sprintf("The exchange url: %v", exchUrl))
	return exchUrl
}

// GetExchangeUrlLocation returns a string with the filename or envvar that GetExchangeUrl is getting the exchange url from
func GetExchangeUrlLocation() string {

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	exchUrlLoc := "HZN_EXCHANGE_URL"
	exchUrl := os.Getenv("HZN_EXCHANGE_URL")
	if exchUrl == "" {
		Verbose(msgPrinter.Sprintf("HZN_EXCHANGE_URL is not set, get it from horizon agent configuration on the node."))
		location := GetExchangeUrlLocationFromAnax()
		if location != "" {
			exchUrlLoc = location
		} else {
			Fatal(CLI_GENERAL_ERROR, msgPrinter.Sprintf("Could not get the Exchange url from environment variable HZN_EXCHANGE_URL or the horizon agent"))
		}
	}

	exchUrl = strings.TrimSuffix(exchUrl, "/")                 // anax puts a trailing slash on it
	if Opts.UsingApiKey || os.Getenv("USING_API_KEY") == "1" { // todo: remove because this was for WIoTP keys that shouldn't have the org prepended
		re := regexp.MustCompile(`edgenode$`)
		exchUrl = re.ReplaceAllLiteralString(exchUrl, "edge")
	}

	Verbose(msgPrinter.Sprintf("The exchange url: %v", exchUrl))
	return exchUrlLoc
}

// Get mms url from /etc/default/horizon file. if not set, check /etc/horizon/anax.json file
func GetMMSUrlFromAnax() string {
	if value, err := GetEnvVarFromFile(ANAX_OVERWRITE_FILE, "HZN_FSS_CSSURL"); err != nil {
		Verbose(i18n.GetMessagePrinter().Sprintf("Error getting HZN_FSS_CSSURL from %v. %v", ANAX_OVERWRITE_FILE, err))
	} else if value != "" {
		return value
	}

	if anaxConfig, err := GetAnaxConfig(ANAX_CONFIG_FILE); err != nil {
		Verbose(i18n.GetMessagePrinter().Sprintf("Error getting model management service Url from %v. %v", ANAX_CONFIG_FILE, err))
	} else if anaxConfig != nil {
		return anaxConfig.GetCSSURL()
	}

	return ""
}

// GetMMSUrl returns the exchange url from the env var or anax api
func GetMMSUrl() string {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	mmsUrl := os.Getenv("HZN_FSS_CSSURL")
	if mmsUrl == "" {
		Verbose(msgPrinter.Sprintf("HZN_FSS_CSSURL is not set, get it from horizon agent configuration on the node."))
		value := GetMMSUrlFromAnax()
		if value != "" {
			mmsUrl = value
		} else {
			Fatal(CLI_GENERAL_ERROR, msgPrinter.Sprintf("Could not get the model management service url from environment variable HZN_FSS_CSSURL or the horizon agent"))
		}
	}

	mmsUrl = strings.TrimSuffix(mmsUrl, "/")                   // anax puts a trailing slash on it
	if Opts.UsingApiKey || os.Getenv("USING_API_KEY") == "1" { // todo: remove because this was for WIoTP keys that shouldn't have the org prepended
		re := regexp.MustCompile(`edgenode$`)
		mmsUrl = re.ReplaceAllLiteralString(mmsUrl, "edge")
	}

	Verbose(msgPrinter.Sprintf("The model management service url: %v", mmsUrl))
	return mmsUrl
}

// GetSdoSvcUrl returns the url of the Horizon mgmt hub SDO owner service from env var or anax overwrite file
func GetSdoSvcUrl() string {
	msgPrinter := i18n.GetMessagePrinter()

	sdoUrl := os.Getenv("HZN_SDO_SVC_URL")
	if sdoUrl == "" {
		Verbose(msgPrinter.Sprintf("HZN_SDO_SVC_URL is not set, get it from %s.", ANAX_OVERWRITE_FILE))
		var err error
		if sdoUrl, err = GetEnvVarFromFile(ANAX_OVERWRITE_FILE, "HZN_SDO_SVC_URL"); err != nil {
			Verbose(i18n.GetMessagePrinter().Sprintf("Error getting HZN_SDO_SVC_URL from %v: %v", ANAX_OVERWRITE_FILE, err))
		} else if sdoUrl == "" {
			Fatal(CLI_GENERAL_ERROR, msgPrinter.Sprintf("Could not get the HZN_SDO_SVC_URL value from the environment, %s, or one of the hzn.json files", ANAX_OVERWRITE_FILE))
		}
	}
	sdoUrl = strings.TrimSuffix(sdoUrl, "/")

	Verbose(msgPrinter.Sprintf("The SDO service url: %v", sdoUrl))
	return sdoUrl
}

func printHorizonServiceRestError(horizonService string, apiMethod string, err error) {
	serviceEnvVarName := "HZN_EXCHANGE_URL"
	article := "an"
	if horizonService == "Model Management Service" {
		serviceEnvVarName = "HZN_FSS_CSSURL"
		article = "a"
	} else if horizonService == "SDO Owner Service" {
		serviceEnvVarName = "HZN_SDO_SVC_URL"
		article = "a"
	}

	if os.Getenv(serviceEnvVarName) == "" {
		Fatal(HTTP_ERROR, i18n.GetMessagePrinter().Sprintf("Can't connect to the Horizon %v REST API to run %s. Set %v to use %v %v other than the one the Horizon Agent is currently configured for. Specific error is: %v", horizonService, apiMethod, serviceEnvVarName, article, horizonService, err))
	} else {
		Fatal(HTTP_ERROR, i18n.GetMessagePrinter().Sprintf("Can't connect to the Horizon %v REST API to run %s. Maybe %v is set incorrectly? Or unset %v to use the %v that the Horizon Agent is configured for. Specific error is: %v", horizonService, apiMethod, serviceEnvVarName, serviceEnvVarName, horizonService, err))
	}

}

// creates an request body for http calls. Only PUT/PATCH/POST calls has request body.
func createRequestBody(body interface{}, apiMsg string) (io.Reader, int, int) {

	bodyType := HTTP_REQ_BODYTYPE_DEFAULT

	if body == nil {
		return nil, 0, bodyType
	}

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Prepare body
	var jsonBytes []byte
	bodyLen := 0

	switch b := body.(type) {
	// If the body is a byte array, we treat it like a file being uploaded (not multi-part)
	case []byte:
		jsonBytes = b
		bodyType = HTTP_REQ_BODYTYPE_BYTES
	case string:
		jsonBytes = []byte(b)
	case *os.File:
		bodyType = HTTP_REQ_BODYTYPE_FILE
	default:
		var err error
		jsonBytes, err = json.Marshal(body)
		if err != nil {
			Fatal(JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal exchange body for %s: %v", apiMsg, err))
		}
	}

	var requestBody io.Reader
	if bodyType == HTTP_REQ_BODYTYPE_FILE {
		file := body.(*os.File)
		requestBody = file
		if fileInfo, err := file.Stat(); err != nil {
			msgPrinter.Printf("Failed to get file info: %v", err)
			msgPrinter.Println()
		} else {
			bodyLen = int(fileInfo.Size())
		}
	} else {
		requestBody = bytes.NewBuffer(jsonBytes)
		bodyLen = len(jsonBytes)
	}

	return requestBody, bodyLen, bodyType
}

// invoke rest api call with retry
func InvokeRestApi(httpClient *http.Client, method string, urlPath string, credentials string, body interface{}, service string, apiMsg string) *http.Response {

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// encode the url so that it can accept unicode
	urlObj, errUrl := url.Parse(urlPath)
	if errUrl != nil {
		Fatal(CLI_INPUT_ERROR, msgPrinter.Sprintf("Malformed URL: %v. %v", urlPath, errUrl))
	}
	urlObj.RawQuery = urlObj.Query().Encode()

	if err := TrustIcpCert(httpClient); err != nil {
		Fatal(FILE_IO_ERROR, err.Error())
	}

	// get retry count and retry interval from env
	maxRetries, retryInterval, err := GetHttpRetryParameters(5, 2)
	if err != nil {
		Fatal(CLI_GENERAL_ERROR, err.Error())
	}

	retryCount := 0
	for {
		retryCount++

		// requestBody is nil if body is nil.
		requestBody, bodyLen, bodyType := createRequestBody(body, apiMsg)

		if requestBody != nil && bodyType == HTTP_REQ_BODYTYPE_FILE && bodyLen != 0 {
			// Calculate and show progress of file uploading
			totalSent := 0
			requestBody = &progressReader{requestBody, func(r int) {
				if r > 0 {
					totalSent += r
					msgPrinter.Printf("\rUploading: %.2f %s", float32(totalSent)/float32(bodyLen)*100, "%")
				} else {
					// Clear the progress info when the file has been fully uploaded
					msgPrinter.Printf("\r")
				}

			}}
		}
		// If we're retrying with an os.File body, then re-open it.
		if retryCount > 1 && body != nil {
			switch body.(type) {
			case *os.File:
				file := body.(*os.File)
				if rb, err := os.Open(file.Name()); err != nil {
					Fatal(CLI_INPUT_ERROR, msgPrinter.Sprintf("unable to open object file %v: %v", file.Name(), err))
				} else {
					requestBody = rb
				}
			}
		}

		// Create the request and run it
		req, err := http.NewRequest(method, urlObj.String(), requestBody)
		if err != nil {
			Fatal(HTTP_ERROR, msgPrinter.Sprintf("%s new request failed: %v", apiMsg, err))
		}

		req.Close = true
		req.Header.Add("Accept", "application/json")

		// for PUT/PATCH/POST
		if requestBody != nil {
			if bodyType == HTTP_REQ_BODYTYPE_BYTES {
				req.Header.Add("Content-Length", strconv.Itoa(bodyLen))
			} else if bodyType == HTTP_REQ_BODYTYPE_FILE {
				req.Header.Add("Content-Type", "application/octet-stream")
			} else {
				req.Header.Add("Content-Type", "application/json")
			}
		}

		// add the language request to the http header
		localeTag, err := i18n.GetLocale()
		if err != nil {
			localeTag = language.English
		}
		req.Header.Add("Accept-Language", localeTag.String())

		if credentials != "" {
			req.Header.Add("Authorization", fmt.Sprintf("Basic %v", base64.StdEncoding.EncodeToString([]byte(credentials))))
		} // else it is an anonymous call

		resp, err := httpClient.Do(req)
		if exchange.IsTransportError(resp, err) {
			http_status := ""
			if resp != nil {
				http_status = resp.Status
				if resp.Body != nil {
					resp.Body.Close()
				}
			}
			if retryCount <= maxRetries {
				Verbose(msgPrinter.Sprintf("Encountered HTTP error: %v calling %v REST API %v. HTTP status: %v. Will retry.", err, service, apiMsg, http_status))
				// retry for network tranport errors
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			} else {
				Fatal(HTTP_ERROR, msgPrinter.Sprintf("Encountered HTTP error: %v calling %v REST API %v. HTTP status: %v.", err, service, apiMsg, http_status))
			}
		} else if err != nil {
			printHorizonServiceRestError(service, apiMsg, err)
		} else {
			return resp
		}
	}
}

// ExchangeGet runs a GET to the specified service api and fills in the specified json structure. If the structure is just a string, fill in the raw json.
// If the list of goodHttpCodes is not empty and none match the actual http code, it will exit with an error. Otherwise the actual code is returned.
func ExchangeGet(service string, urlBase string, urlSuffix string, credentials string, goodHttpCodes []int, structure interface{}) (httpCode int) {
	url := urlBase + "/" + urlSuffix
	apiMsg := http.MethodGet + " " + url

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	Verbose(apiMsg)

	httpClient := GetHTTPClient(config.HTTPRequestTimeoutS)

	resp := InvokeRestApi(httpClient, http.MethodGet, url, credentials, nil, service, apiMsg)
	if resp.Body != nil {
		defer resp.Body.Close()
	}

	respBody := io.Reader(resp.Body)
	if resp.Header.Get("Content-type") == "application/octet-stream" {
		// Show progress of binary files downloading
		msgPrinter.Print("Downloading object")
		chunkNumber := 0
		respBody = &progressReader{resp.Body, func(r int) {
			chunkNumber++
			if r > 0 {
				// Show progress every 5 chunks to make the loading line shorter
				if chunkNumber%5 == 0 {
					fmt.Print(".")
				}
			} else {
				// Go to new line when the file has been fully downloaded
				msgPrinter.Println()
			}
		}}
	}

	bodyBytes, err := ioutil.ReadAll(respBody)
	if err != nil {
		Fatal(HTTP_ERROR, msgPrinter.Sprintf("failed to read body response from %s: %v", apiMsg, err))
	}
	httpCode = resp.StatusCode
	Verbose(msgPrinter.Sprintf("HTTP code: %d", httpCode))
	if !isGoodCode(httpCode, goodHttpCodes) {
		Fatal(HTTP_ERROR, msgPrinter.Sprintf("bad HTTP code %d from %s, output: %s", httpCode, apiMsg, string(bodyBytes)))
	}

	if len(bodyBytes) > 0 && structure != nil { // the DP front-end of exchange will return nothing when auth problem
		switch s := structure.(type) {
		case *[]byte:
			// This is the signal that they want the raw body back
			*s = bodyBytes
		case *string:
			// If the structure to fill in is just a string, unmarshal/remarshal it to get it in json indented form, and then return as a string
			//todo: this gets it in json indented form, but also returns the fields in random order (because they were interpreted as a map)
			var jsonStruct interface{}
			err = json.Unmarshal(bodyBytes, &jsonStruct)
			if err != nil {
				Fatal(JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal exchange body response from %s: %v", apiMsg, err))
			}
			jsonBytes, err := json.MarshalIndent(jsonStruct, "", JSON_INDENT)
			if err != nil {
				Fatal(JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal exchange output from %s: %v", apiMsg, err))
			}
			*s = string(jsonBytes)
		default:
			err = json.Unmarshal(bodyBytes, structure)
			if err != nil {
				Fatal(JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal exchange body response from %s: %v", apiMsg, err))
			}
		}
	}
	return
}

// This function will return response directly, the resp need to be closed by caller
func ExchangeGetResponse(service string, urlBase string, urlSuffix string, credentials string) *http.Response {
	url := urlBase + "/" + urlSuffix
	apiMsg := http.MethodGet + " " + url

	Verbose(apiMsg)

	httpClient := GetHTTPClient(config.HTTPRequestTimeoutS)

	resp := InvokeRestApi(httpClient, http.MethodGet, url, credentials, nil, service, apiMsg)
	return resp
}

// ExchangePutPost runs a PUT, POST or PATCH to the exchange api to create of update a resource. If body is a string, it will be given to the exchange
// as json. Otherwise the struct will be marshaled to json.
// If the list of goodHttpCodes is not empty and none match the actual http code, it will exit with an error. Otherwise the actual code is returned.
func ExchangePutPost(service string, method string, urlBase string, urlSuffix string, credentials string, goodHttpCodes []int, body interface{}, structure interface{}) (httpCode int) {
	url := urlBase + "/" + urlSuffix
	apiMsg := method + " " + url

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	Verbose(apiMsg)
	if IsDryRun() {
		return 201
	}

	httpClient := GetHTTPClient(config.HTTPRequestTimeoutS)
	resp := InvokeRestApi(httpClient, method, url, credentials, body, service, apiMsg)
	if resp.Body != nil {
		defer resp.Body.Close()
	}
	httpCode = resp.StatusCode
	Verbose(msgPrinter.Sprintf("HTTP code: %d", httpCode))
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if !isGoodCode(httpCode, goodHttpCodes) {
		if err != nil {
			Fatal(HTTP_ERROR, msgPrinter.Sprintf("failed to read exchange body response from %s: %v", apiMsg, err))
		}
		respMsg := exchange.PostDeviceResponse{}
		err = json.Unmarshal(bodyBytes, &respMsg)
		if err != nil {
			Fatal(HTTP_ERROR, msgPrinter.Sprintf("bad HTTP code %d from %s: %s", httpCode, apiMsg, string(bodyBytes)))
		}
		Fatal(HTTP_ERROR, msgPrinter.Sprintf("bad HTTP code %d from %s: %s, %s", httpCode, apiMsg, respMsg.Code, respMsg.Msg))
	} else if len(bodyBytes) > 0 && structure != nil { // the DP front-end of exchange will return nothing when auth problem
		switch s := structure.(type) {
		case *[]byte:
			// This is the signal that they want the raw body back
			*s = bodyBytes
		case *string:
			// If the structure to fill in is just a string, unmarshal/remarshal it to get it in json indented form, and then return as a string
			//todo: this gets it in json indented form, but also returns the fields in random order (because they were interpreted as a map)
			var jsonStruct interface{}
			err = json.Unmarshal(bodyBytes, &jsonStruct)
			if err != nil {
				Fatal(JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal exchange body response from %s: %v", apiMsg, err))
			}
			jsonBytes, err := json.MarshalIndent(jsonStruct, "", JSON_INDENT)
			if err != nil {
				Fatal(JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal exchange output from %s: %v", apiMsg, err))
			}
			*s = string(jsonBytes)
		default:
			err = json.Unmarshal(bodyBytes, structure)
			if err != nil {
				Fatal(JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal exchange body response from %s: %v", apiMsg, err))
			}
		}
	}

	return
}

// ExchangeDelete deletes a resource via the exchange api.
// If the list of goodHttpCodes is not empty and none match the actual http code, it will exit with an error. Otherwise the actual code is returned.
func ExchangeDelete(service string, urlBase string, urlSuffix string, credentials string, goodHttpCodes []int) (httpCode int) {
	url := urlBase + "/" + urlSuffix
	apiMsg := http.MethodDelete + " " + url

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	Verbose(apiMsg)
	if IsDryRun() {
		return 204
	}

	httpClient := GetHTTPClient(config.HTTPRequestTimeoutS)

	resp := InvokeRestApi(httpClient, http.MethodDelete, url, credentials, nil, service, apiMsg)
	if resp.Body != nil {
		defer resp.Body.Close()
	}

	// delete never returns a body
	httpCode = resp.StatusCode
	Verbose(msgPrinter.Sprintf("HTTP code: %d", httpCode))
	if !isGoodCode(httpCode, goodHttpCodes) {
		Fatal(HTTP_ERROR, msgPrinter.Sprintf("bad HTTP code %d from %s", httpCode, apiMsg))
	}
	return
}

func ConvertTime(unixSeconds uint64) string {
	if unixSeconds == 0 {
		return ""
	}
	return time.Unix(int64(unixSeconds), 0).String()
}

// find correct credentials to use. Use -u or -n if one of them is not empty.
// If both are empty, use HZN_EXCHANGE_USER_AUTH first, if it is not set use HZN_EXCHANGE_NODE_AUTH.
func GetExchangeAuth(userPw string, nodeIdTok string) string {
	credToUse := ""

	if userPw != "" {
		credToUse = userPw
	} else {
		if nodeIdTok != "" {
			credToUse = nodeIdTok
		} else {
			if tmpU := WithDefaultEnvVar(&userPw, "HZN_EXCHANGE_USER_AUTH"); *tmpU != "" {
				credToUse = *tmpU
			} else if tmpN := WithDefaultEnvVar(&nodeIdTok, "HZN_EXCHANGE_NODE_AUTH"); *tmpN != "" {
				credToUse = *tmpN
			}
		}
	}

	if credToUse == "" {
		Fatal(CLI_INPUT_ERROR, i18n.GetMessagePrinter().Sprintf("exchange authentication must be specified with one of the following: the -u flag, the -n flag, HZN_EXCHANGE_USER_AUTH or HZN_EXCHANGE_NODE_AUTH"))
	}

	return credToUse
}

// Find correct credentials to use. Use -u first.
// If empty, use HZN_EXCHANGE_USER_AUTH
// but do not show an error if credentials are empty
func GetExchangeAuthVersion(userPw string) string {
	credToUse := ""
	if userPw != "" {
		credToUse = userPw
	} else {
		if tmpU := WithDefaultEnvVar(&userPw, "HZN_EXCHANGE_USER_AUTH"); *tmpU != "" {
			credToUse = *tmpU
		}
	}
	return credToUse
}

// set env variable ARCH if it is not set
func SetDefaultArch() {
	arch := os.Getenv("ARCH")
	if arch == "" {
		if err := os.Setenv("ARCH", runtime.GOARCH); err != nil {
			Fatal(CLI_GENERAL_ERROR, err.Error())
		}
	}
}

func GetAndVerifyPublicKey(pubKeyFilePath string) string {
	msgPrinter := i18n.GetMessagePrinter()
	msgPrinter.Printf("Verifying public key file ... ")
	msgPrinter.Println()

	pubKeyFilePath_tmp := WithDefaultEnvVar(&pubKeyFilePath, "HZN_PUBLIC_KEY_FILE")
	pubKeyFilePath = VerifySigningKeyInput(*pubKeyFilePath_tmp, true)
	inBytes := ReadFile(pubKeyFilePath)
	if _, err := verify.ValidKeyOrCert(inBytes); err != nil {
		Fatal(CLI_INPUT_ERROR, msgPrinter.Sprintf("provided public key is not valid; error: %v", err))
	}
	return pubKeyFilePath
}

func getPrivateKeyFromFile(keyFile string) *rsa.PrivateKey {
	msgPrinter := i18n.GetMessagePrinter()
	msgPrinter.Printf("Checking private key file format ... ")
	msgPrinter.Println()

	var privKey *rsa.PrivateKey
	var err error
	if privKey, err = sign.ReadPrivateKey(keyFile); err != nil {
		Fatal(CLI_INPUT_ERROR, msgPrinter.Sprintf("provided private key %v is not valid; error: %v", keyFile, err))
	}

	return privKey
}

// get the default private or public key file name
func GetDefaultSigningKeyFile(isPublic bool) (string, error) {
	// we have to use $HOME for now because os/user is not implemented on some plateforms
	home_dir := os.Getenv("HOME")
	if home_dir == "" {
		home_dir = "/tmp/keys"
	}

	if isPublic {
		return filepath.Join(home_dir, DEFAULT_PUBLIC_KEY_FILE), nil
	} else {
		return filepath.Join(home_dir, DEFAULT_PRIVATE_KEY_FILE), nil
	}
}

func GetDeprecatedDefaultSigningKeyFile(isPublic bool) (string, error) {
	// we have to use $HOME for now because os/user is not implemented on some plateforms
	home_dir := os.Getenv("HOME")
	if home_dir == "" {
		home_dir = "/tmp/keys"
	}

	if isPublic {
		return filepath.Join(home_dir, ".hzn/keys/service.public.pem"), nil
	} else {
		return filepath.Join(home_dir, ".hzn/keys/service.private.key"), nil
	}
}

// Gets default keys if not set, verify key files exist.
func VerifySigningKeyInput(keyFile string, isPublic bool) string {
	keyFile = verifySigningKeyInputHelper(keyFile, isPublic, false)
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		keyFile = verifySigningKeyInputHelper(keyFile, isPublic, true)
		if _, err := os.Stat(keyFile); os.IsNotExist(err) {
			Fatal(CLI_GENERAL_ERROR, i18n.GetMessagePrinter().Sprintf("%v. Please create the signing key.", err))
		}
	}

	return keyFile
}

func verifySigningKeyInputHelper(keyFile string, isPublic, usingDeprecated bool) string {
	var err error
	// get default file names if input is empty
	if keyFile == "" {
		if usingDeprecated {
			if keyFile, err = GetDeprecatedDefaultSigningKeyFile(isPublic); err != nil {
				Fatal(CLI_GENERAL_ERROR, err.Error())
			}
		} else {
			if keyFile, err = GetDefaultSigningKeyFile(isPublic); err != nil {
				Fatal(CLI_GENERAL_ERROR, err.Error())
			}
		}
	}

	// convert to absolute path
	if keyFile, err = filepath.Abs(keyFile); err != nil {
		Fatal(CLI_GENERAL_ERROR, i18n.GetMessagePrinter().Sprintf("Failed to get absolute path for file %v. %v", keyFile, err))
	}

	return keyFile
}

// get default keys if needed and verify them.
// this function is used by `hzn exchange pattern/service publish
func GetSigningKeys(privKeyFilePath, pubKeyFilePath string) (*rsa.PrivateKey, []byte, string) {

	var err error

	// Get default private key if -k not specified
	var privKey *rsa.PrivateKey
	privKeyFilePath_tmp := WithDefaultEnvVar(&privKeyFilePath, "HZN_PRIVATE_KEY_FILE")
	privKeyFilePath = WithDefaultKeyFile(*privKeyFilePath_tmp, false)

	// if a valid private key was given or found at default location, load it
	if privKeyFilePath != "" {
		privKey = getPrivateKeyFromFile(privKeyFilePath)
	// otherwise, generate a random key
	} else if privKey, err = rsa.GenerateKey(rand.Reader, 2048); err != nil {
		Fatal(CLI_GENERAL_ERROR, i18n.GetMessagePrinter().Sprintf("private key could not be generated; error: %v", err))
	}

	// Load in public key, if given
	var pubKeyBytes []byte
	publicKeyName := "default.public.key"
	// get default public key
	if pubKeyFilePath != "" {
		publicKeyName = filepath.Base(pubKeyFilePath)
		pubKeyFilePath = GetAndVerifyPublicKey(pubKeyFilePath)
		pubKeyBytes = ReadFile(pubKeyFilePath)
	} else {
		// calculate public key from private key
		pubKeyBytes, err = x509.MarshalPKIXPublicKey(&privKey.PublicKey)
		if err != nil {
			Fatal(CLI_GENERAL_ERROR, i18n.GetMessagePrinter().Sprintf("%v. Public key could not be generated."))
		}
		// format public key
		pubEnc := &pem.Block{
			Type:    "PUBLIC KEY",
			Headers: nil,
			Bytes:   pubKeyBytes,
		}
		pubKeyBytes = pem.EncodeToMemory(pubEnc)
	}
	return privKey, pubKeyBytes, publicKeyName
}

// Run a command with optional stdin and args, and return stdout, stderr
func RunCmd(stdinBytes []byte, commandString string, args ...string) ([]byte, []byte) {

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// For debug, build the full cmd string
	cmdStr := commandString
	for _, a := range args {
		cmdStr += " " + a
	}
	if stdinBytes != nil {
		cmdStr += " < stdin"
	}
	Verbose(msgPrinter.Sprintf("running: %v", cmdStr))

	// Create the command object with its args
	cmd := exec.Command(commandString, args...)
	if cmd == nil {
		Fatal(EXEC_CMD_ERROR, msgPrinter.Sprintf("did not get a command object"))
	}

	var stdin io.WriteCloser
	//var jInbytes []byte
	var err error
	if stdinBytes != nil {
		// Create the std in pipe
		stdin, err = cmd.StdinPipe()
		if err != nil {
			Fatal(EXEC_CMD_ERROR, msgPrinter.Sprintf("Could not get Stdin pipe, error: %v", err))
		}
		// Read the input file
		//jInbytes, err = ioutil.ReadFile(stdinFilename)
		//if err != nil { Fatal(EXEC_CMD_ERROR,"Unable to read " + stdinFilename + " file, error: %v", err) }
	}
	// Create the stdout pipe to hold the output from the command
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		Fatal(EXEC_CMD_ERROR, msgPrinter.Sprintf("could not retrieve output from command, error: %v", err))
	}
	// Create the stderr pipe to hold the errors from the command
	stderr, err := cmd.StderrPipe()
	if err != nil {
		Fatal(EXEC_CMD_ERROR, msgPrinter.Sprintf("could not retrieve stderr from command, error: %v", err))
	}

	// Start the command, which will block for input from stdin if the cmd reads from it
	err = cmd.Start()
	if err != nil {
		Fatal(EXEC_CMD_ERROR, msgPrinter.Sprintf("Unable to start command, error: %v", err))
	}

	if stdinBytes != nil {
		// Send in the std in bytes
		_, err = stdin.Write(stdinBytes)
		if err != nil {
			Fatal(EXEC_CMD_ERROR, msgPrinter.Sprintf("Unable to write to stdin of command, error: %v", err))
		}
		// Close std in so that the command will begin to execute
		err = stdin.Close()
		if err != nil {
			Fatal(EXEC_CMD_ERROR, msgPrinter.Sprintf("Unable to close stdin, error: %v", err))
		}
	}

	err = error(nil)
	// Read the output from stdout and stderr into byte arrays
	// stdoutBytes, err := readPipe(stdout)
	stdoutBytes, err := ioutil.ReadAll(stdout)
	if err != nil {
		Fatal(EXEC_CMD_ERROR, msgPrinter.Sprintf("could not read stdout, error: %v", err))
	}
	// stderrBytes, err := readPipe(stderr)
	stderrBytes, err := ioutil.ReadAll(stderr)
	if err != nil {
		Fatal(EXEC_CMD_ERROR, msgPrinter.Sprintf("could not read stderr, error: %v", err))
	}

	// Now block waiting for the command to complete
	err = cmd.Wait()
	if err != nil {
		Fatal(EXEC_CMD_ERROR, msgPrinter.Sprintf("command failed: %v, stderr: %s", err, string(stderrBytes)))
	}

	return stdoutBytes, stderrBytes
}

// display a data structure as json format. Unescape the <, >, and & etc.
// (go usually escapes these chars.)
func DisplayAsJson(data interface{}) (string, error) {
	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", JSON_INDENT)
	err := enc.Encode(data)
	if err != nil {
		return "", err
	} else {
		return buf.String(), nil
	}
}

// Common function for getting an HTTP client connection object.
func GetHTTPClient(timeout int) *http.Client {

	// This env var should only be used in our test environments or in an emergency when there is a problem with the SSL certificate of a horizon service.
	skipSSL := false
	if os.Getenv("HZN_SSL_SKIP_VERIFY") != "" {
		skipSSL = true
	}

	// Set request timeout based on environment variables and input values. The environment variable always overrides the
	// input parameter. The other timeouts are subject to the timeout setting also.
	requestTimeout := timeout

	if envTimeout := os.Getenv(config.HTTPRequestTimeoutOverride); envTimeout != "" {
		if t, err := strconv.Atoi(envTimeout); err == nil {
			requestTimeout = t
		} else {
			Warning(i18n.GetMessagePrinter().Sprintf("Unable to use %v to set the request timeout, the value is not a valid number: %v", config.HTTPRequestTimeoutOverride, envTimeout))
		}
	}

	responseTimeout := int(float64(requestTimeout) * 0.8)
	dialTimeout := int(float64(requestTimeout) * 0.5)
	keepAlive := requestTimeout * 2
	TLSHandshake := dialTimeout
	expectContinue := int(float64(requestTimeout) * 0.5)

	Verbose(i18n.GetMessagePrinter().Sprintf("HTTP request timeout set to %v seconds", requestTimeout))

	return &http.Client{
		// remember that this timeout is for the whole request, including
		// body reading. This means that you must set the timeout according
		// to the total payload size you expect
		Timeout: time.Second * time.Duration(requestTimeout),
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   time.Duration(dialTimeout) * time.Second,
				KeepAlive: time.Duration(keepAlive) * time.Second,
			}).Dial,
			TLSHandshakeTimeout:   time.Duration(TLSHandshake) * time.Second,
			ResponseHeaderTimeout: time.Duration(responseTimeout) * time.Second,
			ExpectContinueTimeout: time.Duration(expectContinue) * time.Second,
			MaxIdleConns:          config.MaxHTTPIdleConnections,
			IdleConnTimeout:       config.HTTPIdleConnectionTimeoutS * time.Second,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: skipSSL,
			},
		},
	}

}

// create the exchange context with the given user credentail
func GetUserExchangeContext(userOrg string, credToUse string) exchange.ExchangeContext {
	var ec exchange.ExchangeContext
	if credToUse != "" {
		cred, token := SplitIdToken(credToUse)
		if userOrg != "" {
			cred = AddOrg(userOrg, cred)
		}
		ec = CreateUserExchangeContext(cred, token)
	} else {
		ec = CreateUserExchangeContext("", "")
	}

	return ec
}

// create an exchange context based on the user Id and password.
func CreateUserExchangeContext(userId string, passwd string) exchange.ExchangeContext {
	// GetExchangeUrl trims the last slash, we need to add it back for the exchange API calls.
	exchUrl := GetExchangeUrl() + "/"
	return exchange.NewCustomExchangeContext(userId, passwd, exchUrl, "", NewHTTPClientFactory())
}

// create an http client factory to be used for the exchange calls.
func NewHTTPClientFactory() *config.HTTPClientFactory {
	clientFunc := func(overrideTimeoutS *uint) *http.Client {
		var timeoutS uint
		if overrideTimeoutS != nil {
			timeoutS = *overrideTimeoutS
		} else {
			timeoutS = config.HTTPRequestTimeoutS
		}

		httpClient := GetHTTPClient(int(timeoutS))
		if err := TrustIcpCert(httpClient); err != nil {
			Fatal(FILE_IO_ERROR, err.Error())
		}

		return httpClient
	}

	// get retry count and retry interval from env
	maxRetries, retryInterval, err := GetHttpRetryParameters(5, 2)
	if err != nil {
		Fatal(CLI_GENERAL_ERROR, err.Error())
	}

	return &config.HTTPClientFactory{
		NewHTTPClient: clientFunc,
		RetryCount:    maxRetries,
		RetryInterval: retryInterval,
	}
}

// get the http retry count and interval from the env variables.
func GetHttpRetryParameters(default_count int, default_interval int) (int, int, error) {
	maxRetries := default_count
	retryInterval := default_interval

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// get retry count and retry interval from env
	maxRetries_s := os.Getenv("HZN_HTTP_RETRIES")
	if maxRetries_s != "" {
		var err1 error
		if maxRetries, err1 = strconv.Atoi(maxRetries_s); err1 != nil {
			return 0, 0, fmt.Errorf(msgPrinter.Sprintf("Error converting environmental variable HZN_HTTP_RETRIES %v to integer. %v", maxRetries_s, err1))
		}
	}
	retryInterval_s := os.Getenv("HZN_HTTP_RETRY_INTERVAL")
	if retryInterval_s != "" {
		var err1 error
		if retryInterval, err1 = strconv.Atoi(retryInterval_s); err1 != nil {
			return 0, 0, fmt.Errorf(msgPrinter.Sprintf("Error converting environmental variable HZN_HTTP_RETRY_INTERVAL %v to integer. %v", retryInterval_s, err1))
		}
	}

	return maxRetries, retryInterval, nil
}

// download http response body to outputFilePath if specified, otherwise, downloads to defaultFileName. Returns file path used.
func DownloadToFile(outputFilePath, defaultFileName string, body []byte, extension string, permissions os.FileMode, overwrite bool) string {
	msgPrinter := i18n.GetMessagePrinter()
	var fileName string
	// if no fileName and filePath specified, data will be saved in current dir, with name {defaultFileName}
	if outputFilePath == "" {
		fileName = defaultFileName
	} else {
		// trim the ending "/" if there are more than 1 "/"
		for strings.HasSuffix(outputFilePath, "//") {
			outputFilePath = strings.TrimSuffix(outputFilePath, "/")
		}

		fi, _ := os.Stat(outputFilePath)
		if fi == nil {
			// outputFilePath is not an existing dir, then consider it as fileName, need to remove "/" in the end
			if strings.HasSuffix(outputFilePath, "/") {
				outputFilePath = strings.TrimSuffix(outputFilePath, "/")
			}
			fileName = outputFilePath
		} else {
			if fi.IsDir() {
				if !strings.HasSuffix(outputFilePath, "/") {
					outputFilePath = outputFilePath + "/"
				}
				fileName = fmt.Sprintf("%s%s", outputFilePath, defaultFileName)
			} else {
				fileName = outputFilePath
			}
		}
	}
	if fileName[len(fileName)-len(extension):] != extension {
		fileName += extension
	}

	if !overwrite {
		if _, err := os.Stat(fileName); !os.IsNotExist(err) {
			Fatal(CLI_INPUT_ERROR, msgPrinter.Sprintf("File %s already exists. Please specify a different file path or file name. To overwrite the existing file, use the '--overwrite' flag.", fileName))
		}
	}

	// add newline to end of file
	bodyString := string(body)
	if bodyString[len(bodyString)-1] != '\n' {
		bodyString += "\n"
	}
	body = []byte(bodyString)

	if err := ioutil.WriteFile(fileName, body, permissions); err != nil {
		Fatal(INTERNAL_ERROR, msgPrinter.Sprintf("Failed to save data for object '%s' to file %s, err: %v", defaultFileName, fileName, err))
	}

	return fileName
}

/* Will probably need this....
func getString(v interface{}) string {
	if reflect.ValueOf(v).IsNil() { return "" }
	return fmt.Sprintf("%v", reflect.Indirect(reflect.ValueOf(v)))
}
*/

// This function is used in the service publish command to pull the docker image.
// It  the image name with the digest.
func GetNewDockerImageName(image string, dontTouchImage bool, pullImage bool) string {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	domain, path, tag, digest := cutil.ParseDockerImagePath(image)
	Verbose(msgPrinter.Sprintf("%s parsed into: domain=%s, path=%s, tag=%s", image, domain, path, tag))
	if path == "" {
		msgPrinter.Printf("Warning: could not parse image path '%v'. Not pushing it to a docker registry, just including it in the 'deployment' field as-is.", image)
		msgPrinter.Println()
	} else if digest == "" {
		// This image has a tag, or default tag.
		// We are going to push images to the docker repo only if the user wants us to update the digest of the image.
		if !dontTouchImage {
			// Push it, get the repo digest, and modify the imagePath to use the digest.
			client := NewDockerClient()
			digest := ""
			var err error
			if pullImage {
				if digest, err = PullDockerImage(client, domain, path, tag); err != nil {
					Fatal(CLI_GENERAL_ERROR, msgPrinter.Sprintf("Docker pull failure: %v", err))
				}
			} else {
				digest = PushDockerImage(client, domain, path, tag) // this will error out if the push fails or can't get the digest
			}
			if domain != "" {
				domain = domain + "/"
			}
			newImage := domain + path + "@" + digest
			return newImage
		}
	}
	return image
}

func LoggingDriverSupportsTagging(driverName string) bool {
	for i := range dockerDriversWithTagSupport {
		if dockerDriversWithTagSupport[i] == driverName {
			return true
		}
	}
	return false
}

func ChekServiceLogPossibility(logDriver string) (nonDefaultDriverUsed bool, err error) {
	if logDriver == "" || logDriver == "syslog" {
		return false, nil
	}

	for i, _ := range dockerDriversWithLoggingSupport {
		if logDriver == dockerDriversWithLoggingSupport[i] {
			return true, nil
		}
	}

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	return true, fmt.Errorf(msgPrinter.Sprintf("Provided log-driver (%s) does not support logs viewing. "+
		"Logs are only available on the following drivers: %v", logDriver, dockerDriversWithLoggingSupport))
}

// progressReader is an io.Reader wrapper with progress reporting function
type progressReader struct {
	io.Reader
	Reporter func(r int)
}

func (pr *progressReader) Read(p []byte) (n int, err error) {
	n, err = pr.Reader.Read(p)
	pr.Reporter(n)
	return n, err
}

// Returns HZN_DEVICE_ID or HZN_NODE_ID env variables depending on which is defined
func GetDeviceId() string {
	deviceId := ""
	if nodeId := os.Getenv("HZN_NODE_ID"); nodeId != "" {
		deviceId = nodeId
	} else if deviceIdEnv := os.Getenv("HZN_DEVICE_ID"); deviceIdEnv != "" {
		deviceId = deviceIdEnv
	}
	return deviceId
}
