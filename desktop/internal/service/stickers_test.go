package service

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"junshi-desktop/internal/stickers"
	"testing"
)

func TestStickerRecommendationRejectsUnavailableIDs(t *testing.T) {
	s := &Service{Root: t.TempDir()}
	var b bytes.Buffer
	png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, 2, 2)))
	row, err := s.stickerStore().Import("hello.png", b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	candidate := Data{"sticker_id": row.ID}
	result := Data{"candidates": []interface{}{candidate}}
	if s.attachStickers(result, nil) == nil {
		t.Fatal("accepted ID outside this request")
	}
	if err = s.attachStickers(result, []stickers.Sticker{row}); err != nil {
		t.Fatal(err)
	}
	if candidate["sticker"].(Data)["preview"] == "" {
		t.Fatal("missing image")
	}
	candidate["sticker_id"] = "S001"
	if err = s.attachStickers(result, []stickers.Sticker{row}); err != nil {
		t.Fatal(err)
	}
	if candidate["sticker_id"] != row.ID || candidate["sticker"].(Data)["id"] != row.ID {
		t.Fatal("short ID was not resolved")
	}
	catalog := stickerCatalog([]stickers.Sticker{row})
	if catalog[0]["id"] != "S001" || len(catalog[0]) != 6 {
		t.Fatal("catalog includes unrelated storage metadata")
	}
	candidate["sticker_id"] = ""
	if s.attachStickers(result, []stickers.Sticker{row}) == nil {
		t.Fatal("accepted missing no-image reason")
	}
	candidate["sticker_reason"] = "当前语境更适合认真用文字回应"
	if err = s.attachStickers(result, []stickers.Sticker{row}); err != nil {
		t.Fatal(err)
	}
	candidate["sticker_id"] = row.ID
	if err = s.stickerStore().Delete(row.ID); err != nil {
		t.Fatal(err)
	}
	if s.attachStickers(result, []stickers.Sticker{row}) == nil {
		t.Fatal("accepted missing image")
	}
}
func TestStickerSelectionIncludesAllEligibleImages(t *testing.T) {
	s := &Service{Root: t.TempDir()}
	excluded := map[string]bool{}
	var relevant string
	for i := 0; i < 45; i++ {
		img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
		img.Set(0, 0, color.NRGBA{R: byte(i), A: 255})
		var b bytes.Buffer
		png.Encode(&b, img)
		row, err := s.stickerStore().Import("test.png", b.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		row.Tags = "收到"
		row.Enabled = true
		if i == 42 {
			row.Enabled = false
		}
		if i == 43 {
			row.Intimate = true
		}
		if i == 44 {
			row.Tags = "晚安"
			relevant = row.ID
		}
		if err = s.stickerStore().Update(row); err != nil {
			t.Fatal(err)
		}
		if i == 41 {
			if err = s.stickerStore().Delete(row.ID); err != nil {
				t.Fatal(err)
			}
		}
		if i >= 41 && i <= 43 {
			excluded[row.ID] = true
		}
	}
	selected, err := s.selectStickers(false)
	if err != nil || len(selected) != 42 || selected[len(selected)-1].ID != relevant {
		t.Fatal(len(selected), err)
	}
	for _, row := range selected {
		if excluded[row.ID] {
			t.Fatal("selected unavailable or intimate sticker")
		}
	}
}

func TestStickerNoticeDistinguishesNoSelectionFromMissingFields(t *testing.T) {
	input := Generation{StickerMode: "natural", Stickers: []stickers.Sticker{{ID: "available"}}}
	missing := Data{"candidates": []interface{}{Data{"text": "你好"}}}
	if got := stickerNotice(input, missing); got != "本轮已提供 1 张表情，但模型未完整返回表情选择；可尝试重新生成或换模型。" {
		t.Fatal(got)
	}
	textOnly := Data{"candidates": []interface{}{Data{"sticker_id": ""}}}
	if got := stickerNotice(input, textOnly); got != "本轮已提供 1 张表情，模型选择了纯文字；可切换“优先表情”后重新生成。" {
		t.Fatal(got)
	}
	selected := Data{"candidates": []interface{}{Data{"sticker_id": "available"}}}
	if got := stickerNotice(input, selected); got != "本轮提供 1 张可选表情，已自动搭配 1 条回复。" {
		t.Fatal(got)
	}
	input.Stickers = nil
	if got := stickerNotice(input, missing); got != "本轮没有可用表情，请在表情库启用素材，并核对亲密表情设置。" {
		t.Fatal(got)
	}
	input.StickerMode = "none"
	if got := stickerNotice(input, selected); got != "" {
		t.Fatal(got)
	}
}
