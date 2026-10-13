// ocr-probe compares fixed synthetic images; it never captures a user window.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	_ "image/png"
	"junshi-desktop/internal/ocr"
	"os"
	"time"
)

func main() {
	models := flag.String("models", "", "PP-OCRv4 model directory")
	dll := flag.String("dll", "", "ONNX Runtime DLL")
	path := flag.String("image", "", "synthetic PNG input")
	flag.Parse()
	if err := run(*models, *dll, *path); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(models, dll, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	img, _, err := image.Decode(file)
	file.Close()
	if err != nil {
		return err
	}
	started := time.Now()
	engine, err := ocr.Open(models, dll)
	if err != nil {
		return err
	}
	defer engine.Close()
	loaded := time.Since(started).Seconds()
	started = time.Now()
	rows, err := engine.Read(img)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]interface{}{"load_s": loaded, "ocr_s": time.Since(started).Seconds(), "results": rows})
}
