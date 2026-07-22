import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'

import { RecorderPanel } from '../components/RecorderPanel'
import { ScenarioToggle } from '../components/ScenarioToggle'
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

  return (
    // 填滿內容區：情境切換置頂、錄音鈕置於正中，上傳貼近底部（整頁一屏，不需捲動）
    <div className="flex h-full flex-1 flex-col">
      <div className="flex flex-1 flex-col items-center justify-center gap-8">
        <ScenarioToggle value={scenario} onChange={setScenario} />
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
