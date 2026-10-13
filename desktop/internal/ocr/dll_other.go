//go:build !windows

package ocr

func prepareDLL(path string) (func(), error) { return func() {}, nil }
