package capture

import (
	"image"
	"math"
)

type RGB [3]int
type Area struct {
	Chat, Header image.Rectangle
	Background   RGB
}

func colorAt(img image.Image, x, y int) RGB {
	bounds := img.Bounds()
	r, g, b, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
	return RGB{int(r >> 8), int(g >> 8), int(b >> 8)}
}
func distance(a, b RGB) int {
	sum := 0
	for i := range a {
		diff := a[i] - b[i]
		if diff < 0 {
			diff = -diff
		}
		sum += diff
	}
	return sum
}

func ChatArea(img image.Image, headerHeight int) *Area {
	height, width := img.Bounds().Dy(), img.Bounds().Dx()
	if height < 40 || width < 100 {
		return nil
	}
	if headerHeight <= 0 {
		headerHeight = 60
	}
	counts := map[RGB]int{}
	bg := RGB{}
	most := 0
	for y := 0; y < height; y += 8 {
		for x := width / 2; x < width; x += 8 {
			c := colorAt(img, x, y)
			counts[c]++
			if counts[c] > most || (counts[c] == most && colorLess(c, bg)) {
				most = counts[c]
				bg = c
			}
		}
	}
	left, right := -1, -1
	for x := 0; x < width; x++ {
		count := 0
		for y := height / 4; y < height*3/4; y++ {
			if distance(colorAt(img, x, y), bg) <= 6 {
				count++
			}
		}
		if float64(count)/float64(height*3/4-height/4) > .3 {
			if left < 0 {
				left = x
			}
			right = x + 1
		}
	}
	if left < 0 || right-left < 100 {
		return nil
	}
	row := make([]float64, height)
	top, bottom := -1, -1
	for y := 0; y < height; y++ {
		count := 0
		for x := left; x < right; x++ {
			if distance(colorAt(img, x, y), bg) <= 6 {
				count++
			}
		}
		row[y] = float64(count) / float64(right-left)
		if row[y] > .9 {
			if top < 0 {
				top = y
			}
			bottom = y + 1
		}
	}
	if top < 0 {
		return nil
	}
	separators := []int{}
	previous := -100
	for y := top; y < bottom; y++ {
		if row[y] >= .1 {
			continue
		}
		sum, squares := 0.0, 0.0
		for x := left; x < right; x++ {
			for _, value := range colorAt(img, x, y) {
				v := float64(value)
				sum += v
				squares += v * v
			}
		}
		n := float64((right - left) * 3)
		deviation := math.Sqrt(max(0, squares/n-(sum/n)*(sum/n)))
		if deviation < 4 {
			if y-previous > 3 {
				separators = append(separators, y)
			}
			previous = y
		}
	}
	input := bottom
	for _, y := range separators {
		if float64(y) > float64(top)+.45*float64(bottom-top) {
			input = y
			break
		}
	}
	chatTop := top + headerHeight
	for _, y := range separators {
		if y > top+headerHeight && y < input-50 {
			chatTop = y
		}
	}
	if input-chatTop < 40 {
		return nil
	}
	return &Area{image.Rect(left, chatTop, right, input), image.Rect(left, top, right, chatTop), bg}
}

func colorLess(a, b RGB) bool {
	for i := 0; i < 3; i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}
