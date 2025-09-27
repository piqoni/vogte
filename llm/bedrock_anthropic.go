package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

type BedrockRequest struct {
	AnthropicVersion string    `json:"anthropic_version"`
	MaxTokens        int       `json:"max_tokens"`
	System           string    `json:"system,omitempty"`
	Messages         []Message `json:"messages"`
}

type BedrockResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	ID           string `json:"id"`
	Model        string `json:"model"`
	Role         string `json:"role"`
	StopReason   string `json:"stop_reason"`
	StopSequence string `json:"stop_sequence"`
	Type         string `json:"type"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func (c *Client) sendBedrockRequest(request ChatRequest) (string, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return "", fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := bedrockruntime.NewFromConfig(cfg)

	var systemPrompt string
	var messages []Message

	for _, msg := range request.Messages {
		if msg.Role == "system" {
			systemPrompt = msg.Content
		} else {
			messages = append(messages, msg)
		}
	}

	maxTokens := request.MaxCompletionTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	requestBody := BedrockRequest{
		AnthropicVersion: "bedrock-2023-05-31",
		MaxTokens:        maxTokens,
		System:           systemPrompt,
		Messages:         messages,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal bedrock request: %w", err)
	}

	input := &bedrockruntime.InvokeModelInput{
		Body:    body,
		ModelId: aws.String(request.Model),
	}

	response, err := client.InvokeModel(context.TODO(), input)
	if err != nil {
		return "", fmt.Errorf("failed to invoke bedrock model: %w", err)
	}

	var bedrockResponse BedrockResponse
	err = json.Unmarshal(response.Body, &bedrockResponse)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal bedrock response: %w", err)
	}

	if len(bedrockResponse.Content) > 0 && bedrockResponse.Content[0].Type == "text" {
		return bedrockResponse.Content[0].Text, nil
	}

	return "", fmt.Errorf("no text content received from bedrock")
}

func isBedrockModel(model string) bool {
	model = strings.TrimSpace(model)
	return strings.HasPrefix(model, "arn:aws:bedrock:")
}
