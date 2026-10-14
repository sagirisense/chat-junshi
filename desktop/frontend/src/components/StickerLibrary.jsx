import { useState } from 'react'
import { Button, TextField, Input, Label } from '@heroui/react'

export default function StickerLibrary({ stickers, onImport, onSave, onDelete, onCopy, busy }) {
  const [query, setQuery] = useState('')
  const [edit, setEdit] = useState(null)
  const [saving, setSaving] = useState(false)
  const rows = stickers.filter(row => `${row.name} ${row.tags} ${row.suitable}`.toLowerCase().includes(query.toLowerCase()))
  async function save() {
    setSaving(true)
    try { if (await onSave(edit)) setEdit(null) } finally { setSaving(false) }
  }
  return <section className="glass-panel sticker-library">
    <div className="panel-heading"><h2>我的表情库 · {stickers.length}</h2><Button variant="secondary" isDisabled={busy || saving} onPress={onImport}>导入图片</Button></div>
    <p className="status-line">支持 PNG、JPG、静态 WebP。填写标签并启用后，AI 才会推荐；素材保存在本机。</p>
    <TextField value={query} onChange={setQuery}><Label>搜索表情</Label><Input placeholder="名称、标签或场景，例如：晚安" /></TextField>
    {edit && <div className="sticker-editor">
      <img src={edit.preview} alt={edit.name} />
      <div className="sticker-fields">
        {[['name', '名称', 80], ['tags', '标签（逗号分隔）', 200], ['suitable', '适用场景', 300], ['avoid', '避免场景', 300]].map(([key, label, max]) => <TextField key={key} value={edit[key] || ''} onChange={value => setEdit(previous => ({ ...previous, [key]: value }))}><Label>{label}</Label><Input maxLength={max} /></TextField>)}
        <label className="check-row"><input type="checkbox" checked={edit.intimate} onChange={event => setEdit({ ...edit, intimate: event.target.checked })} />仅适合亲密关系</label>
        <label className="check-row"><input type="checkbox" checked={edit.enabled} onChange={event => setEdit({ ...edit, enabled: event.target.checked })} />启用 AI 推荐</label>
        <p className="footnote">{edit.license}{edit.source && <> · <a href={edit.source} target="_blank" rel="noreferrer">素材来源</a></>}</p>
        <div className="sticker-actions"><Button isDisabled={busy || saving} isPending={saving} onPress={save}>保存</Button><Button variant="ghost" isDisabled={saving} onPress={() => setEdit(null)}>取消</Button></div>
      </div>
    </div>}
    <div className="sticker-grid">{rows.map(row => <article className="sticker-tile" key={row.id}>
      <img src={row.preview} alt={row.name} loading="lazy" /><b>{row.name}</b><span>{row.tags || '待填写标签'}</span><small>{row.enabled ? '可推荐' : '未启用'}{row.intimate ? ' · 亲密关系' : ''}</small>
      <div className="sticker-actions"><Button size="sm" variant="secondary" onPress={() => onCopy(row.id)}>复制图片</Button><Button size="sm" variant="ghost" isDisabled={busy || saving} onPress={() => setEdit({ ...row })}>编辑</Button><Button size="sm" variant="ghost" isDisabled={busy || saving} onPress={() => { if (edit?.id === row.id) setEdit(null); onDelete(row) }}>删除</Button></div>
    </article>)}</div>
    {!rows.length && <div className="empty-state"><h3>{stickers.length ? '没有匹配的表情' : '导入你喜欢的表情'}</h3><p>给图片标上“收到”“晚安”等标签，方便搭配回复。</p></div>}
  </section>
}
