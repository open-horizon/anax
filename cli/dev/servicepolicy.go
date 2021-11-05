package dev

import (
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/externalpolicy"
)

const SERVICE_POLICY_FILE = "service.policy.json"
const DEFUALT_PROP_NAME = "prop1"
const DEFAULT_PROP_VALUE = "value1"

// Sort of like a constructor, it creates an in memory object from the service policy file. This function assumes the caller has determined the exact location of the file.
func GetServicePolicy(directory string, name string) (*exchangecommon.ServicePolicy, error) {
	res := new(exchangecommon.ServicePolicy)
	// GetFile will write to the res object, demarshalling the bytes into a json object that can be returned.
	if err := GetFile(directory, name, res); err != nil {
		return nil, err
	}
	return res, nil

}

// Sort of like a constructor, it creates a service policy object and writes it to the project
// in the file system.
func CreateServicePolicy(directory string) error {
	svcBuiltInProps := new(externalpolicy.PropertyList)
	svcBuiltInProps.Add_Property(externalpolicy.Property_Factory(DEFUALT_PROP_NAME, DEFAULT_PROP_VALUE), false)

	res := exchangecommon.ServicePolicy{
		ExternalPolicy: externalpolicy.ExternalPolicy{Properties: *svcBuiltInProps, Constraints: []string{}},
		Description:    "a policy for your service",
	}

	return CreateFile(directory, SERVICE_POLICY_FILE, res)
}

// Check for the existence of the service policy file in the project.
func ServicePolicyExists(directory string) (bool, error) {
	return FileExists(directory, SERVICE_POLICY_FILE)
}
