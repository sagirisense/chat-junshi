package ocr

import (
	"image"
	_ "image/png"
	"os"
	"testing"
)

func TestV4SyntheticRecognition(t *testing.T) {
	models, dll, fixture := os.Getenv("JUNSHI_TEST_OCR_MODELS"), os.Getenv("JUNSHI_TEST_OCR_DLL"), os.Getenv("JUNSHI_TEST_OCR_IMAGE")
	if models == "" || dll == "" || fixture == "" {
		t.Skip("set JUNSHI_TEST_OCR_MODELS/DLL/IMAGE for real v4 inference")
	}
	file, err := os.Open(fixture)
	if err != nil {
		t.Fatal(err)
	}
	img, _, err := image.Decode(file)
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	engine, err := Open(models, dll)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	rows, err := engine.Read(img)
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{"虚构聊天测试", "你好，今天辛苦了", "明天下午三点见"}
	if len(rows) != len(expected) {
		t.Fatal(rows)
	}
	for i, row := range rows {
		if row.Text != expected[i] || row.Score < .9 {
			t.Fatal(row)
		}
	}
}

func TestDetectionConfigurationAndComponents(t *testing.T) {
	mask := []bool{true, true, false, false, false, false, false, true, true}
	components := connectedComponents(mask, 3, 3)
	if len(components) != 2 {
		t.Fatal(components)
	}
	config := NewDetector().GetDefaultConfig().(*DetectionConfig)
	if config.GetOnnxConfig() == nil {
		t.Fatal("missing session configuration")
	}
}
