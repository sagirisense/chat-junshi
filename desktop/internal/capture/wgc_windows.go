//go:build windows

package capture

import (
	"errors"
	"golang.org/x/sys/windows"
	"image"
	"path/filepath"
	"runtime"
	"sync"
	"unsafe"
)

type WGC struct {
	mu          sync.Mutex
	window      Window
	dll         *windows.DLL
	handle      uintptr
	frame, stop *windows.Proc
	sequence    uint64
}

func OpenWGC(window Window, path string) (*WGC, error) {
	if !window.Valid() {
		return nil, errors.New("微信窗口身份已变化，请重新开启采集")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.LoadLibraryEx(absolute, 0, windows.LOAD_LIBRARY_SEARCH_DLL_LOAD_DIR|windows.LOAD_LIBRARY_SEARCH_DEFAULT_DIRS)
	if err != nil {
		return nil, errors.New("WGC 原生库加载失败，请检查安装包")
	}
	dll := &windows.DLL{Name: absolute, Handle: handle}
	start, err := dll.FindProc("junshi_wgc_start")
	if err != nil {
		dll.Release()
		return nil, err
	}
	frame, err := dll.FindProc("junshi_wgc_frame")
	if err != nil {
		dll.Release()
		return nil, err
	}
	stop, err := dll.FindProc("junshi_wgc_stop")
	if err != nil {
		dll.Release()
		return nil, err
	}
	session, _, _ := start.Call(window.HWND)
	if session == 0 {
		dll.Release()
		return nil, errors.New("WGC 无法启动，请检查窗口与系统版本")
	}
	return &WGC{window: window, dll: dll, handle: session, frame: frame, stop: stop}, nil
}

func (w *WGC) Snapshot() (*image.RGBA, uint64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.handle == 0 || !w.window.Valid() {
		return nil, 0, errors.New("微信窗口已关闭或身份已变化")
	}
	var width, height uint32
	var sequence uint64
	code, _, _ := w.frame.Call(w.handle, 0, 0, uintptr(unsafe.Pointer(&width)), uintptr(unsafe.Pointer(&height)), uintptr(unsafe.Pointer(&sequence)))
	if int32(code) < 0 {
		return nil, 0, errors.New("微信采集会话已关闭")
	}
	if code == 0 {
		return nil, 0, nil
	}
	for attempt := 0; attempt < 3; attempt++ {
		if width == 0 || height == 0 || uint64(width)*uint64(height) > 64*1024*1024 {
			return nil, 0, errors.New("采集帧尺寸无效")
		}
		pixels := make([]byte, int(width)*int(height)*4)
		code, _, _ := w.frame.Call(w.handle, uintptr(unsafe.Pointer(&pixels[0])), uintptr(len(pixels)), uintptr(unsafe.Pointer(&width)), uintptr(unsafe.Pointer(&height)), uintptr(unsafe.Pointer(&sequence)))
		runtime.KeepAlive(pixels)
		if code == 2 {
			continue
		}
		if int32(code) < 0 {
			return nil, 0, errors.New("微信采集会话已关闭")
		}
		if code == 0 {
			return nil, 0, nil
		}
		pixels = pixels[:int(width)*int(height)*4]
		w.sequence = sequence
		return &image.RGBA{Pix: pixels, Stride: int(width) * 4, Rect: image.Rect(0, 0, int(width), int(height))}, sequence, nil
	}
	return nil, 0, nil
}

func (w *WGC) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.handle != 0 {
		w.stop.Call(w.handle)
		w.handle = 0
	}
	if w.dll != nil {
		w.dll.Release()
		w.dll = nil
	}
}
