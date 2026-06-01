package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type FunctionDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type Tool struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

type Client struct {
	BaseURL    string
	Model      string
	APIKey     string
	HTTPClient *http.Client
}

func New(baseURL, model, apiKey string) *Client {
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), Model: model, APIKey: apiKey, HTTPClient: http.DefaultClient}
}

func (c *Client) Structured(ctx context.Context, system string, history []Message, prompt, name string, schema map[string]any, out any) error {
	messages := append([]Message{{Role: "system", Content: system}},
		append(cloneMessages(history), Message{Role: "user", Content: prompt})...)
	var decodeErr error
	for attempt := 1; attempt <= 2; attempt++ {
		content, err := c.structuredContent(ctx, messages, name, schema)
		if err != nil {
			return err
		}
		if os.Getenv("OPENHOME_DEBUG_STRUCTURED") != "" {
			fmt.Fprintf(os.Stderr, "[structured:%s] %s\n", name, content)
		}
		if err := json.Unmarshal([]byte(jsonContent(content)), out); err == nil {
			return nil
		} else {
			decodeErr = err
		}
		messages = append(messages,
			Message{Role: "user", Content: "Respond to the original request now. Output only one JSON object matching the requested schema. Do not discuss formatting, prior responses, corrections, Markdown fences, or commentary."},
		)
	}
	return fmt.Errorf("decode structured response after retry: %w", decodeErr)
}

func (c *Client) structuredContent(ctx context.Context, messages []Message, name string, schema map[string]any) (string, error) {
	body := map[string]any{
		"model": c.Model, "messages": messages,
		"response_format": map[string]any{"type": "json_schema", "json_schema": map[string]any{
			"name": name, "strict": true, "schema": schema,
		}},
		"stream": false, "temperature": 0,
	}
	response, err := c.request(ctx, body)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	var decoded struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return "", err
	}
	if len(decoded.Choices) == 0 {
		return "", fmt.Errorf("OpenAI-compatible response contained no choices")
	}
	return decoded.Choices[0].Message.Content, nil
}

func jsonContent(content string) string {
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
	}
	return strings.TrimSpace(content)
}

func (c *Client) Complete(ctx context.Context, system string, messages []Message, tools []Tool) (Message, error) {
	body := map[string]any{
		"model": c.Model, "messages": append([]Message{{Role: "system", Content: system}}, cloneMessages(messages)...),
		"tools": tools, "stream": false, "temperature": 0,
	}
	response, err := c.request(ctx, body)
	if err != nil {
		return Message{}, err
	}
	defer response.Body.Close()
	var decoded struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return Message{}, err
	}
	if len(decoded.Choices) == 0 {
		return Message{}, fmt.Errorf("OpenAI-compatible response contained no choices")
	}
	return decoded.Choices[0].Message, nil
}

func (c *Client) Stream(ctx context.Context, system string, history []Message, prompt string, onToken func(string)) (string, error) {
	body := map[string]any{
		"model": c.Model,
		"messages": append([]Message{{Role: "system", Content: system}},
			append(cloneMessages(history), Message{Role: "user", Content: prompt})...),
		"stream": true,
	}
	response, err := c.request(ctx, body)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	var answer strings.Builder
	scanner := bufio.NewScanner(response.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return "", err
		}
		if len(chunk.Choices) > 0 {
			token := chunk.Choices[0].Delta.Content
			answer.WriteString(token)
			onToken(token)
		}
	}
	return answer.String(), scanner.Err()
}

func (c *Client) Models(ctx context.Context) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/models", nil)
	if err != nil {
		return err
	}
	c.headers(request)
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return fmt.Errorf("OpenAI-compatible endpoint unavailable: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(response.Body)
		return fmt.Errorf("OpenAI-compatible request failed: %d %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (c *Client) request(ctx context.Context, body any) (*http.Response, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	c.headers(request)
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("OpenAI-compatible endpoint unavailable: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		defer response.Body.Close()
		content, _ := io.ReadAll(response.Body)
		return nil, fmt.Errorf("OpenAI-compatible request failed: %d %s", response.StatusCode, strings.TrimSpace(string(content)))
	}
	return response, nil
}

func (c *Client) headers(request *http.Request) {
	request.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		request.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
}

func cloneMessages(messages []Message) []Message {
	return append([]Message(nil), messages...)
}
