package codexclient

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The test executable acts as a synthetic CLI; no account or external model is used.
func TestFakeServerProcess(t *testing.T) {
	if os.Getenv("JUNSHI_FAKE_CODEX") != "1" {
		return
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var message Message
		json.Unmarshal(scanner.Bytes(), &message)
		if message["id"] == nil {
			continue
		}
		result := Message{}
		switch message["method"] {
		case "model/list":
			result = Message{"data": []Message{{"model": "synthetic", "isDefault": true}}}
		case "thread/start":
			params, _ := message["params"].(map[string]interface{})
			if params["sandbox"] != "read-only" || params["approvalPolicy"] != "never" || params["ephemeral"] != true {
				os.Exit(4)
			}
			result = Message{"thread": Message{"id": "isolated-thread"}}
		case "turn/start":
			result = Message{"turn": Message{"id": "synthetic-turn"}}
			data, _ := json.Marshal(Message{"id": message["id"], "result": result})
			fmt.Println(string(data))
			final := `{"summary":"合成测试","facts":[],"hypotheses":[],"unknowns":[],"recommended_index":0,"recommendation_reason":"测试","candidates":[{"text":"你好","pros":[],"cons":[],"suitable_when":"测试","next_step":"等待"}]}`
			data, _ = json.Marshal(Message{"method": "item/agentMessage/delta", "params": Message{"threadId": "isolated-thread", "turnId": "synthetic-turn", "delta": final}})
			fmt.Println(string(data))
			data, _ = json.Marshal(Message{"method": "item/completed", "params": Message{"threadId": "isolated-thread", "turnId": "synthetic-turn", "item": Message{"type": "agentMessage", "text": final}}})
			fmt.Println(string(data))
			fmt.Println(`{"method":"turn/completed","params":{"threadId":"isolated-thread","turn":{"id":"synthetic-turn","status":"completed"}}}`)
			continue
		case "test/tool":
			fmt.Println(`{"id":99,"method":"item/tool/request","params":{}}`)
			if !scanner.Scan() {
				os.Exit(2)
			}
			var refusal Message
			json.Unmarshal(scanner.Bytes(), &refusal)
			if refusal["error"] == nil {
				os.Exit(3)
			}
		case "test/wait":
			continue
		}
		data, _ := json.Marshal(Message{"id": message["id"], "result": result})
		fmt.Println(string(data))
	}
	os.Exit(0)
}

func fakeServer(t *testing.T) *Server {
	t.Helper()
	t.Setenv("JUNSHI_FAKE_CODEX", "1")
	s, err := startCommand([]string{os.Args[0], "-test.run=^TestFakeServerProcess$"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func TestProtocolModelsAndToolRefusal(t *testing.T) {
	s := fakeServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Initialize(ctx, "test"); err != nil {
		t.Fatal(err)
	}
	models, err := s.Models(ctx)
	if err != nil || len(models) != 1 || models[0]["model"] != "synthetic" {
		t.Fatal(models, err)
	}
	if _, err = s.Request(ctx, "test/tool", Message{}); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s.Close()
}

func TestRequestCancellation(t *testing.T) {
	s := fakeServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := s.Request(ctx, "test/wait", Message{}); err == nil {
		t.Fatal("ignored timeout")
	}
}

func TestAnalyzeIsolationAndReuse(t *testing.T) {
	t.Setenv("JUNSHI_FAKE_CODEX", "1")
	starts := 0
	transport := &Transport{Start: func() (*Server, error) {
		starts++
		return startCommand([]string{os.Args[0], "-test.run=^TestFakeServerProcess$"})
	}}
	defer transport.Close()
	previews := 0
	for i := 0; i < 2; i++ {
		got, err := transport.Analyze(context.Background(), AnalyzeRequest{Packet: "synthetic", Quick: true}, func(stage string, metrics Message, rows []interface{}) {
			if len(rows) > 0 {
				previews++
			}
		})
		if err != nil || got["summary"] != "合成测试" {
			t.Fatal(got, err)
		}
	}
	if starts != 1 || previews != 2 {
		t.Fatal(starts, previews)
	}
	if len(PreviewCandidates(`"candidates"`, "reply")) != 0 || len(PreviewCandidates(`{"candidates":[{"text":"unfinished`, "reply")) != 0 {
		t.Fatal("repaired incomplete JSON")
	}
}

func TestDesktopCommandWithoutPATH(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "OpenAI", "Codex", "bin", "version", "codex.exe")
	if err := os.MkdirAll(filepath.Dir(binary), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "")
	t.Setenv("APPDATA", dir)
	t.Setenv("LOCALAPPDATA", dir)
	command, err := Command()
	if err != nil || len(command) != 2 || !strings.EqualFold(command[0], binary) {
		t.Fatal(command, err)
	}
}
