export const initialState = {
  title: '', active_title: '', messages: [], sessions: [], confirmed: false,
  capturing: false, blocked: false, busy: false, status: '正在连接本地采集服务…',
  result: null, profile: {}, memory: {}, memory_rows: [], memory_notice: '', onboarded: false,
  reply_preview: [], reply_metrics: {},
  assistance_notice: '',
}

export const goalOptions = ['自然接话', '继续了解对方', '主动邀约', '表达好感', '确认关系', '修复误会', '表达边界', '礼貌收尾', '先弄清局面']
export const profileGroups = [
  [['user_gender', '你的性别', '未填写'], ['user_mbti', '你的 MBTI', '不知道可以留空'], ['user_score', '你的主观综合评分', '0–100，仅代表你的评价'], ['user_strengths', '你的优势与短板', '表达习惯、优势、容易困扰你的地方'], ['user_boundaries', '你的偏好与边界', '希望怎样相处，哪些事不能接受']],
  [['peer_gender', '对象的性别', '未填写'], ['peer_label', '对象的称呼或代号', '例如：小林'], ['peer_mbti', '对象的 MBTI', '未知可以留空'], ['peer_score', '你对对象的主观综合评分', '0–100，不代替真实互动'], ['relationship', '你们现在是什么关系', '认识方式、发展多久、当前阶段']],
  [['history', '最近的关键互动', '最近两三件事、双方原话和投入情况'], ['goal', '关系中的整体目标', '推进、确认、修复、退出，或先弄清局面'], ['emotion', '你当下的感受', '最难受的点、强度0–10；是否急需回复']],
]
export const scopeLabels = { user: '用户档案', object: '对象档案', relationship: '关系状态', event: '关键事件', hypothesis: '暂定推测' }

export function previewState() {
  return {
    ...initialState, title: '小林', active_title: '小林', sessions: ['小林', '阿远'],
    confirmed: true, onboarded: true, status: '建议已准备好，选择适合你的表达',
    profile: { peer_label: '小林', relationship: '认识两周，见过一次。相处轻松，还在互相了解。', goal: '自然接话' },
    memory: { consent_enabled: true, paused: false },
    messages: [
      { id: 1, sender: 'peer', text: '今天加班好累' },
      { id: 2, sender: 'me', text: '刚到家吗' },
      { id: 3, sender: 'peer', text: '嗯，刚到家' },
      { id: 4, sender: 'peer', text: '今晚只想躺着哈哈' },
    ],
    result: {
      summary: '先接住疲惫，轻松回应', recommendation_reason: '给对方休息空间，不急着推进。',
      facts: ['[#3] 对方已经到家', '[#4] 对方表示今晚想休息'],
      hypotheses: ['可能更需要轻松的陪伴，也可能只是想休息'], unknowns: ['是否愿意继续聊天'], recommended_index: 0,
      candidates: [
        { text: '那今晚给沙发打工，别给自己加班了哈哈', pros: ['轻松承接', '不催回复'], cons: ['不主动推进话题'], suitable_when: '你们平时会开轻松的玩笑', next_step: '她继续接梗就顺着聊；没回就让她休息' },
        { text: '今晚的任务就是好好躺着', pros: ['简短自然'], cons: ['话题可能结束'], suitable_when: '你想温和收尾', next_step: '今晚不追问，之后自然联系' },
        { text: '那就歇着，明天再聊', pros: ['留出空间'], cons: ['暂时收尾'], suitable_when: '对方明显需要休息', next_step: '之后找合适的时机联系' },
      ],
    },
    memory_rows: [
      { id: 'demo1', scope: 'relationship', field: 'stage', value: '认识两周，见过一次，尚未确认关系', source_type: 'user_explicit', confidence: 'high' },
      { id: 'demo2', scope: 'user', field: 'boundaries', value: '希望交流自然，尊重彼此节奏', source_type: 'user_explicit', confidence: 'high' },
    ],
  }
}
