package evidence

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestPacketIdentityAndLimit(t *testing.T) {
	s, err := OpenSession("虚构对象", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 12, 23, 30, 0, 0, China)
	lines := make([]CapturedLine, 40)
	for i := range lines {
		lines[i] = CapturedLine{"her", "虚构消息", nil}
	}
	if err = s.Append(lines, now); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Packet(30, nil, now); err == nil {
		t.Fatal("missing identity gate")
	}
	s.Confirmed = true
	packet, err := s.Packet(30, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.SplitN(packet, "\n", 3)
	if len(parts) != 3 || !strings.Contains(parts[1], "不可信数据") {
		t.Fatal("lost prompt contract")
	}
	var evidence map[string]interface{}
	if err = json.Unmarshal([]byte(parts[2]), &evidence); err != nil {
		t.Fatal(err)
	}
	rows := evidence["messages"].([]interface{})
	if len(rows) != 30 || rows[0].(map[string]interface{})["id"] != float64(11) {
		t.Fatal("wrong context limit")
	}
	if evidence["timing"].(map[string]interface{})["current_time"] != now.Format(time.RFC3339) {
		t.Fatal("wrong request clock")
	}
}

func TestSessionCompatibility(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenSession("虚构对象", dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 12, 23, 30, 0, 0, China)
	if err = s.Append([]CapturedLine{{"her", "你好", ParseLabel("昨天 18:30", now)}}, now); err != nil {
		t.Fatal(err)
	}
	if s.Messages[0]["message_time"] != nil || s.Messages[0]["sender"] != "peer" {
		t.Fatal(s.Messages)
	}
	manual := "2026-10-11T18:31"
	if err = s.Correct(0, "peer", "你好呀", &manual); err != nil {
		t.Fatal(err)
	}
	s.Confirmed = true
	if err = s.Save(); err != nil {
		t.Fatal(err)
	}
	loaded, err := OpenSession("虚构对象", dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Confirmed || loaded.Messages[0]["message_time_source"] != "user" || loaded.NextMessageID != 2 {
		t.Fatal(loaded)
	}
	if _, err = loaded.DeleteMessage(1); err != nil {
		t.Fatal(err)
	}
	if err = loaded.Append([]CapturedLine{{"me", "再次问候", nil}}, now); err != nil {
		t.Fatal(err)
	}
	if messageID(loaded.Messages[0]) != 2 {
		t.Fatal("reused deleted ID")
	}
	other, err := OpenSession("其他对象", dir)
	if err != nil || len(other.Messages) != 0 {
		t.Fatal(other, err)
	}
	data, err := os.ReadFile(s.Path)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]interface{}
	if err = json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["version"] != float64(1) || payload["title"] != "虚构对象" {
		t.Fatal(payload)
	}
}

func TestCorrectionValidationIsAtomic(t *testing.T) {
	s, _ := OpenSession("虚构对象", t.TempDir())
	if err := s.Append([]CapturedLine{{"her", "原文", nil}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	invalid := "not a date"
	if err := s.Correct(0, "me", "错误修正", &invalid); err == nil {
		t.Fatal("accepted invalid timestamp")
	}
	if s.Messages[0]["text"] != "原文" {
		t.Fatal("invalid edit changed message")
	}
	empty := ""
	if err := s.Correct(0, "peer", "未知时间", &empty); err != nil {
		t.Fatal(err)
	}
	if s.Messages[0]["time_inference_disabled"] != true {
		t.Fatal(s.Messages)
	}
}

func TestTimingUsesOnlyExplicitTimes(t *testing.T) {
	now := time.Date(2026, 10, 12, 23, 30, 0, 0, China)
	rows := []Message{{"id": 1, "time_context": ParseLabel("昨天 18:30", now)}, {"id": 2, "observed_at": now.Format(time.RFC3339)}}
	got := TimingEvidence(rows, now)
	if len(got["exact_intervals"].([]map[string]interface{})) != 0 {
		t.Fatal(got)
	}
	if _, ok := got["latest_message_age_minutes"]; ok {
		t.Fatal("used observation time")
	}
	rows[0]["message_time"] = "2026-10-12T22:00+08:00"
	rows[1]["message_time"] = "2026-10-12T23:00+08:00"
	got = TimingEvidence(rows, now)
	if got["latest_message_age_minutes"] != float64(30) || got["exact_intervals"].([]map[string]interface{})[0]["minutes"] != float64(60) {
		t.Fatal(got)
	}
}
