package watcher

import (
	"os"
	"path/filepath"
	"testing"
)

type noopHandler struct{}

func (noopHandler) Handle(string) error { return nil }

func TestShouldProcess_NoTrigger(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.md")
	if err := os.WriteFile(path, []byte("plain text\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	w := &Watcher{cfg: Config{TriggerKeyword: "@vogte", EndMarker: "/v/"}}
	got, err := w.shouldProcess(path)
	if err != nil {
		t.Fatalf("shouldProcess: %v", err)
	}
	if got {
		t.Fatalf("expected false, got true")
	}
}

func TestShouldProcess_Answered(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.md")
	content := "Question @vogte\nAnswer\n/v/\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	w := &Watcher{cfg: Config{TriggerKeyword: "@vogte", EndMarker: "/v/"}}
	got, err := w.shouldProcess(path)
	if err != nil {
		t.Fatalf("shouldProcess: %v", err)
	}
	if got {
		t.Fatalf("expected false, got true")
	}
}

func TestShouldProcess_Unanswered(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.md")
	content := "Question @vogte\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	w := &Watcher{cfg: Config{TriggerKeyword: "@vogte", EndMarker: "/v/"}}
	got, err := w.shouldProcess(path)
	if err != nil {
		t.Fatalf("shouldProcess: %v", err)
	}
	if !got {
		t.Fatalf("expected true, got false")
	}
}

func TestFindUnansweredBlocks_Multiple(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	content := stringsJoin(
		"// answered question @vogte",
		"// answer",
		"// /v/",
		"",
		"// open question @vogte",
		"",
	)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	r := &Responder{cfg: Config{TriggerKeyword: "@vogte", EndMarker: "/v/"}}
	blocks, err := r.findUnansweredBlocks(path)
	if err != nil {
		t.Fatalf("findUnansweredBlocks: %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].TriggerLine != 4 {
		t.Fatalf("expected trigger line 4, got %d", blocks[0].TriggerLine)
	}
	if blocks[0].Prompt != "open question" {
		t.Fatalf("unexpected prompt: %q", blocks[0].Prompt)
	}
}

func TestFormatResponse_Go(t *testing.T) {
	r := &Responder{cfg: Config{EndMarker: "/v/"}}
	got := r.formatResponse("Use sort.Slice\nPlace it before the loop", ".go")
	want := stringsJoin(
		"// Use sort.Slice",
		"// Place it before the loop",
		"// /v/",
	)
	if got != want {
		t.Fatalf("unexpected format:\n%s", got)
	}
}

func TestFormatResponse_Markdown(t *testing.T) {
	r := &Responder{cfg: Config{EndMarker: "/v/"}}
	got := r.formatResponse("Short answer", ".md")
	want := stringsJoin(
		"Short answer",
		"/v/",
	)
	if got != want {
		t.Fatalf("unexpected format:\n%s", got)
	}
}

func TestFormatResponse_HTML(t *testing.T) {
	r := &Responder{cfg: Config{EndMarker: "/v/"}}
	got := r.formatResponse("Inline answer", ".html")
	want := stringsJoin(
		"<!--",
		"Inline answer",
		"/v/",
		"-->",
	)
	if got != want {
		t.Fatalf("unexpected format:\n%s", got)
	}
}

func TestInsertResponse_BottomToTop(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.md")
	content := stringsJoin(
		"first @vogte",
		"middle",
		"second @vogte",
		"",
	)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	r := &Responder{cfg: Config{EndMarker: "/v/"}}
	if err := r.insertResponse(path, Block{TriggerLine: 2}, stringsJoin("second answer", "/v/")); err != nil {
		t.Fatalf("insert bottom response: %v", err)
	}
	if err := r.insertResponse(path, Block{TriggerLine: 0}, stringsJoin("first answer", "/v/")); err != nil {
		t.Fatalf("insert top response: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	want := stringsJoin(
		"first @vogte",
		"first answer",
		"/v/",
		"middle",
		"second @vogte",
		"second answer",
		"/v/",
		"",
	)
	if string(got) != want {
		t.Fatalf("unexpected content:\n%s", string(got))
	}
}

func stringsJoin(lines ...string) string {
	if len(lines) == 0 {
		return ""
	}
	out := lines[0]
	for _, line := range lines[1:] {
		out += "\n" + line
	}
	return out
}
