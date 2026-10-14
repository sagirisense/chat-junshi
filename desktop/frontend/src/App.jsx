import { useCallback, useEffect, useRef, useState } from 'react'
import { Button, Input, Label, ListBox, Select, Modal, Tabs, TextArea, TextField } from '@heroui/react'
import { Download, Upload, Brain, Check, ChevronDown, ChevronRight, Copy, HeartHandshake, MessageCircle, Minus, Monitor, Moon, Pause, Pencil, Pin, Play, RotateCcw, Settings2, ShieldCheck, Sparkles, Square, Sun, Trash2, X } from 'lucide-react'
import ProfileDialog from './components/ProfileDialog'
import StickerLibrary from './components/StickerLibrary'
import StickerReply from './components/StickerReply'
import { providers } from './providers'
import { goalOptions, initialState, previewState, previewStickers, profileGroups, scopeLabels } from './domain'

const preview = new URLSearchParams(location.search).has('preview')
const pages = [['chat', '回复工作台'], ['profile', '关系档案'], ['memory', '长期记忆'], ['stickers', '表情库']]
const themeNames = { light: '浅色', dark: '深色', system: '跟随系统' }
function savedTheme() { try { return localStorage.getItem('junshi-theme') || 'system' } catch { return 'system' } }
function messageTimeLabel(message) {
  if (message.message_time) {
    const date = new Date(message.message_time)
    if (!Number.isNaN(date.getTime())) return `${date.toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false })} 北京时间 · ${message.message_time_source === 'user' ? '已手动核对' : '导入时间'}`
  }
  if (message.time_context) {
    const date = new Date(message.time_context.anchor_time)
    const display = Number.isNaN(date.getTime()) ? message.time_context.label : date.toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit', ...(message.time_context.precision === 'day' ? {} : { hour: '2-digit', minute: '2-digit', hour12: false }) })
    return `${display} · ${message.time_context.precision === 'day' ? '日期标签' : '所属时间段'}（原标签：${message.time_context.label}），待核对`
  }
  if (message.inferred_time) {
    const range = message.inferred_time
    const format = value => new Date(value).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false })
    const display = range.earliest && range.latest ? `${format(range.earliest)} — ${format(range.latest)}` : range.earliest ? `${format(range.earliest)}之后` : `${format(range.latest)}之前`
    return `约 ${display} · 前后时间推算，待核对`
  }
  return '发送时间未知'
}
function editTimeValue(message) {
  if (!message.message_time) return ''
  const date = new Date(message.message_time)
  return Number.isNaN(date.getTime()) ? '' : new Date(date.getTime() + 8 * 3600000).toISOString().slice(0, 16)
}

export default function App() {
  const [state, setState] = useState(() => preview ? previewState() : initialState)
  const [clock, setClock] = useState(() => new Date())
  useEffect(() => { const timer = setInterval(() => setClock(new Date()), 30000); return () => clearInterval(timer) }, [])
  const stateRef = useRef(state)
  const [page, setPage] = useState('chat')
  const [profileOpen, setProfileOpen] = useState(false)
  const [importedDraft, setImportedDraft] = useState(null)
  const profileOpenRef = useRef(false)
  const [edit, setEdit] = useState(null)
  const [settings, setSettings] = useState(false)
  const [model, setModel] = useState(() => { try { return localStorage.getItem('junshi-model') || '' } catch { return '' } })
  const [provider, setProvider] = useState(() => { try { return localStorage.getItem('junshi-provider') || 'codex' } catch { return 'codex' } })
  const [baseUrl, setBaseUrl] = useState(() => { try { return localStorage.getItem('junshi-api-url') || '' } catch { return '' } })
  const [apiKey, setApiKey] = useState('')
  const [apiSource, setApiSource] = useState(() => { try { return localStorage.getItem('junshi-api-source') || 'custom_openai' } catch { return 'custom_openai' } })
  const apiPreset = providers.find(item => item.id === apiSource) || providers.find(item => item.id === 'custom_openai')
  const apiConfig = { base_url: baseUrl, api_key: apiKey, protocol: apiPreset.protocol, source: apiPreset.id }
  const [customModel, setCustomModel] = useState(() => { try { return localStorage.getItem('junshi-api-model') || '' } catch { return '' } })
  const [modelList, setModelList] = useState([])
  const [loadingModels, setLoadingModels] = useState(false)
  const [modelError, setModelError] = useState('')
  const modelRequest = useRef(0)
  useEffect(() => { modelRequest.current += 1; setModelList([]); setModelError(''); setLoadingModels(false) }, [provider, apiSource, baseUrl, apiKey])
  const chosenModel = provider === 'custom' ? customModel : model
  useEffect(() => { try {
    localStorage.setItem('junshi-model', model); localStorage.setItem('junshi-provider', provider)
    localStorage.setItem('junshi-api-source', apiSource); localStorage.setItem('junshi-api-url', baseUrl); localStorage.setItem('junshi-api-model', customModel)
  } catch {} }, [model, provider, baseUrl, customModel, apiSource])
  const [goal, setGoal] = useState(state.profile.goal || '')
  const [assistanceMode, setAssistanceMode] = useState('reply')
  const [settingsExpanded, setSettingsExpanded] = useState(false)
  useEffect(() => { if (state.result && !state.busy) setSettingsExpanded(false) }, [state.busy, state.result?.task, state.result?.summary])
  const [drafts, setDrafts] = useState({})
  const [interestsByTitle, setInterestsByTitle] = useState({})
  const [polishStyle, setPolishStyle] = useState('自然一点')
  const [useHotspots, setUseHotspots] = useState(false)
  const [stickers, setStickers] = useState(() => preview ? previewStickers : [])
  const [stickerMode, setStickerMode] = useState('natural')
  const [allowIntimate, setAllowIntimate] = useState(false)
  const draft = drafts[state.title] || ''
  const interests = interestsByTitle[state.title] || ''
  const [customGoal, setCustomGoal] = useState(() => !!state.profile.goal && !goalOptions.includes(state.profile.goal))
  const [theme, setTheme] = useState(savedTheme)
  const [systemDark, setSystemDark] = useState(() => matchMedia('(prefers-color-scheme: dark)').matches)
  const [pinned, setPinned] = useState(false)
  const [error, setError] = useState('')
  const [toast, setToast] = useState('')
  const [confirmation, setConfirmation] = useState(null)
  const toastTimer = useRef(null)
  const messagesRef = useRef(null)
  const messageScrollRef = useRef({ title: null, lastId: 0, nearBottom: true })
  const activeMemory = state.memory.consent_enabled && !state.memory.paused
  const dark = theme === 'dark' || (theme === 'system' && systemDark)

  const notify = useCallback(text => {
    setToast(text); clearTimeout(toastTimer.current)
    toastTimer.current = setTimeout(() => setToast(''), 3200)
  }, [])
  const merge = useCallback(data => {
    const previous = stateRef.current
    const next = { ...previous, ...data }
    stateRef.current = next
    setState(next)
    if (next.title !== previous.title) setAllowIntimate(false)
    if (next.title !== previous.title || next.profile.goal !== previous.profile.goal) {
      setGoal(next.profile.goal || '')
      setCustomGoal(!!next.profile.goal && !goalOptions.includes(next.profile.goal))
    }
    if (next.title !== previous.title && next.title && !next.onboarded && !profileOpenRef.current) {
      profileOpenRef.current = true; setProfileOpen(true)
    }
  }, [])
  const call = useCallback(async (action, payload = {}) => {
    setError('')
    try {
      if (preview) { notify('演示模式，不读取或修改真实数据'); return null }
      const data = await window.go.main.App.Call(action, payload)
      if (data && 'messages' in data) merge(data)
      return data
    } catch (e) { setError(String(e.message || e)); return null }
  }, [merge, notify])

  const refreshStickers = useCallback(async () => {
    const data = await call('stickers')
    if (data) setStickers(data.stickers)
  }, [call])
  useEffect(() => { if (!preview) refreshStickers() }, [refreshStickers])
  async function copySticker(id) {
    try {
      if (preview) { notify('演示模式不复制真实素材'); return }
      await window.go.main.App.CopySticker(id)
      notify('图片已复制，请到微信粘贴并确认发送')
    } catch (e) { setError(String(e.message || e)) }
  }
  async function importStickers() {
    if (preview) { notify('演示模式不导入素材'); return }
    try {
      const result = await window.go.main.App.ImportStickers()
      await refreshStickers()
      notify(`已处理 ${result.count} 张图片；请核对标签并启用推荐`)
      if (result.failures.length) setError(result.failures.join('；'))
    } catch (e) { setError(String(e.message || e)) }
  }
  async function saveSticker(row) {
    const { preview: imagePreview, ...metadata } = row
    const data = await call('sticker_update', metadata)
    if (data) { setStickers(data.stickers); notify('表情标签已保存'); return true }
    return false
  }
  async function analyzeSticker(id) {
    if (preview) { notify('演示模式不分析真实素材'); return null }
    try {
      return await window.go.main.App.AnalyzeSticker(id, provider, chosenModel, provider === 'custom' ? apiConfig : {})
    } catch (e) { throw new Error(`${String(e.message || e)}；请确认当前模型支持图片理解`) }
  }
  async function transferStickerLibrary(action) {
    if (preview) { notify('演示模式不导入或导出真实素材'); return }
    try {
      if (action === 'export') {
        if (await window.go.main.App.ExportStickerLibrary()) notify('表情库已导出，包含图片、标签和推荐设置')
      } else {
        const result = await window.go.main.App.ImportStickerLibrary()
        if (result) { await refreshStickers(); notify(`新增 ${result.count} 张，跳过 ${result.skipped} 张重复图片`) }
      }
    } catch (e) { setError(String(e.message || e)) }
  }
  function deleteSticker(row) {
    setConfirmation({ text: `删除本地表情“${row.name}”？`, run: async () => {
      const data = await call('sticker_delete', { id: row.id, confirm: true })
      if (data) setStickers(data.stickers)
    } })
  }
  useEffect(() => {
    if (preview) return
    if (!window.go?.main?.App || !window.runtime?.EventsOnMultiple) { setError('桌面运行时尚未连接，请从完整项目的 run.bat 启动'); return }
    const off = window.runtime.EventsOnMultiple('service:state', merge, -1)
    const offError = window.runtime.EventsOnMultiple('service:error', value => setError(String(value)), -1)
    let alive = true
    call('state').then(result => {
      if (alive && result && !result.onboarded) { profileOpenRef.current = true; setProfileOpen(true) }
    })
    return () => { alive = false; off?.(); offError?.(); clearTimeout(toastTimer.current) }
  }, [call, merge])
  useEffect(() => {
    const query = matchMedia('(prefers-color-scheme: dark)')
    const update = event => setSystemDark(event.matches)
    query.addEventListener('change', update)
    return () => query.removeEventListener('change', update)
  }, [])
  useEffect(() => {
    document.documentElement.classList.toggle('dark', dark)
    document.documentElement.dataset.theme = dark ? 'dark' : 'light'
    try { localStorage.setItem('junshi-theme', theme) } catch {}
  }, [dark, theme])
  useEffect(() => {
    const list = messagesRef.current
    const previous = messageScrollRef.current
    const lastId = state.messages.at(-1)?.id || 0
    const switched = previous.title !== state.title
    if (list && (switched || (lastId > previous.lastId && previous.nearBottom))) {
      list.scrollTop = list.scrollHeight
    }
    messageScrollRef.current = { title: state.title, lastId, nearBottom: switched ? true : previous.nearBottom }
  }, [state.title, state.messages.at(-1)?.id])

  function openProfile() { setImportedDraft(null); profileOpenRef.current = true; setProfileOpen(true) }
  function closeProfile() { profileOpenRef.current = false; setProfileOpen(false) }
  async function copy(text) {
    try {
      if (preview) await navigator.clipboard.writeText(text)
      else await window.go.main.App.Copy(text)
      notify('已复制，粘贴后请自行确认发送')
    } catch { setError('复制失败，请手动选择正文复制') }
  }
  async function exportChat() {
    const data = await call('export')
    if (data) try { if (await window.go.main.App.Export(data.text)) notify('当前聊天已导出') } catch (e) { setError(String(e)) }
  }
  async function importFile(kind) {
    try {
      if (preview) { notify('演示模式不导入真实数据'); return }
      const text = await window.go.main.App.ImportJSON()
      if (!text) return
      const data = await call(kind === 'chat' ? 'prepare_import_chat' : 'prepare_import_profile', { text })
      if (!data) return
      if (kind === 'chat') setConfirmation({ text: `导入“${data.title}”的${data.count}条消息？${data.existing ? '会追加到同名会话，重复导入会产生重复消息。' : '将建立新会话。'}导入后需重新核对身份。`, run: () => call('import_chat', { text, confirm: true }) })
      else setConfirmation({ text: `导入“${data.title || state.title || '待绑定对象'}”的档案到编辑表单？核对后再选择保存方式。`, run: async () => {
        if (stateRef.current.capturing) await call('capture')
        if (data.title && data.title !== stateRef.current.title && !await call('bind_profile', { title: data.title })) return
        setImportedDraft({ ...data.profile, ...(data.title ? { chat_title: data.title } : {}) })
        profileOpenRef.current = true; setProfileOpen(true)
      } })
    } catch (e) { setError(String(e.message || e)) }
  }
  async function exportProfile() {
    const data = await call('export_profile')
    if (data) try { if (await window.go.main.App.ExportProfile(data.text)) notify('用户与关系档案已导出') } catch (e) { setError(String(e)) }
  }
  async function copyPacket() { const data = await call('packet'); if (data) copy(data.text) }
  async function pin() {
    try { if (!preview) await window.go.main.App.Pin(!pinned); setPinned(!pinned) } catch (e) { setError(String(e)) }
  }
  async function windowAction(action) {
    if (!preview) try { await window.go.main.App.WindowAction(action) } catch (e) { setError(String(e)) }
  }
  async function fetchModels() {
    setLoadingModels(true); setModelError('')
    const requestId = ++modelRequest.current
    try {
      if (preview) { setModelList(['demo-chat', 'demo-reasoner']); return }
      const data = await window.go.main.App.Call('models', { provider, api: apiConfig })
      if (requestId !== modelRequest.current) return
      setModelList(data.models)
      if (!data.models.length) setModelError('未返回模型，请手动填写模型名')
    } catch (e) { if (requestId === modelRequest.current) { setModelError(String(e.message || e)); setModelList([]) } }
    finally { if (requestId === modelRequest.current) setLoadingModels(false) }
  }
  function memoryAction(operation) {
    const prompts = {
      forget: `忘记“${state.title}”的关系记忆？共享用户档案和聊天文字会保留。`,
      revoke: '撤回记忆同意？已有资料保留，但停止用于分析。',
      delete_all: '撤回同意并删除全部关系记忆？此操作无法撤销，聊天文字会保留。',
    }
    if (prompts[operation]) setConfirmation({ text: prompts[operation], run: () => call('memory', { operation, confirm: true }) })
    else call('memory', { operation })
  }
  function clearChat() { setConfirmation({ text: '删除当前本地会话及聊天文字？关系档案和长期记忆会保留。', run: () => call('delete_session', { confirm: true }) }) }

  function conversation() {
    return <section className="glass-panel conversation">
      <div className="panel-heading"><h2>最近对话</h2><div className="panel-tools"><Button variant="ghost" size="sm" onPress={() => importFile('chat')}><Upload size={14} />导入对话</Button><Button variant="ghost" size="sm" isIconOnly aria-label="导出聊天" isDisabled={!state.title} onPress={exportChat}><Download size={16} /></Button><Button variant="ghost" size="sm" isIconOnly aria-label="删除当前会话" className="danger-text" isDisabled={!state.title} onPress={clearChat}><Trash2 size={15} /></Button></div></div>
      <div className="contact-header"><div className="avatar contact-avatar">{(state.profile.peer_label || state.title || '聊').slice(0, 1)}</div><div className="contact-info"><Select className="session-select" aria-label="选择已保存会话" placeholder="选择一段对话" selectedKey={state.title || null} onSelectionChange={value => { if (value) call('select', { title: String(value) }) }}><Select.Trigger><Select.Value /><Select.Indicator /></Select.Trigger><Select.Popover placement="bottom start" className="styled-select-menu"><ListBox>{state.sessions.map(title => <ListBox.Item key={title} id={title} textValue={title}><Label>{title}</Label><ListBox.ItemIndicator /></ListBox.Item>)}</ListBox></Select.Popover></Select><span><i className={state.capturing && state.active_title === state.title ? 'status-dot live' : 'status-dot'} />{state.capturing && state.active_title === state.title ? '正在读取微信' : state.title ? '已保存会话' : '等待微信单聊'}</span></div><Button variant="ghost" size="sm" isIconOnly aria-label="编辑关系档案" onPress={openProfile}><Pencil size={15} /></Button></div>
      <div className="messages" ref={messagesRef} onScroll={event => { const list = event.currentTarget; messageScrollRef.current.nearBottom = list.scrollHeight - list.scrollTop - list.clientHeight < 60 }}>
        {!state.messages.length ? <div className="empty-state"><MessageCircle size={38} /><h3>先听听，你们聊到哪里了</h3><p>打开微信单聊并保持未最小化，<br />开启采集后核对原文。</p><Button variant="secondary" onPress={() => call('capture')}><Play size={14} />{state.capturing ? '暂停采集' : '开启采集'}</Button></div> : <>
          <p className="scope-note">时间标签仅代表所属消息组；未识别的发送时间保持未知</p>
          {state.messages.map(message => <div key={message.id} className={`message-row ${message.sender === 'me' ? 'mine' : ''}`}><div className="avatar message-avatar">{message.sender === 'me' ? '我' : (state.profile.peer_label || state.title).slice(0, 1)}</div><div className="message-content"><div className="message-meta"><span>{message.sender === 'me' ? '我' : '对方'}</span><span>#{message.id}</span><button aria-label={`纠正消息${message.id}的正文或时间`} onClick={() => setEdit({ ...message, message_time: editTimeValue(message), time_edit: false })}><Pencil size={12} /></button><button className="danger-text" aria-label={`删除消息${message.id}`} onClick={() => setConfirmation({ title: '删除这条消息？', text: '只删除本工具中的这一条记录，微信原文和关系记忆保留；相关旧建议会失效。', run: () => call('delete_message', { title: state.title, id: message.id, confirm: true }) })}><Trash2 size={12} /></button></div><div className="bubble">{message.text}</div><small className="message-time">{messageTimeLabel(message)}</small></div></div>)}
        </>}
      </div>
      <div className="conversation-footer"><label className="check-row"><input type="checkbox" checked={state.confirmed} disabled={!state.title || state.blocked} onChange={event => call('confirm', { confirmed: event.target.checked })} /><span>已核对说话人，当前为单聊</span><ShieldCheck size={15} /></label><div className="conversation-footer-line"><span>{state.messages.length} 条已采集</span><Button size="sm" variant="ghost" isDisabled={!state.confirmed} onPress={copyPacket}><Copy size={12} />复制 Skill 输入</Button></div></div>
    </section>
  }

  function advice() {
    const resultMatches = !state.result || (state.result.task || 'reply') === assistanceMode
    return <section className="glass-panel advice">
      <div className="panel-heading"><h2><Sparkles size={21} className="accent-icon" />军师建议</h2><span className="model-caption">{chosenModel || (provider === 'custom' ? '自定义 API' : 'Codex 默认模型')}</span></div>
      <div className="advice-controls">
        <div className="assistance-tabs" role="group" aria-label="辅助功能">{[['reply', '回复建议'], ['polish', '润色草稿'], ['topics', '找话题']].map(([id, label]) => <Button key={id} size="sm" variant={assistanceMode === id ? 'secondary' : 'ghost'} aria-pressed={assistanceMode === id} isDisabled={state.busy} onPress={() => { setAssistanceMode(id); setSettingsExpanded(id !== 'reply'); call('invalidate_assistance', { title: state.title }) }}>{label}</Button>)}</div>
        <button className="settings-disclosure" aria-expanded={settingsExpanded} aria-controls="assistance-settings" onClick={() => setSettingsExpanded(value => !value)}><span>{assistanceMode === 'reply' ? `回复目标：${goal || '自然接话'}` : assistanceMode === 'polish' ? '草稿与润色方向' : '兴趣与资讯设置'}</span><span>{settingsExpanded ? '收起' : '调整'}<ChevronDown size={14} /></span></button>
        <div id="assistance-settings" className="assistance-settings" hidden={!settingsExpanded}>
        {assistanceMode === 'polish' && <><TextField value={draft} isDisabled={state.busy} onChange={value => { setDrafts(previous => ({ ...previous, [state.title]: value })); call('invalidate_assistance', { title: state.title }) }}><Label>你想发送的话</Label><TextArea rows={3} maxLength={2000} placeholder="保留你的原意，帮你调整表达" /></TextField><Select aria-label="润色方向" selectedKey={polishStyle} isDisabled={state.busy} onSelectionChange={value => { setPolishStyle(String(value)); call('invalidate_assistance', { title: state.title }) }}><Label>润色方向</Label><Select.Trigger><Select.Value /><Select.Indicator /></Select.Trigger><Select.Popover><ListBox>{['自然一点', '简短一点', '温柔一点', '轻松一点', '表达边界'].map(value => <ListBox.Item key={value} id={value} textValue={value}><Label>{value}</Label><ListBox.ItemIndicator /></ListBox.Item>)}</ListBox></Select.Popover></Select></>}
        {assistanceMode === 'topics' && <><TextField value={interests} isDisabled={state.busy} onChange={value => { setInterestsByTitle(previous => ({ ...previous, [state.title]: value })); call('invalidate_assistance', { title: state.title }) }}><Label>对方明确的兴趣（可选，仅本次使用）</Label><Input maxLength={200} placeholder="例如：喜欢摄影；只是提过某部电影" /></TextField><label className="check-row"><input type="checkbox" checked={useHotspots} disabled={state.busy} onChange={event => { setUseHotspots(event.target.checked); call('invalidate_assistance', { title: state.title }) }} /><span>结合近期科技、游戏与文化资讯</span></label><p className="status-line">只获取公开资讯，不向资讯源发送聊天或档案。</p></>}
        {assistanceMode === 'reply' && <div className="goal-picker">
          <Select className="goal-select" placeholder="选择这次的回复目标" selectedKey={customGoal ? '__custom__' : goal || null} onSelectionChange={selected => {
            const value = String(selected || '')
            setCustomGoal(value === '__custom__')
            if (value === '__custom__') setGoal('')
            else { setGoal(value); if (state.title) call('goal', { goal: value }) }
          }}>
            <Label>这次，想怎么推进？</Label>
            <Select.Trigger><Select.Value /><Select.Indicator /></Select.Trigger>
            <Select.Popover placement="bottom start" className="styled-select-menu"><ListBox aria-label="回复目标">
              {[...goalOptions, '__custom__'].map(value => <ListBox.Item key={value} id={value} textValue={value === '__custom__' ? '自定义目标…' : value}><Label>{value === '__custom__' ? '自定义目标…' : value}</Label><ListBox.ItemIndicator /></ListBox.Item>)}
            </ListBox></Select.Popover>
          </Select>
          {customGoal && <TextField className="custom-goal" value={goal} onChange={setGoal} onBlur={() => { if (state.title && goal !== state.profile.goal) call('goal', { goal }) }}>
            <Label>你的具体目标</Label>
            <Input autoFocus placeholder="例如：轻松约对方周末喝咖啡" maxLength={200} />
          </TextField>}
        </div>}
        </div>
        <div className="generation-actions">
        <div className="sticker-controls"><Select className="goal-select sticker-mode-select" aria-label="搭配表情模式" selectedKey={stickerMode} isDisabled={state.busy} onSelectionChange={value => { setStickerMode(String(value)); call('invalidate_assistance', { title: state.title }) }}><Label>搭配表情</Label><Select.Trigger><Select.Value /><Select.Indicator /></Select.Trigger><Select.Popover placement="bottom start" className="styled-select-menu"><ListBox>{[['none', '纯文字'], ['natural', '自然搭配'], ['prefer', '优先表情']].map(([id, name]) => <ListBox.Item key={id} id={id} textValue={name}><Label>{name}</Label><ListBox.ItemIndicator /></ListBox.Item>)}</ListBox></Select.Popover></Select>{stickerMode !== 'none' && <><span>{stickers.filter(row => row.enabled && (!row.intimate || allowIntimate)).length} 张可推荐</span><label className="check-row"><input type="checkbox" checked={allowIntimate} disabled={state.busy} onChange={event => { setAllowIntimate(event.target.checked); call('invalidate_assistance', { title: state.title }) }} />允许亲密表情</label><Button size="sm" variant="ghost" onPress={() => setPage('stickers')}>管理表情</Button></>}</div>
        <Button className="violet-button generate-button" fullWidth isDisabled={state.busy || !state.confirmed || state.blocked || (assistanceMode === 'polish' && !draft.trim())} isPending={state.busy} onPress={() => call('analyze', { task: assistanceMode, draft, style: polishStyle, interests, use_hotspots: useHotspots, sticker_mode: stickerMode, allow_intimate: allowIntimate, mode: 'quick', model: chosenModel, goal, provider, api: provider === 'custom' ? apiConfig : {} })}><Sparkles size={18} />{state.busy ? '正在生成…' : assistanceMode === 'polish' ? '优化这段话' : assistanceMode === 'topics' ? '推荐话题' : '快速回复'}</Button>
        {assistanceMode === 'reply' && <Button variant="ghost" fullWidth isDisabled={state.busy || !state.confirmed || state.blocked} onPress={() => call('analyze', { mode: 'deep', sticker_mode: stickerMode, allow_intimate: allowIntimate, model: chosenModel, goal, provider, api: provider === 'custom' ? apiConfig : {} })}><Brain size={16} />深入分析</Button>}
        </div>
        <p className="status-line" role="status">{state.status}</p>
        {assistanceMode === 'topics' && state.assistance_notice && <p className="status-line">{state.assistance_notice}</p>}
        <p className="status-line clock-line">北京时间：{clock.toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai', hour12: false })}</p>
        {!!state.reply_metrics?.total_s && <details className="status-line"><summary>本次用时 {state.reply_metrics.total_s} 秒</summary><p>连接准备：{state.reply_metrics.initialize_s ?? '—'} 秒 · 会话就绪：{state.reply_metrics.thread_ready_s ?? '—'} 秒 · 首条候选：{state.reply_metrics.first_candidate_s ?? '—'} 秒</p></details>}
        {state.result?.sticker_notice && !state.busy && <p className="status-line" role="status">{state.result.sticker_notice}</p>}
      </div>
      <div className="advice-scroll">{state.busy ? state.reply_preview?.length ? <><p className="status-line">候选预览，完整结果核对后可复制</p>{state.reply_preview.map((candidate, index) => <article className="reply-card" key={index}><p className="reply-text">{candidate.text || '建议暂时不回复'}</p></article>)}</> : <div className="empty-state thinking"><Sparkles size={30} /><h3>正在整理适合你的表达</h3><p>{state.status}</p></div> : !state.result || !resultMatches ? <div className="empty-state"><Sparkles size={30} /><h3>{assistanceMode === 'polish' ? '先写下你真正想说的话' : assistanceMode === 'topics' ? '从对方聊过的事，自然开个头' : '不用急着想一句完美回复'}</h3><p>{assistanceMode === 'polish' ? '保留草稿原意，结合当前关系和时间调整表达。' : assistanceMode === 'topics' ? '根据历史与兴趣找话题；资讯不合适时不硬凑热点。' : '核对说话人后，获取自然的表达和各自的优点与代价。'}</p></div> : <>
        <details className="judgment judgment-compact"><summary><MessageCircle size={17} /><span>{state.result.summary}</span><ChevronDown size={14} /></summary><div className="judgment-body"><p>{state.result.recommendation_reason}</p>{[['事实', 'facts'], ['推测', 'hypotheses'], ['未知', 'unknowns']].map(([label, key]) => <div key={key}><b>{label}</b>{state.result[key].map((item, index) => <p key={index}>{item}</p>)}</div>)}</div></details>
        {state.result.candidates.map((candidate, index) => <article key={index} className={`reply-card ${index === state.result.recommended_index ? 'recommended' : ''}`}><div className="reply-topline"><span className={index === state.result.recommended_index ? 'recommend-badge' : 'alternative-label'}>{index === state.result.recommended_index ? '推荐' : '另一种表达'}</span><Button variant="secondary" size="sm" isDisabled={!candidate.text} onPress={() => copy(candidate.text)}><Copy size={13} />复制文字</Button></div>{candidate.topic_title && <h3 className="topic-title">{candidate.topic_title}</h3>}{candidate.basis && <p className="topic-basis">{{ history: '历史话题', preference: '兴趣话题', hotspot: '近期资讯', general: '探索性话题' }[candidate.basis]} · {{ confirmed: '明确兴趣', mentioned: '只是提过', inferred: '暂定推测', unknown: '喜好未知' }[candidate.preference_level]}{candidate.evidence_ids?.length ? ` · 消息 ${candidate.evidence_ids.map(id => `#${id}`).join('、')}` : ''}</p>}<p className="reply-text">{candidate.text || (candidate.sticker ? '建议用表情回应' : '建议暂时不回复')}</p><StickerReply candidate={candidate} stickers={stickers} mode={stickerMode} allowIntimate={allowIntimate} onCopy={copySticker} />{candidate.sources?.length > 0 && <div className="topic-sources">{candidate.sources.map(source => <a key={source.id} href={source.url} target="_blank" rel="noopener noreferrer" onClick={event => { if (!preview && window.runtime?.BrowserOpenURL) { event.preventDefault(); window.runtime.BrowserOpenURL(source.url) } }}>{source.title} · {source.publisher} · {new Date(source.published_at).toLocaleDateString('zh-CN', { timeZone: 'Asia/Shanghai' })}</a>)}</div>}<div className="tradeoffs"><div className="pro"><Check size={13} /><span><b>优点：</b>{candidate.pros.join('，')}</span></div><div className="con"><ShieldCheck size={13} /><span><b>代价：</b>{candidate.cons.join('，')}</span></div></div><details><summary>适用场景与后续<ChevronRight size={12} /></summary><p>{candidate.suitable_when}</p><p>{candidate.next_step}</p></details></article>)}
      </>}</div>
      <div className="advice-footer"><ShieldCheck size={13} />本地识别 · 分析时提交文字 · 发送由你决定</div>
    </section>
  }

  function profiles() {
    return <section className="glass-panel page-panel"><div className="page-intro"><div className="page-emblem"><HeartHandshake size={28} /></div><div><h1>理解你，也理解你们的局面</h1><p>{state.title ? `当前档案：${state.title}` : '先填写准确的微信单聊名称，为对象绑定独立档案。'}</p></div><div className="profile-actions"><Button variant="secondary" onPress={() => importFile('profile')}><Upload size={15} />导入档案</Button><Button variant="secondary" isDisabled={!Object.values(state.profile).some(value => value !== '' && value != null)} onPress={exportProfile}><Download size={15} />导出档案</Button><Button className="violet-button" onPress={openProfile}><Pencil size={15} />{state.onboarded ? '编辑关系档案' : '建立关系档案'}</Button></div></div><div className="profile-grid">{profileGroups.map((group, index) => <section className="profile-section" key={index}><h2>{['关于你', '关于对象', '你们的局面'][index]}</h2><dl>{group.map(([key, label]) => <div key={key}><dt>{label}</dt><dd>{state.profile[key] || '尚未填写'}</dd></div>)}</dl></section>)}</div><p className="footnote">未知项留空，主观评分只由你填写。{activeMemory ? '保存时可选择写入长期记忆或仅用于本次。' : state.memory.paused ? '记忆已暂停；本次明确填写的信息仍可用于回复。' : '记忆尚未启用，本次填写不会自动写入长期档案。'}</p></section>
  }
  function memory() {
    const label = !state.memory.consent_enabled ? '尚未启用' : state.memory.paused ? '已暂停' : '已启用'
    return <section className="glass-panel page-panel"><div className="page-intro"><div className="page-emblem"><Brain size={28} /></div><div><h1>记住重要的事，不积累全部聊天</h1><p>只召回当前对象，事实与推测分别保存。</p></div><span className={`memory-status ${activeMemory ? 'enabled' : ''}`}><i className="status-dot" />{label}</span></div><div className="memory-controls">{!state.memory.consent_enabled ? <Button className="violet-button" onPress={openProfile}>了解并启用记忆</Button> : <Button variant="secondary" onPress={() => memoryAction(state.memory.paused ? 'resume' : 'pause')}>{state.memory.paused ? <Play size={15} /> : <Pause size={15} />}{state.memory.paused ? '恢复记忆' : '暂停记忆'}</Button>}<Button variant="secondary" isDisabled={!state.memory.consent_enabled} onPress={() => memoryAction('undo')}><RotateCcw size={15} />撤销最近更新</Button></div>{state.memory_notice && <p className="inline-notice">{state.memory_notice}</p>}<div className="memory-list">{state.memory_rows.length ? state.memory_rows.map(row => <div className="memory-row" key={row.id}><span className={`scope-pill ${row.scope}`}>{scopeLabels[row.scope]}</span><div><p>{row.value}</p><small>{row.scope === 'hypothesis' ? '暂定推测，可纠正 · ' : ''}{row.source_type === 'user_explicit' ? '你明确填写' : row.source_type === 'tool' ? '聊天证据' : '分析推测'}{row.source_ref ? ` · ${row.source_ref}` : ''}</small></div></div>) : <div className="empty-state"><Brain size={34} /><h3>这里还没有关系记忆</h3><p>启用后保存精简档案，后续只补充有证据的关键变化。</p></div>}</div><div className="memory-boundary"><h3>记忆由你掌握</h3><p>聊天文字与关系记忆分别管理。暂停时不读写长期记忆；撤回同意可保留资料，或直接删除。</p><div className="footer-actions"><Button variant="secondary" isDisabled={!state.title} onPress={() => memoryAction('forget')}><Trash2 size={15} />忘记当前对象</Button><Button variant="secondary" isDisabled={!state.memory.consent_enabled} onPress={() => memoryAction('revoke')}><ShieldCheck size={15} />撤回同意，保留资料</Button><Button variant="ghost" className="danger-text" onPress={() => memoryAction('delete_all')}><Trash2 size={15} />删除全部关系记忆</Button></div></div></section>
  }

  return <div className="app-shell">
    <header className="titlebar" style={{ '--wails-draggable': 'drag' }}>
      <div className="brand"><div className="brand-symbol"><MessageCircle size={20} /></div><strong>聊天军师</strong>{preview && <span className="preview-badge">虚构数据预览</span>}</div>
      <div className="titlebar-actions" style={{ '--wails-draggable': 'no-drag' }}><div className="theme-toggle" aria-label="界面主题">{[['light', Sun], ['dark', Moon], ['system', Monitor]].map(([value, Icon]) => <Button key={value} variant="ghost" size="sm" isIconOnly aria-label={`${themeNames[value]}主题`} aria-pressed={theme === value} className={theme === value ? 'selected' : ''} onPress={() => setTheme(value)}><Icon size={15} /></Button>)}</div><Button variant="ghost" size="sm" isIconOnly aria-label="窗口置顶" aria-pressed={pinned} className={pinned ? 'pinned' : ''} onPress={pin}><Pin size={16} /></Button><Button variant="ghost" size="sm" isIconOnly aria-label="最小化窗口" onPress={() => windowAction('minimize')}><Minus size={18} /></Button><Button variant="ghost" size="sm" isIconOnly aria-label="最大化窗口" onPress={() => windowAction('maximize')}><Square size={13} /></Button><Button variant="ghost" size="sm" isIconOnly aria-label="关闭窗口" className="window-close" onPress={() => windowAction('close')}><X size={18} /></Button></div>
    </header>

    <Tabs className="application-tabs" selectedKey={page} onSelectionChange={value => setPage(String(value))}>
      <div className="nav-bar"><Tabs.List aria-label="工作页面" className="main-nav">{pages.map(([id, name]) => <Tabs.Tab key={id} id={id}>{name}<Tabs.Indicator /></Tabs.Tab>)}</Tabs.List><div className="nav-actions"><Button variant="secondary" size="sm" aria-label="模型设置" onPress={() => setSettings(true)}><Settings2 size={16} />模型设置</Button><Button variant="secondary" size="sm" className={state.capturing ? 'capture-on' : ''} onPress={() => call('capture')}>{state.capturing ? <Pause size={13} /> : <Play size={13} />}{state.capturing ? '暂停采集' : '开启采集'}</Button></div></div>
      {error && <div className="error-banner" role="alert"><span>{error}</span><button aria-label="关闭错误提示" onClick={() => setError('')}><X size={16} /></button></div>}
      <Tabs.Panel id="chat" className="tab-panel chat-layout">{conversation()}{advice()}</Tabs.Panel>
      <Tabs.Panel id="profile" className="tab-panel">{profiles()}</Tabs.Panel>
      <Tabs.Panel id="memory" className="tab-panel">{memory()}</Tabs.Panel>
      <Tabs.Panel id="stickers" className="tab-panel"><StickerLibrary stickers={stickers} busy={state.busy} onImport={importStickers} onSave={saveSticker} onDelete={deleteSticker} onCopy={copySticker} onAnalyze={analyzeSticker} onTransfer={transferStickerLibrary} /></Tabs.Panel>
    </Tabs>
    <ProfileDialog open={profileOpen} onClose={closeProfile} state={state} call={call} notify={notify} importedDraft={importedDraft} />
    <Modal isOpen={settings} onOpenChange={setSettings}><Modal.Backdrop variant="blur" className="dialog-backdrop"><Modal.Container placement="center"><Modal.Dialog className="small-dialog model-dialog"><Modal.CloseTrigger /><Modal.Header><Modal.Heading>模型设置</Modal.Heading></Modal.Header><Modal.Body><div className="model-settings">
      <Select selectedKey={provider} onSelectionChange={value => { setProvider(String(value)); setModelList([]); setModelError('') }}><Label>接入方式</Label><Select.Trigger><Select.Value /><Select.Indicator /></Select.Trigger><Select.Popover placement="bottom start" className="styled-select-menu"><ListBox><ListBox.Item id="codex" textValue="Codex 登录"><Label>Codex 登录</Label></ListBox.Item><ListBox.Item id="custom" textValue="自定义 API"><Label>自定义 API</Label></ListBox.Item></ListBox></Select.Popover></Select>
      {provider === 'custom' && <>
        <Select selectedKey={apiPreset.id} onSelectionChange={value => {
          const preset = providers.find(item => item.id === value)
          if (!preset) return
          setApiSource(preset.id); setBaseUrl(preset.base); setApiKey(''); setCustomModel(''); setModelList([]); setModelError('')
        }}><Label>模型来源</Label><Select.Trigger><Select.Value /><Select.Indicator /></Select.Trigger><Select.Popover placement="bottom start" className="styled-select-menu"><ListBox>{providers.map(item => <ListBox.Item id={item.id} key={item.id} textValue={item.name}><Label>{item.name}</Label><ListBox.ItemIndicator /></ListBox.Item>)}</ListBox></Select.Popover></Select>
        <TextField value={baseUrl} onChange={value => { setBaseUrl(value); setModelList([]) }}><Label>API 基础地址</Label><Input placeholder="https://api.example.com/v1" /></TextField>
        <TextField value={apiKey} onChange={value => { setApiKey(value); setModelList([]) }}><Label>API Key</Label><Input type="password" autoComplete="off" placeholder="本次运行有效；本地服务可留空" /></TextField>
        <p className="footnote">当前协议：{apiPreset.protocol === 'anthropic' ? 'Anthropic Messages（地址不含 /v1）' : apiPreset.protocol === 'gemini' ? 'Gemini（地址包含 /v1beta）' : 'OpenAI Chat Completions'}。切换来源需重新填写密钥，密钥仅本次运行有效。</p>
      </>}
      <Button variant="secondary" isPending={loadingModels} isDisabled={loadingModels} onPress={fetchModels}>获取模型列表</Button>
      {modelError && <p className="inline-error" role="alert">{modelError}</p>}
      {modelList.length > 0 && <Select placeholder="从获取的模型中选择" selectedKey={modelList.includes(chosenModel) ? chosenModel : null} onSelectionChange={value => provider === 'custom' ? setCustomModel(String(value)) : setModel(String(value))}><Label>可选模型</Label><Select.Trigger><Select.Value /><Select.Indicator /></Select.Trigger><Select.Popover placement="bottom start" className="styled-select-menu"><ListBox>{modelList.map(value => <ListBox.Item id={value} key={value} textValue={value}><Label>{value}</Label><ListBox.ItemIndicator /></ListBox.Item>)}</ListBox></Select.Popover></Select>}
      <TextField value={chosenModel} onChange={provider === 'custom' ? setCustomModel : setModel}><Label>模型名称</Label><Input placeholder={provider === 'custom' ? '选择或手动填写模型 ID' : '留空使用 Codex 默认模型'} /></TextField>
      <p className="footnote">{provider === 'custom' ? '分析时将近期聊天与当前档案发送到此 API。模型列表不代表所有模型都支持聊天或具有调用权限。' : '沿用本机 Codex 登录，不修改全局配置。'}</p>
    </div></Modal.Body><Modal.Footer><Button className="violet-button" onPress={() => setSettings(false)}>完成</Button></Modal.Footer></Modal.Dialog></Modal.Container></Modal.Backdrop></Modal>
    <Modal isOpen={Boolean(edit)} onOpenChange={value => { if (!value) setEdit(null) }}><Modal.Backdrop variant="blur" className="dialog-backdrop"><Modal.Container placement="center"><Modal.Dialog className="small-dialog"><Modal.CloseTrigger /><Modal.Header><Modal.Heading>纠正采集原文</Modal.Heading></Modal.Header><Modal.Body>{edit && <><div className="sender-options"><label><input type="radio" name="sender" checked={edit.sender === 'me'} onChange={() => setEdit({ ...edit, sender: 'me' })} />用户（我）</label><label><input type="radio" name="sender" checked={edit.sender === 'peer'} onChange={() => setEdit({ ...edit, sender: 'peer' })} />对象（对方）</label></div><TextField className="correction-field" value={edit.text} onChange={text => setEdit({ ...edit, text })}><Label>消息正文</Label><TextArea rows={5} /></TextField><TextField className="correction-field" value={edit.message_time || ''} onChange={message_time => setEdit({ ...edit, message_time, time_edit: true })}><Label>核对发送时间（北京时间）</Label><Input type="datetime-local" /></TextField><p className="status-line">原识别：{edit.time_context?.label || '无时间标签'}。填写明确时间后才计算间隔；修改为留空表示未知。</p><Button variant="ghost" onPress={() => setEdit({ ...edit, message_time: '', time_edit: true })}>将时间设为未知</Button></>}</Modal.Body><Modal.Footer><Button variant="secondary" onPress={() => setEdit(null)}>取消</Button><Button className="violet-button" onPress={async () => { if (await call('correct', edit)) { setEdit(null); notify('消息已纠正') } }}>保存纠正</Button></Modal.Footer></Modal.Dialog></Modal.Container></Modal.Backdrop></Modal>
    <Modal isOpen={Boolean(confirmation)} onOpenChange={value => { if (!value) setConfirmation(null) }}><Modal.Backdrop variant="blur" className="dialog-backdrop"><Modal.Container placement="center"><Modal.Dialog className="small-dialog"><Modal.CloseTrigger /><Modal.Header><Modal.Heading>确认操作</Modal.Heading></Modal.Header><Modal.Body><p>{confirmation?.text}</p></Modal.Body><Modal.Footer><Button variant="secondary" onPress={() => setConfirmation(null)}>取消</Button><Button variant="danger" onPress={async () => { await confirmation.run(); setConfirmation(null) }}>确认</Button></Modal.Footer></Modal.Dialog></Modal.Container></Modal.Backdrop></Modal>
    {toast && <div className="toast" role="status"><Check size={16} />{toast}</div>}
  </div>
}
