package policy

import (
    "errors"
    "fmt"
    "github.com/golang/glog"
    "os"
    "github.com/open-horizon/anax/events"
    gpolicy "repo.hovitos.engineering/MTN/go-policy"
    "strings"
)

// Functions used to interact with the policy library

// Return the Sensor API Spec URL
func GetSenorApiSpecUrl(sensorName string) (string, error) {

	if len(sensorName) == 0 {
		return "", errors.New(fmt.Sprintf("Error getting sensor API specification url, sensorName not specified"))
	}

	sensor_short_name := strings.ToLower(strings.Split(sensorName, " ")[0])
	return "https://bluehorizon.network/documentation/" + sensor_short_name + "-device-api", nil
}

// This function generates policy files for each sensor on the device.
func GeneratePolicy(e chan events.Message, sensorName string, arch string, props *map[string]string, filePath string) error {

    glog.V(5).Infof("Generating policy for %v",sensorName)

    if len(sensorName) == 0 {
        return errors.New(fmt.Sprintf("Error generating policy, sensorName not specified"))
    } else if len(arch) == 0 {
        return errors.New(fmt.Sprintf("Error generating policy, architecture not specified"))
    }

    fileName := strings.ToLower(strings.Split(sensorName, " ")[0])
    p := gpolicy.Policy_Factory("Policy for " + fileName)

    p.Add_API_Spec(gpolicy.APISpecification_Factory("https://bluehorizon.network/documentation/"+ fileName + "-device-api", "1.0.0", 1, arch))
    p.Add_Agreement_Protocol(gpolicy.AgreementProtocol_Factory(gpolicy.CitizenScientist))
    
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
    details_bc1["genesis"]   = []string{"https://dal05.objectstorage.softlayer.net/v1/AUTH_773b8ed6-b3c8-4683-9d7a-dbe2ee11095e/volcano/genesis.json",
                                        "https://tok02.objectstorage.softlayer.net/v1/AUTH_773b8ed6-b3c8-4683-9d7a-dbe2ee11095e/volcano/genesis.json",
                                        "https://lon02.objectstorage.softlayer.net/v1/AUTH_773b8ed6-b3c8-4683-9d7a-dbe2ee11095e/volcano/genesis.json"}
    details = details_bc1
    p.Add_Blockchain(gpolicy.Blockchain_Factory(gpolicy.Ethereum_bc, details))

    // Add properties to the policy
    for prop, val := range (*props) {
        p.Add_Property(gpolicy.Property_Factory(prop, val))
    }

    // Store the policy on the filesystem
    fullFileName := filePath + fileName + ".policy"
    if err := os.MkdirAll(filePath, 0644); err != nil {
        return errors.New(fmt.Sprintf("Error writing policy file, cannot create file path %v", filePath))
    } else if err := gpolicy.WritePolicyFile(p, fullFileName); err != nil {
        return errors.New(fmt.Sprintf("Error writing out policy file %v, to %v, error: %v", *p, fullFileName, err))
    }

    // Fire an event
    glog.V(5).Infof("About to fire event for policy %v", fullFileName)
    e <- events.NewPolicyCreatedMessage(events.NEW_POLICY, fullFileName)

    glog.V(5).Infof("Queued policy %v for event handler", fullFileName)

    return nil
}


func GetAllPolicies(location string) ([]*gpolicy.Policy, error) {

    var errorDetected = 0

    policies := make([]*gpolicy.Policy, 0, 10)
    changeNotify := func(fileName string, policy *gpolicy.Policy) {
        policies = append(policies, policy)
    }

    deleteNotify := func(fileName string, policy *gpolicy.Policy) {
        glog.Errorf("Policy watcher invoked delete notification unexpectedly for %v.", fileName)
    }

    errorNotify := func(fileName string, err error) {
        glog.Errorf("Policy watcher invoked error notification unexpectedly for %v.", fileName)
        errorDetected += 1
    }

    // Read in all the existing policy files
    if err := gpolicy.PolicyFileChangeWatcher(location, changeNotify, deleteNotify, errorNotify, 0); err != nil {
        return nil, err
    } else if errorDetected != 0 {
        return nil, errors.New(fmt.Sprintf("%v errors occurred while reading policies from disk", errorDetected))
    } else {
        return policies, nil
    }
}

// func AreCompatible(producer_policy *gpolicy.Policy, consumer_policy *gpolicy.Policy) error {
//     return gpolicy.Are_Compatible(producer_policy, consumer_policy)
// }

func RetrieveAllProperties(policy *gpolicy.Policy) (*gpolicy.PropertyList, error) {
    pl := new(gpolicy.PropertyList)

    for _, p := range policy.Properties {
        *pl = append(*pl, p)
    }

    *pl = append(*pl, gpolicy.Property {Name:  "version", Value: policy.APISpecs[0].Version, })
    *pl = append(*pl, gpolicy.Property {Name:  "arch", Value: policy.APISpecs[0].Arch, })
    *pl = append(*pl, gpolicy.Property {Name:  "dataVerification", Value: policy.DataVerify.Enabled, })
    *pl = append(*pl, gpolicy.Property {Name:  "agreementProtocols", Value: policy.AgreementProtocols.As_String_Array(), })

    return pl, nil
}

func AllAgreementProtocols(location string) ([]string, error) {
    r := make([]string, 0, 10)
    m := make(map[string]int)
    if policies, err := GetAllPolicies(location); err != nil {
        return r, err
    } else {
        for _, p := range policies {
            for _, e := range p.AgreementProtocols {
                m[e.Name] = 1
            }
        }
        for agp, _ := range m {
            r = append(r, agp)
        }
        return r, nil
    }
}
