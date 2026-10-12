import multiprocessing as mp
import queue
import unittest
import itertools
import threading
from unittest.mock import patch
import numpy as np
from app.chatapps import WECHAT
from collector import run


class CollectorTests(unittest.TestCase):
    def test_shutdown_wakes_paused_collector_without_capturing(self):
        enabled = threading.Event()
        shutdown = threading.Event()

        def stop_while_paused():
            shutdown.set()
            enabled.set()

        with patch('app.capture.find_chat_hwnd') as find, patch('app.capture.Capture') as capture, patch.object(enabled, 'wait', side_effect=stop_while_paused):
            run(queue.Queue(), enabled, shutdown=shutdown)
        find.assert_not_called()
        capture.assert_not_called()

    def test_missing_first_frame_reconnects_without_restarting_program(self):
        output = queue.Queue()
        enabled = mp.Event()
        enabled.set()
        frame = np.full((200, 100, 3), 238, np.uint8)
        with patch('app.capture.find_chat_hwnd', return_value=(123, WECHAT)) as find, patch('app.capture.Capture') as capture, patch('app.capture.chat_area', return_value=(0, 40, 100, 160, np.array([238]*3), 0)), patch('app.ocr.read_title', return_value='虚构单聊'), patch('app.ocr.Reader') as reader, patch('ctypes.windll.user32.IsIconic', return_value=False), patch('collector.time.sleep'), patch('collector.time.perf_counter', side_effect=itertools.count()):
            from unittest.mock import MagicMock
            stalled, recovered = MagicMock(), MagicMock()
            capture.side_effect = [stalled, recovered]
            stalled.alive.return_value = True
            stalled.settled.return_value = stalled.snapshot.return_value = None
            recovered.alive.side_effect = [True, False]
            recovered.settled.return_value = frame
            reader.return_value.read.return_value = [('her', None, '虚构正文', 70)]
            reader.return_value.new_lines.return_value = [('her', None, '虚构正文')]
            run(output, enabled)
        self.assertEqual(find.call_count, 2)
        stalled.stop.assert_called_once()
        stalled.wait.assert_called_once()
        self.assertTrue(any(row[0] == 'lines' for row in output.queue))

    def test_repeated_missing_frames_stop_after_two_retries(self):
        output = queue.Queue()
        enabled = mp.Event()
        enabled.set()
        with patch('app.capture.find_chat_hwnd', return_value=(123, WECHAT)), patch('app.capture.Capture') as capture, patch('ctypes.windll.user32.IsIconic', return_value=False), patch('collector.time.sleep'), patch('collector.time.perf_counter', side_effect=itertools.count()):
            capture.return_value.alive.return_value = True
            capture.return_value.settled.return_value = capture.return_value.snapshot.return_value = None
            run(output, enabled)
        self.assertEqual(capture.call_count, 3)
        self.assertTrue(any(row[0] == 'dead' and '自动重连仍失败' in row[1] for row in output.queue))

    def test_static_frame_retries_failed_first_title_without_new_message(self):
        output = queue.Queue()
        enabled = mp.Event()
        enabled.set()
        frame = np.full((200, 100, 3), 238, np.uint8)
        with patch('app.capture.find_chat_hwnd', return_value=(123, WECHAT)), patch('app.capture.Capture') as capture, patch('app.capture.chat_area', return_value=(0, 40, 100, 160, np.array([238]*3), 0)), patch('app.ocr.read_title', side_effect=['', '虚构单聊']) as title, patch('app.ocr.Reader') as reader, patch('ctypes.windll.user32.IsIconic', return_value=False), patch('collector.time.sleep'):
            capture.return_value.alive.side_effect = [True, True, False]
            capture.return_value.settled.side_effect = [frame, None]
            capture.return_value.snapshot.return_value = frame
            reader.return_value.read.return_value = [('her', None, '虚构正文', 70)]
            reader.return_value.new_lines.return_value = [('her', None, '虚构正文')]
            run(output, enabled)
        self.assertEqual(title.call_count, 2)
        self.assertTrue(any(row[0] == 'lines' for row in output.queue))

    def test_prewarm_does_not_find_or_capture_window_before_enabled(self):
        output = queue.Queue()
        enabled = mp.Event()
        with patch('app.capture.find_chat_hwnd') as find, patch('app.capture.Capture') as capture, patch('app.ocr._engine') as engine, patch.object(enabled, 'wait', side_effect=RuntimeError('end synthetic test')):
            run(output, enabled, prewarm=True)
        find.assert_not_called()
        capture.assert_not_called()
        engine.return_value.assert_called_once()
        self.assertTrue(any(row[0] == 'ocr_ready' for row in output.queue))

    def collect(self, names):
        output = queue.Queue()
        enabled = mp.Event()
        enabled.set()
        lines = [('her', name, f'虚构正文{i}', i * 20) for i, name in enumerate(names)]
        with patch('app.capture.find_chat_hwnd', return_value=(123, WECHAT)), \
             patch('app.capture.Capture') as capture, \
             patch('app.capture.chat_area', return_value=(0, 40, 100, 160, np.array([238]*3), 0)), \
             patch('app.ocr.read_title', return_value='虚构单聊'), \
             patch('app.ocr.Reader') as reader, \
             patch('ctypes.windll.user32.IsIconic', return_value=False):
            capture.return_value.alive.side_effect = [True, False]
            capture.return_value.settled.return_value = np.full((200, 100, 3), 238, dtype=np.uint8)
            reader.return_value.read.return_value = lines
            reader.return_value.new_lines.return_value = [(w, n, t) for w, n, t, _ in lines]
            run(output, enabled)
        return list(output.queue)

    def test_one_nickname_does_not_drop_single_chat_messages(self):
        events = self.collect(['虚构昵称'])
        self.assertTrue(any(item[0] == 'lines' and len(item[2]) == 1 for item in events))
        self.assertFalse(any(item[0] == 'blocked' for item in events))

    def test_multiple_senders_remain_blocked(self):
        events = self.collect(['虚构甲', '虚构乙'])
        self.assertTrue(any(item[0] == 'blocked' for item in events))
        self.assertFalse(any(item[0] == 'lines' for item in events))
