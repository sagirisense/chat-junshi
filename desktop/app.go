package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx        context.Context
	process    *exec.Cmd
	input      io.WriteCloser
	mu         sync.Mutex
	sequence   int
	pending    map[int]chan response
	startError error
}

type response struct {
	ID    int             `json:"id"`
	Data  json.RawMessage `json:"data"`
	Error string          `json:"error"`
	Event string          `json:"event"`
}

func NewApp() *App { return &App{pending: make(map[int]chan response)} }

func projectRoot() (string, error) {
	executable, _ := os.Executable()
	cwd, _ := os.Getwd()
	for _, start := range []string{filepath.Dir(executable), cwd} {
		for current := start; ; current = filepath.Dir(current) {
			if _, err := os.Stat(filepath.Join(current, "desktop_service.py")); err == nil {
				return current, nil
			}
			if filepath.Dir(current) == current {
				break
			}
		}
	}
	return "", errors.New("未找到采集服务，请从完整项目目录启动")
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	executable, _ := os.Executable()
	bundled := filepath.Join(filepath.Dir(executable), "service", "junshi-service.exe")
	if _, err := os.Stat(bundled); err == nil {
		local, err := os.UserCacheDir()
		if err != nil {
			a.startError = err
			return
		}
		root := filepath.Join(local, "ChatJunshi")
		if err = os.MkdirAll(root, 0700); err != nil {
			a.startError = err
			return
		}
		a.process = exec.Command(bundled)
		a.process.Dir = root
		a.process.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8", "JUNSHI_DATA_ROOT="+root)
	} else {
		root, err := projectRoot()
		if err != nil {
			a.startError = err
			return
		}
		a.process = exec.Command(filepath.Join(root, ".venv", "Scripts", "python.exe"), "-u", filepath.Join(root, "desktop_service.py"))
		a.process.Dir = root
		a.process.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")
	}
	var err error
	a.process.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	a.input, err = a.process.StdinPipe()
	if err != nil {
		a.startError = err
		return
	}
	output, err := a.process.StdoutPipe()
	if err != nil {
		a.startError = err
		return
	}
	if err = a.process.Start(); err != nil {
		a.startError = err
		return
	}
	go func() {
		scanner := bufio.NewScanner(output)
		scanner.Buffer(make([]byte, 65536), 8*1024*1024)
		for scanner.Scan() {
			var item response
			if json.Unmarshal(scanner.Bytes(), &item) != nil {
				continue
			}
			if item.Event != "" {
				var data interface{}
				json.Unmarshal(item.Data, &data)
				runtime.EventsEmit(a.ctx, "service:"+item.Event, data)
			} else {
				a.mu.Lock()
				channel := a.pending[item.ID]
				a.mu.Unlock()
				if channel != nil {
					select {
					case channel <- item:
					default:
					}
				}
			}
		}
		a.mu.Lock()
		a.startError = errors.New("本地采集服务已停止，请重启应用")
		for _, channel := range a.pending {
			select {
			case channel <- response{Error: a.startError.Error()}:
			default:
			}
		}
		a.mu.Unlock()
		runtime.EventsEmit(a.ctx, "service:error", "本地采集服务已停止，请重启应用")
	}()
}

func (a *App) Call(action string, payload map[string]interface{}) (interface{}, error) {
	a.mu.Lock()
	if a.startError != nil {
		err := a.startError
		a.mu.Unlock()
		return nil, err
	}
	if a.input == nil {
		a.mu.Unlock()
		return nil, errors.New("服务尚未就绪，请稍后重试")
	}
	a.sequence++
	id := a.sequence
	channel := make(chan response, 1)
	a.pending[id] = channel
	bytes, _ := json.Marshal(map[string]interface{}{"id": id, "action": action, "payload": payload})
	_, err := a.input.Write(append(bytes, '\n'))
	a.mu.Unlock()
	defer func() { a.mu.Lock(); delete(a.pending, id); a.mu.Unlock() }()
	if err != nil {
		return nil, errors.New("无法连接采集服务")
	}
	select {
	case result := <-channel:
		if result.Error != "" {
			return nil, errors.New(result.Error)
		}
		var data interface{}
		err = json.Unmarshal(result.Data, &data)
		return data, err
	case <-time.After(60 * time.Second):
		return nil, errors.New("本地操作超时，请检查采集服务")
	}
}

func (a *App) Copy(text string) error { return runtime.ClipboardSetText(a.ctx, text) }
func (a *App) Export(text string) (bool, error) {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: "chat-evidence.json", Title: "导出当前会话",
		Filters: []runtime.FileFilter{{DisplayName: "JSON 聊天证据", Pattern: "*.json"}},
	})
	if err != nil || path == "" {
		return false, err
	}
	err = os.WriteFile(path, []byte(text), 0600)
	return err == nil, err
}
func (a *App) ExportProfile(text string) (bool, error) {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: "relationship-profile.json", Title: "导出用户与关系档案",
		Filters: []runtime.FileFilter{{DisplayName: "JSON 档案", Pattern: "*.json"}},
	})
	if err != nil || path == "" {
		return false, err
	}
	err = os.WriteFile(path, []byte(text), 0600)
	return err == nil, err
}
func (a *App) ImportJSON() (string, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择导入的 JSON 文件", Filters: []runtime.FileFilter{{DisplayName: "JSON 文件", Pattern: "*.json"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 16*1024*1024+1))
	if err != nil {
		return "", err
	}
	if len(data) > 16*1024*1024 {
		return "", errors.New("导入文件不能超过16MB")
	}
	return string(data), nil
}
func (a *App) Pin(enabled bool) { runtime.WindowSetAlwaysOnTop(a.ctx, enabled) }
func (a *App) WindowAction(action string) {
	switch action {
	case "minimize":
		runtime.WindowMinimise(a.ctx)
	case "maximize":
		runtime.WindowToggleMaximise(a.ctx)
	case "close":
		runtime.Quit(a.ctx)
	}
}
func (a *App) shutdown(ctx context.Context) {
	if a.input != nil {
		a.input.Close()
	}
	if a.process != nil && a.process.Process != nil {
		done := make(chan struct{})
		go func() { a.process.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			a.process.Process.Kill()
		}
	}
}
