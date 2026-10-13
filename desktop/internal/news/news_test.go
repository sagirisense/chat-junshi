package news

import (
	"testing"
	"time"
)

func TestFeedFreshnessAndSafeLinks(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	data := []byte(`<rss><channel><item><title><![CDATA[<b>虚构</b> &amp; 标题]]></title><link>https://example.com/new</link><pubDate>Tue, 13 Oct 2026 10:00:00 GMT</pubDate></item><item><title>过期</title><link>https://example.com/old</link><pubDate>Fri, 09 Oct 2026 10:00:00 GMT</pubDate></item><item><title>不安全</title><link>http://example.com/</link><pubDate>Tue, 13 Oct 2026 10:00:00 GMT</pubDate></item><item><title>未来</title><link>https://example.com/future</link><pubDate>Tue, 13 Oct 2026 14:00:00 GMT</pubDate></item></channel></rss>`)
	rows, err := Parse(data, "虚构来源", now)
	if err != nil || len(rows) != 1 || rows[0]["title"] != "虚构 & 标题" {
		t.Fatal(rows, err)
	}
	for _, bad := range []string{`<!DOCTYPE rss><rss/>`, `<!ENTITY foo "bar"><rss/>`, `<rss>`} {
		if _, err := Parse([]byte(bad), "测试", now); err == nil {
			t.Fatal("accepted invalid feed")
		}
	}
}
