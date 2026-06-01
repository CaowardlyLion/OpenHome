package openai

import (
	"context"
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
