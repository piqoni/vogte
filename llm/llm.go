package llm

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/piqoni/vogte/config"
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
			Timeout: 180 * time.Second,
		},
		baseDir: ".",
	}
}

func (c *Client) Config() *config.Config {
	return c.config
}

func (c *Client) SetBaseDir(baseDir string) {
	c.baseDir = baseDir
}

func (c *Client) BaseDir() string {
	return c.baseDir
}

// SendMessage sends a message to the LLM using a two-step approach:
// 1. First asks which files are needed
// 2. Then sends full file contents for patching
func (c *Client) SendMessage(userMessage, projectStructure, mode string) (string, error) {
	// if mode == "ASK" {
	// 	log.Print("SIMPLE ASK PATH")
	// 	log.Printf("userMessage: %s, projectStructure: %s", userMessage, projectStructure)
	// 	return c.sendSimpleMessage(userMessage, projectStructure)
	// }

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
	fullFiles, err := c.getFileContentsFromBase(c.baseDir, fileList)
	if err != nil {
		return "", fmt.Errorf("error reading file contents: %w", err)
	}

	// Step 3: Request patch with full file contents
	return c.requestPatch(userMessage, fullFiles)
}

func (c *Client) SendInlineMessage(systemPrompt, userMessage string) (string, error) {
	messages := []Message{}
	if strings.TrimSpace(systemPrompt) != "" {
		messages = append(messages, Message{
			Role:    "system",
			Content: systemPrompt,
		})
	}
	messages = append(messages, Message{
		Role:    "user",
		Content: userMessage,
	})

	request := ChatRequest{
		Model:               c.config.LLM.Model,
		Messages:            messages,
		MaxCompletionTokens: 1200,
	}

	return c.sendChatRequest(request)
}

func (c *Client) AskForRequiredFiles(task, blueprint string) ([]string, error) {
	return c.askForRequiredFiles(task, blueprint)
}

func (c *Client) GetFileContents(baseDir string, fileList []string) (map[string]string, error) {
	return c.getFileContentsFromBase(baseDir, fileList)
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

	if !isAnthropicModel(c.config.LLM.Model) {
		messages = append(messages, Message{
			Role:    "system",
			Content: "You are a precise coding assistant. Always follow instructions exactly.",
		})
	}

	request := ChatRequest{
		Model:    c.config.LLM.Model,
		Messages: messages,
		// Temperature:         0.1,
		// MaxCompletionTokens: 500, // Shorter response expected
	}

	response, err := c.sendChatRequest(request)
	// if err := os.WriteFile("llm.log", []byte(prompt+"\n\n"+response), 0644); err != nil { // TODO debug log
	// fmt.Printf("Error writing to file: %v\n", err)
	// }
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
func (c *Client) getFileContentsFromBase(baseDir string, fileList []string) (map[string]string, error) {
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
			fullPath = fmt.Sprintf("%s/%s", baseDir, file) // Relative to baseDir
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
Please generate a patch to complete the requested task. Use this EXACT format.

For existing files, use string replacement blocks with exact old/new text:

*** Begin Patch ***
*** Update File: filename.go ***
*** Replace ***
*** Old ***
fmt.Println("Hello")
*** New ***
fmt.Println("Hello, World!")
*** End Patch ***

You can include multiple replacement blocks in the same file patch:

*** Begin Patch ***
*** Update File: filename.go ***
*** Replace ***
*** Old ***
old text
*** New ***
new text
*** Replace ***
*** Old ***
another old text
*** New ***
another new text
*** End Patch ***

To create a new file, use this format:

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
2. In "*** Old ***", include the exact existing text to replace, matching whitespace exactly
3. In "*** New ***", include the exact replacement text
4. To remove text, keep "*** New ***" empty
5. To insert text, replace a nearby anchor with anchor-plus-new-content
6. For new files, use "*** Add File: <path> ***"
7. Preserve indentation and formatting
8. Only modify what's necessary to fulfill the request
9. If creating new files, start with appropriate package declaration
10. Ensure all imports are properly handled
11. Consider Go best practices and idiomatic code

Working example:
*** Begin Patch ***
*** Update File: main.go ***
*** Replace ***
*** Old ***
fmt.Println("Hello")
*** New ***
fmt.Println("Hello, World!")
*** End Patch ***`)

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
		// MaxCompletionTokens: 4000,
	}

	return c.sendChatRequest(request)
}

// func (c *Client) sendSimpleMessage(userMessage, projectStructure string) (string, error) {
// 	systemPrompt := c.buildSystemPrompt(projectStructure)

// 	messages := []Message{
// 		{
// 			Role:    "system",
// 			Content: systemPrompt,
// 		},
// 		{
// 			Role:    "user",
// 			Content: userMessage,
// 		},
// 	}

// 	request := ChatRequest{
// 		Model:    c.config.LLM.Model,
// 		Messages: messages,
// 		// Temperature:         0.1,
// 		MaxCompletionTokens: 4000,
// 	}

// 	return c.sendChatRequest(request)
// }

// func (c *Client) buildSystemPrompt(projectStructure string) string {
// 	return fmt.Sprintf(`You are an expert Go developer helping to modify a Go project. You have been provided with the current project structure below.

// Current project structure:

// %s

// Please analyze the user's request and provide the necessary help to fulfill their request.`, projectStructure)
// }

func (c *Client) sendChatRequest(request ChatRequest) (string, error) {
	if err := c.ValidateConfig(); err != nil {
		return "", err
	}
	if isBedrockModel(request.Model) {
		return c.sendBedrockRequest(request)
	}
	if isAnthropicModel(request.Model) || strings.Contains(strings.ToLower(c.config.LLM.Endpoint), "anthropic.com") {
		return c.sendAnthropicRequest(request)
	}
	return c.sendOpenAIRequest(request)
}

func (c *Client) ValidateConfig() error {
	if c.config.LLM.Model == "" {
		return fmt.Errorf("model is required")
	}

	// Bedrock models don't need API keys or endpoints
	if isBedrockModel(c.config.LLM.Model) {
		return nil
	}

	if c.config.LLM.APIKey == "" {
		return fmt.Errorf("API key is required")
	}

	if c.config.LLM.Endpoint == "" {
		return fmt.Errorf("API endpoint is required")
	}

	return nil
}

// ReviewDiff asks the LLM to review a diff and point out potential issues.
func (c *Client) ReviewDiff(diff, description string) (string, error) {
	if c.config.LLM.APIKey == "" {
		return "", fmt.Errorf("LLM API key not configured")
	}

	desc := strings.TrimSpace(description)
	if desc == "" {
		desc = "(no additional description provided)"
	}

	systemMsg := "You are a senior code reviewer. Be concise, specific, and pragmatic. Focus on correctness, safety, backwards compatibility, tests, performance, security, and idiomatic approaches. When you suggest a change, explain why."

	userPrompt := fmt.Sprintf(`Please review the following uncommitted changes (Git diff) against the base branch. Review only what's being changed.

Change description:
%s

Diff:
%s

What to do:
- Identify potential issues
- Reference the file and approximate line based on the diff where possible.
- Provide concrete, actionable suggestions or quick patches when simple.
- Call out anything that requires additional context or tests.

Format your response in markdown, with code examples where relevant using appropriate syntax highlighting.

It should have these sections:
Summary:
- One or two sentences summarizing the change and risk profile.

Findings:
- [Severity: High|Medium|Low] file.go:~line — Short title
  Explanation: ...
  Suggestion: ...

Verdict: Ready / Needs attention`, desc, diff)

	messages := []Message{
		{
			Role:    "user",
			Content: userPrompt,
		},
	}

	if !isAnthropicModel(c.config.LLM.Model) {
		messages = append([]Message{{Role: "system", Content: systemMsg}}, messages...)
	}

	request := ChatRequest{
		Model:    c.config.LLM.Model,
		Messages: messages,
	}

	return c.sendChatRequest(request)
}
