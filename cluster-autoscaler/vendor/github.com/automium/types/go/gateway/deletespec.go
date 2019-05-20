package gateway

type DeleteSpec struct {
	ServiceName string    `json:"name"`
	GitConfig   GitConfig `json:"git"`
}
