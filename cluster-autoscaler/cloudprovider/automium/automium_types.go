package automium

import (
	"sync"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	kube_client "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	// ProviderName is the provider name
	ProviderName = "automium"
)

type automiumCloudProvider struct {
	nodeGroups      []*AutomiumServiceNodeGroup
	resourceLimiter *cloudprovider.ResourceLimiter
}

type AutomiumAutoscalingService struct {
	ServiceName   string
	MinCount      int
	MaxCount      int
	OperationLock *sync.Mutex
}

type AutomiumAgent struct {
	kubeClient          *rest.RESTClient
	autoscalingServices []AutomiumAutoscalingService
	lockMap             map[string]*sync.Mutex
}

type AutomiumServiceNodeGroup struct {
	serviceName             string
	minWorkers              int
	maxWorkers              int
	targetClusterConnection kube_client.Interface
	agentConnection         *AutomiumAgent
	workersLock             *sync.Mutex
	dryRun                  bool
	dryRunNodeCount         int
	dryRunInstanceList      []cloudprovider.Instance
	dryRunLock              *sync.Mutex
}

type CloudProviderConfig struct {
	AutomiumKubeConfigPath string `yaml:"automiumKubeconfig"`
	ClusterName            string `yaml:"clusterName"`
	DryRun                 bool   `yaml:"dryRun"`
}
