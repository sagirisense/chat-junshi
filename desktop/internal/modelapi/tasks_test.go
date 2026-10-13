package modelapi

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTaskInstructions(t *testing.T) {
	text, err := RequestInstructions("polish", "  明天不去了  ", "表达边界", "")
	if err != nil || !strings.Contains(text, "保持原意") || !strings.Contains(text, "明天不去了") {
		t.Fatal(text, err)
	}
	for _, args := range [][4]string{{"polish", "", "自然一点", ""}, {"polish", "你好", "无效方向", ""}, {"topics", "", "", strings.Repeat("中", 201)}} {
		if _, err := RequestInstructions(args[0], args[1], args[2], args[3]); err == nil {
			t.Fatal("accepted invalid task input")
		}
	}
}

func TestTopicCitations(t *testing.T) {
	var result map[string]interface{}
	json.Unmarshal([]byte(syntheticResult), &result)
	row := result["candidates"].([]interface{})[0].(map[string]interface{})
	row["topic_title"] = "聊近况"
	row["basis"] = "history"
	row["preference_level"] = "mentioned"
	row["evidence_ids"] = []interface{}{float64(1)}
	row["source_ids"] = []interface{}{}
	if err := ValidateResult(result, "topics"); err != nil {
		t.Fatal(err)
	}
	if err := AttachSources(result, map[int]bool{}, nil); err == nil {
		t.Fatal("accepted unknown history")
	}
	row["evidence_ids"] = []interface{}{}
	if err := AttachSources(result, map[int]bool{}, nil); err != nil {
		t.Fatal(err)
	}
	if row["basis"] != "general" || row["preference_level"] != "unknown" {
		t.Fatal("claimed preference without evidence")
	}
	row["basis"] = "hotspot"
	if err := AttachSources(result, map[int]bool{}, nil); err == nil {
		t.Fatal("accepted unsourced hotspot")
	}
	row["source_ids"] = []interface{}{float64(2)}
	source := map[string]interface{}{"id": 2, "title": "虚构测试资讯"}
	if err := AttachSources(result, map[int]bool{}, []map[string]interface{}{source}); err != nil {
		t.Fatal(err)
	}
	if len(row["sources"].([]map[string]interface{})) != 1 {
		t.Fatal("lost source")
	}
}
