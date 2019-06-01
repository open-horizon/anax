package externalpolicy

import (
	"runtime"
	"syscall"
)

// These are built-in property names that can be used in the policies.
// anax will fill in the properties during the agreement negotiation process.
// The user defined policies (business policy, node policy) need to add constrains on these properties if needed.
const (
	// for node policy
	PROP_NODE_CPU    = "openhorizon.cpu"    //The number of CPUs
	PROP_NODE_MEMORY = "openhorizon.memory" //The amount of memory in MBs
	PROP_NODE_ARCH   = "openhorizon.arch"   //The hardware architecture of the node (e.g. amd64, armv6, etc)

	// for service policy
	PROP_SVC_URL     = "openhorizon.service.url"     // The unique name of the service.
	PROP_SVC_NAME    = "openhorizon.service.name"    // The unique name of the service.
	PROP_SVC_ORG     = "openhorizon.service.org"     // The multi-tenant org where the service is defined. If service.url is specified but this property is omitted, the org defaults to the org of the node, service or policy which is referring to the service.
	PROP_SVC_VERSION = "openhorizon.service.version" // The version of a service using the same semantic version syntax.
	PROP_SVC_ARCH    = "openhorizon.service.arch"    // The hardware architecture of the node this service can run on.
)

const MAX_MEMEORY = 1048576 // the unit is MB. This is 1000G

// get the node's built-in ptoperties to be used in the node policy
// availableMem -- the total memory vs. the available memory size
func CreateNodeBuiltInPolicy(availableMem bool) *ExternalPolicy {
	nodeBuiltInProps := new(PropertyList)

	nodeBuiltInProps.Add_Property(Property_Factory(PROP_NODE_CPU, float64(runtime.NumCPU())), false)
	nodeBuiltInProps.Add_Property(Property_Factory(PROP_NODE_ARCH, runtime.GOARCH), false)

	var info syscall.Sysinfo_t
	err := syscall.Sysinfo(&info)
	if err != nil || info.Totalram == 0 {
		nodeBuiltInProps.Add_Property(Property_Factory(PROP_NODE_MEMORY, float64(MAX_MEMEORY)), false)
	} else {
		if availableMem {
			// TODO-new get available memory
			nodeBuiltInProps.Add_Property(Property_Factory(PROP_NODE_MEMORY, bytesToMegaBytes(info.Totalram)), false)
		} else {
			nodeBuiltInProps.Add_Property(Property_Factory(PROP_NODE_MEMORY, bytesToMegaBytes(info.Totalram)), false)
		}
	}

	buitInPol := ExternalPolicy{
		Properties:  *nodeBuiltInProps,
		Constraints: []string{},
	}

	return &buitInPol
}

// create the built-in properties
func CreateServiceBuiltInPolicy(svcName, svcOrg, svcVersion, svcArch string) *ExternalPolicy {
	svcBuiltInProps := new(PropertyList)

	svcBuiltInProps.Add_Property(Property_Factory(PROP_SVC_URL, svcName), false)
	svcBuiltInProps.Add_Property(Property_Factory(PROP_SVC_NAME, svcName), false)
	svcBuiltInProps.Add_Property(Property_Factory(PROP_SVC_ORG, svcOrg), false)
	svcBuiltInProps.Add_Property(Property_Factory(PROP_SVC_VERSION, svcVersion), false)
	svcBuiltInProps.Add_Property(Property_Factory(PROP_SVC_ARCH, svcArch), false)

	buitInPol := ExternalPolicy{
		Properties:  *svcBuiltInProps,
		Constraints: []string{},
	}

	return &buitInPol
}

// converts bytes to megabytes
func bytesToMegaBytes(c uint64) float64 {
	return float64(c >> 20)
}
