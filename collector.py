"""Reuse Jev's WGC/OCR; reject ambiguous titles and named (group) messages."""
import ctypes
import time
import traceback


def run(out, enabled, prewarm=False):
    cap = None
    try:
        import numpy as np
        from app.capture import Capture, chat_area, find_chat_hwnd
        from app.chatapps import WECHAT
        from app.ocr import Reader, read_title
        ctypes.windll.user32.SetProcessDPIAware()
        if prewarm:
            started = time.perf_counter()
            from app.ocr import _engine
            # Initialize only the model, using a blank image. No window is read.
            _engine(WECHAT)(np.full((64, 128, 3), 255, np.uint8), use_cls=False)
            out.put(('ocr_ready', round(time.perf_counter() - started, 3)))
        readers = {}
        title = ''
        head = None
        valid = False
        has_messages = False
        retry_at = 0.0
        while True:
            if not enabled.is_set():
                if cap:
                    cap.stop()
                    cap = None
                enabled.wait()
                head = None
            if cap is None:
                hwnd, _ = find_chat_hwnd(WECHAT)
                cap = Capture(hwnd)
                out.put(('status', '微信窗口已找到，正在等待画面并识别会话标题'))
            if not cap.alive():
                raise RuntimeError('微信窗口已关闭，请重新开启采集')
            if ctypes.windll.user32.IsIconic(hwnd):
                out.put(('status', '微信已最小化，请手动还原'))
                time.sleep(1)
                continue
            full = cap.settled()
            if full is None and (not valid or not has_messages) and time.perf_counter() >= retry_at:
                full = cap.snapshot()
                retry_at = time.perf_counter() + 1.0
            if full is None:
                time.sleep(.1)
                continue
            area = chat_area(full)
            if area is None:
                valid = False
                has_messages = False
                out.put(('blocked', '消息区未识别，请放大微信窗口'))
                head = None
                time.sleep(1)
                continue
            x0, y0, x1, y1, bg, pane = area
            cap.area = area
            crop = full[pane:y0, x0:x1]
            if head is None or not np.array_equal(head, crop):
                head = crop.copy()
                # Exact titles only. Never fuzzy-merge different contacts.
                previous_title = title
                title = read_title(crop, WECHAT)
                if title != previous_title:
                    has_messages = False
                valid = bool(title)
                if not valid:
                    out.put(('blocked', '会话标题未识别，已停止接收消息'))
                    head = None  # Retry on the next frame.
                    continue
                out.put(('chat', title))
            if not valid:
                continue
            reader = readers.setdefault(title, Reader(WECHAT))
            lines = reader.read(full[y0:y1, x0:x1], bg)
            has_messages = bool(lines)
            if reader.incomplete_count:
                out.put(('status', '发现气泡被截断或文字未识别完整，暂不保存；请让气泡完整显示后重试'))
            names = {name for _, name, _, _ in lines if name}
            # One visible name can occur in a single chat; it does not prove a group.
            if len(names) > 1:
                out.put(('blocked', '检测到发言人昵称；首版仅支持单聊'))
                head = None
                valid = False
                time.sleep(1)
                continue
            if names:
                out.put(('status', '识别到昵称，请核对原文并确认当前为单聊后再分析'))
            new = reader.new_lines(lines, with_times=True)
            if new and enabled.is_set():
                out.put(('lines', title, new))
    except Exception as exc:
        # Show location and exception type without recording screenshot/chat content.
        location = traceback.extract_tb(exc.__traceback__)[-1]
        detail = f'{location.filename.rsplit(chr(92), 1)[-1]}:{location.lineno}'
        message = str(exc) if isinstance(exc, (TypeError, ImportError)) else '请检查微信窗口和采集依赖'
        out.put(('dead', f'采集不可用：{type(exc).__name__} ({detail}) {message}'))
    finally:
        if cap:
            cap.stop()
