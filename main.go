package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
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

func executeCommand(command string) (string, error) {
	cmd := exec.Command("bash", "-c", command)
	output, err := cmd.Output()
	if err != nil {
		logError(err)
		return "", err
	}
	return string(output), nil
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

func getNvidiaSMI() ([]GPU, error) {
	cmd := exec.Command("bash", "-c", "nvidia-smi -L")
	out, err := cmd.Output()
	if err != nil {
		logError(err)
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
		fmt.Println(match)
		return nil, nil
		number, err := strconv.Atoi(strings.TrimSpace(match[1]))
		if err != nil {
			logError(err)
			return nil, err
		}
		gpu := GPU{
			Number: number,
			Model:  strings.TrimSpace(match[2]),
			UUID:   strings.TrimSpace(match[3]),
		}

		gpus = append(gpus, gpu)
	}
	return gpus, nil
}

// set the power limit for each GPU as per prived power limits data
func setPowerLimits(powerLimits *PowerLimits, gpus []GPU) error {
	for _, gpu := range gpus {
		for _, powerLimit := range powerLimits.PowerLimits {
			if gpu.Model == powerLimit.Model {
				// set the power limit for the GPU
				command := fmt.Sprintf("nvidia-smi -i %d -pl %d", gpu.Number, powerLimit.Limit)
				_, err := executeCommand(command)
				if err != nil {
					logError(err)
					return err
				}
			}
		}
	}
	return nil
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
	powerLimits, err := getGithubData()
	if err != nil {
		logError(err)
		log.Fatal(err)
	}
	gpus, err := getNvidiaSMI()
	if err != nil {
		logError(err)
		log.Fatal(err)
	}
	err = setPowerLimits(powerLimits, gpus)
	if err != nil {
		logError(err)
		log.Fatal(err)
	}
	fmt.Println("Successfully set the power limits")
}
