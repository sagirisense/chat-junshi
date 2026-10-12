import unittest
from unittest.mock import patch
import numpy as np
from app.ocr import Reader, similar, has_unread_text


class OCRCompletenessTests(unittest.TestCase):
    def test_missing_visible_text_rows_are_rejected(self):
        chat = np.full((120, 200, 3), 255, np.uint8)
        chat[30:42, 65:100] = 0
        chat[65:77, 65:100] = 0
        self.assertTrue(has_unread_text(chat, (20, 90), [(60, 110, 28, 44)], np.array([255]*3)))
        self.assertFalse(has_unread_text(chat, (20, 90), [(60, 110, 28, 44), (60, 110, 63, 79)], np.array([255]*3)))
        self.assertFalse(similar('今天忙了一整天', '今天忙了一整天，明天再找你聊天'))

    def read(self, bubbles, boxes):
        chat = np.full((200, 300, 3), 238, np.uint8)
        for top, bottom in bubbles:
            chat[top:bottom, 50:200] = 255
        results = []
        for top, bottom, text in boxes:
            results.append(([[60, top], [160, top], [160, bottom], [60, bottom]], text, 1))
        with patch('app.ocr._engine', return_value=lambda *a, **kw: (results, None)), patch('app.ocr.who_said', return_value=('her', np.array([255]*3), 15)):
            reader = Reader()
            lines = reader.read(chat, np.array([238]*3))
        return reader, lines

    def test_partial_bubbles_at_both_edges_not_saved_or_marked_seen(self):
        reader, lines = self.read([(0, 60), (140, 200)], [(20, 35, '只剩后半段'), (160, 175, '只有前半段')])
        self.assertEqual(lines, [])
        self.assertEqual(reader.incomplete_count, 2)
        self.assertEqual(reader.new_lines(lines), [])
        self.assertEqual(reader.seen, [])

    def test_same_bubble_merges_even_with_large_text_line_spacing(self):
        reader, lines = self.read([(40, 130)], [(55, 70, '完整消息第一行'), (95, 110, '完整消息第二行')])
        self.assertEqual(len(lines), 1)
        self.assertEqual(lines[0][2], '完整消息第一行完整消息第二行')
        self.assertEqual(reader.incomplete_count, 0)

    def test_separate_bubbles_remain_separate(self):
        reader, lines = self.read([(20, 50), (70, 100)], [(30, 45, '第一条'), (80, 95, '第二条')])
        self.assertEqual([line[2] for line in lines], ['第一条', '第二条'])
        self.assertEqual(len(reader.new_lines(lines)), 2)


if __name__ == '__main__':
    unittest.main()
