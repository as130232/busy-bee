import { useState, type FormEvent } from 'react'
import { LoaderCircle, Send } from 'lucide-react'

import type { LiveQA } from '../../services/api/client'

/** 會議中提問：只依本場逐字稿與會前資料回答，並標示依據。 */
export function LiveAskPanel({
  qa,
  asking,
  error,
  disabled,
  onAsk,
}: {
  qa: LiveQA[]
  asking: boolean
  error: string | null
  disabled: boolean
  onAsk: (q: string) => void
}) {
  const [q, setQ] = useState('')
  const submit = (e: FormEvent) => {
    e.preventDefault()
    if (!q.trim() || asking) return
    onAsk(q)
    setQ('')
  }
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3">
      <div className="min-h-0 flex-1 overflow-y-auto">
        {qa.length === 0 && !asking && (
          <p className="m-0 py-6 text-center text-sm text-muted">例如：「剛剛說的上線日期是哪天？」</p>
        )}
        <ul className="m-0 flex list-none flex-col gap-3 p-0">
          {qa.map((item, i) => (
            <li key={i} className="flex flex-col gap-1">
              <p className="m-0 text-sm font-medium">Q：{item.question}</p>
              <p className="m-0 text-sm leading-6 whitespace-pre-wrap">{item.answer}</p>
              {item.sources.length > 0 && (
                <div className="flex flex-wrap gap-1">
                  {item.sources.map((s, j) => (
                    <span key={j} className="rounded-full bg-surface-hover px-2 py-0.5 text-xs text-muted">
                      {s.type === 'transcript' ? `逐字稿 ${s.label}` : `資料：${s.label}`}
                    </span>
                  ))}
                </div>
              )}
            </li>
          ))}
        </ul>
        {asking && (
          <p className="m-0 mt-3 flex items-center gap-1.5 text-sm text-muted">
            <LoaderCircle className="size-4 animate-spin" />
            思考中…
          </p>
        )}
        {error && <p className="m-0 mt-2 text-sm text-red-500">{error}</p>}
      </div>
      <form onSubmit={submit} className="flex gap-2">
        <input
          className="input flex-1"
          placeholder="問這場會議的內容…"
          aria-label="會議中提問"
          value={q}
          disabled={disabled}
          onChange={(e) => setQ(e.target.value)}
        />
        <button type="submit" className="btn btn-primary w-11 px-0" aria-label="送出提問" disabled={!q.trim() || asking || disabled}>
          <Send className="size-4" />
        </button>
      </form>
    </div>
  )
}
