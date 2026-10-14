package service

import (
	"encoding/json"
	"errors"
	"junshi-desktop/internal/evidence"
	"junshi-desktop/internal/memory"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (s *Service) Dispatch(action string, payload Data) (interface{}, error) {
	s.mu.Lock()
	result, err := s.dispatch(action, payload)
	var event Data
	if err == nil && action != "state" && result == nil {
		event = s.snapshot()
		result = event
	}
	if err == nil && (action == "sticker_import" || action == "sticker_update" || action == "sticker_delete" || action == "sticker_import_library") {
		event = s.snapshot()
	}
	s.mu.Unlock()
	if event != nil && s.Emit != nil {
		s.Emit(event)
	}
	return result, err
}

func (s *Service) dispatch(action string, payload Data) (interface{}, error) {
	switch action {
	case "stickers", "sticker_update", "sticker_delete", "sticker_import", "sticker_export_library", "sticker_import_library":
		return s.stickerAction(action, payload)
	case "capture":
		if s.collector == nil {
			return nil, errors.New("采集资源尚未准备好，请检查安装包")
		}
		s.Capturing = !s.Capturing
		s.collector.SetEnabled(s.Capturing)
		if s.Capturing {
			s.Status = "正在读取微信窗口，请核对会话与原文"
		} else {
			s.Status = "采集已暂停"
		}
		s.invalidate()
	case "analyze":
		return nil, s.beginAnalysis(payload)
	case "prepare_import_chat", "import_chat", "prepare_import_profile":
		return s.importData(action, payload)
	case "state":
		return s.snapshot(), nil
	case "select":
		title := str(payload, "title")
		if s.Sessions[title] == nil {
			return nil, errors.New("该会话尚未保存")
		}
		return nil, s.selectSession(title)
	case "bind_profile":
		title := trim(str(payload, "title"))
		if title == "" || len([]rune(title)) > 200 {
			return nil, errors.New("请填写准确的微信单聊名称")
		}
		if err := s.selectSession(title); err != nil {
			return nil, err
		}
		return nil, s.Current.Save()
	case "confirm":
		if s.Current == nil || s.Blocked {
			return nil, errors.New("请先读取标题明确的单聊")
		}
		s.Current.Confirmed = truth(payload, "confirmed")
		s.Current.Revision++
		s.invalidate()
	case "skip_onboarding":
		s.Onboarded = true
	case "goal":
		return nil, s.setGoal(str(payload, "goal"))
	case "profile":
		raw, ok := payload["profile"].(map[string]interface{})
		if !ok {
			return nil, errors.New("档案字段格式无效")
		}
		return nil, s.applyProfile(raw, truth(payload, "persist"))
	case "correct":
		if s.Current == nil {
			return nil, errors.New("没有当前会话")
		}
		index := -1
		id := number(payload, "id")
		for i, row := range s.Current.Messages {
			if number(row, "id") == id {
				index = i
				break
			}
		}
		var manual *string
		if truth(payload, "time_edit") {
			value := str(payload, "message_time")
			manual = &value
		}
		if err := s.Current.Correct(index, str(payload, "sender"), str(payload, "text"), manual); err != nil {
			return nil, err
		}
		s.invalidate()
	case "delete_message":
		if s.Current == nil || str(payload, "title") != s.Current.Title {
			return nil, errors.New("会话已变化，请重新选择消息")
		}
		if !truth(payload, "confirm") {
			return nil, errors.New("请确认删除这条本地消息")
		}
		if _, err := s.Current.DeleteMessage(number(payload, "id")); err != nil {
			return nil, err
		}
		s.invalidate()
		s.Status = "这条本地消息已删除，旧建议已失效"
	case "packet":
		if s.Current == nil || s.Blocked {
			return nil, errors.New("请先采集一个单聊")
		}
		goal := s.Profile["goal"]
		packet, err := s.Current.Packet(30, &goal, time.Now())
		return Data{"text": packet}, err
	case "export":
		if s.Current == nil {
			return nil, errors.New("请先采集一个会话")
		}
		data, err := json.MarshalIndent(s.Current.Payload(), "", "  ")
		return Data{"text": string(data)}, err
	case "export_profile":
		profile := map[string]string{}
		for key, value := range s.Profile {
			if _, ok := profileFields[key]; ok && value != "" {
				profile[key] = value
			}
		}
		if len(profile) == 0 {
			return nil, errors.New("请先填写或选择已有档案")
		}
		title := ""
		if s.Current != nil {
			title = s.Current.Title
		}
		data, err := json.MarshalIndent(Data{"version": 1, "type": "relationship-profile", "title": title, "profile": profile}, "", "  ")
		return Data{"text": string(data)}, err
	case "clear_chat":
		if s.Current != nil {
			s.stopCapture()
			s.OCRReady = false
			s.Current.Messages = []evidence.Message{}
			s.Current.Confirmed = false
			s.Current.Revision++
			if err := s.Current.Save(); err != nil {
				return nil, err
			}
			s.invalidate()
			s.Status = "当前聊天文字已清空；关系记忆单独保留"
		}
	case "delete_session":
		if !truth(payload, "confirm") {
			return nil, errors.New("请确认删除当前本地会话")
		}
		if s.Current != nil {
			if err := removeFile(s.Current.Path); err != nil {
				return nil, err
			}
			if err := removeFile(filepath.Join(s.Root, "data", "selected-session.json")); err != nil {
				return nil, err
			}
			title := s.Current.Title
			delete(s.Sessions, title)
			for i, value := range s.order {
				if value == title {
					s.order = append(s.order[:i], s.order[i+1:]...)
					break
				}
			}
			s.Current = nil
			s.ActiveTitle = ""
			s.Profile = map[string]string{}
			s.stopCapture()
			s.OCRReady = false
			s.Blocked = false
			s.Onboarded = false
			s.invalidate()
			s.Status = "本地会话已删除，关系记忆保留；重新采集后可建立会话"
		}
	case "memory":
		return nil, s.memoryOperation(payload)
	case "invalidate_assistance":
		title := ""
		if s.Current != nil {
			title = s.Current.Title
		}
		if str(payload, "title") == title {
			s.invalidate()
			s.AssistanceNotice = ""
		}
	default:
		return nil, errors.New("未知操作")
	}
	return nil, nil
}

func removeFile(path string) error {
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *Service) setGoal(value string) error {
	if s.Current == nil {
		return errors.New("请先选择一个会话")
	}
	goal := trim(value)
	if len([]rune(goal)) > 200 {
		return errors.New("回复目标请控制在200字以内")
	}
	if s.Profile["goal"] != goal {
		s.Profile["goal"] = goal
		if s.profiles[s.subject()] == nil {
			s.profiles[s.subject()] = map[string]string{}
		}
		s.profiles[s.subject()]["goal"] = goal
		s.Current.Revision++
		s.invalidate()
	}
	return nil
}

func cleanProfile(raw Data) (map[string]string, error) {
	cleaned := map[string]string{}
	for key := range profileFields {
		value := ""
		switch v := raw[key].(type) {
		case nil:
		case string:
			value = trim(v)
		case float64:
			value = strconv.FormatFloat(v, 'f', -1, 64)
		case int:
			value = strconv.Itoa(v)
		default:
			return nil, errors.New("档案字段格式无效")
		}
		if len([]rune(value)) > 200 {
			return nil, errors.New("每项档案请控制在200字以内")
		}
		if strings.HasSuffix(key, "_score") && value != "" {
			score, err := strconv.ParseFloat(value, 64)
			if err != nil || math.IsNaN(score) || math.IsInf(score, 0) || score < 0 || score > 100 {
				return nil, errors.New("主观评分请填写0–100数字")
			}
		}
		cleaned[key] = value
	}
	return cleaned, nil
}

func (s *Service) applyProfile(raw Data, persist bool) error {
	if s.Current == nil {
		return errors.New("先开启采集并选择单聊，再为当前对象建档")
	}
	cleaned, err := cleanProfile(raw)
	if err != nil {
		return err
	}
	if persist {
		status, err := s.Memory.Status()
		if err != nil {
			return err
		}
		if status["consent_enabled"] != true || status["paused"] == true {
			return errors.New("请先明确启用记忆；暂停时不能保存新档案")
		}
		rows, err := s.Memory.Rows(s.subject())
		if err != nil {
			return err
		}
		existing := map[profileField]interface{}{}
		for _, row := range rows {
			existing[profileField{row["scope"].(string), row["field"].(string)}] = row["value"]
		}
		for key, value := range cleaned {
			field := profileFields[key]
			if value != "" && existing[field] != value {
				subject := s.subject()
				if field.scope == "user" {
					subject = "user"
				}
				if _, err = s.Memory.Apply(memory.Row{"scope": field.scope, "subject_id": subject, "field": field.field, "value": value, "source_type": "user_explicit", "source_ref": "建档表", "confidence": "high"}); err != nil {
					return err
				}
			}
		}
		s.MemoryNotice = "已保存明确填写的档案。可查看、暂停、撤销或删除。"
	}
	s.Profile = cleaned
	s.profiles[s.subject()] = copyProfile(cleaned)
	s.Current.Revision++
	s.Onboarded = true
	s.invalidate()
	return nil
}

func (s *Service) memoryOperation(payload Data) error {
	operation := str(payload, "operation")
	var err error
	switch operation {
	case "enable":
		err = s.Memory.Enable(truth(payload, "consent"))
	case "pause":
		err = s.Memory.Pause(true)
	case "resume":
		err = s.Memory.Pause(false)
	case "undo":
		_, err = s.Memory.Undo("")
		if err == nil {
			delete(s.profiles, s.subject())
			s.Profile = map[string]string{}
			s.loadProfile(false)
		}
	case "forget":
		if s.Current == nil {
			return errors.New("请确认要忘记当前对象")
		}
		err = s.Memory.Forget(s.subject(), truth(payload, "confirm"))
		if err == nil {
			delete(s.profiles, s.subject())
			for key := range s.Profile {
				if !strings.HasPrefix(key, "user_") {
					delete(s.Profile, key)
				}
			}
			s.Onboarded = false
		}
	case "revoke", "delete_all":
		err = s.Memory.Revoke(truth(payload, "confirm"), operation == "delete_all")
		if err == nil {
			s.profiles = map[string]map[string]string{}
			s.Profile = map[string]string{}
			s.Onboarded = false
		}
	default:
		return errors.New("不支持的记忆操作")
	}
	if err != nil {
		return err
	}
	s.invalidate()
	s.MemoryNotice = map[string]string{"enable": "已启用本地记忆", "pause": "已暂停记忆读写", "resume": "已恢复记忆", "undo": "已撤销最近一次字段更新", "forget": "已忘记当前对象", "revoke": "已撤回同意，资料保留但不用于分析", "delete_all": "已撤回同意并删除全部关系记忆"}[operation]
	return nil
}
