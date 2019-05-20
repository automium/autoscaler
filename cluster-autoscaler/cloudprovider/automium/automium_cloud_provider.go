package automium

import (
	"flag"
	"io/ioutil"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	cloudprovider "k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	"k8s.io/autoscaler/cluster-autoscaler/config"
	kubeerrors "k8s.io/autoscaler/cluster-autoscaler/utils/errors"
	kube_client "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"k8s.io/klog"
)

// Name returns name of the cloud provider.
func (acp *automiumCloudProvider) Name() string {
	return ProviderName
}

// NodeGroups returns all node groups configured for this cloud provider.
func (acp *automiumCloudProvider) NodeGroups() []cloudprovider.NodeGroup {
	ngs := make([]cloudprovider.NodeGroup, len(acp.nodeGroups))
	for i, asg := range acp.nodeGroups {
		ngs[i] = asg
	}
	return ngs
}

// NodeGroupForNode returns the node group for the given node, nil if the node
// should not be processed by cluster autoscaler, or non-nil error if such
// occurred. Must be implemented.
func (acp *automiumCloudProvider) NodeGroupForNode(node *apiv1.Node) (cloudprovider.NodeGroup, error) {
	// TODO: for now all workers belong to the default node group

	for _, ng := range acp.nodeGroups {
		belongs, err := ng.KubeNodeBelongsToMe(node)
		if err != nil {
			return nil, err
		}
		if belongs {
			return ng, nil
		}
	}

	klog.V(2).Infof("node %s does not belong to any node groups, ignoring\n", node.Name)
	return nil, nil
}

// Pricing returns pricing model for this cloud provider or error if not available.
// Implementation optional.
func (acp *automiumCloudProvider) Pricing() (cloudprovider.PricingModel, kubeerrors.AutoscalerError) {
	return nil, cloudprovider.ErrNotImplemented
}

// GetAvailableMachineTypes get all machine types that can be requested from the cloud provider.
// Implementation optional.
func (acp *automiumCloudProvider) GetAvailableMachineTypes() ([]string, error) {
	return nil, cloudprovider.ErrNotImplemented
}

// NewNodeGroup builds a theoretical node group based on the node definition provided. The node group is not automatically
// created on the cloud provider side. The node group is not returned by NodeGroups() until it is created.
// Implementation optional.
func (acp *automiumCloudProvider) NewNodeGroup(machineType string, labels map[string]string, systemLabels map[string]string,
	taints []apiv1.Taint, extraResources map[string]resource.Quantity) (cloudprovider.NodeGroup, error) {
	return nil, cloudprovider.ErrNotImplemented
}

// GetResourceLimiter returns struct containing limits (max, min) for resources (cores, memory etc.).
func (acp *automiumCloudProvider) GetResourceLimiter() (*cloudprovider.ResourceLimiter, error) {
	return acp.resourceLimiter, nil
}

// Cleanup cleans up open resources before the cloud provider is destroyed, i.e. go routines etc.
func (acp *automiumCloudProvider) Cleanup() error {
	// TODO
	return nil
}

// Refresh is called before every main loop and can be used to dynamically update cloud provider state.
// In particular the list of node groups returned by NodeGroups can change as a result of CloudProvider.Refresh().
func (acp *automiumCloudProvider) Refresh() error {
	// TODO
	return nil
}

// BuildAutomiumCloudProvider builds the cloud provider
func BuildAutomiumCloudProvider(automiumProviderConfigPath string, rl *cloudprovider.ResourceLimiter) (cloudprovider.CloudProvider, error) {
	acp := new(automiumCloudProvider)
	conf, err := ReadProviderConfiguration(automiumProviderConfigPath)
	if err != nil {
		return nil, err
	}

	klog.V(5).Infof("agent configuration: %+v\n", conf)

	// Check for cluster kubeconfig
	clusterKubeconfigFlag := flag.Lookup("kubeconfig")
	if err != nil {
		klog.Fatalf("cannot lookup cluster kubeconfig flag: %v\n", err)
	}

	var clusterClient kube_client.Interface
	clusterKubeconfigPath := clusterKubeconfigFlag.Value.String()
	if clusterKubeconfigPath != "" {
		klog.V(1).Infof("using cluster kubeconfig file: %s", clusterKubeconfigPath)
		clusterKubeConfig, err := ioutil.ReadFile(clusterKubeconfigPath)
		if err != nil {
			klog.Fatalf("cannot read kubeconfig file: %v\n", err)
		}
		config, err := clientcmd.RESTConfigFromKubeConfig(clusterKubeConfig)
		if err != nil {
			klog.Fatalf("cannot create REST config for cluster connection: %v\n", err)
		}
		clusterClient = kube_client.NewForConfigOrDie(config)
		klog.Infof("created client object for target cluster")
	} else {
		klog.Fatalf("empty cluster kubeconfig path, cannot continue.\n")
	}

	if conf.DryRun {
		klog.Infof("dry run enabled, no edits will be applied to the service")
	}

	automiumClient, err := CreateAutomiumAgentConnection(conf.AutomiumKubeConfigPath, conf.ClusterName)
	if err != nil {
		return nil, err
	}

	for _, as := range automiumClient.autoscalingServices {
		acp.nodeGroups = append(acp.nodeGroups, CreateAutomiumNodeGroup(automiumClient, as.ServiceName, clusterClient, as.MinCount, as.MaxCount, conf.DryRun))
	}

	acp.resourceLimiter = rl
	return acp, nil
}

// BuildAutomium builds automium cloud provider, manager etc.
func BuildAutomium(opts config.AutoscalingOptions, do cloudprovider.NodeGroupDiscoveryOptions, rl *cloudprovider.ResourceLimiter) cloudprovider.CloudProvider {

	if opts.CloudConfig == "" {
		panic("no Automium config specified")
	}

	cp, err := BuildAutomiumCloudProvider(opts.CloudConfig, rl)
	if err != nil {
		panic(err)
	}

	klog.V(1).Infoln("Automium cluster autoscaler initialized")
	return cp
}
