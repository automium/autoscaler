package automium

import (
	"k8s.io/autoscaler/cluster-autoscaler/utils/units"
)

type FlavorData struct {
	FlavorID int
	CPUs     int64
	Memory   int64
}

var ECSFlavors = map[string]*FlavorData{
	"e3standard.x1": &FlavorData{
		FlavorID: 310,
		CPUs:     1,
		Memory:   512 * units.MB,
	},
	"e3standard.x2": &FlavorData{
		FlavorID: 320,
		CPUs:     1,
		Memory:   1024 * units.MB,
	},
	"e3standard.x3": &FlavorData{
		FlavorID: 330,
		CPUs:     2,
		Memory:   2048 * units.MB,
	},
	"e3standard.x4": &FlavorData{
		FlavorID: 340,
		CPUs:     2,
		Memory:   4096 * units.MB,
	},
	"e3standard.x5": &FlavorData{
		FlavorID: 350,
		CPUs:     4,
		Memory:   8192 * units.MB,
	},
	"e3standard.x6": &FlavorData{
		FlavorID: 360,
		CPUs:     8,
		Memory:   16384 * units.MB,
	},
	"e3standard.x7": &FlavorData{
		FlavorID: 370,
		CPUs:     8,
		Memory:   32768 * units.MB,
	},
	"e3standard.x8": &FlavorData{
		FlavorID: 380,
		CPUs:     16,
		Memory:   65536 * units.MB,
	},
	"e3standard.x9": &FlavorData{
		FlavorID: 390,
		CPUs:     24,
		Memory:   131072 * units.MB,
	},
	"o1.highcpu-spot.x1": &FlavorData{
		FlavorID: 410,
		CPUs:     4,
		Memory:   4096 * units.MB,
	},
	"o1.highcpu-spot.x2": &FlavorData{
		FlavorID: 420,
		CPUs:     8,
		Memory:   8192 * units.MB,
	},
	"o1.highcpu-spot.x3": &FlavorData{
		FlavorID: 430,
		CPUs:     16,
		Memory:   16384 * units.MB,
	},
}
