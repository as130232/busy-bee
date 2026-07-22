import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'

import { RecorderPanel } from '../components/RecorderPanel'
import { ScenarioToggle } from '../components/ScenarioToggle'
import { scenarioThemes } from '../components/scenarioTheme'
import { UploadZone } from '../components/UploadZone'
import type { Scenario } from '../services/api/client'

/** 錄音分頁：核心動作（大錄音鈕）+ 上傳。 */
export function RecordPage() {
  const navigate = useNavigate()

  // 錄音/上傳前先選情境（會議 / 閒聊）；預設會議。決定 AI 產出的摘要區塊模板。
  const [scenario, setScenario] = useState<Scenario>('meeting')

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
    // 填滿內容區：情境切換靠上、錄音鈕置於正中，上傳貼近底部（整頁一屏，不需捲動）
    <div className="relative isolate flex h-full flex-1 flex-col">
      {/* 隨情境變色的背景暈染（大範圍柔光，切換時漸變） */}
      <span
        aria-hidden
        className={`pointer-events-none absolute top-1/2 left-1/2 -z-10 size-[28rem] -translate-x-1/2 -translate-y-1/2 rounded-full opacity-70 blur-3xl transition-colors duration-500 ${theme.tint}`}
      />
      <div className="flex justify-center pt-3">
        <ScenarioToggle value={scenario} onChange={setScenario} />
      </div>
      <div className="flex flex-1 flex-col items-center justify-center">
        <RecorderPanel
          onUploaded={() => navigate('/meetings')}
          highlight={highlight}
          scenario={scenario}
        />
      </div>
      <UploadZone onUploaded={() => navigate('/meetings')} scenario={scenario} />
    </div>
  )
}
