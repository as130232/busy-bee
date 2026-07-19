import { Link } from 'react-router-dom'
import { Check } from 'lucide-react'

import type { ActionItem, PendingActionItem } from '../services/api/client'

type Item = ActionItem | PendingActionItem

function meetingTitleOf(item: Item): string | undefined {
  return 'meetingTitle' in item ? item.meetingTitle : undefined
}

/** 行動項清單。onToggle 由父層做樂觀更新與 API 呼叫。 */
export function ActionItemList({
  items,
  onToggle,
  showMeeting = false,
}: {
  items: Item[]
  onToggle: (id: string, done: boolean) => void
  showMeeting?: boolean
}) {
  if (items.length === 0) {
    return <p className="py-6 text-center text-sm text-muted">目前沒有行動項。</p>
  }

  return (
    <ul className="flex flex-col divide-y divide-border">
      {items.map((item) => {
        const meta = [item.assignee, item.dueText].filter(Boolean).join(' · ')
        const title = showMeeting ? meetingTitleOf(item) : undefined
        return (
          <li key={item.id} className="flex items-start gap-3 py-3">
            <button
              type="button"
              role="checkbox"
              aria-checked={item.done}
              aria-label={item.done ? '標記為未完成' : '標記為完成'}
              onClick={() => onToggle(item.id, !item.done)}
              className={`mt-0.5 flex size-6 shrink-0 items-center justify-center rounded-full border transition ${
                item.done
                  ? 'border-accent bg-accent text-white dark:text-zinc-900'
                  : 'border-border hover:border-accent'
              }`}
            >
              {item.done && <Check className="size-4" strokeWidth={3} />}
            </button>
            <div className="min-w-0 flex-1">
              <p
                className={`m-0 text-sm ${
                  item.done ? 'text-muted line-through' : 'text-fg'
                }`}
              >
                {item.description}
              </p>
              {meta && <p className="m-0 mt-0.5 font-mono text-xs text-muted">{meta}</p>}
              {title && (
                <Link
                  to={`/meetings/${item.meetingId}`}
                  className="mt-0.5 inline-block text-xs text-accent hover:underline"
                >
                  {title}
                </Link>
              )}
            </div>
          </li>
        )
      })}
    </ul>
  )
}
