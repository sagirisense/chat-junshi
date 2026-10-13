package service

import (
	"encoding/json"
	"junshi-desktop/internal/evidence"
	"testing"
	"time"
)

func call(t *testing.T, s *Service, action string, payload Data) Data {
	t.Helper()
	result, err := s.Dispatch(action, payload)
	if err != nil {
		t.Fatal(action, err)
	}
	return result.(map[string]interface{})
}

func TestSessionStateAndIdentity(t *testing.T) {
	root := t.TempDir()
	events := 0
	s, err := New(root, func(Data) { events++ })
	if err != nil {
		t.Fatal(err)
	}
	state := call(t, s, "bind_profile", Data{"title": "虚构对象A"})
	if state["confirmed"] != false || state["title"] != "虚构对象A" {
		t.Fatal(state)
	}
	if err = s.Current.Append([]evidence.CapturedLine{{Speaker: "her", Text: "测试文字"}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Dispatch("packet", Data{}); err == nil {
		t.Fatal("packet bypassed identity")
	}
	call(t, s, "confirm", Data{"confirmed": true})
	call(t, s, "packet", Data{})
	call(t, s, "correct", Data{"id": 1, "sender": "peer", "text": "修正文字", "time_edit": true, "message_time": "2026-10-12T20:30"})
	if s.Current.Messages[0]["text"] != "修正文字" {
		t.Fatal(s.Current.Messages)
	}
	call(t, s, "bind_profile", Data{"title": "虚构对象B"})
	if len(s.Current.Messages) != 0 {
		t.Fatal("cross-object messages")
	}
	call(t, s, "select", Data{"title": "虚构对象A"})
	if s.Current.Confirmed {
		t.Fatal("selection reused identity")
	}
	restored, err := New(root, nil)
	if err != nil || restored.Current.Title != "虚构对象A" || restored.Current.Confirmed {
		t.Fatal(restored, err)
	}
	if events < 5 {
		t.Fatal("missing state notifications")
	}
}

func TestProfileConsentAndObjectIsolation(t *testing.T) {
	s, err := New(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	call(t, s, "bind_profile", Data{"title": "虚构对象A"})
	if _, err = s.Dispatch("profile", Data{"persist": true, "profile": Data{"peer_label": "A"}}); err == nil {
		t.Fatal("saved without consent")
	}
	call(t, s, "memory", Data{"operation": "enable", "consent": true})
	call(t, s, "profile", Data{"persist": true, "profile": Data{"peer_label": "A", "user_mbti": "未知"}})
	call(t, s, "bind_profile", Data{"title": "虚构对象B"})
	if s.Profile["peer_label"] != "" || s.Profile["user_mbti"] != "未知" {
		t.Fatal(s.Profile)
	}
	call(t, s, "select", Data{"title": "虚构对象A"})
	if s.Profile["peer_label"] != "A" {
		t.Fatal(s.Profile)
	}
	call(t, s, "memory", Data{"operation": "forget", "confirm": true})
	if s.Profile["peer_label"] != "" || s.Profile["user_mbti"] != "未知" {
		t.Fatal(s.Profile)
	}
	if _, err = s.Dispatch("profile", Data{"profile": Data{"user_score": "NaN"}}); err == nil {
		t.Fatal("accepted invalid score")
	}
}

func TestGenderProfilePersistenceAndImport(t *testing.T) {
	root := t.TempDir()
	s, err := New(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	call(t, s, "bind_profile", Data{"title": "性别测试对象"})
	call(t, s, "profile", Data{"profile": Data{"user_gender": "男", "peer_gender": "自定义描述"}, "persist": false})
	rows, err := s.Memory.Rows(s.subject())
	if err != nil || len(rows) != 0 {
		t.Fatal("temporary profile persisted", rows, err)
	}
	call(t, s, "memory", Data{"operation": "enable", "consent": true})
	call(t, s, "profile", Data{"profile": Data{"user_gender": "男", "peer_gender": "自定义描述"}, "persist": true})
	exported := call(t, s, "export_profile", Data{})
	draft := call(t, s, "prepare_import_profile", Data{"text": exported["text"]})
	if draft["profile"].(map[string]string)["peer_gender"] != "自定义描述" {
		t.Fatal(draft)
	}
	restored, err := New(root, nil)
	if err != nil || restored.Profile["user_gender"] != "男" || restored.Profile["peer_gender"] != "自定义描述" {
		t.Fatal(restored, err)
	}
	call(t, restored, "bind_profile", Data{"title": "其他测试对象"})
	if restored.Profile["peer_gender"] != "" || restored.Profile["user_gender"] != "男" {
		t.Fatal("gender leaked across objects", restored.Profile)
	}
}

func TestSnapshotIsolationAndDeletion(t *testing.T) {
	s, err := New(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	call(t, s, "bind_profile", Data{"title": "虚构对象"})
	state := call(t, s, "state", Data{})
	state["profile"].(map[string]interface{})["goal"] = "external mutation"
	if s.Profile["goal"] != "" {
		t.Fatal("snapshot aliases internal state")
	}
	if _, err = s.Dispatch("delete_session", Data{}); err == nil {
		t.Fatal("deleted without confirmation")
	}
	call(t, s, "delete_session", Data{"confirm": true})
	if s.Current != nil || len(s.Sessions) != 0 {
		t.Fatal("session retained")
	}
}

func TestImportAppendAndValidation(t *testing.T) {
	s, err := New(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	data := Data{"version": 1, "source": "wechat-window-ocr", "title": "虚构对象", "messages": []Data{{"id": 99, "sender": "peer", "text": "你好", "message_time": nil}}}
	encoded, _ := json.Marshal(data)
	preview := call(t, s, "prepare_import_chat", Data{"text": string(encoded)})
	if preview["count"] != 1 || len(s.Sessions) != 0 {
		t.Fatal(preview)
	}
	if _, err = s.Dispatch("import_chat", Data{"text": string(encoded)}); err == nil {
		t.Fatal("imported without confirmation")
	}
	for i := 0; i < 2; i++ {
		call(t, s, "import_chat", Data{"text": string(encoded), "confirm": true})
	}
	if len(s.Current.Messages) != 2 || number(s.Current.Messages[0], "id") != 1 || number(s.Current.Messages[1], "id") != 2 || s.Current.Confirmed {
		t.Fatal(s.Current.Messages)
	}
	call(t, s, "delete_message", Data{"title": "虚构对象", "id": 2, "confirm": true})
	call(t, s, "import_chat", Data{"text": string(encoded), "confirm": true})
	if number(s.Current.Messages[1], "id") != 3 {
		t.Fatal("import reused deleted ID")
	}
	if _, err = s.Dispatch("prepare_import_chat", Data{"text": "not json"}); err == nil {
		t.Fatal("accepted invalid import")
	}
}

func TestCapturePauseAndBlockedIdentity(t *testing.T) {
	s, err := New(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	event := CaptureEvent{Kind: "lines", Title: "虚构对象", Lines: []evidence.CapturedLine{{Speaker: "her", Text: "你好"}}}
	if err = s.HandleCapture(event); err != nil || s.Current != nil {
		t.Fatal("paused capture changed state", err)
	}
	s.Capturing = true
	if err = s.HandleCapture(CaptureEvent{Kind: "chat", Title: "虚构对象"}); err != nil {
		t.Fatal(err)
	}
	if err = s.HandleCapture(event); err != nil {
		t.Fatal(err)
	}
	call(t, s, "confirm", Data{"confirmed": true})
	if err = s.HandleCapture(CaptureEvent{Kind: "blocked", Status: "请打开单聊"}); err != nil {
		t.Fatal(err)
	}
	if s.Current.Confirmed || !s.Blocked {
		t.Fatal("blocked capture retained identity")
	}
	if err = s.HandleCapture(event); err != nil || len(s.Current.Messages) != 1 {
		t.Fatal("accepted blocked messages", err)
	}
}
