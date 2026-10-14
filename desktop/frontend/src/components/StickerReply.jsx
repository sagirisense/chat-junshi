import { useEffect, useState } from 'react'
import { Button } from '@heroui/react'

export default function StickerReply({ candidate, stickers, allowIntimate, mode, onCopy }) {
  const [replacement, setReplacement] = useState(null)
  const [choosing, setChoosing] = useState(false)
  const [removed, setRemoved] = useState(false)
  useEffect(() => { setReplacement(null); setChoosing(false); setRemoved(false) }, [candidate])
  const selected = removed ? null : replacement || candidate.sticker
  const available = stickers.filter(row => row.enabled && (!row.intimate || allowIntimate))
  if (mode === 'none') return null
  return <div className="sticker-reply">
    {!selected && !removed && candidate.sticker_reason && <p className="status-line">未搭配表情：{candidate.sticker_reason}</p>}
    {selected ? <><img src={selected.preview} alt={selected.name} /><div><b>{selected.name}</b><p>{replacement ? '你手动选择的表情，请确认语气合适。' : candidate.sticker_reason}</p><small>{candidate.text ? ({ image_first: '先发图片，再发文字', text_first: '先发文字，再发图片' }[candidate.send_order] || '先发文字，再发图片') : '单独发送图片'}</small><div className="sticker-actions"><Button size="sm" variant="secondary" onPress={() => onCopy(selected.id)}>复制图片</Button><Button size="sm" variant="ghost" onPress={() => setChoosing(value => !value)}>换一张</Button><Button size="sm" variant="ghost" onPress={() => { setRemoved(true); setChoosing(false) }}>不配表情</Button></div></div></> : <Button size="sm" variant="ghost" onPress={() => setChoosing(value => !value)}>手动搭配表情</Button>}
    {choosing && <div className="sticker-picker">{available.map(row => <button key={row.id} title={`${row.name} · ${row.suitable} · 避免：${row.avoid}`} aria-label={`选择${row.name}`} onClick={() => { setReplacement(row); setRemoved(false); setChoosing(false) }}><img src={row.preview} alt={row.name} /><span>{row.name}</span></button>)}{!available.length && <p>请先在表情库填写标签并启用素材。</p>}</div>}
  </div>
}
