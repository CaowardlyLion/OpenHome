package definitions

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
)

func stringArg(args map[string]any, name string) (string, error) {
	value, ok := args[name]
	if !ok {
		return "", fmt.Errorf("missing required argument %q", name)
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("argument %q must be a string", name)
	}
	return text, nil
}

func optionalStringArg(args map[string]any, name, fallback string) (string, error) {
	value, ok := args[name]
	if !ok {
		return fallback, nil
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("argument %q must be a string", name)
	}
	return text, nil
}

func stringSliceArg(args map[string]any, name string) ([]string, error) {
	value, ok := args[name]
	if !ok {
		return nil, nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("argument %q must be an array", name)
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("argument %q must contain only strings", name)
		}
		result = append(result, text)
	}
	return result, nil
}

func intArg(args map[string]any, name string, fallback int) (int, error) {
	value, ok := args[name]
	if !ok {
		return fallback, nil
	}
	number, ok := value.(float64)
	if !ok || number != float64(int(number)) {
		return 0, fmt.Errorf("argument %q must be an integer", name)
	}
	return int(number), nil
}

func parseHTTPURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, fmt.Errorf("URL must use http or https")
	}
	return parsed, nil
}

func readLimited(body io.Reader, limit int64) ([]byte, bool, error) {
	content, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, false, err
	}
	if int64(len(content)) > limit {
		return content[:limit], true, nil
	}
	return content, false, nil
}

func selectedHeaders(headers http.Header) map[string]string {
	result := map[string]string{}
	for _, name := range []string{"Content-Type", "Content-Length", "Last-Modified"} {
		if value := headers.Get(name); value != "" {
			result[name] = value
		}
	}
	return result
}

func pathSchema(required bool) map[string]any {
	schema := map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{"path": map[string]any{"type": "string"}},
	}
	if required {
		schema["required"] = []string{"path"}
	}
	return schema
}
