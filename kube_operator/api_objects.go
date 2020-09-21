package kube_operator

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	crdv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	crdv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiv1client "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	apiv1beta1client "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"time"
)

type APIObjectInterface interface {
	Install(c KubeClient, namespace string) error
	Uninstall(c KubeClient, namespace string)
	Status(c KubeClient, namespace string) (interface{}, error)
	Name() string
}

// Sort a slice of k8s api objects by kind of object
// Returns a map of object type names to api object interfaces types, the namespace to be used for the operator, and an error if one occurs
// Also verifies that all objects are named so they can be found and uninstalled
func sortAPIObjects(allObjects []APIObjects, customResource *unstructured.Unstructured, envVarMap map[string]string, agreementId string) (map[string][]APIObjectInterface, string, error) {
	namespace := ""
	objMap := map[string][]APIObjectInterface{}
	for _, obj := range allObjects {
		switch obj.Type.Kind {
		case K8S_NAMESPACE_TYPE:
			if typedNS, ok := obj.Object.(*corev1.Namespace); ok {
				newNs := NamespaceCoreV1{NamespaceObject: typedNS}
				if newNs.Name() != "" {
					glog.V(4).Infof(kwlog(fmt.Sprintf("Found kubernetes namespace object %s.", newNs.Name())))
					objMap[K8S_NAMESPACE_TYPE] = append(objMap[K8S_NAMESPACE_TYPE], newNs)
				} else {
					return objMap, namespace, fmt.Errorf(kwlog(fmt.Sprintf("Error: namespace object must have a name in its metadata section.")))
				}
				if namespace != "" && namespace != typedNS.ObjectMeta.Name {
					return objMap, namespace, fmt.Errorf(kwlog(fmt.Sprintf("Error: multiple namespaces specified in operator : %s and %s", namespace, typedNS.ObjectMeta.Name)))
				}
				namespace = typedNS.ObjectMeta.Name
			} else {
				return objMap, namespace, fmt.Errorf(kwlog(fmt.Sprintf("Error: namespace object has unrecognized type %T: %v", obj.Object, obj.Object)))
			}
		case K8S_ROLE_TYPE:
			if typedRole, ok := obj.Object.(*rbacv1.Role); ok {
				newRole := RoleRbacV1{RoleObject: typedRole}
				if newRole.Name() != "" {
					glog.V(4).Infof(kwlog(fmt.Sprintf("Found kubernetes role object %s.", newRole.Name())))
					objMap[K8S_ROLE_TYPE] = append(objMap[K8S_ROLE_TYPE], newRole)
				} else {
					return objMap, namespace, fmt.Errorf(kwlog(fmt.Sprintf("Error: role object must have a name in its metadata section.")))
				}
			} else {
				return objMap, namespace, fmt.Errorf(kwlog(fmt.Sprintf("Error: role object has unrecognized type %T: %v", obj.Object, obj.Object)))
			}
		case K8S_ROLEBINDING_TYPE:
			if typedRoleBinding, ok := obj.Object.(*rbacv1.RoleBinding); ok {
				newRolebinding := RolebindingRbacV1{RolebindingObject: typedRoleBinding}
				if newRolebinding.Name() != "" {
					glog.V(4).Infof(kwlog(fmt.Sprintf("Found kubernetes rolebinding object %s.", newRolebinding.Name())))
					objMap[K8S_ROLEBINDING_TYPE] = append(objMap[K8S_ROLEBINDING_TYPE], newRolebinding)
				} else {
					return objMap, namespace, fmt.Errorf(kwlog(fmt.Sprintf("Error: rolebinding object must have a name in its metadata section.")))
				}
			} else {
				return objMap, namespace, fmt.Errorf(kwlog(fmt.Sprintf("Error: rolebinding object has unrecognized type %T: %v", obj.Object, obj.Object)))
			}
		case K8S_DEPLOYMENT_TYPE:
			if typedDeployment, ok := obj.Object.(*appsv1.Deployment); ok {
				if typedDeployment.ObjectMeta.Namespace != "" {
					if namespace == "" {
						namespace = typedDeployment.ObjectMeta.Namespace
					} else if namespace != "" && namespace != typedDeployment.ObjectMeta.Namespace {
						return objMap, namespace, fmt.Errorf(kwlog(fmt.Sprintf("Error: multiple namespaces specified in operator: %s and %s", namespace, typedDeployment.ObjectMeta.Namespace)))
					}
				}
				newDeployment := DeploymentAppsV1{DeploymentObject: typedDeployment, EnvVarMap: envVarMap, AgreementId: agreementId}
				if newDeployment.Name() != "" {
					glog.V(4).Infof(kwlog(fmt.Sprintf("Found kubernetes deployment object %s.", newDeployment.Name())))
					objMap[K8S_DEPLOYMENT_TYPE] = append(objMap[K8S_DEPLOYMENT_TYPE], newDeployment)
				} else {
					return objMap, namespace, fmt.Errorf(kwlog(fmt.Sprintf("Error: deployment object must have a name in its metadata section.")))
				}
			} else {
				return objMap, namespace, fmt.Errorf(kwlog(fmt.Sprintf("Error: deployment object has unrecognized type %T: %v", obj.Object, obj.Object)))
			}
		case K8S_SERVICEACCOUNT_TYPE:
			if typedServiceAccount, ok := obj.Object.(*corev1.ServiceAccount); ok {
				newServiceAccount := ServiceAccountCoreV1{ServiceAccountObject: typedServiceAccount}
				if newServiceAccount.Name() != "" {
					glog.V(4).Infof(kwlog(fmt.Sprintf("Found kubernetes service account object %s.", newServiceAccount.Name())))
					objMap[K8S_SERVICEACCOUNT_TYPE] = append(objMap[K8S_SERVICEACCOUNT_TYPE], newServiceAccount)
				} else {
					return objMap, namespace, fmt.Errorf(kwlog(fmt.Sprintf("Error: service account object must have a name in its metadata section.")))
				}
			} else {
				return objMap, namespace, fmt.Errorf(kwlog(fmt.Sprintf("Error: service account object has unrecognized type %T: %v", obj.Object, obj.Object)))
			}
		case K8S_CRD_TYPE:
			if typedCRD, ok := obj.Object.(*crdv1beta1.CustomResourceDefinition); ok {
				newCustomResource := CustomResourceV1Beta1{CustomResourceDefinitionObject: typedCRD, CustomResourceObject: customResource}
				if newCustomResource.Name() != "" {
					glog.V(4).Infof(kwlog(fmt.Sprintf("Found kubernetes custom resource definition object %s.", newCustomResource.Name())))
					objMap[K8S_CRD_TYPE] = append(objMap[K8S_CRD_TYPE], newCustomResource)
				} else {
					return objMap, namespace, fmt.Errorf(kwlog(fmt.Sprintf("Error: custom resource definition object must have a name in its metadata section.")))
				}
			} else if typedCRD, ok := obj.Object.(*crdv1.CustomResourceDefinition); ok {
				objMap[K8S_CRD_TYPE] = append(objMap[K8S_CRD_TYPE], CustomResourceV1{CustomResourceDefinitionObject: typedCRD, CustomResourceObject: customResource})
			} else {
				return objMap, namespace, fmt.Errorf(kwlog(fmt.Sprintf("Error: custom resource definition object has unrecognized type %T: %v", obj.Object, obj.Object)))
			}

		}

	}
	if namespace == "" {
		namespace = ANAX_NAMESPACE
	}

	return objMap, namespace, nil
}

//----------------Namespace----------------

type NamespaceCoreV1 struct {
	NamespaceObject *corev1.Namespace
}

func (n NamespaceCoreV1) Install(c KubeClient, namespace string) error {
	glog.V(3).Infof(kwlog(fmt.Sprintf("attempting to create namespace %v", n.NamespaceObject)))
	_, err := c.Client.CoreV1().Namespaces().Create(n.NamespaceObject)
	if err != nil {
		// If the namespace already exists this is not a problem
		glog.Warningf(kwlog(fmt.Sprintf("Failed to create namespace %s. Continuing with installation.", n.Name())))
	}
	return nil
}

func (n NamespaceCoreV1) Uninstall(c KubeClient, namespace string) {
	glog.V(3).Infof(kwlog(fmt.Sprintf("deleting namespace %v", n.NamespaceObject)))
	err := c.Client.CoreV1().Namespaces().Delete(n.Name(), &metav1.DeleteOptions{})
	if err != nil {
		glog.Errorf(kwlog(fmt.Sprintf("unable to delete namespace %s. Error: %v", n.Name(), err)))
	}
}

func (n NamespaceCoreV1) Status(c KubeClient, namespace string) (interface{}, error) {
	nsStatus, err := c.Client.CoreV1().Namespaces().Get(n.Name(), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf(kwlog(fmt.Sprintf("Error getting namespace status: %v", err)))
	}
	return nsStatus, nil
}

func (n NamespaceCoreV1) Name() string {
	return n.NamespaceObject.ObjectMeta.Name
}

//----------------Role----------------

type RoleRbacV1 struct {
	RoleObject *rbacv1.Role
}

func (r RoleRbacV1) Install(c KubeClient, namespace string) error {
	glog.V(3).Infof(kwlog(fmt.Sprintf("creating role %v", r)))
	_, err := c.Client.RbacV1().Roles(namespace).Create(r.RoleObject)
	if err != nil && errors.IsAlreadyExists(err) {
		r.Uninstall(c, namespace)
		_, err = c.Client.RbacV1().Roles(namespace).Create(r.RoleObject)
	}
	if err != nil {
		return fmt.Errorf(kwlog(fmt.Sprintf("Error creating the cluster role: %v", err)))
	}
	return nil
}

func (r RoleRbacV1) Uninstall(c KubeClient, namespace string) {
	glog.V(3).Infof(kwlog(fmt.Sprintf("deleting role %s", r.Name())))
	err := c.Client.RbacV1().Roles(namespace).Delete(r.Name(), &metav1.DeleteOptions{})
	if err != nil {
		glog.Errorf(kwlog(fmt.Sprintf("unable to delete role %s. Error: %v", r.Name(), err)))
	}
}

func (r RoleRbacV1) Status(c KubeClient, namespace string) (interface{}, error) {
	return nil, nil
}

func (r RoleRbacV1) Name() string {
	return r.RoleObject.ObjectMeta.Name
}

//----------------Rolebinding----------------

type RolebindingRbacV1 struct {
	RolebindingObject *rbacv1.RoleBinding
}

func (rb RolebindingRbacV1) Install(c KubeClient, namespace string) error {
	glog.V(3).Infof(kwlog(fmt.Sprintf("creating rolebinding %v", rb)))
	_, err := c.Client.RbacV1().RoleBindings(namespace).Create(rb.RolebindingObject)
	if err != nil && errors.IsAlreadyExists(err) {
		rb.Uninstall(c, namespace)
		_, err = c.Client.RbacV1().RoleBindings(namespace).Create(rb.RolebindingObject)
	}
	if err != nil {
		return fmt.Errorf(kwlog(fmt.Sprintf("Error creating the cluster rolebinding: %v", err)))
	}
	return nil
}

func (rb RolebindingRbacV1) Uninstall(c KubeClient, namespace string) {
	glog.V(3).Infof(kwlog(fmt.Sprintf("deleting role binding %s", rb.RolebindingObject.ObjectMeta.Name)))
	err := c.Client.RbacV1().RoleBindings(namespace).Delete(rb.RolebindingObject.ObjectMeta.Name, &metav1.DeleteOptions{})
	if err != nil {
		glog.Errorf(kwlog(fmt.Sprintf("unable to delete role binding %s. Error: %v", rb.RolebindingObject.ObjectMeta.Name, err)))
	}
}

func (rb RolebindingRbacV1) Status(c KubeClient, namespace string) (interface{}, error) {
	return nil, nil
}

func (rb RolebindingRbacV1) Name() string {
	return rb.RolebindingObject.ObjectMeta.Name
}

//----------------ServiceAccount----------------
type ServiceAccountCoreV1 struct {
	ServiceAccountObject *corev1.ServiceAccount
}

func (sa ServiceAccountCoreV1) Install(c KubeClient, namespace string) error {
	glog.V(3).Infof(kwlog(fmt.Sprintf("creating service account %v", sa)))
	_, err := c.Client.CoreV1().ServiceAccounts(namespace).Create(sa.ServiceAccountObject)
	if err != nil && errors.IsAlreadyExists(err) {
		sa.Uninstall(c, namespace)
		_, err = c.Client.CoreV1().ServiceAccounts(namespace).Create(sa.ServiceAccountObject)
	}
	if err != nil {
		return fmt.Errorf(kwlog(fmt.Sprintf("Error creating the cluster service account: %v", err)))
	}
	return nil
}

func (sa ServiceAccountCoreV1) Uninstall(c KubeClient, namespace string) {
	glog.V(3).Infof(kwlog(fmt.Sprintf("deleting service account %s", sa.ServiceAccountObject.ObjectMeta.Name)))
	err := c.Client.CoreV1().ServiceAccounts(namespace).Delete(sa.ServiceAccountObject.ObjectMeta.Name, &metav1.DeleteOptions{})
	if err != nil {
		glog.Errorf(kwlog(fmt.Sprintf("unable to delete service account %s. Error: %v", sa.ServiceAccountObject.ObjectMeta.Name, err)))
	}
}

func (sa ServiceAccountCoreV1) Status(c KubeClient, namespace string) (interface{}, error) {
	return nil, nil
}

func (sa ServiceAccountCoreV1) Name() string {
	return sa.ServiceAccountObject.ObjectMeta.Name
}

//----------------Deployment----------------
// The deployment object includes the environment variable config map

type DeploymentAppsV1 struct {
	DeploymentObject *appsv1.Deployment
	EnvVarMap        map[string]string
	AgreementId      string
}

func (d DeploymentAppsV1) Install(c KubeClient, namespace string) error {
	glog.V(3).Infof(kwlog(fmt.Sprintf("creating deployment %v", d)))

	// The ESS is not supported in edge cluster services, so for now, remove the ESS env vars.
	envAdds := cutil.RemoveESSEnvVars(d.EnvVarMap, config.ENVVAR_PREFIX)

	// Create the config map.
	mapName, err := c.CreateConfigMap(envAdds, d.AgreementId, namespace)
	if err != nil && errors.IsAlreadyExists(err) {
		d.Uninstall(c, namespace)
		mapName, err = c.CreateConfigMap(envAdds, d.AgreementId, namespace)
	}
	if err != nil {
		return err
	}

	// Let the operator know about the config map
	dWithEnv := addConfigMapVarToDeploymentObject(*d.DeploymentObject, mapName)
	_, err = c.Client.AppsV1().Deployments(namespace).Create(&dWithEnv)
	if err != nil && errors.IsAlreadyExists(err) {
		d.Uninstall(c, namespace)
		mapName, err = c.CreateConfigMap(envAdds, d.AgreementId, namespace)
		_, err = c.Client.AppsV1().Deployments(namespace).Create(&dWithEnv)
	}
	if err != nil {
		return fmt.Errorf(kwlog(fmt.Sprintf("Error creating the operator deployment: %v", err)))
	}
	return nil
}

func (d DeploymentAppsV1) Uninstall(c KubeClient, namespace string) {
	glog.V(3).Infof(kwlog(fmt.Sprintf("deleting deployment %s", d.DeploymentObject.ObjectMeta.Name)))
	err := c.Client.AppsV1().Deployments(namespace).Delete(d.DeploymentObject.ObjectMeta.Name, &metav1.DeleteOptions{})
	if err != nil {
		glog.Errorf(kwlog(fmt.Sprintf("unable to delete deployment %s. Error: %v", d.DeploymentObject.ObjectMeta.Name, err)))
	}

	configMapName := fmt.Sprintf("%s-%s", HZN_ENV_VARS, d.AgreementId)
	glog.V(3).Infof(kwlog(fmt.Sprintf("deleting config map %v", configMapName)))
	// Delete the agreement config map
	err = c.Client.CoreV1().ConfigMaps(namespace).Delete(configMapName, &metav1.DeleteOptions{})
	if err != nil {
		glog.Errorf(kwlog(fmt.Sprintf("unable to delete config map %s. Error: %v", configMapName, err)))
	}
}

// Status will be the status of the operator pod
func (d DeploymentAppsV1) Status(c KubeClient, namespace string) (interface{}, error) {
	opName := d.DeploymentObject.ObjectMeta.Name
	podList, err := c.Client.CoreV1().Pods(ANAX_NAMESPACE).List(metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", "name", opName)})
	if err != nil {
		return nil, err
	}
	return podList, nil
}

func (d DeploymentAppsV1) Name() string {
	return d.DeploymentObject.ObjectMeta.Name
}

//----------------CRD & CR----------------
// A new version requires a new CRD client type and adding the version scheme in getK8sObjectFromYaml

//--------Version v1beta1--------
// NewCRDV1beta1Client returns the client needed to create a CRD in the cluster
func NewCRDV1beta1Client() (*apiv1beta1client.ApiextensionsV1beta1Client, error) {
	config, err := cutil.NewKubeConfig()
	if err != nil {
		return nil, err
	}
	clientset, _ := apiv1beta1client.NewForConfig(config)
	return clientset, nil
}

type CustomResourceV1Beta1 struct {
	CustomResourceDefinitionObject *crdv1beta1.CustomResourceDefinition
	CustomResourceObject           *unstructured.Unstructured
}

func (cr CustomResourceV1Beta1) Install(c KubeClient, namespace string) error {
	apiClient, err := NewCRDV1beta1Client()
	if err != nil {
		return err
	}
	crds := apiClient.CustomResourceDefinitions()
	glog.V(3).Infof(kwlog(fmt.Sprintf("creating custom resource definition %v", cr.CustomResourceDefinitionObject)))
	_, err = crds.Create(cr.CustomResourceDefinitionObject)
	if err != nil && errors.IsAlreadyExists(err) {
		cr.Uninstall(c, namespace)
		_, err = crds.Create(cr.CustomResourceDefinitionObject)
	}
	if err != nil {
		return err
	}

	// Client for creating the CR in the cluster
	dynClient, err := NewDynamicKubeClient()
	if err != nil {
		return err
	}
	gvr, err := cr.gvr()
	if err != nil {
		return err
	}
	crClient := dynClient.Resource(*gvr)

	resourceName := ""
	if typedCrMetadata, ok := cr.CustomResourceObject.Object["metadata"].(map[string]interface{}); ok {
		if name, ok := typedCrMetadata["name"]; ok {
			resourceName = fmt.Sprintf("%v", name)
		}
	}

	// the cluster has to create the endpoint for the custom resource, this can take some time
	// the cr cannot exist without the crd so we don't have to worry about it already existing
	glog.V(3).Infof(kwlog(fmt.Sprintf("creating operator custom resource %v", cr.CustomResourceObject)))
	for {
		_, err = crClient.Namespace(namespace).Create(cr.CustomResourceObject, metav1.CreateOptions{})
		if err != nil {
			glog.Warningf(kwlog(fmt.Sprintf("Failed to create custom resource %s. Trying again in 5s. Error was: %v", resourceName, err)))
			time.Sleep(time.Second * 5)
		} else {
			glog.V(3).Infof(kwlog(fmt.Sprintf("Sucessfully created custom resource %s.", resourceName)))
			break
		}
	}

	return nil
}

func (cr CustomResourceV1Beta1) Uninstall(c KubeClient, namespace string) {
	dynClient, err := NewDynamicKubeClient()
	if err != nil {
		glog.Errorf(kwlog(fmt.Sprintf("Error: unable to get a kubernetes dynamic client for uninstalling the custom resource: %v", err)))
		return
	}
	gvr, err := cr.gvr()
	if err != nil {
		glog.Errorf("%v", err)
		return
	}
	crClient := dynClient.Resource(*gvr)

	var newCrName string
	if metaInterf, ok := cr.CustomResourceObject.Object["metadata"]; ok {
		if metaMap, ok := metaInterf.(map[string]interface{}); ok {
			if metaMapName, ok := metaMap["name"]; ok {
				newCrName = fmt.Sprintf("%v", metaMapName)
			} else {
				glog.Errorf(kwlog(fmt.Sprintf("unable to find operator custom resource name for %v", cr.CustomResourceObject)))
			}
		} else {
			glog.Errorf(kwlog(fmt.Sprintf("unable to find operator custom resource name for %v", cr.CustomResourceObject)))
		}
	} else {
		glog.Errorf(kwlog(fmt.Sprintf("unable to find operator custom resource name for %v", cr.CustomResourceObject)))
	}
	glog.V(3).Infof(kwlog(fmt.Sprintf("deleting operator custom resource %v", newCrName)))

	err = crClient.Namespace(namespace).Delete(newCrName, &metav1.DeleteOptions{})
	if err != nil {
		glog.Warningf(kwlog(fmt.Sprintf("unable to delete operator custom resource %s. Error: %v", newCrName, err)))
	} else {
		err = cr.waitForCRUninstall(c, namespace, 0, newCrName)
		if err != nil {
			glog.Errorf(fmt.Sprintf("%v", err))
		}
	}

	glog.V(3).Infof(kwlog(fmt.Sprintf("deleting operator custom resource definition %v", cr.CustomResourceDefinitionObject.ObjectMeta.Name)))
	// CRDs need a different client
	apiClient, err := NewCRDV1beta1Client()
	if err != nil {
		glog.Errorf(kwlog(fmt.Sprintf("Error: unable to get a kubernetes CustomResourceDefinition client for uninstall: %v", err)))
		return
	}
	crds := apiClient.CustomResourceDefinitions()
	err = crds.Delete(cr.Name(), &metav1.DeleteOptions{})
	if err != nil {
		glog.Errorf(kwlog(fmt.Sprintf("unable to delete operator custom resource definition %s. Error: %v", cr.Name(), err)))
	}
}

func (cr CustomResourceV1Beta1) waitForCRUninstall(c KubeClient, namespace string, timeoutS int, crName string) error {
	status, err := cr.Status(c, namespace)
	if timeoutS < 1 {
		timeoutS = 200
	}
	for timeoutS > 0 {
		if err != nil && status == nil {
			glog.Infof(kwlog(fmt.Sprintf("Custom resource %s removed successfully", crName)))
			return nil
		}
		glog.Infof(kwlog(fmt.Sprintf("Custom Resource %s is not yet down. Pausing for 10 before checking again. Custom resource status is: %v", crName, status)))
		time.Sleep(10 * time.Second)
		status, err = cr.Status(c, namespace)
		timeoutS = timeoutS - 10
	}
	return fmt.Errorf(kwlog(fmt.Sprintf("Error: timeout occured waiting for custom resource %s to be removed. Continuing with uninstall", crName)))
}

// Status returns the status of the operator's service pod. This is a user-defined object
func (cr CustomResourceV1Beta1) Status(c KubeClient, namespace string) (interface{}, error) {
	gvr, err := cr.gvr()
	if err != nil {
		return nil, err
	}
	dynClient, err := NewDynamicKubeClient()
	if err != nil {
		return nil, err
	}

	crClient := dynClient.Resource(*gvr)

	if metadata, ok := cr.CustomResourceObject.Object["metadata"]; ok {
		if metadataTyped, ok := metadata.(map[string]interface{}); ok {
			if name, ok := metadataTyped["name"]; ok {
				res, err := crClient.Namespace(namespace).Get(fmt.Sprintf("%v", name), metav1.GetOptions{})
				if err != nil {
					return nil, err
				}

				if status, ok := res.Object["status"]; ok {
					return status, nil
				} else {
					return nil, fmt.Errorf("Error status not found")
				}
			}
		}
	}

	return nil, fmt.Errorf(kwlog(fmt.Sprintf("Error: failed to find operator name to report status.")))
}

func (cr CustomResourceV1Beta1) Name() string {
	return cr.CustomResourceDefinitionObject.ObjectMeta.Name
}

// gvr is the group version resource that allows the dynamic client to interact with types defined by custom resource definitions
func (cr CustomResourceV1Beta1) gvr() (*schema.GroupVersionResource, error) {
	if apiVers, ok := cr.CustomResourceObject.Object["apiVersion"]; ok {
		if typedApiVers, ok := apiVers.(string); ok {
			gv, err := schema.ParseGroupVersion(typedApiVers)
			if err != nil {
				return nil, fmt.Errorf(kwlog(fmt.Sprintf("Error: failed to parse group version %s: %v", typedApiVers, err)))
			}
			gvr := schema.GroupVersionResource{Resource: cr.CustomResourceDefinitionObject.Spec.Names.Plural, Group: gv.Group, Version: gv.Version}
			return &gvr, nil
		} else {
			return nil, fmt.Errorf(kwlog(fmt.Sprintf("Error: apiversion field is not of type string %v", cr.CustomResourceObject.Object)))
		}

	} else {
		return nil, fmt.Errorf(kwlog(fmt.Sprintf("Error: custom resource object does not have apiversion field: %v", cr.CustomResourceObject.Object)))
	}
}

//--------Version v1--------
// NewCRDV1Client returns a client that can be used to interact with custom resource definitions in the cluster
func NewCRDV1Client() (*apiv1client.ApiextensionsV1Client, error) {
	config, err := cutil.NewKubeConfig()
	if err != nil {
		return nil, err
	}
	clientset, _ := apiv1client.NewForConfig(config)
	return clientset, nil
}

type CustomResourceV1 struct {
	CustomResourceDefinitionObject *crdv1.CustomResourceDefinition
	CustomResourceObject           *unstructured.Unstructured
}

func (cr CustomResourceV1) Install(c KubeClient, namespace string) error {
	apiClient, err := NewCRDV1Client()
	if err != nil {
		return err
	}
	crds := apiClient.CustomResourceDefinitions()
	glog.V(3).Infof(kwlog(fmt.Sprintf("creating custom resource definition %v", cr.CustomResourceDefinitionObject)))
	_, err = crds.Create(cr.CustomResourceDefinitionObject)
	if err != nil && errors.IsAlreadyExists(err) {
		cr.Uninstall(c, namespace)
		_, err = crds.Create(cr.CustomResourceDefinitionObject)
	}
	if err != nil {
		return fmt.Errorf(kwlog(fmt.Sprintf("Error: failed to create custom resource definition %s: %v", cr.Name(), err)))
	}

	// client for interacting with unknown types including custom resource types
	dynClient, err := NewDynamicKubeClient()
	if err != nil {
		return err
	}
	gvr, err := cr.gvr()
	if err != nil {
		return err
	}
	crClient := dynClient.Resource(*gvr)

	resourceName := ""
	if typedCrMetadata, ok := cr.CustomResourceObject.Object["metadata"].(map[string]interface{}); ok {
		if name, ok := typedCrMetadata["name"]; ok {
			resourceName = fmt.Sprintf("%v", name)
		}
	}

	// the cluster has to create the endpoint for the custom resource, this can take some time
	// the cr cannot exist without the crd so we don't have to worry about it already existing
	glog.V(3).Infof(kwlog(fmt.Sprintf("creating operator custom resource %v", cr.CustomResourceObject)))
	for {
		_, err = crClient.Namespace(namespace).Create(cr.CustomResourceObject, metav1.CreateOptions{})
		if err != nil {
			glog.Warningf(kwlog(fmt.Sprintf("Failed to create custom resource %s. Trying again in 5s. Error was: %v", resourceName, err)))
			time.Sleep(time.Second * 5)
		} else {
			glog.V(3).Infof(kwlog("Sucessfully created custom resource."))
			break
		}
	}

	return nil
}

func (cr CustomResourceV1) Uninstall(c KubeClient, namespace string) {
	// Delete the custom resource definitions from the cluster
	glog.V(3).Infof(kwlog(fmt.Sprintf("deleting operator custom resource definition %v", cr.Name())))

	dynClient, err := NewDynamicKubeClient()
	if err != nil {
		glog.Errorf(kwlog(fmt.Sprintf("Error: unable to get a kubernetes dynamic client for uninstalling the custom resource: %v", err)))
		return
	}
	gvr, err := cr.gvr()
	if err != nil {
		glog.Errorf("%v", err)
		return
	}
	crClient := dynClient.Resource(*gvr)

	var newCrName string
	if metaInterf, ok := cr.CustomResourceObject.Object["metadata"]; ok {
		if metaMap, ok := metaInterf.(map[string]interface{}); ok {
			if metaMapName, ok := metaMap["name"]; ok {
				newCrName = fmt.Sprintf("%v", metaMapName)
			} else {
				glog.Errorf(kwlog(fmt.Sprintf("unable to find operator custom resource name for %v", cr.CustomResourceObject)))
			}
		} else {
			glog.Errorf(kwlog(fmt.Sprintf("unable to find operator custom resource name for %v", cr.CustomResourceObject)))
		}
	} else {
		glog.Errorf(kwlog(fmt.Sprintf("unable to find operator custom resource name for %v", cr.CustomResourceObject)))
	}

	glog.V(3).Infof(kwlog(fmt.Sprintf("deleting operator custom resource %v", newCrName)))
	err = crClient.Namespace(namespace).Delete(newCrName, &metav1.DeleteOptions{})
	if err != nil {
		glog.Warningf(kwlog(fmt.Sprintf("unable to delete operator custom resource %s. Error: %v", newCrName, err)))
	} else {
		err = cr.waitForCRUninstall(c, namespace, 0, newCrName)
		if err != nil {
			glog.Errorf(fmt.Sprintf("%v", err))
		}
	}
	// CRDs need a different client
	apiClient, err := NewCRDV1Client()
	if err != nil {
		glog.Errorf(kwlog(fmt.Sprintf("Error: unable to get a kubernetes CustomResourceDefinition client for uninstall: %v", err)))
		return
	}
	crds := apiClient.CustomResourceDefinitions()
	err = crds.Delete(cr.Name(), &metav1.DeleteOptions{})
	if err != nil {
		glog.Errorf(kwlog(fmt.Sprintf("unable to delete operator custom resource definition %s. Error: %v", cr.Name(), err)))
	}
}

func (cr CustomResourceV1) waitForCRUninstall(c KubeClient, namespace string, timeoutS int, crName string) error {
	status, err := cr.Status(c, namespace)
	if timeoutS < 1 {
		timeoutS = 200
	}
	for timeoutS > 0 {
		if err != nil && status == nil {
			glog.Infof(kwlog(fmt.Sprintf("Custom resource %s removed successfully", crName)))
			return nil
		}
		glog.Infof(kwlog(fmt.Sprintf("Custom Resource %s is not yet down. Pausing for 10 before checking again. Custom resource status is: %v", crName, status)))
		time.Sleep(10 * time.Second)
		status, err = cr.Status(c, namespace)
		timeoutS = timeoutS - 10
	}
	return fmt.Errorf(kwlog(fmt.Sprintf("Error: timeout occured waiting for custom resource %s to be removed. Continuing with uninstall", crName)))
}

// Status returns the status of the operator's service pod. This is a user-defined object
func (cr CustomResourceV1) Status(c KubeClient, namespace string) (interface{}, error) {
	dynClient, err := NewDynamicKubeClient()
	if err != nil {
		return nil, fmt.Errorf(kwlog(fmt.Sprintf("Error: failed to get a kubernetes dynamic client: %v", err)))
	}

	gvr, err := cr.gvr()
	if err != nil {
		return nil, err
	}
	crClient := dynClient.Resource(*gvr)

	if metadata, ok := cr.CustomResourceObject.Object["metadata"]; ok {
		if metadataTyped, ok := metadata.(map[string]interface{}); ok {
			if name, ok := metadataTyped["name"]; ok {
				res, err := crClient.Namespace(namespace).Get(fmt.Sprintf("%v", name), metav1.GetOptions{})
				if err != nil {
					return nil, err
				}

				if status, ok := res.Object["status"]; ok {
					return status, nil
				} else {
					return nil, fmt.Errorf("Error status not found")
				}
			}
		}
	}

	return nil, fmt.Errorf(kwlog(fmt.Sprintf("Error: failed to find operator name to report status.")))
}

func (cr CustomResourceV1) Name() string {
	return cr.CustomResourceDefinitionObject.ObjectMeta.Name
}

// group-version-resource is used by the discovry client to interact with a type defined by the custom resource definition
func (cr CustomResourceV1) gvr() (*schema.GroupVersionResource, error) {
	if apiVers, ok := cr.CustomResourceObject.Object["apiVersion"]; ok {
		if typedApiVers, ok := apiVers.(string); ok {
			gv, err := schema.ParseGroupVersion(typedApiVers)
			if err != nil {
				return nil, fmt.Errorf(kwlog(fmt.Sprintf("Error: failed to parse group version %s: %v", typedApiVers, err)))
			}
			gvr := schema.GroupVersionResource{Resource: cr.CustomResourceDefinitionObject.Spec.Names.Plural, Group: gv.Group, Version: gv.Version}
			return &gvr, nil
		} else {
			return nil, fmt.Errorf(kwlog(fmt.Sprintf("Error: apiversion field is not of type string %v", cr.CustomResourceObject.Object)))
		}

	} else {
		return nil, fmt.Errorf(kwlog(fmt.Sprintf("Error: custom resource object does not have apiversion field: %v", cr.CustomResourceObject.Object)))
	}
}
