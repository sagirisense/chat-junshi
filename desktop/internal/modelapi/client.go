// Package modelapi implements the local application's custom API transport.
package modelapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type Config struct {
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key"`
	Protocol string `json:"protocol"`
	Source   string `json:"source"`
}

func (c Config) protocol() string {
	if c.Protocol == "" {
		return "openai"
	}
	return c.Protocol
}

func Endpoint(config Config, route string) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return "", errors.New("API 地址请填写 http(s) 基础地址，例如 https://api.example.com/v1")
	}
	return base + "/" + route, nil
}

type Client struct{ HTTP *http.Client }

func (c Client) Send(ctx context.Context, config Config, route string, body interface{}, timeout time.Duration) (map[string]interface{}, error) {
	protocol := config.protocol()
	if protocol != "openai" && protocol != "anthropic" && protocol != "gemini" {
		return nil, errors.New("不支持的 API 协议")
	}
	key := strings.TrimSpace(config.APIKey)
	if strings.ContainsAny(key, "\r\n") {
		return nil, errors.New("API Key 不能包含换行")
	}
	endpoint, err := Endpoint(config, route)
	if err != nil {
		return nil, err
	}
	var reader io.Reader
	method := http.MethodGet
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
		method = http.MethodPost
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, errors.New("API 无法连接或超时，请检查地址和网络")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	switch protocol {
	case "anthropic":
		req.Header.Set("x-api-key", key)
		req.Header.Set("anthropic-version", "2023-06-01")
	case "gemini":
		req.Header.Set("x-goog-api-key", key)
	default:
		if key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
	}
	if config.Source == "opencode" {
		req.Header.Set("x-opencode-session", uuid.NewString())
		req.Header.Set("User-Agent", "jev-chat-windows")
	}
	client := http.Client{}
	if c.HTTP != nil {
		client = *c.HTTP
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Do(req)
	if err != nil {
		return nil, errors.New("API 无法连接或超时，请检查地址和网络")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("API 请求失败（HTTP %d），请检查地址、密钥和模型权限", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 4*1024*1024+1))
	if err != nil {
		return nil, errors.New("API 无法连接或超时，请检查地址和网络")
	}
	if len(data) > 4*1024*1024 {
		return nil, errors.New("API 返回内容过大")
	}
	var result map[string]interface{}
	if err = json.Unmarshal(data, &result); err != nil {
		return nil, errors.New("API 未返回有效 JSON")
	}
	return result, nil
}

func (c Client) Models(ctx context.Context, config Config) ([]string, error) {
	protocol := config.protocol()
	route := "models"
	if protocol == "anthropic" {
		route = "v1/models"
	}
	ids := map[string]bool{}
	for page := 0; page < 20; page++ {
		data, err := c.Send(ctx, config, route, nil, 25*time.Second)
		if err != nil {
			return nil, err
		}
		key := "data"
		if protocol == "gemini" {
			key = "models"
		}
		rows, ok := data[key].([]interface{})
		if !ok {
			return nil, errors.New("模型列表格式无效，也可手动填写模型名")
		}
		for _, raw := range rows {
			row, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			field := "id"
			if protocol == "gemini" {
				field = "name"
			}
			id, ok := row[field].(string)
			if !ok || id == "" {
				continue
			}
			if protocol == "gemini" {
				supported := false
				methods, _ := row["supportedGenerationMethods"].([]interface{})
				for _, method := range methods {
					if method == "generateContent" {
						supported = true
					}
				}
				if !supported {
					continue
				}
			}
			ids[strings.TrimPrefix(id, "models/")] = true
		}
		if token, ok := data["nextPageToken"].(string); protocol == "gemini" && ok && token != "" {
			route = "models?pageToken=" + url.QueryEscape(token)
		} else if last, ok := data["last_id"].(string); protocol == "anthropic" && data["has_more"] == true && ok && last != "" {
			route = "v1/models?after_id=" + url.QueryEscape(last)
		} else {
			break
		}
	}
	result := []string{}
	for id := range ids {
		if config.Source == "opencode" {
			keep := false
			for _, prefix := range []string{"deepseek-", "glm-", "kimi-", "mimo-", "longcat-", "hy", "space-bunny-"} {
				if strings.HasPrefix(id, prefix) {
					keep = true
				}
			}
			if !keep {
				continue
			}
		}
		result = append(result, id)
	}
	if len(result) == 0 {
		return nil, errors.New("未返回可用聊天模型，也可手动填写模型名")
	}
	sort.Strings(result)
	return result, nil
}
