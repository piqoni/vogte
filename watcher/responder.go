package watcher

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/piqoni/vogte/llm"
	"github.com/piqoni/vogte/parser"
)

const inlineSystemPrompt = `You are an inline coding and writing assistant embedded in files.
The user has written a question or instruction ending with @vogte.
Answer it concisely and precisely. Do NOT include any preamble, meta-commentary, or markdown code fences - your response will be inserted directly into a source file.
If the trigger is in a code file, write your answer as if it were a developer comment.
If the trigger is in a markdown or text file, write plain prose.
End your response with exactly: /v/`

// Responder implements FileHandler. It reads context, calls the LLM, and
// patches the file with the formatted response.
type Responder struct {
	llmClient *llm.Client
	parser    *parser.Parser
	cfg       Config
}

// Block represents one unanswered @vogte prompt.
type Block struct {
	TriggerLine int
	Prompt      string
}

type preparedResponse struct {
	block     Block
	formatted string
}

func NewResponder(llmClient *llm.Client, parser *parser.Parser, cfg Config) *Responder {
	return &Responder{
		llmClient: llmClient,
		parser:    parser,
		cfg:       cfg,
	}
}

// Handle satisfies FileHandler. Orchestrates the full read→LLM→write cycle.
func (r *Responder) Handle(filePath string) error {
	original, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	blocks := findUnansweredBlocksInContent(string(original), r.cfg.TriggerKeyword, r.cfg.EndMarker)
	if len(blocks) == 0 {
		return nil
	}

	prepared := make([]preparedResponse, 0, len(blocks))
	ext := strings.ToLower(filepath.Ext(filePath))

	for i := len(blocks) - 1; i >= 0; i-- {
		block := blocks[i]
		context, err := r.buildContext(filePath, block)
		if err != nil {
			return err
		}

		rawResponse, err := r.llmClient.SendInlineMessage(inlineSystemPrompt, context)
		if err != nil {
			return err
		}

		prepared = append(prepared, preparedResponse{
			block:     block,
			formatted: r.formatResponse(rawResponse, ext),
		})
	}

	current, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, original) {
		return fmt.Errorf("file changed during processing: %s", filePath)
	}

	updated, err := r.applyPreparedResponses(string(original), prepared)
	if err != nil {
		return err
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, []byte(updated), info.Mode())
}

// findUnansweredBlocks returns all line ranges in the file that contain @vogte
// but no following /v/ end-marker within the same block.
func (r *Responder) findUnansweredBlocks(filePath string) ([]Block, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return findUnansweredBlocksInContent(string(content), r.cfg.TriggerKeyword, r.cfg.EndMarker), nil
}

// buildContext collects the file contents and relevant project structure to
// send to the LLM. Calls llmClient.askForRequiredFiles internally for cross-file context.
func (r *Responder) buildContext(filePath string, block Block) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(content), "\n")
	start := max(block.TriggerLine-20, 0)
	end := min(block.TriggerLine+20, len(lines)-1)
	window := strings.Join(lines[start:end+1], "\n")

	rootDir := r.contextRoot(filePath)
	displayPath := relativePath(rootDir, filePath)
	task := block.Prompt
	if strings.TrimSpace(task) == "" {
		task = strings.TrimSpace(lines[block.TriggerLine])
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Current file: %s\n", displayPath))
	builder.WriteString(fmt.Sprintf("Prompt: %s\n", task))
	builder.WriteString(fmt.Sprintf("Surrounding lines (%d-%d):\n%s\n", start+1, end+1, window))

	if r.parser == nil || r.llmClient == nil {
		return builder.String(), nil
	}

	if !hasProjectFiles(rootDir) {
		return builder.String(), nil
	}

	structure, err := r.parser.ParseProject(rootDir)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(structure) == "" {
		return builder.String(), nil
	}

	builder.WriteString("\nProject structure:\n")
	builder.WriteString(structure)

	requiredFiles, err := r.llmClient.AskForRequiredFiles(task+"\n\nCurrent file: "+displayPath+"\n"+window, structure)
	if err != nil {
		return "", err
	}

	if displayPath != filePath {
		requiredFiles = append(requiredFiles, displayPath)
	} else {
		requiredFiles = append(requiredFiles, filePath)
	}

	requiredFiles = uniqueStrings(requiredFiles)
	if len(requiredFiles) == 0 {
		return builder.String(), nil
	}

	fullFiles, err := r.llmClient.GetFileContents(rootDir, requiredFiles)
	if err != nil {
		return "", err
	}
	if len(fullFiles) == 0 {
		return builder.String(), nil
	}

	keys := make([]string, 0, len(fullFiles))
	for name := range fullFiles {
		keys = append(keys, name)
	}
	slices.Sort(keys)

	builder.WriteString("\nRelevant file contents:\n")
	for _, name := range keys {
		builder.WriteString(fmt.Sprintf("\n=== %s ===\n%s\n", name, fullFiles[name]))
	}

	return builder.String(), nil
}

// formatResponse wraps rawResponse in the appropriate comment style for ext
// and appends the end-marker.
func (r *Responder) formatResponse(rawResponse, ext string) string {
	clean := stripTrailingEndMarker(rawResponse, r.cfg.EndMarker)
	if clean == "" {
		clean = "No response provided."
	}

	if prefix, ok := lineCommentPrefix(ext); ok {
		lines := strings.Split(clean, "\n")
		formatted := make([]string, 0, len(lines)+1)
		for _, line := range lines {
			if line == "" {
				formatted = append(formatted, prefix)
				continue
			}
			formatted = append(formatted, prefix+" "+line)
		}
		formatted = append(formatted, prefix+" "+r.cfg.EndMarker)
		return strings.Join(formatted, "\n")
	}

	if open, close, ok := blockCommentStyle(ext); ok {
		var builder strings.Builder
		builder.WriteString(open)
		builder.WriteString("\n")
		builder.WriteString(clean)
		builder.WriteString("\n")
		builder.WriteString(r.cfg.EndMarker)
		builder.WriteString("\n")
		builder.WriteString(close)
		return builder.String()
	}

	return clean + "\n" + r.cfg.EndMarker
}

// insertResponse writes the formatted response into the file immediately after
// the trigger line of the given block, without disturbing any other content.
func (r *Responder) insertResponse(filePath string, block Block, formatted string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	updated, err := insertResponseIntoContent(string(content), block, formatted)
	if err != nil {
		return err
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, []byte(updated), info.Mode())
}

func (r *Responder) applyPreparedResponses(content string, responses []preparedResponse) (string, error) {
	updated := content
	var err error
	for _, response := range responses {
		updated, err = insertResponseIntoContent(updated, response.block, response.formatted)
		if err != nil {
			return "", err
		}
	}
	return updated, nil
}

func insertResponseIntoContent(content string, block Block, formatted string) (string, error) {
	lines := strings.Split(content, "\n")
	if block.TriggerLine < 0 || block.TriggerLine >= len(lines) {
		return "", fmt.Errorf("trigger line %d out of range", block.TriggerLine)
	}

	inserted := strings.Split(formatted, "\n")
	lines = append(lines[:block.TriggerLine+1], append(inserted, lines[block.TriggerLine+1:]...)...)
	return strings.Join(lines, "\n"), nil
}

func findUnansweredBlocksInContent(content, triggerKeyword, endMarker string) []Block {
	lines := strings.Split(content, "\n")
	triggerLines := make([]int, 0)

	for i, line := range lines {
		if strings.Contains(line, triggerKeyword) {
			triggerLines = append(triggerLines, i)
		}
	}

	blocks := make([]Block, 0, len(triggerLines))
	for i, triggerLine := range triggerLines {
		blockEnd := len(lines)
		if i+1 < len(triggerLines) {
			blockEnd = triggerLines[i+1]
		}

		answered := false
		for j := triggerLine; j < blockEnd; j++ {
			if strings.Contains(lines[j], endMarker) {
				answered = true
				break
			}
		}
		if answered {
			continue
		}

		blocks = append(blocks, Block{
			TriggerLine: triggerLine,
			Prompt:      extractPrompt(lines, triggerLine, triggerKeyword),
		})
	}

	return blocks
}

func extractPrompt(lines []string, triggerLine int, triggerKeyword string) string {
	current := strings.TrimSpace(stripCommentDecorators(lines[triggerLine]))
	current = strings.TrimSpace(strings.ReplaceAll(current, triggerKeyword, ""))
	if current != "" {
		return current
	}

	for i := triggerLine - 1; i >= 0; i-- {
		candidate := strings.TrimSpace(stripCommentDecorators(lines[i]))
		if candidate != "" {
			return candidate
		}
	}

	return ""
}

func stripCommentDecorators(line string) string {
	trimmed := strings.TrimSpace(line)
	for {
		switch {
		case strings.HasPrefix(trimmed, "//"):
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "//"))
		case strings.HasPrefix(trimmed, "#"):
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
		case strings.HasPrefix(trimmed, "/*"):
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "/*"))
		case strings.HasPrefix(trimmed, "<!--"):
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "<!--"))
		case strings.HasPrefix(trimmed, "*"):
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "*"))
		default:
			trimmed = strings.TrimSpace(strings.TrimSuffix(trimmed, "*/"))
			trimmed = strings.TrimSpace(strings.TrimSuffix(trimmed, "-->"))
			return trimmed
		}
	}
}

func stripTrailingEndMarker(rawResponse, endMarker string) string {
	clean := strings.TrimSpace(rawResponse)
	if clean == "" {
		return ""
	}

	lines := strings.Split(clean, "\n")
	for len(lines) > 0 {
		last := strings.TrimSpace(stripCommentDecorators(lines[len(lines)-1]))
		if last == "" {
			lines = lines[:len(lines)-1]
			continue
		}
		if last == endMarker {
			lines = lines[:len(lines)-1]
		}
		break
	}

	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func lineCommentPrefix(ext string) (string, bool) {
	switch ext {
	case ".go", ".js", ".ts", ".jsx", ".tsx", ".java", ".kt", ".swift", ".c", ".cpp", ".cs":
		return "//", true
	case ".py", ".sh", ".bash", ".zsh":
		return "#", true
	default:
		return "", false
	}
}

func blockCommentStyle(ext string) (string, string, bool) {
	switch ext {
	case ".html", ".xml":
		return "<!--", "-->", true
	case ".css", ".scss":
		return "/*", "*/", true
	default:
		return "", "", false
	}
}

func hasProjectFiles(root string) bool {
	found := false
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		switch filepath.Ext(path) {
		case ".go", ".proto":
			found = true
			return filepath.SkipAll
		}
		if filepath.Base(path) == "go.mod" {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func (r *Responder) contextRoot(filePath string) string {
	absFile, err := filepath.Abs(filePath)
	if err != nil {
		return filepath.Dir(filePath)
	}

	best := ""
	for _, dir := range r.cfg.Dirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(absDir, absFile)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		if len(absDir) > len(best) {
			best = absDir
		}
	}
	if best != "" {
		return best
	}
	return filepath.Dir(absFile)
}

func relativePath(rootDir, filePath string) string {
	rel, err := filepath.Rel(rootDir, filePath)
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filePath
	}
	return rel
}
