package gateway

type ServiceLogs struct {
	ServiceName string    `json:"name"`
	Kubeconfig string `json:"kubeconfig"`
}