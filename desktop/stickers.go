package main

import (
	"errors"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"junshi-desktop/internal/stickers"
	"os"
	"path/filepath"
)

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
