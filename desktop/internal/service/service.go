package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"junshi-desktop/internal/capture"
	"junshi-desktop/internal/codexclient"
	"junshi-desktop/internal/evidence"
	"junshi-desktop/internal/memory"
	"junshi-desktop/internal/news"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Data = map[string]interface{}
type profileField struct{ scope, field string }

var profileFields = map[string]profileField{
	"user_mbti": {"user", "mbti"}, "user_score": {"user", "subjective_score"}, "user_strengths": {"user", "strengths"}, "user_boundaries": {"user", "boundaries"},
	"peer_label": {"object", "display_label"}, "peer_mbti": {"object", "mbti"}, "peer_score": {"object", "subjective_score"}, "relationship": {"relationship", "stage"}, "history": {"relationship", "history"}, "goal": {"relationship", "goal"}, "emotion": {"relationship", "emotion"},
}

type Service struct {
	mu                                                  sync.Mutex
	Root                                                string
	Memory                                              *memory.Store
	Sessions                                            map[string]*evidence.Session
	order                                               []string
	Current                                             *evidence.Session
	Profile                                             map[string]string
	profiles                                            map[string]map[string]string
	ActiveTitle, Status, MemoryNotice, AssistanceNotice string
	Capturing, Blocked, Busy, OCRReady, Onboarded       bool
	Result                                              Data
	Preview                                             []interface{}
	Metrics                                             Data
	epoch                                               int
	Emit                                                func(Data)
	Generate                                            func(context.Context, Generation, codexclient.Progress) (Data, error)
	News                                                func(context.Context) ([]Data, string)
	transport                                           codexclient.Transport
	cancelGeneration                                    context.CancelFunc
	closed                                              bool
	collector                                           *capture.Collector
	SkillRoot                                           string
}

func New(root string, emit func(Data)) (*Service, error) {
	s := &Service{Root: root, Memory: memory.New(filepath.Join(root, "data", "relationship-memory")), Sessions: map[string]*evidence.Session{}, Profile: map[string]string{}, profiles: map[string]map[string]string{}, Status: "采集已暂停，打开微信单聊后开始", Preview: []interface{}{}, Metrics: Data{}, Emit: emit}
	reader := &news.Reader{}
	s.News = reader.Recent
	paths, err := filepath.Glob(filepath.Join(root, "data", "*.json"))
	if err != nil {
		return nil, err
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var payload Data
		if json.Unmarshal(data, &payload) != nil {
			continue
		}
		title, _ := payload["title"].(string)
		if title == "" {
			continue
		}
		session, err := evidence.OpenSession(title, filepath.Join(root, "data"))
		if err != nil {
			continue
		}
		s.Sessions[title] = session
		s.order = append(s.order, title)
	}
	if data, err := os.ReadFile(filepath.Join(root, "data", "selected-session.json")); err == nil {
		var selected Data
		if json.Unmarshal(data, &selected) == nil {
			title, _ := selected["selected_title"].(string)
			if s.Sessions[title] != nil {
				if err = s.selectSession(title); err != nil {
					return nil, err
				}
			}
		}
	}
	if s.Current == nil {
		s.loadProfile(true)
	}
	return s, nil
}

func (s *Service) subject() string {
	title := "__unbound__"
	if s.Current != nil {
		title = s.Current.Title
	}
	digest := sha256.Sum256([]byte(title))
	return fmt.Sprintf("obj-%x", digest[:8])
}
func copyProfile(profile map[string]string) map[string]string {
	result := map[string]string{}
	for k, v := range profile {
		result[k] = v
	}
	return result
}

func (s *Service) loadProfile(userOnly bool) {
	rows, err := s.Memory.Rows(s.subject())
	if err != nil {
		s.MemoryNotice = "关系记忆暂不可用，当前仅使用本次填写的信息。"
		return
	}
	for _, row := range rows {
		for key, field := range profileFields {
			if row["scope"] == field.scope && row["field"] == field.field && (!userOnly || field.scope == "user") {
				s.Profile[key], _ = row["value"].(string)
			}
		}
	}
}

func (s *Service) selectSession(title string) error {
	if s.Current != nil && s.Current.Title == title {
		return nil
	}
	if s.Sessions[title] == nil {
		session, err := evidence.OpenSession(title, filepath.Join(s.Root, "data"))
		if err != nil {
			return err
		}
		s.Sessions[title] = session
		s.order = append(s.order, title)
	}
	selected := filepath.Join(s.Root, "data", "selected-session.json")
	if err := os.MkdirAll(filepath.Dir(selected), 0700); err != nil {
		return err
	}
	data, _ := json.Marshal(Data{"selected_title": title})
	if err := os.WriteFile(selected, data, 0600); err != nil {
		return err
	}
	s.Current = s.Sessions[title]
	s.Current.Confirmed = false
	s.invalidate()
	s.Profile = copyProfile(s.profiles[s.subject()])
	s.loadProfile(false)
	if s.Profile["relationship"] == "" {
		s.Profile["relationship"] = s.Current.Background
	}
	if s.Profile["goal"] == "" {
		s.Profile["goal"] = s.Current.Goal
	}
	s.Onboarded = len(s.profiles[s.subject()]) > 0
	if rows, err := s.Memory.Rows(s.subject()); err == nil {
		for _, row := range rows {
			if row["subject_id"] == s.subject() && (row["scope"] == "object" || row["scope"] == "relationship") {
				s.Onboarded = true
			}
		}
	}
	return nil
}

func (s *Service) invalidate() {
	if s.cancelGeneration != nil {
		s.cancelGeneration()
	}
	s.epoch++
	s.Result = nil
	s.Preview = []interface{}{}
}
func (s *Service) snapshot() Data {
	status, err := s.Memory.Status()
	rows, rowErr := s.Memory.Rows(s.subject())
	if err != nil || rowErr != nil {
		status = memory.Row{"consent_enabled": false, "paused": true, "unavailable": true}
		rows = []memory.Row{}
		s.MemoryNotice = "关系记忆暂不可用，采集仍可继续；请稍后重试记忆操作。"
	}
	title := ""
	confirmed := false
	messages := []evidence.Message{}
	if s.Current != nil {
		title = s.Current.Title
		confirmed = s.Current.Confirmed
		messages = evidence.InferMissingTimes(s.Current.Messages)
		if len(messages) > 150 {
			messages = messages[len(messages)-150:]
		}
	}
	result := Data{"title": title, "active_title": s.ActiveTitle, "messages": messages, "sessions": append([]string{}, s.order...), "confirmed": confirmed, "capturing": s.Capturing, "blocked": s.Blocked, "busy": s.Busy, "status": s.Status, "result": s.Result, "profile": s.Profile, "reply_preview": s.Preview, "reply_metrics": s.Metrics, "assistance_notice": s.AssistanceNotice, "ocr_ready": s.OCRReady, "onboarded": s.Onboarded, "memory": status, "memory_rows": rows, "memory_notice": s.MemoryNotice}
	// Snapshot recipients must never retain maps that a subsequent task mutates.
	encoded, _ := json.Marshal(result)
	var clone Data
	_ = json.Unmarshal(encoded, &clone)
	return clone
}

func str(payload Data, key string) string { value, _ := payload[key].(string); return value }
func truth(payload Data, key string) bool { return payload[key] == true }
func number(payload Data, key string) int {
	switch value := payload[key].(type) {
	case int:
		return value
	case float64:
		return int(value)
	case json.Number:
		n, _ := value.Int64()
		return int(n)
	}
	return 0
}
func trim(value string) string { return strings.TrimSpace(value) }
