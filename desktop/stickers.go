package main

import (
	"context"
	"encoding/base64"
	"errors"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"io"
	"junshi-desktop/internal/codexclient"
	"junshi-desktop/internal/modelapi"
	"junshi-desktop/internal/stickers"
	"os"
	"path/filepath"
)

func (a *App) AnalyzeSticker(id, provider, model string, config modelapi.Config) (interface{}, error) {
	store, err := a.stickerStore()
	if err != nil {
		return nil, err
	}
	data, err := store.Read(id)
	if err != nil {
		return nil, err
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if provider == "custom" {
		return (modelapi.Client{}).AnalyzeSticker(ctx, config, model, data)
	}
	if provider != "codex" {
		return nil, errors.New("模型接入方式无效")
	}
	transport := &codexclient.Transport{}
	defer transport.Close()
	return transport.Analyze(ctx, codexclient.AnalyzeRequest{Task: "sticker", Quick: true, Model: model, Packet: "请分析这张表情图片，按要求返回中文标签 JSON。", ImageURL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)}, nil)
}

func (a *App) ExportStickerLibrary() (bool, error) {
	if _, err := a.stickerStore(); err != nil {
		return false, err
	}
	result, err := a.service.Dispatch("sticker_export_library", nil)
	if err != nil {
		return false, err
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{Title: "导出表情库", DefaultFilename: "chat-junshi-stickers.zip", Filters: []runtime.FileFilter{{DisplayName: "表情库 ZIP", Pattern: "*.zip"}}})
	if err != nil || path == "" {
		return false, err
	}
	if filepath.Ext(path) == "" {
		path += ".zip"
	}
	err = os.WriteFile(path, result.([]byte), 0600)
	return err == nil, err
}

func (a *App) ImportStickerLibrary() (interface{}, error) {
	if _, err := a.stickerStore(); err != nil {
		return nil, err
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "导入表情库", Filters: []runtime.FileFilter{{DisplayName: "本工具导出的表情库 ZIP", Pattern: "*.zip"}}})
	if err != nil || path == "" {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, stickers.MaxArchiveBytes+1))
	if err != nil {
		return nil, err
	}
	return a.service.Dispatch("sticker_import_library", map[string]interface{}{"data": data})
}

func (a *App) stickerStore() (stickers.Store, error) {
	if a.startError != nil {
		return stickers.Store{}, a.startError
	}
	if a.service == nil {
		return stickers.Store{}, errors.New("服务尚未准备好")
	}
	return stickers.Store{Dir: filepath.Join(a.service.Root, "data", "stickers")}, nil
}
func (a *App) ImportStickers() (interface{}, error) {
	_, err := a.stickerStore()
	if err != nil {
		return nil, err
	}
	paths, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{Title: "导入静态表情", Filters: []runtime.FileFilter{{DisplayName: "静态表情 PNG / JPG / WebP", Pattern: "*.png;*.jpg;*.jpeg;*.webp"}}})
	if err != nil {
		return nil, err
	}
	failures := []string{}
	count := 0
	for _, path := range paths {
		info, e := os.Stat(path)
		if e != nil || info.Size() > 8*1024*1024 {
			failures = append(failures, filepath.Base(path)+"：无法读取或超过 8 MB")
			continue
		}
		data, e := os.ReadFile(path)
		if e == nil {
			_, e = a.service.Dispatch("sticker_import", map[string]interface{}{"name": filepath.Base(path), "data": data})
		}
		if e != nil {
			failures = append(failures, filepath.Base(path)+"："+e.Error())
		} else {
			count++
		}
	}
	return map[string]interface{}{"count": count, "failures": failures}, nil
}
func (a *App) CopySticker(id string) error {
	store, err := a.stickerStore()
	if err != nil {
		return err
	}
	data, err := store.Read(id)
	if err != nil {
		return err
	}
	return copyStickerImage(data)
}
