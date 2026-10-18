import { CalendarClock, Check, CircleAlert, Clock, HardDriveUpload, LoaderCircle, Radio } from 'lucide-react'

import type { DisplayStatus } from '../utils/meetingStatus'

type Status = DisplayStatus

const config: Record<
  Status,
  { label: string; Icon: typeof Check; cls: string; spin?: boolean }
> = {
  scheduled: {
    label: '已排程',
    Icon: CalendarClock,
    cls: 'text-violet-600 bg-violet-600/10 dark:text-violet-400 dark:bg-violet-400/10',
  },
  live: {
    label: '即時會議',
    Icon: Radio,
    cls: 'text-rose-600 bg-rose-600/10 dark:text-rose-400 dark:bg-rose-400/10',
  },
  awaitingUpload: {
    label: '待上傳錄音',
    Icon: HardDriveUpload,
    cls: 'text-warning bg-warning/10',
  },
  pending: {
    label: '等待處理',
    Icon: Clock,
    cls: 'text-warning bg-warning/10',
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
    cls: 'text-success bg-success/10',
  },
  failed: {
    label: '失敗',
    Icon: CircleAlert,
    cls: 'text-danger bg-danger/10',
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
