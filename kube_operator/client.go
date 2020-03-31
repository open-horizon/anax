package kube_operator

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
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
	rest "k8s.io/client-go/rest"
	"reflect"
	"strings"
	"time"
)

const (
	// TEST NAMESPACE
	ANAX_NAMESPACE = "ibm-edge"

	K8S_ROLE_TYPE           = "Role"
	K8S_ROLEBINDING_TYPE    = "RoleBinding"
	K8S_DEPLOYMENT_TYPE     = "Deployment"
	K8S_SERVICEACCOUNT_TYPE = "ServiceAccount"
	K8S_CRD_TYPE            = "CustomResourceDefinition"
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

func NewKubeConfig() (*rest.Config, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("Failed to get cluster config information: %v", err)
	}
	return config, nil
}

func NewKubeClient() (*KubeClient, error) {
	config, err := NewKubeConfig()
	if err != nil {
		return nil, err
	}
	clientset, _ := kubernetes.NewForConfig(config)
	return &KubeClient{Client: clientset}, nil
}

// NewDynamicKubeClient returns a kube client that interacts with unstructured.Unstructured type objects
func NewDynamicKubeClient() (dynamic.Interface, error) {
	config, err := NewKubeConfig()
	if err != nil {
		return nil, err
	}
	clientset, _ := dynamic.NewForConfig(config)
	return clientset, nil
}

func NewCRDClient() (*apiv1beta1client.ApiextensionsV1beta1Client, error) {
	config, err := NewKubeConfig()
	if err != nil {
		return nil, err
	}
	clientset, _ := apiv1beta1client.NewForConfig(config)
	return clientset, nil
}

// Install creates the objects specified in the operator deployment in the cluster and creates the custom resource to start the operator
func (c KubeClient) Install(tar string, imageName string) error {
	// Read the yaml files from the commpressed tar files
	r := strings.NewReader(tar)
	yamls, err := getYamlFromTarGz(r)
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

	// Create the role types in the cluster
	for _, roleDef := range apiObjMap[K8S_ROLE_TYPE] {
		newRole := roleDef.Object.(*rbacv1.Role)
		_, err := c.Client.RbacV1().Roles(ANAX_NAMESPACE).Create(newRole)
		if err != nil {
			return err
		}
	}
	// Create the rolebinding types in the cluster
	for _, roleBindingDef := range apiObjMap[K8S_ROLEBINDING_TYPE] {
		newRoleBinding := roleBindingDef.Object.(*rbacv1.RoleBinding)
		_, err := c.Client.RbacV1().RoleBindings(ANAX_NAMESPACE).Create(newRoleBinding)
		if err != nil {
			return err
		}
	}
	// Create the serviceaccount types in the cluster
	for _, svcAcctDef := range apiObjMap[K8S_SERVICEACCOUNT_TYPE] {
		newSvcAcct := svcAcctDef.Object.(*corev1.ServiceAccount)
		_, err := c.Client.CoreV1().ServiceAccounts(ANAX_NAMESPACE).Create(newSvcAcct)
		if err != nil {
			return err
		}
	}
	// Create the deployment types in the cluster
	for _, dep := range apiObjMap[K8S_DEPLOYMENT_TYPE] {
		newDep := dep.Object.(*appsv1.Deployment)
		_, err := c.Client.AppsV1().Deployments(ANAX_NAMESPACE).Create(newDep)
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
		_, err = crds.Create(newCRD)
		if err != nil {
			return err
		}
		kindToGVRMap[newCRD.Spec.Names.Kind] = schema.GroupVersionResource{Resource: newCRD.Spec.Names.Plural, Group: newCRD.Spec.Group, Version: newCRD.Spec.Version}
	}

	// Create the custom resources in the cluster
	for _, crStr := range customResources {
		cr := make(map[string]interface{})
		yaml.UnmarshalStrict([]byte(crStr.Body), &cr)

		newCr := makeAllKeysStrings(cr).(map[string]interface{})

		dynClient, err := NewDynamicKubeClient()
		if err != nil {
			return err
		}
		crClient := dynClient.Resource(kindToGVRMap[newCr["kind"].(string)])

		unstructCr := unstructured.Unstructured{Object: newCr}

		// the cluster has to create the endpoint for the custom resource, this can take some time. giving it 90 seconds for now. should probably be configurable
		i := 0
		for i < 3 {
			_, err := crClient.Namespace(ANAX_NAMESPACE).Create(&unstructCr, metav1.CreateOptions{})
			if err != nil {
				if i == 2 {
					return err
				}
				time.Sleep(time.Second * 30)
			} else {
				break
			}

			i++
		}
	}

	return nil
}

// Install creates the objects specified in the operator deployment in the cluster and creates the custom resource to start the operator
func (c KubeClient) Uninstall(tar string, imageName string) error {
	// Read the yaml files from the commpressed tar files
	r := strings.NewReader(tar)
	yamls, err := getYamlFromTarGz(r)
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

	// Delete the custom resource definitions from the cluster
	kindToGVRMap := map[string]schema.GroupVersionResource{}
	for _, crd := range apiObjMap[K8S_CRD_TYPE] {
		newCRD := crd.Object.(*v1beta1.CustomResourceDefinition)

		// CRD's need a different client
		apiClient, err := NewCRDClient()
		if err != nil {
			return err
		}
		crds := apiClient.CustomResourceDefinitions()
		err = crds.Delete(newCRD.ObjectMeta.Name, &metav1.DeleteOptions{})
		if err != nil {
			return err
		}
		kindToGVRMap[newCRD.Spec.Names.Kind] = schema.GroupVersionResource{Resource: newCRD.Spec.Names.Plural, Group: newCRD.Spec.Group, Version: newCRD.Spec.Version}
	}

	// Delete the custom resources in the cluster
	for _, crStr := range customResources {
		cr := make(map[string]interface{})
		yaml.UnmarshalStrict([]byte(crStr.Body), &cr)

		newCr := makeAllKeysStrings(cr).(map[string]interface{})

		dynClient, err := NewDynamicKubeClient()
		if err != nil {
			return err
		}
		crClient := dynClient.Resource(kindToGVRMap[newCr["kind"].(string)])

		// the cluster has to create the endpoint for the custom resource, this can take some time. giving it 90 seconds for now. should probably be configurable
		err = crClient.Namespace(ANAX_NAMESPACE).Delete(newCr["metadata"].(map[string]interface{})["name"].(string), &metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}
	// Delete the deployment types in the cluster
	for _, dep := range apiObjMap[K8S_DEPLOYMENT_TYPE] {
		newDep := dep.Object.(*appsv1.Deployment)
		err := c.Client.AppsV1().Deployments(ANAX_NAMESPACE).Delete(newDep.ObjectMeta.Name, &metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}
	// Delete the serviceaccount types in the cluster
	for _, svcAcctDef := range apiObjMap[K8S_SERVICEACCOUNT_TYPE] {
		newSvcAcct := svcAcctDef.Object.(*corev1.ServiceAccount)
		err := c.Client.CoreV1().ServiceAccounts(ANAX_NAMESPACE).Delete(newSvcAcct.ObjectMeta.Name, &metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}
	// Delete the rolebinding types in the cluster
	for _, roleBindingDef := range apiObjMap[K8S_ROLEBINDING_TYPE] {
		newRoleBinding := roleBindingDef.Object.(*rbacv1.RoleBinding)
		err := c.Client.RbacV1().RoleBindings(ANAX_NAMESPACE).Delete(newRoleBinding.ObjectMeta.Name, &metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}
	// Delete the role types in the cluster
	for _, roleDef := range apiObjMap[K8S_ROLE_TYPE] {
		newRole := roleDef.Object.(*rbacv1.Role)
		err := c.Client.RbacV1().Roles(ANAX_NAMESPACE).Delete(newRole.ObjectMeta.Name, &metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}
	return nil
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
func getYamlFromTarGz(r io.Reader) ([]YamlFile, error) {
	files := []YamlFile{}
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
