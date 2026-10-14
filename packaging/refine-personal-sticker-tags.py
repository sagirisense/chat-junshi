"""Refine reviewed personal-library tags without changing recommendation switches."""
import json
from pathlib import Path

root = Path(__file__).resolve().parents[1] / 'data' / 'stickers'
path = root / 'index.json'
rows = json.loads(path.read_text(encoding='utf-8'))
groups = [
    (['晚安','休息','困了','毯子'], ['温和','低压力','收尾','关心'], '晚安、疲惫或想休息时，温和收尾，不要求继续聊天'),
    (['收到','点头','OK'], ['中性','简短确认','答应'], '确认收到、答应安排或简短回应'),
    (['谢谢','感谢'], ['友好','感谢回应'], '感谢帮助、礼物或关心'),
    (['吃饭','饿了','早餐','咖啡','饮料'], ['日常','轻松','饮食话题'], '吃饭、饮料或饥饿等生活话题；不要凭空约定一起吃饭'),
    (['开心','哈哈哈','庆祝','欢呼'], ['开心','轻松','接梗'], '开心消息、好笑的内容或轻松庆祝'),
    (['你好','招手','挥手','在吗'], ['友好','打招呼','开场'], '自然打招呼、出现或开启轻松对话'),
    (['疑问','好奇','惊讶','震惊','什么'], ['好奇','轻微惊讶','回应'], '好奇、询问或轻微惊讶；不要用于质疑认真表达'),
    (['点赞','厉害','666','加油'], ['鼓励','认可','支持'], '认可成果、鼓励或赞赏，不夸大关系承诺'),
]
for row in rows:
    tags = [x.strip() for x in row.get('tags','').replace('，',',').split(',') if x.strip()]
    for keys, extra, purpose in groups:
        if any(key in tags for key in keys):
            tags = list(dict.fromkeys(tags + extra))
            if row.get('enabled') and not row.get('intimate'):
                row['suitable'] = purpose
            break
    if row.get('intimate'):
        tags = list(dict.fromkeys(tags + ['亲密','需要双方接受']))
    row['tags'] = ','.join(tags)
path.write_text(json.dumps(rows,ensure_ascii=False,indent=2),encoding='utf-8')
print(f'Refined {len(rows)} personal stickers; switches and provenance retained.')
