package cutil

import (
	"context"
	"fmt"
	"github.com/golang/glog"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"math"
	"os"
)

const AGENT_PVC_NAME = "openhorizon-agent-pvc"

func NewKubeConfig() (*rest.Config, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("Failed to get cluster config information: %v", err)
	}
	return config, nil
}

func NewKubeClient() (*kubernetes.Clientset, error) {
	config, err := NewKubeConfig()
	if err != nil {
		return nil, err
	}
	clientset, _ := kubernetes.NewForConfig(config)
	return clientset, nil
}

// GetClusterCountInfo returns the cluster's available memory, total memory, cpu count, arch, kube version, cluster namespace, agent scope or an error if it cannot get the client
func GetClusterCountInfo() (float64, float64, float64, string, string, string, bool, error) {
	client, err := NewKubeClient()
	if err != nil {
		return 0, 0, 1, "", "", "", false, fmt.Errorf("Failed to get kube client for introspecting cluster properties. Proceding with default values. %v", err)
	}
	versionObj, err := client.Discovery().ServerVersion()
	if err != nil {
		glog.Warningf("Failed to get kubernetes server version: %v", err)
	}
	version := ""
	if versionObj != nil {
		version = versionObj.GitVersion
	}

	// get kube namespace
	ns := GetClusterNamespace()
	isNamespaceScoped := IsNamespaceScoped()

	availMem := float64(0)
	totalMem := float64(0)
	cpu := float64(0)
	arch := ""
	nodes, err := client.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return 0, 0, 0, "", "", "", false, err
	}

	for _, node := range nodes.Items {
		if arch == "" {
			arch = node.Status.NodeInfo.Architecture
		}
		availMem += FloatFromQuantity(node.Status.Allocatable.Memory()) / 1000000
		totalMem += FloatFromQuantity(node.Status.Capacity.Memory()) / 1000000
		cpu += FloatFromQuantity(node.Status.Capacity.Cpu())
	}

	return math.Round(availMem), math.Round(totalMem), cpu, arch, version, ns, isNamespaceScoped, nil
}

// FloatFromQuantity returns a float64 with the value of the given quantity type
func FloatFromQuantity(quantVal *resource.Quantity) float64 {
	if intVal, ok := quantVal.AsInt64(); ok {
		return float64(intVal)
	}
	decVal := quantVal.AsDec()
	unscaledVal := decVal.UnscaledBig().Int64()
	scale := decVal.Scale()
	floatVal := float64(unscaledVal) * math.Pow10(-1*int(scale))
	return floatVal
}

func GetClusterNamespace() string {
	// get kube namespace
	ns := os.Getenv("AGENT_NAMESPACE")
	if ns == "" {
		ns = "openhorizon-agent"
	}

	return ns
}

func IsNamespaceScoped() bool {
	isNamespaceScoped := os.Getenv("HZN_NAMESPACE_SCOPED")
	return isNamespaceScoped == "true"
}

// pvc name: openhorizon-agent-pvc
func GetAgentPVCInfo() (string, []v1.PersistentVolumeAccessMode, error) {
	client, err := NewKubeClient()
	if err != nil {
		return "", []v1.PersistentVolumeAccessMode{}, err
	}

	agentNamespace := GetClusterNamespace()
	if agentPVC, err := client.CoreV1().PersistentVolumeClaims(agentNamespace).Get(context.Background(), AGENT_PVC_NAME, metav1.GetOptions{}); err != nil {
		return "", []v1.PersistentVolumeAccessMode{}, err
	} else {
		scName := agentPVC.Spec.StorageClassName
		accessMode := agentPVC.Spec.AccessModes
		return *scName, accessMode, nil
	}
}
