package ocr

import (
	"errors"
	"github.com/multippt/gopaddleocr/pkg/ocr/common"
	rec "github.com/multippt/gopaddleocr/pkg/ocr/recognize/paddleocr"
	ort "github.com/yalue/onnxruntime_go"
	"image"
	"path/filepath"
	"sort"
	"sync"
)

type Result struct {
	Box   [4][2]int `json:"box"`
	Text  string    `json:"text"`
	Score float64   `json:"score"`
}
type configSource map[string]common.ModelConfig

func (c configSource) GetConfig(name string) common.ModelConfig { return c[name] }

type Engine struct {
	mu         sync.Mutex
	detector   *Detector
	recognizer *rec.Model
	options    *ort.SessionOptions
	releaseDLL func()
}

// A single OCR runtime is owned by the application, not by individual chats.
func Open(modelDirectory, dll string) (*Engine, error) {
	release, err := prepareDLL(dll)
	if err != nil {
		return nil, err
	}
	ort.SetSharedLibraryPath(dll)
	if err := ort.InitializeEnvironment(); err != nil {
		release()
		return nil, err
	}
	options, err := ort.NewSessionOptions()
	if err != nil {
		ort.DestroyEnvironment()
		release()
		return nil, err
	}
	if err = options.SetIntraOpNumThreads(4); err != nil {
		options.Destroy()
		ort.DestroyEnvironment()
		release()
		return nil, err
	}
	engine := &Engine{detector: NewDetector(), recognizer: rec.NewModel(), options: options, releaseDLL: release}
	det := engine.detector.GetDefaultConfig().(*DetectionConfig)
	det.Mean = [3]float32{.5, .5, .5}
	det.Std = [3]float32{.5, .5, .5}
	det.Thresh = .3
	det.BoxThresh = .5
	det.UnclipRatio = 1.6
	det.UseDilation = true
	det.MinArea = 3
	det.SetOnnxConfig(&common.Config{ModelPath: filepath.Join(modelDirectory, "ch_PP-OCRv4_det_infer.onnx"), Options: options})
	rc := engine.recognizer.GetDefaultConfig().(*rec.ModelConfig)
	rc.SetOnnxConfig(&common.Config{ModelPath: filepath.Join(modelDirectory, "ch_PP-OCRv4_rec_infer.onnx"), Options: options})
	config := configSource{DetectorName: det, rec.ModelName: rc}
	if err = engine.detector.Init(config); err != nil {
		engine.Close()
		return nil, err
	}
	if err = engine.recognizer.Init(config); err != nil {
		engine.Close()
		return nil, err
	}
	return engine, nil
}

func (e *Engine) Read(img image.Image) ([]Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.detector == nil {
		return nil, errors.New("OCR 已关闭")
	}
	boxes, err := e.detector.Detect(img)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(boxes, func(i, j int) bool {
		if boxes[i].Quad[0][1] == boxes[j].Quad[0][1] {
			return boxes[i].Quad[0][0] < boxes[j].Quad[0][0]
		}
		return boxes[i].Quad[0][1] < boxes[j].Quad[0][1]
	})
	rows := []Result{}
	for _, box := range boxes {
		result, err := e.recognizer.Recognize(img, box.Quad)
		if err != nil {
			return nil, err
		}
		if result.Score >= .5 && result.Text != "" {
			rows = append(rows, Result{box.Quad, result.Text, result.Score})
		}
	}
	return rows, nil
}

func (e *Engine) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.detector != nil {
		e.detector.Close()
		e.detector = nil
	}
	if e.recognizer != nil {
		e.recognizer.Close()
		e.recognizer = nil
	}
	if e.options != nil {
		e.options.Destroy()
		e.options = nil
	}
	ort.DestroyEnvironment()
	if e.releaseDLL != nil {
		e.releaseDLL()
		e.releaseDLL = nil
	}
}
