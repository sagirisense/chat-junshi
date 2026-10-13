package service

import (
	"errors"
	"junshi-desktop/internal/capture"
	"junshi-desktop/internal/evidence"
	"time"
)

// CaptureEvent is the boundary between native capture and business state.
type CaptureEvent struct {
	Kind, Title, Status string
	Lines               []evidence.CapturedLine
}

func (s *Service) HandleCapture(event CaptureEvent) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	err := s.handleCapture(event)
	state := s.snapshot()
	s.mu.Unlock()
	if s.Emit != nil {
		s.Emit(state)
	}
	return err
}

func (s *Service) ConfigureCapture(models, dll, wgc string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.collector = capture.NewCollector(models, dll, wgc, func(event capture.Event) {
		_ = s.HandleCapture(CaptureEvent{event.Kind, event.Title, event.Status, event.Lines})
	})
}

func (s *Service) stopCapture() {
	s.Capturing = false
	if s.collector != nil {
		s.collector.SetEnabled(false)
	}
}

func (s *Service) handleCapture(event CaptureEvent) error {
	switch event.Kind {
	case "ocr_ready":
		s.OCRReady = true
		if !s.Capturing {
			s.Status = "OCR 已准备好，打开微信单聊后开启采集"
		}
	case "dead":
		s.OCRReady = false
		if s.Capturing {
			s.Capturing = false
			s.Blocked = true
			s.invalidate()
			s.Status = event.Status
		} else {
			s.Status = "OCR 预热失败，开启采集时将重试"
		}
	default:
		if !s.Capturing {
			return nil
		}
		switch event.Kind {
		case "chat":
			if err := s.selectSession(event.Title); err != nil {
				return err
			}
			s.ActiveTitle = event.Title
			s.Blocked = false
		case "lines":
			if s.Blocked {
				return nil
			}
			if err := s.selectSession(event.Title); err != nil {
				return err
			}
			if err := s.Current.Append(event.Lines, time.Now()); err != nil {
				return err
			}
			s.invalidate()
			s.Status = "已收到消息，核对后可生成建议"
		case "status":
			s.Status = event.Status
		case "blocked":
			s.Status = event.Status
			s.Blocked = true
			s.invalidate()
			if s.Current != nil {
				s.Current.Confirmed = false
			}
		default:
			return errors.New("未知采集事件")
		}
	}
	return nil
}
