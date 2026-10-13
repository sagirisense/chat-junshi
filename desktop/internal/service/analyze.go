package service

import (
	"context"
	"encoding/json"
	"errors"
	"junshi-desktop/internal/codexclient"
	"junshi-desktop/internal/memory"
	"junshi-desktop/internal/modelapi"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Generation struct {
	Packet, Model, Provider, Mode, Task, SkillRoot string
	API                                            modelapi.Config
	WithMemory                                     bool
}
type generationToken struct {
	title           string
	revision, epoch int
}

func (s *Service) token() generationToken {
	if s.Current == nil {
		return generationToken{epoch: s.epoch}
	}
	return generationToken{s.Current.Title, s.Current.Revision, s.epoch}
}

func (s *Service) skillRoot() string {
	if s.SkillRoot != "" {
		return s.SkillRoot
	}
	if info, err := os.Stat(filepath.Join(s.Root, "skill", "goutoujunshi", "SKILL.md")); err == nil && !info.IsDir() {
		return filepath.Join(s.Root, "skill", "goutoujunshi")
	}
	home := os.Getenv("CODEX_HOME")
	if home == "" {
		user, _ := os.UserHomeDir()
		home = filepath.Join(user, ".codex")
	}
	return filepath.Join(home, "skills", "goutoujunshi")
}

func appendJSON(packet, label string, value interface{}) string {
	encoded, _ := json.Marshal(value)
	return packet + label + string(encoded)
}

func (s *Service) beginAnalysis(payload Data) error {
	if s.closed {
		return errors.New("应用已退出")
	}
	if s.Busy {
		return errors.New("正在分析，请稍候")
	}
	if s.Current == nil || s.Blocked {
		return errors.New("请先采集并核对单聊")
	}
	if _, ok := payload["goal"]; ok {
		if err := s.setGoal(str(payload, "goal")); err != nil {
			return err
		}
	}
	task := str(payload, "task")
	if task == "" {
		task = "reply"
	}
	style := str(payload, "style")
	if style == "" {
		style = "自然一点"
	}
	instructions, err := modelapi.RequestInstructions(task, str(payload, "draft"), style, str(payload, "interests"))
	if err != nil {
		return err
	}
	limit := 30
	if task == "topics" {
		limit = 80
	}
	goal := s.Profile["goal"]
	packet, err := s.Current.Packet(limit, &goal, time.Now())
	if err != nil {
		return err
	}
	packet += "\n本轮回复目标优先用于选择话术；长期档案中的目标只作为关系背景。"
	var contextData memory.Row
	status, err := s.Memory.Status()
	if err == nil && status["consent_enabled"] == true && status["paused"] != true {
		contextData, err = s.Memory.Context(s.subject(), 4000)
	}
	if err != nil {
		s.MemoryNotice = "本次未读取长期记忆，仅分析当前聊天。"
	}
	profile := s.profiles[s.subject()]
	if contextData != nil {
		profile = s.Profile
	}
	if profile == nil {
		profile = map[string]string{}
	}
	packet = appendJSON(packet, "\n用户明确填写的本轮档案（未知保持未知）：\n", profile)
	withMemory := contextData != nil && task == "reply"
	if contextData != nil {
		packet += "\n宿主已经确认用户启用了本机关系记忆，无需再次询问同意；记忆读写全部由宿主负责。"
		packet = appendJSON(packet, "\n当前对象的精简长期记忆：\n", contextData)
		if withMemory {
			packet += "\n仅提出高价值的新 memory_updates：event 为原文支持的关键事件；hypothesis 为有证据、可纠正的暂定解释。引用本次消息 id。不把推测写成 event，不记录整份聊天，不代填MBTI评分，不调用记忆脚本；由宿主受控保存，没有变化返回空数组。"
		}
	}
	packet += instructions
	provider := str(payload, "provider")
	if provider == "" {
		provider = "codex"
	}
	mode := str(payload, "mode")
	if mode == "" || task != "reply" {
		mode = "quick"
	}
	var api modelapi.Config
	if raw, ok := payload["api"]; ok {
		encoded, err := json.Marshal(raw)
		if err != nil {
			return err
		}
		if err = json.Unmarshal(encoded, &api); err != nil {
			return errors.New("API 配置无效")
		}
	}
	input := Generation{packet, str(payload, "model"), provider, mode, task, s.skillRoot(), api, withMemory}
	token := s.token()
	subject := s.subject()
	ids := map[int]bool{}
	start := len(s.Current.Messages) - limit
	if start < 0 {
		start = 0
	}
	for _, row := range s.Current.Messages[start:] {
		ids[number(row, "id")] = true
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelGeneration = cancel
	s.Busy = true
	s.Result = nil
	s.Status = "军师正在阅读上下文…"
	s.Preview = []interface{}{}
	s.Metrics = Data{}
	s.AssistanceNotice = ""
	go func() {
		defer cancel()
		s.analyzeBackground(ctx, input, token, subject, ids, truth(payload, "use_hotspots"))
	}()
	return nil
}

func (s *Service) generate(ctx context.Context, input Generation, progress codexclient.Progress) (Data, error) {
	if s.Generate != nil {
		return s.Generate(ctx, input, progress)
	}
	if input.Provider == "custom" {
		rules := ""
		if input.Mode != "quick" {
			for _, name := range []string{"SKILL.md", "references/practical/实战话术编排器：从一句回复到后续分支.md", "references/knowledge/09-在线约会与数字关系.md"} {
				data, err := os.ReadFile(filepath.Join(input.SkillRoot, filepath.FromSlash(name)))
				if err != nil {
					return nil, errors.New("未找到本地狗头军师规则或回复参考文件")
				}
				if rules != "" {
					rules += "\n\n"
				}
				rules += string(data)
			}
		}
		return (modelapi.Client{}).Analyze(ctx, input.API, modelapi.AnalyzeRequest{Packet: input.Packet, Model: input.Model, Rules: rules, WithMemory: input.WithMemory, Quick: input.Mode == "quick", Task: input.Task})
	}
	return s.transport.Analyze(ctx, codexclient.AnalyzeRequest{Packet: input.Packet, Model: input.Model, Task: input.Task, SkillPath: filepath.Join(input.SkillRoot, "SKILL.md"), Directory: s.Root, WithMemory: input.WithMemory, Quick: input.Mode == "quick"}, progress)
}

func (s *Service) analyzeBackground(ctx context.Context, input Generation, token generationToken, subject string, ids map[int]bool, hotspots bool) {
	started := time.Now()
	progress := func(stage string, metrics codexclient.Message, rows []interface{}) {
		s.mu.Lock()
		if s.token() != token || s.closed {
			s.mu.Unlock()
			return
		}
		s.Status = stage
		s.Metrics = metrics
		if rows != nil {
			s.Preview = rows
		}
		state := s.snapshot()
		s.mu.Unlock()
		if s.Emit != nil {
			s.Emit(state)
		}
	}
	sources := []Data{}
	if input.Task == "topics" {
		notice := "本次只根据历史和兴趣找话题，未联网检索热点。"
		if hotspots && s.News != nil {
			progress("正在获取近期资讯", codexclient.Message{}, nil)
			sources, notice = s.News(ctx)
		}
		s.mu.Lock()
		if s.token() == token {
			s.AssistanceNotice = notice
		}
		s.mu.Unlock()
		input.Packet = appendJSON(input.Packet, "\n宿主提供的近期资讯（可能没有适合对象的内容；无来源不得编造热点）：\n", sources)
	}
	result, err := s.generate(ctx, input, progress)
	if err == nil && input.Task == "topics" {
		err = modelapi.AttachSources(result, ids, sources)
	}
	if err == nil && input.Task != "reply" {
		result["task"] = input.Task
	}
	s.mu.Lock()
	s.Busy = false
	s.Preview = []interface{}{}
	s.Metrics["total_s"] = float64(time.Since(started).Milliseconds()) / 1000
	s.cancelGeneration = nil
	if s.token() != token || s.closed {
		s.Status = "会话或上下文已变化，旧建议已丢弃，请重新分析"
	} else if err != nil {
		s.Status = "分析失败：" + err.Error()
	} else {
		s.Result = result
		s.Status = "建议已准备好，选择适合你的表达"
		if input.WithMemory {
			s.saveMemoryUpdates(result, subject, ids)
		}
	}
	state := s.snapshot()
	closed := s.closed
	s.mu.Unlock()
	if !closed && s.Emit != nil {
		s.Emit(state)
	}
}

func (s *Service) saveMemoryUpdates(result Data, subject string, ids map[int]bool) {
	updates, _ := result["memory_updates"].([]interface{})
	delete(result, "memory_updates")
	saved := []string{}
	for _, raw := range updates {
		row, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		scope, _ := row["scope"].(string)
		field, _ := row["field"].(string)
		value, _ := row["value"].(string)
		confidence, _ := row["confidence"].(string)
		refs, ok := row["evidence_ids"].([]interface{})
		if !ok || len(refs) == 0 || len([]rune(value)) == 0 || len([]rune(value)) > 200 || (scope != "event" && scope != "hypothesis") || !in(field, "planned_activity", "conflict", "repair", "key_event", "intent") || !in(confidence, "high", "medium", "low") {
			continue
		}
		valid := true
		references := []string{}
		for _, id := range refs {
			var ident int
			switch v := id.(type) {
			case int:
				ident = v
			case json.Number:
				n, err := v.Int64()
				if err != nil {
					valid = false
				}
				ident = int(n)
			case float64:
				ident = int(v)
				if float64(ident) != v {
					valid = false
				}
			default:
				valid = false
			}
			if !ids[ident] {
				valid = false
			}
			references = append(references, strconv.Itoa(ident))
		}
		if !valid {
			continue
		}
		source := "tool"
		if scope == "hypothesis" {
			source = "assistant_inference"
		}
		if _, err := s.Memory.Apply(memory.Row{"scope": scope, "subject_id": subject, "field": field, "value": value, "source_type": source, "source_ref": "消息 " + strings.Join(references, ","), "confidence": confidence}); err != nil {
			s.MemoryNotice = "建议已生成，但记忆写入失败；未完成保存"
			break
		}
		saved = append(saved, value)
	}
	if len(saved) > 0 {
		s.MemoryNotice = "记忆更新：" + strings.Join(saved, "；") + "。可撤销最近更新。"
	}
}

func in(value string, choices ...string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}
func (s *Service) Close() {
	s.mu.Lock()
	s.closed = true
	s.invalidate()
	s.mu.Unlock()
	if s.collector != nil {
		s.collector.Close()
	}
	s.transport.Close()
}
