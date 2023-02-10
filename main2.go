package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	githubURL = "https://raw.githubusercontent.com/evilcloud/GPU_power_limit/main/gpu_power.json"
	errorFile = "errors.log"
)

type PowerLimit struct {
	Model string `json:"model"`
	Limit int    `json:"limit"`
}

type PowerLimits struct {
	PowerLimits []PowerLimit `json:"power_limits"`
}

type GPU struct {
	Number int
	Model  string
	UUID   string
}

type PowerLimitResponse struct {
	GPU      GPU
	OldLimit int
	NewLimit int
}

func executeCommand(command string) (string, error) {
	cmd := exec.Command("bash", "-c", command)
	// cmd := exec.Command("/bin/sh", "-c", command)
	output, err := cmd.Output()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("command not found")
		}
		return "", err
	}
	return string(output), nil
}

func saveToFile(fileName string, data []byte) error {
	err := ioutil.WriteFile(fileName, data, 0644)
	if err != nil {
		logError(err)
		return err
	}
	return nil
}

func getGithubData() (*PowerLimits, error) {
	res, err := http.Get(githubURL)
	if err != nil {
		logError(err)
		return nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		logError(err)
		return nil, err
	}

	var powerLimits PowerLimits
	err = json.Unmarshal(body, &powerLimits)
	if err != nil {
		logError(err)
		return nil, err
	}
	return &powerLimits, nil
}

func parseNvidiaSMI(line string) (*GPU, error) {
	headtail := strings.SplitN(line, ":", 2)
	if len(headtail) != 2 {
		err := fmt.Errorf("can not parse line: %s", line)
		logError(err)
		return nil, err
	}
	head := headtail[0]
	tail := headtail[1]

	num := strings.Fields(head)[1]
	model := strings.Split(tail, "(UUID:")[0]
	uuid := strings.TrimRight(strings.Split(tail, "(UUID:")[1], ")")

	number, err := strconv.Atoi(strings.TrimSpace(num))
	if err != nil {
		number = 999
	}

	return &GPU{
		Number: number,
		Model:  model,
		UUID:   uuid,
	}, nil
}

func getNvidiaSMI() ([]GPU, error) {
	output, err := executeCommand("nvidia-smi -L")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(output), "\n")

	gpus := make([]GPU, 0)
	for _, line := range lines {
		gpu, err := parseNvidiaSMI(line)
		if err != nil {
			logError(fmt.Errorf("failed to parse line: %s", line))
			continue
		}
		gpus = append(gpus, *gpu)
	}
	return gpus, nil
}

func setGPUPowerLimit(gpu GPU, limit int) error {
	command := fmt.Sprintf("nvidia-smi -i %d -pl %d", gpu.Number, limit)
	output, err := executeCommand(command)
	if err != nil {
		logError(err)
		return err
	}
	fmt.Println(string(output))
	return nil
}

func parseGPUPowerLimitOutput(output string) (int, int, int, error) {
	lines := strings.Split(output, "\n")
	var line string
	for _, line := range lines {
		if !strings.Contains(line, "Power limit for GPU") {
			continue
		}
		fields := strings.Fields(line)
		gpuNumber, err := strconv.Atoi(fields[4])
		if err != nil {
			gpuNumber = 999
		}
		oldLimit, err := strconv.Atoi(fields[8])
		if err != nil {
			oldLimit = 999
		}
		newLimit, err := strconv.Atoi(fields[11])
		if err != nil {
			newLimit = 999
		}
		return gpuNumber, oldLimit, newLimit, nil
	}
	return 0, 0, 0, fmt.Errorf("can not parse line: %s", line)
}

func logError(err error) {
	pc, _, _, _ := runtime.Caller(1)
	source := runtime.FuncForPC(pc).Name()

	logMessage := fmt.Sprintf("[ERROR] %s: %s\n", source, err)
	log.Print(logMessage)

	f, openFileErr := os.OpenFile("errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if openFileErr != nil {
		openFileErrorMessage := fmt.Sprintf("[ERROR] Failed to open errors.log: %s\n", openFileErr)
		log.Print(openFileErrorMessage)
		return
	}
	defer f.Close()

	_, writeError := f.WriteString(fmt.Sprintf("[%s] %s", time.Now().Format(time.RFC3339), logMessage))
	if writeError != nil {
		writeErrorMessage := fmt.Sprintf("[ERROR] Failed to write to errors.log: %s\n", writeError)
		log.Print(writeErrorMessage)
	}
}

func main() {
	powerLimits, err := getGithubData() // get the data from github
	if err != nil {
		logError(err) // log the error
		return
	}

	fmt.Printf("%d power limit settings found\n", len(powerLimits.PowerLimits)) // print the number of power limits found

	gpus, err := getNvidiaSMI()
	if err != nil {
		logError(err)
	}

	for _, gpu := range gpus {
		for _, powerLimit := range powerLimits.PowerLimits {
			if strings.Contains(gpu.Model, powerLimit.Model) {
				err := setGPUPowerLimit(gpu, powerLimit.Limit)
				if err != nil {
					logError(err)
				}
			}
		}
	}

}
