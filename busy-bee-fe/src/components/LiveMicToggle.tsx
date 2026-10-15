import { useState } from 'react'
import { Mic } from 'lucide-react'

import { useApiCall } from '../hooks/useApiCall'
import { useAuth } from '../hooks/useAuth'
import { updatePreferences } from '../services/api/client'

/** 即時會議是否同時即時辨識自己的麥克風（關＝省錢模式，約省 4 成即時辨識費用）。 */
export function LiveMicToggle() {
  const { user, updateUser } = useAuth()
  const call = useApiCall()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const on = user?.liveMicTranscription ?? false

  const toggle = async () => {
    setBusy(true)
    setError(null)
    try {
      const res = await call(updatePreferences, { liveMicTranscription: !on })
      updateUser({ liveMicTranscription: res.liveMicTranscription })
    } catch (e) {
      setError(e instanceof Error ? e.message : '更新失敗')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex flex-col gap-1">
      <div className="flex items-center gap-2">
        <Mic className="size-4" />
        <span className="text-sm">即時辨識我的麥克風</span>
        <button
          type="button"
          role="switch"
          aria-checked={on}
          aria-label="即時辨識我的麥克風"
          disabled={busy}
          onClick={() => void toggle()}
          className={`relative ml-auto h-6 w-11 shrink-0 rounded-full transition disabled:opacity-50 ${on ? 'bg-accent' : 'bg-border'}`}
        >
          <span className={`absolute top-0.5 size-5 rounded-full bg-white shadow transition-all ${on ? 'left-[1.375rem]' : 'left-0.5'}`} />
        </button>
      </div>
      <p className="m-0 text-xs text-muted">
        即時會議中把你說的話也即時轉成文字。關閉較省錢；不論開關，你的聲音都會錄進存檔與會後紀錄。
      </p>
      {error && <p className="m-0 text-xs text-red-500">{error}</p>}
    </div>
  )
}
