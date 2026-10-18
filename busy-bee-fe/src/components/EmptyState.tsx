import type { ReactNode } from 'react'

import { BrandMark } from './BrandMark'

/** 空狀態：品牌六角 + 標題 + 說明 + 選用的引導動作（取代單行灰字）。 */
export function EmptyState({
  title,
  description,
  action,
  className = '',
}: {
  title: string
  description?: string
  action?: ReactNode
  className?: string
}) {
  return (
    <div className={`animate-fade-in flex flex-col items-center gap-2 px-6 py-12 text-center ${className}`.trim()}>
      <span className="mb-1 inline-flex rounded-2xl bg-accent/10 p-3">
        <BrandMark className="size-7" />
      </span>
      <p className="m-0 text-sm font-medium text-fg">{title}</p>
      {description && <p className="m-0 max-w-xs text-xs leading-5 text-muted">{description}</p>}
      {action && <div className="mt-2">{action}</div>}
    </div>
  )
}
