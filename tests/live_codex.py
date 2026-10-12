"""Manual synthetic integration check; never reads WeChat messages."""
import json
from pathlib import Path
import sys
import time

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from codex_bridge import AppServer, SCHEMA, analyze

if '--diagnose' in sys.argv:
    server = AppServer()
    deadline = time.monotonic() + 60
    try:
        server.request('initialize', {'clientInfo': {'name': 'junshi_test', 'version': '0.1.0'},
                                    'capabilities': {}}, deadline)
        server.send('initialized', {}, False)
        if '--models' in sys.argv:
            result = server.request('model/list', {}, deadline)
            print(json.dumps([{'id': item.get('id'), 'model': item.get('model'),
                               'default': item.get('isDefault')} for item in result['data']]))
            sys.exit(0)
        thread = server.request('thread/start', {'cwd': str(Path(__file__).resolve().parents[1]),
                                'sandbox': 'read-only', 'approvalPolicy': 'never',
                                'ephemeral': True}, deadline)['thread']['id']
        server.request('turn/start', {'threadId': thread, 'input': [{'type': 'text',
                       'text': 'Synthetic protocol test. Return one candidate, no tools needed.'}],
                       'outputSchema': SCHEMA}, deadline)
        while True:
            event = server.receive(deadline)
            if event.get('method') == 'turn/completed':
                print(json.dumps(event['params']['turn'].get('error'), ensure_ascii=True))
                break
    finally:
        server.close()
else:
    packet = ('$goutoujunshi 这是虚构即时回复测试。身份已确认：用户=me，对象A=peer。'
              '双方是认识两周的朋友，目标为温和回复。MBTI和主观评分未知，不补猜。'
              '[#1] peer：今天加班好累。[#2] me：刚下班吗？[#3] peer：嗯，刚到家。'
              '先处理眼前回复，最多三条候选，分析优缺点。不写长期记忆，不读取无关文件。'
              '按给定 JSON schema 输出，recommended_index 从0开始。')
    if '--memory' in sys.argv:
        packet += '\n这是虚构的记忆格式测试，由宿主管理记忆，不运行记忆脚本。根据本次消息id给memory_updates，没有高价值变化返回空数组。'
    result = analyze(packet, with_memory='--memory' in sys.argv)
    print(json.dumps({'candidate_count': len(result['candidates']),
                      'recommended_index': result['recommended_index'], 'structure_valid': True,
                      'memory_update_count': len(result.get('memory_updates', []))}))
