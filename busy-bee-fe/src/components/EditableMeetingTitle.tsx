import { type CSSProperties, useEffect, useRef, useState } from 'react'
import { Pencil } from 'lucide-react'

import { renameMeeting } from '../services/api/client'
import { useApiCall } from '../hooks/useApiCall'

/** 標題 + 鉛筆編輯（PATCH rename）。 */
export function EditableMeetingTitle({
  meetingId,
  title,
  onRenamed,
}: {
  meetingId: string
  title: string
  onRenamed: (title: string) => void
}) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(title)
  const [busy, setBusy] = useState(false)
  const call = useApiCall()

  const save = async () => {
    const next = draft.trim()
    if (!next || next === title) {
      setEditing(false)
      setDraft(title)
      return
    }
    setBusy(true)
    try {
      const { meeting } = await call(renameMeeting, meetingId, next)
      onRenamed(meeting.title)
      setEditing(false)
    } catch {
      setDraft(title) // 失敗還原
      setEditing(false)
    } finally {
      setBusy(false)
    }
  }

  if (editing) {
    return (
      <input
        className="input h-9 min-w-0 flex-1"
        value={draft}
        disabled={busy}
        autoFocus
        onChange={(e) => setDraft(e.target.value)}
        onBlur={() => void save()}
        onKeyDown={(e) => {
          if (e.key === 'Enter') e.currentTarget.blur()
          if (e.key === 'Escape') {
            setDraft(title)
            setEditing(false)
          }
        }}
      />
    )
  }
  return (
    <div className="flex min-w-0 flex-1 items-center gap-1.5">
      <MarqueeTitle title={title} />
      <button
        type="button"
        className="btn btn-ghost size-8 shrink-0 px-0 text-muted"
        aria-label="重新命名"
        onClick={() => {
          setDraft(title)
          setEditing(true)
        }}
      >
        <Pencil className="size-3.5" />
      </button>
    </div>
  )
}

/** 標題過長時以跑馬燈（音樂播放器風格）左右來回捲動；未溢出則靜止。 */
function MarqueeTitle({ title }: { title: string }) {
  const containerRef = useRef<HTMLDivElement>(null)
  const textRef = useRef<HTMLHeadingElement>(null)
  const [shift, setShift] = useState(0)

  useEffect(() => {
    const c = containerRef.current
    const t = textRef.current
    if (!c || !t) return
    const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    const overflow = t.scrollWidth - c.clientWidth
    setShift(!reduce && overflow > 4 ? overflow : 0)
  }, [title])

  const style: CSSProperties | undefined =
    shift > 0
      ? ({
        animation: `marquee ${Math.max(6, shift / 24)}s ease-in-out infinite alternate`,
        '--marquee-shift': `-${shift}px`,
      } as CSSProperties)
      : undefined

  return (
    <div ref={containerRef} className="min-w-0 flex-1 overflow-hidden">
      <h1 ref={textRef} className="m-0 inline-block whitespace-nowrap text-lg font-semibold" style={style}>
        {title}
      </h1>
    </div>
  )
}
