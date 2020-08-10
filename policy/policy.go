package policy

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/externalpolicy"
	"strings"
)

// This function generates policy files for each sensor on the device.
// sensorVersion is a pointer to a string.
// If it is nil, it will have the old behaviour befor the ms split. The version will be default to 1.0.0.
// If it is not nil, it will have the new behaviour that the user is registering a microservice. The version value will be used. An empty version means that it
// can take any version.
// maxAgreements: 0 means unlimited.

func GeneratePolicy(sensorUrl string, sensorOrg string, sensorName string, sensorVersion string, arch string, props *map[string]interface{}, haPartners []string, agps []AgreementProtocol, maxAgreements int, filePath string, deviceOrg string) (string, error) {

	glog.V(5).Infof("Generating policy for %v/%v", sensorOrg, sensorUrl)

	// Generate a policy file name
	a_tmp := strings.Split(sensorUrl, "/")
	fileName := fmt.Sprintf("%v_%v", sensorOrg, a_tmp[len(a_tmp)-1])

	p := Policy_Factory("Policy for " + fileName)
	p.Add_API_Spec(APISpecification_Factory(sensorUrl, sensorOrg, sensorVersion, arch))

	if len(agps) != 0 {
		for _, agpEle := range agps {
			p.Add_Agreement_Protocol(&agpEle)
		}
	}

	// Add properties to the policy
	for prop, val := range *props {
		p.Add_Property(externalpolicy.Property_Factory(prop, val), false)
	}

	// Add HA configuration if there is any
	if len(haPartners) != 0 {
		p.Add_HAGroup(HAGroup_Factory(haPartners))
	}

	p.MaxAgreements = maxAgreements

	// Store the policy on the filesystem
	if fullFileName, err := CreatePolicyFile(filePath, deviceOrg, fileName, p); err != nil {
		return "", err
	} else {
		return fullFileName, nil
	}
}

func RetrieveAllProperties(policy *Policy) (*externalpolicy.PropertyList, error) {
	pl := new(externalpolicy.PropertyList)

	for _, p := range policy.Properties {
		*pl = append(*pl, p)
	}

	if len(policy.APISpecs) > 0 {
		*pl = append(*pl, externalpolicy.Property{Name: "arch", Value: policy.APISpecs[0].Arch})
	}

	if len(policy.AgreementProtocols) != 0 {
		*pl = append(*pl, externalpolicy.Property{Name: "agreementProtocols", Value: policy.AgreementProtocols.As_String_Array()})
	}

	return pl, nil
}

// Create a header name for the generated policy that should be unique within the org.
// The input can be a device id or a service id.
func MakeExternalPolicyHeaderName(id string) string {
	return fmt.Sprintf("Policy for %v", id)
}

// Generate a policy from the external policy.
func GenPolicyFromExternalPolicy(extPol *externalpolicy.ExternalPolicy, polName string) (*Policy, error) {
	// validate first
	if err := extPol.ValidateAndNormalize(); err != nil {
		return nil, fmt.Errorf("Failed to validate the external policy: %v", extPol)
	}

	pPolicy := Policy_Factory(polName)

	for _, p := range extPol.Properties {
		if err := pPolicy.Add_Property(&p, false); err != nil {
			return nil, fmt.Errorf("Failed to add property %v to policy. %v", p, err)
		}
	}

	if err := pPolicy.Add_Constraints(&(extPol.Constraints)); err != nil {
		return nil, err
	}

	return pPolicy, nil
}
