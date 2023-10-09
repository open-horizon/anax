package externalpolicy

import (
	"fmt"
	"github.com/go-ini/ini"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/semanticversion"
	"os"
	"runtime"
)

// These are built-in property names that can be used in the policies.
// anax will fill in the properties during the agreement negotiation process.
// The user defined policies (business policy, node policy) need to add constraints on these properties if needed.
const (
	// for node policy
	PROP_NODE_CPU                  = "openhorizon.cpu"                       // The number of CPUs
	PROP_NODE_MEMORY               = "openhorizon.memory"                    // The amount of memory in MBs
	PROP_NODE_ARCH                 = "openhorizon.arch"                      // The hardware architecture of the node (e.g. amd64, armv6, etc)
	PROP_NODE_HARDWAREID           = "openhorizon.hardwareId"                // The device serial number if it can be found. A generated Id otherwise.
	PROP_NODE_PRIVILEGED           = "openhorizon.allowPrivileged"           // Property set to determine if privileged services may be run on this device. Can be set by user, default is false.
	PROP_NODE_K8S_VERSION          = "openhorizon.kubernetesVersion"         // Server version of the cluster the agent is running in
	PROP_NODE_K8S_NAMESPACE        = "openhorizon.kubernetesNamespace"       // The namespace for cluster agent
	PROP_NODE_K8S_NAMESPACE_SCOPED = "openhorizon.kubernetesNamespaceScoped" // Boolean field indicating whter the cluster agent is namespace-scoped
	PROP_NODE_OS                   = "openhorizon.operatingSystem"           // The operating system the agent is installed on. For containerized agents, this is the host os
	PROP_NODE_CONTAINERIZED        = "openhorizon.containerized"             // Boolean field indicating whether the agent is running in a container

	// for install type
	OS_CLUSTER   = "cluster"
	OS_CONTAINER = "anax-in-container"
	OS_MAC       = "macos"
	OS_UBUNTU    = "ubuntu"
	OS_DEBIAN    = "debian"
	OS_RASPBIAN  = "raspbian"
	OS_RHEL      = "rhel"
	OS_CENTOS    = "centos"
	OS_FEDORA    = "fedora"
	OS_SUSE      = "suse"

	// for service policy
	PROP_SVC_URL        = "openhorizon.service.url"     // The unique name of the service.
	PROP_SVC_NAME       = "openhorizon.service.name"    // The unique name of the service.
	PROP_SVC_ORG        = "openhorizon.service.org"     // The multi-tenant org where the service is defined. If service.url is specified but this property is omitted, the org defaults to the org of the node, service or policy which is referring to the service.
	PROP_SVC_VERSION    = "openhorizon.service.version" // The version of a service using the same semantic version syntax.
	PROP_SVC_ARCH       = "openhorizon.service.arch"    // The hardware architecture of the node this service can run on.
	PROP_SVC_PRIVILEGED = "openhorizon.allowPrivileged" // Does the service use workloads that require privileged mode or net==host to run. Can be set by user. Is an error to set to false if service introspection indicates true.
)

const MAX_MEMEORY = 1048576                            // the unit is MB. This is 1000G
const DEFAULT_NODE_K8S_NAMESPACE = "openhorizon-agent" // the default cluster name space for cluster type. The default for device type is an emptry string.

func ListReadOnlyProperties() []string {
	return []string{PROP_NODE_CPU, PROP_NODE_ARCH, PROP_NODE_MEMORY, PROP_NODE_HARDWAREID, PROP_NODE_K8S_VERSION, PROP_NODE_K8S_NAMESPACE, PROP_NODE_K8S_NAMESPACE_SCOPED, PROP_NODE_OS, PROP_NODE_CONTAINERIZED}
}

// returns a map of all the built-in properties used by the given node type
// the property name is the key and the value is the first anax version where that property was included
func NodeBuiltInPropMap(nodeType string) map[string]string {
	if nodeType == "device" {
		return map[string]string{PROP_NODE_CPU: "2.23.4", PROP_NODE_MEMORY: "2.23.4", PROP_NODE_ARCH: "2.23.4", PROP_NODE_HARDWAREID: "2.24.5", PROP_NODE_PRIVILEGED: "2.24.10",
			PROP_NODE_OS: "2.30.0", PROP_NODE_CONTAINERIZED: "2.30.0"}
	} else if nodeType == "cluster" {
		return map[string]string{PROP_NODE_K8S_NAMESPACE_SCOPED: "2.31.0", PROP_NODE_K8S_NAMESPACE: "2.31.0", PROP_NODE_K8S_VERSION: "2.26.4", PROP_NODE_CPU: "2.23.4", PROP_NODE_MEMORY: "2.23.4", PROP_NODE_ARCH: "2.23.4", PROP_NODE_PRIVILEGED: "2.24.10"}
	}

	return map[string]string{}
}

func ListSupportedOperatingSystems() []string {
	return []string{OS_UBUNTU, OS_DEBIAN, OS_RASPBIAN, OS_RHEL, OS_MAC, OS_CENTOS, OS_FEDORA, OS_SUSE}
}

func ContainsAllBuiltInNodeProps(propList *PropertyList, agentVersion string, nodeType string) bool {
	if propList == nil {
		return false
	}

	builtInPropList := NodeBuiltInPropMap(nodeType)
	for prop, vers := range builtInPropList {
		if comp, err := semanticversion.CompareVersions(agentVersion, vers); err != nil {
			glog.Errorf("Failed to compare agent version: %v", err)
			return false
		} else if comp >= 0 && !propList.HasProperty(prop) {
			return false
		}
	}
	return true
}

// CreateNodeBuiltInPolicy returns 2 externalpolicies.
// The first contains read-only built-in properties. The second has read/write properties.
// get the node's built-in ptoperties to be used in the node policy
// availableMem -- the total memory vs. the available memory size
// ominGenHwId -- true to omit the hardware id property if it cannot be found and is not in the existing policy
// existingPolicy -- the current node policy or nil
func CreateNodeBuiltInPolicy(availableMem bool, omitGenHwId bool, existingPolicy *ExternalPolicy, cluster bool) (*ExternalPolicy, *ExternalPolicy) {
	if cluster {
		clusterPolicy := createClusterNodeBuiltInPolicy(availableMem)
		return clusterPolicy, clusterPolicy
	}
	return createDeviceNodeBuiltInPolicy(availableMem, omitGenHwId, existingPolicy)
}

func createClusterNodeBuiltInPolicy(availableMem bool) *ExternalPolicy {
	builtInPol := new(PropertyList)
	availMem, totMem, cpu, arch, vers, ns, isNamespaceScoped, err := cutil.GetClusterCountInfo()
	if err != nil {
		glog.V(2).Infof("Error getting cluster built-in properties: %v", err)
	}

	builtInPol.Add_Property(Property_Factory(PROP_NODE_ARCH, arch), false)
	builtInPol.Add_Property(Property_Factory(PROP_NODE_CPU, cpu), false)
	builtInPol.Add_Property(Property_Factory(PROP_NODE_PRIVILEGED, true), false)
	if vers != "" {
		builtInPol.Add_Property(Property_Factory(PROP_NODE_K8S_VERSION, vers), false)
	}
	builtInPol.Add_Property(Property_Factory(PROP_NODE_K8S_NAMESPACE, ns), false)
	builtInPol.Add_Property(Property_Factory(PROP_NODE_K8S_NAMESPACE_SCOPED, isNamespaceScoped), false)
	if availableMem {
		builtInPol.Add_Property(Property_Factory(PROP_NODE_MEMORY, availMem), false)
	} else {
		builtInPol.Add_Property(Property_Factory(PROP_NODE_MEMORY, totMem), false)
	}
	return &ExternalPolicy{Properties: *builtInPol}
}

func createDeviceNodeBuiltInPolicy(availableMem bool, omitGenHwId bool, existingPolicy *ExternalPolicy) (*ExternalPolicy, *ExternalPolicy) {
	nodeBuiltInReadOnlyProps := new(PropertyList)
	nodeBuiltInReadWriteProps := new(PropertyList)

	cpu, err := cutil.GetCPUCount("")
	if err != nil {
		glog.V(2).Infof("Failed to get cpu count for the local node. Proceeding with default value. %v", err)
		cpu = 1
	}

	total_mem, avail_mem, err := cutil.GetMemInfo("")
	if err != nil {
		glog.V(2).Infof("Failed to get memory info for the local node. Proceeding with default value. %v", err)
		total_mem = 0
		avail_mem = 0
	}

	privileged := false
	if existingPolicy != nil && existingPolicy.Properties.HasProperty(PROP_NODE_PRIVILEGED) {
		privProp, _ := existingPolicy.Properties.GetProperty(PROP_NODE_PRIVILEGED)
		glog.V(5).Infof("Found existing policy with %v", privProp)
		var ok bool
		privileged, ok = privProp.Value.(bool)
		if !ok {
			if privStr, ok := privProp.Value.(string); ok && (privStr == "true" || privStr == "false") {
				if privStr == "true" {
					privileged = true
				} else if privStr == "false" {
					privileged = false
				}
			} else {
				glog.V(1).Infof("Value of property %s must be a boolean (true or false).", PROP_NODE_PRIVILEGED)
			}
		}
	}
	hwId := ""
	if existingPolicy != nil && existingPolicy.Properties.HasProperty(PROP_NODE_HARDWAREID) {
		hwProp, err := existingPolicy.Properties.GetProperty(PROP_NODE_HARDWAREID)
		if err == nil {
			hwId = hwProp.Value.(string)
		}
	}
	if hwId == "" {
		hwId, err = cutil.GetMachineSerial("")
		if hwId == "" && !omitGenHwId {
			if err != nil {
				glog.V(2).Infof("Failed to read device serial number: %v. Proceeding with generated Id.", err)
			} else {
				glog.V(2).Infof("Device serial number not found. Proceeding with generated Id.")
			}

			var err error
			if hwId, err = cutil.GenerateRandomNodeId(); err != nil {
				glog.V(1).Infof("Failed to generate device Id: %v", err)
			}
		} else if hwId == "" {
			if err != nil {
				glog.V(2).Infof("Failed to read device serial number: %v. Omitting hardwareId property.", err)
			} else {
				glog.V(2).Infof("Device serial number not found.")
			}
		}
	}

	if hwId != "" || !omitGenHwId {
		nodeBuiltInReadOnlyProps.Add_Property(Property_Factory(PROP_NODE_HARDWAREID, hwId), false)
	}

	edgeOS, containerized, err := ProfileEdgeOS()
	if err != nil {
		glog.V(2).Infof("Failed to find install type for built-in properties: %v", err)
		nodeBuiltInReadOnlyProps.Add_Property(Property_Factory(PROP_NODE_OS, ""), false)
	} else {
		nodeBuiltInReadOnlyProps.Add_Property(Property_Factory(PROP_NODE_OS, edgeOS), false)
	}
	nodeBuiltInReadOnlyProps.Add_Property(Property_Factory(PROP_NODE_CONTAINERIZED, containerized), false)
	nodeBuiltInReadOnlyProps.Add_Property(Property_Factory(PROP_NODE_CPU, float64(cpu)), false)
	nodeBuiltInReadOnlyProps.Add_Property(Property_Factory(PROP_NODE_ARCH, runtime.GOARCH), false)

	nodeBuiltInReadWriteProps.Add_Property(Property_Factory(PROP_NODE_PRIVILEGED, privileged), false)

	if availableMem {
		nodeBuiltInReadOnlyProps.Add_Property(Property_Factory(PROP_NODE_MEMORY, float64(avail_mem)), false)
	} else {
		nodeBuiltInReadOnlyProps.Add_Property(Property_Factory(PROP_NODE_MEMORY, float64(total_mem)), false)
	}

	buitInPolReadOnly := ExternalPolicy{
		Properties:  *nodeBuiltInReadOnlyProps,
		Constraints: []string{},
	}
	buitInPolReadWrite := ExternalPolicy{
		Properties:  *nodeBuiltInReadWriteProps,
		Constraints: []string{},
	}

	return &buitInPolReadOnly, &buitInPolReadWrite
}

// create the built-in properties
func CreateServiceBuiltInPolicy(svcName, svcOrg, svcVersion, svcArch string) *ExternalPolicy {
	svcBuiltInProps := new(PropertyList)

	svcBuiltInProps.Add_Property(Property_Factory(PROP_SVC_URL, svcName), false)
	svcBuiltInProps.Add_Property(Property_Factory(PROP_SVC_NAME, svcName), false)
	svcBuiltInProps.Add_Property(Property_Factory(PROP_SVC_ORG, svcOrg), false)
	svcBuiltInProps.Add_Property(Property_Factory(PROP_SVC_VERSION, svcVersion), false)
	svcBuiltInProps.Add_Property(Property_Factory(PROP_SVC_ARCH, svcArch), false)

	builtInPol := ExternalPolicy{
		Properties:  *svcBuiltInProps,
		Constraints: []string{},
	}

	return &builtInPol
}

// check if the given property name is a node built-in property name
func IsNodeBuiltinPropertyName(propName string) bool {
	if propName == PROP_NODE_CPU ||
		propName == PROP_NODE_MEMORY ||
		propName == PROP_NODE_ARCH ||
		propName == PROP_NODE_HARDWAREID ||
		propName == PROP_NODE_PRIVILEGED ||
		propName == PROP_NODE_K8S_VERSION ||
		propName == PROP_NODE_K8S_NAMESPACE ||
		propName == PROP_NODE_K8S_NAMESPACE_SCOPED ||
		propName == PROP_NODE_OS ||
		propName == PROP_NODE_CONTAINERIZED {
		return true
	} else {
		return false
	}
}

// check if the given property name is a service built-in property name
func IsServiceBuiltinPropertyName(propName string) bool {
	if propName == PROP_SVC_URL ||
		propName == PROP_SVC_NAME ||
		propName == PROP_SVC_ORG ||
		propName == PROP_SVC_VERSION ||
		propName == PROP_SVC_ARCH ||
		propName == PROP_SVC_PRIVILEGED {
		return true
	} else {
		return false
	}
}

// This function will find what os the agent is running on or if it is running in a container
// options are: Mac, Ubuntu, Debian, RHEL, anax-in-container
func ProfileEdgeOS() (string, bool, error) {
	// check if we are in a docker container. if so check the release file set up by horizon-container to find out the host os
	if os.Getenv("DOCKER_NAME") != "" {
		if sysInfo, err := ini.Load("/etc/host-os-release"); err != nil {
			// the host os is unknown
			return "", true, nil
		} else {
			os := sysInfo.Section("").Key("OS").String()
			if cutil.SliceContains(ListSupportedOperatingSystems(), os) {
				return os, true, nil
			}
		}
	} else if sysInfo, err := ini.Load("/etc/os-release"); err != nil {
		// running natively on some kind of linux. check the relase file to find out what kind
		return "", false, fmt.Errorf("Failed to find installation type: %v", err)
	} else {
		os := sysInfo.Section("").Key("ID").String()
		if cutil.SliceContains(ListSupportedOperatingSystems(), os) {
			return os, false, nil
		}
	}

	return "", false, fmt.Errorf("Failed to find installation type. Did not match any supported profile.")
}
