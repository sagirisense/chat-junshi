package evidence

import (
	"testing"
	"time"
)

func TestLabels(t *testing.T) {
	now := time.Date(2026, 10, 12, 23, 30, 0, 0, China)
	for label, want := range map[string]string{"昨天 18:30": "2026-10-11T18:30+08:00", "星期五 下午3:05": "2026-10-09T15:05+08:00", "周日 09:00": "2026-10-11T09:00+08:00", "2025年12月31日 23:59": "2025-12-31T23:59+08:00", "12月31日": "2025-12-31T00:00+08:00", "00：15": "2026-10-12T00:15+08:00"} {
		if got := ParseLabel(label, now); got == nil || got.AnchorTime != want {
			t.Errorf("%s: %#v, want %s", label, got, want)
		}
	}
	for _, label := range []string{"明天再说", "昨天18:30吃饭了吗", "25:80", "2月30日 12:00", "半小时前"} {
		if ParseLabel(label, now) != nil {
			t.Errorf("accepted %s", label)
		}
	}
}

func TestManualTime(t *testing.T) {
	for _, value := range []string{"2026-10-11T18:31", "2026-10-11T10:31:20Z"} {
		got, err := NormalizeManual(value)
		if err != nil || *got != "2026-10-11T18:31+08:00" {
			t.Fatalf("%s: %v %v", value, got, err)
		}
	}
	for _, value := range []string{"not a date", "2026-10-11", "2026-02-30T12:00"} {
		if _, err := NormalizeManual(value); err == nil {
			t.Errorf("accepted %s", value)
		}
	}
}

func TestInference(t *testing.T) {
	rows := []Message{{"id": 1, "message_time": "2026-10-12T22:37+08:00"}, {"id": 2, "message_time": nil}, {"id": 3, "message_time": "2026-10-12T22:45+08:00"}}
	got := InferMissingTimes(rows)
	if got[1]["inferred_time"].(map[string]interface{})["precision"] != "range" || got[1]["message_time"] != nil {
		t.Fatal(got)
	}
	if _, ok := rows[1]["inferred_time"]; ok {
		t.Fatal("mutated source")
	}
	rows[0]["message_time"] = "2026-10-12T23:00+08:00"
	if _, ok := InferMissingTimes(rows)[1]["inferred_time"]; ok {
		t.Fatal("inferred contradictory neighbors")
	}
	rows[0]["message_time"] = "2026-10-12T22:00+08:00"
	rows[1]["time_inference_disabled"] = true
	if _, ok := InferMissingTimes(rows)[1]["inferred_time"]; ok {
		t.Fatal("inferred explicit unknown")
	}
}
