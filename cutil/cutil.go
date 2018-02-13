package cutil

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"net"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func FirstN(n int, ss []string) []string {
	out := make([]string, 0)

	for ix := 0; ix < n-1; ix++ {
		if len(ss) == ix {
			break
		}

		out = append(out, ss[ix])
	}

	return out
}

func SecureRandomString() (string, error) {
	bytes := make([]byte, 64)

	if _, err := rand.Read(bytes); err != nil {
		return "", err
	} else {
		return base64.URLEncoding.EncodeToString(bytes), nil
	}
}

func GenerateAgreementId() (string, error) {

	bytes := make([]byte, 32, 32)
	agreementIdString := ""
	_, err := rand.Read(bytes)
	if err == nil {
		agreementIdString = hex.EncodeToString(bytes)
	}
	return agreementIdString, err
}

func ArchString() string {
	return runtime.GOARCH
}

// Check if the device has internect connection to the given host or not.
func CheckConnectivity(host string) error {
	var err error
	for i := 0; i < 3; i++ {
		_, err = net.LookupHost(host)
		if err == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return err
}

// Exchange time format. Golang requires the format string to be in reference to the specific time as shown.
// This is so that the formatter and parser can figure out what goes where in the string.
const ExchangeTimeFormat = "2006-01-02T15:04:05.999Z[MST]"

func TimeInSeconds(timestamp string) int64 {
	if t, err := time.Parse(ExchangeTimeFormat, timestamp); err != nil {
		glog.Errorf(fmt.Sprintf("error converting time %v into seconds, error: %v", timestamp, err))
		return 0
	} else {
		return t.Unix()
	}
}

func FormattedTime() string {
	return time.Now().Format(ExchangeTimeFormat)
}

func Min(first int, second int) int {
	if first < second {
		return first
	}
	return second
}

func Minuint64(first uint64, second uint64) uint64 {
	if first < second {
		return first
	}
	return second
}

func Maxuint64(first uint64, second uint64) uint64 {
	if first > second {
		return first
	}
	return second
}

// Convert a native typed user input variable to a string so that the value can be passed as an
// environment variable to a container. This function modifies the input env var map and it will
// modify map keys that already exist in the map.
func NativeToEnvVariableMap(envMap map[string]string, varName string, varValue interface{}) error {
	switch varValue.(type) {
	case bool:
		envMap[varName] = strconv.FormatBool(varValue.(bool))
	case string:
		envMap[varName] = varValue.(string)
	// floats and ints come here when the json parser is not using the UseNumber() parsing flag
	case float64:
		if float64(int64(varValue.(float64))) == varValue.(float64) {
			envMap[varName] = strconv.FormatInt(int64(varValue.(float64)), 10)
		} else {
			envMap[varName] = strconv.FormatFloat(varValue.(float64), 'f', 6, 64)
		}
	// floats and ints come here when the json parser is using the UseNumber() parsing flag
	case json.Number:
		envMap[varName] = varValue.(json.Number).String()
	case []interface{}:
		los := ""
		for _, e := range varValue.([]interface{}) {
			if _, ok := e.(string); ok {
				los = los + e.(string) + " "
			}
		}
		los = los[:len(los)-1]
		envMap[varName] = los
	default:
		return errors.New(fmt.Sprintf("unknown variable type %T for variable %v", varValue, varName))
	}
	return nil
}

// This function checks the input variable value against the expected exchange variable type and returns an error if
// there is no match. This function assumes the varValue was parsed with json decoder set to UseNumber().
func VerifyWorkloadVarTypes(varValue interface{}, expectedType string) error {
	switch varValue.(type) {
	case bool:
		if expectedType != "bool" && expectedType != "boolean" {
			return errors.New(fmt.Sprintf("type %T, expecting %v", varValue, expectedType))
		}
	case string:
		if expectedType != "string" {
			return errors.New(fmt.Sprintf("type %T, expecting %v", varValue, expectedType))
		}
	case json.Number:
		strNum := varValue.(json.Number).String()
		if expectedType != "int" && expectedType != "float" {
			return errors.New(fmt.Sprintf("type json.Number, expecting %v", expectedType))
		} else if strings.Contains(strNum, ".") && expectedType == "int" {
			return errors.New(fmt.Sprintf("type float, expecting int"))
		}
	case []interface{}:
		if expectedType != "list of strings" {
			return errors.New(fmt.Sprintf("type %T, expecting %v", varValue, expectedType))
		} else {
			for _, e := range varValue.([]interface{}) {
				if _, ok := e.(string); !ok {
					return errors.New(fmt.Sprintf("type %T, expecting []string", varValue))
				}
			}
		}
	default:
		return errors.New(fmt.Sprintf("type %T, is an unexpected type.", varValue))
	}
	return nil
}

// This function may seem simple but since it is shared with the hzn dev CLI, an update to it will cause a compile error in the CLI
// code. This will prevent us from adding a new platform env var but forgetting to update the CLI.
func SetPlatformEnvvars(envAdds map[string]string, prefix string, agreementId string, deviceId string, org string, workloadPW string, exchangeURL string) {

	// The agreement id that is controlling the lifecycle of this container.
	if agreementId != "" {
		envAdds[prefix+"AGREEMENTID"] = agreementId
	}

	// The exchange id of the node that is running the container.
	envAdds[prefix+"DEVICE_ID"] = deviceId

	// The exchange organization that the node belongs.
	envAdds[prefix+"ORGANIZATION"] = org

	// Deprecated workload password, used only by legacy POC workloads.
	if workloadPW != "" {
		envAdds[prefix+"HASH"] = workloadPW
	}

	// Add in the exchange URL so that the workload knows which ecosystem its part of
	envAdds[prefix+"EXCHANGE_URL"] = exchangeURL
}

// This function is similar to the above, for env vars that are system related.
func SetSystemEnvvars(envAdds map[string]string, prefix string, lat string, lon string, cpus string, ram string, arch string) {

	// The latitude and longitude of the node are provided.
	envAdds[prefix+"LAT"] = lat
	envAdds[prefix+"LON"] = lon

	// The number of CPUs and amount of RAM to allocate.
	envAdds[prefix+"CPUS"] = cpus
	envAdds[prefix+"RAM"] = ram

	// Set the hardware architecture
	if arch == "" {
		envAdds[prefix+"ARCH"] = runtime.GOARCH
	} else {
		envAdds[prefix+"ARCH"] = arch
	}

}

func MakeMSInstanceKey(specRef string, v string, id string) string {
	s := specRef
	if strings.Contains(specRef, "://") {
		s = strings.Split(specRef, "://")[1]
	}
	new_s := strings.Replace(s, "/", "-", -1)

	return fmt.Sprintf("%v_%v_%v", new_s, v, id)
}
