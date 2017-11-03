package cliutils

import (
	"strings"
	"fmt"
	"os"
	"net/http"
	"io/ioutil"
	"encoding/json"
	"time"
)

const HZN_API = "http://localhost"

// Holds the cmd line flags that were set so other pkgs can access
type Options struct {
	Verbose *bool
	ArchivedAgreements *bool
}
var Opts Options

func Verbose(msg string, args ...interface{}) {
	if !*Opts.Verbose { return }
	if !strings.HasSuffix(msg, "\n") { msg += "\n"}
	fmt.Printf("[verbose] "+msg, args...)
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

// HorizonGet runs a GET on the anax api and fills in the specified json structure
func HorizonGet(urlSuffix string, structure interface{}) {
	url := HZN_API + "/" + urlSuffix
	apiMsg := "GET " + url
	Verbose(apiMsg)
	resp, err := http.Get(url)
	if err != nil { Fatal(3, "%s failed: %v", apiMsg, err) }
	defer resp.Body.Close()
	Verbose("HTTP code: %d", resp.StatusCode)
	//todo: check for good http code
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil { Fatal(3, "failed to read body response for %s: %v", apiMsg, err) }
	err = json.Unmarshal(bodyBytes, structure)
	if err != nil { Fatal(3, "failed to unmarshal body response for %s: %v", apiMsg, err) }
}


func ConvertTime(unixSecond uint64) string {
	 return time.Unix(int64(unixSecond), 0).String()
}
