package powerlimits

import (
	"encoding/json"
	"net/http"

	"github.com/evilcloud/GPU_power_limit/exec"
)

const (
	githubURL = "https://raw.githubusercontent.com/evilcloud/GPU_power_limit/main/gpu_power.json"
)

type PowerLimit struct {
	Model string `json:"model"`
	Limit int    `json:"limit"`
}

type PowerLimits struct {
	PowerLimits []PowerLimit `json:"power_limits"`
}

func GetGithubData() (*PowerLimits, error) {
	res, err := http.Get(githubURL)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := exec.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var powerLimits PowerLimits
	err = json.Unmarshal(body, &powerLimits)
	if err != nil {
		return nil, err
	}

	return &powerLimits, nil
}
