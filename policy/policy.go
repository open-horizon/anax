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
// sensorVersion is a pointer to a string.
// If it is nil, it will have the old behaviour befor the ms split. The version will be default to 1.0.0.
// If it is not nil, it will have the new behaviour that the user is registering a microservice. The version value will be used. An empty version means that it
// can take any version.
func GeneratePolicy(e chan events.Message, sensorUrl string, sensorName string, sensorVersion *string, arch string, props *map[string]interface{}, haPartners []string, meterPolicy Meter, counterPartyProperties RequiredProperty, agps []AgreementProtocol, filePath string) error {

	glog.V(5).Infof("Generating policy for %v", sensorName)

	if len(arch) == 0 {
		return errors.New(fmt.Sprintf("Error generating policy, architecture not specified"))
	}

	var s_version, s_url, fileName string
	if sensorVersion == nil {
		// for backward compatibility
		s_version = "1.0.0"
		if len(sensorName) == 0 {
			return errors.New(fmt.Sprintf("Error generating policy, sensorName not specified"))
		}
		fileName = strings.ToLower(strings.Split(sensorName, " ")[0])
		s_url = "https://bluehorizon.network/documentation/" + fileName + "-device-api"
	} else {
		// new behaviour: the input is for a ms
		s_version = *sensorVersion
		if len(sensorUrl) == 0 {
			return errors.New(fmt.Sprintf("Error generating policy, sensorUrl not specified"))
		}
		a_tmp := strings.Split(sensorUrl, "/")
		fileName = a_tmp[len(a_tmp)-1]
		s_url = sensorUrl
	}

	p := Policy_Factory("Policy for " + fileName)
	p.Add_API_Spec(APISpecification_Factory(s_url, s_version, arch))

	if len(agps) != 0 {
		for _, agpEle := range agps {
			p.Add_Agreement_Protocol(&agpEle)
		}
	} else {
		agp := AgreementProtocol_Factory(CitizenScientist)
		agp.Initialize()
		p.Add_Agreement_Protocol(agp)
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

	*pl = append(*pl, Property{Name: "arch", Value: policy.APISpecs[0].Arch})
	*pl = append(*pl, Property{Name: "agreementProtocols", Value: policy.AgreementProtocols.As_String_Array()})

	return pl, nil
}
