import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Link2, Upload, X } from 'lucide-react'

import { ImportLinkForm } from '../components/ImportLinkForm'
import { LanguageSelect } from '../components/LanguageSelect'
import { RecorderPanel } from '../components/RecorderPanel'
import { ScenarioToggle } from '../components/ScenarioToggle'
import { scenarioThemes } from '../components/scenarioTheme'
import { Sheet } from '../components/Sheet'
import { UploadZone } from '../components/UploadZone'
import { useShareImport } from '../hooks/useShareImport'
import type { Language, Scenario } from '../services/api/client'

type SheetKind = null | 'upload' | 'link'

/** 首頁次要匯入動作按鈕（點擊彈出對應 Sheet）。 */
function ActionButton({
  icon: Icon,
  label,
  onClick,
}: {
  icon: typeof Upload
  label: string
  onClick: () => void
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex h-11 items-center gap-2 rounded-full border border-border bg-surface/70 px-5 text-sm font-medium text-fg backdrop-blur-sm transition hover:bg-surface active:scale-[0.97]"
    >
      <Icon className="size-4 text-muted" />
      {label}
    </button>
  )
}

/** 錄音分頁：核心動作（大錄音鈕）+ 匯入捷徑（上傳 / 貼連結，彈窗）。 */
export function RecordPage() {
  const navigate = useNavigate()
  const toMeetings = () => navigate('/meetings')

  // 錄音/上傳前先選情境；預設會議。決定 AI 產出的摘要區塊模板。
  const [scenario, setScenario] = useState<Scenario>('meeting')
  // 錄音/上傳前先選語言；預設中文。決定 STT 辨識語言。
  const [language, setLanguage] = useState<Language>('zh-TW')
  const [sheet, setSheet] = useState<SheetKind>(null)

  // 分享/捷徑帶進來的連結（?import= 或 PWA share_target）自動匯入
  const shareImport = useShareImport()

  // 由提醒推播深連結（/?record=1）進入時高亮錄音鈕，3 秒後清除 query
  const [searchParams, setSearchParams] = useSearchParams()
  const [highlight, setHighlight] = useState(false)
  useEffect(() => {
    if (searchParams.get('record') !== '1') return
    setHighlight(true)
    const timer = setTimeout(() => {
      setHighlight(false)
      setSearchParams({}, { replace: true })
    }, 3000)
    return () => clearTimeout(timer)
  }, [searchParams, setSearchParams])

  const theme = scenarioThemes[scenario]

  return (
    // 情境切換靠上、錄音鈕置中、匯入捷徑貼底（整頁一屏，不需捲動）
    <div className="relative isolate flex h-full flex-1 flex-col">
      {/* 隨情境變色的背景暈染（大範圍柔光，切換時漸變） */}
      <span
        aria-hidden
        className={`pointer-events-none absolute top-1/2 left-1/2 -z-10 size-[28rem] -translate-x-1/2 -translate-y-1/2 rounded-full opacity-70 blur-3xl transition-colors duration-500 ${theme.tint}`}
      />

      {shareImport === 'importing' && (
        <div className="mx-3 mt-3 rounded-xl border border-accent/30 bg-accent/5 px-4 py-2 text-center text-sm text-accent">
          正在匯入分享的連結…
        </div>
      )}
      {shareImport === 'error' && (
        <div className="mx-3 mt-3 rounded-xl border border-red-500/30 bg-red-500/5 px-4 py-2 text-center text-sm text-red-500">
          匯入失敗，請改用「貼連結」
        </div>
      )}

      <div className="flex flex-wrap items-center justify-center gap-2 pt-3">
        <ScenarioToggle value={scenario} onChange={setScenario} />
        <LanguageSelect value={language} onChange={setLanguage} />
      </div>

      <div className="flex flex-1 flex-col items-center justify-center">
        <RecorderPanel
          onUploaded={toMeetings}
          highlight={highlight}
          scenario={scenario}
          language={language}
        />
      </div>

      {/* 匯入捷徑：兩顆簡約按鈕，點擊彈出對應視窗 */}
      <div className="flex items-center justify-center gap-3 pb-3">
        <ActionButton icon={Upload} label="上傳" onClick={() => setSheet('upload')} />
        <ActionButton icon={Link2} label="連結" onClick={() => setSheet('link')} />
      </div>

      {sheet === 'upload' && (
        <Sheet onClose={() => setSheet(null)}>
          <SheetHeader title="上傳音訊檔" onClose={() => setSheet(null)} />
          <UploadZone onUploaded={toMeetings} scenario={scenario} language={language} />
        </Sheet>
      )}
      {sheet === 'link' && (
        <Sheet onClose={() => setSheet(null)}>
          <SheetHeader title="貼連結匯入" onClose={() => setSheet(null)} />
          <ImportLinkForm onImported={toMeetings} scenario={scenario} language={language} />
        </Sheet>
      )}
    </div>
  )
}

function SheetHeader({ title, onClose }: { title: string; onClose: () => void }) {
  return (
    <div className="flex items-center justify-between">
      <h2 className="m-0 text-base font-semibold">{title}</h2>
      <button type="button" className="btn btn-ghost size-9 px-0" aria-label="關閉" onClick={onClose}>
        <X className="size-4" />
      </button>
    </div>
  )
}
