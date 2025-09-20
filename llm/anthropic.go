package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// Anthropic Messages API structures
type anthropicChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature,omitempty"`
}

type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicChatResponse struct {
	ID      string                  `json:"id"`
	Type    string                  `json:"type"`
	Role    string                  `json:"role"`
	Model   string                  `json:"model"`
	Content []anthropicContentBlock `json:"content"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *Client) sendAnthropicRequest(request ChatRequest) (string, error) {
	// Ensure endpoint and API key appropriate for Anthropic
	endpoint := c.config.LLM.Endpoint
	if endpoint == "" || strings.Contains(strings.ToLower(endpoint), "openai.com") {
		endpoint = "https://api.anthropic.com/v1/messages"
	}

	// Prefer ANTHROPIC_API_KEY if present
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		apiKey = c.config.LLM.APIKey
	}
	if apiKey == "" {
		return "", fmt.Errorf("LLM API key not configured (expect ANTHROPIC_API_KEY for Claude models)")
	}

	maxTokens := request.MaxCompletionTokens
	if maxTokens == 0 {
		maxTokens = 1024
	}
	anthReq := anthropicChatRequest{
		Model:       request.Model,
		Messages:    request.Messages,
		MaxTokens:   maxTokens,
		Temperature: request.Temperature,
	}

	jsonData, err := json.Marshal(anthReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal anthropic request: %w", err)
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create anthropic request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send anthropic request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read anthropic response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Anthropic API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var aResp anthropicChatResponse
	if err := json.Unmarshal(body, &aResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal anthropic response: %w", err)
	}
	if aResp.Error != nil {
		return "", fmt.Errorf("Anthropic API error: %s", aResp.Error.Message)
	}
	if len(aResp.Content) == 0 {
		return "", fmt.Errorf("no content returned from Anthropic")
	}
	return aResp.Content[0].Text, nil
}

func isAnthropicModel(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(m, "claude-")
}
