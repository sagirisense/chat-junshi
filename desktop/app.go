package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"io"
	"junshi-desktop/internal/codexclient"
	"junshi-desktop/internal/modelapi"
	"junshi-desktop/internal/service"
	"os"
	"path/filepath"
	"time"
)

type App struct {
	ctx        context.Context
	service    *service.Service
	startError error
}

func NewApp() *App { return &App{} }

func projectRoot() (string, error) {
	executable, _ := os.Executable()
	cwd, _ := os.Getwd()
	for _, start := range []string{filepath.Dir(executable), cwd} {
		for current := start; ; current = filepath.Dir(current) {
			if _, err := os.Stat(filepath.Join(current, "desktop", "wails.json")); err == nil {
				return current, nil
			}
			if filepath.Dir(current) == current {
				break
			}
		}
	}
	return "", errors.New("未找到源码项目目录，请从完整项目启动")
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	executable, _ := os.Executable()
	assets := filepath.Join(filepath.Dir(executable), "runtime")
	var root string
	if _, err := os.Stat(assets); err == nil {
		local, err := os.UserCacheDir()
		if err != nil {
			a.startError = err
			return
		}
		root = filepath.Join(local, "ChatJunshi")
	} else {
		var err error
		root, err = projectRoot()
		if err != nil {
			a.startError = err
			return
		}
		assets = filepath.Join(root, ".go-cache", "runtime")
	}
	if override:=os.Getenv("JUNSHI_DATA_ROOT"); override!="" { root=override }
	if err := service.BackupBeforeMigration(root); err != nil {
		a.startError = err
		return
	}
	host, err := service.New(root, func(state service.Data) { runtime.EventsEmit(a.ctx, "service:state", state) })
	if err != nil {
		a.startError = err
		return
	}
	a.service = host
	if _, err = os.Stat(filepath.Join(assets, "skill", "goutoujunshi", "SKILL.md")); err == nil {
		host.SkillRoot = filepath.Join(assets, "skill", "goutoujunshi")
	}
	host.ConfigureCapture(filepath.Join(assets, "models"), filepath.Join(assets, "onnxruntime.dll"), filepath.Join(assets, "junshi_wgc.dll"))
}

func (a *App) Call(action string, payload map[string]interface{}) (interface{}, error) {
	if action == "models" && payload["provider"] != "custom" {
		ctx := a.ctx
		if ctx == nil {
			ctx = context.Background()
		}
		ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
		defer cancel()
		server, err := codexclient.Start()
		if err != nil {
			return nil, err
		}
		defer server.Close()
		if err = server.Initialize(ctx, "wechat_junshi"); err != nil {
			return nil, err
		}
		rows, err := server.Models(ctx)
		if err != nil {
			return nil, err
		}
		models := []string{}
		for _, row := range rows {
			if model, ok := row["model"].(string); ok {
				models = append(models, model)
			}
		}
		return map[string]interface{}{"models": models}, nil
	}
	if action == "models" && payload["provider"] == "custom" {
		encoded, err := json.Marshal(payload["api"])
		if err != nil {
			return nil, err
		}
		var config modelapi.Config
		if err = json.Unmarshal(encoded, &config); err != nil {
			return nil, errors.New("API 配置无效")
		}
		ctx := a.ctx
		if ctx == nil {
			ctx = context.Background()
		}
		models, err := (modelapi.Client{}).Models(ctx, config)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"models": models}, nil
	}
	if a.startError != nil {
		return nil, a.startError
	}
	if a.service == nil {
		return nil, errors.New("Go 服务尚未就绪，请稍后重试")
	}
	return a.service.Dispatch(action, payload)
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
	if a.service != nil {
		a.service.Close()
	}
}
