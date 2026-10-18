import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { ChevronLeft, FileText, LoaderCircle, Mic, MicOff, Radio, Square, TriangleAlert } from 'lucide-react'

import { ContextPicker, type ContextItem } from '../components/live/ContextPicker'
import { LiveAskPanel } from '../components/live/LiveAskPanel'
import { LiveNotesPanel } from '../components/live/LiveNotesPanel'
import { LiveTranscriptPanel } from '../components/live/LiveTranscriptPanel'
import { LanguageSelect } from '../components/LanguageSelect'
import { ScenarioToggle } from '../components/ScenarioToggle'
import { Sheet } from '../components/Sheet'
import { useApiCall } from '../hooks/useApiCall'
import { useAuth } from '../hooks/useAuth'
import { useLiveSession } from '../hooks/useLiveSession'
import { isLiveSupported } from '../live/audio'
import type { ChannelState, ContextInput, SessionState } from '../live/session'
import { getMeeting, listMeetingContexts, type Language, type MeetingContext, type Scenario } from '../services/api/client'
import { formatClock, formatFileStamp } from '../utils/format'

const channelLabel: Record<ChannelState, string> = {
  off: '未啟用',
  connecting: '連線中',
  open: '即時辨識中',
  reconnecting: '重新連線中',
  failed: '即時辨識已停用',
  quota: '本月額度已用完',
  stopped: '已停止',
}

/** 停止分享後自動結束的倒數秒數。 */
const AUTO_FINISH_SECONDS = 30

/** 即時會議（F-LIVE）：開始前設定 → 會議中（重點／逐字稿／提問）→ 結束上傳。 */
export function LivePage() {
  const navigate = useNavigate()
  const { session, state } = useLiveSession()

  useEffect(() => {
    if (state.phase === 'done' && state.meetingId) navigate(`/meetings/${state.meetingId}`, { replace: true })
  }, [state.phase, state.meetingId, navigate])

  if (!isLiveSupported()) {
    return (
      <Shell>
        <p className="m-0 rounded-xl border border-border bg-surface p-4 text-sm">
          即時會議需要電腦版 Chrome（用來擷取 Google Meet 分頁的聲音）。手機與 Safari 目前不支援，請改用一般錄音。
        </p>
      </Shell>
    )
  }
  if (state.phase === 'idle' || state.phase === 'starting') {
    return <LiveSetup starting={state.phase === 'starting'} error={state.error} onStart={(i) => void session.start(i)} />
  }
  if (state.phase === 'live') return <LiveView state={state} session={session} />
  return <LiveFinishing state={state} onRetry={() => void session.retryUpload()} />
}

function Shell({ children, back = true }: { children: React.ReactNode; back?: boolean }) {
  return (
    <div className="mx-auto flex min-h-dvh max-w-xl flex-col gap-5 px-4 pt-[calc(0.75rem+env(safe-area-inset-top))] pb-8">
      <header className="flex items-center gap-2">
        {back && (
          <Link to="/" className="btn btn-ghost size-11 shrink-0 px-0" aria-label="返回">
            <ChevronLeft className="size-5" />
          </Link>
        )}
        <h1 className="m-0 flex items-center gap-2 text-lg font-semibold">
          <Radio className="size-5 text-rose-500" />
          即時會議
        </h1>
      </header>
      {children}
    </div>
  )
}

// ---- 開始前 ----

type PendingContext = { key: string; input: ContextInput }

function LiveSetup({
  starting,
  error,
  onStart,
}: {
  starting: boolean
  error: string | null
  onStart: (input: Parameters<ReturnType<typeof useLiveSession>['session']['start']>[0]) => void
}) {
  const { user } = useAuth()
  const call = useApiCall()
  const [params] = useSearchParams()
  const scheduledId = params.get('scheduled') ?? undefined
  const [title, setTitle] = useState(() => `會議 ${formatFileStamp(new Date())}`)
  const [scenario, setScenario] = useState<Scenario>('meeting')
  const [language, setLanguage] = useState<Language>('zh-TW')
  const [pending, setPending] = useState<PendingContext[]>([])
  const [existing, setExisting] = useState<MeetingContext[]>([])

  // 從排程開始：帶入排程的標題、情境、語言與已上傳的會前資料
  useEffect(() => {
    if (!scheduledId) return
    void (async () => {
      try {
        const [{ meeting }, { contexts }] = await Promise.all([
          call(getMeeting, scheduledId),
          call(listMeetingContexts, scheduledId),
        ])
        setTitle(meeting.title)
        setScenario(meeting.scenario)
        setLanguage(meeting.language)
        setExisting(contexts)
      } catch {
        /* 讀不到就用預設值 */
      }
    })()
  }, [scheduledId, call])

  const items: ContextItem[] = [
    ...existing.map((c) => ({ key: c.id, title: c.title, detail: `${c.chars.toLocaleString()} 字` })),
    ...pending.map((p) => ({
      key: p.key,
      title: p.input instanceof File ? p.input.name : p.input.title || '貼上的文字',
      detail: '開始後上傳',
    })),
  ]

  return (
    <Shell>
      <section className="flex flex-col gap-3 rounded-xl border border-border bg-surface p-4">
        <label className="flex flex-col gap-1 text-sm">
          標題
          <input className="input" value={title} onChange={(e) => setTitle(e.target.value)} />
        </label>
        <div className="flex flex-wrap items-center gap-2">
          <ScenarioToggle value={scenario} onChange={setScenario} />
          <LanguageSelect value={language} onChange={setLanguage} />
        </div>
      </section>

      <section className="flex flex-col gap-2 rounded-xl border border-border bg-surface p-4">
        <h2 className="m-0 flex items-center gap-2 text-sm font-semibold">
          <FileText className="size-4" />
          會前資料（選填）
        </h2>
        <p className="m-0 text-xs text-muted">議程、規格、背景說明等，讓 AI 更懂上下文、專有名詞也辨識得更準。</p>
        <ContextPicker
          items={items}
          onAddText={(t) => setPending((p) => [...p, { key: crypto.randomUUID(), input: t }])}
          onAddFile={(f) => setPending((p) => [...p, { key: crypto.randomUUID(), input: f }])}
          onRemove={(key) => setPending((p) => p.filter((x) => x.key !== key))}
        />
      </section>

      <section className="flex flex-col gap-1.5 rounded-xl border border-border bg-surface p-4 text-sm">
        <p className="m-0 flex items-center gap-2">
          {user?.liveMicTranscription ? <Mic className="size-4" /> : <MicOff className="size-4 text-muted" />}
          麥克風即時辨識：{user?.liveMicTranscription ? '開' : '關（省錢模式）'}
          <Link to="/settings" className="ml-auto text-xs text-accent-fg">
            設定
          </Link>
        </p>
        <p className="m-0 text-xs text-muted">
          不論開關，你的聲音都會錄進存檔，會後的完整紀錄一定包含你的發言。
        </p>
      </section>

      <ol className="m-0 flex flex-col gap-1 pl-5 text-xs text-muted">
        <li>按「開始」後，在 Chrome 視窗選擇 Google Meet 分頁，並勾選「分享分頁音訊」。</li>
        <li>建議把 busy-bee 開在另一個視窗，和 Meet 並排。</li>
        <li>建議戴耳機：喇叭外放時麥克風會收到對方聲音，錄音會有回音。</li>
        <li>請依公司規範告知與會者本場有錄音與 AI 整理。</li>
      </ol>

      {error && <p className="m-0 text-sm text-danger">{error}</p>}

      <button
        type="button"
        className="btn btn-primary h-12"
        disabled={starting || !title.trim()}
        onClick={() =>
          onStart({
            title: title.trim(),
            scenario,
            language,
            micTranscription: user?.liveMicTranscription ?? false,
            scheduledMeetingId: scheduledId,
            contexts: pending.map((p) => p.input),
          })
        }
      >
        {starting ? <LoaderCircle className="size-4 animate-spin" /> : <Radio className="size-4" />}
        開始即時會議
      </button>
    </Shell>
  )
}

// ---- 會議中 ----

type MobileTab = 'notes' | 'transcript' | 'ask'

function LiveView({ state, session }: { state: SessionState; session: ReturnType<typeof useLiveSession>['session'] }) {
  const [tab, setTab] = useState<MobileTab>('notes')
  const [confirmEnd, setConfirmEnd] = useState(false)
  const [contextsOpen, setContextsOpen] = useState(false)
  const [now, setNow] = useState(() => Date.now())
  const [countdown, setCountdown] = useState(AUTO_FINISH_SECONDS)

  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(t)
  }, [])

  // 停止分享 → 倒數後自動結束
  useEffect(() => {
    if (!state.sourceEnded) return
    const t = setInterval(() => setCountdown((c) => c - 1), 1000)
    return () => clearInterval(t)
  }, [state.sourceEnded])
  useEffect(() => {
    if (state.sourceEnded && countdown <= 0) void session.finish()
  }, [state.sourceEnded, countdown, session])

  const elapsed = state.startedAt ? Math.floor((now - state.startedAt) / 1000) : 0
  const liveOff = state.liveDisabled || state.remoteState === 'failed'
  const contextItems = useMemo<ContextItem[]>(
    () => state.contexts.map((c) => ({ key: c.id, title: c.title, detail: `${c.chars.toLocaleString()} 字` })),
    [state.contexts],
  )

  const notes = (
    <LiveNotesPanel
      notes={state.notes}
      busy={state.notesBusy}
      error={state.notesError}
      updatedAt={state.notesUpdatedAt}
      disabled={liveOff}
      onRefresh={() => void session.updateNotesNow()}
    />
  )
  const transcript = <LiveTranscriptPanel lines={state.lines} interim={state.interim} />
  const ask = (
    <LiveAskPanel qa={state.qa} asking={state.asking} error={state.askError} disabled={false} onAsk={(q) => void session.ask(q)} />
  )

  return (
    <div className="flex h-dvh flex-col">
      <header className="flex shrink-0 flex-wrap items-center gap-x-3 gap-y-1 border-b border-border px-4 pt-[calc(0.5rem+env(safe-area-inset-top))] pb-2">
        <span className="flex items-center gap-1.5 text-sm font-semibold tabular-nums">
          <span className="size-2 animate-pulse rounded-full bg-rose-500" />
          {formatClock(elapsed)}
        </span>
        <span className="text-xs text-muted">
          對方：{channelLabel[state.remoteState]}
          {state.localState !== 'off' && ` · 我：${channelLabel[state.localState]}`}
        </span>
        {state.usage && (
          <span className="text-xs text-muted tabular-nums">
            本月 {state.usage.usedMinutes}/{state.usage.limitMinutes} 分
          </span>
        )}
        <div className="ml-auto flex gap-2">
          <button type="button" className="btn btn-secondary h-9 px-3" onClick={() => setContextsOpen(true)}>
            <FileText className="size-4" />
            會前資料{state.contexts.length > 0 && ` ${state.contexts.length}`}
          </button>
          <button type="button" className="btn btn-primary h-9 bg-rose-600 px-3" onClick={() => setConfirmEnd(true)}>
            <Square className="size-3.5" />
            結束
          </button>
        </div>
      </header>

      {(state.liveDisabled || state.micDenied || state.backupFailed || state.remoteState === 'failed') && (
        <div className="flex shrink-0 flex-col gap-1 border-b border-amber-500/30 bg-amber-500/10 px-4 py-2 text-xs text-amber-700 dark:text-amber-300">
          {state.liveDisabled && <Notice>本月即時辨識額度已用完，錄音繼續，會後照常產生完整紀錄。</Notice>}
          {state.remoteState === 'failed' && !state.liveDisabled && (
            <Notice>即時辨識連線失敗已停用，錄音繼續，會後照常產生完整紀錄。</Notice>
          )}
          {state.micDenied && <Notice>沒有麥克風權限，這場只錄得到對方的聲音。</Notice>}
          {state.backupFailed && <Notice>本機備份無法使用，請勿關閉這個分頁直到上傳完成。</Notice>}
        </div>
      )}

      {/* 寬版：左（重點＋提問）右（逐字稿）；窄版：頁籤切換 */}
      <nav className="flex shrink-0 border-b border-border md:hidden">
        {(
          [
            ['notes', '重點'],
            ['transcript', '逐字稿'],
            ['ask', '提問'],
          ] as const
        ).map(([k, label]) => (
          <button
            key={k}
            type="button"
            className={`-mb-px h-10 flex-1 border-b-2 text-sm font-medium ${tab === k ? 'border-accent text-fg' : 'border-transparent text-muted'}`}
            onClick={() => setTab(k)}
          >
            {label}
          </button>
        ))}
      </nav>
      <main className="flex min-h-0 flex-1 flex-col p-4 md:hidden">
        {tab === 'notes' ? notes : tab === 'transcript' ? transcript : ask}
      </main>
      <main className="hidden min-h-0 flex-1 gap-4 p-4 md:grid md:grid-cols-2">
        <div className="flex min-h-0 flex-col gap-4">
          <Panel title="即時重點" className="flex-[3]">
            {notes}
          </Panel>
          <Panel title="提問" className="flex-[2]">
            {ask}
          </Panel>
        </div>
        <Panel title="即時逐字稿">{transcript}</Panel>
      </main>

      {confirmEnd && (
        <Sheet onClose={() => setConfirmEnd(false)}>
          <p className="m-0 text-sm">結束會議並上傳錄音？上傳後會自動產生完整紀錄。</p>
          <div className="flex gap-2">
            <button type="button" className="btn btn-secondary flex-1" onClick={() => setConfirmEnd(false)}>
              繼續會議
            </button>
            <button type="button" className="btn btn-primary flex-1 bg-rose-600" onClick={() => void session.finish()}>
              結束並上傳
            </button>
          </div>
        </Sheet>
      )}

      {state.sourceEnded && (
        <Sheet onClose={() => void session.finish()}>
          <p className="m-0 text-sm">已停止分享 Meet 分頁，{Math.max(countdown, 0)} 秒後自動結束並上傳錄音。</p>
          <button type="button" className="btn btn-primary bg-rose-600" onClick={() => void session.finish()}>
            立即結束
          </button>
        </Sheet>
      )}

      {contextsOpen && (
        <Sheet onClose={() => setContextsOpen(false)} className="max-h-[85dvh] overflow-y-auto">
          <h2 className="m-0 text-base font-semibold">會前資料</h2>
          <ContextPicker
            items={contextItems}
            busy={state.contextBusy}
            error={state.contextError}
            onAddText={(t) => void session.addContext(t)}
            onAddFile={(f) => void session.addContext(f)}
          />
        </Sheet>
      )}
    </div>
  )
}

function Notice({ children }: { children: React.ReactNode }) {
  return (
    <p className="m-0 flex items-start gap-1.5">
      <TriangleAlert className="mt-0.5 size-3.5 shrink-0" />
      {children}
    </p>
  )
}

function Panel({ title, children, className = '' }: { title: string; children: React.ReactNode; className?: string }) {
  return (
    <section className={`flex min-h-0 flex-1 flex-col gap-2 rounded-xl border border-border bg-surface p-4 ${className}`}>
      <h2 className="m-0 text-sm font-semibold">{title}</h2>
      {children}
    </section>
  )
}

// ---- 結束 ----

function LiveFinishing({ state, onRetry }: { state: SessionState; onRetry: () => void }) {
  return (
    <Shell back={state.phase === 'error'}>
      <section className="flex flex-col items-center gap-3 rounded-xl border border-border bg-surface p-6 text-center">
        {state.phase === 'error' ? (
          <>
            <p className="m-0 text-sm text-danger">{state.error}</p>
            <p className="m-0 text-xs text-muted">錄音已保存在這台電腦，重試或之後重新開啟 busy-bee 都能再上傳。</p>
            <button type="button" className="btn btn-primary" onClick={onRetry}>
              重試上傳
            </button>
          </>
        ) : (
          <>
            <LoaderCircle className="size-6 animate-spin text-accent" />
            <p className="m-0 text-sm">
              {state.phase === 'finishing' ? '結束會議中…' : `上傳錄音中… ${state.uploadPercent}%`}
            </p>
            <p className="m-0 text-xs text-muted">請勿關閉分頁；上傳完成後會自動產生完整紀錄。</p>
          </>
        )}
      </section>
    </Shell>
  )
}
