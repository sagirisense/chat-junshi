import copy
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

from sessions import Session
from codex_bridge import analyze, validate_result


RESULT = {'summary': '先承接感受', 'facts': ['[#1] 对方说很累'], 'hypotheses': [],
          'unknowns': ['疲惫原因'], 'recommended_index': 0, 'recommendation_reason': '压力较低',
          'candidates': [{'text': '今天辛苦了，先休息一下', 'pros': ['承接感受'],
                          'cons': ['未推进话题'], 'suitable_when': '对方疲惫', 'next_step': '等对方回复'}]}


class StoreTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(dir=Path(__file__).parent)
        self.addCleanup(self.temp.cleanup)
        self.session = Session('对象A', self.temp.name)

    def test_identity_gate_and_unknown_time(self):
        self.session.append([('her', None, '今天好累')])
        with self.assertRaises(ValueError):
            self.session.packet()
        self.session.confirmed = True
        packet = self.session.packet()
        self.assertIn('$goutoujunshi', packet)
        self.assertIn('不可信数据', packet)
        self.assertIsNone(self.session.messages[0]['message_time'])
        self.assertEqual(self.session.messages[0]['sender'], 'peer')

    def test_isolation_and_reconfirmation(self):
        self.session.append([('me', None, '你好')])
        self.session.confirmed = True
        other = Session('对象B', self.temp.name)
        self.assertEqual(other.messages, [])
        loaded = Session('对象A', self.temp.name)
        self.assertEqual(loaded.messages[0]['text'], '你好')
        self.assertFalse(loaded.confirmed)

    def test_transient_windows_lock_does_not_lose_messages(self):
        replace = Path.replace
        calls = []
        def temporarily_locked(path, target):
            calls.append(path)
            if len(calls) == 1:
                exc = PermissionError('simulated Windows sharing lock')
                exc.winerror = 32
                raise exc
            return replace(path, target)
        with patch('sessions.Path.replace', temporarily_locked):
            self.session.append([('her', None, '锁释放后保存')])
        loaded = Session('对象A', self.temp.name)
        self.assertEqual(loaded.messages[0]['text'], '锁释放后保存')

    def test_correct_and_context_limit(self):
        self.session.append([('her', None, str(i)) for i in range(40)])
        previous = self.session.revision
        self.session.correct(39, 'me', '改正后的正文')
        self.assertGreater(self.session.revision, previous)
        self.session.confirmed = True
        evidence = json.loads(self.session.packet().split('\n', 2)[2])
        self.assertEqual(len(evidence['messages']), 30)
        self.assertEqual(evidence['messages'][-1]['sender'], 'me')
        self.assertEqual(evidence['messages'][0]['id'], 11)


class ResultTests(unittest.TestCase):
    def test_valid_and_invalid(self):
        self.assertEqual(validate_result(copy.deepcopy(RESULT)), RESULT)
        for key, value in [('recommended_index', 3), ('recommended_index', True),
                           ('candidates', []), ('facts', 'not a list')]:
            result = copy.deepcopy(RESULT)
            result[key] = value
            with self.subTest(key=key, value=value), self.assertRaises(ValueError):
                validate_result(result)

    def test_protocol_skill_and_final(self):
        class FakeServer:
            instance = None
            def __init__(self):
                FakeServer.instance = self
                self.calls = []
                self.closed = False
            def request(self, method, params, deadline):
                self.calls.append((method, params))
                if method == 'model/list':
                    return {'data': [{'model': 'available-model', 'isDefault': True}]}
                return {'thread': {'id': 'thread'}} if method == 'thread/start' else {'turn': {'id': 'turn'}}
            def send(self, *args, **kwargs):
                pass
            def receive(self, deadline):
                if not hasattr(self, 'sent'):
                    self.sent = True
                    return {'method': 'item/completed', 'params': {'threadId': 'thread', 'turnId': 'turn',
                            'item': {'type': 'agentMessage', 'text': json.dumps(RESULT)}}}
                return {'method': 'turn/completed', 'params': {'threadId': 'thread',
                        'turn': {'id': 'turn', 'status': 'completed'}}}
            def close(self):
                self.closed = True
        with tempfile.TemporaryDirectory(dir=Path(__file__).parent) as directory:
            skill = Path(directory) / 'SKILL.md'
            skill.write_text('test', encoding='utf-8')
            with patch('codex_bridge.AppServer', FakeServer):
                self.assertEqual(analyze('synthetic', skill), RESULT)
        server = FakeServer.instance
        self.assertTrue(server.closed)
        self.assertEqual(server.calls[2][1]['sandbox'], 'read-only')
        self.assertTrue(server.calls[2][1]['ephemeral'])
        self.assertEqual(server.calls[2][1]['model'], 'available-model')
        self.assertEqual(server.calls[3][1]['input'][1]['name'], 'goutoujunshi')


class CodexCommandTests(unittest.TestCase):
    def test_explorer_launch_resolves_fnm_native_binary_without_path(self):
        from codex_bridge import command
        with tempfile.TemporaryDirectory(dir=Path(__file__).parent) as directory:
            root = Path(directory)
            prefix = root / 'fnm/node-versions/v16/installation'
            prefix.mkdir(parents=True)
            (prefix / 'codex.cmd').write_text('test', encoding='utf-8')
            package = prefix / 'node_modules/@openai/codex'
            script = package / 'bin/codex.js'
            script.parent.mkdir(parents=True); script.write_text('test', encoding='utf-8')
            binary = package / 'node_modules/@openai/codex-win32-x64/vendor/x86_64-pc-windows-msvc/bin/codex.exe'
            binary.parent.mkdir(parents=True); binary.write_bytes(b'test')
            with patch('codex_bridge.shutil.which', return_value=None), patch.dict('os.environ', {'APPDATA':str(root), 'LOCALAPPDATA':str(root)}), patch('codex_bridge.platform.machine', return_value='AMD64'):
                self.assertEqual(command(), [str(binary), 'app-server'])


if __name__ == '__main__':
    unittest.main()
