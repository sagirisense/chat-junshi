package service

import (
	"context"
	"encoding/json"
	"junshi-desktop/internal/codexclient"
	"junshi-desktop/internal/evidence"
	"testing"
	"time"
)

func syntheticAnalysis() Data {
	var result Data
	json.Unmarshal([]byte(`{"summary":"虚构建议","facts":[],"hypotheses":[],"unknowns":[],"recommended_index":0,"recommendation_reason":"测试","candidates":[{"text":"你好","pros":[],"cons":[],"suitable_when":"测试","next_step":"等待"}]}`), &result)
	return result
}

func analysisService(t *testing.T) (*Service, chan Data) {
	t.Helper()
	events := make(chan Data, 64)
	s, err := New(t.TempDir(), func(state Data) { events <- state })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	call(t, s, "bind_profile", Data{"title": "虚构对象A"})
	if err = s.Current.Append([]evidence.CapturedLine{{Speaker: "her", Text: "你好"}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	call(t, s, "confirm", Data{"confirmed": true})
	return s, events
}

func awaitFinished(t *testing.T, events chan Data) Data {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case state := <-events:
			if state["busy"] == false && (state["status"] == "建议已准备好，选择适合你的表达" || state["status"] == "会话或上下文已变化，旧建议已丢弃，请重新分析") {
				return state
			}
		case <-deadline.C:
			t.Fatal("generation did not complete")
			return nil
		}
	}
}

func TestAsyncGenerationAndStaleResults(t *testing.T) {
	s, events := analysisService(t)
	started := make(chan struct{})
	release := make(chan struct{})
	s.Generate = func(ctx context.Context, input Generation, progress codexclient.Progress) (Data, error) {
		close(started)
		<-release
		return syntheticAnalysis(), nil
	}
	call(t, s, "analyze", Data{})
	<-started
	if _, err := s.Dispatch("analyze", Data{}); err == nil {
		t.Fatal("accepted overlapping generation")
	}
	call(t, s, "bind_profile", Data{"title": "虚构对象B"})
	close(release)
	state := awaitFinished(t, events)
	if state["result"] != nil || state["title"] != "虚构对象B" {
		t.Fatal("stale result leaked", state)
	}
}

func TestGenerationMemoryWritesOnlyValidatedEvidence(t *testing.T) {
	s, events := analysisService(t)
	call(t, s, "memory", Data{"operation": "enable", "consent": true})
	s.Generate = func(ctx context.Context, input Generation, progress codexclient.Progress) (Data, error) {
		if !input.WithMemory {
			t.Error("memory not enabled for reply")
		}
		result := syntheticAnalysis()
		result["memory_updates"] = []interface{}{Data{"scope": "event", "field": "key_event", "value": "有效虚构事件", "evidence_ids": []interface{}{float64(1)}, "confidence": "high"}, Data{"scope": "event", "field": "key_event", "value": "无效事件", "evidence_ids": []interface{}{float64(999)}, "confidence": "high"}}
		return result, nil
	}
	call(t, s, "analyze", Data{})
	state := awaitFinished(t, events)
	if state["result"] == nil {
		t.Fatal(state)
	}
	rows, err := s.Memory.Rows(s.subject())
	if err != nil || len(rows) != 1 || rows[0]["value"] != "有效虚构事件" {
		t.Fatal(rows, err)
	}
}

func TestInvalidationCancelsGeneration(t *testing.T) {
	s, events := analysisService(t)
	cancelled := make(chan struct{})
	s.Generate = func(ctx context.Context, input Generation, progress codexclient.Progress) (Data, error) {
		<-ctx.Done()
		close(cancelled)
		return nil, ctx.Err()
	}
	call(t, s, "analyze", Data{})
	call(t, s, "goal", Data{"goal": "调整本轮目标"})
	select {
	case <-cancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("task not cancelled")
	}
	state := awaitFinished(t, events)
	if state["result"] != nil {
		t.Fatal(state)
	}
}
