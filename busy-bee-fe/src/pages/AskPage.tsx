import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { RefreshCw, Sparkles } from 'lucide-react'
import ReactMarkdown from 'react-markdown'

import { askCrossMeetingQA, type QAResult, type QASource } from '../services/api/client'
import { useApiCall } from '../hooks/useApiCall'
import { QA_STORAGE_KEY } from '../services/localData'
import { formatDateTime } from '../utils/format'
import { EmptyState } from '../components/EmptyState'


/** 上次問答（存本地端，不進 DB）：切頁回來仍可看到，重新整理也保留。 */
interface StoredQA {
  question: string
  result: QAResult
  askedAt: string // ISO
}

function loadStored(): StoredQA | null {
  try {
    const raw = localStorage.getItem(QA_STORAGE_KEY)
    return raw ? (JSON.parse(raw) as StoredQA) : null
  } catch {
    return null
  }
}

/** 起始畫面的範例問題：點一下直接提問，降低「不知道能問什麼」的門檻。 */
const EXAMPLES = ['上週的會議決定了哪些事？', '還有哪些待辦沒完成？', '最近有人提到預算嗎？', '幫我整理這個月的重點']

/** 載入中的骨架：與答案卡同外框，讓等待有形狀。 */
function AnswerSkeleton() {
  return (
    <div className="animate-fade-in flex flex-col gap-3 rounded-xl border border-border bg-surface px-4 py-4" aria-hidden>
      {['w-2/5', 'w-full', 'w-11/12', 'w-4/5', 'w-3/5'].map((w, i) => (
        <span key={i} className={`h-3 animate-pulse rounded-full bg-surface-hover ${w} ${i === 0 ? 'mb-1 h-3.5' : ''}`} />
      ))}
    </div>
  )
}

/** 把答案中的 [n] 轉成 Markdown 連結錨點 [n](#cite-n)（僅限有對應來源者），交給 ReactMarkdown 渲染。 */
function linkifyCitations(answer: string, byIndex: Map<number, QASource>): string {
  return answer.replace(/\[(\d+)\]/g, (match, n) =>
    byIndex.has(Number(n)) ? `[[${n}]](#cite-${n})` : match,
  )
}

/** 問答分頁：對自己所有會議提問，取得帶引用的 AI 答案（單次問答、無狀態；上次結果存本地端）。 */
export function AskPage() {
  const stored = useMemo(loadStored, [])
  const [question, setQuestion] = useState(stored?.question ?? '')
  const [asked, setAsked] = useState<StoredQA | null>(stored)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const call = useApiCall()

  const result = asked?.result ?? null
  const byIndex = useMemo(
    () => new Map((result?.sources ?? []).map((s) => [s.index, s])),
    [result],
  )
  const markdown = useMemo(
    () => (result && !result.noMatch ? linkifyCitations(result.answer, byIndex) : ''),
    [result, byIndex],
  )

  const ask = async (q: string) => {
    const trimmed = q.trim()
    if (!trimmed || loading) return
    setLoading(true)
    setError(null)
    try {
      const res = await call(askCrossMeetingQA, trimmed)
      const record: StoredQA = { question: trimmed, result: res, askedAt: new Date().toISOString() }
      setAsked(record)
      try {
        localStorage.setItem(QA_STORAGE_KEY, JSON.stringify(record))
      } catch {
        // localStorage 滿或不可用時忽略：不影響本次顯示
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : '問答失敗，請稍後再試')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <form
        className="flex flex-col gap-2"
        onSubmit={(e) => {
          e.preventDefault()
          void ask(question)
        }}
      >
        <textarea
          className="input min-h-20 resize-none"
          placeholder="輸入問題，例如：上週產品會議決定了什麼？"
          value={question}
          maxLength={500}
          onChange={(e) => setQuestion(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
              e.preventDefault()
              void ask(question)
            }
          }}
        />
        <button type="submit" className="btn btn-primary" disabled={loading || !question.trim()}>
          <Sparkles className="size-4" />
          {loading ? '思考中…' : '提問'}
        </button>
      </form>

      {error && (
        <div className="rounded-xl border border-danger/30 bg-danger/5 px-4 py-3 text-sm text-danger">
          {error}
        </div>
      )}

      {loading && <AnswerSkeleton />}

      {!asked && !loading && !error && (
        <EmptyState
          title="問問你所有的紀錄"
          description="AI 會從你的會議與對話中找答案，並附上出處，點引用可直接跳到該場紀錄。"
          action={
            <div className="flex flex-wrap justify-center gap-2">
              {EXAMPLES.map((q) => (
                <button
                  key={q}
                  type="button"
                  className="rounded-full border border-border bg-surface px-3 py-1.5 text-xs text-fg transition outline-none hover:border-accent/50 hover:bg-surface-hover focus-visible:ring-2 focus-visible:ring-accent/50"
                  onClick={() => {
                    setQuestion(q)
                    void ask(q)
                  }}
                >
                  {q}
                </button>
              ))}
            </div>
          }
        />
      )}

      {asked && !error && !loading && (
        <div className="flex flex-col gap-4">
          <div className="rounded-xl border border-border bg-surface px-4 py-3">
            {/* 標頭：問題 + 建立時間 + 重新產生 */}
            <div className="mb-3 flex items-start justify-between gap-3 border-b border-border pb-3">
              <div className="min-w-0">
                <p className="truncate text-sm font-medium">{asked.question}</p>
                <p className="mt-0.5 font-mono text-xs text-muted">
                  {formatDateTime(asked.askedAt)}
                </p>
              </div>
              <button
                type="button"
                className="btn btn-secondary h-8 shrink-0 px-2.5 text-xs"
                disabled={loading}
                onClick={() => void ask(asked.question)}
              >
                <RefreshCw className={`size-3.5 ${loading ? 'animate-spin' : ''}`} />
                重新產生
              </button>
            </div>

            {result?.noMatch ? (
              <p className="text-sm text-muted">{result.answer}</p>
            ) : (
              <div className="prose prose-sm prose-zinc dark:prose-invert max-w-none prose-headings:font-semibold prose-h1:text-base prose-h2:mt-4 prose-h2:text-body prose-h3:text-sm prose-p:my-2 prose-ul:my-2 prose-li:my-0.5">
                <ReactMarkdown
                  skipHtml
                  components={{
                    a: ({ href, children }) => {
                      const m = /^#cite-(\d+)$/.exec(href ?? '')
                      const src = m ? byIndex.get(Number(m[1])) : undefined
                      if (src) {
                        return (
                          <Link
                            to={`/meetings/${src.meetingId}`}
                            className="mx-0.5 font-medium text-accent-fg no-underline"
                          >
                            {children}
                          </Link>
                        )
                      }
                      return (
                        <a href={href} target="_blank" rel="noreferrer">
                          {children}
                        </a>
                      )
                    },
                  }}
                >
                  {markdown}
                </ReactMarkdown>
              </div>
            )}
          </div>

          {result && result.sources.length > 0 && (
            <div className="flex flex-col gap-2">
              <p className="text-xs font-medium text-muted">參考來源</p>
              {result.sources.map((s) => (
                <Link
                  key={s.index}
                  to={`/meetings/${s.meetingId}`}
                  className="rounded-xl border border-border bg-surface px-4 py-3 transition hover:border-accent"
                >
                  <div className="flex items-baseline gap-2">
                    <span className="text-accent-fg text-sm font-semibold">[{s.index}]</span>
                    <span className="text-sm font-medium">{s.title}</span>
                  </div>
                  <p className="mt-1 line-clamp-2 text-xs text-muted">{s.snippet}</p>
                </Link>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
