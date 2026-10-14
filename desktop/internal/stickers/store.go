package stickers

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

type Sticker struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Tags     string `json:"tags"`
	Suitable string `json:"suitable"`
	Avoid    string `json:"avoid"`
	Source   string `json:"source"`
	License  string `json:"license"`
	Intimate bool   `json:"intimate"`
	Enabled  bool   `json:"enabled"`
}
type Store struct{ Dir string }

func (s Store) List() ([]Sticker, error) {
	rows := []Sticker{}
	data, err := os.ReadFile(filepath.Join(s.Dir, "index.json"))
	if os.IsNotExist(err) {
		return rows, nil
	}
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &rows)
	return rows, err
}
func validID(id string) bool {
	if len(id) != 64 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}
func (s Store) Read(id string) ([]byte, error) {
	if !validID(id) {
		return nil, errors.New("表情 ID 无效")
	}
	return os.ReadFile(filepath.Join(s.Dir, id+".png"))
}
func (s Store) Preview(id string) (string, error) {
	b, e := s.Read(id)
	if e != nil {
		return "", e
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(b), nil
}
func (s Store) save(rows []Sticker) error {
	if err := os.MkdirAll(s.Dir, 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.Dir, "index-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(s.Dir, "index.json"))
}
func (s Store) Import(name string, data []byte) (Sticker, error) {
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		for pos := 12; pos+8 <= len(data); {
			chunk := string(data[pos : pos+4])
			if chunk == "ANIM" || chunk == "ANMF" {
				return Sticker{}, errors.New("首版仅支持静态 WebP，请先选择静态画面")
			}
			size := uint64(data[pos+4]) | uint64(data[pos+5])<<8 | uint64(data[pos+6])<<16 | uint64(data[pos+7])<<24
			next := uint64(pos) + 8 + size + (size & 1)
			if next > uint64(len(data)) {
				break
			}
			pos = int(next)
		}
	}
	if len(data) > 8*1024*1024 {
		return Sticker{}, errors.New("图片不能超过 8 MB")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return Sticker{}, errors.New("请选择 PNG、JPG 或 WebP 静态图片")
	}
	if format == "gif" || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 2048 || cfg.Height > 2048 {
		return Sticker{}, errors.New("仅支持最长边不超过 2048 的静态图片")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return Sticker{}, err
	}
	var out bytes.Buffer
	if err = png.Encode(&out, img); err != nil {
		return Sticker{}, err
	}
	hash := sha256.Sum256(out.Bytes())
	id := hex.EncodeToString(hash[:])
	rows, err := s.List()
	if err != nil {
		return Sticker{}, err
	}
	originalHash := sha256.Sum256(data)
	originalID := hex.EncodeToString(originalHash[:])
	for _, row := range rows {
		if row.ID == id || row.ID == originalID {
			return row, nil
		}
	}
	row := Sticker{ID: id, Name: strings.TrimSuffix(filepath.Base(name), filepath.Ext(name)), License: "用户导入，分发授权未确认"}
	if err = os.MkdirAll(s.Dir, 0700); err != nil {
		return Sticker{}, err
	}
	if err = os.WriteFile(filepath.Join(s.Dir, id+".png"), out.Bytes(), 0600); err != nil {
		return Sticker{}, err
	}
	err = s.save(append(rows, row))
	return row, err
}
func Validate(row Sticker) error {
	if len([]rune(row.Name)) > 80 || strings.TrimSpace(row.Name) == "" || len([]rune(row.Tags)) > 200 || len([]rune(row.Suitable)) > 300 || len([]rune(row.Avoid)) > 300 {
		return errors.New("名称或标签长度无效")
	}
	if row.Enabled && strings.TrimSpace(row.Tags) == "" {
		return errors.New("请先填写标签，再启用 AI 推荐")
	}
	return nil
}
func (s Store) Update(row Sticker) error {
	if err := Validate(row); err != nil {
		return err
	}
	rows, err := s.List()
	if err != nil {
		return err
	}
	for i, old := range rows {
		if old.ID == row.ID {
			row.Source = old.Source
			row.License = old.License
			rows[i] = row
			return s.save(rows)
		}
	}
	return errors.New("表情已不存在")
}
func (s Store) Delete(id string) error {
	if !validID(id) {
		return errors.New("表情 ID 无效")
	}
	rows, err := s.List()
	if err != nil {
		return err
	}
	kept := []Sticker{}
	for _, row := range rows {
		if row.ID != id {
			kept = append(kept, row)
		}
	}
	if err = s.save(kept); err != nil {
		return err
	}
	err = os.Remove(filepath.Join(s.Dir, id+".png"))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
