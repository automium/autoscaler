package automium

import (
	"errors"
	"fmt"
	"io/ioutil"
	"sync"
	"time"

	"github.com/automium/types/go/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	kube_client "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"

	"k8s.io/client-go/rest"
)

// CreateAutomiumAgentConnection creates a connection to the Automium Kubernetes cluster
func CreateAutomiumAgentConnection(kubeconfigPath, clusterName string) (*AutomiumAgent, error) {
	var RESTConfig *rest.Config
	var err error

	if kubeconfigPath == "" {
		// Try in-cluster configuration
		klog.V(2).Infof("using in-cluster configuration to connect to Automium\n")
		RESTConfig, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	} else {
		klog.V(2).Infof("using file configuration (%s) to connect to Automium\n", kubeconfigPath)
		kubeConfig, err := ioutil.ReadFile(kubeconfigPath)
		if err != nil {
			return nil, err
		}

		RESTConfig, err = clientcmd.RESTConfigFromKubeConfig(kubeConfig)
		if err != nil {
			return nil, err
		}
	}

	kc, err := createRESTClient(RESTConfig)
	if err != nil {
		return nil, err
	}

	client := new(AutomiumAgent)
	client.kubeClient = kc
	client.lockMap = make(map[string]*sync.Mutex)

	client.retrieveAutoscalingServices(clusterName)

	for len(client.autoscalingServices) == 0 {
		klog.Warningf("no autoscaling services found. Requery in %d seconds\n", ServiceRequerySecs)
		time.Sleep(ServiceRequerySecs * time.Second)
		client.retrieveAutoscalingServices(clusterName)
	}

	klog.V(5).Infof("autoscaling services: %+v\n", client.autoscalingServices)

	for _, s := range client.autoscalingServices {
		client.lockMap[s.ServiceName] = &sync.Mutex{}
	}

	return client, nil
}

func createRESTClient(config *rest.Config) (*rest.RESTClient, error) {
	v1beta1.AddToScheme(scheme.Scheme)

	crdConfig := *config
	crdConfig.ContentConfig.GroupVersion = &schema.GroupVersion{Group: v1beta1.GroupName, Version: v1beta1.GroupVersion}
	crdConfig.APIPath = "/apis"
	crdConfig.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}
	crdConfig.UserAgent = rest.DefaultKubernetesUserAgent()

	rc, err := rest.UnversionedRESTClientFor(&crdConfig)
	if err != nil {
		return nil, err
	}

	return rc, nil
}

// RetrieveActiveWorkersCount retrieves active workers
func (ag *AutomiumAgent) RetrieveActiveWorkersCount(svcName string) (int, error) {
	var nodes v1beta1.NodeList

	nodeCount := 0
	err := ag.kubeClient.Get().Resource("nodes").Do().Into(&nodes)
	if err != nil {
		return -1, err
	}

	for _, node := range nodes.Items {
		if node.ObjectMeta.Annotations["service.automium.io/name"] == svcName {
			klog.V(2).Infof("[svc: %s] found Kubernetes worker %s\n", svcName, node.GetName())
			if node.Status.NodeProperties.ID != "non-existent-machine" {
				if ag.nodeIsHealthy(node) {
					klog.V(2).Infof("[svc: %s] Kubernetes worker %s is alive with ID %s and IP %s\n", svcName, node.GetName(), node.Status.NodeProperties.ID, node.Status.NodeProperties.Address)
					nodeCount++
				} else {
					klog.V(2).Infof("[svc: %s] Kubernetes worker %s registered but unhealthy\n", svcName, node.GetName())
				}
			} else {
				klog.V(2).Infof("[svc: %s] Kubernetes worker %s is not ready\n", svcName, node.GetName())
			}
		}
	}
	klog.V(5).Infof("[svc: %s] total active workers: %d\n", svcName, nodeCount)
	return nodeCount, nil
}

// EditWorkersCount edit the workers count in the Automium service
func (ag *AutomiumAgent) EditWorkersCount(svcName string, delta int) error {
	klog.V(2).Infof("[svc: %s] requesting lock for edit workers count...", svcName)
	ag.lockMap[svcName].Lock()
	defer ag.lockMap[svcName].Unlock()
	klog.V(2).Infof("[svc: %s] successfully requested lock", svcName)

	var services v1beta1.ServiceList
	var targetService v1beta1.Service

	err := ag.kubeClient.Get().Resource("services").Do().Into(&services)
	if err != nil {
		return err
	}

	for _, svc := range services.Items {
		if svc.GetName() == svcName {
			targetService = svc
			break
		}
	}

	if &targetService == nil {
		return errors.New("service not found")
	}

	if targetService.Status.Phase != "Completed" {
		return fmt.Errorf("cannot operate on service %s -- not in Completed phase (found phase: %s)", svcName, targetService.Status.Phase)
	}

	initRep := targetService.Spec.Replicas
	targetService.Spec.Replicas += delta
	finalRep := targetService.Spec.Replicas

	err = ag.kubeClient.Put().Namespace(targetService.GetNamespace()).Resource("services").Name(targetService.GetName()).Body(&targetService).Do().Error()
	if err != nil {
		return err
	}

	klog.V(1).Infof("[svc: %s] waiting for nodes to converge (%d -> %d)\n", svcName, initRep, finalRep)
	nodesHaveConverged := false

	for !nodesHaveConverged {
		curNodes, err := ag.RetrieveActiveWorkersCount(svcName)
		if err != nil {
			return err
		}
		if curNodes == finalRep {
			klog.V(1).Infof("[svc: %s] nodes converged\n", svcName)
			nodesHaveConverged = true
		} else {
			klog.V(1).Infof("[svc: %s] nodes not converged (%d of %d), recheck in 10s.\n", svcName, curNodes, finalRep)
			time.Sleep(10 * time.Second)
		}
	}

	if delta > 0 {
		klog.V(1).Infof("[svc: %s] waiting 1m for the nodes to stabilize...", svcName)
		time.Sleep(1 * time.Minute)
	}
	klog.V(1).Infof("[svc: %s] scale operation terminated -- releasing lock", svcName)
	return nil
}

// GetNodesForService retrieves the Automium nodes which belong to the specified service
func (ag *AutomiumAgent) GetNodesForService(serviceName string) ([]v1beta1.Node, error) {
	var automiumNodes v1beta1.NodeList
	var nodeList []v1beta1.Node

	err := ag.kubeClient.Get().Resource("nodes").Do().Into(&automiumNodes)
	if err != nil {
		return nodeList, err
	}
	for _, node := range automiumNodes.Items {
		if node.ObjectMeta.Annotations["service.automium.io/name"] == serviceName {
			nodeList = append(nodeList, node)
		}
	}

	return nodeList, nil
}

// RetrieveNodeStatus retrieves the status of the instances for the cloudprovider
func (ag *AutomiumAgent) RetrieveNodeStatus(targetCluster kube_client.Interface, serviceName string) ([]cloudprovider.Instance, error) {
	var instanceList []cloudprovider.Instance

	// Retrieve Kubernetes nodes
	kubernetesNodes, err := retrieveKubernetesNodes(targetCluster)
	if err != nil {
		return instanceList, err
	}

	// Retrieve Automium nodes
	automiumNodes, err := ag.GetNodesForService(serviceName)
	if err != nil {
		return instanceList, err
	}

	for _, kubeNode := range kubernetesNodes {
		for _, automiumNode := range automiumNodes {
			if kubeNode.Name == automiumNode.Spec.Hostname {
				klog.V(2).Infof("found Kubernetes worker: %s\n", kubeNode.Name)
				klog.V(2).Infof("spec: %+v\n", kubeNode.Spec)

				var tempInstance cloudprovider.Instance
				tempInstance.Id = kubeNode.Spec.ProviderID
				tempInstance.Status = ag.populateInstanceStatus(automiumNode)
				klog.V(2).Infof("instance ID: %s state: %d\n", tempInstance.Id, tempInstance.Status.State)
				instanceList = append(instanceList, tempInstance)
			}
		}
	}

	return instanceList, nil
}

func (ag *AutomiumAgent) populateInstanceStatus(node v1beta1.Node) *cloudprovider.InstanceStatus {
	if node.Spec.DeletionDate != "" {
		return &cloudprovider.InstanceStatus{
			State:     cloudprovider.InstanceDeleting,
			ErrorInfo: nil,
		}
	}

	if node.Status.NodeProperties.ID == "non-existent-machine" && node.Spec.DeletionDate == "" {
		return &cloudprovider.InstanceStatus{
			State:     cloudprovider.InstanceCreating,
			ErrorInfo: nil,
		}
	}

	return &cloudprovider.InstanceStatus{
		State:     cloudprovider.InstanceRunning,
		ErrorInfo: nil,
	}
}

func (ag *AutomiumAgent) nodeIsHealthy(node v1beta1.Node) bool {
	for _, chk := range node.Status.NodeHealthChecks {
		if chk.CheckID == "serfHealth" && chk.Status == "critical" {
			return false
		}
		return true
	}

	klog.Warningf("cannot find serfHealth check for node %s -- marking as unhealthy", node.GetName())
	return false
}

// GetWorkersFlavor returns the information about the workers flavor
func (ag *AutomiumAgent) GetWorkersFlavor(svcName string) (*FlavorData, error) {
	var services v1beta1.ServiceList
	var targetService *v1beta1.Service

	err := ag.kubeClient.Get().Resource("services").Do().Into(&services)
	if err != nil {
		return nil, err
	}

	for _, svc := range services.Items {
		if svc.GetName() == svcName {
			targetService = &svc
			break
		}
	}

	if targetService == nil {
		return nil, errors.New("service not found")
	}

	if flavData, ok := ECSFlavors[targetService.Spec.Flavor]; ok {
		return flavData, nil
	}

	return nil, errors.New("flavor not found")
}

func (ag *AutomiumAgent) retrieveAutoscalingServices(clusterName string) {
	var svcs v1beta1.ServiceList

	// Get all services
	err := ag.kubeClient.Get().Resource("services").Do().Into(&svcs)
	if err != nil {
		klog.Fatalf("cannot get Automium services: %s\n", err.Error())
	}

	// Check service app and environment
	for _, svc := range svcs.Items {
		klog.V(5).Infof("working on service %s for cluster %s\n", svc.Name, clusterName)
		autoscalingItem, err := generateAutoscalingDataForService(svc, clusterName)

		if err == nil {
			klog.V(5).Infof("autoscaling item: %+v\n", autoscalingItem)
			// Add service to autoscaling services list
			ag.autoscalingServices = append(ag.autoscalingServices, autoscalingItem)
		} else {
			klog.Errorf("skipping service %s: %s\n", svc.Name, err.Error())
		}

	}

}
