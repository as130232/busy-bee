import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'

import { RecorderPanel } from '../components/RecorderPanel'
import { UploadZone } from '../components/UploadZone'

/** 錄音分頁：核心動作（大錄音鈕）+ 上傳。 */
export function RecordPage() {
  const navigate = useNavigate()

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
    <>
      <RecorderPanel onUploaded={() => navigate('/meetings')} highlight={highlight} />
      <UploadZone onUploaded={() => navigate('/meetings')} />
    </>
  )
}
