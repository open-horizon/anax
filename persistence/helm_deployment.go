package persistence

import (
	"encoding/json"
	"errors"
	"fmt"
)

// The structure of the json string in the deployment field of a service definition when the
// service is deployed via Helm to a Kubernetes cluster.

type HelmDeploymentConfig struct {
	ChartArchive string `json:"chart_archive"` // base64 encoded binary of helm package tar file
	ReleaseName  string `json:"release_name"`
}

func NewHelmDeployment(chartArchive string, releaseName string) *HelmDeploymentConfig {
	hd := new(HelmDeploymentConfig)
	hd.ChartArchive = chartArchive
	hd.ReleaseName = releaseName
	return hd
}

func (h HelmDeploymentConfig) String() string {
	maxArchiveLength := 25
	if len(h.ChartArchive) < maxArchiveLength {
		maxArchiveLength = len(h.ChartArchive)
	}
	return fmt.Sprintf("Release Name %v, Package %v", h.ReleaseName, h.ChartArchive[:maxArchiveLength])
}

func IsHelm(dep map[string]interface{}) bool {
	if _, ok := dep["chart_archive"]; ok {
		return true
	}
	return false
}

// Functions that allow HelmDeploymentConfig to support the DeploymentConfig interface.

func (h *HelmDeploymentConfig) IsNative() bool {
	return false
}

func (h *HelmDeploymentConfig) ToPersistentForm() (map[string]interface{}, error) {
	ret := make(map[string]interface{})

	// Marshal to JSON form so that we can unmarshal as a map[string]interface{}.
	if jBytes, err := json.Marshal(h); err != nil {
		return ret, errors.New(fmt.Sprintf("error marshalling helm deployment: %v, error: %v", h, err))
	} else if err := json.Unmarshal(jBytes, &ret); err != nil {
		return ret, errors.New(fmt.Sprintf("error unmarshalling helm deployment: %v, error: %v", string(jBytes), err))
	}

	return ret, nil
}

func (h *HelmDeploymentConfig) FromPersistentForm(pf map[string]interface{}) error {

	// Marshal to JSON form so that we can unmarshal as a HelmDeploymentConfig.
	if jBytes, err := json.Marshal(pf); err != nil {
		return errors.New(fmt.Sprintf("error marshalling helm persistent deployment: %v, error: %v", h, err))
	} else if err := json.Unmarshal(jBytes, h); err != nil {
		return errors.New(fmt.Sprintf("error unmarshalling helm persistent deployment: %v, error: %v", string(jBytes), err))
	}

	return nil
}

func (h *HelmDeploymentConfig) ToString() string {
	if h != nil {
		return h.String()
	} else {
		return ""
	}
}

// Given a deployment string, unmarshal it as a HelmDeployment object. It might not be a HelmDeployment, so
// we have to verify what was just unmarshalled.
func GetHelmDeployment(depStr string) (*HelmDeploymentConfig, error) {

	hd := new(HelmDeploymentConfig)
	err := json.Unmarshal([]byte(depStr), hd)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("error unmarshalling deployment config as HelmDeployment: %v", err))
	}

	if len(hd.ChartArchive) == 0 {
		return nil, errors.New(fmt.Sprintf("deployment config is not a HelmDeployment"))
	}

	return hd, nil

}
