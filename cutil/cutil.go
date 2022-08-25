package cutil

import (
	"bufio"
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	mrand "math/rand"
	"net"
	"os"
	"path"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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

	random := mrand.New(mrand.NewSource(int64(time.Now().Nanosecond())))

	randStr := ""
	randStr += string(rune(random.Intn(10) + 48)) // add a random digit to the string
	randStr += string(rune(random.Intn(26) + 65)) // add an uppercase letter to the string
	randStr += string(rune(random.Intn(26) + 97)) // add a lowercase letter to the string
	randStr += string(rune(random.Intn(10) + 48)) // add one more random digit so we reach 64 bytes at the end

	// pad out the password to make it <=15 chars
	bytes := make([]byte, 63)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	randStr += base64.URLEncoding.EncodeToString(bytes)

	// shuffle the string
	shuffledStr := []rune(randStr)
	mrand.Shuffle(len(shuffledStr), func(i, j int) {
		shuffledStr[i], shuffledStr[j] = shuffledStr[j], shuffledStr[i]
	})

	return string(shuffledStr), nil
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

func GenerateRandomNodeId() (string, error) {

	bytes := make([]byte, 20, 20)
	nodeIdString := ""
	_, err := rand.Read(bytes)
	if err == nil {
		nodeIdString = hex.EncodeToString(bytes)
	}
	return nodeIdString, err
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

func FormattedUTCTime() string {
	return time.Now().UTC().Format(ExchangeTimeFormat)
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
		if len(los) > 1 {
			los = los[:len(los)-1]
		}
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
		// if the type is empty, it defaults to string
		if expectedType != "string" && expectedType != "" {
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
func SetPlatformEnvvars(envAdds map[string]string, prefix string, agreementId string, deviceId string, org string, exchangeURL string, pattern string, fssProtocol string, fssAddress string, fssPort string) {

	// The agreement id that is controlling the lifecycle of this container.
	if agreementId != "" {
		envAdds[prefix+"AGREEMENTID"] = agreementId
	}

	// The exchange id of the node that is running the container (deprecated).
	envAdds[prefix+"DEVICE_ID"] = deviceId

	// The exchange id of the node that is running the container.
	envAdds[prefix+"NODE_ID"] = deviceId

	// The exchange organization that the node belongs.
	envAdds[prefix+"ORGANIZATION"] = org

	// The pattern that the node is hosting.
	envAdds[prefix+"PATTERN"] = pattern

	// Add in the exchange URL so that the workload knows which ecosystem its part of
	envAdds[prefix+"EXCHANGE_URL"] = exchangeURL

	// Add in the File Sync Service related env vars. Note the env var names contain ESS instead of FSS. This is intentional for now
	// because the sync service code is going to remain independent and potentially reusable/stand alone. A service implementation
	// is expected to read these env vars so that it can form the correct URL to invoke the FSS (ESS) API.
	// The API_PROTOCOL determines how to form the ESS API address.
	envAdds[prefix+"ESS_API_PROTOCOL"] = fssProtocol

	// The address of the file sync service API.
	namespace := os.Getenv("AGENT_NAMESPACE")
	if namespace != "" {
		// ESS in cluster
		fssAddress = fmt.Sprintf("agent-service.%v.svc.cluster.local", namespace)
	}
	envAdds[prefix+"ESS_API_ADDRESS"] = fssAddress

	// The port of the file sync service API. Zero when using a unix domain socket.
	if strings.Contains(fssPort, "\"") {
		fssPort = strings.ReplaceAll(fssPort, "\"", "")

	}
	envAdds[prefix+"ESS_API_PORT"] = fssPort

	if namespace != "" {
		// The secret name of the FSS credentials that the operator should use.
		envAdds[prefix+"ESS_AUTH"] = config.HZN_FSS_AUTH_PATH + "-" + agreementId

		// The secret name of FSS API SSL Certificate that the operator should use.
		envAdds[prefix+"ESS_CERT"] = config.HZN_FSS_CERT_PATH

	} else {
		// The name of the mounted file containing the FSS credentials that the container should use.
		envAdds[prefix+"ESS_AUTH"] = path.Join(config.HZN_FSS_AUTH_MOUNT, config.HZN_FSS_AUTH_FILE)

		// The name of the mounted file containing the FSS API SSL Certificate that the container should use.
		envAdds[prefix+"ESS_CERT"] = path.Join(config.HZN_FSS_CERT_MOUNT, config.HZN_FSS_CERT_FILE)
	}
}

// Temporary function to remove ESS env vars for the edge cluster case.
// HZN_ESS_CERT=/ess-cert/cert.pem
// HZN_ESS_AUTH=/ess-auth/auth.json
// HZN_ESS_API_PROTOCOL=secure
// HZN_ESS_API_ADDRESS=/var/tmp/horizon/horizon7/fss-domain-socket/essapi.sock -->
// HZN_ESS_API_PORT=0
// func RemoveESSEnvVars(envAdds map[string]string, prefix string) map[string]string {
// 	delete(envAdds, prefix+"ESS_API_PROTOCOL")
// 	delete(envAdds, prefix+"ESS_API_ADDRESS")
// 	delete(envAdds, prefix+"ESS_API_PORT")
// 	delete(envAdds, prefix+"ESS_AUTH")
// 	delete(envAdds, prefix+"ESS_CERT")
// 	return envAdds
// }

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

// This is also used as the container name.
// The container name must start with an alphanumeric character and can then use _ . or - in addition to alphanumeric.
// [a-zA-Z0-9][a-zA-Z0-9_.-]+
func MakeMSInstanceKey(specRef string, org string, v string, id string) string {

	new_s := NormalizeURL(specRef)

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

func GetMapKeys(aMap interface{}) []string {

	keySlice := make([]string, 0)

	theMap := reflect.ValueOf(aMap)
	if theMap.IsNil() {
		return keySlice
	}

	keys := theMap.MapKeys()
	for _, k := range keys {
		keySlice = append(keySlice, k.String())
	}

	return keySlice
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

// merge 2 slices, removing duplicates
func MergeSlices(a []string, b []string) []string {
	ret := make([]string, len(a))
	copy(ret, a)
	for _, bEle := range b {
		if !SliceContains(a, bEle) {
			ret = append(ret, bEle)
		}
	}
	return ret
}

// it returns the org/url form for an api spec
func FormOrgSpecUrl(url string, org string) string {
	if org == "" {
		return url
	} else {
		return fmt.Sprintf("%v/%v", org, url)
	}
}

// it returns the org_url form for an api spec
func NormalizeOrgSpecUrl(url string, org string) string {
	if org == "" {
		return url
	} else {
		return fmt.Sprintf("%v_%v", org, url)
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

// Get the machine serial number. If cpuInfoFile is an empty string,
// this function will use /proc/cpuinfo for Linux. For mac OS, nothing will
// be returned.
func GetMachineSerial(cpuInfoFile string) (string, error) {
	if cpuInfoFile == "" {
		// does not support
		if runtime.GOOS == "darwin" {
			return "", fmt.Errorf("Does not support mac os for getting machine serial number.")
		} else {
			cpuInfoFile = "/proc/cpuinfo"
		}
	}

	// Linux case
	if _, err := os.Stat(cpuInfoFile); err != nil {
		return "", err
	}

	fh, err := os.Open(cpuInfoFile)
	if err != nil {
		return "", err
	}
	defer fh.Close()

	scanner := bufio.NewScanner(fh)
	r, _ := regexp.Compile("Serial([ \t]*):")
	for scanner.Scan() {
		line := string(scanner.Bytes())
		if r.MatchString(line) {
			serialParts := strings.Split(line, " ")
			if len(serialParts) > 1 {
				return serialParts[1], nil
			}
		}
	}
	if err := scanner.Err(); err != nil {
		glog.Errorf(fmt.Sprintf("Error scanning %v, error: %v", cpuInfoFile, err))
		return "", nil
	}
	return "", nil

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

// FormExchangeId combines url, version, arch the same way the exchange does to form the resource ID.
func FormExchangeIdForService(url, version, arch string) string {
	// Remove the https:// from the beginning of workloadUrl and replace troublesome chars with a dash.
	//val workloadUrl2 = """^[A-Za-z0-9+.-]*?://""".r replaceFirstIn (url, "")
	//val workloadUrl3 = """[$!*,;/?@&~=%]""".r replaceAllIn (workloadUrl2, "-")     // I think possible chars in valid urls are: $_.+!*,;/?:@&~=%-
	//return OrgAndId(orgid, workloadUrl3 + "_" + version + "_" + arch).toString
	url1 := FormExchangeIdWithSpecRef(url)
	return url1 + "_" + version + "_" + arch
}

// Remove the https:// from the beginning of workloadUrl and replace troublesome chars with a dash.
func FormExchangeIdWithSpecRef(specRef string) string {
	re := regexp.MustCompile(`^[A-Za-z0-9+.-]*?://`)
	specRef2 := re.ReplaceAllLiteralString(specRef, "")
	return FormExchangeId(specRef2)
}

// Replace unwanted charactore with - in the id
func FormExchangeId(id string) string {
	re := regexp.MustCompile(`[$!*,;/?@&~=%]`)
	return re.ReplaceAllLiteralString(id, "-")
}

func RemoveArchFromServiceId(sId string) string {
	sId_no_arch := sId
	tempA := strings.Split(sId, "_")
	if len(tempA) >= 3 {
		sId_no_arch = strings.Join(tempA[:len(tempA)-1], "_")
	}

	return sId_no_arch
}

func NewKubeConfig() (*rest.Config, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("Failed to get cluster config information: %v", err)
	}
	return config, nil
}

func NewKubeClient() (*kubernetes.Clientset, error) {
	config, err := NewKubeConfig()
	if err != nil {
		return nil, err
	}
	clientset, _ := kubernetes.NewForConfig(config)
	return clientset, nil
}

// GetClusterCountInfo returns the cluster's available memory, total memory, cpu count, arch, kube version, or an error if it cannot get the client
func GetClusterCountInfo() (float64, float64, float64, string, string, error) {
	client, err := NewKubeClient()
	if err != nil {
		return 0, 0, 1, "", "", fmt.Errorf("Failed to get kube client for introspecting cluster properties. Proceding with default values. %v", err)
	}
	versionObj, err := client.Discovery().ServerVersion()
	if err != nil {
		glog.Warningf("Failed to get kubernetes server version: %v", err)
	}
	version := ""
	if versionObj != nil {
		version = versionObj.GitVersion
	}
	availMem := float64(0)
	totalMem := float64(0)
	cpu := float64(0)
	arch := ""
	nodes, err := client.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return 0, 0, 0, "", "", nil
	}

	for _, node := range nodes.Items {
		if arch == "" {
			arch = node.Status.NodeInfo.Architecture
		}
		availMem += FloatFromQuantity(node.Status.Allocatable.Memory()) / 1000000
		totalMem += FloatFromQuantity(node.Status.Capacity.Memory()) / 1000000
		cpu += FloatFromQuantity(node.Status.Capacity.Cpu())
	}

	return math.Round(availMem), math.Round(totalMem), cpu, arch, version, nil
}

// FloatFromQuantity returns a float64 with the value of the given quantity type
func FloatFromQuantity(quantVal *resource.Quantity) float64 {
	if intVal, ok := quantVal.AsInt64(); ok {
		return float64(intVal)
	}
	decVal := quantVal.AsDec()
	unscaledVal := decVal.UnscaledBig().Int64()
	scale := decVal.Scale()
	floatVal := float64(unscaledVal) * math.Pow10(-1*int(scale))
	return floatVal
}

// GetHashFromString returns the md5 hash for given string
func GetHashFromString(str string) string {
	hasher := md5.New()
	hasher.Write([]byte(str))
	return hex.EncodeToString(hasher.Sum(nil))
}

// Get Docker endpoint to use. Default to docker.sock and only check podman if that isn't there
// If neither are there, default to docker.sock and there will be a failure later on
func GetDockerEndpoint() string {
	dockerSocket := "/var/run/docker.sock"
	podmanSocket := "/var/run/podman/podman.sock"
	listenerSocket := dockerSocket
	if _, err := os.Stat(dockerSocket); os.IsNotExist(err) {
		if _, err := os.Stat(podmanSocket); err == nil {
			listenerSocket = podmanSocket
		}
	}
	dockerEP := "unix://" + listenerSocket
	return dockerEP
}
