package capture

import (
	"image"
	"image/color"
	"image/draw"
	"testing"
)

func TestPixelAnchoredChatArea(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 600, 500))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{200, 200, 200, 255}}, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(200, 0, 600, 500), &image.Uniform{C: color.RGBA{238, 238, 238, 255}}, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(200, 350, 600, 351), &image.Uniform{C: color.RGBA{180, 180, 180, 255}}, image.Point{}, draw.Src)
	area := ChatArea(img, 60)
	if area == nil || area.Chat != image.Rect(200, 60, 600, 350) || area.Header != image.Rect(200, 0, 600, 60) {
		t.Fatal(area)
	}
	if ChatArea(image.NewRGBA(image.Rect(0, 0, 50, 30)), 60) != nil {
		t.Fatal("accepted tiny window")
	}
}
