package attribute

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/cli/cliutils"
)

const HTTPSBasicAuthAttributes = "HTTPSBasicAuthAttributes"

// Our form of the attributes output
type OurAttributes struct {
	Type       string                 `json:"type"`
	Label      string                 `json:"label"`
	SensorUrls []string               `json:"sensor_urls,omitempty"`
	Variables  map[string]interface{} `json:"variables"`
}

func List() {
	// Get the attributes
	apiOutput := map[string][]api.Attribute{}
	httpCode := cliutils.HorizonGet("attribute", []int{200, cliutils.ANAX_NOT_CONFIGURED_YET}, &apiOutput)
	if httpCode == cliutils.ANAX_NOT_CONFIGURED_YET {
		cliutils.Fatal(cliutils.HTTP_ERROR, cliutils.MUST_REGISTER_FIRST)
	}
	var ok bool
	if _, ok = apiOutput["attributes"]; !ok {
		cliutils.Fatal(cliutils.HTTP_ERROR, "horizon api attributes output did not include 'attributes' key")
	}
	apiAttrs := apiOutput["attributes"]

	// Only include interesting fields in our output
	var attrs []OurAttributes
	for _, a := range apiAttrs {
		if len(*a.SensorUrls) == 0 {
			attrs = append(attrs, OurAttributes{Type: *a.Type, Label: *a.Label, Variables: *a.Mappings})
		} else if *a.Type == HTTPSBasicAuthAttributes {
			attrs = append(attrs, OurAttributes{Type: *a.Type, Label: *a.Label, SensorUrls: *a.SensorUrls, Variables: *a.Mappings})
		}
	}

	// Convert to json and output
	jsonBytes, err := json.MarshalIndent(attrs, "", cliutils.JSON_INDENT)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'hzn attribute list' output: %v", err)
	}
	fmt.Printf("%s\n", jsonBytes)
}
