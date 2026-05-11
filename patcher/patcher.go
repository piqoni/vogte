package patcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type replaceOperation struct {
	old string
	new string
}

type Patcher struct {
	baseDir string
}

func New(baseDir string) *Patcher {
	return &Patcher{
		baseDir: baseDir,
	}
}

func (pc *Patcher) ParseAndApply(patchContent string) error {
	patches := pc.splitPatches(patchContent)

	for i, patch := range patches {
		if err := pc.parseSinglePatch(patch); err != nil {
			return fmt.Errorf("error applying patch %d: %w", i+1, err)
		}
	}

	return nil
}

func (pc *Patcher) splitPatches(content string) []string {
	var patches []string
	lines := strings.Split(content, "\n")

	var currentPatch strings.Builder
	inPatch := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "*** Begin Patch") {
			if inPatch && currentPatch.Len() > 0 {
				patches = append(patches, currentPatch.String())
				currentPatch.Reset()
			}
			inPatch = true
			currentPatch.WriteString(line + "\n")
			continue
		}

		if strings.HasPrefix(trimmed, "*** End Patch") {
			if inPatch {
				currentPatch.WriteString(line + "\n")
				patches = append(patches, currentPatch.String())
				currentPatch.Reset()
				inPatch = false
			}
			continue
		}

		if inPatch {
			currentPatch.WriteString(line + "\n")
		}
	}

	if inPatch && currentPatch.Len() > 0 {
		patches = append(patches, currentPatch.String())
	}

	if len(patches) == 0 && strings.TrimSpace(content) != "" {
		patches = append(patches, content)
	}

	return patches
}

func (pc *Patcher) parseSinglePatch(patchContent string) error {
	lines := strings.Split(strings.TrimSpace(patchContent), "\n")

	var filename string
	var addFileContent []string
	var replaceOps []replaceOperation
	var isAddFile bool

	for i := 0; i < len(lines); {
		line := strings.TrimSpace(lines[i])

		switch {
		case strings.HasPrefix(line, "*** Begin Patch"):
			i++
		case strings.HasPrefix(line, "*** Update File:") || strings.HasPrefix(line, "*** Add File:"):
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid file specification: %s", line)
			}

			filename = strings.TrimSpace(strings.TrimRight(parts[1], " *"))
			isAddFile = strings.HasPrefix(line, "*** Add File:")
			i++

			if isAddFile {
				for i < len(lines) {
					trimmed := strings.TrimSpace(lines[i])
					if strings.HasPrefix(trimmed, "*** End Patch") {
						break
					}

					contentLine := lines[i]
					if strings.HasPrefix(contentLine, "+") {
						addFileContent = append(addFileContent, contentLine[1:])
					} else {
						addFileContent = append(addFileContent, contentLine)
					}
					i++
				}
			}
		case strings.HasPrefix(line, "*** Replace ***"):
			op, next, err := pc.parseReplaceBlock(lines, i+1)
			if err != nil {
				return err
			}
			replaceOps = append(replaceOps, op)
			i = next
		case strings.HasPrefix(line, "*** End Patch"):
			i = len(lines)
		default:
			i++
		}
	}

	if filename == "" {
		return fmt.Errorf("no filename specified in patch")
	}

	if isAddFile {
		return pc.writeFile(filename, strings.Join(addFileContent, "\n"))
	}

	if len(replaceOps) == 0 {
		return fmt.Errorf("no replace operations specified for %s", filename)
	}

	return pc.applyReplacements(filename, replaceOps)
}

func (pc *Patcher) parseReplaceBlock(lines []string, start int) (replaceOperation, int, error) {
	if start >= len(lines) || !hasMarker(lines[start], "*** Old ***") {
		return replaceOperation{}, start, fmt.Errorf("replace block missing *** Old *** marker")
	}

	oldText, next, err := pc.collectBlock(lines, start+1, "*** New ***")
	if err != nil {
		return replaceOperation{}, start, err
	}

	newText, next, err := pc.collectBlock(lines, next+1, "*** Replace ***", "*** End Patch")
	if err != nil {
		return replaceOperation{}, start, err
	}

	return replaceOperation{old: oldText, new: newText}, next, nil
}

func (pc *Patcher) collectBlock(lines []string, start int, terminators ...string) (string, int, error) {
	var block []string

	for i := start; i < len(lines); i++ {
		for _, terminator := range terminators {
			if hasMarker(lines[i], terminator) {
				return strings.Join(block, "\n"), i, nil
			}
		}
		block = append(block, lines[i])
	}

	return "", start, fmt.Errorf("unterminated block, expected one of %s", strings.Join(terminators, ", "))
}

func (pc *Patcher) applyReplacements(filename string, replaceOps []replaceOperation) error {
	filePath := filepath.Join(pc.baseDir, filename)

	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	updated := string(content)
	for _, op := range replaceOps {
		if op.old == "" {
			return fmt.Errorf("replace operation for %s has empty old text", filename)
		}
		if !strings.Contains(updated, op.old) {
			return fmt.Errorf("old text not found in %s", filename)
		}
		updated = strings.Replace(updated, op.old, op.new, 1)
	}

	return pc.writeFile(filename, updated)
}

func (pc *Patcher) writeFile(filename, content string) error {
	filePath := filepath.Join(pc.baseDir, filename)

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", filePath, err)
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	return nil
}

func (pc *Patcher) CreateNewFile(filename, content string) error {
	return pc.writeFile(filename, content)
}

func hasMarker(line, marker string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), marker)
}
