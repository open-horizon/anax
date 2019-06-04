package sync_service

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/edge-sync-service/common"
	"io/ioutil"
	"net/http"
	"os"
	"path"
)

type MMSObjectInfo struct {
	Definition   common.MetaData             `json:"definition"`
	Destinations []common.DestinationsStatus `json:"destinations,omitempty"`
}

// Display the object metadata for a given object in the MMS.
func ObjectList(org string, userPw string, objType string, objId string, details bool) {

	if userPw == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "must specify exchange credentials to access the model management service")
	}

	// For this command, object type and id are required parameters, No null checking is needed.

	// Set the API key env var if that's what we're using.
	cliutils.SetWhetherUsingApiKey(userPw)

	// Display the full object metadata.
	var objectMeta common.MetaData

	// Construct the URL path from the input pieces. They are required inputs so we know they are at least non-null.
	urlPath := path.Join("api/v1/objects/", org, objType, objId)

	// Call the MMS service over HTTP to get the basic object metadata.
	httpCode := cliutils.ExchangeGet("Model Management Service", cliutils.GetMMSUrl(), urlPath, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &objectMeta)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, "object '%s' of type '%s' not found in org %s", objId, objType, org)
	}

	mmsObjectInfo := MMSObjectInfo{
		Definition: objectMeta,
	}

	// If the user wants additional details, provide the destination information from the destination API.
	if details {

		// Display the full object metadata.
		var objectDests []common.DestinationsStatus

		// Construct the URL path the additional destination detail.
		urlPath := path.Join("api/v1/objects/", org, objType, objId, "destinations")

		// Call the MMS service over HTTP to get the object's destination status.
		httpCode := cliutils.ExchangeGet("Model Management Service", cliutils.GetMMSUrl(), urlPath, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &objectDests)
		if httpCode == 404 {
			cliutils.Verbose("destination detail for object '%s' of type '%s' not found in org %s", objId, objType, org)
		}

		mmsObjectInfo.Destinations = objectDests

	}

	output := cliutils.MarshalIndent(mmsObjectInfo, "mms object list")
	fmt.Println(output)

}

// Display an empty template for the metadata of an object in the MMS. The user can use this template on 'hzn mms object publish' to provide
// the object definition when uploading it to the MMS. The policy section is filled in with empty values so that the user can see the
// schema of fields.
func ObjectNew(org string) {

	// Display the full object metadata
	var objectMeta common.MetaData
	objectMeta.DestOrgID = org
	objectMeta.DestinationPolicy = &common.Policy{
		Properties: []common.PolicyProperty{
			common.PolicyProperty{
				Name:  "",
				Value: "",
				Type:  "string",
			},
		},
		Constraints: []string{""},
		Services: []common.ServiceID{
			common.ServiceID{
				OrgID:       "",
				Arch:        "",
				ServiceName: "",
				Version:     "",
			},
		},
	}

	output := cliutils.MarshalIndent(objectMeta, "mms object metadata")
	fmt.Println(output)

}

// Upload an object to the MMS. The user can provide a copy of the object's metadata in a file, or they can simply provide
// object id and type.
func ObjectPublish(org string, userPw string, objType string, objId string, objPattern string, objMetadataFile string, objFile string) {

	// Validate the inputs because the combination of inputs that are required is complex.
	if userPw == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "must specify exchange credentials to access the model management service")
	}

	if objType == "" && objId != "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "must specify --type with --id")
	} else if objType != "" && objId == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "must specify --id with --type")
	} else if objType != "" && objMetadataFile != "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "cannot specify --id and --type with --def")
	} else if objType == "" && objMetadataFile == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "must specify either --type and --id or --def")
	} else if objPattern != "" && objMetadataFile != "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "cannot specify --pattern with --def")
	}

	// If we were given a full metadata file, read it in and use it to create the object. Otherwise, construct a minimal
	// object metadata file based on the other input paramaters.
	var objectMeta common.MetaData
	if objMetadataFile != "" {
		if _, err := os.Stat(objMetadataFile); err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "unable to read definition file %v: %v", objMetadataFile, err)
		}
		metaBytes := cliutils.ReadJsonFile(objMetadataFile)
		if err := json.Unmarshal(metaBytes, &objectMeta); err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to unmarshal definition file %s: %v", objMetadataFile, err)
		}
	} else {
		objectMeta.ObjectID = objId
		objectMeta.ObjectType = objType
		objectMeta.DestOrgID = org
		objectMeta.DestType = objPattern
	}

	type ObjectWrapper struct {
		Meta common.MetaData `json:"meta"`
		Data []byte          `json:"data"`
	}

	wrapper := ObjectWrapper{Meta: objectMeta}

	// Upload the file contents and the object described by objectMeta.
	if _, err := os.Stat(objFile); err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "unable to read object file %v: %v", objFile, err)
	}

	if fileBytes, err := ioutil.ReadFile(objFile); err != nil {
		cliutils.Fatal(cliutils.FILE_IO_ERROR, "reading %s failed: %v", objFile, err)
	} else {
		wrapper.Data = fileBytes
	}

	// Call the MMS service over HTTP to add the metadata and the object.
	urlPath := path.Join("api/v1/objects/", org, objectMeta.ObjectType, objectMeta.ObjectID)
	cliutils.ExchangePutPost("Model Management Service", http.MethodPut, cliutils.GetMMSUrl(), urlPath, cliutils.OrgAndCreds(org, userPw), []int{204}, wrapper)

	// urlPath = path.Join(urlPath, "data")
	// cliutils.ExchangePutPost("Model Management Service", http.MethodPut, cliutils.GetMMSUrl(), urlPath, cliutils.OrgAndCreds(org, userPw), []int{204}, fileBytes)

	fmt.Println("Object " + objFile + " added to org " + org + " in the Model Management Service")

}

// Upload an object to the MMS. The user can provide a copy of the object's metadata in a file, or they can simply provide
// object id and type.
func ObjectDelete(org string, userPw string, objType string, objId string) {

	if userPw == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "must specify exchange credentials to access the model management service")
	}

	// For this command, object type and id are required parameters, No null checking is needed.

	// Set the API key env var if that's what we're using.
	cliutils.SetWhetherUsingApiKey(userPw)

	// Call the MMS service over HTTP to delete the object.
	urlPath := path.Join("api/v1/objects/", org, objType, objId)
	httpCode := cliutils.ExchangeDelete("Model Management Service", cliutils.GetMMSUrl(), urlPath, cliutils.OrgAndCreds(org, userPw), []int{204, 404})
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, "object '%s' of type '%s' not found in org %s", objId, objType, org)
	}

	fmt.Println("Object " + objId + " deleted from org " + org + " in the Model Management Service")

}
