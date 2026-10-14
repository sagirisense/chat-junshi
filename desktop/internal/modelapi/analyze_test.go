package modelapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const syntheticResult = `{"summary":"先承接感受","facts":["[#1] 对方说很累"],"hypotheses":[],"unknowns":["疲惫原因"],"recommended_index":0,"recommendation_reason":"压力较低","candidates":[{"text":"今天辛苦了，先休息一下","pros":["承接感受"],"cons":["未推进话题"],"suitable_when":"对方疲惫","next_step":"等对方回复"}]}`

func responseFor(protocol, content string) map[string]interface{} {
	switch protocol {
	case "anthropic":
		return map[string]interface{}{"content": []interface{}{map[string]interface{}{"type": "text", "text": content}}}
	case "gemini":
		return map[string]interface{}{"candidates": []interface{}{map[string]interface{}{"content": map[string]interface{}{"parts": []interface{}{map[string]interface{}{"thought": true, "text": "internal reasoning"}, map[string]interface{}{"text": content}}}}}}
	default:
		return map[string]interface{}{"choices": []interface{}{map[string]interface{}{"message": map[string]interface{}{"content": content}}}}
	}
}

func TestAnalyzeThreeProtocols(t *testing.T) {
	for _, protocol := range []string{"openai", "anthropic", "gemini"} {
		t.Run(protocol, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				encoded, _ := json.Marshal(body)
				if !strings.Contains(string(encoded), "synthetic evidence") || strings.Contains(string(encoded), "$goutoujunshi") {
					t.Error("wrong prompt")
				}
				json.NewEncoder(w).Encode(responseFor(protocol, syntheticResult))
			}))
			defer server.Close()
			got, err := (Client{}).Analyze(context.Background(), Config{BaseURL: server.URL, Protocol: protocol}, AnalyzeRequest{Packet: "$goutoujunshi synthetic evidence", Model: "test-model", Quick: true})
			if err != nil || got["summary"] != "先承接感受" {
				t.Fatal(got, err)
			}
		})
	}
}

func TestResultRejectsInvalidStructure(t *testing.T) {
	for _, content := range []string{"not json", syntheticResult + " {}", strings.Replace(syntheticResult, `"recommended_index":0`, `"recommended_index":true`, 1), strings.Replace(syntheticResult, `"recommended_index":0`, `"recommended_index":0.5`, 1), strings.Replace(syntheticResult, `"recommended_index":0`, `"recommended_index":3`, 1)} {
		if _, err := DecodeResult(responseFor("openai", content), "openai", "reply", false); err == nil {
			t.Error("accepted invalid output")
		}
	}
	if _, err := DecodeResult(responseFor("openai", "```json\n"+syntheticResult+"\n```"), "openai", "reply", false); err != nil {
		t.Fatal(err)
	}
}

func TestMemoryAndTopicSchema(t *testing.T) {
	content := strings.TrimSuffix(syntheticResult, "}") + `,"memory_updates":[]}`
	got, err := DecodeResult(responseFor("openai", content), "openai", "reply", true)
	if err != nil || len(got["memory_updates"].([]interface{})) != 0 {
		t.Fatal(got, err)
	}
	if _, err := DecodeResult(responseFor("openai", strings.Replace(content, `"memory_updates":[]`, `"memory_updates":null`, 1)), "openai", "reply", true); err == nil {
		t.Fatal("accepted invalid memory")
	}
	schema := Schema(true, true, "topics")
	props := schema["properties"].(map[string]interface{})
	candidate := props["candidates"].(map[string]interface{})["items"].(map[string]interface{})
	if len(candidate["required"].([]interface{})) != 13 || props["memory_updates"] == nil {
		t.Fatal(schema)
	}
	if len(Schema(false, false, "reply")["properties"].(map[string]interface{})) != 7 {
		t.Fatal("mutated base schema")
	}
}
