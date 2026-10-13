package ocr

import (
	"image"
	"image/color"
	"image/draw"
	"junshi-desktop/internal/evidence"
	"testing"
	"time"
)

func solid(img *image.RGBA, rect image.Rectangle, c color.RGBA) {
	draw.Draw(img, rect, &image.Uniform{C: c}, image.Point{}, draw.Src)
}
func boxResult(top, bottom int, text string) Result {
	return Result{Box: [4][2]int{{60, top}, {160, top}, {160, bottom}, {60, bottom}}, Text: text, Score: 1}
}
func bubbleFixture(bubbles [][2]int, boxes []Result) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 300, 200))
	solid(img, img.Bounds(), color.RGBA{238, 238, 238, 255})
	for _, bounds := range bubbles {
		solid(img, image.Rect(50, bounds[0], 200, bounds[1]), color.RGBA{255, 255, 255, 255})
	}
	for _, box := range boxes {
		r := rect(box.Box)
		solid(img, image.Rect(70, r.Min.Y+2, 120, r.Max.Y-2), color.RGBA{0, 0, 0, 255})
	}
	return img
}

func TestBubbleCompletenessAndMerge(t *testing.T) {
	for _, test := range []struct {
		bubbles           [][2]int
		boxes             []Result
		count, incomplete int
		text              string
	}{
		{[][2]int{{0, 60}, {140, 200}}, []Result{boxResult(20, 35, "后半段"), boxResult(160, 175, "前半段")}, 0, 2, ""},
		{[][2]int{{40, 130}}, []Result{boxResult(55, 70, "完整第一行"), boxResult(95, 110, "完整第二行")}, 1, 0, "完整第一行完整第二行"},
		{[][2]int{{20, 50}, {70, 100}}, []Result{boxResult(30, 45, "第一条"), boxResult(80, 95, "第二条")}, 2, 0, "第一条"},
	} {
		reader := &Reader{}
		rows := reader.Read(bubbleFixture(test.bubbles, test.boxes), RGB{238, 238, 238}, test.boxes, time.Now())
		if len(rows) != test.count || reader.IncompleteCount != test.incomplete {
			t.Fatal(rows, reader.IncompleteCount)
		}
		if len(rows) > 0 && rows[0].Text != test.text {
			t.Fatal(rows)
		}
		if test.count == 0 && len(reader.NewLines(rows)) != 0 {
			t.Fatal("clipped text marked seen")
		}
	}
}

func TestMissedVisibleRows(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 120))
	white := color.RGBA{255, 255, 255, 255}
	black := color.RGBA{0, 0, 0, 255}
	solid(img, img.Bounds(), white)
	solid(img, image.Rect(65, 30, 100, 42), black)
	solid(img, image.Rect(65, 65, 100, 77), black)
	bounds := &[2]int{20, 90}
	spans := []image.Rectangle{image.Rect(60, 28, 110, 44)}
	if !HasUnreadText(img, bounds, spans, RGB{255, 255, 255}) {
		t.Fatal("accepted missed line")
	}
	spans = append(spans, image.Rect(60, 63, 110, 79))
	if HasUnreadText(img, bounds, spans, RGB{255, 255, 255}) {
		t.Fatal("rejected complete text")
	}
}

func TestRepeatedRepliesAndScrolledHistory(t *testing.T) {
	old := []Line{{Speaker: "her", Text: "是的"}, {Speaker: "me", Text: "我吃了药"}, {Speaker: "her", Text: "嗯嗯"}}
	reader := &Reader{Seen: append([]Line{}, old...)}
	lines := append(append([]Line{}, old...), Line{Speaker: "me", Text: "那你早点休息吧"}, Line{Speaker: "her", Text: "嗯嗯"})
	for i := range lines {
		lines[i].Y = i * 50
	}
	if got := reader.NewLines(lines); len(got) != 2 || got[0].Text != "那你早点休息吧" {
		t.Fatal(got)
	}
	if len(reader.NewLines(lines)) != 0 {
		t.Fatal("repeated existing messages")
	}
	reader = &Reader{Seen: []Line{{Speaker: "her", Text: "嗯嗯"}}}
	lines = []Line{{Speaker: "her", Text: "嗯嗯", Y: 20}, {Speaker: "her", Text: "嗯嗯", Y: 80}}
	if len(reader.NewLines(lines)) != 1 || len(reader.NewLines(lines)) != 0 {
		t.Fatal("lost new repeated reply")
	}
	if len(reader.NewLines(append([]Line{{Speaker: "me", Text: "更早历史", Y: 0}}, lines...))) != 0 {
		t.Fatal("appended scrolled history")
	}
	if Similar("今天忙了一整天", "今天忙了一整天，明天再找你聊天") {
		t.Fatal("swallowed fuller OCR")
	}
}

func TestCenteredTimeLabelAndBubbleText(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 300, 200))
	pane := color.RGBA{238, 238, 238, 255}
	solid(img, img.Bounds(), pane)
	solid(img, image.Rect(50, 70, 200, 110), color.RGBA{255, 255, 255, 255})
	stamp := Result{Box: [4][2]int{{110, 20}, {190, 20}, {190, 40}, {110, 40}}, Text: "昨天 18:30"}
	message := boxResult(80, 100, "昨天 18:30")
	solid(img, image.Rect(120, 25, 180, 35), color.RGBA{140, 140, 140, 255})
	solid(img, image.Rect(70, 84, 120, 96), color.RGBA{0, 0, 0, 255})
	reader := &Reader{}
	rows := reader.Read(img, RGB{238, 238, 238}, []Result{stamp, message}, time.Date(2026, 10, 13, 12, 0, 0, 0, evidence.China))
	if len(rows) != 1 || rows[0].Text != "昨天 18:30" || rows[0].Context == nil {
		t.Fatal(rows)
	}
}
