package cutil

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"net"
	"os"
	"path"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	// Please do not change
	// they are used as error strings in api/path_service_config.go
	// they also used as templates for parsing the error strings in cli/register.go
	ANAX_SVC_MISSING_VARIABLE = "variable %v for service %v is missing from mappings."
	ANAX_SVC_MISSING_CONFIG   = "service config for version %v of %v is missing."
	ANAX_SVC_WRONG_TYPE       = "variable %v for service %v is "
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
			return errors.New(fmt.Sprintf("type %T, expecting %v.", varValue, expectedType))
		}
	case string:
		if expectedType != "string" {
			return errors.New(fmt.Sprintf("type %T, expecting %v.", varValue, expectedType))
		}
	case json.Number:
		if expectedType != "int" && !strings.Contains(expectedType, "float") {
			return errors.New(fmt.Sprintf("type json.Number, expecting %v.", expectedType))
		}
		numVal, err := varValue.(json.Number).Float64()
		if err != nil {
			return errors.New(fmt.Sprintf("type json.Number could not be parsed to float"))
		}
		if float64(int(numVal)) != numVal && expectedType == "int" {
			return errors.New(fmt.Sprintf("type float, expecting int."))
		}
	case float64, float32:
		if expectedType != "int" && !strings.Contains(expectedType, "float") {
			return errors.New(fmt.Sprintf("type number, expecting %v.", expectedType))
		}
		numVal := varValue.(float64)
		if float64(int(numVal)) != numVal && expectedType == "int" {
			return errors.New(fmt.Sprintf("type float64, expecting int."))
		}
	case []interface{}:
		if expectedType != "list of strings" {
			return errors.New(fmt.Sprintf("type %T, expecting %v.", varValue, expectedType))
		} else {
			for _, e := range varValue.([]interface{}) {
				if _, ok := e.(string); !ok {
					return errors.New(fmt.Sprintf("type %T, expecting []string.", varValue))
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
func SetPlatformEnvvars(envAdds map[string]string, prefix string, agreementId string, deviceId string, org string, workloadPW string, exchangeURL string, pattern string, fssProtocol string, fssAddress string, fssPort string) {

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

	// Add in the File Sync Service related env vars. Note the env var names contain ESS instead of FSS. This is intentional for now
	// because the sync service code is going to remain independent and potentially reusable/stand alone. A service implementation
	// is expected to read these env vars so that it can form the correct URL to invoke the FSS (ESS) API.
	// The API_PROTOCOL determines how to form the ESS API address.
	envAdds[prefix+"ESS_API_PROTOCOL"] = fssProtocol

	// The address of the file sync service API.
	envAdds[prefix+"ESS_API_ADDRESS"] = fssAddress

	// The port of the file sync service API. Zero when using a unix domain socket.
	envAdds[prefix+"ESS_API_PORT"] = fssPort

	// The name of the mounted file containing the FSS credentials that the container should use.
	envAdds[prefix+"ESS_AUTH"] = path.Join(config.HZN_FSS_AUTH_MOUNT, config.HZN_FSS_AUTH_FILE)

	// The name of the mounted file containing the FSS API SSL Certificate that the container should use.
	envAdds[prefix+"ESS_CERT"] = path.Join(config.HZN_FSS_CERT_MOUNT, config.HZN_FSS_CERT_FILE)

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

// This is also used as the container name
// The container name must start with an alphanumeric character and can then use _ . or - in addition to alphanumeric.
// [a-zA-Z0-9][a-zA-Z0-9_.-]+
func MakeMSInstanceKey(specRef string, org string, v string, id string) string {
	s := specRef
	// only take the part after "*://" for specRef
	if strings.Contains(specRef, "://") {
		s = strings.Split(specRef, "://")[1]
	}
	new_s := strings.Replace(s, "/", "-", -1)

	// form the key
	instKey1 := ""
	if org == "" {
		instKey1 = fmt.Sprintf("%v_%v_%v", new_s, v, id)
	} else {
		instKey1 = fmt.Sprintf("%v_%v_%v_%v", org, new_s, v, id)
	}

	// replace any characters not in [a-zA-Z0-9_.-] with "-"
	re1 := regexp.MustCompile(`[^a-zA-Z0-9_.-]`)
	instKey2 := re1.ReplaceAllLiteralString(instKey1, "-")

	// now make sure the first charactor is alphanumeric, replace it with "0" if not.
	re2 := regexp.MustCompile(`^[^a-zA-Z0-9]`)
	instKey3 := re2.ReplaceAllLiteralString(instKey2, "0")

	return instKey3
}

func NormalizeURL(specRef string) string {
	s := specRef
	if strings.Contains(specRef, "://") {
		s = strings.Split(specRef, "://")[1]
	}
	return strings.Replace(s, "/", "-", -1)
}

// This function parsed the given image name to disfferent parts. The image name has the following format:
// [[repo][:port]/][somedir/]image[:tag][@digest]
// If the image path as an improper form (we could not parse it), path will be empty.
// image names can be domain.com/dir/dir:tag  or  domain.com/dir/dir@sha256:ac88f4...  or  domain.com/dir/dir:tag@sha256:ac88f4...
func ParseDockerImagePath(imagePath string) (domain, path, tag, digest string) {
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

// for the image from the items parsed from ParseDockerImagePath function
func FormDockerImageName(domain, path, tag, digest string) string {
	image := ""
	if domain != "" {
		image = domain + "/" + path
	} else {
		image = path
	}

	if tag != "" {
		image = image + ":" + tag
	}

	if digest != "" {
		image = image + "@" + digest
	}
	return image
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

// Get the number of cpus on local node. If cpuinfo_file is an empty string,
// this function will get /proc/cpuinfo for Linux.
func GetCPUCount(cpuinfo_file string) (int, error) {
	if cpuinfo_file == "" {
		// does not support
		if runtime.GOOS == "darwin" {
			return 0, fmt.Errorf("Does not support mac os for getting cpu count.")
		} else {
			cpuinfo_file = "/proc/cpuinfo"
		}
	}

	// Linux case
	if _, err := os.Stat(cpuinfo_file); err != nil {
		return 0, err
	}

	if fh, err := os.Open(cpuinfo_file); err != nil {
		return 0, err
	} else {
		defer fh.Close()

		cpu_count := 0
		scanner := bufio.NewScanner(fh)
		r, _ := regexp.Compile("processor([ \t]*):")
		for scanner.Scan() {
			if r.MatchString(string(scanner.Bytes())) {
				cpu_count++
			}
		}
		if err := scanner.Err(); err != nil {
			return 0, nil
		}
		return cpu_count, nil
	}
}

// Get the total memory size and available memory size in MegaBytes
// If meminfo_file is an empty string, this function will get /proc/meminfo for Linux.
func GetMemInfo(meminfo_file string) (uint64, uint64, error) {
	if meminfo_file == "" {
		// does not support
		if runtime.GOOS == "darwin" {
			return 0, 0, fmt.Errorf("Does not support mac os for getting memory info.")
		} else {
			meminfo_file = "/proc/meminfo"
		}
	}

	// Linux case
	if _, err := os.Stat(meminfo_file); err != nil {
		return 0, 0, err
	}

	if fh, err := os.Open(meminfo_file); err != nil {
		return 0, 0, err
	} else {
		defer fh.Close()

		total_mem := uint64(0)
		avail_mem := uint64(0)
		r_total, _ := regexp.Compile(`MemTotal[ \t]*:[ \t]*([\d]+)[ \t]*(.*)$`)
		r_avail, _ := regexp.Compile(`MemAvailable[ \t]*:[ \t]*([\d]+)[ \t]*(.*)$`)

		scanner := bufio.NewScanner(fh)

		for scanner.Scan() {
			match := r_total.FindAllStringSubmatch(string(scanner.Bytes()), 1)
			if match != nil && len(match) > 0 && len(match[0]) > 2 {
				total_mem, _ = ConvertToMB(match[0][1], match[0][2])
				if avail_mem != 0 {
					break
				}
			}
			match = r_avail.FindAllStringSubmatch(string(scanner.Bytes()), 1)
			if match != nil && len(match) > 0 && len(match[0]) > 2 {
				avail_mem, _ = ConvertToMB(match[0][1], match[0][2])
				if total_mem != 0 {
					break
				}
			}
		}
		if err := scanner.Err(); err != nil {
			return 0, 0, nil
		}
		return total_mem, avail_mem, nil
	}
}

// Converts the given number (in string) to mega bytes. The unit can be MB, KB, GB, or B.
func ConvertToMB(value string, unit string) (uint64, error) {
	if s, err := strconv.ParseUint(value, 10, 64); err != nil {
		return 0, fmt.Errorf("Failed to convert the string to uint64. %v", err)
	} else {
		switch strings.ToUpper(unit) {
		case "B":
			return s >> 20, nil
		case "KB":
			return s >> 10, nil
		case "MB":
			return s, nil
		case "GB":
			return s << 10, nil
		default:
			return s, nil
		}
	}
}
