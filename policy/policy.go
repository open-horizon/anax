package policy

import (
	"github.com/golang/glog"
	"github.com/open-horizon/anax/events"
	"strings"
)

// This function generates policy files for each sensor on the device.
// sensorVersion is a pointer to a string.
// If it is nil, it will have the old behaviour befor the ms split. The version will be default to 1.0.0.
// If it is not nil, it will have the new behaviour that the user is registering a microservice. The version value will be used. An empty version means that it
// can take any version.
// maxAgreements: 0 means unlimited.

func GeneratePolicy(sensorUrl string, sensorOrg string, sensorName string, sensorVersion string, arch string, props *map[string]interface{}, haPartners []string, meterPolicy Meter, counterPartyProperties RequiredProperty, agps []AgreementProtocol, maxAgreements int, filePath string, deviceOrg string) (*events.PolicyCreatedMessage, error) {

	glog.V(5).Infof("Generating policy for %v", sensorUrl)

	// Generate a policy file name
	a_tmp := strings.Split(sensorUrl, "/")
	fileName := a_tmp[len(a_tmp)-1]

	p := Policy_Factory("Policy for " + fileName)
	p.Add_API_Spec(APISpecification_Factory(sensorUrl, sensorOrg, sensorVersion, arch))

	if len(agps) != 0 {
		for _, agpEle := range agps {
			p.Add_Agreement_Protocol(&agpEle)
		}
	}

	// Add properties to the policy
	for prop, val := range *props {
		p.Add_Property(Property_Factory(prop, val))
	}

	// Add HA configuration if there is any
	if len(haPartners) != 0 {
		p.Add_HAGroup(HAGroup_Factory(haPartners))
	}

	// Add Metering policy to the policy file
	if meterPolicy.Tokens != 0 {
		p.Add_DataVerification(DataVerification_Factory("", "", "", 0, 0, meterPolicy))
	}

	// Add counterparty properties if there are any
	if len(counterPartyProperties) != 0 {
		p.Add_CounterPartyProperties(&counterPartyProperties)
	}

	p.MaxAgreements = maxAgreements

	// Store the policy on the filesystem
	if fullFileName, err := CreatePolicyFile(filePath, deviceOrg, fileName, p); err != nil {
		return nil, err
	} else {

		// Create the new policy event
		msg := events.NewPolicyCreatedMessage(events.NEW_POLICY, fullFileName)
		return msg, nil
	}
}

func RetrieveAllProperties(policy *Policy) (*PropertyList, error) {
	pl := new(PropertyList)

	for _, p := range policy.Properties {
		*pl = append(*pl, p)
	}

	*pl = append(*pl, Property{Name: "arch", Value: policy.APISpecs[0].Arch})

	if len(policy.AgreementProtocols) != 0 {
		*pl = append(*pl, Property{Name: "agreementProtocols", Value: policy.AgreementProtocols.As_String_Array()})
	}

	return pl, nil
}
