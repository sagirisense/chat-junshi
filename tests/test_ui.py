from pathlib import Path
import tempfile
import tkinter as tk
import unittest
from unittest.mock import patch

from main import Window
from test_core import RESULT


class UiTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(dir=Path(__file__).parent)
        self.addCleanup(self.temp.cleanup)
        self.patch = patch('main.ROOT', Path(self.temp.name))
        self.patch.start()
        self.addCleanup(self.patch.stop)
        self.root = tk.Tk()
        self.root.withdraw()
        self.window = Window(self.root)
        self.addCleanup(self.window.close)
        self.window.select('测试A')
        self.window.current.append([('her', None, '测试原文')])
        self.window.current.confirmed = True

    def enqueue(self):
        session = self.window.current
        self.window.results.put((session.title, session.revision, self.window.capture_epoch, RESULT, None))

    def test_new_message_invalidates_pending_result(self):
        self.enqueue()
        self.window.current.append([('her', None, '更新原文')])
        self.window.drain()
        self.assertIn('旧分析已丢弃', self.window.status.get())
        self.assertEqual(str(self.window.cards[0][2]['state']), 'disabled')

    def test_switch_invalidates_identity_and_result(self):
        self.enqueue()
        self.window.select('测试B')
        self.assertFalse(self.window.current.confirmed)
        self.window.drain()
        self.assertIn('旧分析已丢弃', self.window.status.get())

    def test_background_changes_invalidate_result(self):
        self.enqueue()
        self.window.goal.insert(0, '目标改变')
        self.window.drain()
        self.assertIn('旧分析已丢弃', self.window.status.get())

    def test_valid_result_enables_copy(self):
        self.enqueue()
        self.window.drain()
        self.assertEqual(str(self.window.cards[0][2]['state']), 'normal')
        self.assertIn('推荐', self.window.cards[0][0]['text'])


if __name__ == '__main__':
    unittest.main()
