"""Short-lived public RSS cache. Never sends chat text or profiles to news sites."""
from concurrent.futures import ThreadPoolExecutor
from datetime import datetime, timedelta, timezone
from email.utils import parsedate_to_datetime
import html
import re
import threading
import time
from urllib import request, parse
from xml.etree import ElementTree

FEEDS = [('IT之家 · 科技', 'https://www.ithome.com/rss/'),
         ('机核 · 游戏与文化', 'https://www.gcores.com/rss')]
CACHE = {}
LOCK = threading.Lock()


def parse_feed(data, publisher, now=None):
    now = now or datetime.now(timezone.utc)
    if b'<!DOCTYPE' in data.upper() or b'<!ENTITY' in data.upper():
        raise ValueError('不支持的资讯格式')
    root = ElementTree.fromstring(data)
    rows = []
    for item in root.findall('.//item')[:100]:
        title = html.unescape(re.sub('<[^>]*>', '', item.findtext('title', ''))).strip()[:180]
        url = item.findtext('link', '').strip()
        parts = parse.urlsplit(url)
        if not title or parts.scheme != 'https' or not parts.hostname or parts.username or parts.password:
            continue
        try:
            published = parsedate_to_datetime(item.findtext('pubDate', ''))
            if published.tzinfo is None or not now - timedelta(days=3) <= published <= now + timedelta(minutes=5):
                continue
        except (ValueError, TypeError, OverflowError):
            continue
        rows.append({'title': title, 'url': url, 'publisher': publisher,
                     'published_at': published.isoformat(), 'retrieved_at': now.isoformat()})
    return sorted(rows, key=lambda row: row['published_at'], reverse=True)[:12]


def fetch_feed(feed):
    publisher, url = feed
    with LOCK:
        cached = CACHE.get(url)
        if cached and time.monotonic() - cached[0] < 300:
            # Recheck freshness even when a cache spans the three-day cutoff.
            now = datetime.now(timezone.utc)
            return [dict(row) for row in cached[1] if datetime.fromisoformat(row['published_at']) >= now - timedelta(days=3)]
    req = request.Request(url, headers={'User-Agent': 'ChatJunshi/0.1 RSS reader'})
    with request.urlopen(req, timeout=5) as response:
        final = parse.urlsplit(response.geturl())
        if final.scheme != 'https' or final.hostname != parse.urlsplit(url).hostname:
            raise ValueError('资讯地址发生变化')
        data = response.read(512 * 1024 + 1)
    if len(data) > 512 * 1024:
        raise ValueError('资讯内容过大')
    rows = parse_feed(data, publisher)
    with LOCK:
        CACHE[url] = (time.monotonic(), rows)
    return [dict(row) for row in rows]


def recent_news():
    rows, failed = [], 0
    with ThreadPoolExecutor(max_workers=2) as pool:
        futures = [pool.submit(fetch_feed, feed) for feed in FEEDS]
        for future in futures:
            try:
                rows.extend(future.result())
            except Exception:
                failed += 1
    # Public headlines, no personal keyword queries; the model filters relevance.
    unique = {}
    for row in rows:
        unique.setdefault(row['url'], row)
    sources = [dict(row, id=i + 1) for i, row in enumerate(list(unique.values())[:24])]
    notice = '已获取近三天的科技、游戏与文化资讯；由模型选择相关话题。' if sources else '近期资讯获取失败或没有新内容，本次仅根据历史与兴趣找话题。'
    if sources and failed:
        notice += '部分资讯源暂不可用。'
    return sources, notice
