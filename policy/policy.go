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
func GeneratePolicy(e chan events.Message, sensorName string, arch string, props *map[string]string, filePath string) error {

	glog.V(5).Infof("Generating policy for %v", sensorName)

	if len(sensorName) == 0 {
		return errors.New(fmt.Sprintf("Error generating policy, sensorName not specified"))
	} else if len(arch) == 0 {
		return errors.New(fmt.Sprintf("Error generating policy, architecture not specified"))
	}

	fileName := strings.ToLower(strings.Split(sensorName, " ")[0])
	p := Policy_Factory("Policy for " + fileName)

	p.MaxAgreements = 1
	p.Add_API_Spec(APISpecification_Factory("https://bluehorizon.network/documentation/"+fileName+"-device-api", "1.0.0", arch))
	p.Add_Agreement_Protocol(AgreementProtocol_Factory(CitizenScientist))

	// Hardwire to existing ethereum platform
	var details interface{}

	details_bc1 := make(map[string][]string)
	details_bc1["bootnodes"] = []string{"https://dal05.objectstorage.softlayer.net/v1/AUTH_773b8ed6-b3c8-4683-9d7a-dbe2ee11095e/volcano/peers",
		"https://tok02.objectstorage.softlayer.net/v1/AUTH_773b8ed6-b3c8-4683-9d7a-dbe2ee11095e/volcano/peers",
		"https://lon02.objectstorage.softlayer.net/v1/AUTH_773b8ed6-b3c8-4683-9d7a-dbe2ee11095e/volcano/peers"}
	details_bc1["networkid"] = []string{"https://dal05.objectstorage.softlayer.net/v1/AUTH_773b8ed6-b3c8-4683-9d7a-dbe2ee11095e/volcano/networkid",
		"https://tok02.objectstorage.softlayer.net/v1/AUTH_773b8ed6-b3c8-4683-9d7a-dbe2ee11095e/volcano/networkid",
		"https://lon02.objectstorage.softlayer.net/v1/AUTH_773b8ed6-b3c8-4683-9d7a-dbe2ee11095e/volcano/networkid"}
	details_bc1["directory"] = []string{"https://dal05.objectstorage.softlayer.net/v1/AUTH_773b8ed6-b3c8-4683-9d7a-dbe2ee11095e/volcano/directory.address",
		"https://tok02.objectstorage.softlayer.net/v1/AUTH_773b8ed6-b3c8-4683-9d7a-dbe2ee11095e/volcano/directory.address",
		"https://lon02.objectstorage.softlayer.net/v1/AUTH_773b8ed6-b3c8-4683-9d7a-dbe2ee11095e/volcano/directory.address"}
	details_bc1["genesis"] = []string{"https://dal05.objectstorage.softlayer.net/v1/AUTH_773b8ed6-b3c8-4683-9d7a-dbe2ee11095e/volcano/genesis.json",
		"https://tok02.objectstorage.softlayer.net/v1/AUTH_773b8ed6-b3c8-4683-9d7a-dbe2ee11095e/volcano/genesis.json",
		"https://lon02.objectstorage.softlayer.net/v1/AUTH_773b8ed6-b3c8-4683-9d7a-dbe2ee11095e/volcano/genesis.json"}
	details = details_bc1
	p.Add_Blockchain(Blockchain_Factory(Ethereum_bc, details))

	// Add properties to the policy
	for prop, val := range *props {
		p.Add_Property(Property_Factory(prop, val))
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
