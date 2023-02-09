package smi

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

type GPU struct {
	Number int
	Model  string
	UUID   string
}

func GetNvidiaSMI() ([]GPU, error) {
	out, err := exec.Command("bash", "-c", "nvidia-smi -L").Output()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("nvidia-smi not found, please check if it is installed on your system")
		}
		return nil, err
	}

	// Parse the output of "nvidia-smi -L" to isolate the GPU number, model and UUID
	re := regexp.MustCompile(`GPU (\d+): ([\w ]+) \(UUID: GPU-[\w-]+\)`)
	matches := re.FindAllStringSubmatch(string(out), -1)

	var gpus []GPU
	for _, match := range matches {
		gpu := GPU{
			Number: strings.TrimSpace(match[1]),
			Model:  strings.TrimSpace(match[2]),
			UUID:   strings.TrimSpace(match[3]),
		}
		gpus = append(gpus, gpu)
	}
	return gpus, nil
}
