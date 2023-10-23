package exchangecommon

import (
	"fmt"
	"github.com/open-horizon/anax/externalpolicy"
)

const NODEPOLICY_VERSION_VERSION_2 = "v2"

// NodePolicy the node policy
// The properties and constraints defined in the top-level are common
// properties and constraints that are used by both Deployment and Management.
// If the same property name is defined in the second level (Deployment or Management),
// the perperty value of the second level takes the precedence.
// If there are constraints defined in the second level, all the constraints defined in
// the top level will be ignored.
// swagger:model
type NodePolicy struct {
	Label                         string                        `json:"label,omitempty"`
	Description                   string                        `json:"description,omitempty"`
	externalpolicy.ExternalPolicy                               // top level properties and constraints,
	Deployment                    externalpolicy.ExternalPolicy `json:"deployment,omitempty"` // properties and constrians for deopoyment
	Management                    externalpolicy.ExternalPolicy `json:"management,omitempty"` // properties and constrians for node management
}

func (n NodePolicy) String() string {
	return fmt.Sprintf("NodePolicy: Label: %v, Description: %v, Properties: %v, Constraints: %v, Deployment: %v, Management: %v", n.Label, n.Description, n.Properties, n.Constraints, n.Deployment, n.Management)
}

// This function validates the properties and constrains. It also updates the node's
// writable built-in properties inside this policy to make sure they have correct data types.
// The validation returns errors if the policy does not validate. It uses the constraint language
// plugins to handle the constraints field.
func (n *NodePolicy) ValidateAndNormalize() error {
	if err := (&n.ExternalPolicy).ValidateAndNormalize(); err != nil {
		return err
	}

	if err := (&n.Deployment).ValidateAndNormalize(); err != nil {
		return err
	}
	if err := (&n.Management).ValidateAndNormalize(); err != nil {
		return err
	}

	// We only get here if the input object is nil OR all of the top level fields are empty.
	return nil
}

// return a pointer to a copy of NodePolicy
func (n NodePolicy) DeepCopy() *NodePolicy {
	copyN := NodePolicy{}

	copyN.Label = n.Label
	copyN.Description = n.Description

	copyN.Properties = externalpolicy.CopyProperties(n.Properties)
	copyN.Constraints = externalpolicy.CopyConstraints(n.Constraints)

	copyN.Deployment = *(n.Deployment.DeepCopy())

	copyN.Management = *(n.Management.DeepCopy())

	return &copyN
}

// return the properties and constrians for node deployment.
// The properties are from both top level and the Deployment level with
// the Deployment level take precedence if the property name is the same.
// The constrains are from Deployment level if it exists. Otherwise take
// from the top level.
func (n NodePolicy) GetDeploymentPolicy() *externalpolicy.ExternalPolicy {
	copyE := externalpolicy.ExternalPolicy{}

	// For properties, merge the top level and the Deployment level
	copyE.Properties = externalpolicy.CopyProperties(n.Properties)
	if len(n.Deployment.Properties) != 0 {
		if copyE.Properties == nil {
			copyE.Properties = externalpolicy.CopyProperties(n.Deployment.Properties)
		} else {
			(&copyE.Properties).MergeWith(&n.Deployment.Properties, true)
		}
	}

	// For constraints, always take the Deployment.Constrains if it exists.
	// Take the top level constraints otherwise.
	if len(n.Deployment.Constraints) != 0 {
		copyE.Constraints = externalpolicy.CopyConstraints(n.Deployment.Constraints)
	} else {
		copyE.Constraints = externalpolicy.CopyConstraints(n.Constraints)
	}

	return &copyE
}

// return the properties and constrians for node management.
// The properties are from both top level and the Management level with
// the Management level take precedence if the property name is the same.
// The constrains are from Management level if it exists. Otherwise take
// from the top level.
func (n NodePolicy) GetManagementPolicy() *externalpolicy.ExternalPolicy {
	copyE := externalpolicy.ExternalPolicy{}

	// For properties, merge the top level and the Management level
	copyE.Properties = externalpolicy.CopyProperties(n.Properties)
	if len(n.Management.Properties) != 0 {
		if copyE.Properties == nil {
			copyE.Properties = externalpolicy.CopyProperties(n.Management.Properties)
		} else {
			(&copyE.Properties).MergeWith(&n.Management.Properties, true)
		}
	}

	// For constraints, always take the Management.Constrains if it exists.
	// Take the top level constraints otherwise.
	if len(n.Management.Constraints) != 0 {
		copyE.Constraints = externalpolicy.CopyConstraints(n.Management.Constraints)
	} else {
		copyE.Constraints = externalpolicy.CopyConstraints(n.Constraints)
	}
	return &copyE
}

// Convert the node policy from version v1 to version v2.
// It keeps the node built-in properties on the top level and move other properties and
// all the constraints to the Deployment level.
// The node built-in property names are defined in externalpolicy/builtin_properties.go.
func ConvertNodePolicy_v1Tov2(nodePol_v1 externalpolicy.ExternalPolicy) *NodePolicy {
	nodePol_v2 := NodePolicy{}
	if nodePol_v1.Properties != nil && len(nodePol_v1.Properties) != 0 {
		propDeploy := new(externalpolicy.PropertyList)
		propTop := new(externalpolicy.PropertyList)
		for _, prop := range nodePol_v1.Properties {
			copyP := prop
			if externalpolicy.IsNodeBuiltinPropertyName(prop.Name) {
				propTop.Add_Property(&copyP, false)
			} else {
				propDeploy.Add_Property(&copyP, false)
			}
		}
		nodePol_v2.Properties = *propTop
		nodePol_v2.Deployment.Properties = *propDeploy
	}

	if nodePol_v1.Constraints != nil && len(nodePol_v1.Constraints) != 0 {
		nodePol_v2.Deployment.Constraints = externalpolicy.CopyConstraints(nodePol_v1.Constraints)
	}

	return &nodePol_v2
}

// Check if the properties and the constrains under Deployment are both empty
func (n NodePolicy) IsDeploymentEmpty() bool {
	return len(n.Deployment.Properties) == 0 && len(n.Deployment.Constraints) == 0
}

// Check if the properties and the constrains under Management are both empty
func (n NodePolicy) IsManagementEmpty() bool {
	return len(n.Management.Properties) == 0 && len(n.Management.Constraints) == 0
}

// Compare with the given node policy
// Assuming both node policies have been validated.
// It returns the comparision result code that is defined in externalpolicy/ExternalPolicy.go
// (EP_COMPARE_*) for deployment and management.
func (np NodePolicy) CompareWith(nodePol2 *NodePolicy) (int, int) {
	if nodePol2 == nil {
		return externalpolicy.EP_COMPARE_DELETED, externalpolicy.EP_COMPARE_DELETED
	}

	// compare deployment policies
	depl_pol1 := np.GetDeploymentPolicy()
	depl_pol2 := nodePol2.GetDeploymentPolicy()
	depl_rc := depl_pol1.CompareWith(depl_pol2)

	// compare magement policies
	mgmt_pol1 := np.GetManagementPolicy()
	mgmt_pol2 := nodePol2.GetManagementPolicy()
	mgmt_rc := mgmt_pol1.CompareWith(mgmt_pol2)

	return depl_rc, mgmt_rc
}
