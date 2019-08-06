package sync_service

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/edge-sync-service/common"
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
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if userPw == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("must specify exchange credentials to access the model management service."))
	}

	if details && objId == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("must specify object ID when requesting object details."))
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
			cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("no objects type '%s' found in org %s", objType, org))
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
			cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("object '%s' of type '%s' not found in org %s", objId, objType, org))
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
				cliutils.Verbose(msgPrinter.Sprintf("destination detail for object '%s' of type '%s' not found in org %s", objId, objType, org))
			}

			mmsObjectInfo.Destinations = objectDests

		}

		output := cliutils.MarshalIndent(mmsObjectInfo, "mms object list")
		fmt.Println(output)

		// Grab the object status and display it.
		urlPath = path.Join("api/v1/objects/", org, objType, objId, "status")
		var resp []byte
		cliutils.ExchangeGet("Model Management Service", cliutils.GetMMSUrl(), urlPath, cliutils.OrgAndCreds(org, userPw), []int{200}, &resp)
		msgPrinter.Printf("Object status: %v", string(resp))
		msgPrinter.Println()
	}

}


func ObjectNew(org string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Display an empty template for the metadata of an object in the MMS. The user can use this template on 'hzn mms object publish' to provide
	// the object definition when uploading it to the MMS. The policy section is filled in with empty values so that the user can see the
	// schema of fields.

	// This struct is a copy (and subset) of github.com/open-horizon/edge-sync-service/common.Metadata os that we only show the
	// user which fields they should be setting

	var hzn_object_metadata = []string{
		`{`, 
  		`  "objectID": "",            /* ` + msgPrinter.Sprintf("Required: A unique identifier of the object.") + ` */`, 
  		`  "objectType": "",          /* ` + msgPrinter.Sprintf("Required: The type of the object.") + ` */`, 
  		`  "destinationOrgID": "$HZN_ORG_ID", /* ` + msgPrinter.Sprintf("Required: The organization ID of the object (an object belongs to exactly one organization).") + ` */`, 
  		`  "destinationID": "",       /* ` + msgPrinter.Sprintf("The node id (without org prefix) where the object should be placed.") + ` */`, 
        `                             /* ` + msgPrinter.Sprintf("If omitted the object is sent to all nodes the same destinationType.") + ` */`, 
        `                             /* ` + msgPrinter.Sprintf("Delete this field when you are using destinationPolicy.") + ` */`, 
  		`  "destinationType": "",     /* ` + msgPrinter.Sprintf("The pattern in use by nodes that should receive this object.") + ` */`, 
  		`                             /* ` + msgPrinter.Sprintf("If omitted (and if destinationsList is omitted too) the object is broadcast to all known nodes.") + ` */`, 
  		`                             /* ` + msgPrinter.Sprintf("Delete this field when you are using policy.") + ` */`, 
  		`  "destinationsList": null,  /* ` + msgPrinter.Sprintf("The list of destinations as an array of pattern:nodeId pairs that should receive this object.") + ` */`, 
  		`                             /* ` + msgPrinter.Sprintf("If provided, destinationType and destinationID must be omitted.") + ` */`, 
  		`                             /* ` + msgPrinter.Sprintf("Delete this field when you are using policy.") + ` */`, 
  		`  "destinationPolicy": {     /* ` + msgPrinter.Sprintf("The policy specification that should be used to distribute this object.") + ` */`, 
  		`                             /* ` + msgPrinter.Sprintf("Delete these fields if the target node is using a pattern.") + ` */`, 
  		`    "properties": [          /* ` + msgPrinter.Sprintf("A list of policy properties that describe the object.") + ` */`, 
  		`      {`,
  		`        "name": "",`, 
  		`        "value": nil,`,
 	    `        "type": ""           /* ` + msgPrinter.Sprintf("Valid types are string, bool, int, float, list of string (comma separated), version.") + ` */`, 
  		`                             /* ` + msgPrinter.Sprintf("Type can be omitted if the type is discernable from the value, e.g. unquoted true is boolean.") + ` */`, 
  		`      }`,
  		`    ],`,
  		`    "constraints": [         /* ` + msgPrinter.Sprintf("A list of constraint expressions of the form <property name> <operator> <property value>, separated by boolean operators AND (&&) or OR (||).") + ` */`, 
  		`      ""`,
  		`    ],`,
  		`    "services": [            /* ` + msgPrinter.Sprintf("The service(s) that will use this object.") + ` */`, 
  		`      {`,
  		`        "orgID": "",         /* ` + msgPrinter.Sprintf("The org of the service.") + ` */`, 
  		`        "serviceName": "",   /* ` + msgPrinter.Sprintf("The name of the service.") + ` */`, 
  		`        "arch": "",          /* ` + msgPrinter.Sprintf("Set to '*' to indcate services of any hardware architecture.") + ` */`, 
  		`        "version": ""        /* ` + msgPrinter.Sprintf("A version range.") + ` */`, 
  		`      }`,
  		`    ]`,
  		`  },`,
  		`  "expiration": "",          /* ` + msgPrinter.Sprintf("A timestamp/date indicating when the object expires (it is automatically deleted). The timestamp should be provided in RFC3339 format. ") + ` */`, 
  		`  "version": "",             /* ` + msgPrinter.Sprintf("Arbitrary string value. The value is not semantically interpreted. The Model Management System does not keep multiple version of an object.") + ` */`, 
  		`  "description": "",         /* ` + msgPrinter.Sprintf("An arbitrary description.") + ` */`, 
  		`  "activationTime": ""       /* ` + msgPrinter.Sprintf("A timestamp/date as to when this object should automatically be activated. The timestamp should be provided in RFC3339 format.") + ` */`, 
		`}`,
  	}
	// Display the limited object metadata that the user is allowed to set.
	for _, s := range(hzn_object_metadata) {
		fmt.Println(s)
	}	

}

// Upload an object to the MMS. The user can provide a copy of the object's metadata in a file, or they can simply provide
// object id and type.
func ObjectPublish(org string, userPw string, objType string, objId string, objPattern string, objMetadataFile string, objFile string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Validate the inputs because the combination of inputs that are required is complex.
	if userPw == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("must specify exchange credentials to access the model management service"))
	}

	if objType == "" && objId != "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("must specify --type with --id"))
	} else if objType != "" && objId == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("must specify --id with --type"))
	} else if objType != "" && objMetadataFile != "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("cannot specify --id and --type with --def"))
	} else if objType == "" && objMetadataFile == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("must specify either --type and --id or --def"))
	} else if objPattern != "" && objMetadataFile != "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("cannot specify --pattern with --def"))
	}

	// If we were given a full metadata file, read it in and use it to create the object. Otherwise, construct a minimal
	// object metadata file based on the other input paramaters.
	var objectMeta common.MetaData
	if objMetadataFile != "" {
		if _, err := os.Stat(objMetadataFile); err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("unable to read definition file %v: %v", objMetadataFile, err))
		}
		metaBytes := cliutils.ReadJsonFile(objMetadataFile)
		if err := json.Unmarshal(metaBytes, &objectMeta); err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal definition file %s: %v", objMetadataFile, err))
		}
	} else {
		objectMeta.ObjectID = objId
		objectMeta.ObjectType = objType
		objectMeta.DestOrgID = org
		objectMeta.DestType = objPattern
	}

	// If there is no data to upload, set the metaonly flag to indicate that we are only updating the object's metadata. This ensures
	// that the MSS (CSS) correctly interpets the PUT.
	if objFile == "" {
		objectMeta.MetaOnly = true
	}

	type ObjectWrapper struct {
		Meta common.MetaData `json:"meta"`
		Data []byte          `json:"data"`
	}

	wrapper := ObjectWrapper{Meta: objectMeta}

	// Call the MMS service over HTTP to add the object's metadata to the MMS.
	urlPath := path.Join("api/v1/objects/", org, objectMeta.ObjectType, objectMeta.ObjectID)
	cliutils.ExchangePutPost("Model Management Service", http.MethodPut, cliutils.GetMMSUrl(), urlPath, cliutils.OrgAndCreds(org, userPw), []int{204}, wrapper)

	// The object's data might be quite large, so upload it in a second call that will stream the file contents
	// to the MSS (CSS).
	if objFile != "" {

		file, err := os.Open(objFile)
		if err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("unable to open object file %v: %v", objFile, err))
		}
		defer file.Close()

		// Stream the file to the MMS (CSS).
		urlPath = path.Join("api/v1/objects/", org, objectMeta.ObjectType, objectMeta.ObjectID, "data")
		cliutils.ExchangePutPost("Model Management Service", http.MethodPut, cliutils.GetMMSUrl(), urlPath, cliutils.OrgAndCreds(org, userPw), []int{204}, file)

		cliutils.Verbose(msgPrinter.Sprintf("Object %v uploaded to org %v in the Model Management Service", objFile, org))
	}

	// Grab the object status and display it.
	urlPath = path.Join("api/v1/objects/", org, objectMeta.ObjectType, objectMeta.ObjectID, "status")
	var resp []byte
	cliutils.ExchangeGet("Model Management Service", cliutils.GetMMSUrl(), urlPath, cliutils.OrgAndCreds(org, userPw), []int{200}, &resp)
	cliutils.Verbose(msgPrinter.Sprintf("Object status: %v", string(resp)))

	msgPrinter.Printf("Object %v added to org %v in the Model Management Service", objectMeta.ObjectID, org)
	msgPrinter.Println()

}

// Upload an object to the MMS. The user can provide a copy of the object's metadata in a file, or they can simply provide
// object id and type.
func ObjectDelete(org string, userPw string, objType string, objId string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if userPw == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("must specify exchange credentials to access the model management service"))
	}

	// For this command, object type and id are required parameters, No null checking is needed.

	// Set the API key env var if that's what we're using.
	cliutils.SetWhetherUsingApiKey(userPw)

	// Call the MMS service over HTTP to delete the object.
	urlPath := path.Join("api/v1/objects/", org, objType, objId)
	httpCode := cliutils.ExchangeDelete("Model Management Service", cliutils.GetMMSUrl(), urlPath, cliutils.OrgAndCreds(org, userPw), []int{204, 404})
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("object '%s' of type '%s' not found in org %s", objId, objType, org))
	}

	msgPrinter.Printf("Object %v deleted from org %v in the Model Management Service", objId, org)
	msgPrinter.Println()

}
