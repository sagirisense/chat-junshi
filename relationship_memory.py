"""Adapter for the installed Skill's own consent-gated, bounded memory store."""
import json
import os
from pathlib import Path
import importlib.util
import threading
import time
from contextlib import contextmanager


def skill_root():
    return Path(os.environ.get('CODEX_HOME', Path.home() / '.codex')) / 'skills/goutoujunshi'


class RelationshipMemory:
    def __init__(self, directory):
        self.directory = Path(directory).resolve()
        self.module = None
        self.lock = threading.RLock()
        self.cache = {}

    def call(self, action, *args):
        script = skill_root() / 'scripts/memory_store.py'
        if not script.is_file():
            raise RuntimeError('本地军师 Skill 缺少记忆脚本，请检查安装')
        with self.lock:
            key = (action, args)
            cached = self.cache.get(key)
            if action in ('status', 'show', 'context') and cached and time.monotonic() - cached[0] < 1:
                return cached[1]
            if self.module is None:
                spec = importlib.util.spec_from_file_location(f'_junshi_memory_{id(self)}', script)
                self.module = importlib.util.module_from_spec(spec)
                spec.loader.exec_module(self.module)
                # Isolated module instance; no changes to the installed Skill or global environment.
                self.module.memory_dir = lambda: self.directory
                original_connect = self.module.connect
                @contextmanager
                def managed_connect(create=False):
                    connection = original_connect(create)
                    try:
                        with connection:
                            yield connection
                    finally:
                        connection.close()
                # The CLI normally relies on process exit to close SQLite handles.
                self.module.connect = managed_connect
            emitted = []
            self.module.emit = emitted.append
            try:
                parsed = self.module.build_parser().parse_args([action, *args])
                parsed.func(parsed)
            except Exception as exc:
                raise RuntimeError(f'关系记忆操作失败：{exc}') from None
            if not emitted:
                raise RuntimeError('记忆脚本未返回有效结果')
            data = emitted[-1]
            if action not in ('status', 'show', 'context'):
                self.cache.clear()
            else:
                self.cache[key] = (time.monotonic(), data)
            return data

    def status(self):
        return self.call('status')

    def rows(self, subject):
        if not self.directory.joinpath('memory.sqlite3').exists():
            return []
        return self.call('show', '--subject-id', subject)['memories']

    def put(self, scope, subject, field, value, source='user_explicit', ref='建档表', confidence='high'):
        return self.call('apply', '--json', json.dumps({
            'scope': scope, 'subject_id': subject, 'field': field, 'value': value,
            'source_type': source, 'source_ref': ref, 'occurred_at': None,
            'confidence': confidence}, ensure_ascii=False))

    def context(self, subject):
        status = self.status()
        if not status.get('consent_enabled') or status.get('paused'):
            return None
        return self.call('context', '--subject-id', subject, '--max-chars', '4000')
