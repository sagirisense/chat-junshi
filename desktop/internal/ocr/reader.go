package ocr

import (
	"image"
	"junshi-desktop/internal/evidence"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Line struct {
	Speaker, Name, Text string
	Y                   int
	Context             *evidence.TimeContext
}
type Reader struct {
	LineHeight      float64
	Seen            []Line
	IncompleteCount int
}
type bubbleLine struct {
	line             Line
	top, bottom, ink int
	bounds           *[2]int
	clipped          bool
	spans            []image.Rectangle
	bg               RGB
}
type anchor struct {
	bottom  int
	context *evidence.TimeContext
}

var memberSuffix = regexp.MustCompile(`\s*[（(]\d+[)）]\s*$`)

func ReadTitle(results []Result) string {
	if len(results) == 0 {
		return ""
	}
	first := results[0]
	for _, row := range results {
		if row.Box[0][1] < first.Box[0][1] {
			first = row
		}
	}
	floor := first.Box[2][1]
	text := first.Text
	x := first.Box[0][0]
	for _, row := range results {
		if row.Box[0][1] < floor && row.Box[0][0] < x {
			x = row.Box[0][0]
			text = row.Text
		}
	}
	return memberSuffix.ReplaceAllString(strings.TrimSpace(text), "")
}

func (r *Reader) Read(img image.Image, pane RGB, results []Result, now time.Time) []Line {
	r.IncompleteCount = 0
	sorted := append([]Result{}, results...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Box[0][1] < sorted[j].Box[0][1] })
	raw := []bubbleLine{}
	anchors := []anchor{}
	name := ""
	width, height := img.Bounds().Dx(), img.Bounds().Dy()
	for _, row := range sorted {
		kind, bg, ink := WhoSaid(img, row.Box)
		box := rect(row.Box)
		onPane := colorDistance(bg, pane) <= 6
		center := float64(box.Min.X+box.Max.X) / 2
		if onPane && center >= .3*float64(width) && center <= .7*float64(width) {
			if stamp := evidence.ParseLabel(row.Text, now); stamp != nil {
				var combined *evidence.TimeContext
				if len(anchors) > 0 && abs(anchors[len(anchors)-1].bottom-box.Max.Y) <= 5 {
					previous := anchors[len(anchors)-1].context.Label
					combined = evidence.ParseLabel(previous+" "+row.Text, now)
					if combined == nil {
						combined = evidence.ParseLabel(row.Text+" "+previous, now)
					}
				}
				if combined != nil {
					anchors[len(anchors)-1] = anchor{max(anchors[len(anchors)-1].bottom, box.Max.Y), combined}
				} else {
					anchors = append(anchors, anchor{box.Max.Y, stamp})
				}
				name = ""
				continue
			}
		}
		if kind == "gray" || (onPane && kind == "her") {
			if onPane && row.Box[0][0] < width/4 && len([]rune(row.Text)) <= 16 && !strings.ContainsAny(row.Text, ":：") {
				name = row.Text
			}
			continue
		}
		if kind == "" || (r.LineHeight > 0 && float64(ink) < .6*r.LineHeight) {
			continue
		}
		bounds := BubbleBounds(img, row.Box, bg)
		clipped := box.Min.Y <= 2 || box.Max.Y >= height-2 || (bounds != nil && (bounds[0] <= 1 || bounds[1] >= height-2))
		lineName := ""
		if kind == "her" {
			lineName = name
		}
		raw = append(raw, bubbleLine{Line{Speaker: kind, Name: lineName, Text: row.Text, Y: box.Min.Y}, box.Min.Y, box.Max.Y, ink, bounds, clipped, []image.Rectangle{box}, bg})
	}
	if r.LineHeight == 0 && len(raw) >= 3 {
		heights := []int{}
		for _, row := range raw {
			heights = append(heights, row.ink)
		}
		sort.Ints(heights)
		r.LineHeight = float64(heights[len(heights)/2])
		if len(heights)%2 == 0 {
			r.LineHeight = float64(heights[len(heights)/2-1]+heights[len(heights)/2]) / 2
		}
	}
	merged := []bubbleLine{}
	for _, row := range raw {
		merge := false
		if len(merged) > 0 {
			previous := &merged[len(merged)-1]
			same := row.bounds != nil && previous.bounds != nil && abs(row.bounds[0]-previous.bounds[0]) <= 3 && abs(row.bounds[1]-previous.bounds[1]) <= 3
			height := r.LineHeight
			if height == 0 {
				height = float64(row.ink)
			}
			merge = previous.line.Speaker == row.line.Speaker && previous.line.Name == row.line.Name && (same || (row.bounds == nil && previous.bounds == nil && float64(row.top-previous.bottom) < .6*height))
			if merge {
				previous.line.Text += row.line.Text
				previous.bottom = row.bottom
				previous.clipped = previous.clipped || row.clipped
				previous.spans = append(previous.spans, row.spans...)
			}
		}
		if !merge {
			merged = append(merged, row)
		}
	}
	lines := []Line{}
	for _, row := range merged {
		if row.clipped || HasUnreadText(img, row.bounds, row.spans, row.bg) {
			r.IncompleteCount++
			continue
		}
		for _, stamp := range anchors {
			if stamp.bottom < row.line.Y {
				row.line.Context = stamp.context
			}
		}
		lines = append(lines, row.line)
	}
	return lines
}

func Similar(a, b string) bool {
	x, y := []rune(a), []rune(b)
	if a != b && (strings.Contains(a, b) || strings.Contains(b, a)) && abs(len(x)-len(y)) >= 4 {
		return false
	}
	if a == b {
		return true
	}
	// Match Python SequenceMatcher, including its >=200-character autojunk rule.
	positions := map[rune][]int{}
	for j, char := range y {
		positions[char] = append(positions[char], j)
	}
	if len(y) >= 200 {
		for char, indices := range positions {
			if len(indices) > len(y)/100+1 {
				delete(positions, char)
			}
		}
	}
	var matched func(int, int, int, int) int
	matched = func(alo, ahi, blo, bhi int) int {
		best, xi, yi := 0, alo, blo
		prior := map[int]int{}
		for i := alo; i < ahi; i++ {
			current := map[int]int{}
			for _, j := range positions[x[i]] {
				if j < blo {
					continue
				}
				if j >= bhi {
					break
				}
				n := prior[j-1] + 1
				current[j] = n
				if n > best {
					best = n
					xi = i - n + 1
					yi = j - n + 1
				}
			}
			prior = current
		}
		for xi > alo && yi > blo && x[xi-1] == y[yi-1] {
			xi--
			yi--
			best++
		}
		for xi+best < ahi && yi+best < bhi && x[xi+best] == y[yi+best] {
			best++
		}
		if best == 0 {
			return 0
		}
		count := best
		if xi > alo && yi > blo {
			count += matched(alo, xi, blo, yi)
		}
		if xi+best < ahi && yi+best < bhi {
			count += matched(xi+best, ahi, yi+best, bhi)
		}
		return count
	}
	if len(x)+len(y) > 0 && float64(2*matched(0, len(x), 0, len(y)))/float64(len(x)+len(y)) >= .75 {
		return true
	}
	if len(x) == len(y) && len(x) >= 3 {
		differences := 0
		for i := range x {
			if x[i] != y[i] {
				differences++
			}
		}
		return differences <= 1
	}
	return false
}

func (r *Reader) NewLines(lines []Line) []evidence.CapturedLine {
	cursor, floor := 0, -1
	for _, line := range lines {
		for i := cursor; i < len(r.Seen); i++ {
			old := r.Seen[i]
			if old.Speaker == line.Speaker && Similar(old.Text, line.Text) {
				floor = max(floor, line.Y)
				cursor = i + 1
				break
			}
		}
	}
	result := []evidence.CapturedLine{}
	for _, line := range lines {
		if line.Y > floor {
			r.Seen = append(r.Seen, line)
			result = append(result, evidence.CapturedLine{Speaker: line.Speaker, Text: line.Text, Context: line.Context})
		}
	}
	if len(r.Seen) > 500 {
		r.Seen = r.Seen[len(r.Seen)-500:]
	}
	return result
}
