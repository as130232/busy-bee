import { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'

import { extractURL } from '../services/url'

/**
 * 讀取由分享/捷徑帶進來的連結：`?import=<url>`，或 PWA share_target 的 `url` / `text` 參數。
 * 只抽出連結並清掉 query（避免重整重複觸發），**不自動匯入**——匯入會產生抓取與 AI 費用，
 * 外部網頁可構造此連結，故由呼叫端預填匯入表單，使用者按下確認才送出。
 */
export function useShareImport(): string {
  const [params, setParams] = useSearchParams()
  const [sharedUrl] = useState(() =>
    extractURL(params.get('import') || params.get('url') || params.get('text') || ''),
  )

  useEffect(() => {
    if (sharedUrl) setParams({}, { replace: true })
  }, [sharedUrl, setParams])

  return sharedUrl
}
