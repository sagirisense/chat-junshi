"""Reuse Jev's WGC/OCR; reject ambiguous titles and named (group) messages."""
import ctypes
import time
import traceback


def run(out, enabled):
    cap = None
    try:
        import numpy as np
        from app.capture import Capture, chat_area, find_chat_hwnd
        from app.chatapps import WECHAT
        from app.ocr import Reader, read_title
        ctypes.windll.user32.SetProcessDPIAware()
        hwnd, _ = find_chat_hwnd(WECHAT)
        readers = {}
        title = ''
        head = None
        valid = False
        while True:
            if not enabled.is_set():
                if cap:
                    cap.stop()
                    cap = None
                enabled.wait()
                head = None
            if cap is None:
                cap = Capture(hwnd)
            if not cap.alive():
                raise RuntimeError('微信窗口已关闭，请重新开启采集')
            if ctypes.windll.user32.IsIconic(hwnd):
                out.put(('status', '微信已最小化，请手动还原'))
                time.sleep(1)
                continue
            full = cap.settled()
            if full is None:
                time.sleep(.1)
                continue
            area = chat_area(full)
            if area is None:
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
                title = read_title(crop, WECHAT)
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
            names = {name for _, name, _, _ in lines if name}
            # One visible name can occur in a single chat; it does not prove a group.
            if len(names) > 1:
                out.put(('blocked', '检测到发言人昵称；首版仅支持单聊'))
                head = None
                time.sleep(1)
                continue
            if names:
                out.put(('status', '识别到昵称，请核对原文并确认当前为单聊后再分析'))
            new = reader.new_lines(lines)
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
