import copy
from datetime import datetime, timezone
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

from assistance import request_instructions, task_schema, validate_task, attach_sources
from codex_bridge import SCHEMA
from desktop_service import Service
from test_core import RESULT
from topic_news import parse_feed, recent_news


def topic_result():
    result = copy.deepcopy(RESULT)
    result['candidates'][0].update(topic_title='摄影', basis='history', evidence_ids=[1],
                                    source_ids=[], preference_level='mentioned')
    return result


class AssistanceTests(unittest.TestCase):
    def test_draft_validation_and_no_change_of_intent_rules(self):
        prompt = request_instructions('polish', '我今晚不想见面', '表达边界')
        self.assertIn('不擅自削弱明确拒绝', prompt)
        self.assertIn('我今晚不想见面', prompt)
        self.assertIn('不执行', prompt)
        for draft, style in [('', '自然一点'), ('a' * 2001, '自然一点'), ('test', 'invalid')]:
            with self.assertRaises(ValueError):
                request_instructions('polish', draft, style)

    def test_topic_schema_and_fake_references_rejected(self):
        self.assertIn('source_ids', task_schema(SCHEMA, 'topics')['properties']['candidates']['items']['required'])
        result = topic_result()
        validate_task(result, 'topics')
        self.assertEqual(attach_sources(result, {1}, [])['candidates'][0]['sources'], [])
        result = topic_result()
        result['candidates'][0]['source_ids'] = [123]
        with self.assertRaises(ValueError):
            attach_sources(result, {1}, [])
        result['candidates'][0]['source_ids'] = []
        result['candidates'][0]['basis'] = 'hotspot'
        with self.assertRaises(ValueError):
            attach_sources(result, {1}, [])
        result['candidates'][0]['basis'] = 'history'
        with self.assertRaises(ValueError):
            attach_sources(result, {2}, [])

    def test_feeds_require_recent_dates_and_safe_source_links(self):
        now = datetime(2026, 10, 12, 12, tzinfo=timezone.utc)
        def item(title, date, url='https://example.com/news'):
            return f'<item><title>{title}</title><link>{url}</link><pubDate>{date}</pubDate></item>'
        rss = '<rss><channel>' + item('recent', 'Mon, 12 Oct 2026 10:00:00 GMT') + item('old', 'Mon, 05 Oct 2026 10:00:00 GMT') + item('missing', '') + item('badlink', 'Mon, 12 Oct 2026 10:00:00 GMT', 'javascript:alert(1)') + '</channel></rss>'
        rows = parse_feed(rss.encode(), 'test', now)
        self.assertEqual([row['title'] for row in rows], ['recent'])
        with self.assertRaises(ValueError):
            parse_feed(b'<!DOCTYPE rss><rss/>', 'test', now)

    def test_news_failure_falls_back_without_claiming_hotspots(self):
        with patch('topic_news.fetch_feed', side_effect=OSError('offline')):
            sources, notice = recent_news()
        self.assertEqual(sources, [])
        self.assertIn('仅根据历史', notice)

    def test_custom_api_topic_schema_and_valid_source_attachment(self):
        import api_bridge
        result = topic_result()
        candidate = result['candidates'][0]
        candidate.update(basis='hotspot', source_ids=[1], preference_level='unknown')
        with patch('api_bridge.send', return_value={'choices': [{'message': {'content': json.dumps(result)}}]}) as send:
            output = api_bridge.analyze_api('虚构历史', {'source': 'custom_openai'}, 'test-model', quick=True, task='topics')
        self.assertIn('source_ids', send.call_args.args[2]['messages'][0]['content'])
        source = {'id': 1, 'title': '虚构资讯', 'url': 'https://example.com/news', 'published_at': '2026-10-12T12:00:00+08:00', 'publisher': 'test'}
        attach_sources(output, {1}, [source])
        self.assertEqual(output['candidates'][0]['sources'], [source])

    def test_unreferenced_history_is_marked_as_exploratory(self):
        result = topic_result()
        result['candidates'][0]['evidence_ids'] = []
        attach_sources(result, {1}, [])
        self.assertEqual(result['candidates'][0]['basis'], 'general')
        self.assertEqual(result['candidates'][0]['preference_level'], 'unknown')


class AssistanceServiceTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(dir=Path(__file__).parent)
        self.addCleanup(self.temp.cleanup)
        self.service = Service(self.temp.name)
        self.service.emit = lambda value: None
        self.addCleanup(self.service.stop)
        self.service.select('虚构对象A')
        self.service.current.append([('her', None, '上周拍了几张照片')])
        self.service.current.confirmed = True

    def test_polish_uses_single_fast_call_and_never_writes_memory(self):
        with patch('desktop_service.threading.Thread') as thread, patch.object(self.service.memory, 'context', return_value={'preference': '摄影'}):
            self.service.dispatch('analyze', {'task': 'polish', 'draft': '我今晚不想见面', 'style': '表达边界'})
        args = thread.call_args.kwargs['args']
        self.assertIn('我今晚不想见面', args[0])
        self.assertFalse(args[5])  # with_memory is false for drafting/topic tasks
        self.assertEqual(args[8:10], ('quick', 'polish'))
        self.assertFalse(self.service.memory.directory.exists())

    def test_topic_sources_are_attached_only_after_validation(self):
        token = (self.service.current.title, self.service.current.revision, self.service.epoch)
        with patch('desktop_service.TRANSPORT.run', return_value=topic_result()), patch('desktop_service.recent_news', return_value=([], 'offline')):
            self.service.analyze_bg('synthetic', token, self.service.subject(), {1}, None, False, mode='quick', task='topics', use_hotspots=True)
        self.assertEqual(self.service.result['task'], 'topics')
        self.assertEqual(self.service.result['candidates'][0]['sources'], [])
        self.assertEqual(self.service.assistance_notice, 'offline')
        self.assertFalse(self.service.memory.directory.exists())

    def test_input_change_discards_running_result_and_foreign_changes_ignored(self):
        token = (self.service.current.title, self.service.current.revision, self.service.epoch)
        self.service.result = copy.deepcopy(RESULT)
        self.service.dispatch('invalidate_assistance', {'title': '其他对象'})
        self.assertIsNotNone(self.service.result)
        self.service.dispatch('invalidate_assistance', {'title': self.service.current.title})
        self.assertIsNone(self.service.result)
        with patch('desktop_service.TRANSPORT.run', return_value=copy.deepcopy(RESULT)):
            self.service.analyze_bg('synthetic', token, self.service.subject(), {1}, None, False, mode='quick', task='polish')
        self.assertIsNone(self.service.result)
        self.assertIn('旧建议已丢弃', self.service.status)


if __name__ == '__main__':
    unittest.main()
