package evidence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Session struct {
	Title         string
	Path          string
	Messages      []Message
	Confirmed     bool
	Background    string
	Goal          string
	Revision      int
	NextMessageID int
}

type sessionPayload struct {
	Version       int       `json:"version"`
	Title         string    `json:"title"`
	Source        string    `json:"source"`
	Confirmed     bool      `json:"identity_confirmed"`
	Background    string    `json:"background"`
	Goal          string    `json:"goal"`
	Messages      []Message `json:"messages"`
	NextMessageID int       `json:"next_message_id"`
}

func messageID(row Message) int {
	switch v := row["id"].(type) {
	case int:
		return v
	case float64:
		return int(v)
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	}
	return 0
}

func OpenSession(title, directory string) (*Session, error) {
	digest := sha256.Sum256([]byte(title))
	s := &Session{Title: title, Path: filepath.Join(directory, hex.EncodeToString(digest[:])+".json"), Messages: []Message{}, NextMessageID: 1}
	data, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	var saved sessionPayload
	if err = json.Unmarshal(data, &saved); err != nil {
		return nil, err
	}
	s.Messages = saved.Messages
	if s.Messages == nil {
		s.Messages = []Message{}
	}
	s.Background = saved.Background
	s.Goal = saved.Goal
	s.NextMessageID = saved.NextMessageID
	s.ensureNextID()
	// Every launch requires confirmation against the current captured window.
	return s, nil
}

func (s *Session) ensureNextID() {
	if s.NextMessageID < 1 {
		s.NextMessageID = 1
	}
	for _, row := range s.Messages {
		if id := messageID(row); id >= s.NextMessageID {
			s.NextMessageID = id + 1
		}
	}
}

func (s *Session) Payload() interface{} {
	return sessionPayload{1, s.Title, "wechat-window-ocr", s.Confirmed, s.Background, s.Goal, InferMissingTimes(s.Messages), s.NextMessageID}
}

func (s *Session) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.Payload(), "", "  ")
	if err != nil {
		return err
	}
	temporary := strings.TrimSuffix(s.Path, filepath.Ext(s.Path)) + ".tmp"
	if err = os.WriteFile(temporary, data, 0600); err != nil {
		return err
	}
	for _, delay := range []time.Duration{40 * time.Millisecond, 120 * time.Millisecond, 250 * time.Millisecond, 0} {
		err = os.Rename(temporary, s.Path)
		if err == nil {
			return nil
		}
		if !retryableReplace(err) || delay == 0 {
			return err
		}
		time.Sleep(delay)
	}
	return err
}

type CapturedLine struct {
	Speaker, Text string
	Context       *TimeContext
}

func (s *Session) Append(lines []CapturedLine, now time.Time) error {
	for _, line := range lines {
		if line.Speaker != "me" && line.Speaker != "her" {
			return errors.New("说话人未识别")
		}
	}
	s.ensureNextID()
	for _, line := range lines {
		sender := "peer"
		if line.Speaker == "me" {
			sender = "me"
		}
		row := Message{"id": s.NextMessageID, "sender": sender, "text": line.Text, "message_time": nil, "observed_at": now.UTC().Format("2006-01-02T15:04:05.000000Z07:00")}
		if c := CleanContext(line.Context); c != nil {
			row["time_context"] = c
		}
		s.Messages = append(s.Messages, row)
		s.NextMessageID++
	}
	s.Revision++
	return s.Save()
}

func (s *Session) DeleteMessage(id int) (Message, error) {
	for i, row := range s.Messages {
		if messageID(row) == id {
			s.Messages = append(s.Messages[:i], s.Messages[i+1:]...)
			s.Revision++
			return row, s.Save()
		}
	}
	return nil, errors.New("消息已不存在")
}

// A nil manualTime preserves the current timestamp; a pointer to empty clears it.
func (s *Session) Correct(index int, sender, text string, manualTime *string) error {
	if (sender != "me" && sender != "peer") || strings.TrimSpace(text) == "" {
		return errors.New("请选择说话人并填写正文")
	}
	if index < 0 || index >= len(s.Messages) {
		return errors.New("消息已不存在")
	}
	var timestamp *string
	var err error
	if manualTime != nil {
		timestamp, err = NormalizeManual(*manualTime)
		if err != nil {
			return err
		}
	}
	row := s.Messages[index]
	row["sender"] = sender
	row["text"] = strings.TrimSpace(text)
	if manualTime != nil {
		row["message_time"] = nil
		row["message_time_source"] = nil
		row["time_inference_disabled"] = timestamp == nil
		if timestamp != nil {
			row["message_time"] = *timestamp
			row["message_time_source"] = "user"
		}
		delete(row, "time_context")
	}
	s.Revision++
	return s.Save()
}
