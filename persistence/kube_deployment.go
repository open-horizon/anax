package persistence

import (
	"encoding/json"
	"fmt"
)

type KubeDeploymentConfig struct {
	OperatorYamlArchive   string `json:"operatorYamlArchive"`
}

func (k *KubeDeploymentConfig) ToString() string {
	if k != nil {
		return fmt.Sprintf("OperatorYamlArchive: %v", k.OperatorYamlArchive)
	}
	return ""
}

func GetKubeDeployment(deployStr string) (*KubeDeploymentConfig, error) {
	kd := new(KubeDeploymentConfig)
	err := json.Unmarshal([]byte(deployStr), kd)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling deployment config as KubeDeployment: %v", err)
	} else if kd.OperatorYamlArchive == "" {
		return nil, fmt.Errorf("required field 'operatorYamlArchive' is missing in the deployment string.")
	}
	return kd, nil
}

func (k *KubeDeploymentConfig) FromPersistentForm(pf map[string]interface{}) error {
	// Marshal to JSON form so that we can unmarshal as a KubeDeploymentConfig.
	if jBytes, err := json.Marshal(pf); err != nil {
		return fmt.Errorf("error marshalling kube persistent deployment: %v, error: %v", k, err)
	} else if err := json.Unmarshal(jBytes, k); err != nil {
		return fmt.Errorf("error unmarshalling kube persistent deployment: %v, error: %v", string(jBytes), err)
	}
	return nil
}

func (k *KubeDeploymentConfig) ToPersistentForm() (map[string]interface{}, error) {
	pf := make(map[string]interface{})

	// Marshal to JSON form so that we can unmarshal as a map[string]interface{}.
	if jBytes, err := json.Marshal(k); err != nil {
		return pf, fmt.Errorf("error marshalling kube deployment: %v, error: %v", k, err)
	} else if err := json.Unmarshal(jBytes, &pf); err != nil {
		return pf, fmt.Errorf("error unmarshalling kube deployment: %v, error: %v", string(jBytes), err)
	}

	return pf, nil
}

func (k *KubeDeploymentConfig) IsNative() bool {
	return false
}

// Check if the deployment is a kube deployment or not
func IsKube(dep map[string]interface{}) bool {
	if _, ok := dep["operatorYamlArchive"]; ok {
		return true
	}
	return false
}
