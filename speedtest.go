package main

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strings"
)

type SpeedResult struct {
	Download string `json:"download"`
	Upload   string `json:"upload"`
}

func executeSequentialSpeedtest(targets []string) map[string]SpeedResult {
	results := make(map[string]SpeedResult)

	for _, host := range targets {
		cmd := exec.Command("speedtest-cli", "--simple")
		out, err := cmd.Output()
		
		res := SpeedResult{Download: "Ошибка выполнения теста", Upload: "Ошибка выполнения теста"}
		if err == nil {
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "Download:") {
					res.Download = strings.TrimSpace(strings.TrimPrefix(line, "Download:"))
				}
				if strings.HasPrefix(line, "Upload:") {
					res.Upload = strings.TrimSpace(strings.TrimPrefix(line, "Upload:"))
				}
			}
		}
		results[host] = res
	}
	return results
}

func handleSpeedtest(w http.ResponseWriter, r *http.Request) {
	segment := r.URL.Query().Get("segment")
	
	var targets []string
	configMu.RLock()
	if segment == "ru" {
		targets = append([]string{}, config.RuTargets...)
	} else {
		targets = append([]string{}, config.IntlTargets...)
	}
	configMu.RUnlock()

	results := executeSequentialSpeedtest(targets)
	
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(results)
}