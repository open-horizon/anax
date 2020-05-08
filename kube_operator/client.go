package kube_operator

import (
	"archive/tar"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/cutil"
	yaml "gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiv1beta1client "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	dynamic "k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"reflect"
	"strings"
	"time"
)

const (
	ANAX_NAMESPACE = "openhorizon-agent"
	// Name for the env var config map. Only characters allowed: [a-z] "." and "-"
	HZN_ENV_VARS = "hzn-env-vars"
	// Variable that contains the name of the config map
	HZN_ENV_KEY = "HZN_ENV_VARS"

	K8S_ROLE_TYPE           = "Role"
	K8S_ROLEBINDING_TYPE    = "RoleBinding"
	K8S_DEPLOYMENT_TYPE     = "Deployment"
	K8S_SERVICEACCOUNT_TYPE = "ServiceAccount"
	K8S_CRD_TYPE            = "CustomResourceDefinition"
	K8S_NAMESPACE_TYPE      = "Namespace"
)

type YamlFile struct {
	Header tar.Header
	Body   string
}

type APIObjects struct {
	Type   *schema.GroupVersionKind
	Object interface{}
}

type KubeClient struct {
	Client *kubernetes.Clientset
}

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
	return &KubeClient{Client: clientset}, nil
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

func NewCRDClient() (*apiv1beta1client.ApiextensionsV1beta1Client, error) {
	config, err := cutil.NewKubeConfig()
	if err != nil {
		return nil, err
	}
	clientset, _ := apiv1beta1client.NewForConfig(config)
	return clientset, nil
}

// Install creates the objects specified in the operator deployment in the cluster and creates the custom resource to start the operator
func (c KubeClient) Install(tar string, envVars map[string]string, agId string) error {
	// Read the yaml files from the commpressed tar files
	yamls, err := getYamlFromTarGz(tar)
	if err != nil {
		return err
	}

	// Convert the yaml files to kubernetes objects
	k8sObjs, customResources, err := getK8sObjectFromYaml(yamls, nil)
	if err != nil {
		return err
	}

	if len(customResources) != 1 {
		return fmt.Errorf("Expected one custom resource in deployment. Got %d", len(customResources))
	}
	// Sort the k8s api objects by kind
	apiObjMap := sortAPIObjects(k8sObjs)

	nsObj, namespace, err := getOperatorNamespace(apiObjMap)
	if err != nil {
		return err
	}

	if nsObj != nil {
		glog.V(3).Infof(kwlog(fmt.Sprintf("attempting to create namespace %v", nsObj)))
		_, err := c.Client.CoreV1().Namespaces().Create(nsObj)
		if err != nil {
			glog.Warningf(kwlog(fmt.Sprintf("Failed to create namspace %s. Continuing with installation.", nsObj.ObjectMeta.Name)))
		}
	}

	mapName, err := c.CreateConfigMap(envVars, agId, namespace)
	if err != nil {
		return err
	}

	// Create the role types in the cluster
	for _, roleDef := range apiObjMap[K8S_ROLE_TYPE] {
		newRole := roleDef.Object.(*rbacv1.Role)
		glog.V(3).Infof(kwlog(fmt.Sprintf("creating role %v", newRole)))
		_, err := c.Client.RbacV1().Roles(namespace).Create(newRole)
		if err != nil {
			return err
		}
	}
	// Create the rolebinding types in the cluster
	for _, roleBindingDef := range apiObjMap[K8S_ROLEBINDING_TYPE] {
		newRoleBinding := roleBindingDef.Object.(*rbacv1.RoleBinding)
		glog.V(3).Infof(kwlog(fmt.Sprintf("creating rolebinding %v", newRoleBinding)))
		_, err := c.Client.RbacV1().RoleBindings(namespace).Create(newRoleBinding)
		if err != nil {
			return err
		}
	}
	// Create the serviceaccount types in the cluster
	for _, svcAcctDef := range apiObjMap[K8S_SERVICEACCOUNT_TYPE] {
		newSvcAcct := svcAcctDef.Object.(*corev1.ServiceAccount)
		glog.V(3).Infof(kwlog(fmt.Sprintf("creating service account %v", newSvcAcct)))
		_, err := c.Client.CoreV1().ServiceAccounts(namespace).Create(newSvcAcct)
		if err != nil {
			return err
		}
	}
	// Create the deployment types in the cluster
	for _, dep := range apiObjMap[K8S_DEPLOYMENT_TYPE] {
		newDep := dep.Object.(*appsv1.Deployment)
		glog.V(3).Infof(kwlog(fmt.Sprintf("creating deployment %v", newDep)))
		newDepWithEnv := addConfigMapVarToDeploymentObject(*newDep, mapName)
		_, err := c.Client.AppsV1().Deployments(namespace).Create(&newDepWithEnv)
		if err != nil {
			return err
		}
	}
	// Add the custom resource definitions to the client schema
	kindToGVRMap := map[string]schema.GroupVersionResource{}
	for _, crd := range apiObjMap[K8S_CRD_TYPE] {
		newCRD := crd.Object.(*v1beta1.CustomResourceDefinition)

		apiClient, err := NewCRDClient()
		if err != nil {
			return err
		}
		crds := apiClient.CustomResourceDefinitions()
		glog.V(3).Infof(kwlog(fmt.Sprintf("creating custom resource definition %v", newCRD)))
		_, err = crds.Create(newCRD)
		if err != nil {
			return err
		}
		kindToGVRMap[newCRD.Spec.Names.Kind] = schema.GroupVersionResource{Resource: newCRD.Spec.Names.Plural, Group: newCRD.Spec.Group, Version: newCRD.Spec.Version}
	}

	// Create the custom resources in the cluster
	for _, crStr := range customResources {
		cr := make(map[string]interface{})
		err := yaml.UnmarshalStrict([]byte(crStr.Body), &cr)
		if err != nil {
			return fmt.Errorf("Error unmarshaling custom resource in deployment. %v", err)
		}

		newCr := makeAllKeysStrings(cr).(map[string]interface{})

		dynClient, err := NewDynamicKubeClient()
		if err != nil {
			return err
		}
		crClient := dynClient.Resource(kindToGVRMap[newCr["kind"].(string)])

		unstructCr := unstructured.Unstructured{Object: newCr}

		// the cluster has to create the endpoint for the custom resource, this can take some time
		glog.V(3).Infof(kwlog(fmt.Sprintf("creating operator custom resource %v", newCr)))
		for {
			_, err := crClient.Namespace(namespace).Create(&unstructCr, metav1.CreateOptions{})
			if err != nil {
				time.Sleep(time.Second * 5)
			} else {
				break
			}
		}
	}
	glog.V(3).Infof(kwlog(fmt.Sprintf("all operator objects installed")))

	return nil
}

// Install creates the objects specified in the operator deployment in the cluster and creates the custom resource to start the operator
func (c KubeClient) Uninstall(tar string, agId string) error {
	// Read the yaml files from the commpressed tar files
	yamls, err := getYamlFromTarGz(tar)
	if err != nil {
		return err
	}
	// Convert the yaml files to kubernetes objects
	k8sObjs, customResources, err := getK8sObjectFromYaml(yamls, nil)
	if err != nil {
		return err
	}
	// Sort the k8s api objects by kind
	apiObjMap := sortAPIObjects(k8sObjs)

	_, namespace, err := getOperatorNamespace(apiObjMap)
	if err != nil {
		return err
	}

	configMapName := fmt.Sprintf("%s-%s", HZN_ENV_VARS, agId)
	glog.V(3).Infof(kwlog(fmt.Sprintf("deleting config map %v", configMapName)))
	// Delete the agreement config map
	err = c.Client.CoreV1().ConfigMaps(namespace).Delete(configMapName, &metav1.DeleteOptions{})
	if err != nil {
		glog.Errorf(kwlog(fmt.Sprintf("unable to delete config map %s. Error: %v", configMapName, err)))
	}

	// Delete the custom resource definitions from the cluster
	kindToGVRMap := map[string]schema.GroupVersionResource{}
	for _, crd := range apiObjMap[K8S_CRD_TYPE] {
		newCRD := crd.Object.(*v1beta1.CustomResourceDefinition)

		glog.V(3).Infof(kwlog(fmt.Sprintf("deleting operator custom resource definition %v", newCRD.ObjectMeta.Name)))

		// CRD's need a different client
		apiClient, err := NewCRDClient()
		if err != nil {
			return err
		}
		crds := apiClient.CustomResourceDefinitions()
		err = crds.Delete(newCRD.ObjectMeta.Name, &metav1.DeleteOptions{})
		if err != nil {
			glog.Errorf(kwlog(fmt.Sprintf("unable to delete operator custom resource definition %s. Error: %v", newCRD.ObjectMeta.Name, err)))
		}
		kindToGVRMap[newCRD.Spec.Names.Kind] = schema.GroupVersionResource{Resource: newCRD.Spec.Names.Plural, Group: newCRD.Spec.Group, Version: newCRD.Spec.Version}
	}

	// Delete the agreement config map
	c.Client.CoreV1().ConfigMaps(ANAX_NAMESPACE).Delete(fmt.Sprintf("%s-%s", HZN_ENV_VARS, agId), &metav1.DeleteOptions{})

	// Delete the custom resources in the cluster
	for _, crStr := range customResources {
		cr := make(map[string]interface{})
		err := yaml.UnmarshalStrict([]byte(crStr.Body), &cr)
		if err != nil {
			return fmt.Errorf("Error unmarshaling custom resource in deployment. %v", err)
		}

		newCr := makeAllKeysStrings(cr).(map[string]interface{})

		dynClient, err := NewDynamicKubeClient()
		if err != nil {
			return err
		}
		crClient := dynClient.Resource(kindToGVRMap[newCr["kind"].(string)])

		var newCrName string
		if metaInterf, ok := newCr["metadata"]; ok {
			if metaMap, ok := metaInterf.(map[string]interface{}); ok {
				if metaMapName, ok := metaMap["name"]; ok {
					newCrName = fmt.Sprintf("%v", metaMapName)
				} else {
					glog.Errorf(kwlog(fmt.Sprintf("unable to find operator custom resource name for %v", newCr)))
				}
			} else {
				glog.Errorf(kwlog(fmt.Sprintf("unable to find operator custom resource name for %v", newCr)))
			}
		} else {
			glog.Errorf(kwlog(fmt.Sprintf("unable to find operator custom resource name for %v", newCr)))
		}
		glog.V(3).Infof(kwlog(fmt.Sprintf("deleting operator custom resource %v", newCrName)))
		// the cluster has to create the endpoint for the custom resource, this can take some time. giving it 90 seconds for now. should probably be configurable
		err = crClient.Namespace(namespace).Delete(newCrName, &metav1.DeleteOptions{})
		if err != nil {
			glog.Errorf(kwlog(fmt.Sprintf("unable to delete operator custom resource %s. Error: %v", newCrName, err)))
		}
	}
	// Delete the deployment types in the cluster
	for _, dep := range apiObjMap[K8S_DEPLOYMENT_TYPE] {
		newDep := dep.Object.(*appsv1.Deployment)
		glog.V(3).Infof(kwlog(fmt.Sprintf("deleting deployment %s", newDep.ObjectMeta.Name)))
		err := c.Client.AppsV1().Deployments(namespace).Delete(newDep.ObjectMeta.Name, &metav1.DeleteOptions{})
		if err != nil {
			glog.Errorf(kwlog(fmt.Sprintf("unable to delete deployment %s. Error: %v", newDep.ObjectMeta.Name, err)))
		}
	}
	// Delete the serviceaccount types in the cluster
	for _, svcAcctDef := range apiObjMap[K8S_SERVICEACCOUNT_TYPE] {
		newSvcAcct := svcAcctDef.Object.(*corev1.ServiceAccount)
		glog.V(3).Infof(kwlog(fmt.Sprintf("deleting service account %s", newSvcAcct.ObjectMeta.Name)))
		err := c.Client.CoreV1().ServiceAccounts(namespace).Delete(newSvcAcct.ObjectMeta.Name, &metav1.DeleteOptions{})
		if err != nil {
			glog.Errorf(kwlog(fmt.Sprintf("unable to delete service account %s. Error: %v", newSvcAcct.ObjectMeta.Name, err)))
		}
	}
	// Delete the rolebinding types in the cluster
	for _, roleBindingDef := range apiObjMap[K8S_ROLEBINDING_TYPE] {
		newRoleBinding := roleBindingDef.Object.(*rbacv1.RoleBinding)
		glog.V(3).Infof(kwlog(fmt.Sprintf("deleting role binding %s", newRoleBinding.ObjectMeta.Name)))
		err := c.Client.RbacV1().RoleBindings(namespace).Delete(newRoleBinding.ObjectMeta.Name, &metav1.DeleteOptions{})
		if err != nil {
			glog.Errorf(kwlog(fmt.Sprintf("unable to delete role binding %s. Error: %v", newRoleBinding.ObjectMeta.Name, err)))
		}
	}
	// Delete the role types in the cluster
	for _, roleDef := range apiObjMap[K8S_ROLE_TYPE] {
		newRole := roleDef.Object.(*rbacv1.Role)
		glog.V(3).Infof(kwlog(fmt.Sprintf("deleting role %s", newRole.ObjectMeta.Name)))
		err := c.Client.RbacV1().Roles(namespace).Delete(newRole.ObjectMeta.Name, &metav1.DeleteOptions{})
		if err != nil {
			glog.Errorf(kwlog(fmt.Sprintf("unable to delete role %s. Error: %v", newRole.ObjectMeta.Name, err)))
		}
	}
	for _, namespaceDef := range apiObjMap[K8S_NAMESPACE_TYPE] {
		newNs := namespaceDef.Object.(*corev1.Namespace)
		glog.V(3).Infof(kwlog(fmt.Sprintf("deleting namespace %v", newNs)))
		err := c.Client.CoreV1().Namespaces().Delete(newNs.ObjectMeta.Name, &metav1.DeleteOptions{})
		if err != nil {
			glog.Errorf(kwlog(fmt.Sprintf("unable to delete namespace %s. Error: %v", newNs.ObjectMeta.Name, err)))
		}
	}
	glog.V(3).Infof(kwlog(fmt.Sprintf("Completed removal of all operator objects from the cluster.")))
	return nil
}
func (c KubeClient) OperatorStatus(tar string) (interface{}, error) {
	// Read the yaml files from the commpressed tar files
	yamls, err := getYamlFromTarGz(tar)
	if err != nil {
		return nil, err
	}
	// Convert the yaml files to kubernetes objects
	k8sObjs, customResources, err := getK8sObjectFromYaml(yamls, nil)
	if err != nil {
		return nil, err
	}
	// Sort the k8s api objects by kind
	apiObjMap := sortAPIObjects(k8sObjs)

	_, namespace, err := getOperatorNamespace(apiObjMap)
	if err != nil {
		return nil, err
	}

	kindToGVRMap := map[string]schema.GroupVersionResource{}
	for _, crd := range apiObjMap[K8S_CRD_TYPE] {
		crdDef := crd.Object.(*v1beta1.CustomResourceDefinition)

		kindToGVRMap[crdDef.Spec.Names.Kind] = schema.GroupVersionResource{Resource: crdDef.Spec.Names.Plural, Group: crdDef.Spec.Group, Version: crdDef.Spec.Version}
	}

	if len(customResources) != 1 {
		return nil, fmt.Errorf("Expected one custom resource in deployment. Got %d", len(customResources))
	}

	crStr := customResources[0]
	cr := make(map[string]interface{})
	err1 := yaml.UnmarshalStrict([]byte(crStr.Body), &cr)
	if err1 != nil {
		return nil, fmt.Errorf("Error unmarshaling custom resource in deployment. %v", err1)
	}
	crMap := makeAllKeysStrings(cr).(map[string]interface{})

	dynClient, err := NewDynamicKubeClient()
	if err != nil {
		return nil, err
	}
	crClient := dynClient.Resource(kindToGVRMap[fmt.Sprintf("%v", crMap["kind"])])
	name := fmt.Sprintf("%v", crMap["metadata"].(map[string]interface{})["name"])
	res, err := crClient.Namespace(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if status, ok := res.Object["status"]; ok {
		return status, nil
	} else {
		return nil, fmt.Errorf("Error status not found")
	}

}
func (c KubeClient) Status(tar string) ([]ContainerStatus, error) {
	// Read the yaml files from the commpressed tar files
	yamls, err := getYamlFromTarGz(tar)
	if err != nil {
		return nil, err
	}
	// Convert the yaml files to kubernetes objects
	k8sObjs, _, err := getK8sObjectFromYaml(yamls, nil)
	if err != nil {
		return nil, err
	}
	// Sort the k8s api objects by kind
	apiObjMap := sortAPIObjects(k8sObjs)

	deploymentUnstruct := apiObjMap[K8S_DEPLOYMENT_TYPE]
	deploymentObj := deploymentUnstruct[0].Object.(*appsv1.Deployment)
	opName := deploymentObj.Spec.Template.ObjectMeta.Labels["name"]

	podList, err := c.Client.CoreV1().Pods(ANAX_NAMESPACE).List(metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", "name", opName)})
	if err != nil {
		return nil, err
	}
	if len(podList.Items) < 1 {
		return nil, nil
	}
	pod := podList.Items[0]
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
}

// CreateConfigMap will create a config map with the provided environment variable map
func (c KubeClient) CreateConfigMap(envVars map[string]string, agId string, namespace string) (string, error) {
	hznEnvConfigMap := corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-%s", HZN_ENV_VARS, agId)}, Data: envVars}
	res, err := c.Client.CoreV1().ConfigMaps(namespace).Create(&hznEnvConfigMap)
	if err != nil {
		return "", fmt.Errorf("Error: failed to create config map for %s: %v", agId, err)
	}
	return res.ObjectMeta.Name, nil
}

func getOperatorNamespace(allObjects map[string][]APIObjects) (*corev1.Namespace, string, error) {
	namespace := ANAX_NAMESPACE
	var nsObj *corev1.Namespace
	if len(allObjects[K8S_NAMESPACE_TYPE]) > 1 {
		return nsObj, "", fmt.Errorf("Error: only 1 namespace yaml object can be given in the operator deployment.")
	}
	if len(allObjects[K8S_NAMESPACE_TYPE]) == 1 {
		typedNs := allObjects[K8S_NAMESPACE_TYPE][0].Object.(*corev1.Namespace)
		if typedNs.ObjectMeta.Name != "" {
			nsObj = typedNs
			namespace = typedNs.ObjectMeta.Name
		} else {
			return nsObj, "", fmt.Errorf("Error: namespace yaml with no name specified.")
		}

	}
	// Check the deployment objects to see if a alternative namespace is given
	for _, dep := range allObjects[K8S_DEPLOYMENT_TYPE] {
		newDep := dep.Object.(*appsv1.Deployment)
		if newDep.ObjectMeta.Namespace != "" && newDep.ObjectMeta.Namespace != namespace {
			if namespace != ANAX_NAMESPACE {
				return nsObj, "", fmt.Errorf("Mismatched namespaces specified %s and %s", newDep.ObjectMeta.Namespace, namespace)
			} else {
				namespace = newDep.ObjectMeta.Namespace
				nsObj = &corev1.Namespace{TypeMeta: metav1.TypeMeta{Kind: "Namespace"}, ObjectMeta: metav1.ObjectMeta{Name: namespace}}
			}
		}
	}
	return nsObj, namespace, nil
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
	}
	return unmarshYaml
}

// Sort a slice of k8s api objects by kind of object
func sortAPIObjects(allObjects []APIObjects) map[string][]APIObjects {
	objMap := map[string][]APIObjects{}
	for _, obj := range allObjects {
		switch obj.Type.Kind {
		case K8S_NAMESPACE_TYPE:
			objMap[K8S_NAMESPACE_TYPE] = append(objMap[K8S_NAMESPACE_TYPE], obj)
		case K8S_ROLE_TYPE:
			objMap[K8S_ROLE_TYPE] = append(objMap[K8S_ROLE_TYPE], obj)
		case K8S_ROLEBINDING_TYPE:
			objMap[K8S_ROLEBINDING_TYPE] = append(objMap[K8S_ROLEBINDING_TYPE], obj)
		case K8S_DEPLOYMENT_TYPE:
			objMap[K8S_DEPLOYMENT_TYPE] = append(objMap[K8S_DEPLOYMENT_TYPE], obj)
		case K8S_SERVICEACCOUNT_TYPE:
			objMap[K8S_SERVICEACCOUNT_TYPE] = append(objMap[K8S_SERVICEACCOUNT_TYPE], obj)
		case K8S_CRD_TYPE:
			objMap[K8S_CRD_TYPE] = append(objMap[K8S_CRD_TYPE], obj)
		default:
		}

	}
	return objMap
}

// Convert the given yaml files into k8s api objects
func getK8sObjectFromYaml(yamlFiles []YamlFile, sch *runtime.Scheme) ([]APIObjects, []YamlFile, error) {
	retObjects := []APIObjects{}
	customResources := []YamlFile{}

	if sch == nil {
		sch = runtime.NewScheme()
	}

	// This is required to allow the schema to recognize custom resource definition types
	_ = v1beta1.AddToScheme(sch)
	_ = scheme.AddToScheme(sch)

	for _, fileStr := range yamlFiles {
		decode := serializer.NewCodecFactory(sch).UniversalDeserializer().Decode
		obj, gvk, err := decode([]byte(fileStr.Body), nil, nil)

		if err != nil {
			customResources = append(customResources, fileStr)
		} else {
			// If the object can not be recognized, return the yaml file
			newObj := APIObjects{Type: gvk, Object: obj}
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
