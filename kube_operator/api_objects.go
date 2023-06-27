package kube_operator

import (
	"context"
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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamic "k8s.io/client-go/dynamic"
	"strings"
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
func sortAPIObjects(allObjects []APIObjects, customResources map[string][]*unstructured.Unstructured, metadata map[string]interface{}, envVarMap map[string]string, agreementId string, crInstallTimeout int64) (map[string][]APIObjectInterface, string, error) {
	namespace := ""

	// get the namespace from metadata
	if metadata != nil {
		if ns, ok := metadata["namespace"]; ok {
			namespace = ns.(string)
		}
	}

	// parse operator
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
				kind := typedCRD.Spec.Names.Kind
				if kind == "" {
					return objMap, namespace, fmt.Errorf(kwlog(fmt.Sprintf("Error: custom resource definition object missing kind field.", obj.Object)))
				}
				customResourceList, ok := customResources[kind]
				if !ok {
					return objMap, namespace, fmt.Errorf(kwlog(fmt.Sprintf("Error: no custom resource object with kind %v found in %v.", kind, customResources)))
				}
				newCustomResource := CustomResourceV1Beta1{CustomResourceDefinitionObject: typedCRD, CustomResourceObjectList: customResourceList, InstallTimeout: crInstallTimeout}
				if newCustomResource.Name() != "" {
					glog.V(4).Infof(kwlog(fmt.Sprintf("Found kubernetes custom resource definition object %s.", newCustomResource.Name())))
					objMap[K8S_CRD_TYPE] = append(objMap[K8S_CRD_TYPE], newCustomResource)
				} else {
					return objMap, namespace, fmt.Errorf(kwlog(fmt.Sprintf("Error: custom resource definition object must have a name in its metadata section.")))
				}
			} else if typedCRD, ok := obj.Object.(*crdv1.CustomResourceDefinition); ok {
				kind := typedCRD.Spec.Names.Kind
				if kind == "" {
					return objMap, namespace, fmt.Errorf(kwlog(fmt.Sprintf("Error: custom resource definition object missing kind field.", obj.Object)))
				}
				customResourceList, ok := customResources[kind]
				if !ok {
					return objMap, namespace, fmt.Errorf(kwlog(fmt.Sprintf("Error: no custom resource object with kind %v found in %v.", kind, customResources)))
				}
				objMap[K8S_CRD_TYPE] = append(objMap[K8S_CRD_TYPE], CustomResourceV1{CustomResourceDefinitionObject: typedCRD, CustomResourceObjectList: customResourceList, InstallTimeout: crInstallTimeout})
			} else {
				return objMap, namespace, fmt.Errorf(kwlog(fmt.Sprintf("Error: custom resource definition object has unrecognized type %T: %v", obj.Object, obj.Object)))
			}
		default:
			// for all other types, convert it to an unstructured object
			if typedOO, ok := obj.Object.(*unstructured.Unstructured); ok {
				newOO := OtherObject{Object: typedOO, GVK: obj.Type}
				glog.V(4).Infof(kwlog(fmt.Sprintf("Found object %v of unstructured type %v", newOO.Name(), obj.Type)))
				objMap[K8S_UNSTRUCTURED_TYPE] = append(objMap[K8S_UNSTRUCTURED_TYPE], newOO)
			} else {
				glog.Errorf(kwlog(fmt.Sprintf("Object with gvk %v has type %T, not unstructured kube type.", obj.Type, obj.Object)))
			}
		}
	}

	return objMap, namespace, nil
}

// ----------------OtherObjectType----------------
// this will only work if the resource name is the kind but lowercase with an "s" on the end
type OtherObject struct {
	Object *unstructured.Unstructured
	GVK    *schema.GroupVersionKind
}

func (o OtherObject) Install(c KubeClient, namespace string) error {
	name := o.Name()
	glog.V(3).Infof(kwlog(fmt.Sprintf("attempting to create object %v with GroupVersionResource %v", name, o.gvr())))

	dynClient := c.DynClient.Resource(o.gvr())

	if _, err1 := dynClient.Namespace(namespace).Create(context.Background(), o.Object, metav1.CreateOptions{}); err1 == nil {
		glog.V(3).Infof(kwlog(fmt.Sprintf("successfully created namespaced object %v with GroupVersionResource %v", name, o.gvr())))
	} else if _, err2 := dynClient.Create(context.Background(), o.Object, metav1.CreateOptions{}); err2 == nil {
		glog.V(3).Infof(kwlog(fmt.Sprintf("successfully created cluster-wide object %v with GroupVersionResource %v", name, o.gvr())))
	} else {
		return fmt.Errorf("%v, %v", err1, err2)
	}

	return nil
}

func (o OtherObject) Uninstall(c KubeClient, namespace string) {
	name := o.Name()
	glog.V(3).Infof(kwlog(fmt.Sprintf("attempting to delete object %v with GroupVersionResource %v", name, o.gvr())))

	dynClient := c.DynClient.Resource(o.gvr())

	if err1 := dynClient.Namespace(namespace).Delete(context.Background(), name, metav1.DeleteOptions{}); err1 == nil {
		glog.V(3).Infof(kwlog(fmt.Sprintf("successfully deleted namespaced object %v with GroupVersionResource %v", name, o.gvr())))
	} else if err2 := dynClient.Delete(context.Background(), name, metav1.DeleteOptions{}); err2 == nil {
		glog.V(3).Infof(kwlog(fmt.Sprintf("successfully deleted cluster-wide object %v with GroupVersionResource %v", name, o.gvr())))
	} else {
		glog.Errorf(kwlog(fmt.Sprintf("Failed to uninstall %v object %v: %v, %v", o.gvr(), name, err1, err2)))
	}

	glog.V(3).Infof(kwlog(fmt.Sprintf("successfully deleted object %v with GroupVersionResource %v", name, o.GVK)))
}

func (o OtherObject) Name() string {
	return o.Object.GetName()
}

func (o OtherObject) Status(c KubeClient, namespace string) (interface{}, error) {
	return nil, nil
}

func (o OtherObject) gvr() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: o.GVK.Group, Version: o.GVK.Version, Resource: fmt.Sprintf("%ss", strings.ToLower(o.GVK.Kind))}
}

//----------------Namespace----------------

type NamespaceCoreV1 struct {
	NamespaceObject *corev1.Namespace
}

func (n NamespaceCoreV1) Install(c KubeClient, namespace string) error {
	glog.V(3).Infof(kwlog(fmt.Sprintf("attempting to create namespace %v", n.NamespaceObject)))
	_, err := c.Client.CoreV1().Namespaces().Create(context.Background(), n.NamespaceObject, metav1.CreateOptions{})
	if err != nil {
		// If the namespace already exists this is not a problem
		glog.Warningf(kwlog(fmt.Sprintf("Failed to create namespace %s. Continuing with installation.", n.Name())))
	}
	return nil
}

func (n NamespaceCoreV1) Uninstall(c KubeClient, namespace string) {
	if namespace == cutil.GetClusterNamespace() {
		glog.V(3).Infof(kwlog(fmt.Sprintf("skipping deletion of namespace used by agent %v", n.NamespaceObject)))
		return
	}
	glog.V(3).Infof(kwlog(fmt.Sprintf("deleting namespace %v", n.NamespaceObject)))
	err := c.Client.CoreV1().Namespaces().Delete(context.Background(), n.Name(), metav1.DeleteOptions{})
	if err != nil {
		glog.Errorf(kwlog(fmt.Sprintf("unable to delete namespace %s. Error: %v", n.Name(), err)))
	}
}

func (n NamespaceCoreV1) Status(c KubeClient, namespace string) (interface{}, error) {
	nsStatus, err := c.Client.CoreV1().Namespaces().Get(context.Background(), n.Name(), metav1.GetOptions{})
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
	_, err := c.Client.RbacV1().Roles(namespace).Create(context.Background(), r.RoleObject, metav1.CreateOptions{})
	if err != nil && errors.IsAlreadyExists(err) {
		r.Uninstall(c, namespace)
		_, err = c.Client.RbacV1().Roles(namespace).Create(context.Background(), r.RoleObject, metav1.CreateOptions{})
	}
	if err != nil {
		return fmt.Errorf(kwlog(fmt.Sprintf("Error creating the cluster role: %v", err)))
	}
	return nil
}

func (r RoleRbacV1) Uninstall(c KubeClient, namespace string) {
	glog.V(3).Infof(kwlog(fmt.Sprintf("deleting role %s", r.Name())))
	err := c.Client.RbacV1().Roles(namespace).Delete(context.Background(), r.Name(), metav1.DeleteOptions{})
	if err != nil {
		glog.Errorf(kwlog(fmt.Sprintf("unable to delete role %s. Error: %v", r.Name(), err)))
	}
}

func (r RoleRbacV1) Status(c KubeClient, namespace string) (interface{}, error) {
	return &RoleRbacV1{}, nil
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
	_, err := c.Client.RbacV1().RoleBindings(namespace).Create(context.Background(), rb.RolebindingObject, metav1.CreateOptions{})
	if err != nil && errors.IsAlreadyExists(err) {
		rb.Uninstall(c, namespace)
		_, err = c.Client.RbacV1().RoleBindings(namespace).Create(context.Background(), rb.RolebindingObject, metav1.CreateOptions{})
	}
	if err != nil {
		return fmt.Errorf(kwlog(fmt.Sprintf("Error creating the cluster rolebinding: %v", err)))
	}
	return nil
}

func (rb RolebindingRbacV1) Uninstall(c KubeClient, namespace string) {
	glog.V(3).Infof(kwlog(fmt.Sprintf("deleting role binding %s", rb.RolebindingObject.ObjectMeta.Name)))
	err := c.Client.RbacV1().RoleBindings(namespace).Delete(context.Background(), rb.RolebindingObject.ObjectMeta.Name, metav1.DeleteOptions{})
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

// ----------------ServiceAccount----------------
type ServiceAccountCoreV1 struct {
	ServiceAccountObject *corev1.ServiceAccount
}

func (sa ServiceAccountCoreV1) Install(c KubeClient, namespace string) error {
	glog.V(3).Infof(kwlog(fmt.Sprintf("creating service account %v", sa)))
	_, err := c.Client.CoreV1().ServiceAccounts(namespace).Create(context.Background(), sa.ServiceAccountObject, metav1.CreateOptions{})
	if err != nil && errors.IsAlreadyExists(err) {
		sa.Uninstall(c, namespace)
		_, err = c.Client.CoreV1().ServiceAccounts(namespace).Create(context.Background(), sa.ServiceAccountObject, metav1.CreateOptions{})
	}
	if err != nil {
		return fmt.Errorf(kwlog(fmt.Sprintf("Error creating the cluster service account: %v", err)))
	}
	return nil
}

func (sa ServiceAccountCoreV1) Uninstall(c KubeClient, namespace string) {
	glog.V(3).Infof(kwlog(fmt.Sprintf("deleting service account %s", sa.ServiceAccountObject.ObjectMeta.Name)))
	err := c.Client.CoreV1().ServiceAccounts(namespace).Delete(context.Background(), sa.ServiceAccountObject.ObjectMeta.Name, metav1.DeleteOptions{})
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
	_, err = c.Client.AppsV1().Deployments(namespace).Create(context.Background(), &dWithEnv, metav1.CreateOptions{})
	if err != nil && errors.IsAlreadyExists(err) {
		d.Uninstall(c, namespace)
		mapName, err = c.CreateConfigMap(envAdds, d.AgreementId, namespace)
		_, err = c.Client.AppsV1().Deployments(namespace).Create(context.Background(), &dWithEnv, metav1.CreateOptions{})
	}
	if err != nil {
		return fmt.Errorf(kwlog(fmt.Sprintf("Error creating the operator deployment: %v", err)))
	}
	return nil
}

func (d DeploymentAppsV1) Uninstall(c KubeClient, namespace string) {
	glog.V(3).Infof(kwlog(fmt.Sprintf("deleting deployment %s", d.DeploymentObject.ObjectMeta.Name)))
	err := c.Client.AppsV1().Deployments(namespace).Delete(context.Background(), d.DeploymentObject.ObjectMeta.Name, metav1.DeleteOptions{})
	if err != nil {
		glog.Errorf(kwlog(fmt.Sprintf("unable to delete deployment %s. Error: %v", d.DeploymentObject.ObjectMeta.Name, err)))
	}

	configMapName := fmt.Sprintf("%s-%s", HZN_ENV_VARS, d.AgreementId)
	glog.V(3).Infof(kwlog(fmt.Sprintf("deleting config map %v", configMapName)))
	// Delete the agreement config map
	err = c.Client.CoreV1().ConfigMaps(namespace).Delete(context.Background(), configMapName, metav1.DeleteOptions{})
	if err != nil {
		glog.Errorf(kwlog(fmt.Sprintf("unable to delete config map %s. Error: %v", configMapName, err)))
	}
}

// Status will be the status of the operator pod
func (d DeploymentAppsV1) Status(c KubeClient, namespace string) (interface{}, error) {
	opName := d.DeploymentObject.ObjectMeta.Name
	podList, err := c.Client.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", "name", opName)})
	if err != nil {
		return nil, err
	} else if podList == nil || len(podList.Items) == 0 {
		labelSelector := metav1.LabelSelector{MatchLabels: d.DeploymentObject.Spec.Selector.MatchLabels}
		podList, err = c.Client.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: labels.Set(labelSelector.MatchLabels).String()})
	}
	return podList, nil
}

func (d DeploymentAppsV1) Name() string {
	return d.DeploymentObject.ObjectMeta.Name
}

//----------------CRD & CR----------------
// A new version requires a new CRD client type and adding the version scheme in getK8sObjectFromYaml

// --------Version v1beta1--------
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
	CustomResourceObjectList       []*unstructured.Unstructured
	InstallTimeout                 int64
}

func (cr CustomResourceV1Beta1) Install(c KubeClient, namespace string) error {
	apiClient, err := NewCRDV1beta1Client()
	if err != nil {
		return err
	}
	crds := apiClient.CustomResourceDefinitions()
	glog.V(3).Infof(kwlog(fmt.Sprintf("creating custom resource definition %v", cr.CustomResourceDefinitionObject)))
	_, err = crds.Create(context.Background(), cr.CustomResourceDefinitionObject, metav1.CreateOptions{})
	if err != nil && errors.IsAlreadyExists(err) {
		// If the crd already exists this is not a problem
		glog.V(3).Infof(kwlog(fmt.Sprintf("Failed to create custom resource definition %s because it already exists. Continuing with installation. %v", cr.CustomResourceDefinitionObject.Name, err)))
	} else if err != nil {
		return fmt.Errorf("Error installing custom resource definition: %v", err)
	}

	// Client for creating the CR in the cluster
	dynClient, err := NewDynamicKubeClient()
	if err != nil {
		return fmt.Errorf("Error creating the dynamic kube client: %v", err)
	}
	gvr, err := cr.gvr()
	if err != nil {
		return fmt.Errorf("Error getting the custom resource definition GroupVersionResource: %v", err)
	}
	crClient := dynClient.Resource(*gvr)

	for _, customResourceObject := range cr.CustomResourceObjectList {
		resourceName := ""
		if typedCrMetadata, ok := customResourceObject.Object["metadata"].(map[string]interface{}); ok {
			if name, ok := typedCrMetadata["name"]; ok {
				resourceName = fmt.Sprintf("%v", name)
			}
		}

		// the cluster has to create the endpoint for the custom resource, this can take some time
		// the cr cannot exist without the crd so we don't have to worry about it already existing
		timeout := cr.InstallTimeout
		glog.V(3).Infof(kwlog(fmt.Sprintf("creating the operator custom resource. Timeout is %v. Resource is %v", timeout, customResourceObject)))
		for {
			_, err = crClient.Namespace(namespace).Create(context.Background(), customResourceObject, metav1.CreateOptions{})
			if err != nil && timeout > 0 {
				glog.Warningf(kwlog(fmt.Sprintf("Failed to create custom resource %s. Trying again in 5s. Error was: %v", resourceName, err)))
				time.Sleep(time.Second * 5)
			} else if err != nil {
				return fmt.Errorf(kwlog(fmt.Sprintf("Failed to create custom resource %s. Timeout exceeded. Error was: %v", resourceName, err)))
			} else {
				glog.V(3).Infof(kwlog(fmt.Sprintf("Sucessfully created custom resource %s.", resourceName)))
				break
			}
			timeout = timeout - 5
		}
	}

	return nil
}

func (cr CustomResourceV1Beta1) Uninstall(c KubeClient, namespace string) {
	glog.V(3).Infof(kwlog(fmt.Sprintf("deleting operator custom resource created by this CRD %v %v %v %v", cr.Name(), cr.kind(), cr.group(), cr.versions())))

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

	for _, customResourceObject := range cr.CustomResourceObjectList {
		var newCrName string
		if metaInterf, ok := customResourceObject.Object["metadata"]; ok {
			if metaMap, ok := metaInterf.(map[string]interface{}); ok {
				if metaMapName, ok := metaMap["name"]; ok {
					newCrName = fmt.Sprintf("%v", metaMapName)
				} else {
					glog.Errorf(kwlog(fmt.Sprintf("unable to find operator custom resource name for %v", customResourceObject)))
				}
			} else {
				glog.Errorf(kwlog(fmt.Sprintf("unable to find operator custom resource name for %v", customResourceObject)))
			}
		} else {
			glog.Errorf(kwlog(fmt.Sprintf("unable to find operator custom resource name for %v", customResourceObject)))
		}
		glog.V(3).Infof(kwlog(fmt.Sprintf("deleting operator custom resource %v", newCrName)))

		err = crClient.Namespace(namespace).Delete(context.Background(), newCrName, metav1.DeleteOptions{})
		if err != nil {
			glog.Warningf(kwlog(fmt.Sprintf("unable to delete operator custom resource %s. Error: %v", newCrName, err)))
		} else {
			err = cr.waitForCRUninstall(c, namespace, 0, newCrName)
			if err != nil {
				glog.Errorf(kwlog(fmt.Sprintf("%v", err)))
			}
		}

	}

	crdInuse, err := cr.crdUsedByCRInOtherNamespace(crClient, namespace)
	if err != nil {
		glog.Errorf(fmt.Sprintf("%v", err))
	}

	if crdInuse {
		glog.V(3).Infof(kwlog(fmt.Sprintf("operator custom resource definition %v is still used by other custom resource, skip deleting CRD", cr.Name())))
	} else {
		glog.V(3).Infof(kwlog(fmt.Sprintf("deleting operator custom resource definition %v", cr.Name())))
		// CRDs need a different client
		apiClient, err := NewCRDV1beta1Client()
		if err != nil {
			glog.Errorf(kwlog(fmt.Sprintf("Error: unable to get a kubernetes CustomResourceDefinition client for uninstall: %v", err)))
			return
		}
		crds := apiClient.CustomResourceDefinitions()
		err = crds.Delete(context.Background(), cr.Name(), metav1.DeleteOptions{})
		if err != nil {
			glog.Errorf(kwlog(fmt.Sprintf("unable to delete operator custom resource definition %s. Error: %v", cr.Name(), err)))
		}
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
	statusArray := make([]interface{}, 1)

	for _, customResourceObject := range cr.CustomResourceObjectList {
		if metadata, ok := customResourceObject.Object["metadata"]; ok {
			if metadataTyped, ok := metadata.(map[string]interface{}); ok {
				if name, ok := metadataTyped["name"]; ok {
					res, err := crClient.Namespace(namespace).Get(context.Background(), fmt.Sprintf("%v", name), metav1.GetOptions{})
					if err != nil {
						return nil, err
					}

					if status, ok := res.Object["status"]; ok {
						statusArray = append(statusArray, status)
					} else {
						return nil, fmt.Errorf("Error status not found")
					}
				}
			}
		}
	}

	return statusArray, nil
}

func (cr CustomResourceV1Beta1) Name() string {
	return cr.CustomResourceDefinitionObject.ObjectMeta.Name
}

func (cr CustomResourceV1Beta1) kind() string {
	crdSpecNames := cr.CustomResourceDefinitionObject.Spec.Names
	crKind := crdSpecNames.Kind
	return crKind
}

func (cr CustomResourceV1Beta1) group() string {
	return cr.CustomResourceDefinitionObject.Spec.Group
}

func (cr CustomResourceV1Beta1) versions() []string {
	var versions []crdv1beta1.CustomResourceDefinitionVersion
	versions = cr.CustomResourceDefinitionObject.Spec.Versions

	var versionNames []string
	for _, version := range versions {
		versionNames = append(versionNames, version.Name)
	}
	return versionNames
}

// gvr is the group version resource that allows the dynamic client to interact with types defined by custom resource definitions
func (cr CustomResourceV1Beta1) gvr() (*schema.GroupVersionResource, error) {
	if apiVers, ok := cr.CustomResourceObjectList[0].Object["apiVersion"]; ok {
		if typedApiVers, ok := apiVers.(string); ok {
			gv, err := schema.ParseGroupVersion(typedApiVers)
			if err != nil {
				return nil, fmt.Errorf(kwlog(fmt.Sprintf("Error: failed to parse group version %s: %v", typedApiVers, err)))
			}
			gvr := schema.GroupVersionResource{Resource: cr.CustomResourceDefinitionObject.Spec.Names.Plural, Group: gv.Group, Version: gv.Version}
			return &gvr, nil
		} else {
			return nil, fmt.Errorf(kwlog(fmt.Sprintf("Error: apiversion field is not of type string %v", cr.CustomResourceObjectList[0].Object)))
		}

	} else {
		return nil, fmt.Errorf(kwlog(fmt.Sprintf("Error: custom resource object does not have apiversion field: %v", cr.CustomResourceObjectList[0].Object)))
	}
}

func (cr CustomResourceV1Beta1) crdUsedByCRInOtherNamespace(crClient dynamic.NamespaceableResourceInterface, namespace string) (bool, error) {
	return checkCRDInUse(crClient, cr.kind(), cr.group(), cr.versions(), namespace)
}

// --------Version v1--------
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
	CustomResourceObjectList       []*unstructured.Unstructured
	InstallTimeout                 int64
}

func (cr CustomResourceV1) Install(c KubeClient, namespace string) error {
	apiClient, err := NewCRDV1Client()
	if err != nil {
		return err
	}
	crds := apiClient.CustomResourceDefinitions()
	glog.V(3).Infof(kwlog(fmt.Sprintf("creating custom resource definition %v", cr.CustomResourceDefinitionObject)))
	_, err = crds.Create(context.Background(), cr.CustomResourceDefinitionObject, metav1.CreateOptions{})
	if err != nil && errors.IsAlreadyExists(err) {
		glog.V(3).Infof(kwlog(fmt.Sprintf("Failed to create custom resource definition %s because it already exists. Continuing with installation. %v", cr.CustomResourceDefinitionObject.Name, err)))
	} else if err != nil {
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

	for _, customResourceObject := range cr.CustomResourceObjectList {
		resourceName := ""
		if typedCrMetadata, ok := customResourceObject.Object["metadata"].(map[string]interface{}); ok {
			if name, ok := typedCrMetadata["name"]; ok {
				resourceName = fmt.Sprintf("%v", name)
			}
		}

		// the cluster has to create the endpoint for the custom resource, this can take some time
		// the cr cannot exist without the crd so we don't have to worry about it already existing
		timeout := cr.InstallTimeout
		glog.V(3).Infof(kwlog(fmt.Sprintf("creating the operator custom resource. Timeout is %v. Resource is %v", timeout, customResourceObject)))
		for {
			_, err = crClient.Namespace(namespace).Create(context.Background(), customResourceObject, metav1.CreateOptions{})
			if err != nil && timeout > 0 {
				glog.Warningf(kwlog(fmt.Sprintf("Failed to create custom resource %s. Trying again in 5s. Error was: %v", resourceName, err)))
				time.Sleep(time.Second * 5)
			} else if err != nil {
				return fmt.Errorf(kwlog(fmt.Sprintf("Failed to create custom resource %s. Timeout exceeded. Error was: %v", resourceName, err)))
			} else {
				glog.V(3).Infof(kwlog(fmt.Sprintf("Sucessfully created custom resource %s.", resourceName)))
				break
			}
			timeout = timeout - 5
		}
	}

	return nil
}

func (cr CustomResourceV1) Uninstall(c KubeClient, namespace string) {
	glog.V(3).Infof(kwlog(fmt.Sprintf("deleting operator custom resource created by this CRD %v %v %v %v", cr.Name(), cr.kind(), cr.group(), cr.versions())))

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
	for _, customResourceObject := range cr.CustomResourceObjectList {
		var newCrName string
		if metaInterf, ok := customResourceObject.Object["metadata"]; ok {
			if metaMap, ok := metaInterf.(map[string]interface{}); ok {
				if metaMapName, ok := metaMap["name"]; ok {
					newCrName = fmt.Sprintf("%v", metaMapName)
				} else {
					glog.Errorf(kwlog(fmt.Sprintf("unable to find operator custom resource name for %v", customResourceObject)))
				}
			} else {
				glog.Errorf(kwlog(fmt.Sprintf("unable to find operator custom resource name for %v", customResourceObject)))
			}
		} else {
			glog.Errorf(kwlog(fmt.Sprintf("unable to find operator custom resource name for %v", customResourceObject)))
		}

		glog.V(3).Infof(kwlog(fmt.Sprintf("deleting operator custom resource %v", newCrName))) // newCrName: example-nginxoperator, cr.Name(): nginxoperators.nginx.operator.com
		err = crClient.Namespace(namespace).Delete(context.Background(), newCrName, metav1.DeleteOptions{})
		if err != nil {
			glog.Warningf(kwlog(fmt.Sprintf("unable to delete operator custom resource %s. Error: %v", newCrName, err)))
		} else {
			err = cr.waitForCRUninstall(c, namespace, 0, newCrName)
			if err != nil {
				glog.Errorf(fmt.Sprintf("%v", err))
			}
		}
	}

	crdInuse, err := cr.crdUsedByCRInOtherNamespace(crClient, namespace)
	if err != nil {
		glog.Errorf(fmt.Sprintf("%v", err))
	}

	if crdInuse {
		glog.V(3).Infof(kwlog(fmt.Sprintf("operator custom resource definition %v is still used by other custom resource, skip deleting CRD", cr.Name())))
	} else {
		glog.V(3).Infof(kwlog(fmt.Sprintf("deleting operator custom resource definition %v", cr.Name())))
		// CRDs need a different client
		apiClient, err := NewCRDV1Client()
		if err != nil {
			glog.Errorf(kwlog(fmt.Sprintf("Error: unable to get a kubernetes CustomResourceDefinition client for uninstall: %v", err)))
			return
		}
		crds := apiClient.CustomResourceDefinitions()
		err = crds.Delete(context.Background(), cr.Name(), metav1.DeleteOptions{})
		if err != nil {
			glog.Errorf(kwlog(fmt.Sprintf("unable to delete operator custom resource definition %s. Error: %v", cr.Name(), err)))
		}

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
	statusArray := make([]interface{}, 1)

	for _, customResourceObject := range cr.CustomResourceObjectList {
		if metadata, ok := customResourceObject.Object["metadata"]; ok {
			if metadataTyped, ok := metadata.(map[string]interface{}); ok {
				if name, ok := metadataTyped["name"]; ok {
					res, err := crClient.Namespace(namespace).Get(context.Background(), fmt.Sprintf("%v", name), metav1.GetOptions{})
					if err != nil {
						return nil, err
					}

					if status, ok := res.Object["status"]; ok {
						statusArray = append(statusArray, status)
					} else {
						return nil, fmt.Errorf("Error status not found")
					}
				}
			}
		}
	}

	return statusArray, nil
}

func (cr CustomResourceV1) Name() string {
	return cr.CustomResourceDefinitionObject.ObjectMeta.Name
}

func (cr CustomResourceV1) kind() string {
	crdSpecNames := cr.CustomResourceDefinitionObject.Spec.Names
	crKind := crdSpecNames.Kind
	return crKind
}

func (cr CustomResourceV1) group() string {
	return cr.CustomResourceDefinitionObject.Spec.Group
}

func (cr CustomResourceV1) versions() []string {
	var versions []crdv1.CustomResourceDefinitionVersion
	versions = cr.CustomResourceDefinitionObject.Spec.Versions

	var versionNames []string
	for _, version := range versions {
		versionNames = append(versionNames, version.Name)
	}
	return versionNames
}

// group-version-resource is used by the discovry client to interact with a type defined by the custom resource definition
func (cr CustomResourceV1) gvr() (*schema.GroupVersionResource, error) {
	if apiVers, ok := cr.CustomResourceObjectList[0].Object["apiVersion"]; ok {
		if typedApiVers, ok := apiVers.(string); ok {
			gv, err := schema.ParseGroupVersion(typedApiVers)
			if err != nil {
				return nil, fmt.Errorf(kwlog(fmt.Sprintf("Error: failed to parse group version %s: %v", typedApiVers, err)))
			}
			gvr := schema.GroupVersionResource{Resource: cr.CustomResourceDefinitionObject.Spec.Names.Plural, Group: gv.Group, Version: gv.Version}
			return &gvr, nil
		} else {
			return nil, fmt.Errorf(kwlog(fmt.Sprintf("Error: apiversion field is not of type string %v", cr.CustomResourceObjectList[0].Object)))
		}

	} else {
		return nil, fmt.Errorf(kwlog(fmt.Sprintf("Error: custom resource object does not have apiversion field: %v", cr.CustomResourceObjectList[0].Object)))
	}
}

func (cr CustomResourceV1) crdUsedByCRInOtherNamespace(crClient dynamic.NamespaceableResourceInterface, namespace string) (bool, error) {
	return checkCRDInUse(crClient, cr.kind(), cr.group(), cr.versions(), namespace)
}

func checkCRDInUse(crClient dynamic.NamespaceableResourceInterface, crdKind string, crdGroup string, crdVersions []string, namespace string) (bool, error) {
	glog.V(3).Infof(kwlog(fmt.Sprintf("Check if the CRD (Kind: %v, Group: %v, Versions: %v) is used by crs in the namespace other than %v", crdKind, crdGroup, crdVersions, namespace)))
	lOps := metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.namespace!=%v", namespace),
	}
	crsInOtherNS, err := crClient.Namespace("").List(context.Background(), lOps)
	if err != nil && !errors.IsNotFound(err) && !strings.Contains(err.Error(), "not find") {
		glog.Errorf(kwlog(fmt.Sprintf("failed to list all CRs in other namespace error: %v, will keep this crd", err)))
		return true, err
	} else {
		glog.V(5).Infof(kwlog(fmt.Sprintf("CRs in other namespace result: %v", crsInOtherNS)))
		items := crsInOtherNS.Items
		for _, item := range items {
			//  eg: item.GetKind():NginxOperator, item.GetAPIVersion(): nginx.operator.com/v1alpha1
			if item.GetNamespace() != namespace && item.GetKind() == crdKind {
				if groupVersion, err := schema.ParseGroupVersion(item.GetAPIVersion()); err != nil {
					glog.Errorf(kwlog(fmt.Sprintf("unable to get group and version for %v", item)))
					return true, err
				} else if groupVersion.Group == crdGroup && cutil.SliceContains(crdVersions, groupVersion.Version) {
					glog.V(3).Infof(kwlog(fmt.Sprintf("The crd (Kind: %v, Group: %v, Version: %v) is still in use, skip deleting crd", crdKind, crdGroup, crdVersions)))
					return true, nil
				}
			}
		}
	}

	return false, nil
}
