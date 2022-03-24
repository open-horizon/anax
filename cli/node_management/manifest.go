package node_management

import (
	"fmt"
	"github.com/open-horizon/anax/cli/cliconfig"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/semanticversion"
	"github.com/open-horizon/edge-sync-service/common"
	"io/ioutil"
	"net/http"
	"path"
)

type ManifestInfo struct {
	ManifestID   string `json:"manifestID"`
	ManifestType string `json:"manifestType"`
}

type AgentUpgradeManifestData struct {
	SoftwareUpgrade      ManifestUpgradeDef `json:"softwareUpgrade,omitempty"`
	CertificateUpgrade   ManifestUpgradeDef `json:"certificateUpgrade,omitempty"`
	ConfigurationUpgrade ManifestUpgradeDef `json:"configurationUpgrade,omitempty"`
}

type validManifestTypes []string

type ManifestUpgradeDef struct {
	Version string   `json:"version"`
	Files   []string `json:"files"`
}

var (
	// Right now, there are only agent upgrade manifests, but there may be more types in future
	// which should be added to this list
	validManTypes = validManifestTypes{"agent_upgrade_manifests"}
)

func (m validManifestTypes) contains(element string) bool {
	for _, t := range m {
		if t == element {
			return true
		}
	}
	return false
}

func (m validManifestTypes) string() string {
	str := ""
	for _, t := range m {
		str += fmt.Sprintf("%v, ", t)
	}
	return str[:len(str)-2]
}

func ManifestList(org, credToUse, manifestId, manifestType string, longDetails bool) {
	cliutils.SetWhetherUsingApiKey(credToUse)
	var manOrg string
	manOrg, manifestId = cliutils.TrimOrg(org, manifestId)

	if manifestId == "*" {
		manifestId = ""
	}

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Ensure that specified type, if any, is a valid type
	if manifestType != "" && !validManTypes.contains(manifestType) {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Invalid manifest type specified. Valid types include: %v", validManTypes.string()))
	}
	// Ensure that if the user gives a manifest ID, that they also gave a manifest type
	if manifestId != "" && manifestType == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("must specify --type with --id"))
	}
	// Ensure that the user specified a type and id if they want the contents of the manifest
	if longDetails && (manifestId == "" || manifestType == "") {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("must specify --type and --id with --long"))
	}

	// Assemble URL
	filterURLPath := ""
	if manifestType != "" {
		filterURLPath += fmt.Sprintf("&objectType=%s", manifestType)
	}
	if manifestId != "" {
		filterURLPath += fmt.Sprintf("&objectID=%s", manifestId)
	}
	urlPath := "api/v1/objects/" + manOrg + "?filters=true"
	fullPath := urlPath + filterURLPath

	// Call the MMS service over HTTP to get the manifest metadata.
	var manifestsMeta []common.MetaData
	httpCode := cliutils.ExchangeGet("Model Management Service", cliutils.GetMMSUrl(), fullPath, cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &manifestsMeta)
	if httpCode == 404 || len(manifestsMeta) == 0 {
		fmt.Println("[]")
		return
	}

	var output string

	// If the user did not request manifest details (--long), return list of manifest ID's with thier type's
	if !longDetails {
		manifestObjects := make([]ManifestInfo, 0)
		for _, manifest := range manifestsMeta {
			if validManTypes.contains(manifest.ObjectType) {
				manifestInfo := ManifestInfo{
					ManifestID:   manOrg + "/" + manifest.ObjectID,
					ManifestType: manifest.ObjectType,
				}
				manifestObjects = append(manifestObjects, manifestInfo)
			}
		}
		output = cliutils.MarshalIndent(manifestObjects, "nodemanagement manifest list")

		// Otherwise get the contents of the specified manifest file
	} else {

		// The manifest metadata will be the only entry in the list since we force the user
		// to specify the type and ID of the manifest
		manifest := manifestsMeta[0]

		// Assemble URL
		urlPath := path.Join("api/v1/objects/", manOrg, manifest.ObjectType, manifest.ObjectID, "/data")
		apiMsg := http.MethodGet + " " + cliutils.GetMMSUrl() + urlPath

		// Call the MMS service over HTTP to get the manifest data.
		resp := cliutils.ExchangeGetResponse("Model Management Service", cliutils.GetMMSUrl(), urlPath, cliutils.OrgAndCreds(org, credToUse))
		if resp.Body != nil {
			defer resp.Body.Close()
		}

		// Read in the manifest data bytes
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			cliutils.Fatal(cliutils.HTTP_ERROR, msgPrinter.Sprintf("failed to read body response from %s: %v", apiMsg, err))
		}

		// Unmarshal the manifest into the correct struct type before marshaling back
		// to ensure consistent output. As more manifest types are added, these lines
		// will need to be repeated with the corresponding structs.
		if manifest.ObjectType == "agent_upgrade_manifests" {
			var manifestData AgentUpgradeManifestData
			cliutils.Unmarshal(bodyBytes, &manifestData, "nodemanagement manifest list")
			output = cliutils.MarshalIndent(manifestData, "nodemanagement manifest list")
		}
	}

	fmt.Println(output)
}

func ManifestAdd(org, credToUse, manifestFile, manifestId, manifestType string) {
	cliutils.SetWhetherUsingApiKey(credToUse)
	var manOrg string
	manOrg, manifestId = cliutils.TrimOrg(org, manifestId)

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Ensure that specified type, if any, is a valid type
	if manifestType != "" && !validManTypes.contains(manifestType) {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Invalid manifest type specified. Valid types include: %v", validManTypes.string()))
	}

	// Create the metadata file used by manifests in the MMS
	var manifestsMeta common.MetaData
	manifestsMeta.ObjectID = manifestId
	manifestsMeta.ObjectType = manifestType

	// Read in the file from the host
	var manifestData AgentUpgradeManifestData
	manifestBytes := cliconfig.ReadJsonFileWithLocalConfig(manifestFile)
	cliutils.Unmarshal(manifestBytes, &manifestData, "nodemanagement manifest add")

	checkManifestFile(manOrg, credToUse, manifestData)

	// Call the MMS service over HTTP to see if manifest exists.
	updatedManifest := false
	filterURLPath := fmt.Sprintf("&objectType=%s&objectID=%s", manifestsMeta.ObjectType, manifestsMeta.ObjectID)
	urlPath := "api/v1/objects/" + manOrg + "?filters=true"
	fullPath := urlPath + filterURLPath
	var dummyManifestsMeta []common.MetaData
	httpCode := cliutils.ExchangeGet("Model Management Service", cliutils.GetMMSUrl(), fullPath, cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &dummyManifestsMeta)
	if httpCode != 404 {
		updatedManifest = true
	}

	// TODO - remove when CSS implements ACL
	manifestsMeta.Public = true

	// Create an object wrapper to use as the input body to the PUT request
	type ObjectWrapper struct {
		Meta common.MetaData `json:"meta"`
		Data []byte          `json:"data"`
	}
	wrapper := ObjectWrapper{Meta: manifestsMeta}

	// Call the MMS service over HTTP to add the manifest's metadata to the MMS.
	urlPath = path.Join("api/v1/objects/", manOrg, manifestsMeta.ObjectType, manifestsMeta.ObjectID)
	cliutils.ExchangePutPost("Model Management Service", http.MethodPut, cliutils.GetMMSUrl(), urlPath, cliutils.OrgAndCreds(org, credToUse), []int{204}, wrapper, nil)

	// Call the MMS service over HTTP to add the manifest's data to the MMS.
	urlPath = path.Join("api/v1/objects/", manOrg, manifestsMeta.ObjectType, manifestsMeta.ObjectID, "data")
	cliutils.ExchangePutPost("Model Management Service", http.MethodPut, cliutils.GetMMSUrl(), urlPath, cliutils.OrgAndCreds(org, credToUse), []int{204}, manifestData, nil)

	if updatedManifest {
		msgPrinter.Printf("Manifest %v/%v updated in the Management Hub", manOrg, manifestsMeta.ObjectID)
	} else {
		msgPrinter.Printf("Manifest %v/%v added to the Management Hub", manOrg, manifestsMeta.ObjectID)
	}
	msgPrinter.Println()
}

func checkManifestFile(org, credToUse string, manifestData AgentUpgradeManifestData) {

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	validFile := true
	errMsg := msgPrinter.Sprintf("The following files were specified in the manifest file but do not exist in the Management Hub:")
	errMsg += msgPrinter.Sprintln()

	// Check software files list and version, if files were specified
	manSoftwareFiles := manifestData.SoftwareUpgrade.Files
	if len(manSoftwareFiles) > 0 {
		var manSoftwareFilesVersion string
		if manifestData.SoftwareUpgrade.Version == "latest" {
			manSoftwareFilesVersion = ""
		} else if isValidVersion := semanticversion.IsVersionString(manifestData.SoftwareUpgrade.Version); !isValidVersion {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("The version specified in SoftwareUpgrade is not a valid version string or \"latest\": %v", manifestData.SoftwareUpgrade.Version))
		} else {
			manSoftwareFilesVersion = manifestData.SoftwareUpgrade.Version
		}
		mmsSoftwareFiles := getAgentFiles(org, credToUse, "agent_software_files", manSoftwareFilesVersion)
		for _, manFile := range manSoftwareFiles {
			found := false
			for _, mmsFile := range mmsSoftwareFiles {
				if mmsFile.AgentFileName == manFile {
					found = true
					break
				}
			}
			if !found {
				validFile = false
				errMsg += msgPrinter.Sprintf("File \"%s\" version \"%s\" of type \"agent_software_files\".", manFile, manifestData.SoftwareUpgrade.Version)
				errMsg += msgPrinter.Sprintln()
			}
		}
	}

	// Check certificate files list and version, if files were specified
	manCertFiles := manifestData.CertificateUpgrade.Files
	if len(manCertFiles) > 0 {
		var manCertFilesVersion string
		if manifestData.CertificateUpgrade.Version == "latest" {
			manCertFilesVersion = ""
		} else if isValidVersion := semanticversion.IsVersionString(manifestData.CertificateUpgrade.Version); !isValidVersion {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("The version specified in CertificateUpgrade is not a valid version string or \"latest\": %v", manifestData.CertificateUpgrade.Version))
		} else {
			manCertFilesVersion = manifestData.CertificateUpgrade.Version
		}
		mmsCertFiles := getAgentFiles(org, credToUse, "agent_cert_files", manCertFilesVersion)
		for _, manFile := range manCertFiles {
			found := false
			for _, mmsFile := range mmsCertFiles {
				if mmsFile.AgentFileName == manFile {
					found = true
					break
				}
			}
			if !found {
				validFile = false
				errMsg += msgPrinter.Sprintf("File \"%s\" version \"%s\" of type \"agent_cert_files\".", manFile, manifestData.CertificateUpgrade.Version)
				errMsg += msgPrinter.Sprintln()
			}
		}
	}

	// Check config files list and version, if files were specified
	manConfigFiles := manifestData.ConfigurationUpgrade.Files
	if len(manConfigFiles) > 0 {
		var manConfigFilesVersion string
		if manifestData.ConfigurationUpgrade.Version == "latest" {
			manConfigFilesVersion = ""
		} else if isValidVersion := semanticversion.IsVersionString(manifestData.ConfigurationUpgrade.Version); !isValidVersion {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("The version specified in ConfigurationUpgrade is not a valid version string or \"latest\": %v", manifestData.ConfigurationUpgrade.Version))
		} else {
			manConfigFilesVersion = manifestData.ConfigurationUpgrade.Version
		}
		mmsConfigFiles := getAgentFiles(org, credToUse, "agent_config_files", manConfigFilesVersion)
		for _, manFile := range manConfigFiles {
			found := false
			for _, mmsFile := range mmsConfigFiles {
				if mmsFile.AgentFileName == manFile {
					found = true
					break
				}
			}
			if !found {
				validFile = false
				errMsg += msgPrinter.Sprintf("File \"%s\" version \"%s\" of type \"agent_config_files\".", manFile, manifestData.ConfigurationUpgrade.Version)
				errMsg += msgPrinter.Sprintln()
			}
		}
	}

	// Throw error if any of the sections had an incorrect entry
	if !validFile {
		errMsg += msgPrinter.Sprintf("Run 'hzn nodemanagement agentfiles list' to get a list of valid files.")
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, errMsg)
	}
}

func ManifestRemove(org, credToUse, manifestId, manifestType string, force bool) {
	cliutils.SetWhetherUsingApiKey(credToUse)
	var manOrg string
	manOrg, manifestId = cliutils.TrimOrg(org, manifestId)

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Ensure that specified type, if any, is a valid type
	if manifestType != "" && !validManTypes.contains(manifestType) {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Invalid manifest type specified. Valid types include: %v", validManTypes.string()))
	}

	// Check to make sure user wants to remove the specified manifest
	if !force {
		cliutils.ConfirmRemove(msgPrinter.Sprintf("Are you sure you want to remove manifest %v/%v from the Management Hub?", manOrg, manifestId))
	}

	// Set the API key env var if that's what we're using.
	cliutils.SetWhetherUsingApiKey(credToUse)

	// Call the MMS service over HTTP to delete the object.
	urlPath := path.Join("api/v1/objects/", manOrg, manifestType, manifestId)
	httpCode := cliutils.ExchangeDelete("Model Management Service", cliutils.GetMMSUrl(), urlPath, cliutils.OrgAndCreds(org, credToUse), []int{204, 400, 404})
	if httpCode != 204 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("Manifest '%s/%s' of type '%s' not found in the Management Hub", manOrg, manifestId, manifestType))
	}

	msgPrinter.Printf("Manifest %v/%v deleted from the Management Hub", manOrg, manifestId)
	msgPrinter.Println()
}

func ManifestNew() {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	var business_policy_template = []string{
		`{`,
		`  "softwareUpgrade": {       /* ` + msgPrinter.Sprintf("Fill in this section to perform a software upgrade of the agent. Remove this section to prevent software upgrade.") + ` */`,
		`    "files": [               /* ` + msgPrinter.Sprintf("A list of agent software files stored in the Management Hub.") + ` */`,
		`      ""                     /* ` + msgPrinter.Sprintf("Run 'hzn nm agentfiles list -t agent_software_files' to get a list of available files.") + ` */`,
		`    ],`,
		`    "version": ""            /* ` + msgPrinter.Sprintf("The agent software version this manifest applies to. Specify \"latest\" to get the most recent version.") + ` */`,
		`  },`,
		`  "certificateUpgrade": {    /* ` + msgPrinter.Sprintf("Fill in this section to upgrade the agent certificate. Remove this section to prevent certificate upgrade.") + ` */`,
		`    "files": [               /* ` + msgPrinter.Sprintf("The name of a cert file stored in the Management Hub. Default is \"agent-install.crt\".") + ` */`,
		`      "agent-install.crt"    /* ` + msgPrinter.Sprintf("Run 'hzn nm agentfiles list -t agent_cert_files' to get a list of available files.") + ` */`,
		`    ],`,
		`    "version": ""            /* ` + msgPrinter.Sprintf("The agent cert version this manifest applies to. Specify \"latest\" to get the most recent version.") + ` */`,
		`  },`,
		`  "configurationUpgrade": {  /* ` + msgPrinter.Sprintf("Fill in this section to upgrade the agent config. Remove this section to prevent config upgrade.") + ` */`,
		`    "files": [               /* ` + msgPrinter.Sprintf("The name of a cert file stored in the Management Hub. Default is \"agent-install.crt\".") + ` */`,
		`      "agent-install.cfg"    /* ` + msgPrinter.Sprintf("Run 'hzn nm agentfiles list -t agent_config_files' to get a list of available files.") + ` */`,
		`    ],`,
		`    "version": ""            /* ` + msgPrinter.Sprintf("The agent config version this manifest applies to. Specify \"latest\" to get the most recent version.") + ` */`,
		`  }`,
		`}`,
	}

	for _, s := range business_policy_template {
		fmt.Println(s)
	}
}
