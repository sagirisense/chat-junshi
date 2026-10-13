//go:build windows

package ocr

import (
	"golang.org/x/sys/windows"
	"path/filepath"
)

func prepareDLL(path string) (func(), error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.LoadLibraryEx(absolute, 0, windows.LOAD_LIBRARY_SEARCH_DLL_LOAD_DIR|windows.LOAD_LIBRARY_SEARCH_DEFAULT_DIRS)
	if err != nil {
		return nil, err
	}
	return func() { windows.FreeLibrary(handle) }, nil
}
