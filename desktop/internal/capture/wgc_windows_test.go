//go:build windows

package capture

import (
	"golang.org/x/sys/windows"
	"os"
	"runtime"
	"testing"
	"time"
	"unsafe"
)

func TestNativeWGCOnSyntheticWindow(t *testing.T) {
	path := os.Getenv("JUNSHI_TEST_WGC_DLL")
	if path == "" {
		t.Skip("set JUNSHI_TEST_WGC_DLL for native synthetic-window integration")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	class, _ := windows.UTF16PtrFromString("STATIC")
	title, _ := windows.UTF16PtrFromString("JUNSHI SYNTHETIC CAPTURE TEST")
	create := user32.NewProc("CreateWindowExW")
	destroy := user32.NewProc("DestroyWindow")
	update := user32.NewProc("UpdateWindow")
	hwnd, _, err := create.Call(0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)), 0x10cf0000, 30, 30, 600, 400, 0, 0, 0, 0)
	if hwnd == 0 {
		t.Fatal(err)
	}
	defer destroy.Call(hwnd)
	showWindow.Call(hwnd, 5)
	update.Call(hwnd)
	pid := uint32(os.Getpid())
	exe, created, err := processIdentity(pid)
	if err != nil {
		t.Fatal(err)
	}
	target := Window{HWND: hwnd, PID: pid, Created: created, Executable: exe}
	cap, err := OpenWGC(target, path)
	if err != nil {
		t.Fatal(err)
	}
	defer cap.Close()
	wait := func(differentWidth int) int {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			frame, sequence, err := cap.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			if frame != nil && sequence > 0 && frame.Bounds().Dx() != differentWidth {
				if len(frame.Pix) != frame.Bounds().Dx()*frame.Bounds().Dy()*4 {
					t.Fatal("frame length mismatch")
				}
				return frame.Bounds().Dx()
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatal("no valid synthetic frame")
		return 0
	}
	width := wait(0)
	setPosition.Call(hwnd, 0, 30, 30, 700, 450, 0x14)
	update.Call(hwnd)
	wait(width)
	cap.Close()
	cap.Close()
	if _, _, err = cap.Snapshot(); err == nil {
		t.Fatal("read after close")
	}
	cap, err = OpenWGC(target, path)
	if err != nil {
		t.Fatal(err)
	}
	wait(0)
	destroy.Call(hwnd)
	if _, _, err = cap.Snapshot(); err == nil {
		t.Fatal("accepted destroyed HWND")
	}
	cap.Close()
}
