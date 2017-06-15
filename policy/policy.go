package policy

import (
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/events"
	"os"
	"strings"
)

// This function generates policy files for each sensor on the device.
func GeneratePolicy(e chan events.Message, sensorName string, arch string, props *map[string]interface{}, haPartners []string, meterPolicy Meter, counterPartyProperties RequiredProperty, agps []string, filePath string) error {

	glog.V(5).Infof("Generating policy for %v", sensorName)

	if len(sensorName) == 0 {
		return errors.New(fmt.Sprintf("Error generating policy, sensorName not specified"))
	} else if len(arch) == 0 {
		return errors.New(fmt.Sprintf("Error generating policy, architecture not specified"))
	}

	fileName := strings.ToLower(strings.Split(sensorName, " ")[0])
	p := Policy_Factory("Policy for " + fileName)

	p.Add_API_Spec(APISpecification_Factory("https://bluehorizon.network/documentation/"+fileName+"-device-api", "1.0.0", arch))

	if len(agps) != 0 {
		for _, agp := range agps {
			p.Add_Agreement_Protocol(AgreementProtocol_Factory(agp))
		}
	} else {
		p.Add_Agreement_Protocol(AgreementProtocol_Factory(CitizenScientist))
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

	// Default the max agreements to 1
	p.MaxAgreements = 1

	// Store the policy on the filesystem
	fullFileName := filePath + fileName + ".policy"
	if err := os.MkdirAll(filePath, 0644); err != nil {
		return errors.New(fmt.Sprintf("Error writing policy file, cannot create file path %v", filePath))
	} else if err := WritePolicyFile(p, fullFileName); err != nil {
		return errors.New(fmt.Sprintf("Error writing out policy file %v, to %v, error: %v", *p, fullFileName, err))
	}

	// Fire an event
	glog.V(5).Infof("About to fire event for policy %v", fullFileName)
	e <- events.NewPolicyCreatedMessage(events.NEW_POLICY, fullFileName)

	glog.V(5).Infof("Queued policy %v for event handler", fullFileName)

	return nil
}

func RetrieveAllProperties(policy *Policy) (*PropertyList, error) {
	pl := new(PropertyList)

	for _, p := range policy.Properties {
		*pl = append(*pl, p)
	}

	*pl = append(*pl, Property{Name: "version", Value: policy.APISpecs[0].Version})
	*pl = append(*pl, Property{Name: "arch", Value: policy.APISpecs[0].Arch})
	*pl = append(*pl, Property{Name: "agreementProtocols", Value: policy.AgreementProtocols.As_String_Array()})

	return pl, nil
}
