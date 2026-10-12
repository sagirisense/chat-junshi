import copy
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

from desktop_service import Service
from test_core import RESULT


class MemoryServiceTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(dir=Path(__file__).parent)
        self.addCleanup(self.temp.cleanup)
        self.service = Service(self.temp.name)
        self.service.emit = lambda value: None
        self.addCleanup(self.service.stop)
        self.service.select('虚构对象A')

    def test_delete_session_removes_saved_selection_and_preserves_memory(self):
        self.enable()
        self.service.apply_profile({'peer_label': '虚构甲'}, True)
        self.service.current.save()
        path = self.service.current.path
        with self.assertRaises(ValueError):
            self.service.dispatch('delete_session', {})
        self.service.dispatch('delete_session', {'confirm': True})
        self.assertFalse(path.exists())
        self.assertIsNone(self.service.current)
        self.assertNotIn('虚构对象A', self.service.sessions)
        restored = Service(self.temp.name)
        self.addCleanup(restored.stop)
        self.assertNotIn('虚构对象A', restored.sessions)
        restored.select('虚构对象A')
        self.assertEqual(restored.profile.get('peer_label'), '虚构甲')

    def test_export_profile_excludes_chat_and_does_not_write_memory(self):
        self.service.apply_profile({'user_mbti': 'INTP', 'peer_label': '虚构甲'}, False)
        self.service.current.append([('her', None, '不应导出的聊天文字')])
        exported = self.service.dispatch('export_profile', {})['text']
        self.assertEqual(json.loads(exported)['profile']['user_mbti'], 'INTP')
        self.assertNotIn('不应导出的聊天文字', exported)
        self.assertFalse(self.service.memory.directory.exists())

    def test_import_chat_roundtrip_append_identity_and_invalid_file(self):
        self.service.current.append([('her', None, '导入测试')])
        text = self.service.dispatch('export', {})['text']
        prepared = self.service.dispatch('prepare_import_chat', {'text': text})
        self.assertEqual(prepared['count'], 1)
        self.assertEqual(len(self.service.current.messages), 1)
        with self.assertRaises(ValueError):
            self.service.dispatch('import_chat', {'text': text})
        self.service.dispatch('import_chat', {'text': text, 'confirm': True})
        self.assertEqual([r['id'] for r in self.service.current.messages], [1, 2])
        self.assertFalse(self.service.current.confirmed)
        with self.assertRaises(ValueError):
            self.service.dispatch('import_chat', {'text': '{}', 'confirm': True})
        self.assertEqual(len(self.service.current.messages), 2)

    def test_import_profile_prepares_without_writing_or_changing_current(self):
        text = json.dumps({'version': 1, 'type': 'relationship-profile', 'title': '虚构B',
                           'profile': {'user_mbti': 'INTP', 'user_score': 0}})
        draft = self.service.dispatch('prepare_import_profile', {'text': text})
        self.assertEqual(draft['profile']['user_score'], '0')
        self.assertEqual(self.service.current.title, '虚构对象A')
        self.assertFalse(any(self.service.profile.values()))
        self.assertFalse(self.service.memory.directory.exists())
        invalid = json.loads(text)
        invalid['profile']['user_score'] = 101
        with self.assertRaises(ValueError):
            self.service.dispatch('prepare_import_profile', {'text': json.dumps(invalid)})

    def enable(self):
        self.service.dispatch('memory', {'operation': 'enable', 'consent': True})

    def test_no_consent_no_persistent_profile(self):
        self.service.apply_profile({'peer_label': '虚构甲', 'relationship': '朋友'}, False)
        self.assertFalse(self.service.memory.directory.exists())
        self.service.current.append([('her', None, '虚构消息')])
        saved = self.service.current.path.read_text(encoding='utf-8')
        self.assertNotIn('虚构甲', saved)
        self.assertNotIn('朋友', saved)
        with self.assertRaises(ValueError):
            self.service.apply_profile({'peer_label': '甲'}, True)

    def test_enable_requires_explicit_consent(self):
        with self.assertRaises(ValueError):
            self.service.dispatch('memory', {'operation': 'enable'})
        self.assertFalse(self.service.memory.directory.exists())

    def test_saved_profile_restores_without_capture_after_restart(self):
        self.enable()
        self.service.current.save()
        self.service.apply_profile({'peer_label': '虚构甲', 'relationship': '朋友'}, True)
        restarted = Service(self.temp.name)
        self.addCleanup(restarted.stop)
        self.assertEqual(restarted.current.title, '虚构对象A')
        self.assertEqual(restarted.profile['peer_label'], '虚构甲')
        self.assertEqual(restarted.profile['relationship'], '朋友')
        self.assertFalse(restarted.enabled.is_set())
        self.assertFalse(restarted.current.confirmed)

    def test_profile_can_bind_without_running_capture(self):
        service = Service(Path(self.temp.name) / 'another')
        self.addCleanup(service.stop)
        service.emit = lambda value: None
        service.dispatch('bind_profile', {'title': '虚构单聊'})
        service.apply_profile({'peer_label': '虚构用户'}, False)
        self.assertEqual(service.profile['peer_label'], '虚构用户')
        self.assertFalse(service.enabled.is_set())

    def test_reply_goal_changes_without_rebuilding_or_persisting_profile(self):
        self.enable()
        self.service.apply_profile({'peer_label': '虚构甲', 'goal': '确认关系'}, True)
        before = self.service.current.revision
        self.service.dispatch('goal', {'goal': '先聊聊电影，不急着邀约'})
        self.assertEqual(self.service.profile['peer_label'], '虚构甲')
        self.assertEqual(self.service.profile['goal'], '先聊聊电影，不急着邀约')
        self.assertGreater(self.service.current.revision, before)
        saved_goal = next(row for row in self.service.memory.rows(self.service.subject()) if row['field'] == 'goal')
        self.assertEqual(saved_goal['value'], '确认关系')
        with self.assertRaises(ValueError):
            self.service.set_goal('x' * 201)

    def test_memory_failure_does_not_block_chat_snapshot(self):
        self.service.current.append([('her', None, '虚构采集消息')])
        with patch.object(self.service.memory, 'status', side_effect=RuntimeError('unavailable')):
            snapshot = self.service.snapshot()
        self.assertEqual(snapshot['messages'][0]['text'], '虚构采集消息')
        self.assertTrue(snapshot['memory']['unavailable'])

    def test_object_isolation_and_pause(self):
        self.enable()
        self.service.apply_profile({'peer_label': '虚构甲', 'user_mbti': 'INFP'}, True)
        first = self.service.subject()
        self.service.select('虚构对象B')
        self.service.apply_profile({'peer_label': '虚构乙'}, True)
        context = str(self.service.memory.context(self.service.subject()))
        self.assertIn('虚构乙', context)
        self.assertIn('INFP', context)
        self.assertNotIn('虚构甲', context)
        self.service.dispatch('memory', {'operation': 'pause'})
        self.assertIsNone(self.service.memory.context(first))
        with self.assertRaises(ValueError):
            self.service.apply_profile({'peer_label': '丙'}, True)

    def test_unknown_scores_and_invalid_scores(self):
        self.enable()
        self.service.apply_profile({'user_score': '', 'peer_mbti': ''}, True)
        self.assertEqual(self.service.memory.rows(self.service.subject()), [])
        with self.assertRaises(ValueError):
            self.service.apply_profile({'user_score': '101'}, True)

    def test_memory_update_must_reference_current_evidence(self):
        self.enable()
        self.service.current.append([('her', None, '虚构消息')])
        result = copy.deepcopy(RESULT)
        result['memory_updates'] = [{'scope': 'event', 'field': 'key_event', 'value': '无证据',
                                    'evidence_ids': [999], 'confidence': 'high'}]
        token = (self.service.current.title, self.service.current.revision, self.service.epoch)
        with patch('desktop_service.analyze', return_value=result):
            self.service.analyze_bg('synthetic', token, self.service.subject(), {1}, None, True)
        self.assertEqual(self.service.memory.rows(self.service.subject()), [])

    def test_valid_hypothesis_stays_hypothesis_and_undo(self):
        self.enable()
        self.service.current.append([('her', None, '虚构消息')])
        result = copy.deepcopy(RESULT)
        result['memory_updates'] = [{'scope': 'hypothesis', 'field': 'intent', 'value': '可能需要休息',
                                    'evidence_ids': [1], 'confidence': 'low'}]
        token = (self.service.current.title, self.service.current.revision, self.service.epoch)
        with patch('desktop_service.analyze', return_value=result):
            self.service.analyze_bg('synthetic', token, self.service.subject(), {1}, None, True)
        row = self.service.memory.rows(self.service.subject())[0]
        self.assertEqual(row['scope'], 'hypothesis')
        self.assertEqual(row['confidence'], 'low')
        self.assertIn('记忆更新', self.service.memory_notice)
        self.service.dispatch('memory', {'operation': 'undo'})
        self.assertEqual(self.service.memory.rows(self.service.subject()), [])

    def test_revoke_delete_requires_confirmation(self):
        self.enable()
        self.service.apply_profile({'peer_label': '虚构甲'}, True)
        with self.assertRaises(ValueError):
            self.service.dispatch('memory', {'operation': 'delete_all'})
        self.service.dispatch('memory', {'operation': 'delete_all', 'confirm': True})
        self.assertFalse(self.service.memory.status()['consent_enabled'])
        self.assertEqual(self.service.memory.rows(self.service.subject()), [])


if __name__ == '__main__':
    unittest.main()
