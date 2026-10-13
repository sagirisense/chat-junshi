package service

import (
	"encoding/json"
	"errors"
	"junshi-desktop/internal/evidence"
	"path/filepath"
	"strings"
)

func (s *Service) importData(action string, payload Data) (interface{}, error) {
	var data Data
	if json.Unmarshal([]byte(strings.TrimPrefix(str(payload, "text"), "\ufeff")), &data) != nil {
		return nil, errors.New("请选择有效的 JSON 文件")
	}
	if data["version"] != float64(1) {
		return nil, errors.New("不支持的文件格式，请使用本工具导出的 JSON")
	}
	title, ok := data["title"].(string)
	if !ok || len([]rune(trim(title))) > 200 {
		return nil, errors.New("会话名称格式无效")
	}
	title = trim(title)
	if action == "prepare_import_profile" {
		raw, ok := data["profile"].(map[string]interface{})
		if data["type"] != "relationship-profile" || !ok {
			return nil, errors.New("这不是用户档案文件")
		}
		if len(raw) == 0 {
			return nil, errors.New("文件没有档案字段")
		}
		for key := range raw {
			if _, ok := profileFields[key]; !ok {
				return nil, errors.New("档案字段格式无效")
			}
		}
		cleaned, err := cleanProfile(raw)
		if err != nil {
			return nil, err
		}
		draft := map[string]string{}
		for key := range raw {
			draft[key] = cleaned[key]
		}
		return Data{"title": title, "profile": draft}, nil
	}
	messages, ok := data["messages"].([]interface{})
	if data["source"] != "wechat-window-ocr" || title == "" || !ok || len(messages) == 0 || len(messages) > 10000 {
		return nil, errors.New("请选择本工具导出的非空聊天 JSON（最多10000条）")
	}
	cleaned := []evidence.Message{}
	for _, raw := range messages {
		row, ok := raw.(map[string]interface{})
		if !ok {
			return nil, errors.New("聊天消息格式无效")
		}
		sender, _ := row["sender"].(string)
		text, ok := row["text"].(string)
		if (sender != "me" && sender != "peer") || !ok || trim(text) == "" || len([]rune(text)) > 20000 {
			return nil, errors.New("聊天消息格式无效")
		}
		manual := ""
		if row["message_time"] != nil {
			var ok bool
			manual, ok = row["message_time"].(string)
			if !ok {
				return nil, errors.New("请填写有效的消息日期和时间（北京时间），或留空表示未知")
			}
		}
		timestamp, err := evidence.NormalizeManual(manual)
		if err != nil {
			return nil, err
		}
		message := evidence.Message{"sender": sender, "text": text, "message_time": nil, "message_time_source": nil, "time_context": evidence.CleanContext(row["time_context"]), "time_inference_disabled": row["time_inference_disabled"] == true, "observed_at": nil}
		if timestamp != nil {
			message["message_time"] = *timestamp
			message["message_time_source"] = "import"
		}
		if observed, ok := row["observed_at"].(string); ok {
			message["observed_at"] = observed
		}
		cleaned = append(cleaned, message)
	}
	if action == "prepare_import_chat" {
		return Data{"title": title, "count": len(cleaned), "existing": s.Sessions[title] != nil}, nil
	}
	if !truth(payload, "confirm") {
		return nil, errors.New("请确认导入聊天")
	}
	session := s.Sessions[title]
	if session == nil {
		var err error
		session, err = evidence.OpenSession(title, filepath.Join(s.Root, "data"))
		if err != nil {
			return nil, err
		}
	}
	first := session.NextMessageID
	for _, row := range session.Messages {
		if id := number(row, "id"); id >= first {
			first = id + 1
		}
	}
	for i, row := range cleaned {
		row["id"] = first + i
	}
	session.Messages = append(session.Messages, cleaned...)
	session.NextMessageID = first + len(cleaned)
	session.Confirmed = false
	session.Revision++
	if err := session.Save(); err != nil {
		return nil, err
	}
	if s.Sessions[title] == nil {
		s.Sessions[title] = session
		s.order = append(s.order, title)
	}
	if err := s.selectSession(title); err != nil {
		return nil, err
	}
	s.stopCapture()
	s.OCRReady = false
	s.Current.Confirmed = false
	s.invalidate()
	s.Blocked = false
	s.ActiveTitle = ""
	s.Status = "聊天已导入，请重新核对说话人；重复导入会追加重复消息"
	return nil, nil
}
