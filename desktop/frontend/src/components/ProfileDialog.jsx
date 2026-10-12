import { useEffect, useState } from 'react'
import { Button, Input, Label, Modal, TextArea, TextField } from '@heroui/react'
import { Brain, ChevronLeft, ChevronRight, HeartHandshake } from 'lucide-react'
import { profileGroups } from '../domain'

export default function ProfileDialog({ open, onClose, state, call, notify, importedDraft }) {
  const [step, setStep] = useState(0)
  const [form, setForm] = useState({})
  const [consent, setConsent] = useState(false)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const active = state.memory.consent_enabled && !state.memory.paused
  useEffect(() => {
    if (open) { setForm({ ...state.profile, ...importedDraft }); setStep(0); setConsent(false); setError('') }
  }, [open, importedDraft])

  function field(key, label, placeholder) {
    const long = ['history', 'relationship', 'user_strengths', 'user_boundaries', 'emotion'].includes(key)
    const Control = long ? TextArea : Input
    return <TextField key={key} className="profile-field" value={form[key] || ''} onChange={value => setForm(old => ({ ...old, [key]: value }))}>
      <Label>{label}{key.endsWith('_score') && <span className="optional">可选 · 主观评价</span>}</Label>
      <Control placeholder={placeholder} maxLength={200} rows={long ? 3 : undefined} type={key.endsWith('_score') ? 'number' : 'text'} min={key.endsWith('_score') ? 0 : undefined} max={key.endsWith('_score') ? 100 : undefined} />
    </TextField>
  }

  async function save(persist) {
    if (!state.title && !form.chat_title?.trim()) { setError('请填写与微信顶部一致的单聊名称'); setStep(1); return }
    setSaving(true); setError('')
    try {
      if (!state.title && !await call('bind_profile', { title: form.chat_title })) throw new Error('会话绑定失败，请检查名称')
      if (persist && !active) {
        if (!consent) throw new Error('请明确同意保存，或选择仅用于本次')
        if (!await call('memory', { operation: 'enable', consent: true })) throw new Error('未能启用记忆，请重试')
      }
      if (!await call('profile', { profile: form, persist })) throw new Error('档案未保存，请查看错误提示')
      notify(persist ? '已保存关系档案，可随时查看和撤销' : '档案仅用于本次，不写入长期记忆')
      onClose()
    } catch (e) { setError(String(e.message || e)) }
    finally { setSaving(false) }
  }

  return <Modal isOpen={open} onOpenChange={value => { if (!value && !saving) onClose() }}>
    <Modal.Backdrop variant="blur" className="dialog-backdrop">
      <Modal.Container placement="center" scroll="inside">
        <Modal.Dialog className="profile-dialog" aria-label="关系建档">
          <Modal.CloseTrigger isDisabled={saving} />
          <Modal.Header className="profile-header">
            <div className="dialog-emblem"><HeartHandshake size={25} /></div>
            <Modal.Heading>先认识你，再一起想办法</Modal.Heading>
            <p>不知道的地方留空就好。你始终可以先处理眼前回复。</p>
          </Modal.Header>
          <div className="wizard-tabs" aria-label="建档步骤">
            {['关于你', '关于对象', '你们的局面', '保存与记忆'].map((name, index) => <button key={name} disabled={saving} className={step === index ? 'selected' : ''} onClick={() => setStep(index)}><span>{index + 1}</span>{name}</button>)}
          </div>
          <Modal.Body className="profile-body">
            {step < 3 ? <>
              {step === 1 && !state.title && field('chat_title', '微信单聊名称', '请填写与微信顶部一致的会话名称')}
              {profileGroups[step].map(([key, label, placeholder]) => field(key, label, placeholder))}
            </> : <div className="consent-body">
              <Brain size={36} /><h2>下次，不必从头讲起</h2>
              <p>启用后在本机保存精简关系档案，分析只补充有证据的关键事件和暂定解释。</p>
              <ul><li>不把整份聊天写入关系记忆</li><li>只召回当前对象，不混用其他关系</li><li>随时查看、暂停、撤销或删除</li></ul>
              {!active ? <label className="check-row consent-check"><input type="checkbox" checked={consent} onChange={event => setConsent(event.target.checked)} /><span>我同意在本机保存精简关系档案</span></label> : <div className="inline-notice">记忆已启用。你仍可选择仅用于本次。</div>}
              <p className="footnote">留空项不写入，也不清除已有字段。清除对象档案请使用“忘记当前对象”。</p>
            </div>}
            {error && <p className="inline-error" role="alert">{error}</p>}
          </Modal.Body>
          <Modal.Footer className="profile-footer">
            <Button variant="ghost" size="sm" isDisabled={saving} onPress={async () => { await call('skip_onboarding'); onClose() }}>先处理眼前回复</Button>
            <div className="footer-actions">{step < 3 ? <>
              {step > 0 && <Button variant="secondary" isDisabled={saving} onPress={() => setStep(step - 1)}><ChevronLeft size={15} />上一步</Button>}
              <Button className="violet-button" onPress={() => setStep(step + 1)}>下一步<ChevronRight size={15} /></Button>
            </> : <>
              <Button variant="secondary" isDisabled={saving} onPress={() => save(false)}>仅用于本次</Button>
              <Button className="violet-button" isDisabled={saving || (!active && !consent)} isPending={saving} onPress={() => save(true)}>{active ? '保存到关系记忆' : '启用并保存档案'}</Button>
            </>}</div>
          </Modal.Footer>
        </Modal.Dialog>
      </Modal.Container>
    </Modal.Backdrop>
  </Modal>
}
