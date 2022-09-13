package resource

import (
	"context"
	"fmt"

	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

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

//----------------Secret----------------
// kubectl get secret openhorizon-agent-secrets -n openhorizon-agent -o yaml
/*
apiVersion: v1
data:
  cert.pem: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS.........
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
	glog.V(3).Infof(rmLogString(fmt.Sprintf("Get secret %v under agent namespace %v", secretName, namespace)))

	rawSecret := c.Client.CoreV1().Secrets(namespace)
	return rawSecret.Get(context.Background(), secretName, metav1.GetOptions{})
}

func (c KubeClient) CreateSecretFromCertificate(namespace string, secretName string, certificateContent []byte) error {
	glog.V(3).Infof(rmLogString(fmt.Sprintf("Create secret %v from ESS certificate file under agent namespace %v", secretName, namespace)))
	secretData := make(map[string][]byte)
	secretData[config.HZN_FSS_CERT_FILE] = []byte(certificateContent)
	certSecret := v1.Secret{
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

	currentEssCertSecret, err := c.GetSecret(namespace, secretName)
	if k8serrors.IsNotFound(err) || currentEssCertSecret == nil {
		_, err = c.Client.CoreV1().Secrets(namespace).Create(context.Background(), &certSecret, metav1.CreateOptions{})
	} else {
		_, err = c.Client.CoreV1().Secrets(namespace).Update(context.Background(), &certSecret, metav1.UpdateOptions{})
	}

	if err != nil {
		return err
	}

	glog.V(3).Infof(rmLogString(fmt.Sprintf("ESS Cert Secret %v is created successfully under agent namespace %v", secretName, namespace)))
	return nil
}
