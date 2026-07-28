import { useState } from 'react'
import { Tag, X } from 'lucide-react'

import { updateMeetingTags, type MeetingDetail } from '../services/api/client'
import { getIdToken } from '../services/token'

/** 會議標籤就地編輯：晶片可移除、輸入框加標籤（Enter/失焦送出）；即時 PATCH。 */
export function TagEditor({
  meeting,
  onUpdated,
}: {
  meeting: MeetingDetail
  onUpdated: (m: MeetingDetail) => void
}) {
  const tags = meeting.tags ?? []
  const [input, setInput] = useState('')
  const [busy, setBusy] = useState(false)

  const save = async (next: string[]) => {
    setBusy(true)
    try {
      const { meeting: m } = await updateMeetingTags(await getIdToken(), meeting.id, next)
      onUpdated(m)
    } catch {
      // 失敗保留原狀，不中斷檢視
    } finally {
      setBusy(false)
    }
  }

  const add = () => {
    const t = input.trim()
    setInput('')
    if (!t || tags.includes(t)) return
    void save([...tags, t])
  }

  return (
    <div className="flex flex-wrap items-center gap-1.5">
      <Tag className="size-3.5 shrink-0 text-muted" />
      {tags.map((t) => (
        <span
          key={t}
          className="inline-flex items-center gap-1 rounded-full border border-border bg-surface px-2 py-0.5 text-xs"
        >
          #{t}
          <button
            type="button"
            onClick={() => void save(tags.filter((x) => x !== t))}
            disabled={busy}
            aria-label={`移除標籤 ${t}`}
          >
            <X className="size-3 text-muted hover:text-fg" />
          </button>
        </span>
      ))}
      <input
        className="w-20 border-b border-border bg-transparent text-xs outline-none focus:border-accent disabled:opacity-50"
        placeholder="加標籤"
        value={input}
        maxLength={30}
        disabled={busy}
        onChange={(e) => setInput(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter') {
            e.preventDefault()
            add()
          }
        }}
        onBlur={add}
      />
    </div>
  )
}
