package ocr

import (
	"image"
	"math"
)

type RGB [3]int

func pixel(img image.Image, x, y int) RGB {
	b := img.Bounds()
	r, g, blue, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
	return RGB{int(r >> 8), int(g >> 8), int(blue >> 8)}
}
func colorDistance(a, b RGB) int { return abs(a[0]-b[0]) + abs(a[1]-b[1]) + abs(a[2]-b[2]) }
func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
func brightness(p RGB) float64 { return .299*float64(p[0]) + .587*float64(p[1]) + .114*float64(p[2]) }
func rect(box [4][2]int) image.Rectangle {
	left, top, right, bottom := box[0][0], box[0][1], box[0][0], box[0][1]
	for _, p := range box {
		left = min(left, p[0])
		top = min(top, p[1])
		right = max(right, p[0])
		bottom = max(bottom, p[1])
	}
	return image.Rect(left, top, right, bottom)
}

func WhoSaid(img image.Image, box [4][2]int) (string, RGB, int) {
	bounds := rect(box).Intersect(image.Rect(0, 0, img.Bounds().Dx(), img.Bounds().Dy()))
	if bounds.Empty() {
		return "", RGB{}, 0
	}
	counts := map[RGB]int{}
	bg := RGB{}
	most := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			color := pixel(img, x, y)
			counts[color]++
			count := counts[color]
			if count > most || (count == most && lessColor(color, bg)) {
				most = count
				bg = color
			}
		}
	}
	if float64(most)/float64(bounds.Dx()*bounds.Dy()) < .45 {
		return "", bg, 0
	}
	ink, run := 0, 0
	contrast := 0.0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		present := false
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			diff := math.Abs(brightness(pixel(img, x, y)) - brightness(bg))
			contrast = max(contrast, diff)
			if diff > 60 {
				present = true
			}
		}
		if present {
			run++
		} else {
			run = 0
		}
		ink = max(ink, run)
	}
	if bg[1] > bg[0]+40 && bg[1] > bg[2]+40 {
		return "me", bg, ink
	}
	if contrast >= 150 {
		return "her", bg, ink
	}
	return "gray", bg, ink
}

func lessColor(a, b RGB) bool {
	for i := 0; i < 3; i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

func BubbleBounds(img image.Image, box [4][2]int, bg RGB) *[2]int {
	r := rect(box)
	width, height := img.Bounds().Dx(), img.Bounds().Dy()
	left, right := max(0, r.Min.X), min(width, r.Max.X)
	if right <= left || height == 0 {
		return nil
	}
	rows := make([]bool, height)
	for y := 0; y < height; y++ {
		count := 0
		for x := left; x < right; x++ {
			if colorDistance(pixel(img, x, y), bg) <= 8 {
				count++
			}
		}
		rows[y] = float64(count)/float64(right-left) > .15
	}
	middle := max(0, min(height-1, (r.Min.Y+r.Max.Y)/2))
	if !rows[middle] {
		return nil
	}
	top, bottom := middle, middle
	for top > 0 && rows[top-1] {
		top--
	}
	for bottom < height-1 && rows[bottom+1] {
		bottom++
	}
	return &[2]int{top, bottom}
}

func HasUnreadText(img image.Image, bounds *[2]int, spans []image.Rectangle, bg RGB) bool {
	if bounds == nil || len(spans) == 0 {
		return false
	}
	left, right := spans[0].Min.X, spans[0].Max.X
	for _, span := range spans {
		left = min(left, span.Min.X)
		right = max(right, span.Max.X)
	}
	left = max(0, left)
	right = min(img.Bounds().Dx(), right)
	if left >= right {
		return false
	}
	run, longest := 0, 0
	for y := bounds[0]; y <= bounds[1]; y++ {
		covered := false
		for _, span := range spans {
			if y >= span.Min.Y-4 && y < span.Max.Y+5 {
				covered = true
				break
			}
		}
		count := 0
		if !covered {
			for x := left; x < right; x++ {
				if math.Abs(brightness(pixel(img, x, y))-brightness(bg)) > 80 {
					count++
				}
			}
		}
		if count >= 2 {
			run++
		} else {
			run = 0
		}
		longest = max(longest, run)
	}
	return longest >= 5
}
