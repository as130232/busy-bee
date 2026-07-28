import { useEffect, useRef, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'

import { importMeeting } from '../services/api/client'
import { getIdToken } from '../services/token'
import { extractURL } from '../services/url'

export type ShareImportState = 'idle' | 'importing' | 'error'

/**
 * 處理由分享/捷徑帶進來的連結：`?import=<url>`，或 PWA share_target 的 `url` / `text` 參數。
 * 抓到連結即自動匯入並導向紀錄頁；先清掉 query 避免重整重複匯入。
 */
export function useShareImport(): ShareImportState {
  const navigate = useNavigate()
  const [params, setParams] = useSearchParams()
  const [state, setState] = useState<ShareImportState>('idle')
  const handled = useRef(false)

  useEffect(() => {
    if (handled.current) return
    const raw = params.get('import') || params.get('url') || params.get('text') || ''
    const url = extractURL(raw)
    if (!url) return

    handled.current = true
    setParams({}, { replace: true }) // 清掉 query，避免重整/返回時重複匯入
    setState('importing')
    void (async () => {
      try {
        await importMeeting(await getIdToken(), { url })
        navigate('/meetings')
      } catch {
        setState('error')
      }
    })()
  }, [params, setParams, navigate])

  return state
}
