package modelapi

import (
	"encoding/json"
	"errors"
	"strings"
)

func RequestInstructions(task, draft, style, interests string) (string, error) {
	switch task {
	case "polish":
		if strings.TrimSpace(draft) == "" || len([]rune(draft)) > 2000 {
			return "", errors.New("请输入想发送的内容（最多2000字）")
		}
		if !contains([]string{"自然一点", "简短一点", "温柔一点", "轻松一点", "表达边界"}, style) {
			return "", errors.New("请选择有效的润色方向")
		}
		data, _ := json.Marshal(map[string]string{"draft": strings.TrimSpace(draft), "style": style})
		return polishInstructions + string(data), nil
	case "topics":
		if len([]rune(interests)) > 200 {
			return "", errors.New("兴趣补充请控制在200字以内")
		}
		data, _ := json.Marshal(strings.TrimSpace(interests))
		return topicsInstructions + string(data), nil
	case "reply", "":
		return "", nil
	default:
		return "", errors.New("不支持的辅助任务")
	}
}

func AttachSources(result map[string]interface{}, evidenceIDs map[int]bool, sources []map[string]interface{}) error {
	valid := map[int]map[string]interface{}{}
	for _, source := range sources {
		if id, ok := integer(source["id"]); ok {
			valid[id] = source
		}
	}
	rows, ok := result["candidates"].([]interface{})
	if !ok {
		return errors.New("话题结果格式无效")
	}
	// Validate every citation before attaching any sources.
	for _, raw := range rows {
		row, ok := raw.(map[string]interface{})
		if !ok {
			return errors.New("话题结果格式无效")
		}
		ids, ok := row["evidence_ids"].([]interface{})
		if !ok {
			return errors.New("话题引用无效")
		}
		for _, value := range ids {
			id, ok := integer(value)
			if !ok || !evidenceIDs[id] {
				return errors.New("话题引用了本轮未提供的历史消息，请重试")
			}
		}
		refs, ok := row["source_ids"].([]interface{})
		if !ok {
			return errors.New("话题引用无效")
		}
		for _, value := range refs {
			id, ok := integer(value)
			if !ok || valid[id] == nil {
				return errors.New("话题引用了未检索到的资讯，请重试")
			}
		}
		if row["basis"] == "hotspot" && len(refs) == 0 {
			return errors.New("热点话题没有可核验来源，请重试")
		}
	}
	for _, raw := range rows {
		row := raw.(map[string]interface{})
		if row["basis"] == "history" && len(row["evidence_ids"].([]interface{})) == 0 {
			row["basis"] = "general"
			row["preference_level"] = "unknown"
		}
		attached := []map[string]interface{}{}
		for _, value := range row["source_ids"].([]interface{}) {
			id, _ := integer(value)
			attached = append(attached, valid[id])
		}
		row["sources"] = attached
	}
	return nil
}
