import { useMemo, useState } from 'react'
import { Search } from 'lucide-react'

import { MeetingList } from '../components/MeetingList'
import { useMeetings } from '../hooks/useMeetings'
import { scenarioLabels, type Meeting, type Scenario } from '../services/api/client'

type ScenarioFilter = Scenario | 'all'
type SourceFilter = 'all' | 'recorded' | 'imported'

/** 篩選晶片：選中時 accent 高亮。 */
function Chip({
  active,
  onClick,
  children,
}: {
  active: boolean
  onClick: () => void
  children: React.ReactNode
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`shrink-0 rounded-full border px-3 py-1 text-xs font-medium transition ${
        active
          ? 'border-accent bg-accent/10 text-accent'
          : 'border-border text-muted hover:text-fg'
      }`}
    >
      {children}
    </button>
  )
}

/** 紀錄分頁：搜尋 + 情境/來源/標籤篩選 + 歷史列表（排除排程中的未來紀錄）。 */
export function MeetingsPage() {
  const [search, setSearch] = useState('')
  const { meetings, error, reload } = useMeetings(search)
  const [scenario, setScenario] = useState<ScenarioFilter>('all')
  const [source, setSource] = useState<SourceFilter>('all')
  const [tag, setTag] = useState<string | null>(null)

  const list = useMemo(() => meetings.filter((m) => m.status !== 'scheduled'), [meetings])

  // 現有紀錄用到的標籤（去重，供標籤篩選列）。
  const allTags = useMemo(() => {
    const set = new Set<string>()
    for (const m of list) for (const t of m.tags ?? []) set.add(t)
    return [...set]
  }, [list])

  const filtered = useMemo(
    () =>
      list.filter((m: Meeting) => {
        if (scenario !== 'all' && m.scenario !== scenario) return false
        if (source === 'recorded' && m.imported) return false
        if (source === 'imported' && !m.imported) return false
        if (tag && !(m.tags ?? []).includes(tag)) return false
        return true
      }),
    [list, scenario, source, tag],
  )

  const scenarioKeys = Object.keys(scenarioLabels) as Scenario[]

  return (
    <div className="flex flex-col gap-3">
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

      {/* 情境 + 來源篩選 */}
      <div className="flex gap-2 overflow-x-auto pb-1">
        <Chip active={scenario === 'all' && source === 'all'} onClick={() => { setScenario('all'); setSource('all') }}>
          全部
        </Chip>
        {scenarioKeys.map((s) => (
          <Chip key={s} active={scenario === s} onClick={() => setScenario(scenario === s ? 'all' : s)}>
            {scenarioLabels[s]}
          </Chip>
        ))}
        <span className="w-px shrink-0 self-stretch bg-border" />
        <Chip active={source === 'recorded'} onClick={() => setSource(source === 'recorded' ? 'all' : 'recorded')}>
          錄音
        </Chip>
        <Chip active={source === 'imported'} onClick={() => setSource(source === 'imported' ? 'all' : 'imported')}>
          匯入
        </Chip>
      </div>

      {/* 標籤篩選（有標籤才顯示） */}
      {allTags.length > 0 && (
        <div className="flex gap-2 overflow-x-auto pb-1">
          {allTags.map((t) => (
            <Chip key={t} active={tag === t} onClick={() => setTag(tag === t ? null : t)}>
              #{t}
            </Chip>
          ))}
        </div>
      )}

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
          meetings={filtered}
          emptyText={search || scenario !== 'all' || source !== 'all' || tag ? '沒有符合的紀錄。' : '尚無紀錄，錄下第一段吧。'}
        />
      )}
    </div>
  )
}
