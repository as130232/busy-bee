import { useState } from 'react'
import { Search } from 'lucide-react'

import { MeetingList } from '../components/MeetingList'
import { useMeetings } from '../hooks/useMeetings'

/** 紀錄分頁：搜尋 + 歷史列表（排除排程中的未來紀錄）。 */
export function MeetingsPage() {
  const [search, setSearch] = useState('')
  const { meetings, error, reload } = useMeetings(search)
  const list = meetings.filter((m) => m.status !== 'scheduled')

  return (
    <>
      <div className="relative">
        <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted" />
        <input
          className="input pl-9"
          type="search"
          placeholder="搜尋標題或逐字稿…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
      </div>

      {error && (
        <div className="flex items-center justify-between gap-3 rounded-xl border border-red-500/30 bg-red-500/5 px-4 py-3 text-sm text-red-500">
          {error}
          <button type="button" className="btn btn-secondary h-9" onClick={reload}>
            重新載入
          </button>
        </div>
      )}

      {!error && (
        <MeetingList
          meetings={list}
          emptyText={search ? '沒有符合的紀錄。' : '尚無紀錄，錄下第一段吧。'}
        />
      )}
    </>
  )
}
