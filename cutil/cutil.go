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
	"regexp"
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

func TimeInSeconds(timestamp string, format string) int64 {
	if t, err := time.Parse(format, timestamp); err != nil {
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
func SetPlatformEnvvars(envAdds map[string]string, prefix string, agreementId string, deviceId string, org string, workloadPW string, exchangeURL string, pattern string) {

	// The agreement id that is controlling the lifecycle of this container.
	if agreementId != "" {
		envAdds[prefix+"AGREEMENTID"] = agreementId
	}

	// The exchange id of the node that is running the container.
	envAdds[prefix+"DEVICE_ID"] = deviceId

	// The exchange organization that the node belongs.
	envAdds[prefix+"ORGANIZATION"] = org

	// The pattern that the node is hosting.
	envAdds[prefix+"PATTERN"] = pattern

	// Deprecated workload password, used only by legacy POC workloads.
	if workloadPW != "" {
		envAdds[prefix+"HASH"] = workloadPW
	}

	// Add in the exchange URL so that the workload knows which ecosystem its part of
	envAdds[prefix+"EXCHANGE_URL"] = exchangeURL
}

// This function is similar to the above, for env vars that are system related. It is only used by workloads.
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

	// Set the Host IPv4 addresses, omit interfaces that are down.
	if ips, err := GetAllHostIPv4Addresses([]NetFilter{OmitDown}); err != nil {
		glog.Errorf("Error obtaining host IP addresses: %v", err)
	} else {
		envAdds[prefix+"HOST_IPS"] = strings.Join(ips, ",")
	}

}

func MakeMSInstanceKey(specRef string, org string, v string, id string) string {
	s := specRef
	if strings.Contains(specRef, "://") {
		s = strings.Split(specRef, "://")[1]
	}
	new_s := strings.Replace(s, "/", "-", -1)

	if org == "" {
		return fmt.Sprintf("%v_%v_%v", new_s, v, id)
	} else {
		return fmt.Sprintf("%v_%v_%v_%v", org, new_s, v, id)
	}
}

// This function parsed the given image name to disfferent parts. The image name has the following format:
// [[repo][:port]/][somedir/]image[:tag][@digest]
// If the image path as an improper form (we could not parse it), path will be empty.
func ParseDockerImagePath(imagePath string) (domain, path, tag, digest string) {
	// image names can be domain.com/dir/dir:tag  or  domain.com/dir/dir@sha256:ac88f4...  or  domain.com/dir/dir:tag@sha256:ac88f4...
	reDigest := regexp.MustCompile(`^(\S*)@(\S+)$`)
	reTag := regexp.MustCompile(`^([^/ ]*)(\S*):([^:/ ]+)$`)
	reNoTag := regexp.MustCompile(`^([^/ ]*)(\S*)$`)

	var imagePath2 string

	// take out the digest
	if digestMatches := reDigest.FindStringSubmatch(imagePath); len(digestMatches) == 3 {
		digest = digestMatches[2]
		imagePath2 = digestMatches[1]
	} else {
		imagePath2 = imagePath
	}

	if imagePath2 == "" {
		return // path being blank is the indication that it did not match our parsing
	}

	// match the rest
	var matches []string
	if matches = reTag.FindStringSubmatch(imagePath2); len(matches) == 4 {
		path = matches[2]
		tag = matches[3]
	} else if matches = reNoTag.FindStringSubmatch(imagePath2); len(matches) == 3 {
		path = matches[2]
	} else {
		return // path being blank is the indication that it did not match our parsing
	}

	domain = matches[1]
	// An image in docker hub has no domain, the chars before the 1st / are part of the path
	if !strings.ContainsAny(domain, ".:") {
		path = domain + path
		domain = ""
	} else {
		path = strings.TrimPrefix(path, "/")
	}
	return
}

func CopyMap(m1 map[string]interface{}, m2 map[string]interface{}) {
	for k, v := range m1 {
		m2[k] = v
	}
}

// It will return the first n characters of the string and the rest will be as "..."
func TruncateDisplayString(s string, n int) string {
	if len(s) <= n {
		return s
	} else {
		return s[:n] + "..."
	}
}

func IsIPv4(address string) bool {
	if net.ParseIP(address) == nil {
		return false
	}
	return strings.Count(address, ":") < 2
}

type NetFilter func(net.Interface) bool

func OmitLoopback(i net.Interface) bool {
	if (i.Flags & net.FlagLoopback) != 0 {
		return false
	}
	return true
}

func OmitUp(i net.Interface) bool {
	if (i.Flags & net.FlagUp) != 0 {
		return false
	}
	return true
}

func OmitDown(i net.Interface) bool {
	if (i.Flags & net.FlagUp) == 0 {
		return false
	}
	return true
}

// Interface filter functions return false if the interface should be filtered out.
func GetAllHostIPv4Addresses(interfaceFilters []NetFilter) ([]string, error) {

	ips := make([]string, 0, 5)

	interfaces, err := net.Interfaces()
	if err != nil {
		return ips, errors.New(fmt.Sprintf("could not get network interfaces, error: %v", err))
	}

	// Run through all the host's network interaces, filtering out interfaces as per in the input filters,
	// and then return the remaining IPv4 addresses.
	for _, i := range interfaces {

		// Filter out interfaces that we don't care about.
		keep := true
		for _, f := range interfaceFilters {
			if !f(i) {
				keep = false
				break
			}
		}

		if !keep {
			continue
		}

		// The interface filter didnt remove the interface, so grab it's IP address and make sure it's a v4 address.
		addrs, err := i.Addrs()
		if err != nil {
			glog.Warningf("Could not get IP address(es) for network interface %v, error: %v", i.Name, err)
		} else {
			for _, addr := range addrs {

				// Grab the IP address.
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				default:
					return ips, errors.New(fmt.Sprintf("interface %v has address object of unexpected type %T.", i.Name, addr))
				}

				// If it's a v4 address keep it.
				if IsIPv4(ip.String()) {
					ips = append(ips, ip.String())
				}

			}
		}
	}

	return ips, nil
}

// check if a slice contains a string
func SliceContains(a []string, s string) bool {
	for _, v := range a {
		if s == v {
			return true
		}
	}
	return false
}

// it returns the org/url form for an api spec
func FormOrgSpecUrl(url string, org string) string {
	if org == "" {
		return url
	} else {
		return fmt.Sprintf("%v/%v", org, url)
	}
}

// The input is org/url, output is (org, url).
// assume no `/` in the org
func SplitOrgSpecUrl(org_url string) (string, string) {
	if org_url == "" {
		return "", ""
	} else {
		s := strings.SplitN(org_url, "/", 2)
		if len(s) == 1 {
			return "", s[0]
		} else {
			return s[0], s[1]
		}
	}
}
