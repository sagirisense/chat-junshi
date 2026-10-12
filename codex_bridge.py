"""Minimal stdio App Server client. Chat evidence is submitted only on Analyze."""
import json
import copy
import os
import platform
from pathlib import Path
import queue
import shutil
import subprocess
import threading
import time
from relationship_memory import skill_root

ACTIVE_SERVERS = set()
ACTIVE_LOCK = threading.Lock()


def close_active_servers():
    with ACTIVE_LOCK:
        servers = list(ACTIVE_SERVERS)
    for server in servers:
        server.close()


def object_schema(properties):
    return {'type': 'object', 'properties': properties, 'required': list(properties),
            'additionalProperties': False}


STRING = {'type': 'string'}
STRINGS = {'type': 'array', 'items': STRING}
SCHEMA = object_schema({
    'summary': STRING, 'facts': STRINGS, 'hypotheses': STRINGS, 'unknowns': STRINGS,
    'recommended_index': {'type': 'integer'}, 'recommendation_reason': STRING,
    'candidates': {'type': 'array', 'minItems': 1, 'maxItems': 3,
                   'items': object_schema({
                       'text': STRING, 'pros': STRINGS, 'cons': STRINGS,
                       'suitable_when': STRING, 'next_step': STRING})}})

MEMORY_SCHEMA = copy.deepcopy(SCHEMA)
MEMORY_SCHEMA['properties']['memory_updates'] = {
    'type': 'array', 'maxItems': 3,
    'items': object_schema({
        'scope': {'type': 'string', 'enum': ['event', 'hypothesis']},
        'field': {'type': 'string', 'enum': ['planned_activity', 'conflict', 'repair', 'key_event', 'intent']},
        'value': {'type': 'string', 'maxLength': 200},
        'evidence_ids': {'type': 'array', 'items': {'type': 'integer'}, 'minItems': 1},
        'confidence': {'type': 'string', 'enum': ['high', 'medium', 'low']}})}
MEMORY_SCHEMA['required'].append('memory_updates')


def validate_result(result):
    if not isinstance(result, dict) or set(result) != set(SCHEMA['properties']):
        raise ValueError('分析结果结构不完整')
    for field in ('summary', 'recommendation_reason'):
        if not isinstance(result[field], str):
            raise ValueError('分析结果文字无效')
    for field in ('facts', 'hypotheses', 'unknowns'):
        if not isinstance(result[field], list) or not all(isinstance(x, str) for x in result[field]):
            raise ValueError('分析证据结构无效')
    candidates = result['candidates']
    if not isinstance(candidates, list) or not 1 <= len(candidates) <= 3:
        raise ValueError('候选数量无效')
    for candidate in candidates:
        if not isinstance(candidate, dict) or set(candidate) != {'text', 'pros', 'cons', 'suitable_when', 'next_step'}:
            raise ValueError('候选结构无效')
        for field in ('text', 'suitable_when', 'next_step'):
            if not isinstance(candidate[field], str):
                raise ValueError('候选文字无效')
        for field in ('pros', 'cons'):
            if not isinstance(candidate[field], list) or not all(isinstance(x, str) for x in candidate[field]):
                raise ValueError('候选分析无效')
    idx = result['recommended_index']
    if type(idx) is not int or not 0 <= idx < len(candidates):
        raise ValueError('推荐索引无效')
    return result


def command():
    # npm's .cmd shim cannot be executed safely as an argv process on Windows.
    shim = shutil.which('codex.cmd') or shutil.which('codex')
    if not shim:
        # Explorer-launched apps do not inherit fnm's terminal-only PATH.
        roots = [Path(os.environ.get('APPDATA', Path.home() / 'AppData/Roaming')),
                 Path(os.environ.get('LOCALAPPDATA', Path.home() / 'AppData/Local'))]
        candidates = [roots[0] / 'npm/codex.cmd']
        for root in roots:
            versions = root / 'fnm/node-versions'
            if versions.is_dir():
                candidates.extend(sorted(versions.glob('*/installation/codex.cmd'), reverse=True))
        shim = next((str(path) for path in candidates if path.is_file()), None)
    if not shim:
        raise RuntimeError('未找到 Codex CLI，请安装并登录，或使用自定义 API')
    if Path(shim).suffix.lower() in ('.cmd', '.ps1'):
        script = Path(shim).parent / 'node_modules/@openai/codex/bin/codex.js'
        package = script.parents[1]
        arch = 'arm64' if platform.machine().lower() in ('arm64', 'aarch64') else 'x64'
        target = 'aarch64-pc-windows-msvc' if arch == 'arm64' else 'x86_64-pc-windows-msvc'
        for vendor in (package / f'node_modules/@openai/codex-win32-{arch}/vendor',
                       package.parent / f'codex-win32-{arch}/vendor', package / 'vendor'):
            binary = vendor / target / 'bin/codex.exe'
            if binary.is_file():
                # Own the native process so closing the app also stops inference.
                return [str(binary), 'app-server']
        node = str(Path(shim).parent / 'node.exe') if (Path(shim).parent / 'node.exe').is_file() else shutil.which('node')
        if not node or not script.is_file():
            raise RuntimeError('未找到 Codex 的 Node 入口，请检查 CLI 安装')
        return [node, str(script), 'app-server']
    return [shim, 'app-server']


class AppServer:
    def __init__(self):
        self.process = subprocess.Popen(command(), stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                        stderr=subprocess.DEVNULL, text=True, encoding='utf-8',
                                        creationflags=getattr(subprocess, 'CREATE_NO_WINDOW', 0))
        self.events = queue.Queue()
        self.sequence = 0
        with ACTIVE_LOCK:
            ACTIVE_SERVERS.add(self)
        threading.Thread(target=self._read, daemon=True).start()

    def _read(self):
        try:
            for line in self.process.stdout:
                try:
                    self.events.put(json.loads(line))
                except ValueError:
                    continue
        finally:
            self.events.put({'closed': True})

    def send(self, method, params, request=True):
        msg = {'method': method, 'params': params}
        if request:
            self.sequence += 1
            msg['id'] = self.sequence
        self.process.stdin.write(json.dumps(msg, ensure_ascii=False) + '\n')
        self.process.stdin.flush()
        return msg.get('id')

    def receive(self, deadline):
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            raise TimeoutError('Codex 分析超时，请稍后重试')
        try:
            msg = self.events.get(timeout=remaining)
        except queue.Empty:
            raise TimeoutError('Codex 分析超时，请稍后重试') from None
        if msg.get('closed'):
            raise RuntimeError('Codex 连接已关闭，请检查 CLI 登录状态')
        if 'method' in msg and 'id' in msg:
            # No external actions or user-input tools are allowed in this analysis client.
            self.process.stdin.write(json.dumps({'id': msg['id'], 'error': {
                'code': -32601, 'message': 'This client supports analysis only'}}) + '\n')
            self.process.stdin.flush()
        return msg

    def request(self, method, params, deadline):
        ident = self.send(method, params)
        while True:
            msg = self.receive(deadline)
            if msg.get('id') == ident and 'method' not in msg:
                if 'error' in msg:
                    raise RuntimeError('Codex 拒绝请求，请检查 CLI 版本、登录与 Skill 路径')
                return msg['result']

    def close(self):
        with ACTIVE_LOCK:
            ACTIVE_SERVERS.discard(self)
        if self.process.poll() is None:
            self.process.terminate()
            try:
                self.process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                self.process.kill()
                self.process.wait()
        for stream in (self.process.stdin, self.process.stdout):
            if not stream.closed:
                stream.close()


def analyze(packet, skill_path=None, timeout=180, model=None, with_memory=False):
    skill = Path(skill_path or skill_root() / 'SKILL.md').resolve()
    if not skill.is_file():
        raise RuntimeError('未找到本地 goutoujunshi/SKILL.md')
    server = AppServer()
    deadline = time.monotonic() + timeout
    try:
        server.request('initialize', {'clientInfo': {'name': 'wechat_junshi', 'version': '0.1.0'},
                                    'capabilities': {}}, deadline)
        server.send('initialized', {}, request=False)
        if not model:
            models = server.request('model/list', {}, deadline)['data']
            default = next((item for item in models if item.get('isDefault')), None)
            if default is None:
                raise RuntimeError('CLI 未提供默认可用模型，请在界面填写模型名')
            model = default['model']
        thread = server.request('thread/start', {'cwd': str(Path(__file__).parent), 'model': model,
                                'sandbox': 'read-only', 'approvalPolicy': 'never',
                                'ephemeral': True}, deadline)['thread']['id']
        turn = server.request('turn/start', {'threadId': thread,
                              'input': [{'type': 'text', 'text': packet},
                                        {'type': 'skill', 'name': 'goutoujunshi', 'path': str(skill)}],
                              'outputSchema': MEMORY_SCHEMA if with_memory else SCHEMA}, deadline)['turn']['id']
        final = ''
        while True:
            event = server.receive(deadline)
            params = event.get('params', {})
            if params.get('threadId') != thread or params.get('turnId', params.get('turn', {}).get('id')) != turn:
                continue
            if event.get('method') == 'item/completed':
                item = params.get('item', {})
                if item.get('type') == 'agentMessage':
                    final = item.get('text', '')
            if event.get('method') == 'turn/completed':
                if params['turn'].get('status') != 'completed':
                    raise RuntimeError('Codex 未完成分析，请检查登录或额度')
                if not final:
                    raise ValueError('Codex 未返回分析正文')
                result = json.loads(final)
                if with_memory:
                    updates = result.pop('memory_updates', [])
                    if not isinstance(updates, list) or len(updates) > 3:
                        raise ValueError('记忆更新结构无效')
                    validate_result(result)
                    result['memory_updates'] = updates
                    return result
                return validate_result(result)
    finally:
        server.close()
