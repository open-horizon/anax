package common

import (
	"archive/tar"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/open-horizon/anax/containermessage"
	"github.com/open-horizon/anax/i18n"
	olmv1scheme "github.com/operator-framework/api/pkg/operators/v1"
	olmv1alpha1scheme "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"golang.org/x/text/message"
	"io"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	v1scheme "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	v1beta1scheme "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"strings"
)

const K8S_NAMESPACE_TYPE = "Namespace"
const K8S_DEPLOYMENT_TYPE = "Deployment"

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
			return nil, fmt.Errorf("%s", msgPrinter.Sprintf("failed to marshal body for %v: %v", d, err))
		}
	}

	// Now unmarshal the bytes into the struct we have wanted all along
	depConfig := new(DeploymentConfig)
	err = json.Unmarshal(jsonBytes, depConfig)
	if err != nil {
		return nil, fmt.Errorf("%s", msgPrinter.Sprintf("failed to unmarshal json for deployment field %s: %v", string(jsonBytes), err))
	}

	return depConfig, nil
}

type ClusterDeploymentConfig struct {
	Metadata            map[string]interface{}             `json:"metadata,omitempty"`
	OperatorYamlArchive string                             `json:"operatorYamlArchive"`
	Secrets             map[string]containermessage.Secret `json:"secrets"`
	MMSPVC              map[string]interface{}             `json:"mmspvc,omitempty"`
}

// Take the deployment field, which we have told the json unmarshaller was unknown type (so we can handle both escaped string and struct)
// and turn it into the DeploymentConfig struct we really want.
func ConvertToClusterDeploymentConfig(clusterDeployment interface{}, msgPrinter *message.Printer) (*ClusterDeploymentConfig, error) {
	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	var jsonBytes []byte
	var err error

	// Take whatever type the deployment field is and convert it to marshalled json bytes
	switch d := clusterDeployment.(type) {
	case string:
		if len(d) == 0 {
			return nil, nil
		}
		// In the original input file this was escaped json as a string, but the original unmarshal removed the escapes
		jsonBytes = []byte(d)
	case nil:
		return nil, nil
	default:
		// The only other valid input is regular json in ClusterDeploymentConfig structure. Marshal it back to bytes so we can unmarshal it in a way that lets Go know it is a DeploymentConfig
		jsonBytes, err = json.Marshal(d)
		if err != nil {
			return nil, fmt.Errorf("%s", msgPrinter.Sprintf("failed to marshal body for %v: %v", d, err))
		}
	}

	// Now unmarshal the bytes into the struct we have wanted all along
	clusterDepConfig := new(ClusterDeploymentConfig)
	err = json.Unmarshal(jsonBytes, clusterDepConfig)
	if err != nil {
		return nil, fmt.Errorf("%s", msgPrinter.Sprintf("failed to unmarshal json for deployment field %s: %v", string(jsonBytes), err))
	}

	return clusterDepConfig, nil

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
			return nil, fmt.Errorf("%s", msgPrinter.Sprintf("failed to unmarshal json for cluster deployment field %s: %v", string(jsonBytes), err))
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
		return nil, fmt.Errorf("%s", msgPrinter.Sprintf("Invalid data presented in the cluster deployment field: %v", tempInterf))
	} else if md, ok := depConfig["metadata"]; ok {
		if metadata, ok = md.(map[string]interface{}); !ok {
			return nil, fmt.Errorf("%s", msgPrinter.Sprintf("The metadata attribute in the cluster deployment has wrong format."))
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
					return nil, fmt.Errorf("%s", msgPrinter.Sprintf("Failed to get the namespace from the Kube operator. %v", err))
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
	yamls, err := GetYamlFromTarGz(tar)
	if err != nil {
		return "", err
	}

	return getK8sNamespaceObjectFromYaml(yamls), nil
}

// Intermediate state used for after the objects have been read from the deployment but not converted to k8s objects yet
type YamlFile struct {
	Header tar.Header
	Body   string
}

// Read the compressed tar file from the operator deployments section
func GetYamlFromTarGz(deploymentString string) ([]YamlFile, error) {
	files := []YamlFile{}

	archiveData, err := base64.StdEncoding.DecodeString(deploymentString)
	if err != nil {
		return files, err
	}
	r := strings.NewReader(string(archiveData))

	zipReader, err := gzip.NewReader(r)
	if err != nil {
		return files, err
	}
	tarReader := tar.NewReader(zipReader)

	for {
		header, err := tarReader.Next()

		if err == io.EOF || header == nil {
			break
		} else if header.Typeflag == tar.TypeDir {
			continue
		} else if err == nil {
			tar, err := io.ReadAll(tarReader)
			if err != nil {
				return files, fmt.Errorf("error reading tar file: %v", err)
			}
			newFile := YamlFile{Header: *header, Body: string(tar)}
			files = append(files, newFile)
		} else {
			return files, err
		}
	}
	return files, nil
}

func IsNamespaceType(k8sType string) bool {
	return k8sType == K8S_NAMESPACE_TYPE
}

func IsDeploymentType(k8sType string) bool {
	return k8sType == K8S_DEPLOYMENT_TYPE
}

// Convert the given yaml files into k8s api objects
func getK8sNamespaceObjectFromYaml(yamlFiles []YamlFile) string {
	namespace := ""

	sch := runtime.NewScheme()

	// This is required to allow the schema to recognize custom resource definition types
	_ = v1beta1scheme.AddToScheme(sch)
	_ = v1scheme.AddToScheme(sch)
	_ = scheme.AddToScheme(sch)
	_ = olmv1alpha1scheme.AddToScheme(sch)
	_ = olmv1scheme.AddToScheme(sch)

	// multiple yaml files can be in one file separated by '---'
	// these are split here and rejoined with the single files
	indivYamls := []YamlFile{}
	for _, file := range yamlFiles {
		if multFiles := strings.Split(file.Body, "---"); len(multFiles) > 1 {
			for _, indivYaml := range multFiles {
				if strings.TrimSpace(indivYaml) != "" {
					indivYamls = append(indivYamls, YamlFile{Body: indivYaml})
				}
			}
		} else {
			indivYamls = append(indivYamls, file)
		}
	}

	for _, fileStr := range indivYamls {
		decode := serializer.NewCodecFactory(sch).UniversalDecoder(v1beta1scheme.SchemeGroupVersion, v1scheme.SchemeGroupVersion, rbacv1.SchemeGroupVersion, appsv1.SchemeGroupVersion, corev1.SchemeGroupVersion, olmv1alpha1scheme.SchemeGroupVersion, olmv1scheme.SchemeGroupVersion).Decode
		obj, gvk, _ := decode([]byte(fileStr.Body), nil, nil)

		if gvk != nil && IsNamespaceType(gvk.Kind) {
			if typedNS, ok := obj.(*corev1.Namespace); ok {
				namespace = typedNS.ObjectMeta.Name
			}
		}

		if gvk != nil && IsDeploymentType(gvk.Kind) && namespace == "" {
			if typedDeployment, ok := obj.(*appsv1.Deployment); ok {
				if typedDeployment.ObjectMeta.Namespace != "" {
					namespace = typedDeployment.ObjectMeta.Namespace
				}
			}
		}
	}

	return namespace
}
