package externalpolicy

import (
	"github.com/golang/glog"
	"github.com/open-horizon/anax/cutil"
	"runtime"
)

// These are built-in property names that can be used in the policies.
// anax will fill in the properties during the agreement negotiation process.
// The user defined policies (business policy, node policy) need to add constraints on these properties if needed.
const (
	// for node policy
	PROP_NODE_CPU        = "openhorizon.cpu"             // The number of CPUs
	PROP_NODE_MEMORY     = "openhorizon.memory"          // The amount of memory in MBs
	PROP_NODE_ARCH       = "openhorizon.arch"            // The hardware architecture of the node (e.g. amd64, armv6, etc)
	PROP_NODE_HARDWAREID = "openhorizon.hardwareId"      // The device serial number if it can be found. A generated Id otherwise.
	PROP_NODE_PRIVILEGED = "openhorizon.allowPrivileged" // Property set to determine if privileged services may be run on this device. Can be set by user, default is false.

	// for service policy
	PROP_SVC_URL        = "openhorizon.service.url"     // The unique name of the service.
	PROP_SVC_NAME       = "openhorizon.service.name"    // The unique name of the service.
	PROP_SVC_ORG        = "openhorizon.service.org"     // The multi-tenant org where the service is defined. If service.url is specified but this property is omitted, the org defaults to the org of the node, service or policy which is referring to the service.
	PROP_SVC_VERSION    = "openhorizon.service.version" // The version of a service using the same semantic version syntax.
	PROP_SVC_ARCH       = "openhorizon.service.arch"    // The hardware architecture of the node this service can run on.
	PROP_SVC_PRIVILEGED = "openhorizon.allowPrivileged" // Does the service use workloads that require privileged mode or net==host to run. Can be set by user. Is an error to set to false if service introspection indicates true.
)

const MAX_MEMEORY = 1048576 // the unit is MB. This is 1000G

// CreateNodeBuiltInPolicy returns 2 externalpolicies.
// The first contains read-only built-in properties. The second has read/write properties.
// get the node's built-in ptoperties to be used in the node policy
// availableMem -- the total memory vs. the available memory size
// ominGenHwId -- true to omit the hardware id property if it cannot be found and is not in the existing policy
// existingPolicy -- the current node policy or nil
func CreateNodeBuiltInPolicy(availableMem bool, omitGenHwId bool, existingPolicy *ExternalPolicy) (*ExternalPolicy, *ExternalPolicy) {
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
		glog.Infof("Found existing policy with %v", privProp)
		privileged = privProp.Value.(bool)
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
				glog.V(2).Infof("Device serial number not found. Omitting hardwareId property.")
			}
		}
	}

	if hwId != "" {
		nodeBuiltInReadOnlyProps.Add_Property(Property_Factory(PROP_NODE_HARDWAREID, hwId), false)
	}
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
