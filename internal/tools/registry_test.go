package tools_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CaowardlyLion/OpenHome/internal/tools"
	"github.com/CaowardlyLion/OpenHome/internal/tools/definitions"
)

func registry(t *testing.T) (*tools.Registry, string) {
	t.Helper()
	root := t.TempDir()
	result, err := tools.NewRegistry(root, definitions.All())
	if err != nil {
		t.Fatal(err)
	}
	return result, root
}

func TestRegistryWritesReadsAndAcceptsAlias(t *testing.T) {
	registry, root := registry(t)
	if _, err := registry.Execute("writeFile", `{"path":"plans/week.md","content":"# Week"}`, []string{"writeFile"}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, "plans/week.md"))
	if err != nil || string(content) != "# Week" {
		t.Fatalf("write result = %q, %v", content, err)
	}
	result, err := registry.Execute("readFile", `{"file_path":"plans/week.md"}`, []string{"readFile"})
	if err != nil || result.Result != "# Week" {
		t.Fatalf("read result = %#v, %v", result.Result, err)
	}
}

func TestRegistryRejectsUnsafePathsAndTools(t *testing.T) {
	registry, _ := registry(t)
	cases := []struct {
		tool, args string
		allowed    []string
		want       string
	}{
		{"writeFile", `{"path":"../escape.md","content":"bad"}`, []string{"writeFile"}, "path escapes workspace"},
		{"readFile", `{"path":"/etc/passwd"}`, []string{"readFile"}, "absolute paths"},
		{"readFile", `{"path":"x"}`, []string{"writeFile"}, "not allowed"},
	}
	for _, test := range cases {
		_, err := registry.Execute(test.tool, test.args, test.allowed)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("%s error = %v, want %q", test.tool, err, test.want)
		}
	}
}

func TestRegistryRejectsSymlinkEscapes(t *testing.T) {
	registry, root := registry(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Execute("writeFile", `{"path":"linked/escape.md","content":"bad"}`, []string{"writeFile"}); err == nil || !strings.Contains(err.Error(), "resolved path escapes workspace") {
		t.Fatalf("parent symlink error = %v", err)
	}
	target := filepath.Join(outside, "secret.md")
	if err := os.WriteFile(target, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "output.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Execute("writeFile", `{"path":"output.md","content":"bad"}`, []string{"writeFile"}); err == nil || !strings.Contains(err.Error(), "writing through symlinks") {
		t.Fatalf("file symlink error = %v", err)
	}
}
