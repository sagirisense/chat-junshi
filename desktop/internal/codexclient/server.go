package codexclient

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"sync"
)

type Message map[string]interface{}

type Server struct {
	process   *exec.Cmd
	input     io.WriteCloser
	events    chan Message
	done      chan struct{}
	stop      chan struct{}
	mu        sync.Mutex
	sequence  int
	closeOnce sync.Once
}

func Start() (*Server, error) {
	command, err := Command()
	if err != nil {
		return nil, err
	}
	return startCommand(command)
}

func startCommand(command []string) (*Server, error) {
	cmd := exec.Command(command[0], command[1:]...)
	hideWindow(cmd)
	input, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		input.Close()
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		input.Close()
		output.Close()
		return nil, err
	}
	s := &Server{process: cmd, input: input, events: make(chan Message, 128), done: make(chan struct{}), stop: make(chan struct{})}
	go func() {
		defer close(s.done)
		scanner := bufio.NewScanner(output)
		scanner.Buffer(make([]byte, 65536), 8*1024*1024)
		for scanner.Scan() {
			var message Message
			if json.Unmarshal(scanner.Bytes(), &message) != nil {
				continue
			}
			select {
			case s.events <- message:
			case <-s.stop:
				return
			}
		}
	}()
	return s, nil
}

func (s *Server) write(message Message) error {
	encoded, err := json.Marshal(message)
	if err != nil {
		return err
	}
	_, err = s.input.Write(append(encoded, '\n'))
	if err != nil {
		return errors.New("Codex 连接已关闭，请检查 CLI 登录状态")
	}
	return nil
}

func (s *Server) Send(method string, params interface{}, request bool) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	message := Message{"method": method, "params": params}
	id := 0
	if request {
		s.sequence++
		id = s.sequence
		message["id"] = id
	}
	return id, s.write(message)
}

func (s *Server) Receive(ctx context.Context) (Message, error) {
	var message Message
	select {
	case <-ctx.Done():
		return nil, errors.New("Codex 分析已取消或超时，请稍后重试")
	case message = <-s.events:
	case <-s.done:
		select {
		case message = <-s.events:
		default:
			return nil, errors.New("Codex 连接已关闭，请检查 CLI 登录状态")
		}
	}
	if _, method := message["method"]; method {
		if id, request := message["id"]; request {
			s.mu.Lock()
			err := s.write(Message{"id": id, "error": Message{"code": -32601, "message": "This client supports analysis only"}})
			s.mu.Unlock()
			if err != nil {
				return nil, err
			}
		}
	}
	return message, nil
}

func (s *Server) Request(ctx context.Context, method string, params interface{}) (Message, error) {
	id, err := s.Send(method, params, true)
	if err != nil {
		return nil, err
	}
	for {
		message, err := s.Receive(ctx)
		if err != nil {
			return nil, err
		}
		if message["id"] == float64(id) && message["method"] == nil {
			if message["error"] != nil {
				return nil, errors.New("Codex 拒绝请求，请检查 CLI 版本、登录与 Skill 路径")
			}
			result, ok := message["result"].(map[string]interface{})
			if !ok {
				return nil, errors.New("Codex 返回格式无效")
			}
			return result, nil
		}
	}
}

func (s *Server) Initialize(ctx context.Context, name string) error {
	_, err := s.Request(ctx, "initialize", Message{"clientInfo": Message{"name": name, "version": "0.1.0"}, "capabilities": Message{}})
	if err != nil {
		return err
	}
	_, err = s.Send("initialized", Message{}, false)
	return err
}

func (s *Server) Models(ctx context.Context) ([]Message, error) {
	result, err := s.Request(ctx, "model/list", Message{})
	if err != nil {
		return nil, err
	}
	rows, ok := result["data"].([]interface{})
	if !ok {
		return nil, errors.New("Codex 模型列表格式无效")
	}
	models := []Message{}
	for _, raw := range rows {
		row, ok := raw.(map[string]interface{})
		if !ok {
			return nil, errors.New("Codex 模型列表格式无效")
		}
		models = append(models, row)
	}
	return models, nil
}

func (s *Server) Close() {
	s.closeOnce.Do(func() { close(s.stop); s.input.Close(); s.process.Process.Kill(); s.process.Wait(); <-s.done })
}
