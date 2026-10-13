package modelapi

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
)

func exactFields(row map[string]interface{}, fields ...string) bool {
	if len(row) != len(fields) {
		return false
	}
	for _, field := range fields {
		if _, ok := row[field]; !ok {
			return false
		}
	}
	return true
}

func stringList(value interface{}) bool {
	rows, ok := value.([]interface{})
	if !ok {
		return false
	}
	for _, row := range rows {
		if _, ok := row.(string); !ok {
			return false
		}
	}
	return true
}

func integer(value interface{}) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case float64:
		if !math.IsNaN(v) && !math.IsInf(v, 0) && math.Trunc(v) == v && v >= -9007199254740991 && v <= 9007199254740991 {
			return int(v), true
		}
	case json.Number:
		n, err := v.Int64()
		return int(n), err == nil
	}
	return 0, false
}

func ValidateResult(result map[string]interface{}, task string) error {
	if !exactFields(result, "summary", "facts", "hypotheses", "unknowns", "recommended_index", "recommendation_reason", "candidates") {
		return errors.New("分析结果结构不完整")
	}
	for _, field := range []string{"summary", "recommendation_reason"} {
		if _, ok := result[field].(string); !ok {
			return errors.New("分析结果文字无效")
		}
	}
	for _, field := range []string{"facts", "hypotheses", "unknowns"} {
		if !stringList(result[field]) {
			return errors.New("分析证据结构无效")
		}
	}
	rows, ok := result["candidates"].([]interface{})
	if !ok || len(rows) < 1 || len(rows) > 3 {
		return errors.New("候选数量无效")
	}
	for _, raw := range rows {
		row, ok := raw.(map[string]interface{})
		if !ok {
			return errors.New("候选结构无效")
		}
		fields := []string{"text", "pros", "cons", "suitable_when", "next_step"}
		if task == "topics" {
			fields = append(fields, "topic_title", "basis", "evidence_ids", "source_ids", "preference_level")
			if title, ok := row["topic_title"].(string); !ok || strings.TrimSpace(title) == "" {
				return errors.New("话题标题无效")
			}
			basis, _ := row["basis"].(string)
			level, _ := row["preference_level"].(string)
			if !contains([]string{"history", "preference", "hotspot", "general"}, basis) || !contains([]string{"confirmed", "mentioned", "inferred", "unknown"}, level) {
				return errors.New("话题依据类型无效")
			}
			for field, limit := range map[string]int{"evidence_ids": 5, "source_ids": 3} {
				ids, ok := row[field].([]interface{})
				if !ok || len(ids) > limit {
					return errors.New("话题引用无效")
				}
				for _, id := range ids {
					if _, ok := integer(id); !ok {
						return errors.New("话题引用无效")
					}
				}
			}
		}
		if !exactFields(row, fields...) {
			return errors.New("候选结构无效")
		}
		for _, field := range []string{"text", "suitable_when", "next_step"} {
			if _, ok := row[field].(string); !ok {
				return errors.New("候选文字无效")
			}
		}
		for _, field := range []string{"pros", "cons"} {
			if !stringList(row[field]) {
				return errors.New("候选分析无效")
			}
		}
	}
	idx, ok := integer(result["recommended_index"])
	if !ok || idx < 0 || idx >= len(rows) {
		return errors.New("推荐索引无效")
	}
	return nil
}

func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
