package logging

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type RunLogger struct {
	FilePath string
	mu       sync.Mutex
}

func NewRunLogger(runsDir string) *RunLogger {
	id := fmt.Sprintf("%s-%d", time.Now().UTC().Format("2006-01-02T15-04-05-000Z"), time.Now().UnixNano()%1_000_000)
	return &RunLogger{FilePath: filepath.Join(runsDir, id+".jsonl")}
}

func (l *RunLogger) Append(eventType string, data any) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(l.FilePath), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(l.FilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewEncoder(file).Encode(map[string]any{
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"type":      eventType,
		"data":      data,
	})
}
