package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"junshi-desktop/internal/stickers"
	"path/filepath"
	"strings"
)

func (s *Service) stickerStore() stickers.Store {
	return stickers.Store{Dir: filepath.Join(s.Root, "data", "stickers")}
}
func stickerNotice(input Generation, result Data) string {
	if input.StickerMode == "none" {
		return ""
	}
	if len(input.Stickers) == 0 {
		return "本轮没有可用表情，请在表情库启用素材，并核对亲密表情设置。"
	}
	rows, _ := result["candidates"].([]interface{})
	selected, missing := 0, 0
	for _, raw := range rows {
		row, _ := raw.(map[string]interface{})
		id, present := row["sticker_id"]
		if !present {
			missing++
		} else if id != "" {
			selected++
		}
	}
	if selected > 0 {
		return fmt.Sprintf("本轮提供 %d 张可选表情，已自动搭配 %d 条回复。", len(input.Stickers), selected)
	}
	if missing > 0 {
		return fmt.Sprintf("本轮已提供 %d 张表情，但模型未完整返回表情选择；可尝试重新生成或换模型。", len(input.Stickers))
	}
	if input.StickerMode == "prefer" {
		return fmt.Sprintf("本轮已提供 %d 张表情，模型仍未选择搭配，保留文字回复。", len(input.Stickers))
	}
	return fmt.Sprintf("本轮已提供 %d 张表情，模型选择了纯文字；可切换“优先表情”后重新生成。", len(input.Stickers))
}
func (s *Service) selectStickers(allowIntimate bool) ([]stickers.Sticker, error) {
	rows, err := s.stickerStore().List()
	if err != nil {
		return nil, err
	}
	available := []stickers.Sticker{}
	for _, row := range rows {
		if !row.Enabled || row.Intimate && !allowIntimate {
			continue
		}
		if _, err := s.stickerStore().Read(row.ID); err != nil {
			continue
		}
		available = append(available, row)
	}
	return available, nil
}
func stickerCatalog(rows []stickers.Sticker) []Data {
	entries := []Data{}
	for i, row := range rows {
		entries = append(entries, Data{"id": fmt.Sprintf("S%03d", i+1), "name": row.Name, "tags": row.Tags, "suitable": row.Suitable, "avoid": row.Avoid, "intimate": row.Intimate})
	}
	return entries
}
func (s *Service) attachStickers(result Data, available []stickers.Sticker) error {
	allowed := map[string]stickers.Sticker{}
	for i, row := range available {
		allowed[row.ID] = row
		allowed[fmt.Sprintf("S%03d", i+1)] = row
	}
	rows, _ := result["candidates"].([]interface{})
	for _, raw := range rows {
		row, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := row["sticker_id"].(string)
		if id == "" {
			reason, _ := row["sticker_reason"].(string)
			if len(available) > 0 && strings.TrimSpace(reason) == "" {
				return errors.New("模型未说明不搭配表情的原因，请重新生成")
			}
			continue
		}
		sticker, ok := allowed[id]
		if !ok {
			return errors.New("模型选择了本轮不可用的表情，请重新生成")
		}
		preview, err := s.stickerStore().Preview(sticker.ID)
		if err != nil {
			return errors.New("表情文件已不存在，请重新生成")
		}
		row["sticker"] = Data{"id": sticker.ID, "name": sticker.Name, "preview": preview}
		row["sticker_id"] = sticker.ID
	}
	return nil
}
func (s *Service) stickerAction(action string, payload Data) (interface{}, error) {
	store := s.stickerStore()
	switch action {
	case "sticker_import":
		data, ok := payload["data"].([]byte)
		if !ok {
			return nil, errors.New("图片数据无效")
		}
		if _, err := store.Import(str(payload, "name"), data); err != nil {
			return nil, err
		}
	case "sticker_export_library":
		return store.ExportLibrary()
	case "sticker_import_library":
		data, ok := payload["data"].([]byte)
		if !ok {
			return nil, errors.New("表情库数据无效")
		}
		added, skipped, err := store.ImportLibrary(data)
		if err != nil {
			return nil, err
		}
		s.invalidate()
		return Data{"count": added, "skipped": skipped}, nil
	case "stickers":
		rows, err := store.List()
		if err != nil {
			return nil, err
		}
		result := []Data{}
		for _, row := range rows {
			preview, err := store.Preview(row.ID)
			if err != nil {
				continue
			}
			b, _ := json.Marshal(row)
			entry := Data{}
			json.Unmarshal(b, &entry)
			entry["preview"] = preview
			result = append(result, entry)
		}
		return Data{"stickers": result}, nil
	case "sticker_update":
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		var row stickers.Sticker
		if err = json.Unmarshal(b, &row); err != nil {
			return nil, err
		}
		if err = store.Update(row); err != nil {
			return nil, err
		}
	case "sticker_delete":
		if !truth(payload, "confirm") {
			return nil, errors.New("请确认删除表情")
		}
		if err := store.Delete(str(payload, "id")); err != nil {
			return nil, err
		}
	}
	s.invalidate()
	return s.stickerAction("stickers", nil)
}
