package codexclient

import (
	"context"
	"encoding/json"
	"errors"
	"junshi-desktop/internal/modelapi"
	"os"
	"strings"
	"sync"
	"time"
)

type Progress func(stage string, metrics Message, rows []interface{})
type AnalyzeRequest struct {
	Packet, Model, Task, SkillPath, Directory string
	ImageURL                                  string
	WithMemory, Quick                         bool
}

// Transport reuses only the CLI connection; each request starts a fresh ephemeral thread.
type Transport struct {
	mu     sync.Mutex
	server *Server
	models []Message
	Start  func() (*Server, error)
}

func (t *Transport) reset() {
	if t.server != nil {
		t.server.Close()
	}
	t.server = nil
	t.models = nil
}
func (t *Transport) Close() { t.mu.Lock(); defer t.mu.Unlock(); t.reset() }

func nestedID(value interface{}) string {
	row, _ := value.(map[string]interface{})
	id, _ := row["id"].(string)
	return id
}

func PreviewCandidates(text, task string) []interface{} {
	rows := []interface{}{}
	start := strings.Index(text, `"candidates"`)
	if start < 0 {
		return rows
	}
	offset := strings.Index(text[start:], "[")
	if offset < 0 {
		return rows
	}
	start += offset
	rest := strings.TrimSpace(text[start+1:])
	for len(rows) < 3 && strings.HasPrefix(rest, "{") {
		decoder := json.NewDecoder(strings.NewReader(rest))
		decoder.UseNumber()
		var row map[string]interface{}
		if decoder.Decode(&row) != nil {
			break
		}
		probe := map[string]interface{}{"summary": "", "facts": []interface{}{}, "hypotheses": []interface{}{}, "unknowns": []interface{}{}, "recommended_index": 0, "recommendation_reason": "", "candidates": []interface{}{row}}
		if modelapi.ValidateResult(probe, task) != nil {
			break
		}
		rows = append(rows, row)
		rest = strings.TrimSpace(rest[decoder.InputOffset():])
		if !strings.HasPrefix(rest, ",") {
			break
		}
		rest = strings.TrimSpace(rest[1:])
	}
	return rows
}

func (t *Transport) Analyze(ctx context.Context, input AnalyzeRequest, progress Progress) (result map[string]interface{}, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()
	started := time.Now()
	task := input.Task
	if task == "" {
		task = "reply"
	}
	metrics := Message{"tool_calls": 0, "reused": t.server != nil}
	report := func(stage string, rows []interface{}) {
		metrics["elapsed_s"] = float64(time.Since(started).Milliseconds()) / 1000
		if progress != nil {
			copy := Message{}
			for k, v := range metrics {
				copy[k] = v
			}
			progress(stage, copy, rows)
		}
	}
	defer func() {
		if err != nil {
			report("生成失败", nil)
			t.reset()
		}
		if !input.Quick {
			t.reset()
		}
	}()
	if !input.Quick {
		if info, e := os.Stat(input.SkillPath); e != nil || info.IsDir() {
			return nil, errors.New("未找到本地 goutoujunshi/SKILL.md")
		}
		t.reset()
	}
	report("正在连接模型", nil)
	if t.server == nil {
		start := t.Start
		if start == nil {
			start = Start
		}
		t.server, err = start()
		if err != nil {
			return nil, err
		}
		metrics["startup_s"] = float64(time.Since(started).Milliseconds()) / 1000
		if err = t.server.Initialize(ctx, "junshi_reply"); err != nil {
			return nil, err
		}
	}
	if t.models == nil {
		t.models, err = t.server.Models(ctx)
		if err != nil {
			return nil, err
		}
	}
	model := input.Model
	if model == "" {
		for _, row := range t.models {
			if row["isDefault"] == true {
				model, _ = row["model"].(string)
				break
			}
		}
		if model == "" {
			return nil, errors.New("请选择可用模型")
		}
	}
	metrics["initialize_s"] = float64(time.Since(started).Milliseconds()) / 1000
	params := Message{"model": model, "sandbox": "read-only", "approvalPolicy": "never", "ephemeral": true}
	if input.Quick {
		params["baseInstructions"] = modelapi.QuickRules
		params["developerInstructions"] = "只完成本轮回复，不调用工具。"
		if task == "sticker" {
			params["baseInstructions"] = modelapi.StickerRules
			params["developerInstructions"] = "只分析本轮表情图片，不调用工具。"
		}
	} else {
		params["cwd"] = input.Directory
	}
	response, err := t.server.Request(ctx, "thread/start", params)
	if err != nil {
		return nil, err
	}
	thread := nestedID(response["thread"])
	if thread == "" {
		return nil, errors.New("Codex 会话格式无效")
	}
	metrics["thread_ready_s"] = float64(time.Since(started).Milliseconds()) / 1000
	packet := input.Packet
	if input.Quick {
		packet = strings.Replace(packet, "$goutoujunshi", "即时回复任务", 1)
	}
	items := []Message{{"type": "text", "text": packet}}
	if input.ImageURL != "" {
		items = append(items, Message{"type": "image", "url": input.ImageURL})
	}
	if !input.Quick {
		items = append(items, Message{"type": "skill", "name": "goutoujunshi", "path": input.SkillPath})
	}
	params = Message{"threadId": thread, "input": items, "outputSchema": modelapi.Schema(input.WithMemory, input.Quick, task)}
	if input.Quick {
		for _, row := range t.models {
			if row["model"] != model {
				continue
			}
			efforts, _ := row["supportedReasoningEfforts"].([]interface{})
			for _, raw := range efforts {
				effort, _ := raw.(map[string]interface{})
				if effort["reasoningEffort"] == "low" {
					params["effort"] = "low"
				}
			}
			if params["effort"] == nil && row["defaultReasoningEffort"] != nil {
				params["effort"] = row["defaultReasoningEffort"]
			}
		}
	}
	metrics["effort"] = "default"
	if params["effort"] != nil {
		metrics["effort"] = params["effort"]
	}
	response, err = t.server.Request(ctx, "turn/start", params)
	if err != nil {
		return nil, err
	}
	turn := nestedID(response["turn"])
	if turn == "" {
		return nil, errors.New("Codex 生成任务格式无效")
	}
	text, final, shown := "", "", 0
	report("正在生成候选回复", nil)
	for {
		event, e := t.server.Receive(ctx)
		if e != nil {
			return nil, e
		}
		params, _ := event["params"].(map[string]interface{})
		turnID := params["turnId"]
		if turnID == nil {
			turnID = nestedID(params["turn"])
		}
		if params["threadId"] != thread || turnID != turn {
			continue
		}
		switch event["method"] {
		case "item/agentMessage/delta":
			delta, _ := params["delta"].(string)
			text += delta
			if metrics["first_output_s"] == nil {
				metrics["first_output_s"] = float64(time.Since(started).Milliseconds()) / 1000
			}
			rows := PreviewCandidates(text, task)
			if len(rows) > shown {
				shown = len(rows)
				if metrics["first_candidate_s"] == nil {
					metrics["first_candidate_s"] = float64(time.Since(started).Milliseconds()) / 1000
				}
				report("候选已生成，正在核对完整结果", rows)
			}
		case "item/completed":
			item, _ := params["item"].(map[string]interface{})
			switch item["type"] {
			case "agentMessage":
				final, _ = item["text"].(string)
			case "commandExecution", "fileChange", "mcpToolCall":
				metrics["tool_calls"] = metrics["tool_calls"].(int) + 1
			}
		case "turn/completed":
			turnInfo, _ := params["turn"].(map[string]interface{})
			if turnInfo["status"] != "completed" {
				return nil, errors.New("模型未完成回复，请检查登录或额度")
			}
			if final == "" {
				final = text
			}
			result, err = modelapi.DecodeResult(map[string]interface{}{"choices": []interface{}{map[string]interface{}{"message": map[string]interface{}{"content": final}}}}, "openai", task, input.WithMemory)
			if err != nil {
				return nil, err
			}
			report("回复已完成", nil)
			if input.Quick {
				if _, err = t.server.Send("thread/unsubscribe", Message{"threadId": thread}, true); err != nil {
					return nil, err
				}
			}
			return result, nil
		}
	}
}
