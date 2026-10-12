import json
import unittest
from unittest.mock import patch
from types import SimpleNamespace

from quick_reply import ReplyTransport, preview_candidates, schema
from test_core import RESULT


class QuickReplyTests(unittest.TestCase):
    def test_only_complete_valid_candidates_are_previewed(self):
        row = json.dumps(RESULT['candidates'][0], ensure_ascii=False)
        self.assertEqual(preview_candidates('{"candidates":[' + row + ',{"text":'), RESULT['candidates'])
        self.assertEqual(preview_candidates('{"candidates":[{"text":"incomplete"}]'), [])
        self.assertEqual(preview_candidates('{"candidates":[' + row[:-2]), [])

    def test_transport_reused_but_threads_and_evidence_isolated(self):
        class FakeServer:
            def __init__(self):
                self.process = SimpleNamespace(poll=lambda: None)
                self.calls = []
                self.threads = 0
                self.closed = False
            def request(self, method, params, deadline):
                self.calls.append((method, params))
                if method == 'model/list':
                    return {'data': [{'model': 'primary', 'isDefault': True,
                        'supportedReasoningEfforts': [{'reasoningEffort': 'low'}]}]}
                if method == 'thread/start':
                    self.threads += 1
                    return {'thread': {'id': str(self.threads)}}
                if method == 'turn/start':
                    self.sent = False
                    return {'turn': {'id': str(self.threads)}}
                return {}
            def send(self, *args, **kwargs):
                pass
            def receive(self, deadline):
                params = {'threadId': str(self.threads), 'turnId': str(self.threads)}
                if not self.sent:
                    self.sent = True
                    return {'method': 'item/completed', 'params': {**params,
                        'item': {'type': 'agentMessage', 'text': json.dumps(RESULT)}}}
                return {'method': 'turn/completed', 'params': {**params,
                    'turn': {'id': str(self.threads), 'status': 'completed'}}}
            def close(self):
                self.closed = True
        transport = ReplyTransport()
        with patch('quick_reply.AppServer', FakeServer):
            self.assertEqual(transport.run('object A'), RESULT)
            server = transport.server
            self.assertEqual(transport.run('object B'), RESULT)
            self.assertIs(transport.server, server)
            self.assertEqual(sum(method == 'initialize' for method, _ in server.calls), 1)
            self.assertEqual(sum(method == 'model/list' for method, _ in server.calls), 1)
            turns = [params for method, params in server.calls if method == 'turn/start']
            self.assertNotEqual(turns[0]['threadId'], turns[1]['threadId'])
            self.assertEqual(turns[1]['input'], [{'type': 'text', 'text': 'object B'}])
            self.assertEqual(turns[1]['effort'], 'low')
            transport.reset()
            self.assertTrue(server.closed)

    def test_memory_schema_remains_explicit(self):
        self.assertNotIn('memory_updates', schema()['properties'])
        self.assertIn('memory_updates', schema(True)['required'])


if __name__ == '__main__':
    unittest.main()
