// prepare-ocr extracts the dictionary from the pinned model at build time.
package main

import (
	"fmt"
	"github.com/multippt/gopaddleocr/pkg/ocr/recognize/paddleocr"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: prepare-ocr MODEL_DIRECTORY")
		os.Exit(1)
	}
	directory := os.Args[1]
	if _, err := paddleocr.NewCharsetDict(filepath.Join(directory, "ch_PP-OCRv4_rec_infer.onnx"), filepath.Join(directory, paddleocr.CharDictPath)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
