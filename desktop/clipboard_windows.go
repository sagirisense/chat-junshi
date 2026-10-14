package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"golang.org/x/sys/windows"
	"image/color"
	"image/png"
	"runtime"
	"time"
	"unsafe"
)

// CF_DIB is supported by Windows chat applications. Flatten transparency onto white.
func copyStickerImage(data []byte) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return err
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	stride := (w*3 + 3) &^ 3
	dib := make([]byte, 40+stride*h)
	binary.LittleEndian.PutUint32(dib, 40)
	binary.LittleEndian.PutUint32(dib[4:], uint32(w))
	binary.LittleEndian.PutUint32(dib[8:], uint32(h))
	binary.LittleEndian.PutUint16(dib[12:], 1)
	binary.LittleEndian.PutUint16(dib[14:], 24)
	binary.LittleEndian.PutUint32(dib[20:], uint32(stride*h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.NRGBAModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
			a := int(c.A)
			i := 40 + (h-1-y)*stride + x*3
			dib[i] = byte((int(c.B)*a + 255*(255-a) + 127) / 255)
			dib[i+1] = byte((int(c.G)*a + 255*(255-a) + 127) / 255)
			dib[i+2] = byte((int(c.R)*a + 255*(255-a) + 127) / 255)
		}
	}
	kernel := windows.NewLazySystemDLL("kernel32.dll")
	user := windows.NewLazySystemDLL("user32.dll")
	class, _ := windows.UTF16PtrFromString("STATIC")
	owner, _, _ := user.NewProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(class)), 0, 0, 0, 0, 0, 0, ^uintptr(2), 0, 0, 0)
	if owner == 0 {
		return errors.New("无法创建图片剪贴板窗口")
	}
	defer user.NewProc("DestroyWindow").Call(owner)
	alloc := kernel.NewProc("GlobalAlloc")
	lock := kernel.NewProc("GlobalLock")
	unlock := kernel.NewProc("GlobalUnlock")
	free := kernel.NewProc("GlobalFree")
	handle, _, _ := alloc.Call(0x2, uintptr(len(dib)))
	if handle == 0 {
		return errors.New("无法分配图片剪贴板内存")
	}
	owned := true
	defer func() {
		if owned {
			free.Call(handle)
		}
	}()
	ptr, _, _ := lock.Call(handle)
	if ptr == 0 {
		return errors.New("无法写入图片剪贴板")
	}
	windows.NewLazySystemDLL("ntdll.dll").NewProc("RtlMoveMemory").Call(ptr, uintptr(unsafe.Pointer(&dib[0])), uintptr(len(dib)))
	unlock.Call(handle)
	opened := false
	for i := 0; i < 5; i++ {
		v, _, _ := user.NewProc("OpenClipboard").Call(owner)
		if v != 0 {
			opened = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !opened {
		return errors.New("剪贴板正在被占用，请重试")
	}
	defer user.NewProc("CloseClipboard").Call()
	v, _, _ := user.NewProc("EmptyClipboard").Call()
	if v == 0 {
		return errors.New("无法清空剪贴板")
	}
	v, _, _ = user.NewProc("SetClipboardData").Call(8, handle)
	if v == 0 {
		return errors.New("无法复制图片")
	}
	owned = false
	return nil
}
