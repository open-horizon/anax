package node_management

import (
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/semanticversion"
	"github.com/open-horizon/edge-sync-service/common"
	"sort"
	"strings"
)

type agentFileInfo struct {
	AgentFileName    string `json:"fileName"`
	AgentFileType    string `json:"fileType"`
	AgentFileVersion string `json:"fileVersion"`
}

type agentFileType struct {
	AgentFileType    string `json:"fileType"`
	AgentFileVersion string `json:"version"`
}

type validAgentFileTypes []string

var (
	// Right now, there are only agent upgrade types, but there may be more types in future
	// which should be added to this list
	validFileTypes = validAgentFileTypes{"agent_software_files", "agent_cert_files", "agent_config_files"}
)

func (a validAgentFileTypes) contains(element string) bool {
	for _, t := range a {
		if t == element {
			return true
		}
	}
	return false
}

func (a validAgentFileTypes) string() string {
	str := ""
	for _, t := range a {
		str += fmt.Sprintf("%v, ", t)
	}
	return str[:len(str)-2]
}

func AgentFilesList(org, credToUse, fileTypeFilter, fileVersionFilter string) {

	var agentFileObjects []agentFileInfo
	agentFileObjects = getAgentFiles(org, credToUse, fileTypeFilter, fileVersionFilter)

	// Output the list of agent files
	var output string
	if len(agentFileObjects) > 0 {
		output = cliutils.MarshalIndent(agentFileObjects, "nodemanagement agentfiles list")

		// Return an empty list if there were no files with the specified features
	} else {
		output = "[]"
	}

	fmt.Println(output)
}

func getAgentFiles(org, credToUse, fileTypeFilter, fileVersionFilter string) []agentFileInfo {

	cliutils.SetWhetherUsingApiKey(credToUse)

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Ensure that specified type, if any, is a valid type
	if fileTypeFilter != "" && !validFileTypes.contains(fileTypeFilter) {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Invalid agent file type specified. Valid types include: %v", validFileTypes.string()))
	}

	// Ensure that specified version, if any, is a valid semantic version string, valid version range or "latest"
	var err error
	usingVersionRange := false
	fileVersionFilter = strings.TrimSpace(strings.ReplaceAll(fileVersionFilter, " ", ""))
	var fileVersionFilterRange *semanticversion.Version_Expression
	if !semanticversion.IsVersionString(fileVersionFilter) {
		if fileVersionFilterRange, err = semanticversion.Version_Expression_Factory(fileVersionFilter); err == nil {
			usingVersionRange = true
		} else if fileVersionFilter != "latest" && fileVersionFilter != "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("--version must specify a valid version range, a valid version string or \"latest\""))
		}
	}

	// Assemble URL
	urlPath := "api/v1/objects/IBM?filters=true"

	// Slice to store agent files
	agentFileObjects := make([]agentFileInfo, 0)

	// Call the MMS service over HTTP to get the manifest metadata.
	var agentFileMeta []common.MetaData
	httpCode := cliutils.ExchangeGet("Model Management Service", cliutils.GetMMSUrl(), urlPath, cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &agentFileMeta)
	if httpCode == 404 {
		return agentFileObjects
	}

	// If no manifestID was specified, return list of manifest ID's with thier type's
	for _, agentFile := range agentFileMeta {

		// Determine if there is an underscore signifying a version string (can't be first or last character)
		agentFileType := agentFile.ObjectType
		splitIdx := strings.Index(agentFileType, "-")
		if splitIdx > 0 && splitIdx < len(agentFileType)-1 {

			// If there is a version, separate type from version, check the version string,
			// make sure the type is in the list of valid types, filter for the type specified
			// by --type and the version specified by --version, if applicable. If everything
			// checks out, add to list.
			fileVersion := agentFileType[splitIdx+1:]
			fileType := agentFileType[:splitIdx]
			if !semanticversion.IsVersionString(fileVersion) {
				continue
			}
			if !validFileTypes.contains(fileType) {
				continue
			}
			isWithinRange := true
			if usingVersionRange {
				if isWithinRange, err = fileVersionFilterRange.Is_within_range(fileVersion); err != nil {
					cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("failed to check version range %s: %v", fileVersion, err))
				}
			} else if fileVersionFilter != "" && fileVersionFilter != "latest" && fileVersion != fileVersionFilter {
				isWithinRange = false
			}
			if isWithinRange {
				if fileTypeFilter == fileType || fileTypeFilter == "" {
					agentFileInfo := agentFileInfo{
						AgentFileName:    agentFile.ObjectID,
						AgentFileType:    fileType,
						AgentFileVersion: fileVersion,
					}
					agentFileObjects = append(agentFileObjects, agentFileInfo)
				}
			}
		}
	}

	// Sort the files by type (if the type was not filtered) and then by version in descending order
	sort.Slice(agentFileObjects, func(i, j int) bool {
		if agentFileObjects[i].AgentFileType == agentFileObjects[j].AgentFileType {
			if greaterThan, err := semanticversion.CompareVersions(agentFileObjects[i].AgentFileVersion, agentFileObjects[j].AgentFileVersion); err != nil {
				cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("error comparing agent file versions: %v", err))
			} else {
				return greaterThan > 0
			}
		}
		return agentFileObjects[i].AgentFileType > agentFileObjects[j].AgentFileType
	})

	// If the user just wants the latest files, grab the first instance of each type in the list
	// since it is already sorted
	if fileVersionFilter == "latest" {
		latestAgentFileObjects := make([]agentFileInfo, 0)
		for _, validType := range validFileTypes {
			for _, agentFile := range agentFileObjects {
				if agentFile.AgentFileType == validType {
					latestAgentFileObjects = append(latestAgentFileObjects, agentFile)
					break
				}
			}
		}
		agentFileObjects = latestAgentFileObjects
	}

	return agentFileObjects
}

func AgentFilesVersions(org, credToUse, fileTypeFilter string, versionOnly bool) {

	cliutils.SetWhetherUsingApiKey(credToUse)

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Ensure that specified type, if any, is a valid type
	if fileTypeFilter != "" && !validFileTypes.contains(fileTypeFilter) {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Invalid agent file type specified. Valid types include: %v", validFileTypes.string()))
	}

	// Ensure that a type was specified if the user wants only a versions list
	if versionOnly && fileTypeFilter == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("must specify --type with --version-only"))
	}

	// Call the MMS service over HTTP to get the object types.
	var types []string
	urlPath := fmt.Sprintf("api/v1/objects/IBM?list_object_type=true")
	httpCode := cliutils.ExchangeGet("Model Management Service", cliutils.GetMMSUrl(), urlPath, cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &types)
	if httpCode == 404 {
		fmt.Println("[]")
		return
	}

	// Check each type returned from the CSS type API and add to list, if valid
	var agentFileTypes []agentFileType
	for _, t := range types {

		// Determine if there is an underscore signifying a version string
		splitIdx := strings.Index(t, "-")
		if splitIdx > 0 && splitIdx < len(t)-1 {

			// If there is a version, separate type from version, check the version string,
			// make sure the type is in the list of valid types, and filter for the type specified
			// by --type, if applicable. If everything checks out, add to list.
			fileVersion := t[splitIdx+1:]
			fileType := t[:splitIdx]
			if !semanticversion.IsVersionString(fileVersion) {
				continue
			}
			if !validFileTypes.contains(fileType) {
				continue
			}
			if semanticversion.IsVersionString(fileVersion) && validFileTypes.contains(fileType) {
				if fileTypeFilter == fileType || fileTypeFilter == "" {
					agentFileType := agentFileType{
						AgentFileType:    fileType,
						AgentFileVersion: fileVersion,
					}
					agentFileTypes = append(agentFileTypes, agentFileType)
				}
			}
		}
	}

	// Sort the files by type (if the type was not filtered) and then by version in descending order
	sort.Slice(agentFileTypes, func(i, j int) bool {
		if agentFileTypes[i].AgentFileType == agentFileTypes[j].AgentFileType {
			return agentFileTypes[i].AgentFileVersion > agentFileTypes[j].AgentFileVersion
		}
		return agentFileTypes[i].AgentFileType > agentFileTypes[j].AgentFileType
	})

	// If the user specified --version-only, return a list of versions extracted from the
	// list of agent types
	var output string
	if versionOnly {
		var versions []string
		for _, v := range agentFileTypes {
			versions = append(versions, v.AgentFileVersion)
		}
		output = cliutils.MarshalIndent(versions, "nodemanagement agentfiles versions")

		// Otherwise, output the list of agent types
	} else if len(agentFileTypes) > 0 {
		output = cliutils.MarshalIndent(agentFileTypes, "nodemanagement agentfiles versions")

		// Return an empty list if there were no files with the specified features
	} else {
		output = "[]"
	}

	fmt.Println(output)
}
