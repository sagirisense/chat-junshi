package stickers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLibraryRoundTripAndDuplicatePreservation(t *testing.T) {
	source := Store{Dir: t.TempDir()}
	row, err := source.Import("测试.png", fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	row.Tags, row.Suitable, row.Avoid, row.Enabled, row.Intimate = "晚安,困了", "结束聊天", "严肃讨论", true, true
	if err = source.Update(row); err != nil {
		t.Fatal(err)
	}
	archive, err := source.ExportLibrary()
	if err != nil {
		t.Fatal(err)
	}
	destination := Store{Dir: t.TempDir()}
	added, skipped, err := destination.ImportLibrary(archive)
	if err != nil || added != 1 || skipped != 0 {
		t.Fatal(added, skipped, err)
	}
	rows, err := destination.List()
	if err != nil || len(rows) != 1 || rows[0] != row {
		t.Fatal(rows, row, err)
	}
	b, err := destination.Read(row.ID)
	original, _ := source.Read(row.ID)
	if err != nil || !bytes.Equal(b, original) {
		t.Fatal("image changed", err)
	}
	rows[0].Tags = "本地修改"
	if err = destination.Update(rows[0]); err != nil {
		t.Fatal(err)
	}
	added, skipped, err = destination.ImportLibrary(archive)
	if err != nil || added != 0 || skipped != 1 {
		t.Fatal(added, skipped, err)
	}
	rows, _ = destination.List()
	if rows[0].Tags != "本地修改" {
		t.Fatal("overwrote local edits")
	}
}

func TestLibraryRejectsCorruptionBeforeMutating(t *testing.T) {
	source := Store{Dir: t.TempDir()}
	row, _ := source.Import("测试.png", fixture(t))
	var out bytes.Buffer
	w := zip.NewWriter(&out)
	b, _ := json.Marshal(libraryManifest{"chat-junshi-stickers", 1, []Sticker{row}})
	f, _ := w.Create("index.json")
	f.Write(b)
	f, _ = w.Create("images/" + row.ID + ".png")
	f.Write([]byte("corrupt"))
	w.Close()
	destination := Store{Dir: filepath.Join(t.TempDir(), "library")}
	if _, _, err := destination.ImportLibrary(out.Bytes()); err == nil {
		t.Fatal("accepted corrupt archive")
	}
	if _, err := os.Stat(destination.Dir); !os.IsNotExist(err) {
		t.Fatal("invalid archive mutated store")
	}
	row.ID = "../../outside"
	out.Reset()
	w = zip.NewWriter(&out)
	b, _ = json.Marshal(libraryManifest{"chat-junshi-stickers", 1, []Sticker{row}})
	f, _ = w.Create("index.json")
	f.Write(b)
	w.Close()
	if _, _, err := destination.ImportLibrary(out.Bytes()); err == nil {
		t.Fatal("accepted unsafe ID")
	}
}
