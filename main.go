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
	logFile   = "log.log"
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

type PowerLimitData struct {
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
	if !strings.Contains(line, ":") {
		err := fmt.Errorf("the line is not valid: %s", line)
		logError(err)
		return nil, err
	}
	headtail := strings.SplitN(line, ":", 2)
	if len(headtail) != 2 {
		err := fmt.Errorf("can not parse line: %s", line)
		logError(err)
		return nil, err
	}
	head := headtail[0]
	tail := headtail[1]

	num := strings.Fields(head)[1]
	model := strings.TrimSpace(strings.Split(tail, "(UUID:")[0])
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
		if !strings.Contains(line, "GPU") {
			continue
		}
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
	gpuNumber, oldLimit, newLimit, err := parseGPUPowerLimitOutput(output)
	if err != nil {
		logError(err)
		return err
	}
	fmt.Printf(" %d %s: %d -> %d\n", gpuNumber, gpu.Model, oldLimit, newLimit)
	// string with current date and time, gpu number, gpu model, old limit, new limit
	writeToFile(fmt.Sprintf(" %d %s: %d -> %d\n", gpuNumber, gpu.Model, oldLimit, newLimit))
	return nil
}

func strToInt(s string) int {
	if strings.Contains(s, ".") {
		s = strings.Split(s, ".")[0]
	}
	if strings.Contains(s, ":") {
		s = strings.Split(s, ":")[1]
		s, err := strconv.ParseInt(s, 16, 32)
		if err == nil {
			return int(s)
		}
	}
	i, err := strconv.Atoi(s)
	if err == nil {
		return i
	}
	return 999
}

func parseGPUPowerLimitOutput(output string) (int, int, int, error) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if !strings.Contains(line, "Power limit for GPU") {
			continue
		}
		fields := strings.Fields(line)

		gpuNumber := strToInt(fields[4])
		oldLimit := strToInt(fields[11])
		newLimit := strToInt(fields[8])

		return gpuNumber, oldLimit, newLimit, nil
	}
	return 0, 0, 0, fmt.Errorf("can not parse line: %s", output)
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

// function that writes into a file provided string by the user
// file name log.txt
func writeToFile(s string) {
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logError(err)
		return
	}
	defer f.Close()

	if _, err := f.WriteString(s); err != nil {
		logError(err)
		return
	}
}

func main() {
	start := time.Now()
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

	writeToFile(fmt.Sprintf("%s %d power limit settings found\n", time.Now().Format(time.RFC3339), len(powerLimits.PowerLimits)))
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
	elapsed := time.Since(start)
	fmt.Printf("Execution time: %s\n", (elapsed.Round(time.Millisecond)))
	writeToFile(fmt.Sprintf("Execution time: %s\n", (time.Since(start).Round(time.Millisecond))))

}
