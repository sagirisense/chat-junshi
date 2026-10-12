from datetime import datetime
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import numpy as np
from app.ocr import Reader
from message_times import CHINA, parse_label, timing_evidence, infer_missing_times
from sessions import Session

NOW = datetime(2026, 10, 12, 23, 30, tzinfo=CHINA)


class MessageTimeTests(unittest.TestCase):
    def test_unknown_time_uses_neighbor_range_without_exact_timestamp(self):
        rows = [{'id': 1, 'message_time': '2026-10-12T22:37+08:00'},
                {'id': 2, 'message_time': None}, {'id': 3, 'message_time': '2026-10-12T22:45+08:00'}]
        inferred = infer_missing_times(rows)
        self.assertEqual(inferred[1]['inferred_time']['precision'], 'range')
        self.assertEqual(inferred[1]['inferred_time']['source_ids'], [1, 3])
        self.assertIsNone(inferred[1]['message_time'])
        self.assertNotIn('inferred_time', rows[1])
        self.assertEqual(timing_evidence(inferred, NOW)['exact_intervals'], [])

    def test_one_sided_group_anchor_and_explicit_unknown(self):
        rows = [{'id': 1, 'time_context': parse_label('22:37', NOW)}, {'id': 2, 'message_time': None}]
        value = infer_missing_times(rows)[1]['inferred_time']
        self.assertEqual(value['precision'], 'after')
        self.assertTrue(value['earliest'].endswith('22:37+08:00'))
        rows[1]['time_inference_disabled'] = True
        self.assertNotIn('inferred_time', infer_missing_times(rows)[1])

    def test_contradictory_neighbors_do_not_infer_and_corrections_recompute(self):
        rows = [{'id': 1, 'message_time': '2026-10-12T22:45+08:00'},
                {'id': 2, 'message_time': None}, {'id': 3, 'message_time': '2026-10-12T22:37+08:00'}]
        self.assertNotIn('inferred_time', infer_missing_times(rows)[1])
        rows[0]['message_time'] = '2026-10-12T22:30+08:00'
        self.assertTrue(infer_missing_times(rows)[1]['inferred_time']['earliest'].endswith('22:30+08:00'))

    def test_labels_dates_weekdays_and_midnight(self):
        for label, expected in [('昨天 18:30', '2026-10-11T18:30'),
                ('星期五 下午3:05', '2026-10-09T15:05'), ('周日 09:00', '2026-10-11T09:00'),
                ('2025年12月31日 23:59', '2025-12-31T23:59'),
                ('12月31日', '2025-12-31T00:00'), ('00：15', '2026-10-12T00:15')]:
            with self.subTest(label=label):
                parsed = parse_label(label, NOW)
                self.assertTrue(parsed['anchor_time'].startswith(expected))
        self.assertEqual(parse_label('昨天', NOW)['precision'], 'day')
        self.assertEqual(parse_label('14:05', NOW)['precision'], 'group')
        for text in ('明天再说', '昨天18:30吃饭了吗', '25:80', '2月30日 12:00', '半小时前'):
            self.assertIsNone(parse_label(text, NOW))
        self.assertTrue(parse_label('昨天 23:59', datetime(2026, 1, 1, 0, 5, tzinfo=CHINA))['anchor_time'].startswith('2025-12-31'))

    def test_group_anchors_never_become_exact_intervals(self):
        messages = [{'id': 1, 'message_time': None, 'time_context': parse_label('昨天 18:30', NOW)},
                    {'id': 2, 'message_time': None, 'observed_at': NOW.isoformat()}]
        value = timing_evidence(messages, NOW)
        self.assertEqual(value['exact_intervals'], [])
        self.assertNotIn('latest_message_age_minutes', value)
        self.assertEqual(value['current_period'], '深夜')
        messages[0]['message_time'] = '2026-10-12T22:00+08:00'
        messages[1]['message_time'] = '2026-10-12T23:00+08:00'
        value = timing_evidence(messages, NOW)
        self.assertEqual(value['exact_intervals'][0]['minutes'], 60)
        self.assertEqual(value['latest_message_age_minutes'], 30)
        messages[1]['message_time'] = '2026-10-13T23:00+08:00'
        self.assertEqual(timing_evidence(messages, NOW)['exact_intervals'], [])

    def test_time_storage_manual_correction_and_request_clock(self):
        with tempfile.TemporaryDirectory(dir=Path(__file__).parent) as directory:
            session = Session('虚构对象', directory)
            session.append([('her', None, '你好', parse_label('昨天 18:30', NOW))])
            self.assertIsNone(session.messages[0]['message_time'])
            session.correct(0, 'peer', '你好呀')
            self.assertIn('time_context', session.messages[0])
            session.correct(0, 'peer', '你好呀', '2026-10-11T18:31')
            self.assertEqual(session.messages[0]['message_time'], '2026-10-11T18:31+08:00')
            self.assertNotIn('time_context', session.messages[0])
            session.confirmed = True
            with patch('message_times.now_china', return_value=NOW):
                data = json.loads(session.packet().split('\n', 2)[2])
            self.assertEqual(data['timing']['current_time'], NOW.isoformat())
            self.assertEqual(Session('虚构对象', directory).messages[0]['message_time_source'], 'user')
            session.correct(0, 'peer', '你好呀', '')
            self.assertIsNone(session.messages[0]['message_time'])
            with self.assertRaises(ValueError):
                session.correct(0, 'peer', '不能保存', 'not a date')
            with self.assertRaises(ValueError):
                session.correct(0, 'peer', '不能保存', '2026-10-11')
            self.assertEqual(session.messages[0]['text'], '你好呀')

    def test_ocr_centered_label_association_and_bubble_text(self):
        def box(x, y, width=70):
            return [[x, y], [x + width, y], [x + width, y + 18], [x, y + 18]]
        results = [[box(130, 20), '昨天 18:30', 1], [box(40, 60), '你好', 1],
                   [box(40, 100), '昨天 18:30', 1]]
        pane = np.array([238, 238, 238])
        with patch('app.ocr._engine', return_value=lambda *a, **kw: (results, None)), \
             patch('app.ocr.who_said', side_effect=[('gray', pane, 16), ('her', np.array([255]*3), 16), ('her', np.array([255]*3), 16)]), \
             patch('app.ocr.now_china', return_value=NOW):
            reader = Reader()
            lines = reader.read(np.full((150, 330, 3), 238, np.uint8), pane)
            output = reader.new_lines(lines, with_times=True)
        self.assertEqual(len(output), 2)
        self.assertEqual(output[1][2], '昨天 18:30')
        self.assertEqual(output[0][3]['label'], '昨天 18:30')
        self.assertEqual(output[0][3]['precision'], 'group')


if __name__ == '__main__':
    unittest.main()
