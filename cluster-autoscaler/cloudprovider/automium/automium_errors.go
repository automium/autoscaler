package automium

import "errors"

// ErrNotAutoscalingService is the error returned when a service must not be autoscaled.
var ErrNotAutoscalingService = errors.New("not an autoscaling service")
