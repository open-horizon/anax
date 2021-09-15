package sync_service

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/open-horizon/anax/cli/cliconfig"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/edge-sync-service/common"
	"github.com/open-horizon/rsapss-tool/sign"
	"hash"
	"io"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const BatchSize = 50

type MMSObjectInfo struct {
	ObjectID     string                      `json:"objectID,omitempty"`
	ObjectType   string                      `json:"objectType,omitempty"`
	Definition   *common.MetaData            `json:"definition,omitempty"`
	Destinations []common.DestinationsStatus `json:"destinations,omitempty"`
	ObjectStatus string                      `json:"objectStatus,omitempty"`
}

// Display the object metadata for given flags in the MMS.
func ObjectList(org string, userPw string, objType string, objId string, destPolicy string, dpService string, dpPropertyName string, dpUpdateTimeSince string, destType string, destId string, withData string, expirationTimeBefore string, deleted string, long bool, details bool) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if userPw == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("must specify exchange credentials to access the model management service."))
	}

	// Set the API key env var if that's what we're using.
	cliutils.SetWhetherUsingApiKey(userPw)

	var objectsMeta []common.MetaData

	// validate params:
	// 1. if --policy is not omitted, must set value to true or false
	//    1a. must omit --policy or set it to true when use --service, --property, or --updateTime
	//    1b. service should be in format: service-org/service-name
	//    1c. service exists
	//    1d. updateTime in RC3339 format or just use yyyy-MM-dd
	// 2. if --data is not omitted, must set value to true or false
	// 3. must set --objectType if use --objectId
	// 4. must set --destinationType if use --destinationId
	// 5. expiration in RC3339 format or use "now"
	// 6. if --deleted is not omitted, must set value to true or false
	if destPolicy != "" {
		if strings.ToLower(destPolicy) != "true" && strings.ToLower(destPolicy) != "false" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Invalid --policy/-p value: %s, --policy/-p should be true or false", destPolicy))
		} else {
			destPolicy = strings.ToLower(destPolicy)
		}
	}

	noData := ""
	if withData != "" {
		if strings.ToLower(withData) != "true" && strings.ToLower(withData) != "false" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Invalid --data value: %s, --data should be true or false", withData))
		} else {
			withDataBool, _ := strconv.ParseBool(strings.ToLower(withData))
			noData = strconv.FormatBool(!withDataBool)
		}
	}

	if dpService != "" || dpPropertyName != "" || dpUpdateTimeSince != "" {
		if destPolicy == "false" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("must omit --policy or set it to true when filtering by --service, --property, or --updateTime"))
		}

		if !dpServiceIsValid(dpService) {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("service should be in format service-org/service-name"))
		}

		timeValidated, convTimeStamp := timeIsValid(dpUpdateTimeSince)
		if !timeValidated {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("updateTime should be in RC3339 format: yyyy-MM-ddTHH:mm:ssZ, or yyyy-MM-dd"))
		} else {
			if convTimeStamp != "" {
				dpUpdateTimeSince = convTimeStamp
			}
		}

		destPolicy = "true"

	}

	if objType == "" && objId != "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("must specify --type with --id"))
	}

	if destType == "" && destId != "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("must specify destinationType if set destinationId"))
	}

	if strings.ToLower(expirationTimeBefore) == "now" {
		expirationTimeBefore = time.Now().Format(time.RFC3339)
	} else {
		if timeValidated, _ := timeIsValid(expirationTimeBefore); !timeValidated {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("expirationTimeBefore should be specified 'now' or timestamp in RC3339 format: yyyy-MM-ddTHH:mm:ssZ"))
		}
	}

	deletedValue := ""
	if deleted != "" {
		deletedValue = strings.ToLower(deleted)
		if deletedValue != "true" && deletedValue != "false" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Invalid --deleted value: %s, --data should be true or false", withData))
		}
	}

	filterURLPath := fmt.Sprintf("&objectType=%s&objectID=%s&destinationPolicy=%s&dpService=%s&dpPropertyName=%s&since=%s&destinationType=%s&destinationID=%s&noData=%s&expirationTimeBefore=%s&deleted=%s", objType, objId, destPolicy, dpService, dpPropertyName, dpUpdateTimeSince, destType, destId, noData, expirationTimeBefore, deletedValue)

	urlPath := "api/v1/objects/" + org + "?filters=true"
	fullPath := urlPath + filterURLPath

	// Call the MMS service over HTTP to get the basic object metadata.
	httpCode := cliutils.ExchangeGet("Model Management Service", cliutils.GetMMSUrl(), fullPath, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &objectsMeta)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("no objects found in org %s", org))
	}

	output := ""

	if details {
		// Cut the objectsMeta into batches of size 50. For each batch, process the API call concurrently. Use batches strategy to 1) reduce the processing time, 2) avoid overwhelming API calls sent to CSS server at one time
		batchSize := BatchSize
		var batches [][]common.MetaData
		for batchSize < len(objectsMeta) {
			objectsMeta, batches = objectsMeta[batchSize:], append(batches, objectsMeta[0:batchSize:batchSize])
		}
		batches = append(batches, objectsMeta)

		mmsObjects := make([]MMSObjectInfo, 0)
		c := make(chan MMSObjectInfo)

		for i := 0; i < len(batches); i++ {
			for _, obj := range batches[i] {

				// Pass in the iterator object by value so that a copy is made.
				// This function should not share variables with the main thread unless
				// those variables are unchanging, or safe to use concuurently on multiple threads.
				go func(obj common.MetaData) {

					//1. call destination API
					mmsObjectInfo := MMSObjectInfo{}
					var objectDests []common.DestinationsStatus

					mmsObjectInfo.ObjectType = obj.ObjectType
					mmsObjectInfo.ObjectID = obj.ObjectID
					mmsObjectInfo.Definition = &obj

					urlPath := path.Join("api/v1/objects/", org, obj.ObjectType, obj.ObjectID, "destinations")

					httpCode := cliutils.ExchangeGet("Model Management Service", cliutils.GetMMSUrl(), urlPath, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &objectDests)
					if httpCode == 404 {
						cliutils.Verbose(msgPrinter.Sprintf("destination detail for object '%s' of type '%s' not found in org %s", obj.ObjectID, obj.ObjectType, org))
					}
					mmsObjectInfo.Destinations = objectDests

					//2. call status API
					urlPath = path.Join("api/v1/objects/", org, obj.ObjectType, obj.ObjectID, "status")
					var resp []byte
					cliutils.ExchangeGet("Model Management Service", cliutils.GetMMSUrl(), urlPath, cliutils.OrgAndCreds(org, userPw), []int{200}, &resp)
					mmsObjectInfo.ObjectStatus = string(resp)

					//3. write the response data to channel c <- mmsObjectInfo
					c <- mmsObjectInfo
				}(obj)

			}

			for range batches[i] {
				select {
				case mmsObjectInfo := <-c:
					if long {
						mmsObjectInfo.ObjectID = ""
						mmsObjectInfo.ObjectType = ""
					} else {
						mmsObjectInfo.Definition = nil
					}
					mmsObjects = append(mmsObjects, mmsObjectInfo)
				}
			}
		}

		output = cliutils.MarshalIndent(mmsObjects, "mms object list")
	} else {
		if !long {
			mmsObjects := make([]MMSObjectInfo, 0)
			for _, obj := range objectsMeta {
				mmsObjectInfo := MMSObjectInfo{
					ObjectID:   obj.ObjectID,
					ObjectType: obj.ObjectType,
				}
				mmsObjects = append(mmsObjects, mmsObjectInfo)
			}
			output = cliutils.MarshalIndent(mmsObjects, "mms object list")
		} else {
			var err1 error
			output, err1 = cliutils.DisplayAsJson(objectsMeta)
			if err1 != nil {
				cliutils.Fatal(cliutils.JSON_PARSING_ERROR, i18n.GetMessagePrinter().Sprintf("failed to marshal 'hzn mms object list' output: %v", err1))
			}
		}
	}

	msgPrinter.Printf("Listing objects in org %v:", org)
	msgPrinter.Println()
	fmt.Println(output)
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
		`                             /* ` + msgPrinter.Sprintf("If omitted the object is sent to all nodes with the same destinationType.") + ` */`,
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
		`        "value": null,`,
		`        "type": ""           /* ` + msgPrinter.Sprintf("Valid types are string, bool, int, float, list of strings (comma separated), version.") + ` */`,
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
	for _, s := range hzn_object_metadata {
		fmt.Println(s)
	}

}

// Upload an object to the MMS. The user can provide a copy of the object's metadata in a file, or they can simply provide
// object id and type.
func ObjectPublish(org string, userPw string, objType string, objId string, objPattern string, objMetadataFile string, objFile string, skipDigitalSig bool, dsHashAlgo string, dsHash string, privKeyFilePath string) {
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
	} else if skipDigitalSig && dsHash != "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("cannot specify --skipDigitalSig with --hash"))
	} else if skipDigitalSig && dsHashAlgo != "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("cannot specify --skipDigitalSig with --hashAlgo"))
	} else if dsHashAlgo != "" && dsHashAlgo != common.Sha1 && dsHashAlgo != common.Sha256 {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("invalid value for --hashAlgo, please use SHA1 or SHA256"))
	}

	// If we were given a full metadata file, read it in and use it to create the object. Otherwise, construct a minimal
	// object metadata file based on the other input paramaters.
	var objectMeta common.MetaData
	if objMetadataFile != "" {
		metaBytes := cliconfig.ReadJsonFileWithLocalConfig(objMetadataFile)
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
	// that the MMS (CSS) correctly interpets the PUT.
	if objFile == "" {
		objectMeta.MetaOnly = true
	} else if !skipDigitalSig {

		hashAlgorithm := common.Sha1
		if dsHashAlgo == common.Sha256 {
			hashAlgorithm = common.Sha256
		}

		msgPrinter.Printf("Digital sign with %s will be performed for data integrity. It will delay the MMS object publish.\n", hashAlgorithm)

		// Create public key. Sign data. Set "hashAlgorithm", "publicKey" and "signature" field
		if publicKey, signature, err := signObjData(objFile, hashAlgorithm, dsHash, privKeyFilePath); err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("failed to digital sign the file %v, Error: %v", objFile, err))
		} else {
			objectMeta.HashAlgorithm = hashAlgorithm
			objectMeta.PublicKey = publicKey
			objectMeta.Signature = signature
		}

		msgPrinter.Printf("Digital sign finished.")
		msgPrinter.Println()
	}

	type ObjectWrapper struct {
		Meta common.MetaData `json:"meta"`
		Data []byte          `json:"data"`
	}

	wrapper := ObjectWrapper{Meta: objectMeta}

	// Call the MMS service over HTTP to add the object's metadata to the MMS.
	urlPath := path.Join("api/v1/objects/", org, objectMeta.ObjectType, objectMeta.ObjectID)
	cliutils.ExchangePutPost("Model Management Service", http.MethodPut, cliutils.GetMMSUrl(), urlPath, cliutils.OrgAndCreds(org, userPw), []int{204}, wrapper, nil)

	// The object's data might be quite large, so upload it in a second call that will stream the file contents
	// to the MSS (CSS).
	if objFile != "" {
		file, err := os.Open(objFile)
		if err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("unable to open object file %v: %v", objFile, err))
		}
		defer file.Close()

		// Establish the HTTP request override because the upload could take some time.
		setHTTPOverride := false
		if os.Getenv(config.HTTPRequestTimeoutOverride) == "" {
			setHTTPOverride = true
			os.Setenv(config.HTTPRequestTimeoutOverride, "0")
		}

		// Stream the file to the MMS (CSS).
		urlPath = path.Join("api/v1/objects/", org, objectMeta.ObjectType, objectMeta.ObjectID, "data")
		cliutils.ExchangePutPost("Model Management Service", http.MethodPut, cliutils.GetMMSUrl(), urlPath, cliutils.OrgAndCreds(org, userPw), []int{204}, file, nil)

		// Restore HTTP request override if necessary.
		if setHTTPOverride {
			os.Setenv(config.HTTPRequestTimeoutOverride, "")
		}

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

// Delete an object in the MMS.
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

func dpServiceIsValid(dpService string) bool {
	trimString := strings.TrimSpace(dpService)
	if trimString != "" {
		parts := strings.Split(trimString, "/")
		if len(parts) != 2 {
			return false
		}
	}
	return true
}

func timeIsValid(timestamp string) (bool, string) {
	trimString := strings.TrimSpace(timestamp)
	if trimString != "" {
		_, err := time.Parse(time.RFC3339, trimString)
		if err != nil {
			regex := *regexp.MustCompile(`(\d{4})-(\d{2})-(\d{2})`)
			if res := regex.FindStringSubmatch(trimString); res != nil {
				convTimeStamp := fmt.Sprintf("%sT00:00:00Z", res[0])
				return true, convTimeStamp
			}

			return false, ""
		}
	}
	return true, ""
}

func signObjData(objFile string, dsHashAlgo string, dsHash string, privKeyFilePath string) (string, string, error) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	var fileHash hash.Hash
	var fileHashSum []byte
	var privateKey *rsa.PrivateKey
	var err error

	if dsHash != "" {
		//parse dsHash
		if fileHashSum, err = hex.DecodeString(dsHash); err != nil {
			return "", "", err
		}
		msgPrinter.Printf("Hash value is loaded.")
		msgPrinter.Println()
	} else {
		file, err := os.Open(objFile)
		if err != nil {
			return "", "", err
		}
		defer file.Close()

		msgPrinter.Printf("Start hashing the file...")
		msgPrinter.Println()

		if fileHash, err = GetHash(dsHashAlgo); err != nil {
			return "", "", err
		} else if _, err = io.Copy(fileHash, file); err != nil {
			return "", "", err
		}
		fileHashSum = fileHash.Sum(nil)

		msgPrinter.Printf("Data hash is generated. Start digital signing with the data hash...")
		msgPrinter.Println()
	}

	// use given key pair, if given, otherwise try to fetch default key file
	privKeyFilePath_tmp := cliutils.WithDefaultEnvVar(&privKeyFilePath, "HZN_PRIVATE_KEY_FILE")
	privKeyFilePath = cliutils.WithDefaultKeyFile(*privKeyFilePath_tmp, false)
	if privKeyFilePath != "" {
		if privateKey, err = sign.ReadPrivateKey(privKeyFilePath); err != nil {
			return "", "", err
		}
		// if there is no given private key or defualt value, generate private
		// and public key pair
	} else if privateKey, err = rsa.GenerateKey(rand.Reader, 2048); err != nil {
		return "", "", err
	}

	if publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey); err != nil {
		return "", "", err
	} else if cryptoHash, err := GetCryptoHashType(dsHashAlgo); err != nil {
		return "", "", err
	} else if signature, err := rsa.SignPSS(rand.Reader, privateKey, cryptoHash, fileHashSum, nil); err != nil {
		return "", "", err
	} else {
		publicKeyString := base64.StdEncoding.EncodeToString(publicKeyBytes)
		signatureString := base64.StdEncoding.EncodeToString(signature)

		return publicKeyString, signatureString, nil
	}
}

func GetHash(hashAlgo string) (hash.Hash, error) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if hashAlgo == common.Sha1 {
		return sha1.New(), nil
	} else if hashAlgo == common.Sha256 {
		return sha256.New(), nil
	} else {
		return nil, errors.New(msgPrinter.Sprintf("Hash algorithm %s is not supported", hashAlgo))
	}

}

func GetCryptoHashType(hashAlgo string) (crypto.Hash, error) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if hashAlgo == common.Sha1 {
		return crypto.SHA1, nil
	} else if hashAlgo == common.Sha256 {
		return crypto.SHA256, nil
	} else {
		return 0, errors.New(msgPrinter.Sprintf("Hash algorithm %s is not supported", hashAlgo))
	}
}
