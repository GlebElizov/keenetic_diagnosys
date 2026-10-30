package main

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
)

// Глобальная переменная для хранения версии. 
// Значение будет подменяться компилятором через -ldflags
var AppVersion = "0.0.0-dev"

//go:embed templates/*
var templatesFS embed.FS
var tmpl *template.Template

type PageData struct {
	DeviceName string
	Uptime     string
	User       string
	Version    string // Добавлено поле версии для передачи в шаблон
}

func main() {
	loadConfig()
	tmpl = template.Must(template.ParseFS(templatesFS, "templates/*.html"))

	// Веб-страницы интерфейса
	http.HandleFunc("/", authMiddleware(handleHome))
	http.HandleFunc("/login", handleLogin)
	http.HandleFunc("/logout", handleLogout)
	http.HandleFunc("/ping", authMiddleware(handlePingPage))
	http.HandleFunc("/speedtest", authMiddleware(handleSpeedtestPage))
	http.HandleFunc("/collect", authMiddleware(handleCollectPage))

	// API бизнес-логики
	http.HandleFunc("/api/config/get", authMiddleware(handleGetTargets))
	http.HandleFunc("/api/config/save", authMiddleware(handleSaveTargets))
	http.HandleFunc("/api/ping", authMiddleware(handlePing))
	http.HandleFunc("/api/speedtest", authMiddleware(handleSpeedtest))
	http.HandleFunc("/api/collect/download", authMiddleware(handleDownloadArchive))

	fmt.Printf("=== Сервер запущен (%s) на http://localhost:8080 ===\n", AppVersion)
	_ = http.ListenAndServe(":8080", nil)
}