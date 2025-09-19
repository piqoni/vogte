package patcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Patcher struct {
	baseDir string
}

func New(baseDir string) *Patcher {
	return &Patcher{
		baseDir: baseDir,
	}
}

func (pc *Patcher) ParseAndApply(patchContent string) error {

	// if err := os.WriteFile("debug.log", []byte(patchContent), 0644); err != nil {
	// fmt.Printf("Error writing to file: %v\n", err)
	// }
	// Handle multiple patches in a single response
	patches := pc.splitPatches(patchContent)

	for i, patch := range patches {
		if err := pc.parseSinglePatch(patch); err != nil {
			return fmt.Errorf("error applying patch %d: %w", i+1, err)
		}
	}

	return nil
}

// splitPatches splits content into individual patches
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
		} else if strings.HasPrefix(trimmed, "*** End Patch") {
			if inPatch {
				currentPatch.WriteString(line + "\n")
				patches = append(patches, currentPatch.String())
				currentPatch.Reset()
				inPatch = false
			}
		} else if inPatch {
			currentPatch.WriteString(line + "\n")
		}
	}

	// Handle case where patch doesn't have proper end marker
	if inPatch && currentPatch.Len() > 0 {
		patches = append(patches, currentPatch.String())
	}

	// If no proper patch format found, treat entire content as one patch
	if len(patches) == 0 && strings.TrimSpace(content) != "" {
		patches = append(patches, content)
	}

	return patches
}

func (pc *Patcher) parseSinglePatch(patchContent string) error {
	lines := strings.Split(strings.TrimSpace(patchContent), "\n")

	var filename string
	var context string
	var removals []string
	var additions []string
	var isAddFile bool

	i := 0
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		if strings.HasPrefix(line, "*** Begin Patch") {
			i++
			continue
		}

		if strings.HasPrefix(line, "*** Update File:") || strings.HasPrefix(line, "*** Add File:") {
			// Extract filename
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid file specification: %s", line)
			}
			filename = strings.TrimSpace(strings.TrimRight(parts[1], " ***"))
			// filename = strings.TrimSpace(parts[1])
			isAddFile = strings.HasPrefix(line, "*** Add File:")
			i++

			// If it's an Add File patch, collect content until End Patch
			if isAddFile {
				for i < len(lines) {
					changeLine := lines[i]
					trimmedChange := strings.TrimSpace(changeLine)

					if strings.HasPrefix(trimmedChange, "*** End Patch") {
						break
					}

					// Some models might include a stray @@ line; ignore it for add-file
					if strings.HasPrefix(trimmedChange, "@@") {
						i++
						continue
					}

					// Prefer + prefixed lines, but also accept raw lines
					if strings.HasPrefix(changeLine, "+") {
						additions = append(additions, changeLine[1:])
					} else if strings.HasPrefix(changeLine, "-") {
						// ignore removals for Add File blocks
					} else {
						// treat as literal content line
						additions = append(additions, changeLine)
					}
					i++
				}
				if filename == "" {
					return fmt.Errorf("no filename specified in patch")
				}

				return pc.applyPatch(filename, "", nil, additions)
			}
			continue
		}

		if strings.HasPrefix(line, "@@") {
			// Extract context (everything after @@)
			context = strings.TrimSpace(line[2:])
			i++

			// Parse the changes
			for i < len(lines) {
				changeLine := lines[i]
				trimmedChange := strings.TrimSpace(changeLine)

				if strings.HasPrefix(trimmedChange, "*** End Patch") {
					break
				}

				if strings.HasPrefix(changeLine, "-") {
					// Removal line - preserve original indentation
					removals = append(removals, changeLine[1:])
				} else if strings.HasPrefix(changeLine, "+") {
					// Addition line - preserve original indentation
					additions = append(additions, changeLine[1:])

				}
				i++
			}
			break
		}
		i++
	}

	if filename == "" {
		return fmt.Errorf("no filename specified in patch")
	}

	return pc.applyPatch(filename, context, removals, additions)
}

func (pc *Patcher) applyPatch(filename, context string, removals, additions []string) error {
	filePath := filepath.Join(pc.baseDir, filename)

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", filePath, err)
	}

	var content []byte
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		content = []byte{}
	} else {
		content, err = os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", filePath, err)
		}
	}

	lines := strings.Split(string(content), "\n")

	// Handle empty file case
	if len(lines) == 1 && lines[0] == "" {
		lines = []string{}
	}

	var newLines []string

	// If no context specified and file is empty, just add all additions
	if context == "" && len(lines) == 0 {
		newLines = additions
	} else if context == "" {
		// No context specified, apply changes to end of file
		newLines = make([]string, len(lines))
		copy(newLines, lines)

		if len(removals) > 0 {
			// Try to find and remove the lines from the end
			if len(newLines) >= len(removals) {
				match := true
				startIdx := len(newLines) - len(removals)
				for j, removal := range removals {
					if newLines[startIdx+j] != removal {
						match = false
						break
					}
				}
				if match {
					newLines = newLines[:startIdx]
				}
			}
		}

		newLines = append(newLines, additions...)
	} else {
		// Find the context line
		contextIndex := pc.findContextLine(lines, context)
		if contextIndex == -1 {
			return fmt.Errorf("context not found: %s", context)
		}

		// if err := os.WriteFile("debug2.log", []byte(strconv.Itoa(contextIndex)), 0644); err != nil {
		// fmt.Printf("Error writing to file: %v\n", err)
		// }

		newLines = pc.applyChangesAfterContext(lines, contextIndex, removals, additions)
	}

	// Write the modified content back to the file
	newContent := strings.Join(newLines, "\n")
	if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	return nil
}

func (pc *Patcher) findContextLine(lines []string, context string) int {
	for i, line := range lines {
		if strings.Contains(line, context) {
			return i
		}
	}
	return -1
}

// applyChangesAfterContext applies removals and additions after the context line
func (pc *Patcher) applyChangesAfterContext(lines []string, contextIndex int, removals, additions []string) []string {
	if len(removals) > 0 {
		// Find and replace the removal lines
		startIdx := pc.findRemovalStart(lines, contextIndex, removals)
		if startIdx == -1 {
			// If exact match not found, insert additions after context
			newLines := make([]string, 0, len(lines)+len(additions))
			newLines = append(newLines, lines[:contextIndex+1]...)
			newLines = append(newLines, additions...)
			newLines = append(newLines, lines[contextIndex+1:]...)
			return newLines
		}

		// Remove the old lines and insert new ones
		newLines := make([]string, 0, len(lines)-len(removals)+len(additions))
		newLines = append(newLines, lines[:startIdx]...)
		newLines = append(newLines, additions...)
		newLines = append(newLines, lines[startIdx+len(removals):]...)

		return newLines
	} else if len(additions) > 0 {
		// Only additions, insert after context
		newLines := make([]string, 0, len(lines)+len(additions))
		newLines = append(newLines, lines[:contextIndex+1]...)
		newLines = append(newLines, additions...)
		newLines = append(newLines, lines[contextIndex+1:]...)

		return newLines
	}

	return lines
}

// findRemovalStart finds where the removal pattern starts after the context
func (pc *Patcher) findRemovalStart(lines []string, contextIndex int, removals []string) int {
	for i := contextIndex + 1; i <= len(lines)-len(removals); i++ {
		match := true
		for j, removal := range removals {
			if i+j >= len(lines) || lines[i+j] != removal {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// CreateNewFile creates a new file with the given content
func (pc *Patcher) CreateNewFile(filename, content string) error {
	filePath := filepath.Join(pc.baseDir, filename)

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", filePath, err)
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to create file %s: %w", filePath, err)
	}

	return nil
}
