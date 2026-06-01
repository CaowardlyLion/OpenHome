package definitions

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CaowardlyLion/OpenHome/internal/permissions"
	"github.com/CaowardlyLion/OpenHome/internal/tools"
)

func advancedRegistry(t *testing.T, serverURL string) (*tools.Registry, string) {
	t.Helper()
	root := t.TempDir()
	manager, err := permissions.New(t.TempDir(), root, permissions.Allow, nil)
	if err != nil {
		t.Fatal(err)
	}
	definitions := []tools.Definition{ReadExternalFile(), RunCommand(), FetchURL(), DownloadFile(), WebSearch(DuckDuckGoHTML{Endpoint: serverURL})}
	registry, err := tools.NewRegistry(root, manager, definitions)
	if err != nil {
		t.Fatal(err)
	}
	return registry, root
}

func TestReadExternalFileAndRunCommand(t *testing.T) {
	registry, _ := advancedRegistry(t, "")
	file := filepath.Join(t.TempDir(), "note.txt")
	os.WriteFile(file, []byte("hello"), 0o644)
	result, err := registry.Execute(context.Background(), "readExternalFile", `{"path":"`+file+`","reason":"inspect"}`)
	if err != nil || !strings.Contains(result.Result.(map[string]any)["content"].(string), "hello") {
		t.Fatalf("result = %#v, %v", result, err)
	}
	result, err = registry.Execute(context.Background(), "runCommand", `{"command":"pwd","reason":"inspect cwd"}`)
	if err != nil || !strings.Contains(result.Result.(map[string]any)["stdout"].(string), "/") {
		t.Fatalf("result = %#v, %v", result, err)
	}
}

func TestFetchDownloadAndSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/html/" {
			response.Header().Set("Content-Type", "text/html")
			response.Write([]byte(`<a class="result__a" href="https://example.com">Example</a><a class="result__snippet">Useful result</a>`))
			return
		}
		response.Header().Set("Content-Type", "text/plain")
		response.Write([]byte("download body"))
	}))
	defer server.Close()
	registry, root := advancedRegistry(t, server.URL+"/html/")
	fetch, err := registry.Execute(context.Background(), "fetchURL", `{"url":"`+server.URL+`/page","reason":"inspect"}`)
	if err != nil || !strings.Contains(fetch.Result.(map[string]any)["body"].(string), "download body") {
		t.Fatalf("fetch = %#v, %v", fetch, err)
	}
	download, err := registry.Execute(context.Background(), "downloadFile", `{"url":"`+server.URL+`/file","path":"downloads/file.txt","reason":"save"}`)
	if err != nil {
		t.Fatal(err)
	}
	if content, err := os.ReadFile(filepath.Join(root, download.Result.(map[string]any)["path"].(string))); err != nil || string(content) != "download body" {
		t.Fatalf("content = %q, %v", content, err)
	}
	search, err := registry.Execute(context.Background(), "webSearch", `{"query":"test","reason":"research"}`)
	if err != nil || len(search.Result.([]map[string]string)) != 1 {
		t.Fatalf("search = %#v, %v", search, err)
	}
}

func TestAdvancedToolReturnsStructuredDenial(t *testing.T) {
	root := t.TempDir()
	manager, err := permissions.New(t.TempDir(), root, permissions.Default, nil)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := tools.NewRegistry(root, manager, []tools.Definition{FetchURL()})
	if err != nil {
		t.Fatal(err)
	}
	result, err := registry.Execute(context.Background(), "fetchURL", `{"url":"https://example.com","reason":"research"}`)
	if err != nil || result.Result.(map[string]any)["denied"] != true {
		t.Fatalf("result = %#v, %v", result, err)
	}
}

func TestFetchRedirectDenialIsStructured(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/redirect" {
			http.Redirect(response, request, "/final", http.StatusFound)
			return
		}
		response.Write([]byte("final"))
	}))
	defer server.Close()
	calls := 0
	manager, err := permissions.New(t.TempDir(), t.TempDir(), permissions.Default, func(context.Context, permissions.Request) permissions.Decision {
		calls++
		if calls == 1 {
			return permissions.AllowOnce
		}
		return permissions.Deny
	})
	if err != nil {
		t.Fatal(err)
	}
	registry, err := tools.NewRegistry(t.TempDir(), manager, []tools.Definition{FetchURL()})
	if err != nil {
		t.Fatal(err)
	}
	result, err := registry.Execute(context.Background(), "fetchURL", `{"url":"`+server.URL+`/redirect","reason":"inspect"}`)
	if err != nil || result.Result.(map[string]any)["denied"] != true {
		t.Fatalf("result = %#v, %v", result, err)
	}
}
