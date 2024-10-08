package config

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
)

const configPath = ".cmd/config.json"

const (
	ProviderGroq   = "groq"
	ProviderCohere = "cohere"

	ModelTypeChat         = "chat"
	ModelTypeChatImage    = "chat-image"
	ModelTypeSpeechToText = "stt"
)

type Config struct {
	Provider string `json:"provider,omitempty"`

	Record     bool              `json:"record,omitempty"`
	Model      map[string]string `json:"model,omitempty"`
	Connectors []string          `json:"connectors,omitempty"`

	// Sampling parameters
	Temperature      *float64 `json:"temperature,omitempty"`
	TopP             *float64 `json:"top_p,omitempty"`
	TopK             *int     `json:"top_k,omitempty"`
	FrequencyPenalty *float64 `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64 `json:"presence_penalty,omitempty"`
}

func ReadConfig() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}
	cfg := &Config{
		// default to groq
		Provider: ProviderGroq,
		// record by default
		Record: true,
		Model:  make(map[string]string),
	}
	data, err := os.ReadFile(path)
	if err != nil {
		// no config file, return default config
		return cfg, nil
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func WriteConfig(cfg *Config) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), fs.ModePerm); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func ConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, configPath), nil
}

func Ref(v string) *string {
	return &v
}
