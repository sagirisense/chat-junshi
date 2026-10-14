package stickers

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	"io"
	"net/url"
	"os"
	"path/filepath"
)

const MaxArchiveBytes = 256 * 1024 * 1024

type libraryManifest struct {
	Type     string    `json:"type"`
	Version  int       `json:"version"`
	Stickers []Sticker `json:"stickers"`
}

func (s Store) ExportLibrary() ([]byte, error) {
	rows, err := s.List()
	if err != nil {
		return nil, err
	}
	if len(rows) > 2000 {
		return nil, errors.New("一次最多导出 2000 张表情")
	}
	var out bytes.Buffer
	w := zip.NewWriter(&out)
	manifest, err := json.Marshal(libraryManifest{"chat-junshi-stickers", 1, rows})
	if err != nil {
		return nil, err
	}
	f, err := w.Create("index.json")
	if err != nil {
		return nil, err
	}
	if _, err = f.Write(manifest); err != nil {
		return nil, err
	}
	total := len(manifest)
	for _, row := range rows {
		data, err := s.Read(row.ID)
		if err != nil {
			return nil, err
		}
		total += len(data)
		if total > MaxArchiveBytes {
			return nil, errors.New("表情库超过 256 MB，请减少素材后导出")
		}
		f, err := w.Create("images/" + row.ID + ".png")
		if err != nil {
			return nil, err
		}
		if _, err = f.Write(data); err != nil {
			return nil, err
		}
	}
	if err = w.Close(); err != nil {
		return nil, err
	}
	if out.Len() > MaxArchiveBytes {
		return nil, errors.New("导出的表情库不能超过 256 MB")
	}
	return out.Bytes(), nil
}

func archiveFile(file *zip.File, limit int64) ([]byte, error) {
	if file == nil || file.UncompressedSize64 > uint64(limit) {
		return nil, errors.New("表情库文件缺失或过大")
	}
	r, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("表情库文件过大")
	}
	return data, nil
}

// Validate the entire archive before merging; duplicate images keep local metadata.
func (s Store) ImportLibrary(data []byte) (added, skipped int, err error) {
	if len(data) > MaxArchiveBytes {
		return 0, 0, errors.New("表情库不能超过 256 MB")
	}
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return 0, 0, errors.New("请选择本工具导出的 ZIP 表情库")
	}
	if len(z.File) > 2001 {
		return 0, 0, errors.New("表情库文件数量过多")
	}
	files := map[string]*zip.File{}
	for _, f := range z.File {
		if files[f.Name] != nil {
			return 0, 0, errors.New("表情库存在重复文件")
		}
		files[f.Name] = f
	}
	manifestData, err := archiveFile(files["index.json"], 4*1024*1024)
	if err != nil {
		return 0, 0, err
	}
	var manifest libraryManifest
	if json.Unmarshal(manifestData, &manifest) != nil || manifest.Type != "chat-junshi-stickers" || manifest.Version != 1 || len(manifest.Stickers) > 2000 {
		return 0, 0, errors.New("表情库格式或版本无效")
	}
	images := map[string][]byte{}
	total := len(manifestData)
	for _, row := range manifest.Stickers {
		if !validID(row.ID) || images[row.ID] != nil {
			return 0, 0, errors.New("表情库图片 ID 无效或重复")
		}
		if err = Validate(row); err != nil {
			return 0, 0, err
		}
		if len(row.Source) > 2000 || len(row.License) > 1000 {
			return 0, 0, errors.New("素材来源信息过长")
		}
		if row.Source != "" {
			u, e := url.Parse(row.Source)
			if e != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
				return 0, 0, errors.New("素材来源必须是 http(s) 链接")
			}
		}
		b, e := archiveFile(files["images/"+row.ID+".png"], 8*1024*1024)
		if e != nil {
			return 0, 0, e
		}
		total += len(b)
		if total > MaxArchiveBytes {
			return 0, 0, errors.New("解压后的表情库不能超过 256 MB")
		}
		hash := sha256.Sum256(b)
		cfg, format, e := image.DecodeConfig(bytes.NewReader(b))
		if e != nil || format != "png" || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 2048 || cfg.Height > 2048 || hex.EncodeToString(hash[:]) != row.ID {
			return 0, 0, errors.New("表情库图片损坏或校验失败")
		}
		if _, _, e = image.Decode(bytes.NewReader(b)); e != nil {
			return 0, 0, errors.New("表情库图片损坏")
		}
		images[row.ID] = b
	}
	rows, err := s.List()
	if err != nil {
		return 0, 0, err
	}
	existing := map[string]bool{}
	for _, row := range rows {
		existing[row.ID] = true
	}
	if err = os.MkdirAll(s.Dir, 0700); err != nil {
		return 0, 0, err
	}
	written := []string{}
	defer func() {
		if err != nil {
			for _, path := range written {
				os.Remove(path)
			}
		}
	}()
	for _, row := range manifest.Stickers {
		if existing[row.ID] {
			skipped++
			continue
		}
		path := filepath.Join(s.Dir, row.ID+".png")
		// Preserve any pre-existing orphan file if committing the index fails.
		_, statErr := os.Stat(path)
		if statErr != nil && !os.IsNotExist(statErr) {
			return 0, 0, statErr
		}
		if os.IsNotExist(statErr) {
			written = append(written, path)
		}
		if err = os.WriteFile(path, images[row.ID], 0600); err != nil {
			return 0, 0, err
		}
		rows = append(rows, row)
		added++
	}
	if added > 0 {
		err = s.save(rows)
	}
	return added, skipped, err
}
