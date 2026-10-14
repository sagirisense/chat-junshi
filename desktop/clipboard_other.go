//go:build !windows

package main

import "errors"

func copyStickerImage(data []byte) error { return errors.New("图片复制仅支持 Windows") }
