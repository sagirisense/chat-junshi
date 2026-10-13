//go:build windows

package capture

import (
	"errors"
	"golang.org/x/sys/windows"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var user32 = windows.NewLazySystemDLL("user32.dll")
var enumWindows = user32.NewProc("EnumWindows")
var isVisible = user32.NewProc("IsWindowVisible")
var isWindow = user32.NewProc("IsWindow")
var getPID = user32.NewProc("GetWindowThreadProcessId")
var getTitle = user32.NewProc("GetWindowTextW")
var getRect = user32.NewProc("GetWindowRect")
var isIconic = user32.NewProc("IsIconic")
var showWindow = user32.NewProc("ShowWindow")
var setPosition = user32.NewProc("SetWindowPos")

type Window struct {
	HWND       uintptr
	PID        uint32
	Created    windows.Filetime
	Executable string
}

func processIdentity(pid uint32) (string, windows.Filetime, error) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", windows.Filetime{}, err
	}
	defer windows.CloseHandle(handle)
	buffer := make([]uint16, 1024)
	length := uint32(len(buffer))
	if err = windows.QueryFullProcessImageName(handle, 0, &buffer[0], &length); err != nil {
		return "", windows.Filetime{}, err
	}
	var created, exit, kernel, user windows.Filetime
	if err = windows.GetProcessTimes(handle, &created, &exit, &kernel, &user); err != nil {
		return "", windows.Filetime{}, err
	}
	return strings.ToLower(filepath.Base(windows.UTF16ToString(buffer[:length]))), created, nil
}

func FindWeChat() (Window, error) {
	type candidate struct {
		window Window
		title  string
		area   int64
	}
	candidates := []candidate{}
	callback := syscall.NewCallback(func(hwnd, parameter uintptr) uintptr {
		visible, _, _ := isVisible.Call(hwnd)
		if visible == 0 {
			return 1
		}
		var pid uint32
		getPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		exe, created, err := processIdentity(pid)
		if err != nil || (exe != "wechat.exe" && exe != "weixin.exe") {
			return 1
		}
		text := make([]uint16, 256)
		getTitle.Call(hwnd, uintptr(unsafe.Pointer(&text[0])), uintptr(len(text)))
		var rect struct{ left, top, right, bottom int32 }
		ok, _, _ := getRect.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
		if ok == 0 {
			return 1
		}
		candidates = append(candidates, candidate{Window{HWND: hwnd, PID: pid, Created: created, Executable: exe}, windows.UTF16ToString(text), int64(rect.right-rect.left) * int64(rect.bottom-rect.top)})
		return 1
	})
	enumWindows.Call(callback, 0)
	if len(candidates) == 0 {
		return Window{}, errors.New("没找到微信窗口，开着吗？")
	}
	best := candidates[0]
	for _, candidate := range candidates {
		if candidate.title == "微信" {
			return candidate.window, nil
		}
		if candidate.area > best.area {
			best = candidate
		}
	}
	return best.window, nil
}

func (w Window) Valid() bool {
	exists, _, _ := isWindow.Call(w.HWND)
	if exists == 0 {
		return false
	}
	var pid uint32
	getPID.Call(w.HWND, uintptr(unsafe.Pointer(&pid)))
	if pid != w.PID {
		return false
	}
	exe, created, err := processIdentity(pid)
	if w.Executable != "" {
		return err == nil && created == w.Created && exe == w.Executable
	}
	return err == nil && created == w.Created && (exe == "wechat.exe" || exe == "weixin.exe")
}

func (w Window) RestoreWithoutActivation() bool {
	minimized, _, _ := isIconic.Call(w.HWND)
	if minimized == 0 {
		return false
	}
	showWindow.Call(w.HWND, 4)
	setPosition.Call(w.HWND, 1, 0, 0, 0, 0, 0x13)
	return true
}
