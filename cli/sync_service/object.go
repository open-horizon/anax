package sync_service

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/exchange"
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
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "must specify exchange credentials to access the model management service.")
	}

	if details && objId == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "must specify object ID when requesting object details.")
	}

	// Set the API key env var if that's what we're using.
	cliutils.SetWhetherUsingApiKey(userPw)

	// If object ID is omitted, query all objects of the given type.
	if objId == "" {
		objectList := new(exchange.ObjectDestinationPolicies)
		urlPath := path.Join("api/v1/objects/", org, objType, "?all_objects=true")

		// Call the MMS service over HTTP to get the basic object metadata.
		httpCode := cliutils.ExchangeGet("Model Management Service", cliutils.GetMMSUrl(), urlPath, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, objectList)
		if httpCode == 404 {
			cliutils.Fatal(cliutils.NOT_FOUND, "no objects type '%s' found in org %s", objType, org)
		}

		output := cliutils.MarshalIndent(objectList, "mms object list")
		fmt.Println(output)

	} else {
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

}

// Display an empty template for the metadata of an object in the MMS. The user can use this template on 'hzn mms object publish' to provide
// the object definition when uploading it to the MMS. The policy section is filled in with empty values so that the user can see the
// schema of fields.

// This struct is a copy (and subset) of github.com/open-horizon/edge-sync-service/common.Metadata os that we only show the
// user which fields they should be setting

const HZN_OBJECT_METADATA = `{
  "objectID": "",                    /* Required: A unique identifier of the object */
  "objectType": "",                  /* Required: The type of the object. */
  "destinationOrgID": "$HZN_ORG_ID", /* Required: The organization ID of the object (an object belongs to exactly one organization). */
  "destinationID": "",               /* The node id (without org prefix) where the object should be placed. */
                                     /* If omitted the object is sent to all nodes the same destinationType. */
                                     /* Delete this field when you are using destinationPolicy. */
  "destinationType": "",             /* The pattern in use by nodes that should receive this object. */
                                     /* If omitted (and if destinationsList is omitted too) the object is broadcast to all known nodes. */
                                     /* Delete this field when you are using policy. */
  "destinationsList": null,          /* The list of destinations as an array of pattern:nodeId pairs that should receive this object. */
                                     /* If provided, destinationType and destinationID must be omitted. */
                                     /* Delete this field when you are using policy. */
  "destinationPolicy": {             /* The policy specification that should be used to distribute this object. */
                                     /* Delete these fields if the target node is using a pattern. */
    "properties": [   /* A list of policy properties that describe the object. */
      {
        "name": "", 
        "value": nil,
        "type": ""    /* Valid types are string, bool, int, float, list of string (comma separated), version. */
                      /* Type can be omitted if the type is discernable from the value, e.g. unquoted true is boolean. */
      }
    ],
    "constraints": [  /* A list of constraint expressions of the form <property name> <operator> <property value>, separated by boolean operators AND (&&) or OR (||). */
      ""
    ],
    "services": [     /* The service(s) that will use this object. */
      {
        "orgID": "",        /* The org of the service. */
        "serviceName": "",  /* The name of the service. */
        "arch": "",         /* Set to '*' to indcate services of any hardware architecture. */
        "version": ""       /* A version range. */
      }
    ]
  },
  "expiration": "",     /* A timestamp/date indicating when the object expires (it is automatically deleted). The timestamp should be provided in RFC3339 format. */
  "version": "",        /* Arbitrary string value. The value is not semantically interpreted. The Model Management System does not keep multiple version of an object. */
  "description": "",    /* An arbitrary description. */
  "activationTime": ""  /* A timestamp/date as to when this object should automatically be activated. The timestamp should be provided in RFC3339 format. */
}`

func ObjectNew(org string) {

	// Display the limited object metadata that the user is allowed to set.
	fmt.Println(HZN_OBJECT_METADATA)

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

	objString := objectMeta.ObjectID
	wrapper := ObjectWrapper{Meta: objectMeta}

	// Upload the file contents and the object described by objectMeta.
	if objFile != "" {
		if _, err := os.Stat(objFile); err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "unable to read object file %v: %v", objFile, err)
		}

		if fileBytes, err := ioutil.ReadFile(objFile); err != nil {
			cliutils.Fatal(cliutils.FILE_IO_ERROR, "reading %s failed: %v", objFile, err)
		} else {
			wrapper.Data = fileBytes
			objString = objFile
		}
	} else {
		// If there is no data to upload, set the metaonly flag to indicate that we are only updating the object's metadata. This ensures
		// that the MSS (CSS) correctly interpets the PUT.
		wrapper.Meta.MetaOnly = true
	}

	// Call the MMS service over HTTP to add the metadata and the object (if provided).
	urlPath := path.Join("api/v1/objects/", org, objectMeta.ObjectType, objectMeta.ObjectID)
	cliutils.ExchangePutPost("Model Management Service", http.MethodPut, cliutils.GetMMSUrl(), urlPath, cliutils.OrgAndCreds(org, userPw), []int{204}, wrapper)

	// Grab the object status and display it.
	urlPath = path.Join("api/v1/objects/", org, objectMeta.ObjectType, objectMeta.ObjectID, "status")
	var resp []byte
	cliutils.ExchangeGet("Model Management Service", cliutils.GetMMSUrl(), urlPath, cliutils.OrgAndCreds(org, userPw), []int{200}, &resp)
	cliutils.Verbose("Object status: %v", string(resp))

	fmt.Println("Object " + objString + " added to org " + org + " in the Model Management Service")

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
