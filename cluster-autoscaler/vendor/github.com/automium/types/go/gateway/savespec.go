package gateway

type SaveSpec struct {
	ServiceName string    `json:"name"`
	Service     Service   `json:"service"`
	GitConfig   GitConfig `json:"git"`
}
