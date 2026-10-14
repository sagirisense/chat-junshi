import { useState } from 'react'
import { Button, TextField, Input, Label, Modal } from '@heroui/react'

export default function StickerLibrary({ stickers, onImport, onSave, onDelete, onCopy, onAnalyze, onTransfer, busy }) {
  const [query, setQuery] = useState('')
  const [edit, setEdit] = useState(null)
  const [saving, setSaving] = useState(false)
  const [analyzing, setAnalyzing] = useState(false)
  const [transferring, setTransferring] = useState(false)
  const [analysisNotice, setAnalysisNotice] = useState('')
  const [analysisError, setAnalysisError] = useState('')
  const [intimacyAnalysis, setIntimacyAnalysis] = useState(null)
  const working = saving || analyzing || transferring
  const rows = stickers.filter(row => `${row.name} ${row.tags} ${row.suitable}`.toLowerCase().includes(query.toLowerCase()))
  async function save() {
    setSaving(true); setAnalysisError('')
    try {
      if (await onSave(edit)) setEdit(null)
      else setAnalysisError('保存失败，草稿已保留；请检查名称、标签和推荐设置后重试。')
    } finally { setSaving(false) }
  }
  async function analyze() {
    setAnalyzing(true); setAnalysisNotice(''); setAnalysisError('')
    try {
      const result = await onAnalyze(edit.id)
      if (result) {
        const { intimate_reason, ...metadata } = result
        setEdit(previous => ({ ...previous, ...metadata }))
        setIntimacyAnalysis({ intimate: result.intimate, reason: intimate_reason })
        setAnalysisNotice('已填入 AI 建议，包括亲密使用限制，请核对后保存。')
      }
    } catch (e) { setAnalysisError(String(e.message || e)) }
    finally { setAnalyzing(false) }
  }
  async function transfer(action) {
    setTransferring(true)
    try { await onTransfer(action) } finally { setTransferring(false) }
  }
  return <section className="glass-panel sticker-library">
    <div className="panel-heading"><h2>我的表情库 · {stickers.length}</h2><div className="sticker-actions"><Button variant="secondary" isDisabled={busy || working} onPress={onImport}>导入图片</Button><Button variant="secondary" isDisabled={busy || working} onPress={() => transfer('import')}>导入表情库</Button><Button variant="ghost" isDisabled={busy || working || !stickers.length} onPress={() => transfer('export')}>导出表情库</Button></div></div>
    <p className="status-line">支持 PNG、JPG、静态 WebP。填写标签并启用后，AI 才会推荐；素材保存在本机。</p>
    <TextField value={query} onChange={setQuery}><Label>搜索表情</Label><Input placeholder="名称、标签或场景，例如：晚安" /></TextField>
    <Modal isOpen={Boolean(edit)} onOpenChange={open => { if (!open && !working) setEdit(null) }}>
      <Modal.Backdrop variant="blur" className="dialog-backdrop">
        <Modal.Container placement="center" scroll="inside">
          <Modal.Dialog className="small-dialog sticker-edit-dialog" aria-label="编辑表情">
            <Modal.CloseTrigger isDisabled={working} />
            <Modal.Header><Modal.Heading>编辑表情</Modal.Heading></Modal.Header>
            <Modal.Body>
    {edit && <div className="sticker-editor">
      <img src={edit.preview} alt={edit.name} />
      <div className="sticker-fields">
        <Button variant="secondary" isDisabled={busy || working} isPending={analyzing} onPress={analyze}>{analyzing ? '正在分析图片…' : '分析图片标签'}</Button>
        <p className="footnote">将当前图片发送到所选模型，需要支持图片理解。AI 建议会替换此处草稿，核对后再保存。</p>
        {analysisNotice && <p className="status-line" role="status">{analysisNotice}</p>}
        {analysisError && <p className="inline-error" role="alert">{analysisError}</p>}
        {[['name', '名称', 80], ['tags', '标签（逗号分隔）', 200], ['suitable', '适用场景', 300], ['avoid', '避免场景', 300]].map(([key, label, max]) => <TextField key={key} isDisabled={working} value={edit[key] || ''} onChange={value => setEdit(previous => ({ ...previous, [key]: value }))}><Label>{label}</Label><Input maxLength={max} /></TextField>)}
        <label className="check-row"><input type="checkbox" disabled={working} checked={edit.intimate} onChange={event => setEdit({ ...edit, intimate: event.target.checked })} />仅适合亲密关系</label>
        {intimacyAnalysis && <div className="footnote" role="status"><b>AI 判断：{intimacyAnalysis.intimate ? '建议限制为亲密关系使用' : '无需限制为亲密关系使用'}</b><p>{intimacyAnalysis.reason}</p>{edit.intimate !== intimacyAnalysis.intimate && <p>你已手动修改此限制，保存时以勾选状态为准。</p>}</div>}
        <p className="footnote">此项限制表情的推荐范围，不代表已经确认双方是亲密关系。</p>
        <label className="check-row"><input type="checkbox" disabled={working} checked={edit.enabled} onChange={event => setEdit({ ...edit, enabled: event.target.checked })} />启用 AI 推荐</label>
        <p className="footnote">{edit.license}{edit.source && <> · <a href={edit.source} target="_blank" rel="noreferrer">素材来源</a></>}</p>
      </div>
    </div>}
            </Modal.Body>
            <Modal.Footer><Button variant="ghost" isDisabled={working} onPress={() => setEdit(null)}>取消</Button><Button isDisabled={busy || working} isPending={saving} onPress={save}>保存</Button></Modal.Footer>
          </Modal.Dialog>
        </Modal.Container>
      </Modal.Backdrop>
    </Modal>
    <div className="sticker-grid">{rows.map(row => <article className="sticker-tile" key={row.id}>
      <img src={row.preview} alt={row.name} loading="lazy" /><b>{row.name}</b><span>{row.tags || '待填写标签'}</span><small>{row.enabled ? '可推荐' : '未启用'}{row.intimate ? ' · 亲密关系' : ''}</small>
      <div className="sticker-actions"><Button size="sm" variant="secondary" onPress={() => onCopy(row.id)}>复制图片</Button><Button size="sm" variant="ghost" isDisabled={busy || working} onPress={() => { setEdit({ ...row }); setAnalysisNotice(''); setAnalysisError(''); setIntimacyAnalysis(null) }}>编辑</Button><Button size="sm" variant="ghost" isDisabled={busy || working} onPress={() => { if (edit?.id === row.id) setEdit(null); onDelete(row) }}>删除</Button></div>
    </article>)}</div>
    {!rows.length && <div className="empty-state"><h3>{stickers.length ? '没有匹配的表情' : '导入你喜欢的表情'}</h3><p>给图片标上“收到”“晚安”等标签，方便搭配回复。</p></div>}
  </section>
}
