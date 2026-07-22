import { useState } from 'react'
import { Link } from 'react-router-dom'
import { CalendarPlus, Check, Pencil } from 'lucide-react'

import type { ActionItem, PendingActionItem } from '../services/api/client'
import { addToCalendar } from '../services/ics'
import { speakerColor } from './speakerColor'

type Item = ActionItem | PendingActionItem

function meetingTitleOf(item: Item): string | undefined {
  return 'meetingTitle' in item ? item.meetingTitle : undefined
}

/**
 * 待辦清單。onToggle 勾選、onEdit（可選）就地修改內容，皆由父層做樂觀更新與 API 呼叫。
 * speakerNames/speakerOrder 供指派人以講者顯示名與配色呈現（跟著改名連動）。
 */
export function ActionItemList({
  items,
  onToggle,
  onEdit,
  showMeeting = false,
  speakerNames = {},
  speakerOrder = [],
}: {
  items: Item[]
  onToggle: (id: string, done: boolean) => void
  onEdit?: (id: string, description: string) => Promise<void> | void
  showMeeting?: boolean
  speakerNames?: Record<string, string>
  speakerOrder?: string[]
}) {
  if (items.length === 0) {
    return <p className="py-6 text-center text-sm text-muted">目前沒有待辦。</p>
  }

  return (
    <ul className="flex flex-col divide-y divide-border">
      {items.map((item, i) => (
        <ActionItemRow
          key={item.id}
          item={item}
          index={i}
          onToggle={onToggle}
          onEdit={onEdit}
          showMeeting={showMeeting}
          speakerNames={speakerNames}
          speakerOrder={speakerOrder}
        />
      ))}
    </ul>
  )
}

// ActionItemRow 單筆待辦：勾選 + 內容（可就地編輯）+ 指派人晶片 + 加入行事曆。
function ActionItemRow({
  item,
  index,
  onToggle,
  onEdit,
  showMeeting,
  speakerNames,
  speakerOrder,
}: {
  item: Item
  index: number
  onToggle: (id: string, done: boolean) => void
  onEdit?: (id: string, description: string) => Promise<void> | void
  showMeeting: boolean
  speakerNames: Record<string, string>
  speakerOrder: string[]
}) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(item.description)
  const [busy, setBusy] = useState(false)

  // 指派人以講者顯示名呈現（改名後連動）；為講者代號則沿用其配色，否則中性色。
  const assigneeName = item.assignee ? (speakerNames[item.assignee]?.trim() || item.assignee) : ''
  const assigneeChip = speakerOrder.includes(item.assignee)
    ? speakerColor(item.assignee, speakerOrder)
    : 'bg-surface-hover text-muted'
  const title = showMeeting ? meetingTitleOf(item) : undefined

  const cancel = () => {
    setDraft(item.description)
    setEditing(false)
  }

  const save = async () => {
    const next = draft.trim()
    if (!next || next === item.description || !onEdit) {
      cancel()
      return
    }
    setBusy(true)
    try {
      await onEdit(item.id, next)
      setEditing(false)
    } catch {
      setDraft(item.description) // 失敗還原
    } finally {
      setBusy(false)
    }
  }

  return (
    <li
      className="animate-fade-in-up flex items-start gap-3 py-3"
      style={{ animationDelay: `${Math.min(index, 8) * 40}ms` }}
    >
      <button
        type="button"
        role="checkbox"
        aria-checked={item.done}
        aria-label={item.done ? '標記為未完成' : '標記為完成'}
        onClick={() => onToggle(item.id, !item.done)}
        className={`mt-0.5 flex size-5 shrink-0 items-center justify-center rounded-full border transition ${
          item.done ? 'border-accent bg-accent text-white dark:text-zinc-900' : 'border-border hover:border-accent'
        }`}
      >
        {item.done && <Check className="animate-scale-in size-3" strokeWidth={3} />}
      </button>

      {editing ? (
        <div className="flex min-w-0 flex-1 flex-col gap-2">
          <input
            type="text"
            className="input text-sm"
            value={draft}
            disabled={busy}
            autoFocus
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Escape') cancel()
              if (e.key === 'Enter') void save()
            }}
          />
          <div className="flex justify-end gap-2">
            <button type="button" className="btn btn-secondary h-8" disabled={busy} onClick={cancel}>
              取消
            </button>
            <button type="button" className="btn btn-primary h-8" disabled={busy} onClick={() => void save()}>
              {busy ? '儲存中…' : '儲存'}
            </button>
          </div>
        </div>
      ) : (
        <div className="flex min-w-0 flex-1 items-start gap-2">
          <div className="min-w-0 flex-1">
            <div className="flex items-start gap-1.5">
              <p className={`m-0 text-sm leading-6 ${item.done ? 'text-muted line-through' : 'text-fg'}`}>
                {item.description}
              </p>
              {onEdit && !item.done && (
                <button
                  type="button"
                  className="mt-0.5 shrink-0 text-muted transition-colors hover:text-accent"
                  aria-label="修改待辦"
                  onClick={() => {
                    setDraft(item.description)
                    setEditing(true)
                  }}
                >
                  <Pencil className="size-3.5" />
                </button>
              )}
            </div>
            {(assigneeName || item.dueText || title) && (
              <div className="mt-1 flex flex-wrap items-center gap-1.5">
                {assigneeName && (
                  <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${assigneeChip}`}>
                    {assigneeName}
                  </span>
                )}
                {item.dueText && <span className="text-xs text-muted">{item.dueText}</span>}
                {title && (
                  <Link to={`/meetings/${item.meetingId}`} className="text-xs text-accent hover:underline">
                    {title}
                  </Link>
                )}
              </div>
            )}
          </div>
          {item.dueAt && !item.done && (
            <button
              type="button"
              aria-label="加入行事曆"
              onClick={() => void addToCalendar(item)}
              className="flex size-8 shrink-0 items-center justify-center rounded-md text-muted transition hover:bg-surface-hover hover:text-accent"
            >
              <CalendarPlus className="size-4" />
            </button>
          )}
        </div>
      )}
    </li>
  )
}
