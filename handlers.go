package main

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strings"
)

func handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	uptimeRaw, _ := exec.Command("uptime").Output()
	uptime := strings.TrimSpace(string(uptimeRaw))
	if uptime == "" { uptime = "00:00:00 (эмуляция)" }

	routerName := "Keenetic Device"
	versionRaw, err := exec.Command("ndmq", "-p", "show version", "-u", "admin").Output()
	if err == nil {
		var kv struct { Model string `json:"model"` }
		if json.Unmarshal(versionRaw, &kv) == nil && kv.Model != "" {
			routerName = kv.Model
		}
	}

	data := PageData{
		DeviceName: routerName,
		Uptime:     uptime,
		User:       "admin",
		Version:    AppVersion,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmpl.ExecuteTemplate(w, "home.html", data)
}

func handlePingPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmpl.ExecuteTemplate(w, "ping.html", nil)
}

func handleSpeedtestPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmpl.ExecuteTemplate(w, "speedtest.html", nil)
}

func handleGetTargets(w http.ResponseWriter, r *http.Request) {
	configMu.RLock()
	defer configMu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(config)
}

func handleSaveTargets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req Config
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	configMu.Lock()
	config.RuTargets = req.RuTargets
	config.IntlTargets = req.IntlTargets
	saveConfigNoLock()
	configMu.Unlock()
	w.Write([]byte("OK"))
}