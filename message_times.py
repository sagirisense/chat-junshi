"""Conservative WeChat time labels. Group anchors are never exact message times."""
from datetime import datetime, timedelta, timezone
import re

CHINA = timezone(timedelta(hours=8), 'Asia/Shanghai')


def now_china():
    return datetime.now(CHINA)


def parse_label(label, now=None):
    now = (now or now_china()).astimezone(CHINA)
    text = re.sub(r'\s+', '', label).replace('：', ':')
    match = re.fullmatch(r'(?:(今天|昨天|前天|星期[一二三四五六日天]|周[一二三四五六日天])|(?:(\d{4})[年/.-])?(\d{1,2})[月/.-](\d{1,2})日?)?(凌晨|早上|上午|中午|下午|晚上)?(\d{1,2}):(\d{2})', text)
    date_only = False
    if not match:
        match = re.fullmatch(r'(?:(今天|昨天|前天|星期[一二三四五六日天]|周[一二三四五六日天])|(?:(\d{4})[年/.-])?(\d{1,2})[月/.-](\d{1,2})日?)', text)
        date_only = True
    if not match:
        return None
    relative, year, month, day = match.groups()[:4]
    date, basis = now.date(), 'today_inferred'
    try:
        if relative in ('今天', '昨天', '前天'):
            date -= timedelta(days={'今天': 0, '昨天': 1, '前天': 2}[relative])
            basis = 'relative_to_capture_date'
        elif relative:
            weekday = '一二三四五六日'.index(relative[-1].replace('天', '日'))
            date -= timedelta(days=(date.weekday() - weekday) % 7)
            basis = 'weekday_inferred'
        elif month:
            date = datetime(int(year or now.year), int(month), int(day)).date()
            if not year and date > now.date():
                date = date.replace(year=date.year - 1)
            basis = 'explicit_date' if year else 'year_inferred'
        hour = minute = 0
        if not date_only:
            period, hour, minute = match.groups()[4:]
            hour, minute = int(hour), int(minute)
            if period:
                if not 1 <= hour <= 12:
                    return None
                if period in ('下午', '晚上', '中午') and hour < 12:
                    hour += 12
                elif period in ('凌晨', '早上', '上午') and hour == 12:
                    hour = 0
        anchor = datetime(date.year, date.month, date.day, hour, minute, tzinfo=CHINA)
    except (ValueError, OverflowError):
        return None
    return {'label': label.strip(), 'anchor_time': anchor.isoformat(timespec='minutes'),
            'precision': 'day' if date_only else 'group', 'source': 'ocr', 'date_basis': basis}


def clean_context(value):
    if not isinstance(value, dict) or value.get('precision') not in ('group', 'day'):
        return None
    try:
        anchor = datetime.fromisoformat(value['anchor_time'])
        if anchor.tzinfo is None:
            return None
        label = value['label']
        if not isinstance(label, str) or len(label) > 80:
            return None
    except (ValueError, TypeError, KeyError):
        return None
    return {'label': label, 'anchor_time': anchor.astimezone(CHINA).isoformat(timespec='minutes'),
            'precision': value['precision'], 'source': 'ocr',
            'date_basis': value.get('date_basis') if value.get('date_basis') in
                ('today_inferred', 'relative_to_capture_date', 'weekday_inferred', 'explicit_date', 'year_inferred') else 'unknown'}


def normalize_manual(value):
    if not value:
        return None
    try:
        if not isinstance(value, str) or len(value) > 80 or not re.search(r'[T ]\d{2}:\d{2}', value):
            raise ValueError()
        parsed = datetime.fromisoformat(value)
        if parsed.tzinfo is None:
            parsed = parsed.replace(tzinfo=CHINA)
        return parsed.astimezone(CHINA).isoformat(timespec='minutes')
    except (ValueError, TypeError):
        raise ValueError('请填写有效的消息日期和时间（北京时间），或留空表示未知') from None


def infer_missing_times(messages):
    """Derive neighboring-time ranges without modifying or fabricating exact times."""
    def anchor(row):
        value = row.get('message_time')
        context = clean_context(row.get('time_context'))
        if not value and context:
            value = context['anchor_time']
        try:
            date = datetime.fromisoformat(value)
            if date.tzinfo is None:
                return None
            end = date + timedelta(days=1) - timedelta(minutes=1) if context and not row.get('message_time') and context['precision'] == 'day' else date
            return date, end, row['id']
        except (ValueError, TypeError):
            return None
    anchors = [anchor(row) for row in messages]
    following = [None] * len(messages)
    right = None
    for i in range(len(messages) - 1, -1, -1):
        following[i] = right
        if anchors[i]:
            right = anchors[i]
    result, left = [], None
    for i, original in enumerate(messages):
        row = dict(original)
        row.pop('inferred_time', None)
        right = following[i]
        if not anchors[i] and not row.get('time_inference_disabled') and (left or right):
            # Collected history may be out of order after scrolling; contradictory
            # neighbors provide no usable interval.
            if not (left and right and left[0] > right[1]):
                row['inferred_time'] = {'earliest': left[0].isoformat(timespec='minutes') if left else None,
                    'latest': right[1].isoformat(timespec='minutes') if right else None,
                    'source_ids': [a[2] for a in (left, right) if a],
                    'precision': 'range' if left and right else 'after' if left else 'before',
                    'source': 'neighbor_estimate', 'note': '按已采集的相邻时间推算，不代表精确发送时间；历史可能不完整或采集顺序不同'}
        result.append(row)
        if anchors[i]:
            left = anchors[i]
    return result


def timing_evidence(messages, now=None):
    now = (now or now_china()).astimezone(CHINA)
    hour = now.hour
    period = '凌晨' if hour < 6 else '上午' if hour < 12 else '下午' if hour < 18 else '晚上' if hour < 23 else '深夜'
    result = {'current_time': now.isoformat(timespec='seconds'), 'timezone': 'Asia/Shanghai',
              'current_period': period, 'source': '本机时钟，生成请求时读取', 'exact_intervals': []}
    for first, second in zip(messages, messages[1:]):
        if not first.get('message_time') or not second.get('message_time'):
            continue
        try:
            a, b = (datetime.fromisoformat(row['message_time']) for row in (first, second))
            if a.tzinfo is None or b.tzinfo is None or b < a or b > now:
                continue
            result['exact_intervals'].append({'from_id': first['id'], 'to_id': second['id'],
                                             'minutes': round((b - a).total_seconds() / 60, 1)})
        except (ValueError, TypeError):
            continue
    if messages and messages[-1].get('message_time'):
        try:
            last = datetime.fromisoformat(messages[-1]['message_time'])
            if last.tzinfo is not None and last <= now:
                result['latest_message_age_minutes'] = round((now - last).total_seconds() / 60, 1)
        except (ValueError, TypeError):
            pass
    return result
