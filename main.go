package main

import (
	"log"

	"github.com/evilcloud/GPU_power_limit/powerlimits"
	"github.com/evilcloud/GPU_power_limit/smi"
)

func main() {
	powerLimits, err := powerlimits.GetGithubData()
	if err != nil {
		log.Fatalf("Error getting power limits: %v", err)
	}

	gpus, err := smi.GetNvidiaSMI()
	if err != nil {
		log.Fatalf("Error getting GPUs: %v", err)
	}

	err = smi.SetPowerLimits(powerLimits, gpus)
	if err != nil {
		log.Fatalf("Error setting power limits: %v", err)
	}
}
