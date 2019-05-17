package externalpolicy

import (
	"errors"
	"fmt"
)

// These are built-in property names that can be used in the policies.
// anax will fill in the properties during the agreement negotiation process.
// The user defined policies (business policy, node policy) need to add constrains on these properties if needed.
const (
	PROP_NODE_CPU    = "openhorizon.cpu"             //The number of CPUs
	PROP_NODE_MEMORY = "openhorizon.memory"          //The amount of memory in MBs
	PROP_NODE_ARCH   = "openhorizon.arch"            //The hardware architecture of the node (e.g. amd64, armv6, etc)
	PROP_SVC_URL     = "openhorizon.service.url"     // The unique name of the service.
	PROP_SVC_NAME    = "openhorizon.service.name"    // The unique name of the service.
	PROP_SVC_ORG     = "openhorizon.service.org"     // The multi-tenant org where the service is defined. If service.url is specified but this property is omitted, the org defaults to the org of the node, service or policy which is referring to the service.
	PROP_SVC_VERSION = "openhorizon.service.version" // The version of a service using the same semantic version syntax.
	PROP_SVC_ARCH    = "openhorizon.service.arch"    // The hardware architecture of the node this service can run on.
)

type ExternalPolicy struct {
	// The properties this node wishes to expose about itself. These properties can be referred to by constraint expressions in other policies,
	// (e.g. service policy, model policy, business policy).
	Properties PropertyList `json:"properties,omitempty"`

	// A textual expression indicating requirements on the other party in order to make an agreement.
	Constraints ConstraintExpression `json:"constraints,omitempty"`
}

func (e ExternalPolicy) String() string {
	return fmt.Sprintf("ExternalPolicy: Properties: %v, Constraints: %v", e.Properties, e.Constraints)
}

// The validate function returns errors if the policy does not validate. It uses the constraint language
// plugins to handle the constraints field.
func (e *ExternalPolicy) Validate() error {

	// Validate the PropertyList.
	if e != nil && len(e.Properties) != 0 {
		if err := e.Properties.Validate(); err != nil {
			return errors.New(fmt.Sprintf("properties contains an invalid property: %v", err))
		}
	}

	// Validate the Constraints expression by invoking the plugins.
	if e != nil && len(e.Constraints) != 0 {
		return e.Constraints.Validate()
	}

	// We only get here if the input object is nil OR all of the top level fields are empty.
	return nil
}
