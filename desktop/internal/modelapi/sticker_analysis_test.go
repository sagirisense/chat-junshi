package modelapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStickerAnalysisProtocols(t *testing.T) {
	final := `{"name":"晚安","tags":"晚安,困了","suitable":"结束聊天","avoid":"严肃讨论","intimate":false,"intimate_reason":"普通晚安问候，无明显暧昧或越界表达"}`
	for _, protocol := range []string{"openai", "anthropic", "gemini"} {
		t.Run(protocol, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				b, _ := json.Marshal(body)
				if !strings.Contains(string(b), base64.StdEncoding.EncodeToString([]byte("synthetic-image"))) {
					t.Error("missing actual image bytes")
				}
				var response interface{}
				switch protocol {
				case "anthropic":
					if r.URL.Path != "/v1/messages" || !strings.Contains(string(b), `"media_type":"image/png"`) {
						t.Error("invalid anthropic image request")
					}
					response = map[string]interface{}{"content": []map[string]interface{}{{"type": "text", "text": final}}}
				case "gemini":
					if r.URL.Path != "/models/vision:generateContent" || !strings.Contains(string(b), `"inlineData"`) {
						t.Error("invalid gemini image request")
					}
					response = map[string]interface{}{"candidates": []map[string]interface{}{{"content": map[string]interface{}{"parts": []map[string]interface{}{{"text": final}}}}}}
				default:
					if r.URL.Path != "/chat/completions" || !strings.Contains(string(b), `"image_url"`) {
						t.Error("invalid openai image request")
					}
					response = map[string]interface{}{"choices": []map[string]interface{}{{"message": map[string]interface{}{"content": final}}}}
				}
				json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()
			result, err := (Client{}).AnalyzeSticker(context.Background(), Config{BaseURL: server.URL, Protocol: protocol}, "vision", []byte("synthetic-image"))
			if err != nil || result["tags"] != "晚安,困了" {
				t.Fatal(result, err)
			}
		})
	}
}

func TestRejectInvalidStickerAnalysis(t *testing.T) {
	for _, content := range []string{`{}`, `{"name":"晚安","tags":"","suitable":"","avoid":"","intimate":false}`, `{"name":"晚安","tags":"困了","suitable":"","avoid":"","intimate":"false"}`, `{"name":"亲亲","tags":"索吻","suitable":"情侣","avoid":"陌生关系","intimate":true}`, `{"name":"亲亲","tags":"索吻","suitable":"情侣","avoid":"陌生关系","intimate":true,"intimate_reason":""}`} {
		_, err := DecodeResult(map[string]interface{}{"choices": []interface{}{map[string]interface{}{"message": map[string]interface{}{"content": content}}}}, "openai", "sticker", false)
		if err == nil {
			t.Fatal("accepted invalid metadata", content)
		}
	}
}

func TestIntimacyJudgmentRequiresReasonAndPreservesBothValues(t *testing.T) {
	for _, intimate := range []bool{true, false} {
		result := map[string]interface{}{"name": "测试", "tags": "测试", "suitable": "", "avoid": "", "intimate": intimate, "intimate_reason": "根据图片中文字和动作判断使用边界"}
		content, _ := json.Marshal(result)
		decoded, err := DecodeResult(map[string]interface{}{"choices": []interface{}{map[string]interface{}{"message": map[string]interface{}{"content": string(content)}}}}, "openai", "sticker", false)
		if err != nil || decoded["intimate"] != intimate || decoded["intimate_reason"] != result["intimate_reason"] {
			t.Fatal(decoded, err)
		}
	}
}
