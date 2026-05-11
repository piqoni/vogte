package patcher

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseAndApplyReplaceText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	original := "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	patch := `*** Begin Patch ***
*** Update File: main.go ***
*** Replace ***
*** Old ***
println("hello")
*** New ***
println("hello world")
*** End Patch ***`

	if err := New(dir).ParseAndApply(patch); err != nil {
		t.Fatalf("apply patch: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read updated: %v", err)
	}

	want := "package main\n\nfunc main() {\n\tprintln(\"hello world\")\n}\n"
	if string(got) != want {
		t.Fatalf("unexpected content:\n%s", string(got))
	}
}

func TestParseAndApplyRemoveText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	original := "package main\n\nfunc main() {\n\tprintln(\"hello\")\n\tprintln(\"goodbye\")\n}\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	patch := `*** Begin Patch ***
*** Update File: main.go ***
*** Replace ***
*** Old ***
	println("goodbye")
*** New ***
*** End Patch ***`

	if err := New(dir).ParseAndApply(patch); err != nil {
		t.Fatalf("apply patch: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read updated: %v", err)
	}

	want := "package main\n\nfunc main() {\n\tprintln(\"hello\")\n\n}\n"
	if string(got) != want {
		t.Fatalf("unexpected content:\n%s", string(got))
	}
}

func TestParseAndApplyInsertTextViaAnchorReplacement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	original := "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	patch := `*** Begin Patch ***
*** Update File: main.go ***
*** Replace ***
*** Old ***
func main() {
	println("hello")
}
*** New ***
func main() {
	println("hello")
	println("goodbye")
}
*** End Patch ***`

	if err := New(dir).ParseAndApply(patch); err != nil {
		t.Fatalf("apply patch: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read updated: %v", err)
	}

	want := "package main\n\nfunc main() {\n\tprintln(\"hello\")\n\tprintln(\"goodbye\")\n}\n"
	if string(got) != want {
		t.Fatalf("unexpected content:\n%s", string(got))
	}
}

func TestParseAndApplyAddFile(t *testing.T) {
	dir := t.TempDir()
	patch := `*** Begin Patch ***
*** Add File: internal/hello/hello.go ***
+package hello
+
+func Message() string {
+	return "hello"
+}
*** End Patch ***`

	if err := New(dir).ParseAndApply(patch); err != nil {
		t.Fatalf("apply patch: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "internal/hello/hello.go"))
	if err != nil {
		t.Fatalf("read new file: %v", err)
	}

	want := "package hello\n\nfunc Message() string {\n\treturn \"hello\"\n}"
	if string(got) != want {
		t.Fatalf("unexpected content:\n%s", string(got))
	}
}
