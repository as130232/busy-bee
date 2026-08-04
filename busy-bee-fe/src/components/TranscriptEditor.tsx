import { useState } from 'react'
import { Pencil, Play } from 'lucide-react'

import {
  editMeetingSegment,
  updateMeetingSpeakers,
  type MeetingDetail,
  type TranscriptSegment,
} from '../services/api/client'
import { useApiCall } from '../hooks/useApiCall'
import { formatClock } from '../utils/format'
import { Sheet } from './Sheet'
import { speakerColor } from './speakerColor'

/** 分講者逐字稿：講者晶片可點擊改名（PATCH speakers），逐段顯示，可就地修正文字。 */
export function TranscriptEditor({
  meeting,
  onUpdated,
  onSeek,
}: {
  meeting: MeetingDetail
  onUpdated: (m: MeetingDetail) => void
  onSeek?: (seconds: number) => void
}) {
  const [editing, setEditing] = useState<string | null>(null)

  const names = meeting.speakerNames ?? {}
  const displayName = (code: string) => names[code]?.trim() || code
  // 依首次出現順序取得講者代號
  const order: string[] = []
  for (const s of meeting.transcriptSegments) {
    if (!order.includes(s.speaker)) order.push(s.speaker)
  }

  return (
    <div className="flex flex-col gap-4">
      {/* 只偵測到一位講者時提示：可能是單人錄音，或多人但音色相近/錄音條件未能分離 */}
      {order.length <= 1 && meeting.transcriptSegments.length > 0 && (
        <p className="m-0 rounded-lg border border-border bg-surface px-3 py-2 text-xs leading-5 text-muted">
          只偵測到一位講者。若實際為多人，可能因音色相近或錄音條件，未能自動分離。
        </p>
      )}
      {/* 講者圖例：點晶片可改名 */}
      <div className="flex flex-wrap gap-2 border-b border-border pb-3">
        {order.map((code) => (
          <button
            key={code}
            type="button"
            className={`inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-medium ${speakerColor(code, order)}`}
            onClick={() => setEditing(code)}
          >
            {displayName(code)}
            <Pencil className="size-3 opacity-60" />
          </button>
        ))}
      </div>

      {/* 逐段內容：標頭（時間 · 講者 · ▶）整列可點跳播、✏️ 可修正文字 + 全寬文字 */}
      <div className="flex flex-col gap-4">
        {meeting.transcriptSegments.map((s, i) => (
          <SegmentRow
            key={i}
            meetingId={meeting.id}
            index={i}
            seg={s}
            speakerName={displayName(s.speaker)}
            colorClass={speakerColor(s.speaker, order)}
            onSeek={onSeek}
            onUpdated={onUpdated}
          />
        ))}
      </div>

      {editing && (
        <SpeakerRenameSheet
          meetingId={meeting.id}
          code={editing}
          current={displayName(editing)}
          existingNames={names}
          onClose={() => setEditing(null)}
          onUpdated={(m) => {
            onUpdated(m)
            setEditing(null)
          }}
        />
      )}
    </div>
  )
}

/** 單段逐字稿：標頭可跳播、✏️ 可就地修正文字（PATCH /meetings/:id/transcript）。 */
function SegmentRow({
  meetingId,
  index,
  seg,
  speakerName,
  colorClass,
  onSeek,
  onUpdated,
}: {
  meetingId: string
  index: number
  seg: TranscriptSegment
  speakerName: string
  colorClass: string
  onSeek?: (seconds: number) => void
  onUpdated: (m: MeetingDetail) => void
}) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(seg.text)
  const [busy, setBusy] = useState(false)
  const call = useApiCall()

  const save = async () => {
    const next = draft.trim()
    if (!next || next === seg.text) {
      setEditing(false)
      setDraft(seg.text)
      return
    }
    setBusy(true)
    try {
      const { meeting } = await call(editMeetingSegment, meetingId, index, next)
      onUpdated(meeting)
      setEditing(false)
    } catch {
      setDraft(seg.text) // 失敗還原
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex flex-col gap-1">
      <div className="flex items-center gap-1">
        <button
          type="button"
          className="group -mx-1 flex flex-1 items-center gap-2 rounded-md px-1 py-0.5 transition-colors hover:bg-surface-hover"
          onClick={() => onSeek?.(seg.startMs / 1000)}
          aria-label={`從 ${formatClock(seg.startMs / 1000)} 播放`}
        >
          <span className="font-mono text-[11px] tabular-nums text-muted">{formatClock(seg.startMs / 1000)}</span>
          <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${colorClass}`}>{speakerName}</span>
          <Play className="ml-auto size-3.5 shrink-0 text-muted transition-colors group-hover:text-accent" />
        </button>
      </div>

      {editing ? (
        <div className="flex flex-col gap-2">
          <textarea
            className="input min-h-[4.5rem] text-sm leading-7"
            value={draft}
            disabled={busy}
            autoFocus
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Escape') {
                setDraft(seg.text)
                setEditing(false)
              }
            }}
          />
          <div className="flex justify-end gap-2">
            <button
              type="button"
              className="btn btn-secondary h-9"
              disabled={busy}
              onClick={() => {
                setDraft(seg.text)
                setEditing(false)
              }}
            >
              取消
            </button>
            <button type="button" className="btn btn-primary h-9" disabled={busy} onClick={() => void save()}>
              {busy ? '儲存中…' : '儲存'}
            </button>
          </div>
        </div>
      ) : (
        <p className="m-0 text-sm leading-7">
          {seg.text}
          <button
            type="button"
            className="ml-1 inline-flex translate-y-0.5 text-muted transition-colors hover:text-accent"
            aria-label="修正文字"
            onClick={() => {
              setDraft(seg.text)
              setEditing(true)
            }}
          >
            <Pencil className="size-3.5" />
          </button>
        </p>
      )}
    </div>
  )
}

/** 講者改名底部彈窗（PATCH /meetings/:id/speakers）。 */
function SpeakerRenameSheet({
  meetingId,
  code,
  current,
  existingNames,
  onClose,
  onUpdated,
}: {
  meetingId: string
  code: string
  current: string
  existingNames: Record<string, string>
  onClose: () => void
  onUpdated: (m: MeetingDetail) => void
}) {
  const [draft, setDraft] = useState(current === code ? '' : current)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  const call = useApiCall()

  const save = async () => {
    const next = draft.trim()
    setBusy(true)
    setErr(null)
    try {
      const merged = { ...existingNames }
      if (next) merged[code] = next
      else delete merged[code]
      const { meeting } = await call(updateMeetingSpeakers, meetingId, merged)
      onUpdated(meeting)
    } catch (e) {
      setErr(e instanceof Error ? e.message : '更新失敗')
      setBusy(false)
    }
  }

  return (
    <Sheet onClose={onClose}>
      <p className="m-0 text-sm font-medium">重新命名講者 {code}</p>
      <input
        className="input h-10"
        value={draft}
        placeholder={`例如 Ben（留空還原為 ${code}）`}
        disabled={busy}
        autoFocus
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter') void save()
          if (e.key === 'Escape') onClose()
        }}
      />
      {err && <p className="m-0 text-xs text-red-500">{err}</p>}
      <div className="flex gap-2">
        <button type="button" className="btn btn-secondary flex-1" onClick={onClose} disabled={busy}>
          取消
        </button>
        <button type="button" className="btn btn-primary flex-1" onClick={() => void save()} disabled={busy}>
          {busy ? '儲存中…' : '儲存'}
        </button>
      </div>
    </Sheet>
  )
}
