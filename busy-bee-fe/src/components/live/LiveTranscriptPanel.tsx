import { useLayoutEffect, useRef, useState } from 'react'
import { ArrowDown } from 'lucide-react'

import type { TranscriptLine, Who } from '../../live/session'

const whoLabel: Record<Who, string> = { remote: '對方', local: '我' }

function hhmm(ts: number): string {
  const d = new Date(ts)
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

/** 離底部這麼近就視為「在看最新」，新內容進來自動捲到底。 */
const STICK_THRESHOLD_PX = 48

/** 即時逐字稿：新內容自動捲到底；使用者往上捲時暫停並顯示「回到最新」。 */
export function LiveTranscriptPanel({
  lines,
  interim,
}: {
  lines: TranscriptLine[]
  interim: Record<Who, string>
}) {
  const boxRef = useRef<HTMLDivElement>(null)
  const [stick, setStick] = useState(true)

  useLayoutEffect(() => {
    const box = boxRef.current
    if (box && stick) box.scrollTop = box.scrollHeight
  }, [lines, interim, stick])

  const onScroll = () => {
    const box = boxRef.current
    if (!box) return
    setStick(box.scrollHeight - box.scrollTop - box.clientHeight < STICK_THRESHOLD_PX)
  }

  const toLatest = () => {
    const box = boxRef.current
    if (box) box.scrollTop = box.scrollHeight
    setStick(true)
  }

  const pending = (['remote', 'local'] as const).filter((w) => interim[w])

  return (
    <div className="relative flex min-h-0 flex-1 flex-col">
      <div
        ref={boxRef}
        data-testid="live-transcript"
        onScroll={onScroll}
        className="min-h-0 flex-1 overflow-y-auto pr-1 text-sm leading-6"
        aria-live="polite"
      >
        {lines.length === 0 && pending.length === 0 && (
          <p className="m-0 py-6 text-center text-muted">等待對方說話…</p>
        )}
        {lines.map((l) =>
          l.gap ? (
            <p key={l.id} className="my-1 text-center text-xs text-amber-600 dark:text-amber-400">
              {l.text}
            </p>
          ) : (
            <p key={l.id} className="my-1.5 flex gap-2">
              <span className="w-10 shrink-0 text-xs leading-6 text-muted tabular-nums">{hhmm(l.at)}</span>
              <span
                className={`shrink-0 text-xs leading-6 font-medium ${l.who === 'local' ? 'text-accent' : 'text-muted'}`}
              >
                {whoLabel[l.who]}
              </span>
              <span className="min-w-0 flex-1 break-words">{l.text}</span>
            </p>
          ),
        )}
        {pending.map((w) => (
          <p key={w} className="my-1.5 flex gap-2 text-muted">
            <span className="w-10 shrink-0" />
            <span className="shrink-0 text-xs leading-6 font-medium">{whoLabel[w]}</span>
            <span className="min-w-0 flex-1 break-words italic">{interim[w]}</span>
          </p>
        ))}
      </div>
      {!stick && (
        <button
          type="button"
          onClick={toLatest}
          className="btn btn-secondary absolute bottom-2 left-1/2 h-8 -translate-x-1/2 rounded-full px-3 text-xs shadow"
        >
          <ArrowDown className="size-3.5" />
          回到最新
        </button>
      )}
    </div>
  )
}
