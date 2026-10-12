import copy
import json
import threading
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer
from unittest.mock import patch
import api_bridge
from test_core import RESULT


class APIBridgeTests(unittest.TestCase):
    def test_http_models_auth_and_no_chat_submission(self):
        seen = []
        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args): pass
            def do_GET(self):
                seen.append((self.path, self.headers.get('Authorization')))
                self.send_response(200); self.end_headers()
                self.wfile.write(b'{"data":[{"id":"chat-b"},{"id":"chat-a"},{"id":"chat-a"}]}')
        server = HTTPServer(('127.0.0.1', 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True); thread.start()
        try:
            self.assertEqual(api_bridge.models({'base_url': f'http://127.0.0.1:{server.server_port}/v1', 'api_key': 'test-only'}), ['chat-a', 'chat-b'])
            self.assertEqual(seen, [('/v1/models', 'Bearer test-only')])
        finally:
            server.shutdown(); server.server_close(); thread.join()

    def test_provider_model_lists_and_gemini_pagination(self):
        with patch('api_bridge.send', side_effect=[{'models': [{'name': 'models/a', 'supportedGenerationMethods': ['generateContent']}, {'name':'models/embed','supportedGenerationMethods':['embedContent']}], 'nextPageToken':'next'}, {'models':[{'name':'models/b','supportedGenerationMethods':['generateContent']}]}]) as send:
            self.assertEqual(api_bridge.models({'protocol':'gemini'}), ['a','b'])
            self.assertEqual(send.call_args.args[1], 'models?pageToken=next')
        with patch('api_bridge.send', return_value={'data':[{'id':'claude-test'}]}):
            self.assertEqual(api_bridge.models({'protocol':'anthropic'}), ['claude-test'])

    def test_three_protocol_results_use_same_validation(self):
        for protocol in ('openai','anthropic','gemini'):
            content = json.dumps(RESULT, ensure_ascii=False)
            data = {'choices':[{'message':{'content':content}}]} if protocol == 'openai' else {'content':[{'type':'text','text':content}]} if protocol == 'anthropic' else {'candidates':[{'content':{'parts':[{'text':content}]}}]}
            with self.subTest(protocol=protocol), patch('api_bridge.send',return_value=data) as send:
                self.assertEqual(api_bridge.analyze_api('synthetic chat', {'protocol':protocol}, 'test-model'), RESULT)
                self.assertIn('synthetic chat', json.dumps(send.call_args.args[2]))

    def test_memory_updates_and_invalid_output(self):
        result = copy.deepcopy(RESULT); result['memory_updates'] = []
        with patch('api_bridge.send',return_value={'choices':[{'message':{'content':json.dumps(result)}}]}):
            self.assertEqual(api_bridge.analyze_api('test', {}, 'test', True)['memory_updates'], [])
        with patch('api_bridge.send',return_value={'choices':[{'message':{'content':'not json'}}]}):
            with self.assertRaises(ValueError): api_bridge.analyze_api('test', {}, 'test')

    def test_url_credentials_and_invalid_models(self):
        with self.assertRaises(ValueError): api_bridge.endpoint({'base_url':'https://user:secret@example.com/v1'}, 'models')
        with patch('api_bridge.send',return_value={'oops':[]}):
            with self.assertRaises(ValueError): api_bridge.models({})
