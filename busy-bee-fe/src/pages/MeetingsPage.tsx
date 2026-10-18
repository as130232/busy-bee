import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Mic, Search, SlidersHorizontal, X } from 'lucide-react'

import { MeetingList } from '../components/MeetingList'
import { scenarioThemes } from '../components/scenarioTheme'
import { Sheet } from '../components/Sheet'
import { useMeetings } from '../hooks/useMeetings'
import { scenarioLabels, type Meeting, type Scenario } from '../services/api/client'

type ScenarioFilter = Scenario | 'all'
type SourceFilter = 'all' | 'recorded' | 'imported'

/** 篩選晶片：選中時 accent 高亮；dot 帶情境色點（兼作列表色點的圖例）。 */
function Chip({
  active,
  onClick,
  dot,
  children,
}: {
  active: boolean
  onClick: () => void
  dot?: string
  children: React.ReactNode
}) {
  return (
    <button
      type="button"
      aria-pressed={active}
      onClick={onClick}
      className={`inline-flex h-8 shrink-0 items-center gap-1.5 rounded-full border px-3 text-xs font-medium transition outline-none focus-visible:ring-2 focus-visible:ring-accent/50 ${
        active
          ? 'border-accent bg-accent/10 text-accent-fg'
          : 'border-border text-muted hover:text-fg'
      }`}
    >
      {dot && <span aria-hidden className={`size-1.5 rounded-full ${dot}`} />}
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
  const [filterOpen, setFilterOpen] = useState(false)
  const navigate = useNavigate()

  // 排程（未開始）不列在紀錄；即時會議已結束待上傳（scheduled + liveStartedAt）要列出來提醒上傳
  const list = useMemo(() => meetings.filter((m) => m.status !== 'scheduled' || m.liveStartedAt), [meetings])

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
  // 進階篩選（來源、標籤）收在 Sheet，作用中的數量顯示在「篩選」鈕上
  const advancedCount = (source !== 'all' ? 1 : 0) + (tag ? 1 : 0)
  const filtering = Boolean(search) || scenario !== 'all' || advancedCount > 0

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

      {/* 單排篩選：情境 chips + 進階篩選鈕（來源、標籤） */}
      <div className="-mx-4 flex gap-2 overflow-x-auto px-4 pb-1 [scrollbar-width:none]">
        <Chip active={scenario === 'all'} onClick={() => setScenario('all')}>
          全部
        </Chip>
        {scenarioKeys.map((s) => (
          <Chip
            key={s}
            active={scenario === s}
            dot={scenarioThemes[s].dot}
            onClick={() => setScenario(scenario === s ? 'all' : s)}
          >
            {scenarioLabels[s]}
          </Chip>
        ))}
        <Chip active={advancedCount > 0} onClick={() => setFilterOpen(true)}>
          <SlidersHorizontal className="size-3.5" />
          篩選{advancedCount > 0 && ` ${advancedCount}`}
        </Chip>
      </div>

      {filterOpen && (
        <Sheet onClose={() => setFilterOpen(false)} className="max-h-[85dvh] overflow-y-auto">
          <div className="flex items-center justify-between">
            <h2 className="m-0 text-heading font-semibold">篩選</h2>
            <button type="button" className="btn btn-ghost size-9 px-0" aria-label="關閉" onClick={() => setFilterOpen(false)}>
              <X className="size-4" />
            </button>
          </div>
          <div className="flex flex-col gap-2">
            <h3 className="m-0 text-xs font-medium text-muted">來源</h3>
            <div className="flex flex-wrap gap-2">
              <Chip active={source === 'all'} onClick={() => setSource('all')}>
                全部
              </Chip>
              <Chip active={source === 'recorded'} onClick={() => setSource('recorded')}>
                錄音
              </Chip>
              <Chip active={source === 'imported'} onClick={() => setSource('imported')}>
                匯入
              </Chip>
            </div>
          </div>
          {allTags.length > 0 && (
            <div className="flex flex-col gap-2">
              <h3 className="m-0 text-xs font-medium text-muted">標籤</h3>
              <div className="flex flex-wrap gap-2">
                {allTags.map((t) => (
                  <Chip key={t} active={tag === t} onClick={() => setTag(tag === t ? null : t)}>
                    #{t}
                  </Chip>
                ))}
              </div>
            </div>
          )}
          <div className="mt-2 flex gap-2">
            <button
              type="button"
              className="btn btn-secondary flex-1"
              onClick={() => {
                setSource('all')
                setTag(null)
              }}
            >
              清除
            </button>
            <button type="button" className="btn btn-primary flex-1" onClick={() => setFilterOpen(false)}>
              完成
            </button>
          </div>
        </Sheet>
      )}

      {error && (
        <div className="flex items-center justify-between gap-3 rounded-xl border border-danger/30 bg-danger/5 px-4 py-3 text-sm text-danger">
          {error}
          <button type="button" className="btn btn-secondary h-9" onClick={reload}>
            重新載入
          </button>
        </div>
      )}

      {!error && (
        <MeetingList
          meetings={filtered}
          groupByDate
          emptyText={filtering ? '沒有符合的紀錄' : '還沒有紀錄'}
          emptyDescription={filtering ? '換個關鍵字或清除篩選試試。' : '錄下第一段對話，AI 會幫你整理重點與待辦。'}
          emptyAction={
            filtering ? undefined : (
              <button type="button" className="btn btn-primary" onClick={() => navigate('/')}>
                <Mic className="size-4" />
                開始錄音
              </button>
            )
          }
        />
      )}
    </div>
  )
}
