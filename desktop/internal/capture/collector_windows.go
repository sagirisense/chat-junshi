//go:build windows

package capture

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/draw"
	"junshi-desktop/internal/evidence"
	"junshi-desktop/internal/ocr"
	"sync/atomic"
	"time"
)

type Event struct {
	Kind, Title, Status string
	Lines               []evidence.CapturedLine
}
type Collector struct {
	enabled          atomic.Bool
	cancel           context.CancelFunc
	done             chan struct{}
	emit             func(Event)
	models, dll, wgc string
}

func NewCollector(models, dll, wgc string, emit func(Event)) *Collector {
	ctx, cancel := context.WithCancel(context.Background())
	c := &Collector{cancel: cancel, done: make(chan struct{}), emit: emit, models: models, dll: dll, wgc: wgc}
	go c.run(ctx)
	return c
}
func (c *Collector) SetEnabled(value bool) { c.enabled.Store(value) }
func (c *Collector) Close()                { c.cancel(); <-c.done }
func (c *Collector) send(event Event) {
	if c.emit != nil {
		c.emit(event)
	}
}
func crop(img *image.RGBA, bounds image.Rectangle) *image.RGBA {
	result := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(result, result.Bounds(), img, bounds.Min, draw.Src)
	return result
}

func (c *Collector) run(ctx context.Context) {
	defer close(c.done)
	var engine *ocr.Engine
	var cap *WGC
	defer func() {
		if cap != nil {
			cap.Close()
		}
		if engine != nil {
			engine.Close()
		}
	}()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	readers := map[string]*ocr.Reader{}
	var window Window
	var last, head []byte
	var latest, pending *image.RGBA
	var changed, firstChange, started, retry time.Time
	var area *Area
	var frameBounds image.Rectangle
	title := ""
	valid, hasMessages := false, false
	reconnects := 0
	fail := func(message string) {
		c.enabled.Store(false)
		c.send(Event{Kind: "dead", Status: message})
		if cap != nil {
			cap.Close()
			cap = nil
		}
		head = nil
		last = nil
		latest = nil
		pending = nil
		valid = false
		hasMessages = false
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if engine == nil {
			var err error
			engine, err = ocr.Open(c.models, c.dll)
			if err != nil {
				fail("OCR 初始化失败，请检查模型和运行库") // Retry initialization only on an explicit start.
				for !c.enabled.Load() {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
					}
				}
				continue
			}
			blank := image.NewRGBA(image.Rect(0, 0, 128, 64))
			draw.Draw(blank, blank.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
			if _, err = engine.Read(blank); err != nil {
				engine.Close()
				engine = nil
				fail("OCR 预热失败，请检查模型和运行库")
				continue
			}
			c.send(Event{Kind: "ocr_ready"})
		}
		if !c.enabled.Load() {
			if cap != nil {
				cap.Close()
				cap = nil
			}
			head = nil
			last = nil
			latest = nil
			pending = nil
			area = nil
			valid = false
			hasMessages = false
			retry = time.Time{}
			reconnects = 0
			continue
		}
		if cap == nil {
			var err error
			window, err = FindWeChat()
			if err != nil {
				fail(err.Error())
				continue
			}
			cap, err = OpenWGC(window, c.wgc)
			if err != nil {
				fail(err.Error())
				continue
			}
			started = time.Now()
			c.send(Event{Kind: "ocr_ready"})
			c.send(Event{Kind: "status", Status: "微信窗口已找到，正在等待画面并识别会话标题"})
		}
		if window.RestoreWithoutActivation() {
			started = time.Now()
			c.send(Event{Kind: "status", Status: "已无激活还原微信，正在等待窗口画面"})
			continue
		}
		full, _, err := cap.Snapshot()
		if err != nil {
			fail(err.Error())
			continue
		}
		now := time.Now()
		if full != nil {
			latest = full
			if full.Bounds() != frameBounds {
				frameBounds = full.Bounds()
				area = nil
				last = nil
				head = nil
			}
			if area == nil {
				area = ChatArea(full, 60)
			}
			bounds := full.Bounds()
			if area != nil {
				bounds = image.Rect(area.Header.Min.X, area.Header.Min.Y, area.Chat.Max.X, area.Chat.Max.Y).Intersect(full.Bounds())
			}
			pixels := crop(full, bounds).Pix
			if !bytes.Equal(last, pixels) {
				last = append(last[:0], pixels...)
				if pending == nil {
					firstChange = now
				}
				pending = full
				changed = now
			}
		}
		var settled *image.RGBA
		if pending != nil && (now.Sub(changed) >= 250*time.Millisecond || now.Sub(firstChange) >= time.Second) {
			settled = pending
			pending = nil
		}
		if settled == nil && (!valid || !hasMessages) && !now.Before(retry) {
			settled = latest
			retry = now.Add(time.Second)
		}
		if settled == nil {
			if latest == nil && now.Sub(started) >= 3*time.Second {
				if reconnects >= 2 {
					fail("微信采集未返回画面，自动重连仍失败；请还原微信窗口后重新开启采集")
					continue
				}
				cap.Close()
				cap = nil
				reconnects++
				head = nil
				valid = false
				hasMessages = false
				c.send(Event{Kind: "status", Status: "微信未返回首帧，正在重新连接"})
			}
			continue
		}
		area = ChatArea(settled, 60)
		if area == nil {
			valid = false
			hasMessages = false
			head = nil
			c.send(Event{Kind: "blocked", Status: "消息区未识别，请放大微信窗口"})
			continue
		}
		header := crop(settled, area.Header)
		if head == nil || !bytes.Equal(head, header.Pix) {
			rows, err := engine.Read(header)
			if err != nil {
				fail("标题识别失败，请检查OCR运行库")
				continue
			}
			newTitle := ocr.ReadTitle(rows)
			head = append(head[:0], header.Pix...)
			if newTitle != title {
				hasMessages = false
			}
			title = newTitle
			valid = title != ""
			if !valid {
				head = nil
				c.send(Event{Kind: "blocked", Status: "会话标题未识别，已停止接收消息"})
				continue
			}
			c.send(Event{Kind: "chat", Title: title})
		}
		if !valid {
			continue
		}
		reader := readers[title]
		if reader == nil {
			reader = &ocr.Reader{}
			readers[title] = reader
		}
		chat := crop(settled, area.Chat)
		rows, err := engine.Read(chat)
		if err != nil {
			fail("消息识别失败，请检查OCR运行库")
			continue
		}
		lines := reader.Read(chat, ocr.RGB(area.Background), rows, now)
		hasMessages = len(lines) > 0
		if reader.IncompleteCount > 0 {
			c.send(Event{Kind: "status", Status: "发现气泡被截断或文字未识别完整，暂不保存；请让气泡完整显示后重试"})
		}
		names := map[string]bool{}
		for _, line := range lines {
			if line.Name != "" {
				names[line.Name] = true
			}
		}
		if len(names) > 1 {
			valid = false
			head = nil
			c.send(Event{Kind: "blocked", Status: "检测到发言人昵称；首版仅支持单聊"})
			continue
		}
		if len(names) > 0 {
			c.send(Event{Kind: "status", Status: "识别到昵称，请核对原文并确认当前为单聊后再分析"})
		}
		if c.enabled.Load() {
			fresh := reader.NewLines(lines)
			if len(fresh) > 0 {
				c.send(Event{Kind: "lines", Title: title, Lines: fresh})
			}
		}
	}
}
