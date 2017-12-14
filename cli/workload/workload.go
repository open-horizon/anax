package workload

import (
	"encoding/json"
	"fmt"
	dockerclient "github.com/fsouza/go-dockerclient"
	"github.com/open-horizon/anax/cli/cliutils"
)

// Can't use the api and persistence structs because the Attributes type isn't detailed enough to drill down into it
type WorkloadConfigOnly struct {
	WorkloadURL       string                              `json:"workload_url"`
	Org               string                              `json:"organization"`
	VersionExpression string                              `json:"workload_version"` // This is a version range
	Attributes        []map[string]map[string]interface{} `json:"attributes"`
}

type APIWorkloads struct {
	Config     []WorkloadConfigOnly          `json:"config"`     // the workload configurations
	Containers *[]dockerclient.APIContainers `json:"containers"` // the docker info for a running container
}

// What we will output. Need our own structure because we want to pick and choose what we output.
type OurWorkload struct {
	WorkloadURL       string                 `json:"workload_url"`
	Org               string                 `json:"organization"`
	VersionExpression string                 `json:"workload_version"` // This is a version range
	Variables         map[string]interface{} `json:"variables"`
}

func List() {
	// Get the workloads
	var apiOutput APIWorkloads
	// Note: intentionally querying /workload, instead of just /workload/config, because in the future we will probably want to mix in some key runtime info
	httpCode := cliutils.HorizonGet("workload", []int{200, cliutils.ANAX_NOT_CONFIGURED_YET}, &apiOutput)
	if httpCode == cliutils.ANAX_NOT_CONFIGURED_YET {
		cliutils.Fatal(cliutils.HTTP_ERROR, cliutils.MUST_REGISTER_FIRST)
	}
	apiWorkloads := apiOutput.Config

	// Only include interesting fields in our output
	workloads := make([]OurWorkload, len(apiWorkloads))
	for i := range apiWorkloads {
		workloads[i].Org = apiWorkloads[i].Org
		workloads[i].WorkloadURL = apiWorkloads[i].WorkloadURL
		workloads[i].VersionExpression = apiWorkloads[i].VersionExpression
		// Copy all of the variables from each Mappings into our Variables map
		workloads[i].Variables = make(map[string]interface{})
		for _, a := range apiWorkloads[i].Attributes {
			if m, ok := a["mappings"]; ok {
				for k, v := range m {
					workloads[i].Variables[k] = v
				}
			}
		}
	}
	//todo: should we mix in any other info from /workload?

	// Convert to json and output
	jsonBytes, err := json.MarshalIndent(workloads, "", cliutils.JSON_INDENT)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'hzn workload list' output: %v", err)
	}
	fmt.Printf("%s\n", jsonBytes)
}
