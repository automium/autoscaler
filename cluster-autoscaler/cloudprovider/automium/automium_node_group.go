package automium

import (
	"fmt"
	"log"
	"math/rand"
	"regexp"
	"strconv"
	"sync"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	kube_client "k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	schedulernodeinfo "k8s.io/kubernetes/pkg/scheduler/nodeinfo"
)

// MaxSize returns maximum size of the node group.
func (ng *AutomiumServiceNodeGroup) MaxSize() int {
	return ng.maxWorkers
}

// MinSize returns minimum size of the node group.
func (ng *AutomiumServiceNodeGroup) MinSize() int {
	return ng.minWorkers
}

// TargetSize returns the current target size of the node group. It is possible that the
// number of nodes in Kubernetes is different at the moment but should be equal
// to Size() once everything stabilizes (new nodes finish startup and registration or
// removed nodes are deleted completely). Implementation required.
func (ng *AutomiumServiceNodeGroup) TargetSize() (int, error) {
	if ng.dryRun {
		ng.dryRunLock.Lock()
		defer ng.dryRunLock.Unlock()
		klog.Infof("dry run mode - workers count: %d\n", ng.dryRunNodeCount)
		return ng.dryRunNodeCount, nil
	}

	ng.workersLock.Lock()
	defer ng.workersLock.Unlock()
	count, err := ng.agentConnection.RetrieveActiveWorkersCount(ng.serviceName)
	if err != nil {
		return ng.MinSize(), err
	}
	return count, nil
}

// IncreaseSize increases the size of the node group. To delete a node you need
// to explicitly name it and use DeleteNode. This function should wait until
// node group size is updated. Implementation required.
func (ng *AutomiumServiceNodeGroup) IncreaseSize(delta int) error {
	klog.V(1).Infof("increase requested with delta %d\n", delta)
	if ng.dryRun {
		klog.Infof("dry run mode - increasing worker \n")
		ng.dryRunLock.Lock()
		defer ng.dryRunLock.Unlock()
		ng.dryRunNodeCount += delta
		ng.dryRunInstanceList = ng.populateFakeInstanceList()
		klog.Infof("dry run mode - increased worker count: %d\n", ng.dryRunNodeCount)
		return nil
	}

	currentServiceReplicas, err := ng.agentConnection.RetrieveWorkersCount(ng.serviceName)
	if err != nil {
		return fmt.Errorf("cannot retrieve replicas for service %s: %s", ng.serviceName, err.Error())
	}

	klog.V(5).Infof("[svc: %s] IncreaseSize: current replicas: %d\n", ng.serviceName, currentServiceReplicas)
	klog.V(5).Infof("[svc: %s] IncreaseSize: delta replicas: %d\n", ng.serviceName, delta)
	klog.V(5).Infof("[svc: %s] IncreaseSize: max replicas allowed for service: %d\n", ng.serviceName, ng.maxWorkers)
	klog.V(5).Infof("[svc: %s] IncreaseSize: desired total replicas: %d\n", ng.serviceName, currentServiceReplicas+delta)

	if currentServiceReplicas+delta > ng.maxWorkers {
		klog.Errorf("[svc: %s] IncreaseSize: too many nodes requested -  desired: %d, max: %d\n", ng.serviceName, currentServiceReplicas+delta, ng.maxWorkers)
		return fmt.Errorf("size increase too large - desired: %d, max: %d", currentServiceReplicas+delta, ng.maxWorkers)
	}

	ng.workersLock.Lock()
	defer ng.workersLock.Unlock()

	return ng.agentConnection.EditWorkersCount(ng.serviceName, delta)
}

// KubeNodeBelongsToMe check if the provided Kubernetes node belongs to this nodegroup
func (ng *AutomiumServiceNodeGroup) KubeNodeBelongsToMe(kubeNode *apiv1.Node) (bool, error) {

	automiumNodes, err := ng.agentConnection.GetNodesForService(ng.serviceName)
	if err != nil {
		klog.Errorf("cannot get nodes for service %s: %s\n", ng.serviceName, err.Error())
		return false, err
	}

	for _, automiumNode := range automiumNodes {
		if kubeNode.Name == automiumNode.Spec.Hostname {
			return true, nil
		}
	}

	return false, nil
}

// DeleteNodes deletes nodes from this node group. Error is returned either on
// failure or if the given node doesn't belong to this node group. This function
// should wait until node group size is updated. Implementation required.
func (ng *AutomiumServiceNodeGroup) DeleteNodes(nodesToDel []*apiv1.Node) error {

	if ng.dryRun {
		klog.Infof("dry run mode - decreasing worker count\n")
		ng.dryRunLock.Lock()
		defer ng.dryRunLock.Unlock()
		ng.dryRunNodeCount -= len(nodesToDel)
		ng.dryRunInstanceList = ng.populateFakeInstanceList()
		klog.Infof("dry run mode - decreased worker count: %d\n", ng.dryRunNodeCount)
		return nil
	}

	var serviceNodes []*apiv1.Node

	automiumNodes, err := ng.agentConnection.GetNodesForService(ng.serviceName)
	if err != nil {
		klog.Errorf("cannot retrieve automium nodes: %s\n", err.Error())
		return err
	}

	for _, kubeNode := range nodesToDel {
		for _, automiumNode := range automiumNodes {
			if kubeNode.Name == automiumNode.Spec.Hostname {
				serviceNodes = append(serviceNodes, kubeNode)
			}
		}
	}

	nodesWhichCanBeDeleted := ng.validateDeleteNodes(serviceNodes)
	if len(nodesWhichCanBeDeleted) == 0 {
		klog.Infoln("trying to scale down but all nodes chosen are reserved nodes -- skipping")
		return nil
	}
	numbOfNodesToDel := len(nodesWhichCanBeDeleted)
	klog.V(1).Infof("decrease requested with delta %d\n", numbOfNodesToDel)

	ng.workersLock.Lock()
	defer ng.workersLock.Unlock()

	return ng.agentConnection.EditWorkersCount(ng.serviceName, -numbOfNodesToDel)
}

func (ng *AutomiumServiceNodeGroup) validateDeleteNodes(nodes []*apiv1.Node) []*apiv1.Node {
	retArray := make([]*apiv1.Node, 0)
	numberRegex := regexp.MustCompile("[0-9]+$")

	for _, node := range nodes {
		nodeNumber, err := strconv.Atoi(numberRegex.FindString(node.Name))
		if err != nil {
			klog.Warningf("cannot get number from node %s: %s -- ignoring node.\n", node.Name, err.Error())
		}
		klog.V(5).Infof("node: %s - number: %d\n", node.Name, nodeNumber)

		if nodeNumber < ng.minWorkers {
			klog.V(2).Infof("node %s is in range -- not counting for deletion.\n", node.Name)
		} else {
			klog.V(2).Infof("node %s is not in range -- counting for deletion.\n", node.Name)
			retArray = append(retArray, node)
		}
	}

	return retArray
}

// DecreaseTargetSize decreases the target size of the node group. This function
// doesn't permit to delete any existing node and can be used only to reduce the
// request for new nodes that have not been yet fulfilled. Delta should be negative.
// It is assumed that cloud provider will not delete the existing nodes when there
// is an option to just decrease the target. Implementation required.
func (ng *AutomiumServiceNodeGroup) DecreaseTargetSize(delta int) error {
	// TODO
	log.Printf("[NodeGroup] DecreaseTargetSize called with delta: %d\n", delta)
	return nil
}

// Id returns an unique identifier of the node group.
func (ng *AutomiumServiceNodeGroup) Id() string {
	return fmt.Sprintf("%s-nodegroup", ng.serviceName)
}

// Debug returns a string containing all information regarding this node group.
func (ng *AutomiumServiceNodeGroup) Debug() string {
	return fmt.Sprintf("%+v", ng)
}

// Nodes returns a list of all nodes that belong to this node group.
// It is required that Instance objects returned by this method have Id field set.
// Other fields are optional.
func (ng *AutomiumServiceNodeGroup) Nodes() ([]cloudprovider.Instance, error) {
	if ng.dryRun {
		return ng.dryRunInstanceList, nil
	}
	ng.workersLock.Lock()
	defer ng.workersLock.Unlock()

	return ng.agentConnection.RetrieveNodeStatus(ng.targetClusterConnection, ng.serviceName)
}

// TemplateNodeInfo returns a schedulernodeinfo.NodeInfo structure of an empty
// (as if just started) node. This will be used in scale-up simulations to
// predict what would a new node look like if a node group was expanded. The returned
// NodeInfo is expected to have a fully populated Node object, with all of the labels,
// capacity and allocatable information as well as all pods that are started on
// the node by default, using manifest (most likely only kube-proxy). Implementation optional.
func (ng *AutomiumServiceNodeGroup) TemplateNodeInfo() (*schedulernodeinfo.NodeInfo, error) {
	node, err := ng.buildTemplateNode()
	if err != nil {
		return nil, err
	}
	nodeInfo := schedulernodeinfo.NewNodeInfo(cloudprovider.BuildKubeProxy("kube-proxy"))
	nodeInfo.SetNode(node)
	klog.V(5).Infof("template node: %+v\n", node)
	return nodeInfo, nil
}

// Exist checks if the node group really exists on the cloud provider side. Allows to tell the
// theoretical node group from the real one. Implementation required.
func (ng *AutomiumServiceNodeGroup) Exist() bool {
	return true
}

// Create creates the node group on the cloud provider side. Implementation optional.
func (ng *AutomiumServiceNodeGroup) Create() (cloudprovider.NodeGroup, error) {
	return nil, cloudprovider.ErrNotImplemented
}

// Delete deletes the node group on the cloud provider side.
// This will be executed only for autoprovisioned node groups, once their size drops to 0.
// Implementation optional.
func (ng *AutomiumServiceNodeGroup) Delete() error {
	return nil
}

// Autoprovisioned returns true if the node group is autoprovisioned. An autoprovisioned group
// was created by CA and can be deleted when scaled to 0.
func (ng *AutomiumServiceNodeGroup) Autoprovisioned() bool {
	return false
}

func (ng *AutomiumServiceNodeGroup) populateFakeInstanceList() []cloudprovider.Instance {
	retArray := make([]cloudprovider.Instance, 0)
	for i := 0; i < ng.dryRunNodeCount; i++ {
		var tmpNode cloudprovider.Instance
		tmpNode.Id = fmt.Sprintf("kubernetes-workers-%d", i)
		tmpNode.Status = &cloudprovider.InstanceStatus{
			State: cloudprovider.InstanceRunning,
		}
		retArray = append(retArray, tmpNode)
	}
	return retArray
}

func (ng *AutomiumServiceNodeGroup) buildTemplateNode() (*apiv1.Node, error) {
	klog.V(5).Infoln("template node requested")
	node := apiv1.Node{}

	flavorData, err := ng.agentConnection.GetWorkersFlavor(ng.serviceName)
	if err != nil {
		return nil, err
	}

	klog.V(5).Infof("template node flavor data: %+v\n", &flavorData)

	nodeName := fmt.Sprintf("kubernetes-worker-tmpl-%d", rand.Int63())

	node.ObjectMeta = metav1.ObjectMeta{
		Name:     nodeName,
		SelfLink: fmt.Sprintf("/api/v1/nodes/%s", nodeName),
		Labels:   map[string]string{},
	}

	capacity := apiv1.ResourceList{}
	capacity[apiv1.ResourcePods] = *resource.NewQuantity(110, resource.DecimalSI)
	capacity[apiv1.ResourceCPU] = *resource.NewQuantity(flavorData.CPUs, resource.DecimalSI)
	capacity[apiv1.ResourceMemory] = *resource.NewQuantity(flavorData.Memory, resource.DecimalSI)

	node.Status = apiv1.NodeStatus{
		Capacity: capacity,
	}

	// Ready status
	node.Status.Conditions = cloudprovider.BuildReadyConditions()
	return &node, nil
}

// CreateAutomiumNodeGroup creates a new nodegroup for a specific service
func CreateAutomiumNodeGroup(agent *AutomiumAgent, serviceName string, clusterConnection kube_client.Interface, minWorkers, maxWorkers int, dryRun bool) *AutomiumServiceNodeGroup {
	ng := new(AutomiumServiceNodeGroup)
	ng.agentConnection = agent
	ng.serviceName = serviceName
	ng.targetClusterConnection = clusterConnection
	ng.minWorkers = minWorkers
	ng.maxWorkers = maxWorkers
	ng.workersLock = &sync.Mutex{}
	ng.dryRun = dryRun
	if dryRun {
		ng.dryRunNodeCount = minWorkers
		ng.dryRunInstanceList = ng.populateFakeInstanceList()
		ng.dryRunLock = &sync.Mutex{}
	}
	return ng
}
