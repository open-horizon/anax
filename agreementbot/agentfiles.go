package agreementbot

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/semanticversion"
	"sort"
	"strings"
)

// This function get all the agent files from the CSS and updates
// the IBM/AgentFileVersion object on the exchange. This object holds
// all the versions for agent_software_files, agent_config_files and
// agent_cert_files.
func (w *AgreementBotWorker) updateAgentFileVersions() int {
	glog.V(5).Info(AWlogString("updateAgentFileVersions called"))

	// get all the agent files from CSS
	agentFileMeta, err := exchange.GetCSSObjectsByType(w, "IBM", "")
	if err != nil {
		glog.Errorf(AWlogString(fmt.Sprintf("failed to get the metadata for agent files from CSS. %v", err)))
		return 0
	}

	afVersions := map[string]map[string]bool{}

	if agentFileMeta != nil {
		for _, agentFile := range *agentFileMeta {
			// Determine if there is an underscore signifying a version string (can't be first or last character)
			agentFileType := agentFile.ObjectType
			splitIdx := strings.Index(agentFileType, "-")
			if splitIdx > 0 && splitIdx < len(agentFileType)-1 {

				fileVersion := agentFileType[splitIdx+1:]
				fileType := agentFileType[:splitIdx]
				if !semanticversion.IsVersionString(fileVersion) {
					continue
				}
				if !exchangecommon.ValidFileTypes.Contains(fileType) {
					continue
				}
				if afVersions[fileType] == nil {
					afVersions[fileType] = map[string]bool{}
				}
				afVersions[fileType][fileVersion] = true
			}
		}
	}

	sw_versions := getVersions(afVersions, exchangecommon.AU_AGENTFILE_TYPE_SOFTWARE)
	config_versions := getVersions(afVersions, exchangecommon.AU_AGENTFILE_TYPE_CONFIG)
	cert_versions := getVersions(afVersions, exchangecommon.AU_AGENTFILE_TYPE_CERT)

	sortVersions(sw_versions)
	sortVersions(config_versions)
	sortVersions(cert_versions)

	// aget current IBM/AgentFileVersion object for the exchange API
	resp, err := exchange.GetNodeUpgradeVersions(w)
	if err != nil {
		glog.Errorf(AWlogString(fmt.Sprintf("failed to get the IBM/AgentFileVersion object from the exchange. %v", err)))
		return 0
	}

	// compare the current versions with the versionf from the AgenmtFileVersion object
	toUpdate := true
	if compareSortedVersionArrays(sw_versions, resp.SoftwareVersions) &&
		compareSortedVersionArrays(config_versions, resp.ConfigVersions) &&
		compareSortedVersionArrays(cert_versions, resp.CertVersions) {
		toUpdate = false
	}

	// update IBM/AgentFileVersion object with the latest versions
	if toUpdate {
		newAfv := exchangecommon.AgentFileVersions{
			SoftwareVersions: sw_versions,
			ConfigVersions:   config_versions,
			CertVersions:     cert_versions,
		}
		glog.V(3).Infof("AgreementBot worker updating IBM/AgentFileVersion with %v", newAfv)
		if err := exchange.PutNodeUpgradeVersions(w, &newAfv); err != nil {
			glog.Errorf(AWlogString(fmt.Sprintf("failed to update the IBM/AgentFileVersion object from the exchange. %v", err)))
			return 0
		}
	}

	return 0
}

// It returns the an array of versions for a given key.
func getVersions(afVersions map[string]map[string]bool, key string) []string {
	ret := []string{}
	if vMap, ok := afVersions[key]; ok {
		if vMap != nil {
			for k, _ := range vMap {
				ret = append(ret, k)
			}
		}
	}
	return ret
}

// sort the given version array
func sortVersions(versions []string) {
	sort.Slice(versions, func(i, j int) bool {
		comp, _ := semanticversion.CompareVersions(versions[i], versions[j])
		return comp > 0
	})
}

// return true if the 2 given sorted array are identical.
func compareSortedVersionArrays(ver1, ver2 []string) bool {
	if len(ver1) != len(ver2) {
		return false
	}

	for i := 0; i < len(ver1)-1; i++ {
		if ver1[i] != ver2[i] {
			return false
		}
	}

	return true
}
