package stickers

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func fixture(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.NRGBA{255, 0, 0, 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func TestImportPersistenceAndSafeDeletion(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	row, err := store.Import("hello.png", fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if row.Enabled {
		t.Fatal("untagged import must not be recommended")
	}
	again, err := store.Import("duplicate.png", fixture(t))
	if err != nil || again.ID != row.ID {
		t.Fatal(again, err)
	}
	row.Enabled = true
	if store.Update(row) == nil {
		t.Fatal("accepted untagged recommendation")
	}
	row.Tags = "打招呼"
	row.Source = "cannot replace provenance"
	if err = store.Update(row); err != nil {
		t.Fatal(err)
	}
	rows, err := (Store{Dir: store.Dir}).List()
	if err != nil || len(rows) != 1 || !rows[0].Enabled || rows[0].Source != "" {
		t.Fatal(rows, err)
	}
	if _, err = store.Read("../../outside"); err == nil {
		t.Fatal("accepted path traversal")
	}
	if err = store.Delete("../../outside"); err == nil {
		t.Fatal("accepted unsafe deletion")
	}
	if err = store.Delete(row.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Read(row.ID); err == nil {
		t.Fatal("image remains")
	}
	rows, err = store.List()
	if err != nil || len(rows) != 0 {
		t.Fatal(rows, err)
	}
}
func TestRejectInvalidAndAnimatedImages(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	for _, b := range [][]byte{[]byte("not an image"), make([]byte, 8*1024*1024+1), append([]byte("RIFFxxxxWEBPANIM"), 0, 0, 0, 0)} {
		if _, err := store.Import("image.webp", b); err == nil {
			t.Fatal("accepted invalid image")
		}
	}
}
