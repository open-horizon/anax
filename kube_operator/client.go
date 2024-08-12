package kube_operator

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/persistence"
	olmv1scheme "github.com/operator-framework/api/pkg/operators/v1"
	olmv1alpha1scheme "github.com/operator-framework/api/pkg/operators/v1alpha1"
	olmv1client "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned/typed/operators/v1"
	olmv1alpha1client "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned/typed/operators/v1alpha1"
	yaml "gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	v1scheme "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	v1beta1scheme "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	dynamic "k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
)

const (
	DEFAULT_ANAX_NAMESPACE = "openhorizon-agent"
	// Name for the env var config map. Only characters allowed: [a-z] "." and "-"
	HZN_ENV_VARS = "hzn-env-vars"
	// Variable that contains the name of the config map
	HZN_ENV_KEY = "HZN_ENV_VARS"
	// Name for the k8s secrets that contains service secrets. Only characters allowed: [a-z] "." and "-"
	HZN_SERVICE_SECRETS = "hzn-service-secrets"

	SECRETS_VOLUME_NAME = "service-secrets-vol"

	MMS_VOLUME_NAME = "mms-shared-storage"

	K8S_CLUSTER_ROLE_TYPE          = "ClusterRole"
	K8S_CLUSTER_ROLEBINDING_TYPE   = "ClusterRoleBinding"
	K8S_ROLE_TYPE                  = "Role"
	K8S_ROLEBINDING_TYPE           = "RoleBinding"
	K8S_DEPLOYMENT_TYPE            = "Deployment"
	K8S_SERVICEACCOUNT_TYPE        = "ServiceAccount"
	K8S_CRD_TYPE                   = "CustomResourceDefinition"
	K8S_NAMESPACE_TYPE             = "Namespace"
	K8S_SECRET_TYPE                = "Secret"
	K8S_UNSTRUCTURED_TYPE          = "Unstructured"
	K8S_OLM_OPERATOR_GROUP_TYPE    = "OperatorGroup"
	K8S_MMS_SHARED_PVC_NAME        = "mms-shared-storage-pvc"
	STORAGE_CLASS_USERINPUT_NAME   = "MMS_K8S_STORAGE_CLASS"
	PVC_SIZE_USERINPUT_NAME        = "MMS_K8S_STORAGE_SIZE"
	PVC_ACCESS_MODE_USERINPUT_NAME = "MMS_K8S_PVC_ACCESS_MODE"
	DEFAULT_PVC_SIZE_IN_STRING     = "10"
)

var (
	accessModeMap = map[string]corev1.PersistentVolumeAccessMode{
		"ReadWriteOnce": corev1.ReadWriteOnce,
		"ReadWriteMany": corev1.ReadWriteMany,
	}
)

// Order is important here since it will be used to determine install order.
// For example, secrets should be before deployments,
// because it may be an image pull secret used by the deployment
func getBaseK8sKinds() []string {
	return []string{K8S_NAMESPACE_TYPE, K8S_CLUSTER_ROLE_TYPE, K8S_CLUSTER_ROLEBINDING_TYPE, K8S_ROLE_TYPE, K8S_ROLEBINDING_TYPE, K8S_SERVICEACCOUNT_TYPE, K8S_SECRET_TYPE, K8S_CRD_TYPE, K8S_DEPLOYMENT_TYPE}
}

func getDangerKinds() []string {
	return []string{K8S_OLM_OPERATOR_GROUP_TYPE}
}

func IsBaseK8sType(kind string) bool {
	return cutil.SliceContains(getBaseK8sKinds(), kind)
}

func IsDangerType(kind string) bool {
	return cutil.SliceContains(getDangerKinds(), kind)
}

// Intermediate state for the objects used for k8s api objects that haven't had their exact type asserted yet
type APIObjects struct {
	Type   *schema.GroupVersionKind
	Object interface{}
}

// Intermediate state used for after the objects have been read from the deployment but not converted to k8s objects yet
type YamlFile struct {
	Header tar.Header
	Body   string
}

// Client to interact with all standard k8s objects
type KubeClient struct {
	Client            *kubernetes.Clientset
	DynClient         dynamic.Interface
	OLMV1Alpha1Client olmv1alpha1client.OperatorsV1alpha1Client
	OLMV1Client       olmv1client.OperatorsV1Client
}

// KubeStatus contains the status of operator pods and a user-defined status object
type KubeStatus struct {
	ContainerStatuses []ContainerStatus
	OperatorStatus    interface{}
}

type ContainerStatus struct {
	Name        string
	Image       string
	CreatedTime int64
	State       string
}

func NewKubeClient() (*KubeClient, error) {
	clientset, err := cutil.NewKubeClient()
	if err != nil {
		return nil, err
	}
	dynClient, err := NewDynamicKubeClient()
	if err != nil {
		return nil, err
	}
	return &KubeClient{Client: clientset, DynClient: dynClient}, nil
}

// NewDynamicKubeClient returns a kube client that interacts with unstructured.Unstructured type objects
func NewDynamicKubeClient() (dynamic.Interface, error) {
	config, err := cutil.NewKubeConfig()
	if err != nil {
		return nil, err
	}
	clientset, _ := dynamic.NewForConfig(config)
	return clientset, nil
}

// Install creates the objects specified in the operator deployment in the cluster and creates the custom resource to start the operator
func (c KubeClient) Install(tar string, metadata map[string]interface{}, mmsPVCConfig map[string]interface{}, envVars map[string]string, fssAuthFilePath string, fssCertFilePath string, secretsMap map[string]string, agId string, reqNamespace string, crInstallTimeout int64) error {

	apiObjMap, opNamespace, err := ProcessDeployment(tar, metadata, mmsPVCConfig, envVars, fssAuthFilePath, fssCertFilePath, secretsMap, agId, crInstallTimeout)
	if err != nil {
		return err
	}

	// get and check namespace
	namespace := getFinalNamespace(reqNamespace, opNamespace)
	nodeNamespace := cutil.GetClusterNamespace()
	nodeIsNamespaceScope := cutil.IsNamespaceScoped()
	if namespace != nodeNamespace && nodeIsNamespaceScope {
		return fmt.Errorf("Service failed to start for agreement %v. Could not deploy service into namespace %v because the agent's namespace is namespace scoped, and it restricts all services to the agent namespace %v", agId, namespace, nodeNamespace)
	} else if namespace != nodeNamespace {
		// create network policies to allow traffic between the node and service
		ingress := networkingv1.NetworkPolicyIngressRule{From: []networkingv1.NetworkPolicyPeer{networkingv1.NetworkPolicyPeer{NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": namespace}}}}}
		egress := networkingv1.NetworkPolicyEgressRule{To: []networkingv1.NetworkPolicyPeer{networkingv1.NetworkPolicyPeer{NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": namespace}}}}}
		spec := networkingv1.NetworkPolicySpec{PodSelector: metav1.LabelSelector{}, Ingress: []networkingv1.NetworkPolicyIngressRule{ingress}, Egress: []networkingv1.NetworkPolicyEgressRule{egress}, PolicyTypes: []networkingv1.PolicyType{"Ingress", "Egress"}}
		netPol := networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-networkPolicy", agId), Namespace: nodeNamespace}, Spec: spec}
		_, err := c.Client.NetworkingV1().NetworkPolicies(nodeNamespace).Create(context.Background(), &netPol, metav1.CreateOptions{})
		if err != nil {
			glog.Errorf(kwlog(fmt.Sprintf("Error creating network policy: %v. Continuing installation.", err)))
		}
	}

	// If the namespace was specified in the deployment then create the namespace object so it can be created
	_, ok := apiObjMap[K8S_NAMESPACE_TYPE]
	if !ok || namespace != opNamespace {
		nsObj := corev1.Namespace{TypeMeta: metav1.TypeMeta{Kind: "Namespace"}, ObjectMeta: metav1.ObjectMeta{Name: namespace}}
		apiObjMap[K8S_NAMESPACE_TYPE] = []APIObjectInterface{NamespaceCoreV1{NamespaceObject: &nsObj}}
	}

	baseK8sComponents := getBaseK8sKinds()

	// install all the objects of built-in k8s types
	for _, componentType := range baseK8sComponents {
		for _, componentObj := range apiObjMap[componentType] {
			if err = componentObj.Install(c, namespace); err != nil {
				return err
			}
			glog.Infof(kwlog(fmt.Sprintf("successfully installed %v %v", componentType, componentObj.Name())))
		}
	}

	// install any remaining components of unknown type
	for _, unknownObj := range apiObjMap[K8S_UNSTRUCTURED_TYPE] {
		if err = unknownObj.Install(c, namespace); err != nil {
			return err
		}
		glog.Infof(kwlog(fmt.Sprintf("successfully installed %v", unknownObj.Name())))
	}

	// TODO: Update cluster namespace in agreement or microservice?
	glog.V(3).Infof(kwlog(fmt.Sprintf("all operator objects installed")))

	return nil
}

// Install creates the objects specified in the operator deployment in the cluster and creates the custom resource to start the operator
func (c KubeClient) Uninstall(tar string, metadata map[string]interface{}, agId string, reqNamespace string) error {

	apiObjMap, opNamespace, err := ProcessDeployment(tar, metadata, nil, map[string]string{}, "", "", map[string]string{}, agId, 0)
	if err != nil {
		return err
	}
	namespace := getFinalNamespace(reqNamespace, opNamespace)

	for _, crd := range apiObjMap[K8S_CRD_TYPE] {
		crd.Uninstall(c, namespace)
	}

	// uninstall any remaining components of unknown type
	for _, unknownObj := range apiObjMap[K8S_UNSTRUCTURED_TYPE] {
		glog.Infof(kwlog(fmt.Sprintf("attempting to uninstall %v", unknownObj.Name())))
		unknownObj.Uninstall(c, namespace)
	}

	nodeIsNamespaceScope := cutil.IsNamespaceScoped()
	nodeNamespace := cutil.GetClusterNamespace()
	// If the namespace was specified in the deployment then create the namespace object so it can be uninstalled
	if _, ok := apiObjMap[K8S_NAMESPACE_TYPE]; !ok && namespace != nodeNamespace && !nodeIsNamespaceScope {
		nsObj := corev1.Namespace{TypeMeta: metav1.TypeMeta{Kind: "Namespace"}, ObjectMeta: metav1.ObjectMeta{Name: namespace}}
		apiObjMap[K8S_NAMESPACE_TYPE] = []APIObjectInterface{NamespaceCoreV1{NamespaceObject: &nsObj}}
	} else if namespace != nodeNamespace {
		// delete the network policy that allows traffic between the node and service
		err := c.Client.NetworkingV1().NetworkPolicies(nodeNamespace).Delete(context.Background(), fmt.Sprintf("%s-networkPolicy", agId), metav1.DeleteOptions{})
		if err != nil {
			glog.Errorf(kwlog(fmt.Sprintf("Error deleting network policy: %v", err)))
		}
	}

	baseK8sComponents := getBaseK8sKinds()

	// uninstall all the objects of built-in k8s types
	for i := len(baseK8sComponents) - 1; i >= 0; i-- {
		componentType := baseK8sComponents[i]
		for _, componentObj := range apiObjMap[componentType] {
			glog.Infof(kwlog(fmt.Sprintf("attempting to uninstall %v %v", componentType, componentObj.Name())))
			componentObj.Uninstall(c, namespace)
		}
	}

	glog.V(3).Infof(kwlog(fmt.Sprintf("Completed removal of all operator objects from the cluster.")))
	return nil
}
func (c KubeClient) OperatorStatus(tar string, metadata map[string]interface{}, agId string, reqNamespace string) (interface{}, error) {
	apiObjMap, opNamespace, err := ProcessDeployment(tar, metadata, nil, map[string]string{}, "", "", map[string]string{}, agId, 0)
	if err != nil {
		return nil, err
	}
	namespace := getFinalNamespace(reqNamespace, opNamespace)

	if len(apiObjMap[K8S_DEPLOYMENT_TYPE]) < 1 {
		return nil, fmt.Errorf(kwlog(fmt.Sprintf("Error: failed to find operator deployment object.")))
	}

	status, err := apiObjMap[K8S_DEPLOYMENT_TYPE][0].Status(c, namespace)
	if err != nil {
		return nil, err
	}
	return status, nil
}

func (c KubeClient) Status(tar string, metadata map[string]interface{}, agId string, reqNamespace string) ([]ContainerStatus, error) {
	apiObjMap, opNamespace, err := ProcessDeployment(tar, metadata, nil, map[string]string{}, "", "", map[string]string{}, agId, 0)
	if err != nil {
		return nil, err
	}
	namespace := getFinalNamespace(reqNamespace, opNamespace)

	if len(apiObjMap[K8S_DEPLOYMENT_TYPE]) < 1 {
		return nil, fmt.Errorf(kwlog(fmt.Sprintf("Error: failed to find operator deployment object.")))
	}

	deployment := apiObjMap[K8S_DEPLOYMENT_TYPE][0]

	podList, err := deployment.Status(c, namespace)
	if err != nil {
		return nil, err
	}

	if podListTyped, ok := podList.(*corev1.PodList); ok {
		if len(podListTyped.Items) < 1 {
			return nil, nil
		}
		pod := podListTyped.Items[0]
		containerStatuses := []ContainerStatus{}

		for _, status := range pod.Status.ContainerStatuses {
			newStatus := ContainerStatus{Name: pod.ObjectMeta.Name}
			newStatus.Image = status.Image
			newStatus.Name = status.Name
			if status.State.Running != nil {
				newStatus.State = "Running"
				newStatus.CreatedTime = status.State.Running.StartedAt.Time.Unix()
			} else if status.State.Terminated != nil {
				newStatus.State = "Terminated"
				newStatus.CreatedTime = status.State.Terminated.StartedAt.Time.Unix()
			} else {
				newStatus.State = "Waiting"
			}
			containerStatuses = append(containerStatuses, newStatus)
		}
		return containerStatuses, nil
	} else {
		return nil, fmt.Errorf(kwlog(fmt.Sprintf("Error: deployment status returned unexpected type.")))
	}
}

// Currently we only support service/vault secret update, this k8s secret is create with service secret value in agreement. It is not the secret.yml from operator file
func (c KubeClient) Update(tar string, metadata map[string]interface{}, agId string, reqNamespace string, updatedEnv map[string]string, updatedSecrets []persistence.PersistedServiceSecret) error {
	// Convert updatedSecrets to map[string]string
	updatedSecretsMap := make(map[string]string, 0)
	for _, pss := range updatedSecrets {
		secName := pss.SvcSecretName
		secValue := pss.SvcSecretValue
		updatedSecretsMap[secName] = secValue
	}

	// Current implementaion only updatedSecrets will be passed into this function
	apiObjMap, opNamespace, err := ProcessDeployment(tar, metadata, nil, updatedEnv, "", "", updatedSecretsMap, agId, 0)
	if err != nil {
		return err
	}
	namespace := getFinalNamespace(reqNamespace, opNamespace)

	if len(apiObjMap[K8S_DEPLOYMENT_TYPE]) < 1 {
		return fmt.Errorf(kwlog(fmt.Sprintf("Error: failed to find operator deployment object.")))
	}

	deployment := apiObjMap[K8S_DEPLOYMENT_TYPE][0] // deployment with updated secrets
	err = deployment.Update(c, namespace)
	if err != nil {
		return err
	}

	glog.V(3).Infof(kwlog(fmt.Sprintf("Successfully update the service secrets in namespace %v", namespace)))
	return nil
}

// processDeployment takes the deployment string and converts it to a map with the k8s objects, the namespace to be used, and an error if one occurs
func ProcessDeployment(tar string, metadata map[string]interface{}, mmsPVCConfig map[string]interface{}, envVars map[string]string, fssAuthFilePath string, fssCertFilePath string, secretsMap map[string]string, agId string, crInstallTimeout int64) (map[string][]APIObjectInterface, string, error) {
	// Read the yaml files from the commpressed tar files
	yamls, err := getYamlFromTarGz(tar)
	if err != nil {
		return nil, "", err
	}

	// Convert the yaml files to kubernetes objects
	k8sObjs, customResources, err := getK8sObjectFromYaml(yamls, nil)
	if err != nil {
		return nil, "", err
	}

	customResourceKindMap := map[string][]*unstructured.Unstructured{}
	for _, customResource := range customResources {
		unstructCr, err := unstructuredObjectFromYaml(customResource)
		if err != nil {
			return nil, "", err
		}
		customResourceKindMap[unstructCr.GetKind()] = append(customResourceKindMap[unstructCr.GetKind()], unstructCr)
	}

	// Sort the k8s api objects by kind
	return sortAPIObjects(k8sObjs, customResourceKindMap, metadata, mmsPVCConfig, envVars, fssAuthFilePath, fssCertFilePath, secretsMap, agId, crInstallTimeout)
}

// CreateConfigMap will create a config map with the provided environment variable map
func (c KubeClient) CreateConfigMap(envVars map[string]string, agId string, namespace string) (string, error) {
	// a userinput with an empty string for the name will cause an error. need to remove before creating the configmap
	for varName, varVal := range envVars {
		if varName == "" {
			glog.Errorf("Omitting userinput with empty name and value: %v", varVal)
		}
		delete(envVars, "")
	}
	// hzn-env-vars-<agId>
	hznEnvConfigMap := corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-%s", HZN_ENV_VARS, agId)}, Data: envVars}
	res, err := c.Client.CoreV1().ConfigMaps(namespace).Create(context.Background(), &hznEnvConfigMap, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("Error: failed to create config map for %s: %v", agId, err)
	}
	return res.ObjectMeta.Name, nil
}

// DeleteConfigMap will delete the config map with the provided name
func (c KubeClient) DeleteConfigMap(agId string, namespace string) error {
	// hzn-env-vars-<agId>
	hznEnvConfigmapName := fmt.Sprintf("%s-%s", HZN_ENV_VARS, agId)
	err := c.Client.CoreV1().ConfigMaps(namespace).Delete(context.Background(), hznEnvConfigmapName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Error: failed to delete config map for %s: %v", agId, err)
	}
	return nil
}

// CreateESSSecret will create a k8s secrets object from the ess auth file
func (c KubeClient) CreateESSAuthSecrets(fssAuthFilePath string, agId string, namespace string) (string, error) {
	if essAuth, err := os.Open(fssAuthFilePath); err != nil {
		return "", err
	} else if essAuthBytes, err := ioutil.ReadAll(essAuth); err != nil {
		return "", err
	} else {
		secretData := make(map[string][]byte)
		secretData[config.HZN_FSS_AUTH_FILE] = essAuthBytes
		fssSecret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s", config.HZN_FSS_AUTH_PATH, agId), // ess-auth-<agId>
				Namespace: namespace,
			},
			Data: secretData,
		}
		res, err := c.Client.CoreV1().Secrets(namespace).Create(context.Background(), &fssSecret, metav1.CreateOptions{})
		if err != nil {
			return "", fmt.Errorf("Error: failed to create ess auth secret for %s: %v", agId, err)
		}
		return res.ObjectMeta.Name, nil
	}

}

func (c KubeClient) DeleteESSAuthSecrets(agId string, namespace string) error {
	essAuthSecretName := fmt.Sprintf("%s-%s", config.HZN_FSS_AUTH_PATH, agId)
	err := c.Client.CoreV1().Secrets(namespace).Delete(context.Background(), essAuthSecretName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Error: failed to delete ess auth secret for %s: %v", agId, err)
	}
	return nil
}

func (c KubeClient) CreateESSCertSecrets(fssCertFilePath string, agId string, namespace string) (string, error) {
	if essCert, err := os.Open(fssCertFilePath); err != nil {
		return "", err
	} else if essCertBytes, err := ioutil.ReadAll(essCert); err != nil {
		return "", err
	} else {
		secretData := make(map[string][]byte)
		secretData[config.HZN_FSS_CERT_FILE] = essCertBytes
		certSecret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s", config.HZN_FSS_CERT_PATH, agId), // ess-cert-<agId>
				Namespace: namespace,
			},
			Data: secretData,
		}

		res, err := c.Client.CoreV1().Secrets(namespace).Create(context.Background(), &certSecret, metav1.CreateOptions{})
		if err != nil && errors.IsAlreadyExists(err) {
			_, err = c.Client.CoreV1().Secrets(namespace).Update(context.Background(), &certSecret, metav1.UpdateOptions{})
		}
		if err != nil {
			return "", fmt.Errorf("Error: failed to create ess cert secret for %s: %v", agId, err)
		}
		return res.ObjectMeta.Name, nil
	}
}

func (c KubeClient) DeleteESSCertSecrets(agId string, namespace string) error {
	essCertSecretName := fmt.Sprintf("%s-%s", config.HZN_FSS_CERT_PATH, agId)
	err := c.Client.CoreV1().Secrets(namespace).Delete(context.Background(), essCertSecretName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Error: failed to delete ess cert secret for %s: %v", agId, err)
	}
	return nil
}

// CreateK8SSecrets will create a k8s secrets object which contains the service secret name and value
func (c KubeClient) CreateK8SSecrets(serviceSecretsMap map[string]string, agId string, namespace string) (string, error) {
	secretsLabel := map[string]string{"name": HZN_SERVICE_SECRETS}
	hznServiceSecrets := corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-%s", HZN_SERVICE_SECRETS, agId), Labels: secretsLabel}, StringData: serviceSecretsMap}
	res, err := c.Client.CoreV1().Secrets(namespace).Create(context.Background(), &hznServiceSecrets, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("Error: failed to create k8s secrets that contains service secrets for %s: %v", agId, err)
	}
	return res.ObjectMeta.Name, nil
}

// DeleteK8SSecrets will delete k8s secrets object which contains the service secret name and value
func (c KubeClient) DeleteK8SSecrets(agId string, namespace string) error {
	// delete the secrets contains agreement service vault secrets
	secretsName := fmt.Sprintf("%s-%s", HZN_SERVICE_SECRETS, agId)
	err := c.Client.CoreV1().Secrets(namespace).Delete(context.Background(), secretsName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Error: failed to delete k8s secrets that contains service secrets for %s: %v", agId, err)
	}
	return nil
}

func (c KubeClient) CreateMMSPVC(envVars map[string]string, mmsPVCConfig map[string]interface{}, agId string, namespace string) (string, error) {
	storageClass, accessModes, _ := cutil.GetAgentPVCInfo()

	if scInUserinput, ok := envVars[STORAGE_CLASS_USERINPUT_NAME]; ok {
		storageClass = scInUserinput
	}

	if accessModeInUserinput, ok := envVars[PVC_ACCESS_MODE_USERINPUT_NAME]; ok {
		if m, ok := accessModeMap[accessModeInUserinput]; ok {
			accessModes = []corev1.PersistentVolumeAccessMode{m}
		}
	}

	pvcSizeInString := DEFAULT_PVC_SIZE_IN_STRING
	if size, ok := mmsPVCConfig["pvcSize"]; ok {
		sizeInServiceDef := int64(size.(float64))
		if sizeInServiceDef > 0 {
			pvcSizeInString = strconv.FormatInt(sizeInServiceDef, 10)
		}
	}

	if pvcSizeInUserInput, ok := envVars[PVC_SIZE_USERINPUT_NAME]; ok {
		pvcSizeInString = pvcSizeInUserInput
	}

	mmsPvcName := fmt.Sprintf("%s-%s", K8S_MMS_SHARED_PVC_NAME, agId)
	mmsPVC := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mmsPvcName,
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &storageClass,
			AccessModes:      accessModes,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(fmt.Sprintf("%vGi", pvcSizeInString)),
				},
			},
		},
	}

	res, err := c.Client.CoreV1().PersistentVolumeClaims(namespace).Create(context.Background(), &mmsPVC, metav1.CreateOptions{})
	if err != nil && errors.IsAlreadyExists(err) {
		_, err = c.Client.CoreV1().PersistentVolumeClaims(namespace).Update(context.Background(), &mmsPVC, metav1.UpdateOptions{})
	}
	if err != nil {
		return "", fmt.Errorf("Error: failed to create mms pvc for %s: %v", agId, err)
	}
	return res.ObjectMeta.Name, nil
}

func (c KubeClient) DeleteMMSPVC(agId string, namespace string) error {
	mmsPvcName := fmt.Sprintf("%s-%s", K8S_MMS_SHARED_PVC_NAME, agId)

	err := c.Client.CoreV1().PersistentVolumeClaims(namespace).Delete(context.Background(), mmsPvcName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Error: failed to delete mms pvc for %s: %v", agId, err)
	}
	return nil
}

func unstructuredObjectFromYaml(crStr YamlFile) (*unstructured.Unstructured, error) {
	cr := make(map[string]interface{})
	err := yaml.UnmarshalStrict([]byte(crStr.Body), &cr)
	if err != nil {
		return nil, fmt.Errorf(kwlog(fmt.Sprintf("Error unmarshaling custom resource in deployment. %v", err)))
	}

	newCr := makeAllKeysStrings(cr).(map[string]interface{})
	unstructCr := unstructured.Unstructured{Object: newCr}
	return &unstructCr, nil
}

// add a reference to the envvar config map to the deployment
func addConfigMapVarToDeploymentObject(deployment appsv1.Deployment, configMapName string) appsv1.Deployment {
	hznEnvVar := corev1.EnvVar{Name: HZN_ENV_KEY, Value: configMapName}
	i := len(deployment.Spec.Template.Spec.Containers) - 1
	for i >= 0 {
		newEnv := append(deployment.Spec.Template.Spec.Containers[i].Env, hznEnvVar)
		deployment.Spec.Template.Spec.Containers[i].Env = newEnv
		i--
	}
	return deployment
}

// add a reference to the secrets service secrets to the deployment
func addServiceSecretsToDeploymentObject(deployment appsv1.Deployment, secretsName string) appsv1.Deployment {
	// Add secrets (secretsName is $HZN_SERVICE_SECRETS-$agId: hzn-service-secrets-12345) as Volume in deployment
	volumeName := SECRETS_VOLUME_NAME
	volume := corev1.Volume{Name: volumeName, VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: secretsName}}}
	volumes := append(deployment.Spec.Template.Spec.Volumes, volume)
	deployment.Spec.Template.Spec.Volumes = volumes

	// mount the volume to deployment containers
	secretsFilePathInPod := config.HZN_SECRETS_MOUNT
	volumeMount := corev1.VolumeMount{Name: volumeName, MountPath: secretsFilePathInPod}

	// Add secrets as volume mount for containers
	i := len(deployment.Spec.Template.Spec.Containers) - 1
	for i >= 0 {
		newVM := append(deployment.Spec.Template.Spec.Containers[i].VolumeMounts, volumeMount)
		deployment.Spec.Template.Spec.Containers[i].VolumeMounts = newVM
		i--
	}
	return deployment
}

// recursively go over the given interface to ensure any map keys are strings
func makeAllKeysStrings(unmarshYaml interface{}) interface{} {
	if reflect.ValueOf(unmarshYaml).Kind() == reflect.Map {
		retMap := make(map[string]interface{})
		if interfKeyMap, ok := unmarshYaml.(map[interface{}]interface{}); ok {
			for key, value := range interfKeyMap {
				retMap[fmt.Sprintf("%v", key)] = makeAllKeysStrings(value)
			}
		} else {
			for key, value := range unmarshYaml.(map[string]interface{}) {
				retMap[key] = makeAllKeysStrings(value)
			}
		}
		return retMap
	} else if reflect.ValueOf(unmarshYaml).Kind() == reflect.Slice {
		correctedSlice := make([]interface{}, len(unmarshYaml.([]interface{})))
		for _, elem := range unmarshYaml.([]interface{}) {
			correctedSlice = append(correctedSlice, makeAllKeysStrings(elem))
		}
		return correctedSlice
	}
	return unmarshYaml
}

// Convert the given yaml files into k8s api objects
func getK8sObjectFromYaml(yamlFiles []YamlFile, sch *runtime.Scheme) ([]APIObjects, []YamlFile, error) {
	retObjects := []APIObjects{}
	customResources := []YamlFile{}

	if sch == nil {
		sch = runtime.NewScheme()
	}

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
		obj, gvk, err := decode([]byte(fileStr.Body), nil, nil)

		if err != nil {
			customResources = append(customResources, fileStr)
		} else if IsBaseK8sType(gvk.Kind) {
			newObj := APIObjects{Type: gvk, Object: obj}
			retObjects = append(retObjects, newObj)
		} else if IsDangerType(gvk.Kind) {
			// the scheme has recognized this type but does not provide the function for converting it to an unstructured object. skip this one to avoid a panic.
			glog.Errorf(kwlog(fmt.Sprintf("Skipping unsupported kind %v", gvk.Kind)))
		} else {
			newUnstructObj := unstructured.Unstructured{}
			err = sch.Convert(obj, &newUnstructObj, conversion.Meta{})
			if err != nil {
				glog.Errorf("Err converting object to unstructured: %v", err)
			}
			newObj := APIObjects{Type: gvk, Object: &newUnstructObj}
			retObjects = append(retObjects, newObj)
		}
	}

	return retObjects, customResources, nil
}

// Read the compressed tar file from the operator deployments section
func getYamlFromTarGz(deploymentString string) ([]YamlFile, error) {
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
			tar, err := ioutil.ReadAll(tarReader)
			if err != nil {
				return files, fmt.Errorf("Error reading tar file: %v", err)
			}
			newFile := YamlFile{Header: *header, Body: string(tar)}
			files = append(files, newFile)
		} else {
			return files, err
		}
	}
	return files, nil
}

// get the namespace that the service will eventually be deployed to.
// reqNamespace: the requested namespace fromt agbot. It the namespace specified
// in the pattern or policy. If it is empty, agbot assign it to the namespace embedded
// in the metadata attribute of the clusterDeployment string for a service.
// opNamespace: the namespace embedded in the service operator.
// The result namespace is:
//  1. reqNamespace if not empty,
//  2. opNamespace if not empty,
//  3. nodeNamespace.
func getFinalNamespace(reqNamespace string, opNamespace string) string {
	nodeNamespace := cutil.GetClusterNamespace()

	namespace := reqNamespace
	if namespace == "" {
		namespace = opNamespace
	}
	if namespace == "" {
		namespace = nodeNamespace
	}

	return namespace
}
