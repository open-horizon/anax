package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/open-horizon/anax/containermessage"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/kube_operator"
	"golang.org/x/text/message"
)

type DeploymentConfig struct {
	Services map[string]*containermessage.Service `json:"services"`
}

func (dc DeploymentConfig) CLIString() string {
	servs := ""
	for serviceName := range dc.Services {
		servs += serviceName + ", "
	}
	servs = servs[:len(servs)-2]
	return fmt.Sprintf("service(s) %v", servs)
}

func (dc DeploymentConfig) String() string {

	res := ""
	for serviceName, deployConfig := range dc.Services {
		res += fmt.Sprintf("service: %v, config: %v", serviceName, deployConfig)
	}

	return res
}

func (dc DeploymentConfig) HasAnyServices() bool {
	if len(dc.Services) == 0 {
		return false
	}
	return true
}

func (dc DeploymentConfig) AnyServiceName() string {
	for n, _ := range dc.Services {
		return n
	}
	return ""
}

// A validation method. Is there enough info in the deployment config to start a container? If not, the
// missing info is returned in the error message. Note that when there is a complete absence of deployment
// config metadata, that's ok too for services.
func (dc DeploymentConfig) CanStartStop() error {
	// get default message printer if nil
	msgPrinter := i18n.GetMessagePrinter()

	if len(dc.Services) == 0 {
		return nil
		// return errors.New(fmt.Sprintf("no services defined"))
	} else {
		for serviceName, service := range dc.Services {
			if len(serviceName) == 0 {
				return errors.New(msgPrinter.Sprintf("no service name"))
			} else if len(service.Image) == 0 {
				return errors.New(msgPrinter.Sprintf("no docker image for service %s", serviceName))
			}
		}
	}
	return nil
}

// Take the deployment field, which we have told the json unmarshaller was unknown type (so we can handle both escaped string and struct)
// and turn it into the DeploymentConfig struct we really want.
func ConvertToDeploymentConfig(deployment interface{}, msgPrinter *message.Printer) (*DeploymentConfig, error) {
	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	var jsonBytes []byte
	var err error

	// Take whatever type the deployment field is and convert it to marshalled json bytes
	switch d := deployment.(type) {
	case string:
		if len(d) == 0 {
			return nil, nil
		}
		// In the original input file this was escaped json as a string, but the original unmarshal removed the escapes
		jsonBytes = []byte(d)
	case nil:
		return nil, nil
	default:
		// The only other valid input is regular json in DeploymentConfig structure. Marshal it back to bytes so we can unmarshal it in a way that lets Go know it is a DeploymentConfig
		jsonBytes, err = json.Marshal(d)
		if err != nil {
			return nil, fmt.Errorf(msgPrinter.Sprintf("failed to marshal body for %v: %v", d, err))
		}
	}

	// Now unmarshal the bytes into the struct we have wanted all along
	depConfig := new(DeploymentConfig)
	err = json.Unmarshal(jsonBytes, depConfig)
	if err != nil {
		return nil, fmt.Errorf(msgPrinter.Sprintf("failed to unmarshal json for deployment field %s: %v", string(jsonBytes), err))
	}

	return depConfig, nil
}

// Get the metadata filed from the cluster deployment config
// inspectOperatorForNS: get the namespace from the operator if 'metadata' attribute is not defined.
func GetClusterDeploymentMetadata(clusterDeployment interface{}, inspectOperatorForNS bool, msgPrinter *message.Printer) (map[string]interface{}, error) {
	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	var err error
	var tempInterf interface{}

	// Take whatever type the deployment field is and convert it to marshalled json bytes
	switch d := clusterDeployment.(type) {
	case string:
		if len(d) == 0 {
			return nil, nil
		}
		// In the original input file this was escaped json as a string, but the original unmarshal removed the escapes
		jsonBytes := []byte(d)
		err = json.Unmarshal(jsonBytes, &tempInterf)
		if err != nil {
			return nil, fmt.Errorf(msgPrinter.Sprintf("failed to unmarshal json for cluster deployment field %s: %v", string(jsonBytes), err))
		}
	case nil:
		return nil, nil
	default:
		// The only other valid input is regular json in ClusterDeploymentConfig structure. Marshal it back to bytes so we can unmarshal it in a way that lets Go know it is a DeploymentConfig
		tempInterf = clusterDeployment
	}

	var metadata map[string]interface{}

	// check if metadada is already in the deployment config
	depConfig, ok := tempInterf.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf(msgPrinter.Sprintf("Invalid data presented in the cluster deployment field: %v", tempInterf))
	} else if md, ok := depConfig["metadata"]; ok {
		if metadata, ok = md.(map[string]interface{}); !ok {
			return nil, fmt.Errorf(msgPrinter.Sprintf("The metadata attribute in the cluster deployment has wrong format."))
		} else {
			// namespace is has already been inpsected, use it
			if _, ok := metadata["namespace"]; ok {
				return metadata, nil
			}
		}
	}

	// inspect the kube operator to get the namespace
	if inspectOperatorForNS {
		if tempData, ok := depConfig["operatorYamlArchive"]; ok {
			if tarData, ok := tempData.(string); ok {
				if ns, err := GetKubeOperatorNamespace(tarData); err != nil {
					return nil, fmt.Errorf(msgPrinter.Sprintf("Failed to get the namespace from the Kube operator. %v", err))
				} else {
					if metadata == nil {
						metadata = make(map[string]interface{}, 0)
					}
					metadata["namespace"] = ns
				}
			}
		}
	}

	return metadata, nil
}

func GetKubeOperatorNamespace(tar string) (string, error) {
	_, namespace, err := kube_operator.ProcessDeployment(tar, nil, map[string]string{}, "", 0)
	return namespace, err
}
