package diagnostics

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecordPrivacyAndRotation(t *testing.T) {
	root := t.TempDir()
	if err := Init(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { mu.Lock(); target = ""; mu.Unlock() })
	path := filepath.Join(root, "data", "logs", "desktop.log")
	Record("analysis.generate", time.Now(), errors.New("secret-token private-chat"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret-token") || strings.Contains(string(data), "private-chat") {
		t.Fatal("error content leaked")
	}
	var row map[string]interface{}
	if err := json.Unmarshal(data, &row); err != nil {
		t.Fatal(err)
	}
	if row["level"] != "error" || row["stage"] != "analysis.generate" || row["callers"] == nil || row["elapsed_ms"] == nil {
		t.Fatalf("missing diagnostic fields: %v", row)
	}
	if err := os.WriteFile(path, make([]byte, 2*1024*1024), 0600); err != nil {
		t.Fatal(err)
	}
	Record("startup", time.Time{}, nil)
	if info, err := os.Stat(path + ".1"); err != nil || info.Size() != 2*1024*1024 {
		t.Fatal("rotation failed", err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &row); err != nil {
		t.Fatal(err)
	}
	if row["stage"] != "startup" {
		t.Fatal("new log missing")
	}
}
