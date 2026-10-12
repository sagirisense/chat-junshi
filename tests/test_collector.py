import multiprocessing as mp
import queue
import unittest
from unittest.mock import patch
import numpy as np
from app.chatapps import WECHAT
from collector import run


class CollectorTests(unittest.TestCase):
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
