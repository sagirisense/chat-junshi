package modelapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestModelsAuthenticationAndPagination(t *testing.T) {
	for _, protocol := range []string{"openai", "anthropic", "gemini"} {
		t.Run(protocol, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "GET" {
					t.Error("model listing submitted a body")
				}
				switch protocol {
				case "openai":
					if r.Header.Get("Authorization") != "Bearer test-only" || r.URL.Path != "/models" {
						t.Error("incorrect openai request")
					}
					w.Write([]byte(`{"data":[{"id":"b"},{"id":"a"},{"id":"a"}]}`))
				case "anthropic":
					if r.Header.Get("x-api-key") != "test-only" || r.Header.Get("anthropic-version") != "2023-06-01" {
						t.Error("incorrect anthropic auth")
					}
					if calls == 1 {
						w.Write([]byte(`{"data":[{"id":"a"}],"has_more":true,"last_id":"a b"}`))
					} else {
						if r.URL.Query().Get("after_id") != "a b" {
							t.Error("incorrect pagination")
						}
						w.Write([]byte(`{"data":[{"id":"b"}]}`))
					}
				case "gemini":
					if r.Header.Get("x-goog-api-key") != "test-only" {
						t.Error("incorrect gemini auth")
					}
					if calls == 1 {
						w.Write([]byte(`{"models":[{"name":"models/a","supportedGenerationMethods":["generateContent"]},{"name":"models/embed","supportedGenerationMethods":["embedContent"]}],"nextPageToken":"next +"}`))
					} else {
						if r.URL.Query().Get("pageToken") != "next +" {
							t.Error("incorrect pagination")
						}
						w.Write([]byte(`{"models":[{"name":"models/b","supportedGenerationMethods":["generateContent"]}]}`))
					}
				}
			}))
			defer server.Close()
			got, err := (Client{}).Models(context.Background(), Config{BaseURL: server.URL, APIKey: "test-only", Protocol: protocol})
			if err != nil || !reflect.DeepEqual(got, []string{"a", "b"}) {
				t.Fatal(got, err)
			}
		})
	}
}

func TestRedirectAndErrorRedaction(t *testing.T) {
	targetCalls := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { targetCalls++ }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer server.Close()
	_, err := (Client{}).Send(context.Background(), Config{BaseURL: server.URL, APIKey: "private-test-key"}, "models", nil, time.Second)
	if err == nil || targetCalls != 0 || strings.Contains(err.Error(), "private-test-key") || strings.Contains(err.Error(), server.URL) {
		t.Fatal("redirect or error exposure", err)
	}
	for _, base := range []string{"https://user:secret@example.com/v1", "ftp://example.com", "https://example.com?key=secret", "https://example.com/#fragment", ""} {
		if _, err := Endpoint(Config{BaseURL: base}, "models"); err == nil {
			t.Errorf("accepted %s", base)
		}
	}
}

func TestInvalidJSONAndCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("not json")) }))
	defer server.Close()
	_, err := (Client{}).Models(context.Background(), Config{BaseURL: server.URL})
	if err == nil || err.Error() != "API 未返回有效 JSON" {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = (Client{}).Models(ctx, Config{BaseURL: server.URL}); err == nil {
		t.Fatal("ignored cancellation")
	}
}
