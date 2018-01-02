package service

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/cutil"
)

type APIMicroservices struct {
	Config      []api.APIMicroserviceConfig `json:"config"`      // the microservice configurations
	Instances   map[string][]interface{}    `json:"instances"`   // the microservice instances that are running
	Definitions map[string][]interface{}    `json:"definitions"` // the definitions of microservices from the exchange
}

// Not using policy.APISpecification, because it includes a field ExclusiveAccess that is filled in in various parts of the code that we can't easily duplicate
type APISpecification struct {
	SpecRef string `json:"specRef"`      // A URL pointing to the definition of the API spec
	Org     string `json:"organization"` // The organization where the microservice is defined
	Version string `json:"version"`      // The version of the API spec in OSGI version format
	Arch    string `json:"arch"`         // The hardware architecture of the API spec impl. Added in version 2.
}

type OurService struct {
	APISpecs  []APISpecification     `json:"apiSpec"` // removed omitempty
	Variables map[string]interface{} `json:"variables"`
}

func List() {
	// Get the services
	var apiOutput APIMicroservices
	// Note: intentionally querying /microservice, instead of just /microservice/config, because in the future we will probably want to mix in some key runtime info
	httpCode := cliutils.HorizonGet("microservice", []int{200, cliutils.ANAX_NOT_CONFIGURED_YET}, &apiOutput)
	if httpCode == cliutils.ANAX_NOT_CONFIGURED_YET {
		cliutils.Fatal(cliutils.HTTP_ERROR, cliutils.MUST_REGISTER_FIRST)
	}
	apiServices := apiOutput.Config

	// Go thru the services and pull out interesting fields
	services := make([]OurService, 0)
	for _, s := range apiServices {
		serv := OurService{Variables: make(map[string]interface{})}
		//asl := new(policy.APISpecList)
		//asl.Add_API_Spec(policy.APISpecification_Factory(s.SensorUrl, s.SensorOrg, s.SensorVersion, cutil.ArchString()))
		//serv.APISpecs = *asl
		serv.APISpecs = append(serv.APISpecs, APISpecification{SpecRef: s.SensorUrl, Org: s.SensorOrg, Version: s.SensorVersion, Arch: cutil.ArchString()})
		// Copy all of the variables from each Mappings into our Variables map
		for _, a := range s.Attributes {
			if a.Mappings != nil {
				for k, v := range *a.Mappings {
					serv.Variables[k] = v
				}
			}
		}
		services = append(services, serv)
	}
	//todo: should we mix in any info from /microservice/policy?

	// Convert to json and output
	jsonBytes, err := json.MarshalIndent(services, "", cliutils.JSON_INDENT)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'hzn service list' output: %v", err)
	}
	fmt.Printf("%s\n", jsonBytes)
}
