package gateway

type ApplyService struct {
	Kubeconfig string `json:"kubeconfig"`
	Service Service `json:"service"`
}