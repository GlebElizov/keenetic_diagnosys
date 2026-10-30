package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// Структура для парсинга ответа show version от Keenetic
type KeeneticVersionResponse struct {
	Model   string `json:"model"`
	Release string `json:"release"`
}

func handleCollectPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := PageData{
		DeviceName: "Keenetic Device",
		Uptime:     "Архивация",
		User:       "admin",
	}
	_ = tmpl.ExecuteTemplate(w, "collect.html", data)
}

func getNetworkTime(ntpServer string) string {
	conn, err := net.DialTimeout("udp", ntpServer+":123", 3*time.Second)
	if err != nil {
		return fmt.Sprintf("Ошибка подключения к NTP (%s)", err.Error())
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		return "Ошибка таймаута"
	}

	msg := make([]byte, 48)
	msg[0] = 0x1B 

	if _, err := conn.Write(msg); err != nil {
		return "Ошибка отправки UDP"
	}

	if _, err := conn.Read(msg); err != nil {
		return "Ошибка получения UDP"
	}

	secs := binary.BigEndian.Uint32(msg[40:44])
	const ntpEpochOffset = 2208988800
	unixSecs := int64(secs) - ntpEpochOffset

	t := time.Unix(unixSecs, 0)
	return t.Local().Format("2006-01-02 15:04:05")
}

func handleDownloadArchive(w http.ResponseWriter, r *http.Request) {
	scope := r.URL.Query().Get("scope")

	var pingTargets []string
	var speedTargets []string

	configMu.RLock()
	if scope == "ru" {
		pingTargets = append([]string{}, config.RuTargets...)
		speedTargets = append([]string{}, config.RuTargets...)
	} else if scope == "intl" {
		pingTargets = append([]string{}, config.IntlTargets...)
		speedTargets = append([]string{}, config.IntlTargets...)
	} else { 
		pingTargets = append(config.RuTargets, config.IntlTargets...)
		speedTargets = append([]string{}, config.IntlTargets...)
	}
	configMu.RUnlock()

	// Динамически запрашиваем модель и прошивку у роутера
	routerName := "Keenetic (Не определено)"
	firmwareVersion := "Не определено"
	versionRaw, err := exec.Command("ndmq", "-p", "show version", "-u", "admin").Output()
	if err == nil {
		var kv KeeneticVersionResponse
		if json.Unmarshal(versionRaw, &kv) == nil {
			if kv.Model != "" { routerName = kv.Model }
			if kv.Release != "" { firmwareVersion = kv.Release }
		}
	}

	systemTime := time.Now().Format("2006-01-02 15:04:05")
	networkTime := getNetworkTime("pool.ntp.org")
	
	wgStatusRaw, err := exec.Command("ndmq", "-p", "show wireguard status", "-u", "admin").Output()
	wgStatus := string(wgStatusRaw)
	if err != nil || strings.TrimSpace(wgStatus) == "" {
		wgStatus = "Wireguard client status: Интерфейс отключен или ndmq недоступен"
	}

	if len(pingTargets) > 0 {
		executeSerialParallelPing(pingTargets, 4, 1)
	}

	pingResults := make(map[string]float64)
	pingHistoryMu.Lock()
	for _, host := range pingTargets {
		hist := pingHistory[host]
		if len(hist) > 0 {
			pingResults[host] = hist[len(hist)-1]
		} else {
			pingResults[host] = -1
		}
	}
	pingHistoryMu.Unlock()

	speedResults := executeSequentialSpeedtest(speedTargets)

	var report bytes.Buffer
	report.WriteString("=== ДИАГНОСТИЧЕСКИЙ ОТЧЕТ KEENETIC ===\n")
	report.WriteString(fmt.Sprintf("Роутер: %s\n", routerName))
	report.WriteString(fmt.Sprintf("Версия прошивки: %s\n", firmwareVersion))
	report.WriteString(fmt.Sprintf("Локальное системное время: %s\n", systemTime))
	report.WriteString(fmt.Sprintf("Сетевое точное время (NTP):  %s\n\n", networkTime))
	
	report.WriteString("=== СТАТУС WIREGUARD ===\n")
	report.WriteString(wgStatus + "\n\n")

	report.WriteString("=== РЕЗУЛЬТАТЫ ПИНГА ===\n")
	for host, delay := range pingResults {
		if delay < 0 {
			report.WriteString(fmt.Sprintf("%s: Узел недоступен\n", host))
		} else {
			report.WriteString(fmt.Sprintf("%s: %.2f ms\n", host, delay))
		}
	}
	report.WriteString("\n")

	report.WriteString("=== РЕЗУЛЬТАТЫ СКОРОСТИ ===\n")
	for host, speed := range speedResults {
		report.WriteString(fmt.Sprintf("%s -> Download: %s | Upload: %s\n", host, speed.Download, speed.Upload))
	}

	var tarGzBuf bytes.Buffer
	gw := gzip.NewWriter(&tarGzBuf)
	tw := tar.NewWriter(gw)

	header := &tar.Header{
		Name: "diagnostic_report.txt",
		Mode: 0600,
		Size: int64(report.Len()),
	}

	_ = tw.WriteHeader(header)
	_, _ = tw.Write(report.Bytes())

	_ = tw.Close()
	_ = gw.Close()

	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=keenetic_diag_%s.tar.gz", scope))
	_, _ = w.Write(tarGzBuf.Bytes())
}