package node_management

import (
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/semanticversion"
	"github.com/open-horizon/edge-sync-service/common"
	"strings"
)

type agentFileInfo struct {
	AgentFileName    string `json:"fileName"`
	AgentFileType    string `json:"fileType"`
	AgentFileVersion string `json:"fileVersion"`
}

type validAgentFileTypes []string

type agentFileType struct {
	FileType string `json:"fileType"`
	Version  string `json:"version"`
}

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
	cliutils.SetWhetherUsingApiKey(credToUse)

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Right now, there are only agent upgrade types, but there may be more types in future
	// which should be added to this list
	validAgentFileTypes := validAgentFileTypes{"agent-software-files", "agent-cert-files", "agent-config-files"}

	// Ensure that specified type, if any, is a valid type
	if fileTypeFilter != "" && !validAgentFileTypes.contains(fileTypeFilter) {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Invalid agent file type specified. Valid types include: %v", validAgentFileTypes.string()))
	}
	// Ensure that specified version, if any, is a valid semantic version string, valid version range or "latest"
	var err error
	usingVersionRange := false
	fileVersionFilter = strings.TrimSpace(strings.ReplaceAll(fileVersionFilter, " ", ""))
	var fileVersionFilterRange *semanticversion.Version_Expression
	if fileVersionFilterRange, err = semanticversion.Version_Expression_Factory(fileVersionFilter); err == nil {
		usingVersionRange = true
	} else if !semanticversion.IsVersionString(fileVersionFilter) && fileVersionFilter != "latest" && fileVersionFilter != "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("--version must specify a valid version range, a valid version string or \"latest\""))
	}

	// Assemble URL
	urlPath := "api/v1/objects/IBM?filters=true"

	// Call the MMS service over HTTP to get the manifest metadata.
	var agentFileMeta []common.MetaData
	httpCode := cliutils.ExchangeGet("Model Management Service", cliutils.GetMMSUrl(), urlPath, cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &agentFileMeta)
	if httpCode == 404 {
		fmt.Println("[]")
		return
	}

	// If no manifestID was specified, return list of manifest ID's with thier type's
	agentFileObjects := make([]agentFileInfo, 0)
	for _, agentFile := range agentFileMeta {

		// Determine if there is an underscore signifying a version string (can't be last character)
		agentFileType := agentFile.ObjectType
		splitIdx := strings.LastIndex(agentFileType, "_")
		if splitIdx > 0 && splitIdx < len(agentFileType)-1 {

			// If there is a version, separate type from version, check the version string,
			// make sure the type is in the list of valid types, filter for the type specified
			// by --type and the version specified by --version, if applicable. If everything
			// checks out, add to list.
			fileVersion := agentFileType[splitIdx+1:]
			fileType := agentFileType[:splitIdx]
			isWithinRange := true
			if usingVersionRange {
				if isWithinRange, err = fileVersionFilterRange.Is_within_range(fileVersion); err != nil {

				}
			} else if !semanticversion.IsVersionString(fileVersion) || (fileVersionFilter != "" && fileVersion != fileVersionFilter) {
				isWithinRange = false
			}
			if isWithinRange && validAgentFileTypes.contains(fileType) {
				if fileTypeFilter == fileType || fileTypeFilter == "" {
					agentFileInfo := agentFileInfo{
						AgentFileName:    agentFile.ObjectID,
						AgentFileType:    fileType,
						AgentFileVersion: fileVersion,
					}
					agentFileObjects = append(agentFileObjects, agentFileInfo)
				}
			}

			// If there was no underscore, check to see if type is valid. If so, the lack of
			// version signifies this type is the latest version.
		} else if validAgentFileTypes.contains(agentFileType) {
			if (fileTypeFilter == agentFileType || fileTypeFilter == "") && (fileVersionFilter == "latest" || fileVersionFilter == "") {
				agentFileInfo := agentFileInfo{
					AgentFileName:    agentFile.ObjectID,
					AgentFileType:    agentFile.ObjectType,
					AgentFileVersion: "latest",
				}
				agentFileObjects = append(agentFileObjects, agentFileInfo)
			}
		}
	}

	// Output the list of agent files, if it isn't empty
	var output string
	if len(agentFileObjects) > 0 {
		output = cliutils.MarshalIndent(agentFileObjects, "nodemanagement agentfiles list")

		// Finally, return an empty list if there were no files with the specified features
	} else {
		output = "[]"
	}
	msgPrinter.Println("The highest version and \"latest\" refer to the same object.")
	fmt.Println(output)
}

func AgentFilesVersions(org, credToUse, fileTypeFilter string, versionOnly bool) {

	cliutils.SetWhetherUsingApiKey(credToUse)

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Right now, there are only agent upgrade types, but there may be more types in future
	// which should be added to this list
	validAgentFileTypes := validAgentFileTypes{"agent-software-files", "agent-cert-files", "agent-config-files"}

	// Ensure that specified type, if any, is a valid type
	if fileTypeFilter != "" && !validAgentFileTypes.contains(fileTypeFilter) {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Invalid agent file type specified. Valid types include: %v", validAgentFileTypes.string()))
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
	var agentTypes []agentFileType
	for _, t := range types {

		// Determine if there is an underscore signifying a version string
		splitIdx := strings.LastIndex(t, "_")
		if splitIdx > 0 && splitIdx < len(t)-1 {

			// If there is a version, separate type from version, check the version string,
			// make sure the type is in the list of valid types, and filter for the type specified
			// by --type, if applicable. If everything checks out, add to list.
			version := t[splitIdx+1:]
			fileType := t[:splitIdx]
			if semanticversion.IsVersionString(version) && validAgentFileTypes.contains(fileType) {
				if fileTypeFilter == fileType || fileTypeFilter == "" {
					agentFileType := agentFileType{
						FileType: fileType,
						Version:  version,
					}
					agentTypes = append(agentTypes, agentFileType)
				}
			}

			// If there was no underscore, check to see if type is valid. If so, the lack of
			// version signifies this type is the latest version.
		} else if validAgentFileTypes.contains(t) && (fileTypeFilter == t || fileTypeFilter == "") {
			agentFileType := agentFileType{
				FileType: t,
				Version:  "latest",
			}
			agentTypes = append(agentTypes, agentFileType)
		}
	}

	// If the user specified --version-only, return a list of versions extracted from the
	// list of agent types
	var output string
	if versionOnly {
		var versions []string
		for _, v := range agentTypes {
			versions = append(versions, v.Version)
		}
		output = cliutils.MarshalIndent(versions, "nodemanagement agentfiles versions")

		// Otherwise, output the list of agent types, if it isn't empty
	} else if len(agentTypes) > 0 {
		output = cliutils.MarshalIndent(agentTypes, "nodemanagement agentfiles versions")

		// Finally, return an empty list if there were no types with the specified features
	} else {
		output = "[]"
	}
	msgPrinter.Println("The highest version and \"latest\" refer to the same object.")
	fmt.Println(output)
}
