package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	pingHistory   = make(map[string][]float64)
	pingHistoryMu sync.Mutex
)

func executeSingleParallelPingCycle(targets []string, maxWorkers int) map[string]float64 {
	results := make(map[string]float64)
	var mu sync.Mutex
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	for _, target := range targets {
		if target == "" {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}

		go func(host string) {
			defer wg.Done()
			defer func() { 
				<-sem
				if r := recover(); r != nil {
					fmt.Printf("[Паника] Ошибка хоста %s: %v\n", host, r)
				}
			}() 

			cmd := exec.Command("ping", "-c", "1", "-W", "2", host)
			out, err := cmd.Output()
			
			var duration float64 = -1
			if err == nil {
				outStr := string(out)
				if idx := strings.Index(outStr, "time="); idx != -1 {
					sub := outStr[idx+5:]
					endIdx := strings.IndexAny(sub, " m")
					if endIdx != -1 {
						val, parseErr := strconv.ParseFloat(sub[:endIdx], 64)
						if parseErr == nil {
							duration = val
						}
					}
				}
			}

			mu.Lock()
			results[host] = duration
			mu.Unlock()
		}(target)
	}
	wg.Wait()
	return results
}

func executeSerialParallelPing(targets []string, maxWorkers int, iterations int) {
	pingHistoryMu.Lock()
	for _, host := range targets {
		pingHistory[host] = []float64{}
	}
	pingHistoryMu.Unlock()

	for i := 0; i < iterations; i++ {
		cycleResults := executeSingleParallelPingCycle(targets, maxWorkers)

		pingHistoryMu.Lock()
		for host, val := range cycleResults {
			graphVal := val
			if val < 0 {
				graphVal = 0 
			}
			pingHistory[host] = append(pingHistory[host], graphVal)
		}
		pingHistoryMu.Unlock()

		if i < iterations-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func handlePing(w http.ResponseWriter, r *http.Request) {
	segment := r.URL.Query().Get("segment")
	countStr := r.URL.Query().Get("count")
	
	iterations, err := strconv.Atoi(countStr)
	if err != nil || iterations <= 0 {
		iterations = 10
	}

	var targets []string
	configMu.RLock()
	if segment == "ru" {
		targets = append([]string{}, config.RuTargets...)
	} else {
		targets = append([]string{}, config.IntlTargets...)
	}
	configMu.RUnlock()

	if len(targets) > 0 {
		executeSerialParallelPing(targets, 4, iterations)
	}

	w.Header().Set("Content-Type", "application/json")
	response := make(map[string][]float64)
	
	pingHistoryMu.Lock()
	for _, host := range targets {
		response[host] = pingHistory[host]
	}
	pingHistoryMu.Unlock()

	_ = json.NewEncoder(w).Encode(response)
}