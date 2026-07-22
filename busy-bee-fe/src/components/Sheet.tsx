import { type ReactNode, useEffect } from 'react'
import { createPortal } from 'react-dom'

const panelBase =
  'animate-sheet-up sm:animate-fade-in fixed inset-x-0 bottom-0 z-50 flex flex-col gap-3 rounded-t-2xl border-t border-border bg-surface p-4 pb-[calc(1.25rem+env(safe-area-inset-bottom))] sm:inset-x-auto sm:top-1/2 sm:left-1/2 sm:bottom-auto sm:w-full sm:max-w-sm sm:-translate-x-1/2 sm:-translate-y-1/2 sm:rounded-2xl sm:border sm:pb-4'

/**
 * 底部彈出面板（手機）/ 置中對話框（桌機）。
 *
 * 一律透過 Portal 掛到 document.body：詳情頁包在 AppShell 帶 transform 動畫的 <main> 裡，
 * 若直接渲染，transform 祖先會成為 position:fixed 的 containing block（iOS WebKit 尤其明顯），
 * 讓 `fixed bottom-0` 相對 <main> 底部而非視窗，長頁面時面板被推到頁尾要捲動才看得到。
 * Portal 到 body 後 fixed 永遠相對視窗，點擊即彈出。
 *
 * 開啟時鎖背景捲動並支援 Esc 關閉。`className` 可附加到面板（如需可捲動內容）。
 */
export function Sheet({
  onClose,
  children,
  className = '',
}: {
  onClose: () => void
  children: ReactNode
  className?: string
}) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', onKey)
    const prevOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.removeEventListener('keydown', onKey)
      document.body.style.overflow = prevOverflow
    }
  }, [onClose])

  return createPortal(
    <>
      <div className="animate-fade-in fixed inset-0 z-40 bg-black/50" onClick={onClose} />
      <div className={`${panelBase} ${className}`} role="dialog" aria-modal="true">
        {children}
      </div>
    </>,
    document.body,
  )
}
