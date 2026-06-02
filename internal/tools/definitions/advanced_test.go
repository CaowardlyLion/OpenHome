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

type fakeBrowserRunner struct {
	calls int
	args  map[string]any
}

func (runner *fakeBrowserRunner) Run(_ context.Context, args map[string]any) (map[string]any, error) {
	runner.calls++
	runner.args = args
	return map[string]any{"title": "Example", "text": "Compact page text."}, nil
}

func advancedRegistry(t *testing.T, serverURL string) (*tools.Registry, string) {
	t.Helper()
	root := t.TempDir()
	manager, err := permissions.New(t.TempDir(), root, permissions.Allow, nil)
	if err != nil {
		t.Fatal(err)
	}
	definitions := []tools.Definition{ReadExternalFile(), RunCommand(), FetchURL(), DownloadFile(), WebSearch(BingRSS{Endpoint: serverURL})}
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
			response.Header().Set("Content-Type", "application/rss+xml")
			response.Write([]byte(`<rss><channel><item><title>Example</title><link>https://example.com</link><description>Useful result</description></item></channel></rss>`))
			return
		}
		response.Header().Set("Content-Type", "text/plain")
		response.Write([]byte("download body"))
	}))
	defer server.Close()
	registry, root := advancedRegistry(t, server.URL+"/html/")
	fetch, err := registry.Execute(context.Background(), "fetchURL", `{"url":"`+server.URL+`/page","reason":"inspect"}`)
	if err != nil || !strings.Contains(fetch.Result.(map[string]any)["text"].(string), "download body") {
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

func TestCompactResponseTextRemovesHTMLNoiseAndCapsContext(t *testing.T) {
	text, truncated := compactResponseText([]byte(`<html><style>hide</style><script>ignore()</script><h1>Hello &amp; welcome</h1><p>Useful text.</p></html>`), "text/html")
	if truncated || strings.Contains(text, "ignore") || text != "Hello & welcome\n\nUseful text." {
		t.Fatalf("text = %q, truncated = %v", text, truncated)
	}
	text, truncated = compactResponseText([]byte(strings.Repeat("x", contextTextLimit+1)), "text/plain")
	if !truncated || len(text) != contextTextLimit {
		t.Fatalf("text length = %d, truncated = %v", len(text), truncated)
	}
}

func TestBrowserNavigationFallbackReasonForBlockedFetch(t *testing.T) {
	for _, test := range []struct {
		status int
		text   string
	}{
		{status: http.StatusUnauthorized},
		{status: http.StatusForbidden},
		{status: http.StatusTooManyRequests},
		{status: http.StatusOK, text: "Please verify you are human"},
		{status: http.StatusOK, text: "Unfortunately, bots use this site too."},
	} {
		if reason := browserNavigationFallbackReason(test.status, test.text); reason == "" {
			t.Fatalf("missing fallback for status = %d, text = %q", test.status, test.text)
		}
	}
	if reason := browserNavigationFallbackReason(http.StatusNotFound, "missing"); reason != "" {
		t.Fatalf("unexpected fallback = %q", reason)
	}
}

func TestFetchedResponseResultRecommendsBrowserNavigation(t *testing.T) {
	request, err := http.NewRequest(http.MethodGet, "https://example.com/private", nil)
	if err != nil {
		t.Fatal(err)
	}
	result := fetchedResponseResult(&http.Response{
		StatusCode: http.StatusForbidden,
		Header:     http.Header{"Content-Type": []string{"text/html"}},
		Request:    request,
	}, []byte(`<html><body>Access denied</body></html>`), false)
	if result["blocked"] != true || result["recommendedSkill"] != "browser-navigation" ||
		!strings.Contains(result["recommendedAction"].(string), "request_skill_reselection") {
		t.Fatalf("result = %#v", result)
	}
}

func TestFetchedResponseResultIncludesBoundedResolvedLinks(t *testing.T) {
	request, err := http.NewRequest(http.MethodGet, "https://example.com/news/", nil)
	if err != nil {
		t.Fatal(err)
	}
	result := fetchedResponseResult(&http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/html"}},
		Request:    request,
	}, []byte(`<a href="/article?id=1#details"><strong>Useful</strong> headline</a><a href="mailto:editor@example.com">Email</a><a href="/article?id=1">Duplicate</a>`), false)
	links := result["links"].([]map[string]string)
	if len(links) != 1 || links[0]["text"] != "Useful headline" || links[0]["url"] != "https://example.com/article?id=1" {
		t.Fatalf("links = %#v", links)
	}
}

func TestDuckDuckGoChallengeReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.WriteHeader(http.StatusAccepted)
		response.Write([]byte(`<div class="anomaly-modal__title">Unfortunately, bots use DuckDuckGo too.</div>`))
	}))
	defer server.Close()
	_, err := (DuckDuckGoHTML{Endpoint: server.URL}).Search(context.Background(), http.DefaultClient, "test", 5)
	if err == nil || !strings.Contains(err.Error(), "202 Accepted") {
		t.Fatalf("error = %v", err)
	}
}

func TestAdvancedToolReturnsStructuredDenial(t *testing.T) {
	root := t.TempDir()
	runtimeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(runtimeDir, "default-allow-tools.txt"), []byte("# no default tools\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manager, err := permissions.New(runtimeDir, root, permissions.Default, nil)
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

func TestBrowserInteractReadOnlyOpenUsesDefaultPolicy(t *testing.T) {
	root := t.TempDir()
	prompts := 0
	manager, err := permissions.New(t.TempDir(), root, permissions.Allow, func(context.Context, permissions.Request) permissions.Decision {
		prompts++
		return permissions.AllowOnce
	})
	if err != nil {
		t.Fatal(err)
	}
	runner := &fakeBrowserRunner{}
	registry, err := tools.NewRegistry(root, manager, []tools.Definition{BrowserInteract(runner)})
	if err != nil {
		t.Fatal(err)
	}
	result, err := registry.Execute(context.Background(), "browserInteract", `{"url":"https://example.com","reason":"inspect rendered page"}`)
	if err != nil || prompts != 0 || runner.calls != 1 || result.Result.(map[string]any)["text"] != "Compact page text." {
		t.Fatalf("result = %#v, prompts = %d, calls = %d, err = %v", result, prompts, runner.calls, err)
	}
}

func TestBrowserInteractActionsAlwaysPrompt(t *testing.T) {
	root := t.TempDir()
	prompts := 0
	manager, err := permissions.New(t.TempDir(), root, permissions.Allow, func(context.Context, permissions.Request) permissions.Decision {
		prompts++
		return permissions.AllowOnce
	})
	if err != nil {
		t.Fatal(err)
	}
	runner := &fakeBrowserRunner{}
	registry, err := tools.NewRegistry(root, manager, []tools.Definition{BrowserInteract(runner)})
	if err != nil {
		t.Fatal(err)
	}
	_, err = registry.Execute(context.Background(), "browserInteract", `{"url":"https://example.com","reason":"inspect rendered page","actions":[{"action":"wait","ms":100}]}`)
	if err != nil || prompts != 1 || runner.calls != 1 {
		t.Fatalf("prompts = %d, calls = %d, err = %v", prompts, runner.calls, err)
	}
}

func TestNewCloakBrowserRunnerPrefersProjectVenv(t *testing.T) {
	t.Setenv("OPENHOME_CLOAKBROWSER_PYTHON", "")
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".venv", "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".venv", "bin", "python"), []byte{}, 0o755); err != nil {
		t.Fatal(err)
	}
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(original)
	if runner := NewCloakBrowserRunner(); runner.Python != filepath.Join(".venv", "bin", "python") {
		t.Fatalf("python = %q", runner.Python)
	}
}

func TestBrowserInteractRejectsNonHTTPGoto(t *testing.T) {
	root := t.TempDir()
	manager, err := permissions.New(t.TempDir(), root, permissions.Allow, func(context.Context, permissions.Request) permissions.Decision {
		return permissions.AllowOnce
	})
	if err != nil {
		t.Fatal(err)
	}
	runner := &fakeBrowserRunner{}
	registry, err := tools.NewRegistry(root, manager, []tools.Definition{BrowserInteract(runner)})
	if err != nil {
		t.Fatal(err)
	}
	_, err = registry.Execute(context.Background(), "browserInteract", `{"url":"https://example.com","reason":"inspect","actions":[{"action":"goto","url":"file:///etc/passwd"}]}`)
	if err == nil || runner.calls != 0 {
		t.Fatalf("error = %v, calls = %d", err, runner.calls)
	}
}

func TestBrowserInteractResolvesWorkspaceScreenshotPath(t *testing.T) {
	root := t.TempDir()
	manager, err := permissions.New(t.TempDir(), root, permissions.Allow, func(context.Context, permissions.Request) permissions.Decision {
		return permissions.AllowOnce
	})
	if err != nil {
		t.Fatal(err)
	}
	runner := &fakeBrowserRunner{}
	registry, err := tools.NewRegistry(root, manager, []tools.Definition{BrowserInteract(runner)})
	if err != nil {
		t.Fatal(err)
	}
	result, err := registry.Execute(context.Background(), "browserInteract", `{"url":"https://example.com","reason":"capture","screenshotPath":"screenshots/article.png","screenshotOutputPath":"/tmp/smuggled.png"}`)
	if err != nil {
		t.Fatal(err)
	}
	if runner.args["screenshotOutputPath"] != filepath.Join(root, "screenshots", "article.png") {
		t.Fatalf("runner args = %#v", runner.args)
	}
	if result.Result.(map[string]any)["screenshotPath"] != "screenshots/article.png" {
		t.Fatalf("result = %#v", result)
	}
}

func TestBrowserInteractRejectsScreenshotTraversal(t *testing.T) {
	root := t.TempDir()
	manager, err := permissions.New(t.TempDir(), root, permissions.Allow, func(context.Context, permissions.Request) permissions.Decision {
		return permissions.AllowOnce
	})
	if err != nil {
		t.Fatal(err)
	}
	runner := &fakeBrowserRunner{}
	registry, err := tools.NewRegistry(root, manager, []tools.Definition{BrowserInteract(runner)})
	if err != nil {
		t.Fatal(err)
	}
	_, err = registry.Execute(context.Background(), "browserInteract", `{"url":"https://example.com","reason":"capture","screenshotPath":"../outside.png"}`)
	if err == nil || runner.calls != 0 {
		t.Fatalf("error = %v, calls = %d", err, runner.calls)
	}
}

func TestBrowserInteractRejectsScreenshotHideSelectorsWithoutPath(t *testing.T) {
	root := t.TempDir()
	manager, err := permissions.New(t.TempDir(), root, permissions.Allow, func(context.Context, permissions.Request) permissions.Decision {
		return permissions.AllowOnce
	})
	if err != nil {
		t.Fatal(err)
	}
	runner := &fakeBrowserRunner{}
	registry, err := tools.NewRegistry(root, manager, []tools.Definition{BrowserInteract(runner)})
	if err != nil {
		t.Fatal(err)
	}
	_, err = registry.Execute(context.Background(), "browserInteract", `{"url":"https://example.com","reason":"capture","screenshotHideSelectors":["[role=dialog]"]}`)
	if err == nil || runner.calls != 0 {
		t.Fatalf("error = %v, calls = %d", err, runner.calls)
	}
}

func TestSendEmailRequiresConfigurationBeforeSending(t *testing.T) {
	t.Setenv("OPENHOME_SMTP_FROM", "")
	t.Setenv("OPENHOME_SMTP_ADDR", "")
	root := t.TempDir()
	manager, err := permissions.New(t.TempDir(), root, permissions.Allow, nil)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := tools.NewRegistry(root, manager, []tools.Definition{SendEmail()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = registry.Execute(context.Background(), "sendEmail", `{"to":["person@example.com"],"subject":"Hello","body":"Hi","reason":"send requested email"}`)
	if err == nil || !strings.Contains(err.Error(), "OPENHOME_SMTP_FROM") {
		t.Fatalf("error = %v", err)
	}
}

func TestSendEmailRejectsHeaderInjection(t *testing.T) {
	t.Setenv("OPENHOME_SMTP_FROM", "sender@example.com")
	t.Setenv("OPENHOME_SMTP_ADDR", "smtp.example.com:587")
	root := t.TempDir()
	manager, err := permissions.New(t.TempDir(), root, permissions.Allow, nil)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := tools.NewRegistry(root, manager, []tools.Definition{SendEmail()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = registry.Execute(context.Background(), "sendEmail", `{"to":["person@example.com"],"subject":"Hello\r\nBcc: hidden@example.com","body":"Hi","reason":"send requested email"}`)
	if err == nil || !strings.Contains(err.Error(), "line breaks") {
		t.Fatalf("error = %v", err)
	}
}
