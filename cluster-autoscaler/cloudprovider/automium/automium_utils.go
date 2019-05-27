package automium

import (
	"errors"
	"strconv"
	"sync"

	"github.com/automium/types/go/v1beta1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kube_client "k8s.io/client-go/kubernetes"
	"k8s.io/klog"
)

func envSliceToEnvMap(envSlice []corev1.EnvVar) map[string]string {
	envMap := make(map[string]string, 0)
	for _, item := range envSlice {
		klog.V(5).Infof("EnvSliceToEnvMap: Key: %v - Value: %v\n", item.Name, item.Value)
		envMap[item.Name] = item.Value
	}
	return envMap
}

func generateAutoscalingDataForService(svc v1beta1.Service, clusterName string) (AutomiumAutoscalingService, error) {
	var serviceScalingData AutomiumAutoscalingService

	klog.V(5).Infof("working on service %s for cluster %s\n", svc.Name, clusterName)
	if svc.ObjectMeta.Labels["app"] == "kubernetes-nodepool" {
		envMap := envSliceToEnvMap(svc.Spec.Env)
		if clusterName == envMap["cluster_name"] {
			if envMap["autoscaling"] == "true" {
				klog.Infof("configuring autoscaling for service kubernetes-nodepool %s of cluster %s\n", svc.Name, clusterName)
				minVal, err := strconv.Atoi(envMap["nodes_min"])
				if err != nil {
					return AutomiumAutoscalingService{}, err
				}
				maxVal, err := strconv.Atoi(envMap["nodes_max"])
				if err != nil {
					return AutomiumAutoscalingService{}, err
				}

				if minVal > 0 && maxVal > 0 {
					if minVal > maxVal {
						return AutomiumAutoscalingService{}, errors.New("invalid limits specified in configuration: minimum replicas value is higher than the maximum replicas value")
					}
				} else {
					return AutomiumAutoscalingService{}, errors.New("invalid limits specified in configuration: minimum replicas value and/or maximum replicas value are invalid")
				}

				serviceScalingData.ServiceName = svc.Name
				serviceScalingData.OperationLock = &sync.Mutex{}
				serviceScalingData.MinCount = minVal
				serviceScalingData.MaxCount = maxVal
				klog.V(5).Infof("generated automiumautoscalingservice: %+v\n", serviceScalingData)
				return serviceScalingData, nil
			}
			klog.V(5).Infof("kubernetes-nodepool %s has no autoscaling configured - not adding\n", svc.Name)
		} else {
			klog.V(5).Infof("kubernetes-nodepool %s does not belong to this cluster - not adding\n", svc.Name)
		}
	}
	klog.V(5).Infof("service %s is not a kubernetes-nodepool - not adding\n", svc.Name)
	return AutomiumAutoscalingService{}, ErrNotAutoscalingService
}

func retrieveKubernetesNodes(clientInterface kube_client.Interface) ([]v1.Node, error) {
	kubernetesNodes, err := clientInterface.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return []corev1.Node{}, err
	}
	return kubernetesNodes.Items, nil
}
