package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	LLM struct {
		APIKey   string `json:"api_key"`
		Model    string `json:"model"`
		Endpoint string `json:"endpoint"`
	} `json:"llm"`
}

func Load(configPath string) *Config {
	cfg := defaultConfig()

	if configPath == "" {
		return cfg
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return cfg
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return cfg
	}

	return cfg
}

func defaultConfig() *Config {
	cfg := &Config{}

	cfg.LLM.Model = os.Getenv("VOGTE_MODEL")
	if cfg.LLM.Model == "" {
		cfg.LLM.Model = "gpt-5"
	}
	cfg.LLM.Model = "gpt-5"
	cfg.LLM.Endpoint = "https://api.openai.com/v1/chat/completions"
	cfg.LLM.APIKey = os.Getenv("OPENAI_API_KEY")

	return cfg
}
