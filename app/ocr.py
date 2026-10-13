# -*- coding: utf-8 -*-
"""消息区截图 → 谁说了什么。RapidOCR 吃 numpy，不落盘。"""
import difflib
import re
import time

import numpy as np
from rapidocr_onnxruntime import RapidOCR

from app.chatapps import DEFAULT, ChatApp
from message_times import parse_label, now_china

_ENGINES: dict = {}   # backend name -> engine, shared per process
_FELL_BACK = set()    # backends that failed once; don't retry every frame


def _engine(app: ChatApp = DEFAULT):
    """OCR 引擎全进程共用：一个实例 ~40MB，每个会话一个 Reader，不能各带一个。
    det_limit_type 默认 'min' 会把小图放大到短边 736，裁小反而更慢；必须 'max'。

    RapidOCR 的内置模型只有中英文，韩日文全是乱码，所以这类界面走 Windows 自带 OCR
    （离线、随系统语言包装好，见 app/ocr_windows.py）；没有对应识别器就退回 RapidOCR。"""
    want = app.ocr if app.ocr not in _FELL_BACK else "rapidocr"
    if want not in _ENGINES:
        if want == "windows":
            try:
                from app.ocr_windows import WindowsOcr
                _ENGINES[want] = WindowsOcr()
            except Exception:
                _FELL_BACK.add("windows")
                return _engine(app)
        else:
            _ENGINES[want] = RapidOCR(intra_op_num_threads=4, det_limit_type="max",
                                      det_limit_side_len=4000)
    return _ENGINES[want]


def read_title(header, app: ChatApp = DEFAULT):
    """面板头部那一条截图 → 会话名（numpy RGB）。取最靠上的一行，同一行里取最左的
    （右边是图标按钮，OCR 不出字；下面那行是公告）。群聊的成员数「(422)」去掉，只留名字当 key。
    认不出返回 ""。一次约 60ms，所以调用方只在头部像素变了时才问。"""
    res, _ = _engine(app)(header, use_cls=False)
    if not res:
        return ""
    first = min(res, key=lambda r: r[0][0][1])
    row = first[0][0][1] + (first[0][2][1] - first[0][0][1])  # 框底：顶在这之上的算同一行
    text = min((r for r in res if r[0][0][1] < row), key=lambda r: r[0][0][0])[1]
    return re.sub(r"\s*[（(]\d+[)）]\s*$", "", text.strip())


def who_said(chat, box, app: ChatApp = DEFAULT):
    """按 OCR 框里的颜色分类，不看 x 坐标。返回 (谁, 底色, 墨高)：
    先看底色平不平：框里众数颜色占比 <45% 就是图片（头像/照片/表情包）里的字 → None 丢掉。
    绿底 → me；非绿且文字对底色对比度 ≥150 → her；其余（引用块、群里的发言人名、时间戳、系统提示、
    链接卡片描述——都是灰字，对比度 80~95）→ "gray"。
    实测：气泡正文对比度 178~208，me 绿泡 142~150，灰字 ≤ 93。深浅主题都靠这套。
    哪种底色算 "me" 由 app.is_me 决定（微信绿 / KakaoTalk 黄）。
    墨高 = 框里最长一段连续有字的行数（OCR 框对小字有固定 padding、还会蹭到上下行，不能拿框高比大小）。"""
    xs, ys = [p[0] for p in box], [p[1] for p in box]
    reg = chat[int(min(ys)):int(max(ys)), int(min(xs)):int(max(xs))].astype(int)
    if reg.size == 0:
        return None, None, 0
    vals, cnt = np.unique(reg.reshape(-1, 3), axis=0, return_counts=True)
    bg = vals[cnt.argmax()]
    if cnt.max() / reg.shape[0] / reg.shape[1] < 0.45:
        # 文字必须落在平底色上：WGC 帧是精确像素，气泡/面板里众数颜色占 0.56~0.82，
        # 头像/照片/表情包里只有 0.1~0.3——那是图片里的字（头像上的「借仲夏夜之梦」之类），不是消息。
        # ponytail: 只对精确像素的帧成立；缩放/压缩过的截图（比如拿预览窗再截一次的图）底色会糊成几百种颜色，全会被当图片。
        return None, bg, 0
    diff = np.abs(reg @ [0.299, 0.587, 0.114] - bg @ [0.299, 0.587, 0.114])
    ink_h = best = 0
    for r in (diff > 60).any(axis=1):
        best = best + 1 if r else 0
        ink_h = max(ink_h, best)
    if app.is_me(bg):  # 自己的气泡：微信是绿色，KakaoTalk 是 #FEE500（见 app/chatapps.py）
        return "me", bg, ink_h
    return ("her" if diff.max() >= 150 else "gray"), bg, ink_h


def similar(a, b):
    """同一段像素挪个位置 OCR 会抖（「傻逼了」↔「傻逼」、「不好意思」↔「不好竟思」），按相似度判同一条。"""
    if a != b and (a in b or b in a) and abs(len(a) - len(b)) >= 4:
        return False  # A fuller OCR result must not be swallowed as a known fragment.
    if a == b or difflib.SequenceMatcher(None, a, b).ratio() >= 0.75:
        return True
    return len(a) == len(b) >= 3 and sum(x != y for x, y in zip(a, b)) <= 1  # 短句错一个字


def bubble_bounds(chat, box, bg):
    """Find the continuous bubble background around text, including clipped edges."""
    H, W = chat.shape[:2]
    xs, ys = [p[0] for p in box], [p[1] for p in box]
    left, right = max(0, int(min(xs))), min(W, int(max(xs)))
    if right <= left:
        return None
    band = chat[:, left:right].astype(np.int16)
    rows = (np.abs(band - bg).sum(-1) <= 8).mean(1) > .15
    middle = max(0, min(H - 1, int((min(ys) + max(ys)) / 2)))
    if not rows[middle]:
        return None
    top = bottom = middle
    while top > 0 and rows[top - 1]:
        top -= 1
    while bottom < H - 1 and rows[bottom + 1]:
        bottom += 1
    return top, bottom


def has_unread_text(chat, bounds, spans, bg):
    """Reject a complete bubble when visible text rows were missed by OCR."""
    if not bounds:
        return False
    left = max(0, min(span[0] for span in spans))
    right = min(chat.shape[1], max(span[1] for span in spans))
    top, bottom = bounds
    region = chat[top:bottom + 1, left:right].astype(np.int16)
    if not region.size:
        return False
    contrast = np.abs((region - bg) @ np.array([.299, .587, .114]))
    ink = (contrast > 80).sum(1) >= 2
    for _, _, a, b in spans:
        ink[max(0, int(a) - top - 4):min(len(ink), int(b) - top + 5)] = False
    longest = run = 0
    for present in ink:
        run = run + 1 if present else 0
        longest = max(longest, run)
    return longest >= 5


class Reader:
    """一个会话一个 Reader：lh/seen 各自算各自的，切走再切回来不会把旧消息当新的重报一遍。"""

    def __init__(self, app: ChatApp = DEFAULT):
        self.app = app
        self.ocr = _engine(app)
        self.lh = None  # 正常气泡字高，头一帧定
        self.seen = []  # [(who, name, text)]，累计，封顶 500
        self.last_boxes = []  # 调试视图用：[(x0,y0,x1,y1,kind,text)]，消息区裁剪坐标
        self.last_ms = 0  # 上一帧 OCR 耗时
        self.line_times = {}
        self.incomplete_count = 0

    def read(self, chat, pane_bg):
        """→ [(who, name, text, y)]，同一气泡的多行已合并。who ∈ me/her；name 群聊里是发言人，单聊 None。
        顺带把每个框的分类记进 self.last_boxes（调试视图画框用，几十个 tuple，不开也不亏）。"""
        skip = self.app.trim_top(chat)   # 群聊置顶公告等：不是消息，整块跳过
        if skip:
            chat = chat[skip:]
        t0 = time.perf_counter()
        res, _ = self.ocr(chat, use_cls=False)
        self.last_ms = int((time.perf_counter() - t0) * 1000)
        self.last_boxes = []
        self.incomplete_count = 0
        W = chat.shape[1]
        # 群聊：每条 her 气泡上方一行灰色发言人名（靠左、短、不带冒号、印在面板底色上），从上往下扫，名字带给后面的气泡。
        # 引用块/时间戳/公告带冒号，链接卡片灰字印在气泡底色上，都不会被当成名字。
        # ponytail: 名字行被 OCR 漏掉时会挂到上一个人头上。
        name, raw = None, []
        anchors = []
        observed = now_china()
        for box, text, _ in sorted(res or [], key=lambda r: r[0][0][1]):
            kind, bg, h = who_said(chat, box, self.app)
            xs, ys = [p[0] for p in box], [p[1] for p in box]
            rect = (int(min(xs)), int(min(ys)), int(max(xs)), int(max(ys)))
            if bg is None:
                continue
            # 印在面板底色上的字都不是消息：气泡自带底色。微信的发言人名是灰字，
            # KakaoTalk 的是深色（对比度跟正文一样），所以不能只看 kind == "gray"。
            on_pane = np.abs(bg - pane_bg).sum() <= 6
            # Only centered text on the conversation background can be a time label.
            # A message saying "昨天 18:30" must remain message text.
            if on_pane and 0.30 * W <= (min(xs) + max(xs)) / 2 <= 0.70 * W:
                stamp = parse_label(text, observed)
                if stamp:
                    # OCR sometimes splits "昨天" and "18:30" into adjacent boxes.
                    combined = None
                    if anchors and abs(anchors[-1][0] - max(ys)) <= 5:
                        previous = anchors[-1][1]['label']
                        combined = parse_label(previous + ' ' + text, observed) or parse_label(text + ' ' + previous, observed)
                    if combined:
                        anchors[-1] = (max(anchors[-1][0], max(ys)), combined)
                    else:
                        anchors.append((max(ys), stamp))
                    name = None
                    self.last_boxes.append(rect + ('time', text))
                    continue
            if kind == "gray" or (on_pane and kind == "her"):
                taken = bool(on_pane and box[0][0] < 0.25 * W and len(text) <= 16
                             and not re.search("[:：]", text))
                if taken:
                    name = text
                self.last_boxes.append(rect + ("name" if taken else "gray", text))
                continue
            if kind is None or (self.lh and h < 0.6 * self.lh):
                # 字比正常气泡小得多 = 图片消息（截图/表情包）里的字，不是气泡
                self.last_boxes.append(rect + ("image" if kind is None else "tiny", text))
                continue
            self.last_boxes.append(rect + (kind, text))
            bounds = bubble_bounds(chat, box, bg)
            clipped = min(ys) <= 2 or max(ys) >= chat.shape[0] - 2
            clipped = clipped or bool(bounds and (bounds[0] <= 1 or bounds[1] >= chat.shape[0] - 2))
            raw.append((kind, name if kind == "her" else None, text, min(ys), max(ys), h, bounds, clipped, (int(min(xs)), int(max(xs)), min(ys), max(ys)), bg))
        if not self.lh and len(raw) >= 3:
            self.lh = float(np.median([r[5] for r in raw]))
        # 同一气泡的多行合并：同人、上一行底到这一行顶的间距不到半个字高（不同气泡之间至少隔一个字高）
        lines = []
        for who, nm, text, top, bottom, h, bounds, clipped, span, bg in raw:
            same_bubble = bool(lines and bounds and lines[-1][5] and abs(bounds[0] - lines[-1][5][0]) <= 3 and abs(bounds[1] - lines[-1][5][1]) <= 3)
            if lines and lines[-1][0] == who and lines[-1][1] == nm and (same_bubble or (not bounds and not lines[-1][5] and top - lines[-1][4] < 0.6 * (self.lh or h))):
                lines[-1][2] += self.app.join + text
                lines[-1][4] = bottom
                lines[-1][6] |= clipped
                lines[-1][7].append(span)
            else:
                lines.append([who, nm, text, top, bottom, bounds, clipped, [span], bg])
        for line in lines:
            line[6] |= has_unread_text(chat, line[5], line[7], line[8])
        self.incomplete_count = sum(bool(line[6]) for line in lines)
        lines = [line for line in lines if not line[6]]
        self.line_times = {}
        for w, n, t, y, *_ in lines:
            prior = [stamp for bottom, stamp in anchors if bottom < y]
            self.line_times[(w, n, t)] = prior[-1] if prior else None
        return [(w, n, t, y) for w, n, t, y, *_ in lines]

    def new_lines(self, lines, with_times=False):
        """去重（滚动不重复）→ 这一帧里真正新出现的 [(who, name, text)]。
        本帧有已知行时只要已知行下方的：往上滚翻出来的旧消息在已知行上方，不算。
        本帧一行已知的都没有（大图把旧文字全顶出去了、切了聊天、滚远了）：全算，宁可多算不能漏。
        重复文字按出现顺序匹配；旧消息上方的历史内容不追加。"""
        # Match old occurrences in order: a later repeated short reply must not
        # move the old-message boundary below genuinely new messages.
        cursor = 0
        known_y = []
        for who, _, text, y in lines:
            for index in range(cursor, len(self.seen)):
                old_who, _, old_text = self.seen[index]
                if old_who == who and similar(old_text, text):
                    known_y.append(y)
                    cursor = index + 1
                    break
        floor = max(known_y) if known_y else -1
        new = [(w, n, t) for w, n, t, y in lines if y > floor]
        self.seen.extend(new)
        del self.seen[:-500]
        return [(w, n, t, self.line_times.get((w, n, t))) for w, n, t in new] if with_times else new

    def _seen(self, who, name, text):
        # 名字不参与判重：名字行滚出画面后同一条消息会从 her(LO) 变成 her，不能算新消息
        return any(w == who and similar(t, text) for w, _, t in self.seen)
