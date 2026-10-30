package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

const configFile = "config.json"

type Config struct {
	RuTargets   []string `json:"ru_targets"`
	IntlTargets []string `json:"intl_targets"`
}

var (
	config         Config
	configMu       sync.RWMutex
	defaultRu      = []string{"yandex.ru", "mail.ru"}
	defaultIntl    = []string{"8.8.8.8", "google.com"}
)

func loadConfig() {
	configMu.Lock()
	defer configMu.Unlock()

	file, err := os.ReadFile(configFile)
	if err != nil {
		config.RuTargets = defaultRu
		config.IntlTargets = defaultIntl
		saveConfigNoLock()
		return
	}

	if err := json.Unmarshal(file, &config); err != nil {
		fmt.Printf("Ошибка чтения конфигурации: %v\n", err)
		config.RuTargets = defaultRu
		config.IntlTargets = defaultIntl
	}
}

func saveConfigNoLock() {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(configFile, data, 0644)
}