package news

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type Row = map[string]interface{}
type Feed struct{ Publisher, URL string }

var DefaultFeeds = []Feed{{"IT之家 · 科技", "https://www.ithome.com/rss/"}, {"机核 · 游戏与文化", "https://www.gcores.com/rss"}}
var tags = regexp.MustCompile(`<[^>]*>`)

type cacheEntry struct {
	at   time.Time
	rows []Row
}
type Reader struct {
	HTTP  *http.Client
	Feeds []Feed
	mu    sync.Mutex
	cache map[string]cacheEntry
}

func Parse(data []byte, publisher string, now time.Time) ([]Row, error) {
	upper := bytes.ToUpper(data)
	if bytes.Contains(upper, []byte("<!DOCTYPE")) || bytes.Contains(upper, []byte("<!ENTITY")) {
		return nil, errors.New("不支持的资讯格式")
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	rows := []Row{}
	count := 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "item" {
			continue
		}
		var item struct {
			Title string `xml:"title"`
			Link  string `xml:"link"`
			Date  string `xml:"pubDate"`
		}
		if err = decoder.DecodeElement(&item, &start); err != nil {
			return nil, err
		}
		count++
		if count > 100 {
			continue
		}
		title := strings.TrimSpace(html.UnescapeString(tags.ReplaceAllString(item.Title, "")))
		runes := []rune(title)
		if len(runes) > 180 {
			title = string(runes[:180])
		}
		link := strings.TrimSpace(item.Link)
		u, err := url.Parse(link)
		if err != nil || title == "" || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
			continue
		}
		published, err := http.ParseTime(item.Date)
		if err != nil || published.Before(now.Add(-72*time.Hour)) || published.After(now.Add(5*time.Minute)) {
			continue
		}
		rows = append(rows, Row{"title": title, "url": link, "publisher": publisher, "published_at": published.Format(time.RFC3339), "retrieved_at": now.UTC().Format(time.RFC3339Nano)})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, _ := time.Parse(time.RFC3339, rows[i]["published_at"].(string))
		b, _ := time.Parse(time.RFC3339, rows[j]["published_at"].(string))
		return a.After(b)
	})
	if len(rows) > 12 {
		rows = rows[:12]
	}
	return rows, nil
}

func clone(rows []Row) []Row {
	result := []Row{}
	for _, row := range rows {
		copy := Row{}
		for k, v := range row {
			copy[k] = v
		}
		result = append(result, copy)
	}
	return result
}

func (r *Reader) fetch(ctx context.Context, feed Feed) ([]Row, error) {
	now := time.Now()
	r.mu.Lock()
	cached, ok := r.cache[feed.URL]
	r.mu.Unlock()
	if ok && now.Sub(cached.at) < 5*time.Minute {
		rows := []Row{}
		for _, row := range cached.rows {
			date, _ := time.Parse(time.RFC3339, row["published_at"].(string))
			if !date.Before(now.Add(-72 * time.Hour)) {
				rows = append(rows, row)
			}
		}
		return clone(rows), nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", feed.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ChatJunshi/0.1 RSS reader")
	base, err := url.Parse(feed.URL)
	if err != nil {
		return nil, err
	}
	client := http.Client{}
	if r.HTTP != nil {
		client = *r.HTTP
	}
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(via) > 10 || next.URL.Scheme != "https" || next.URL.Hostname() != base.Hostname() {
			return errors.New("资讯地址发生变化")
		}
		return nil
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, errors.New("资讯获取失败")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 512*1024+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 512*1024 {
		return nil, errors.New("资讯内容过大")
	}
	rows, err := Parse(data, feed.Publisher, time.Now())
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	if r.cache == nil {
		r.cache = map[string]cacheEntry{}
	}
	r.cache[feed.URL] = cacheEntry{time.Now(), rows}
	r.mu.Unlock()
	return clone(rows), nil
}

func (r *Reader) Recent(ctx context.Context) ([]Row, string) {
	feeds := r.Feeds
	if feeds == nil {
		feeds = DefaultFeeds
	}
	type fetched struct {
		rows []Row
		err  error
	}
	channels := make([]chan fetched, len(feeds))
	for i, feed := range feeds {
		channels[i] = make(chan fetched, 1)
		go func(index int, feed Feed) { rows, err := r.fetch(ctx, feed); channels[index] <- fetched{rows, err} }(i, feed)
	}
	rows := []Row{}
	failed := 0
	seen := map[string]bool{}
	for _, channel := range channels {
		result := <-channel
		if result.err != nil {
			failed++
			continue
		}
		for _, row := range result.rows {
			link := row["url"].(string)
			if !seen[link] && len(rows) < 24 {
				seen[link] = true
				row["id"] = len(rows) + 1
				rows = append(rows, row)
			}
		}
	}
	notice := "近期资讯获取失败或没有新内容，本次仅根据历史与兴趣找话题。"
	if len(rows) > 0 {
		notice = "已获取近三天的科技、游戏与文化资讯；由模型选择相关话题。"
		if failed > 0 {
			notice += "部分资讯源暂不可用。"
		}
	}
	return rows, notice
}
