package gateway

import (
	corev1 "k8s.io/api/core/v1"
)

type Service struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Labels struct {
			App string `json:"app"`
		} `json:"labels"`
		Name string `json:"name"`
	} `json:"metadata"`
	Spec ServiceSpec `json:"spec"`
}

type ServiceSpec struct {
	Replicas int             `json:"replicas"`
	Flavor   string          `json:"flavor"`
	Version  string          `json:"version"`
	Tags     []string        `json:"tags"`
	Env      []corev1.EnvVar `json:"env"`
	Extra    []ExtraService  `json:"extra"`
}

type ExtraService struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}
