package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/piqoni/vogte/internal/config"
)

type Client struct {
	config     *config.Config
	httpClient *http.Client
	baseDir    string // baseDir for file operations
}

func New(cfg *config.Config) *Client {
	return &Client{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		baseDir: ".",
	}
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest represents the request payload for chat completion
type ChatRequest struct {
	Model               string    `json:"model"`
	Messages            []Message `json:"messages"`
	Temperature         float64   `json:"temperature,omitempty"`
	MaxCompletionTokens int       `json:"max_completion_tokens,omitempty"`
}

// ChatResponse represents the response from chat completion
type ChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// SendMessage sends a message to the LLM using a two-step approach:
// 1. First asks which files are needed
// 2. Then sends full file contents for patching
func (c *Client) SendMessage(userMessage, projectStructure, mode string) (string, error) {
	if c.config.LLM.APIKey == "" {
		return "", fmt.Errorf("LLM API key not configured")
	}

	if mode == "ASK" {
		log.Print("SIMPLE ASK PATH")
		log.Printf("userMessage: %s, projectStructure: %s", userMessage, projectStructure)
		return c.sendSimpleMessage(userMessage, projectStructure)
	}

	// Step 1: Ask LLM which files it needs
	fileList, err := c.askForRequiredFiles(userMessage, projectStructure)
	if err != nil {
		return "", fmt.Errorf("error getting required files: %w", err)
	}

	// TODO: decide what to do when no list of files is returned
	// if len(fileList) == 0 {
	// return "", fmt.Errorf("first step didnt return any file from the llm, inspect your user prompt")
	// }
	// Step 2: Get full content of required files
	fullFiles, err := c.getFileContents(fileList)
	if err != nil {
		return "", fmt.Errorf("error reading file contents: %w", err)
	}

	// Step 3: Request patch with full file contents
	return c.requestPatch(userMessage, fullFiles)
}

// askForRequiredFiles asks the LLM which files it needs to see in full
func (c *Client) askForRequiredFiles(task, blueprint string) ([]string, error) {
	prompt := fmt.Sprintf(`Given this coding task: "%s"

And this project structure showing all structs, interfaces, and function signatures:
%s

Please respond with ONLY a comma-separated list of the specific files you need to see in full to complete this task. Do not include any explanations, just the file paths.

Example response: main.go,utils/helper.go,models/user.go`, task, blueprint)

	messages := []Message{
		{
			Role:    "user",
			Content: prompt,
		},
	}

	request := ChatRequest{
		Model:    c.config.LLM.Model,
		Messages: messages,
		// Temperature:         0.1,
		MaxCompletionTokens: 500, // Shorter response expected
	}

	response, err := c.sendChatRequest(request)
	if err := os.WriteFile("llm.log", []byte(response), 0644); err != nil {
		fmt.Printf("Error writing to file: %v\n", err)
	}
	if err != nil {
		return nil, err
	}

	// Parse the response to extract file list
	files := strings.Split(strings.TrimSpace(response), ",")
	var cleanFiles []string
	for _, file := range files {
		file = strings.TrimSpace(file)
		if file != "" && !strings.Contains(file, "No files") && !strings.Contains(file, "None") {
			cleanFiles = append(cleanFiles, file)
		}
	}

	return cleanFiles, nil
}

// getFileContents reads the full content of specified files
func (c *Client) getFileContents(fileList []string) (map[string]string, error) {
	contents := make(map[string]string)

	for _, file := range fileList {
		if file == "" {
			continue
		}

		// Use baseDir for constructing file paths
		var fullPath string
		if strings.HasPrefix(file, "/") {
			fullPath = file // Absolute path
		} else {
			fullPath = fmt.Sprintf("%s/%s", c.baseDir, file) // Relative to baseDir
		}

		content, err := os.ReadFile(fullPath)
		if err != nil {
			fmt.Printf("Warning: Could not read file %s: %v\n", file, err)
			continue
		}
		contents[file] = string(content)
	}

	return contents, nil
}

// requestPatch asks the LLM to generate a patch for the task with full file contents
func (c *Client) requestPatch(task string, fileContents map[string]string) (string, error) {
	// Build the prompt with file contents
	var promptBuilder strings.Builder
	promptBuilder.WriteString(fmt.Sprintf(`Task: %s

Full File Contents:
`, task))

	for filename, content := range fileContents {
		promptBuilder.WriteString(fmt.Sprintf("\n=== %s ===\n%s\n", filename, content))
	}

	promptBuilder.WriteString(`
Please generate a patch to complete the requested task. Use this EXACT format:

*** Begin Patch ***
*** Update File: filename.go ***
@@ context line that helps locate where changes should be made @@
-old line to remove
-another old line to remove
+new line to add
+another new line to add
*** End Patch ***

To create a new file, use this format (no @@ line):

*** Begin Patch ***
*** Add File: path/to/newfile.go ***
+package mypkg
+
+import "fmt"
+
+func Hello() {
+    fmt.Println("hello")
+}
*** End Patch ***

CRITICAL REQUIREMENTS:
1. Always provide the exact file path relative to the project root
2. Use context lines to help locate where changes should be made
3. For new files, use "*** Add File: <path> ***" and omit the @@ context line and removal lines
4. Preserve indentation and formatting
5. Only modify what's necessary to fulfill the request
6. If creating new files, start with appropriate package declaration
7. Ensure all imports are properly handled
8. Consider Go best practices and idiomatic code
9. Match EXACT indentation and whitespace from the original file
10. The @@ line should be simple: either just "func functionName() {" or a simple context
11. Do NOT use complex function signatures like "func (receiver *Type) Name() error {"
12. If it's a method, just use the method name: "func MethodName() {"

Working example:
*** Begin Patch ***
*** Update File: main.go ***
@@ func main() {
- fmt.Println("Hello")
+ fmt.Println("Hello, World!")
*** End Patch ***

If the function signature is complex, try using a simpler context or just the line content itself.`)

	messages := []Message{
		{
			Role:    "user",
			Content: promptBuilder.String(),
		},
	}

	request := ChatRequest{
		Model:    c.config.LLM.Model,
		Messages: messages,
		// Temperature:         0.1,
		MaxCompletionTokens: 4000,
	}

	return c.sendChatRequest(request)
}

func (c *Client) sendSimpleMessage(userMessage, projectStructure string) (string, error) {
	systemPrompt := c.buildSystemPrompt(projectStructure)

	messages := []Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: userMessage,
		},
	}

	request := ChatRequest{
		Model:    c.config.LLM.Model,
		Messages: messages,
		// Temperature:         0.1,
		MaxCompletionTokens: 4000,
	}

	return c.sendChatRequest(request)
}

func (c *Client) buildSystemPrompt(projectStructure string) string {
	return fmt.Sprintf(`You are an expert Go developer helping to modify a Go project. You have been provided with the current project structure below.

Current project structure:

%s

Please analyze the user's request and provide the necessary help to fulfill their request.`, projectStructure)
}

func (c *Client) sendChatRequest(request ChatRequest) (string, error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.config.LLM.Endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.LLM.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response ChatResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if response.Error != nil {
		return "", fmt.Errorf("API error: %s", response.Error.Message)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no response choices received")
	}

	return response.Choices[0].Message.Content, nil
}

func (c *Client) ValidateConfig() error {
	if c.config.LLM.APIKey == "" {
		return fmt.Errorf("API key is required")
	}

	if c.config.LLM.Endpoint == "" {
		return fmt.Errorf("API endpoint is required")
	}

	if c.config.LLM.Model == "" {
		return fmt.Errorf("model is required")
	}

	return nil
}
