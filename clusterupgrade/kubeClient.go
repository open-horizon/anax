package clusterupgrade

import (
	"context"
	"fmt"
	"github.com/golang/glog"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/open-horizon/anax/cutil"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"strings"
)

const AGENT_SERVICE_ACCOUNT_TOKEN = "agent-service-account-token"

// Client to interact with all standard k8s objects
type KubeClient struct {
	Client *kubernetes.Clientset
}

func NewKubeClient() (*KubeClient, error) {
	clientset, err := cutil.NewKubeClient()
	if err != nil {
		return nil, err
	}
	return &KubeClient{Client: clientset}, nil
}

/*
	Name:         openhorizon-agent-config
	Namespace:    openhorizon-agent
	Labels:       <none>
	Annotations:  <none>

	Data
	====
	horizon:
	----
	HZN_EXCHANGE_URL=https://host-url/edge-exchange/v1
	HZN_FSS_CSSURL=https://host-url/edge-css
	HZN_AGBOT_URL=https://host-url/edge-agbot/
	HZN_SDO_SVC_URL=https://host-url/edge-sdo-ocs/api
	HZN_DEVICE_ID=my-node-id
	HZN_NODE_ID=my-node-id
	HZN_MGMT_HUB_CERT_PATH=/etc/default/cert/agent-install.crt
	HZN_AGENT_PORT=8510
*/

func (c KubeClient) GetConfigMap(namespace string, cmName string) (*v1.ConfigMap, error) {
	glog.V(3).Infof(cuwlog(fmt.Sprintf("Get configmap %v under agent namespace %v", cmName, namespace)))

	rawConfigMap := c.Client.CoreV1().ConfigMaps(namespace)
	return rawConfigMap.Get(context.Background(), cmName, metav1.GetOptions{})
}

func (c KubeClient) ReadConfigMap(namespace string, cmName string) (map[string]string, error) {
	glog.V(3).Infof(cuwlog(fmt.Sprintf("Read configmap value %v under agent namespace %v", cmName, namespace)))
	if cm, err := c.GetConfigMap(namespace, cmName); err != nil {
		return make(map[string]string), err
	} else if cm == nil {
		return make(map[string]string), fmt.Errorf("configmap %v is nil", cmName)
	} else if configInK8S, err := parseAgentConfigMap(cm); err != nil {
		return make(map[string]string), err
	} else {
		return configInK8S, nil
	}
}

// return the key value pairs under "horizon" section inside configmap in a map
func parseAgentConfigMap(configMap *v1.ConfigMap) (map[string]string, error) {
	cmData := configMap.Data
	valuesInConfigMap := cmData["horizon"]

	/*
		valuesInConfigMap is a string:
		"HZN_EXCHANGE_URL=https://host-url/edge-exchange/v1
		HZN_FSS_CSSURL=https://host-url/edge-css
		HZN_AGBOT_URL=https://host-url/edge-agbot/
		HZN_SDO_SVC_URL=https://host-url/edge-sdo-ocs/api
		HZN_DEVICE_ID=my-node-id
		HZN_NODE_ID=my-node-id
		HZN_MGMT_HUB_CERT_PATH=/etc/default/cert/agent-install.crt
		HZN_AGENT_PORT=8510"
	*/

	configValueInMap := make(map[string]string)
	mapEntries := strings.Split(valuesInConfigMap, "\n")
	for _, entry := range mapEntries {
		// entry: HZN_EXCHANGE_URL=https://host-url/edge-exchange/v1
		if strings.Contains(entry, "=") {
			kvOfEntry := strings.Split(entry, "=")
			if len(kvOfEntry) == 2 {
				configValueInMap[kvOfEntry[0]] = kvOfEntry[1]
				glog.V(3).Infof(cuwlog(fmt.Sprintf("In configmap %v find %v=%v", configMap.Name, kvOfEntry[0], kvOfEntry[1])))
			}
		}
	}

	return configValueInMap, nil
}

func prepareConfigmapData(properties map[string]string) string {
	glog.V(3).Infof(cuwlog(fmt.Sprintln("Preparing data for configmap")))
	var result string
	for key, value := range properties {
		result += fmt.Sprintf("%v=%v\n", key, value)
	}
	glog.V(3).Infof(cuwlog(fmt.Sprintf("Data prepared for configmap: %v", result)))
	return result
}

func (c KubeClient) CreateBackupConfigmap(namespace string, cmName string) error {
	glog.V(3).Infof(cuwlog(fmt.Sprintf("Create backup configmap %v-backup under agent namespace %v", cmName, namespace)))
	currentConfigMap, err := c.GetConfigMap(namespace, cmName)
	if err != nil {
		return err
	} else if currentConfigMap == nil {
		return fmt.Errorf("configmap %v is nil", cmName)
	} else {
		backupConfigmapName := fmt.Sprintf("%v-backup", cmName)
		currentData := currentConfigMap.Data
		backupConfigMap := v1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      backupConfigmapName,
				Namespace: namespace,
			},
			Data: currentData,
		}

		currentBackupConfigMap, err := c.GetConfigMap(namespace, backupConfigmapName)
		if errors.IsNotFound(err) || currentBackupConfigMap == nil {
			_, err = c.Client.CoreV1().ConfigMaps(namespace).Create(context.Background(), &backupConfigMap, metav1.CreateOptions{})
		} else {
			_, err = c.Client.CoreV1().ConfigMaps(namespace).Update(context.Background(), &backupConfigMap, metav1.UpdateOptions{})
		}

		if err != nil {
			return err
		}

		glog.V(3).Infof(cuwlog(fmt.Sprintf("Backup configmap %v under agent namespace %v is created successfully", backupConfigmapName, namespace)))
		return nil

	}
}

func (c KubeClient) UpdateAgentConfigmap(namespace string, cmName string, newHorizonValue string) error {
	glog.V(3).Infof(cuwlog(fmt.Sprintf("Update configmap %v under agent namespace %v to use new horizon value %v", cmName, namespace, newHorizonValue)))
	currentConfigMap, err := c.GetConfigMap(namespace, cmName)
	if err != nil {
		return err
	} else {
		newValueInConfigmap := make(map[string]string)
		newValueInConfigmap["horizon"] = newHorizonValue
		updateConfigMap := v1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      cmName,
				Namespace: namespace,
			},
			Data: newValueInConfigmap,
		}

		if errors.IsNotFound(err) || currentConfigMap == nil {
			_, err = c.Client.CoreV1().ConfigMaps(namespace).Create(context.Background(), &updateConfigMap, metav1.CreateOptions{})
		} else {
			_, err = c.Client.CoreV1().ConfigMaps(namespace).Update(context.Background(), &updateConfigMap, metav1.UpdateOptions{})
		}

		if err != nil {
			return err
		}
		glog.V(3).Infof(cuwlog(fmt.Sprintf("Configmap %v under agent namespace %v is updated successfully", cmName, namespace)))
		return nil
	}
}

//----------------Secret----------------
// kubectl get secret openhorizon-agent-secrets -n openhorizon-agent -o yaml
/*
apiVersion: v1
data:
  agent-install.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS.........
kind: Secret
metadata:
  creationTimestamp: "2022-02-15T19:45:50Z"
  managedFields:
  - apiVersion: v1
    fieldsType: FieldsV1
    fieldsV1:
      f:data:
        .: {}
        f:agent-install.crt: {}
      f:type: {}
    manager: kubectl-create
    operation: Update
    time: "2022-02-15T19:45:50Z"
  name: openhorizon-agent-secrets
  namespace: openhorizon-agent
  resourceVersion: "11177589"
  uid: 3001e924-78ee-43eb-8c5b-b9f22abcd013
type: Opaque
*/
func (c KubeClient) GetSecret(namespace string, secretName string) (*v1.Secret, error) {
	glog.V(3).Infof(cuwlog(fmt.Sprintf("Get secret %v under agent namespace %v", secretName, namespace)))

	rawSecret := c.Client.CoreV1().Secrets(namespace)
	return rawSecret.Get(context.Background(), secretName, metav1.GetOptions{})
}

func (c KubeClient) ReadSecret(namespace string, secretName string) ([]byte, error) {
	glog.V(3).Infof(cuwlog(fmt.Sprintf("Read secret value %v under agent namespace %v", secretName, namespace)))

	if secret, err := c.GetSecret(namespace, secretName); err != nil {
		return make([]byte, 0), err
	} else if secret == nil {
		return make([]byte, 0), fmt.Errorf("secret %v is nil", secretName)
	} else {
		certValueBytes := parseAgentSecret(secret)
		glog.V(3).Infof(cuwlog(fmt.Sprintf("Secret value is: %v", string(certValueBytes))))
		return certValueBytes, nil
	}

}

func parseAgentSecret(secret *v1.Secret) []byte {
	secretData := secret.Data
	certValue := secretData[AGENT_CERT_FILE]
	return certValue
}

func (c KubeClient) CreateBackupSecret(namespace string, secretName string) error {
	glog.V(3).Infof(cuwlog(fmt.Sprintf("Create backup secret %v-backup under agent namespace %v", secretName, namespace)))
	currentSecret, err := c.GetSecret(namespace, secretName)
	if err != nil {
		return err
	} else if currentSecret == nil {
		return fmt.Errorf("secret %v is nil", secretName)
	} else {
		backupSecretName := fmt.Sprintf("%v-backup", secretName)
		currentSecretData := currentSecret.Data
		backupSecret := v1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      backupSecretName,
				Namespace: namespace,
			},
			Data: currentSecretData,
		}

		currentBackupSecret, err := c.GetSecret(namespace, backupSecretName)
		if errors.IsNotFound(err) || currentBackupSecret == nil {
			_, err = c.Client.CoreV1().Secrets(namespace).Create(context.Background(), &backupSecret, metav1.CreateOptions{})
		} else {
			_, err = c.Client.CoreV1().Secrets(namespace).Update(context.Background(), &backupSecret, metav1.UpdateOptions{})
		}

		if err != nil {
			return err
		}

		glog.V(3).Infof(cuwlog(fmt.Sprintf("Backup secret %v under agent namespace %v is created successfully", backupSecretName, namespace)))
		return nil
	}
}

func (c KubeClient) UpdateAgentSecret(namespace string, secretName string, newSecretValue []byte) error {
	glog.V(3).Infof(cuwlog(fmt.Sprintf("Update secret %v under agent namespace %v to use new cert value", secretName, namespace)))
	currentSecret, err := c.GetSecret(namespace, secretName)
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else {
		// create secret struct
		secretData := make(map[string][]byte)
		secretData[AGENT_CERT_FILE] = newSecretValue
		updateSecret := v1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
			},
			Data: secretData,
		}
		if errors.IsNotFound(err) || currentSecret == nil {
			_, err = c.Client.CoreV1().Secrets(namespace).Create(context.Background(), &updateSecret, metav1.CreateOptions{})
		} else {
			_, err = c.Client.CoreV1().Secrets(namespace).Update(context.Background(), &updateSecret, metav1.UpdateOptions{})
		}

		if err != nil {
			return err
		}
		glog.V(3).Infof(cuwlog(fmt.Sprintf("Secret %v under agent namespace %v is updated successfully", secretName, namespace)))
		return nil
	}
}

//----------------Service Account----------------
func (c KubeClient) GetServiceAccount(namespace string, serviceAccountName string) (*v1.ServiceAccount, error) {
	glog.V(3).Infof(cuwlog(fmt.Sprintf("Get service account %v under namespace %v", serviceAccountName, namespace)))

	rawServiceAccount := c.Client.CoreV1().ServiceAccounts(namespace)
	return rawServiceAccount.Get(context.Background(), serviceAccountName, metav1.GetOptions{})
}

// oc get secret agent-service-account-token-hltrh -o json
/*
{
    "apiVersion": "v1",
    "data": {
        "ca.crt": "base64 encoded string",
		"namespace": "b3Blbmhvcml6b24tYWdlbnQ=", (which is openhorizon-agent)
        "service-ca.crt": "base64 encoded string",
		"token": "base64 encoded string"
	},
	"kind": "Secret",
    "metadata": {
        "annotations": {
            "kubernetes.io/created-by": "openshift.io/create-dockercfg-secrets",
            "kubernetes.io/service-account.name": "agent-service-account",
            "kubernetes.io/service-account.uid": "a683627d-42cc-4951-94c7-5031f7127a8e"
        },
        "creationTimestamp": "2022-02-12T00:50:10Z",
        "name": "agent-service-account-token-hltrh",
        "namespace": "openhorizon-agent",
        "resourceVersion": "2241414",
        "uid": "023c355a-e33b-4228-aa23-3cc5d80051b6"
    },
    "type": "kubernetes.io/service-account-token"
}
*/
// GetServiceAccountToken returns decode value of service account token
func (c KubeClient) GetServiceAccountToken(namespace string, serviceAccountName string) ([]byte, error) {
	glog.V(3).Infof(cuwlog(fmt.Sprintf("Get token of service account %v under namespace %v", serviceAccountName, namespace)))

	var tokenSecretName string
	if serviceAccount, err := c.GetServiceAccount(namespace, serviceAccountName); err != nil {
		return make([]byte, 0), err
	} else {
		glog.V(3).Infof(cuwlog(fmt.Sprintf("Check secrets associated service account %v under namespace %v: ", serviceAccountName, namespace)))
		secrets := serviceAccount.Secrets
		for _, secret := range secrets {
			glog.V(3).Infof(cuwlog(fmt.Sprintf("Secret name %v is associated with service account", secret.Name)))
			if strings.Contains(secret.Name, AGENT_SERVICE_ACCOUNT_TOKEN) {
				// find the secret name
				tokenSecretName = secret.Name
			}
		}
	}

	if tokenSecretName == "" {
		return make([]byte, 0), fmt.Errorf("token secret associated Agent service account %v is not found", serviceAccountName)
	}

	// get secret, and get token inside
	var tokenValue string
	if secret, err := c.GetSecret(namespace, tokenSecretName); err != nil {
		return make([]byte, 0), err
	} else if secret == nil {
		return make([]byte, 0), fmt.Errorf("secret %v is nil", tokenSecretName)
	} else {
		secretMap := secret.Data
		for k, v := range secretMap {
			if k == "token" {
				tokenValue = string(v) // tokenValue is in base64?
				glog.V(3).Infof(cuwlog(fmt.Sprintf("Token associated with service account %v is %v", secret.Name, tokenValue)))
				return v, nil
			}
		}
	}
	return make([]byte, 0), fmt.Errorf("agent service account token not found in %v", serviceAccountName)
}

func (c KubeClient) GetKeyChain(namespace string, serviceAccountName string) (authn.Keychain, error) {
	kc, err := k8schain.NewInCluster(context.Background(), k8schain.Options{
		Namespace:          namespace,
		ServiceAccountName: serviceAccountName,
	})
	if err != nil {
		return kc, err
	}

	return kc, err
}

//----------------Deployment----------------
func (c KubeClient) GetDeployment(namespace string, deploymentName string) (*appsv1.Deployment, error) {
	glog.V(3).Infof(cuwlog(fmt.Sprintf("Get deployment %v under agent namespace %v", deploymentName, namespace)))
	deployment, err := c.Client.AppsV1().Deployments(namespace).Get(context.Background(), deploymentName, metav1.GetOptions{})
	return deployment, err
}

func (c KubeClient) UpdateAgentDeploymentImageVersion(namespace string, deploymentName string, newImageVersion string) error {
	glog.V(3).Infof(cuwlog(fmt.Sprintf("Update image version to %v in %v deployment under agent namespace %v", newImageVersion, deploymentName, namespace)))
	currentDeployment, err := c.GetDeployment(namespace, deploymentName)
	if err != nil {
		return err
	} else if currentDeployment == nil {
		return fmt.Errorf("get nil agent deployment")
	} else {
		// deployment.Spec.Template.Spec.Containers[0].Image = "newImage:tag"
		if len(currentDeployment.Spec.Template.Spec.Containers) == 0 {
			return fmt.Errorf("no container found for agent deployment")
		}
		currentAgentImage := currentDeployment.Spec.Template.Spec.Containers[0].Image
		parts := strings.Split(currentAgentImage, ":")
		if len(parts) == 0 {
			return fmt.Errorf("currentAgentImage: %v is in wrong format", currentAgentImage)
		}
		parts[len(parts)-1] = newImageVersion
		newAgentImage := strings.Join(parts, ":")
		glog.V(3).Infof(cuwlog(fmt.Sprintf("NewAgentImage is %v, will update agent deployment with this value", newAgentImage)))
		currentDeployment.Spec.Template.Spec.Containers[0].Image = newAgentImage

		glog.V(3).Infof(cuwlog(fmt.Sprintf("Updating agent deployment with new image: %v", newAgentImage)))
		if _, err = c.Client.AppsV1().Deployments(namespace).Update(context.Background(), currentDeployment, metav1.UpdateOptions{}); err == nil {
			glog.V(3).Infof(cuwlog("Agent deployment is updated successfully"))
		}
		return err
	}
}
