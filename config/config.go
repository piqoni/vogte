package config

import (
	"encoding/json"
	"os"
	"strings"
)

type Config struct {
	LLM struct {
		APIKey   string `json:"api_key"`
		Model    string `json:"model"`
		Endpoint string `json:"endpoint"`
	} `json:"llm"`
}

func (cfg *Config) SetModel(model string) {
	cfg.LLM.Model = model
	cfg.ApplyProviderByModel()
}

// If model starts with "claude-", it will use Anthropic endpoint and ANTHROPIC_API_KEY
// Otherwise, it will use OpenAI endpoint and OPENAI_API_KEY (if present).
func (cfg *Config) ApplyProviderByModel() {
	if strings.HasPrefix(strings.ToLower(cfg.LLM.Model), "claude-") {
		cfg.LLM.Endpoint = "https://api.anthropic.com/v1/messages"
		if k := os.Getenv("ANTHROPIC_API_KEY"); k != "" {
			cfg.LLM.APIKey = k
		}
	} else {
		cfg.LLM.Endpoint = "https://api.openai.com/v1/chat/completions"
		if k := os.Getenv("OPENAI_API_KEY"); k != "" {
			cfg.LLM.APIKey = k
		}
	}
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

	cfg.LLM.Model = os.Getenv("VOGTE_LLM_MODEL")
	if cfg.LLM.Model == "" {
		cfg.LLM.Model = "gpt-5"
	}
	cfg.ApplyProviderByModel()
	return cfg
}
