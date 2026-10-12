"""Local JSONL sidecar for Wails. No network listener; Python owns OCR and Skill memory."""
import hashlib
import json
import multiprocessing as mp
import os
from pathlib import Path
import queue
import sys
import threading
import time

from codex_bridge import analyze, close_active_servers
from collector import run
from relationship_memory import RelationshipMemory
from sessions import Session
from api_bridge import analyze_api, models as api_models

ROOT = Path(__file__).resolve().parent
PROFILE_FIELDS = {
    'user_mbti': ('user', 'mbti'), 'user_score': ('user', 'subjective_score'),
    'user_strengths': ('user', 'strengths'), 'user_boundaries': ('user', 'boundaries'),
    'peer_label': ('object', 'display_label'), 'peer_mbti': ('object', 'mbti'),
    'peer_score': ('object', 'subjective_score'), 'relationship': ('relationship', 'stage'),
    'history': ('relationship', 'history'), 'goal': ('relationship', 'goal'),
    'emotion': ('relationship', 'emotion')}


class Service:
    def __init__(self, root=ROOT):
        self.root = Path(root)
        self.memory = RelationshipMemory(self.root / 'data/relationship-memory')
        self.sessions = {}
        self.current = None
        self.child = None
        self.enabled = mp.Event()
        self.incoming = mp.Queue()
        self.lock = threading.RLock()
        self.write_lock = threading.Lock()
        self.busy = False
        self.blocked = False
        self.result = None
        self.status = '采集已暂停，打开微信单聊后开始'
        self.profile = {}
        self.profiles = {}
        self.epoch = 0
        self.memory_notice = ''
        self.onboarded = False
        self.active_title = ''
        self.restore_sessions()

    def restore_sessions(self):
        for path in (self.root / 'data').glob('*.json'):
            try:
                saved = json.loads(path.read_text(encoding='utf-8'))
                title = saved.get('title')
                if title:
                    self.sessions[title] = Session(title, self.root / 'data')
            except (ValueError, OSError, KeyError):
                continue
        # Restore only the last explicitly saved selection, never guess another object.
        selected = self.root / 'data/selected-session.json'
        if selected.exists():
            try:
                title = json.loads(selected.read_text(encoding='utf-8')).get('selected_title')
                if title in self.sessions:
                    self.select(title)
            except (ValueError, OSError):
                pass
        if not self.current:
            try:
                rows = self.memory.rows(self.subject())
            except RuntimeError:
                rows = []
            for row in rows:
                for key, (scope, field) in PROFILE_FIELDS.items():
                    if scope == 'user' and row['scope'] == scope and row['field'] == field:
                        self.profile[key] = row['value']

    def emit(self, value):
        with self.write_lock:
            print(json.dumps(value, ensure_ascii=False), flush=True)

    def subject(self):
        return 'obj-' + hashlib.sha256((self.current.title if self.current else '__unbound__').encode()).hexdigest()[:16]

    def snapshot(self):
        try:
            status = self.memory.status()
            rows = self.memory.rows(self.subject())
        except RuntimeError:
            status = {'consent_enabled': False, 'paused': True, 'unavailable': True}
            rows = []
            self.memory_notice = '关系记忆暂不可用，采集仍可继续；请稍后重试记忆操作。'
        return {'title': self.current.title if self.current else '',
                'active_title': self.active_title,
                'messages': self.current.messages[-150:] if self.current else [],
                'sessions': list(self.sessions), 'confirmed': bool(self.current and self.current.confirmed),
                'capturing': self.enabled.is_set(), 'blocked': self.blocked, 'busy': self.busy,
                'status': self.status, 'result': self.result, 'profile': self.profile,
                'onboarded': self.onboarded, 'memory': status, 'memory_rows': rows,
                'memory_notice': self.memory_notice}

    def notify(self):
        self.emit({'event': 'state', 'data': self.snapshot()})

    def select(self, title):
        if self.current and self.current.title == title:
            return
        if title not in self.sessions:
            self.sessions[title] = Session(title, self.root / 'data')
        self.current = self.sessions[title]
        selected = self.root / 'data/selected-session.json'
        selected.parent.mkdir(parents=True, exist_ok=True)
        selected.write_text(json.dumps({'selected_title': title}, ensure_ascii=False), encoding='utf-8')
        self.current.confirmed = False
        self.result = None
        self.epoch += 1
        self.profile = dict(self.profiles.get(self.subject(), {}))
        try:
            rows = self.memory.rows(self.subject())
        except RuntimeError:
            rows = []
            self.memory_notice = '关系记忆暂不可用，当前仅使用本次填写的信息。'
        for row in rows:
            for key, (scope, field) in PROFILE_FIELDS.items():
                if row['scope'] == scope and row['field'] == field:
                    self.profile[key] = row['value']
        if not self.profile.get('relationship'):
            self.profile['relationship'] = self.current.background
        if not self.profile.get('goal'):
            self.profile['goal'] = self.current.goal
        self.onboarded = bool(self.profiles.get(self.subject()) or any(
            row['subject_id'] == self.subject() and row['scope'] in ('object', 'relationship')
            for row in rows))

    def stop(self):
        self.enabled.clear()
        if self.child:
            self.child.terminate()
            self.child.join(timeout=3)
            self.child = None
        self.incoming = mp.Queue()
        self.epoch += 1

    def apply_profile(self, profile, persist):
        if not self.current:
            raise ValueError('先开启采集并选择单聊，再为当前对象建档')
        cleaned = {}
        for key in PROFILE_FIELDS:
            value = str(profile.get(key, '')).strip()
            if len(value) > 200:
                raise ValueError('每项档案请控制在200字以内')
            if key.endswith('_score') and value:
                try:
                    score = float(value)
                except ValueError:
                    raise ValueError('主观评分请填写0–100数字') from None
                if not 0 <= score <= 100:
                    raise ValueError('主观评分请填写0–100数字')
            cleaned[key] = value
        if persist:
            state = self.memory.status()
            if not state.get('consent_enabled') or state.get('paused'):
                raise ValueError('请先明确启用记忆；暂停时不能保存新档案')
            existing = {(r['scope'], r['field']): r['value'] for r in self.memory.rows(self.subject())}
            for key, value in cleaned.items():
                scope, field = PROFILE_FIELDS[key]
                if value and existing.get((scope, field)) != value:
                    self.memory.put(scope, 'user' if scope == 'user' else self.subject(), field, value)
            self.memory_notice = '已保存明确填写的档案。可查看、暂停、撤销或删除。'
        self.profile = cleaned
        self.profiles[self.subject()] = dict(cleaned)
        self.current.revision += 1
        self.onboarded = True
        self.result = None

    def set_goal(self, value):
        if not self.current:
            raise ValueError('请先选择一个会话')
        goal = str(value).strip()
        if len(goal) > 200:
            raise ValueError('回复目标请控制在200字以内')
        if self.profile.get('goal', '') != goal:
            self.profile['goal'] = goal
            self.profiles.setdefault(self.subject(), {})['goal'] = goal
            self.current.revision += 1
            self.result = None

    def dispatch(self, action, payload):
        if action == 'models':
            if payload.get('provider') == 'custom':
                return {'models': api_models(payload.get('api', {}))}
            from codex_bridge import AppServer
            server = AppServer()
            try:
                deadline = time.monotonic() + 25
                server.request('initialize', {'clientInfo': {'name': 'wechat_junshi', 'version': '0.1.0'}, 'capabilities': {}}, deadline)
                server.send('initialized', {}, request=False)
                return {'models': [row['model'] for row in server.request('model/list', {}, deadline)['data']]}
            finally:
                server.close()
        with self.lock:
            if action == 'state':
                return self.snapshot()
            if action == 'select':
                title = payload.get('title')
                if title not in self.sessions:
                    raise ValueError('该会话尚未保存')
                self.select(title)
            elif action == 'bind_profile':
                title = str(payload.get('title', '')).strip()
                if not title or len(title) > 200:
                    raise ValueError('请填写准确的微信单聊名称')
                self.select(title)
                self.current.save()
            elif action == 'capture':
                if self.enabled.is_set():
                    self.enabled.clear()
                    self.status = '采集已暂停'
                else:
                    if self.child is None or not self.child.is_alive():
                        self.child = mp.Process(target=run, args=(self.incoming, self.enabled), daemon=True)
                        self.child.start()
                    self.enabled.set()
                    self.status = '正在读取微信窗口，请核对会话与原文'
                self.epoch += 1
            elif action == 'confirm':
                if not self.current or self.blocked:
                    raise ValueError('请先读取标题明确的单聊')
                self.current.confirmed = bool(payload.get('confirmed'))
                self.current.revision += 1
                self.result = None
            elif action == 'profile':
                self.apply_profile(payload['profile'], bool(payload.get('persist')))
            elif action == 'goal':
                self.set_goal(payload.get('goal', ''))
            elif action == 'skip_onboarding':
                self.onboarded = True
            elif action == 'correct':
                if not self.current:
                    raise ValueError('没有当前会话')
                index = next((i for i, m in enumerate(self.current.messages) if m['id'] == payload['id']), None)
                if index is None:
                    raise ValueError('消息已不存在')
                self.current.correct(index, payload['sender'], payload['text'])
                self.result = None
            elif action == 'packet':
                if not self.current or self.blocked:
                    raise ValueError('请先采集一个单聊')
                return {'text': self.current.packet(goal=self.profile.get('goal', ''))}
            elif action == 'export':
                if not self.current:
                    raise ValueError('请先采集一个会话')
                return {'text': json.dumps(self.current.payload(), ensure_ascii=False, indent=2)}
            elif action in ('prepare_import_chat', 'import_chat', 'prepare_import_profile'):
                try:
                    data = json.loads(payload.get('text', '').lstrip('\ufeff'))
                except (ValueError, TypeError):
                    raise ValueError('请选择有效的 JSON 文件') from None
                if not isinstance(data, dict) or data.get('version') != 1:
                    raise ValueError('不支持的文件格式，请使用本工具导出的 JSON')
                title = data.get('title', '')
                if not isinstance(title, str) or len(title.strip()) > 200:
                    raise ValueError('会话名称格式无效')
                title = title.strip()
                if action == 'prepare_import_profile':
                    if data.get('type') != 'relationship-profile' or not isinstance(data.get('profile'), dict):
                        raise ValueError('这不是用户档案文件')
                    draft = {}
                    for key, value in data['profile'].items():
                        if key not in PROFILE_FIELDS or not isinstance(value, (str, int, float)) or isinstance(value, bool):
                            raise ValueError('档案字段格式无效')
                        value = str(value).strip()
                        if len(value) > 200:
                            raise ValueError('每项档案请控制在200字以内')
                        if key.endswith('_score') and value:
                            try:
                                if not 0 <= float(value) <= 100: raise ValueError()
                            except ValueError:
                                raise ValueError('评分必须是0–100数字') from None
                        draft[key] = value
                    if not draft: raise ValueError('文件没有档案字段')
                    return {'title': title, 'profile': draft}
                messages = data.get('messages')
                if data.get('source') != 'wechat-window-ocr' or not title or not isinstance(messages, list) or not 0 < len(messages) <= 10000:
                    raise ValueError('请选择本工具导出的非空聊天 JSON（最多10000条）')
                cleaned = []
                for row in messages:
                    if not isinstance(row, dict) or row.get('sender') not in ('me', 'peer') or not isinstance(row.get('text'), str) or not row['text'].strip() or len(row['text']) > 20000:
                        raise ValueError('聊天消息格式无效')
                    cleaned.append({'id': len(cleaned) + 1, 'sender': row['sender'], 'text': row['text'],
                                    'message_time': row.get('message_time') if isinstance(row.get('message_time'), str) else None,
                                    'observed_at': row.get('observed_at') if isinstance(row.get('observed_at'), str) else None})
                if action == 'prepare_import_chat':
                    return {'title': title, 'count': len(cleaned), 'existing': title in self.sessions}
                if payload.get('confirm') is not True:
                    raise ValueError('请确认导入聊天')
                self.stop()
                session = self.sessions.get(title) or Session(title, self.root / 'data')
                previous = list(session.messages)
                session.messages = previous + [dict(row, id=len(previous) + index + 1) for index, row in enumerate(cleaned)]
                session.confirmed = False
                session.revision += 1
                session.save()
                self.sessions[title] = session
                self.select(title)
                self.current.confirmed = False
                self.result = None
                self.blocked = False
                self.active_title = ''
                self.status = '聊天已导入，请重新核对说话人；重复导入会追加重复消息'
            elif action == 'export_profile':
                profile = {key: value for key, value in self.profile.items()
                           if key in PROFILE_FIELDS and value not in ('', None)}
                if not profile:
                    raise ValueError('请先填写或选择已有档案')
                return {'text': json.dumps({'version': 1, 'type': 'relationship-profile',
                                            'title': self.current.title if self.current else '',
                                            'profile': profile}, ensure_ascii=False, indent=2)}
            elif action == 'delete_session':
                if payload.get('confirm') is not True:
                    raise ValueError('请确认删除当前本地会话')
                if self.current:
                    self.stop()
                    title = self.current.title
                    self.current.path.unlink(missing_ok=True)
                    (self.root / 'data/selected-session.json').unlink(missing_ok=True)
                    self.sessions.pop(title, None)
                    self.current = None
                    self.active_title = ''
                    self.profile = {}
                    self.result = None
                    self.blocked = False
                    self.onboarded = False
                    self.status = '本地会话已删除，关系记忆保留；重新采集后可建立会话'
            elif action == 'clear_chat':
                if self.current:
                    self.stop()
                    self.current.messages = []
                    self.current.confirmed = False
                    self.current.revision += 1
                    self.current.save()
                    self.result = None
                    self.status = '当前聊天文字已清空；关系记忆单独保留'
            elif action == 'memory':
                operation = payload['operation']
                if operation == 'enable':
                    if payload.get('consent') is not True:
                        raise ValueError('需明确同意保存精简关系档案')
                    self.memory.call('enable', '--confirm')
                elif operation in ('pause', 'resume', 'undo'):
                    self.memory.call(operation)
                    if operation == 'undo':
                        self.profiles.pop(self.subject(), None)
                        self.profile = {}
                        for row in self.memory.rows(self.subject()):
                            for key, (scope, field) in PROFILE_FIELDS.items():
                                if row['scope'] == scope and row['field'] == field:
                                    self.profile[key] = row['value']
                elif operation == 'forget':
                    if not self.current or payload.get('confirm') is not True:
                        raise ValueError('请确认要忘记当前对象')
                    self.memory.call('forget-object', self.subject(), '--confirm')
                    self.profiles.pop(self.subject(), None)
                    self.profile = {key: value for key, value in self.profile.items() if key.startswith('user_')}
                    self.onboarded = False
                elif operation in ('revoke', 'delete_all'):
                    if payload.get('confirm') is not True:
                        raise ValueError('请明确确认撤回记忆同意')
                    args = ['--confirm'] + (['--delete'] if operation == 'delete_all' else [])
                    self.memory.call('revoke', *args)
                    self.profiles.clear()
                    self.profile = {}
                    self.onboarded = False
                else:
                    raise ValueError('不支持的记忆操作')
                self.epoch += 1
                self.result = None
                self.memory_notice = {'enable': '已启用本地记忆', 'pause': '已暂停记忆读写',
                    'resume': '已恢复记忆', 'undo': '已撤销最近一次字段更新',
                    'forget': '已忘记当前对象', 'revoke': '已撤回同意，资料保留但不用于分析',
                    'delete_all': '已撤回同意并删除全部关系记忆'}[operation]
            elif action == 'analyze':
                if self.busy:
                    raise ValueError('正在分析，请稍候')
                if not self.current or self.blocked:
                    raise ValueError('请先采集并核对单聊')
                if 'goal' in payload:
                    self.set_goal(payload['goal'])
                packet = self.current.packet(goal=self.profile.get('goal', ''))
                packet += '\n本轮回复目标优先用于选择话术；长期档案中的目标只作为关系背景。'
                try:
                    context = self.memory.context(self.subject())
                except RuntimeError:
                    context = None
                    self.memory_notice = '本次未读取长期记忆，仅分析当前聊天。'
                current_profile = self.profile if context else self.profiles.get(self.subject(), {})
                packet += '\n用户明确填写的本轮档案（未知保持未知）：\n' + json.dumps(current_profile, ensure_ascii=False)
                if context:
                    packet += '\n宿主已经确认用户启用了本机关系记忆，无需再次询问同意；记忆读写全部由宿主负责。'
                    packet += '\n当前对象的精简长期记忆：\n' + json.dumps(context, ensure_ascii=False)
                    packet += ('\n仅提出高价值的新 memory_updates：event 为原文支持的关键事件；'
                               'hypothesis 为有证据、可纠正的暂定解释。引用本次消息 id。'
                               '不把推测写成 event，不记录整份聊天，不代填MBTI评分，不调用记忆脚本；'
                               '由宿主受控保存，没有变化返回空数组。')
                token = (self.current.title, self.current.revision, self.epoch)
                evidence_ids = {m['id'] for m in self.current.messages[-30:]}
                subject = self.subject()
                self.busy = True
                self.result = None
                self.status = '军师正在阅读上下文…'
                threading.Thread(target=self.analyze_bg, args=(packet, token, subject, evidence_ids,
                                 payload.get('model') or None, bool(context), payload.get('provider', 'codex'), dict(payload.get('api', {}))), daemon=True).start()
            else:
                raise ValueError('未知操作')
            self.notify()
            return self.snapshot()

    def analyze_bg(self, packet, token, subject, evidence_ids, model, with_memory, provider='codex', api=None):
        try:
            result = analyze_api(packet, api or {}, model, with_memory) if provider == 'custom' else analyze(packet, model=model, with_memory=with_memory)
            error = None
        except Exception as exc:
            result, error = None, str(exc)
        with self.lock:
            self.busy = False
            if not self.current or token != (self.current.title, self.current.revision, self.epoch):
                self.status = '会话或上下文已变化，旧建议已丢弃，请重新分析'
            elif error:
                self.status = '分析失败：' + error
            else:
                self.result = result
                self.status = '建议已准备好，选择适合你的表达'
                saved = []
                if with_memory:
                    for item in result.pop('memory_updates', [])[:3]:
                        if (not isinstance(item, dict) or item.get('scope') not in ('event', 'hypothesis')
                                or item.get('field') not in ('planned_activity', 'conflict', 'repair', 'key_event', 'intent')
                                or not isinstance(item.get('value'), str) or not 0 < len(item['value']) <= 200
                                or not isinstance(item.get('evidence_ids'), list) or not item['evidence_ids']
                                or not all(type(i) is int and i in evidence_ids for i in item['evidence_ids'])
                                or item.get('confidence') not in ('high', 'medium', 'low')):
                            continue
                        try:
                            self.memory.put(item['scope'], subject, item['field'], item['value'],
                                            'assistant_inference' if item['scope'] == 'hypothesis' else 'tool',
                                            '消息 ' + ','.join(map(str, item['evidence_ids'])), item['confidence'])
                            saved.append(item['value'])
                        except RuntimeError:
                            self.memory_notice = '建议已生成，但记忆写入失败；未完成保存'
                            break
                if saved:
                    self.memory_notice = '记忆更新：' + '；'.join(saved) + '。可撤销最近更新。'
            self.notify()

    def capture_tick(self):
        while True:
            try:
                event = self.incoming.get(timeout=.2)
                with self.lock:
                    kind = event[0]
                    if kind == 'dead':
                        self.enabled.clear()
                        self.blocked = True
                        self.epoch += 1
                        self.status = event[1]
                        self.result = None
                    elif not self.enabled.is_set():
                        continue
                    elif kind == 'chat':
                        self.active_title = event[1]
                        self.select(event[1])
                        self.blocked = False
                    elif kind == 'lines' and not self.blocked:
                        self.select(event[1])
                        self.current.append(event[2])
                        self.result = None
                        self.status = '已收到消息，核对后可生成建议'
                    elif kind in ('status', 'blocked'):
                        self.status = event[1]
                        if kind == 'blocked':
                            self.blocked = True
                            self.epoch += 1
                            if self.current:
                                self.current.confirmed = False
                            self.result = None
                    self.notify()
            except queue.Empty:
                continue
            except Exception as exc:
                self.emit({'event': 'error', 'data': str(exc)})

    def serve(self):
        threading.Thread(target=self.capture_tick, daemon=True).start()
        try:
            for line in sys.stdin:
                ident = None
                try:
                    request = json.loads(line)
                    ident = request['id']
                    data = self.dispatch(request['action'], request.get('payload', {}))
                    self.emit({'id': ident, 'data': data})
                except Exception as exc:
                    self.emit({'id': ident, 'error': str(exc)})
        finally:
            self.stop()
            close_active_servers()


if __name__ == '__main__':
    mp.freeze_support()
    os.chdir(ROOT)
    sys.stdin.reconfigure(encoding='utf-8')
    sys.stdout.reconfigure(encoding='utf-8')
    os.environ['GOUTOUJUNSHI_MEMORY_DIR'] = str(ROOT / 'data/relationship-memory')
    Service().serve()
