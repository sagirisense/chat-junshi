// Package diagnostics records operational metadata without request or response content.
package diagnostics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

var mu sync.Mutex
var target string

// Init keeps at most two 2 MiB log files under the application's data root.
func Init(root string) error {
	mu.Lock()
	defer mu.Unlock()
	dir := filepath.Join(root, "data", "logs")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	path := filepath.Join(dir, "desktop.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	target = path
	return nil
}

// Record accepts only developer-defined stages; never pass user text or raw errors.
func Record(stage string, started time.Time, err error) {
	row := map[string]interface{}{"time": time.Now().Format(time.RFC3339Nano), "stage": stage, "pid": os.Getpid(), "level": "info"}
	if !started.IsZero() {
		row["elapsed_ms"] = time.Since(started).Milliseconds()
	}
	if err != nil {
		row["level"] = "error"
		// Error strings can contain API responses, tokens, titles and local paths.
		row["error_type"] = fmt.Sprintf("%T", err)
		frames := []string{}
		for depth := 1; depth <= 5; depth++ {
			pc, file, line, ok := runtime.Caller(depth)
			if !ok {
				break
			}
			frames = append(frames, fmt.Sprintf("%s %s:%d", runtime.FuncForPC(pc).Name(), filepath.Base(file), line))
		}
		row["callers"] = frames
	}
	data, _ := json.Marshal(row)
	mu.Lock()
	defer mu.Unlock()
	if target == "" {
		return
	}
	if info, err := os.Stat(target); err == nil && info.Size()+int64(len(data)+1) > 2*1024*1024 {
		if err := os.Remove(target + ".1"); err != nil && !os.IsNotExist(err) {
			return
		}
		if err := os.Rename(target, target+".1"); err != nil {
			return
		}
	}
	f, err := os.OpenFile(target, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(data, '\n'))
}
