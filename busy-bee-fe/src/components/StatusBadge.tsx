import { CalendarClock, Check, CircleAlert, Clock, LoaderCircle } from 'lucide-react'

import type { Meeting } from '../services/api/client'

type Status = Meeting['status']

const config: Record<
  Status,
  { label: string; Icon: typeof Check; cls: string; spin?: boolean }
> = {
  scheduled: {
    label: '已排程',
    Icon: CalendarClock,
    cls: 'text-violet-600 bg-violet-600/10 dark:text-violet-400 dark:bg-violet-400/10',
  },
  pending: {
    label: '等待處理',
    Icon: Clock,
    cls: 'text-amber-600 bg-amber-600/10 dark:text-amber-400 dark:bg-amber-400/10',
  },
  transcribing: {
    label: '轉錄中',
    Icon: LoaderCircle,
    spin: true,
    cls: 'text-blue-600 bg-blue-600/10 dark:text-blue-400 dark:bg-blue-400/10',
  },
  analyzing: {
    label: '生成文件中',
    Icon: LoaderCircle,
    spin: true,
    cls: 'text-blue-600 bg-blue-600/10 dark:text-blue-400 dark:bg-blue-400/10',
  },
  completed: {
    label: '已完成',
    Icon: Check,
    cls: 'text-emerald-600 bg-emerald-600/10 dark:text-emerald-400 dark:bg-emerald-400/10',
  },
  failed: {
    label: '失敗',
    Icon: CircleAlert,
    cls: 'text-red-600 bg-red-600/10 dark:text-red-400 dark:bg-red-400/10',
  },
}

export function StatusBadge({ status }: { status: Status }) {
  const { label, Icon, cls, spin } = config[status]
  return (
    <span
      className={`inline-flex shrink-0 items-center gap-1 rounded-full px-2.5 py-1 text-xs font-medium ${cls}`}
    >
      <Icon className={`size-3.5${spin ? ' animate-spin' : ''}`} />
      {label}
    </span>
  )
}
