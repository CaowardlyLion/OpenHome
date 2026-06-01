package openai

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientReportsEndpointError(t *testing.T) {
	client := New("http://127.0.0.1:65534/v1", "test", "")
	var output map[string]any
	err := client.Structured(context.Background(), "system", nil, "user", "test", map[string]any{"type": "object"}, &output)
	if err == nil || !strings.Contains(err.Error(), "OpenAI-compatible endpoint unavailable") {
		t.Fatalf("error = %v", err)
	}
}

func TestStructuredRetriesPlainTextResponse(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		requests++
		content := "I ignored JSON."
		if requests == 2 {
			content = `{"answer":"ok"}`
		}
		fmt.Fprintf(response, `{"choices":[{"message":{"role":"assistant","content":%q}}]}`, content)
	}))
	defer server.Close()
	client := New(server.URL, "test", "")
	var output struct {
		Answer string `json:"answer"`
	}
	if err := client.Structured(context.Background(), "system", nil, "user", "test", map[string]any{"type": "object"}, &output); err != nil {
		t.Fatal(err)
	}
	if output.Answer != "ok" || requests != 2 {
		t.Fatalf("output = %#v, requests = %d", output, requests)
	}
}

func TestStructuredAcceptsFencedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(response, `{"choices":[{"message":{"role":"assistant","content":"`+"```json\\n{\\\"answer\\\":\\\"ok\\\"}\\n```"+`"}}]}`)
	}))
	defer server.Close()
	client := New(server.URL, "test", "")
	var output struct {
		Answer string `json:"answer"`
	}
	if err := client.Structured(context.Background(), "system", nil, "user", "test", map[string]any{"type": "object"}, &output); err != nil {
		t.Fatal(err)
	}
	if output.Answer != "ok" {
		t.Fatalf("output = %#v", output)
	}
}

func TestStructuredFailsAfterRetry(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		requests++
		fmt.Fprint(response, `{"choices":[{"message":{"role":"assistant","content":"still prose"}}]}`)
	}))
	defer server.Close()
	client := New(server.URL, "test", "")
	var output map[string]any
	err := client.Structured(context.Background(), "system", nil, "user", "test", map[string]any{"type": "object"}, &output)
	if err == nil || !strings.Contains(err.Error(), "after retry") || requests != 2 {
		t.Fatalf("error = %v, requests = %d", err, requests)
	}
}
