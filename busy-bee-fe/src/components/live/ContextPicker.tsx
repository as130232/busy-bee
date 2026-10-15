import { useId, useState } from 'react'
import { ClipboardPaste, FileText, LoaderCircle, Paperclip, X } from 'lucide-react'

export interface ContextItem {
  key: string
  title: string
  /** 補充說明，如字數或「轉換中…」。 */
  detail?: string
}

/**
 * 會前資料清單與新增（F-LIVE）：貼上文字或上傳 .txt / .md / .pdf。
 * 開始前與會議中、詳情頁共用；實際上傳由呼叫端處理。
 */
export function ContextPicker({
  items,
  onAddText,
  onAddFile,
  onRemove,
  busy = false,
  error,
}: {
  items: ContextItem[]
  onAddText: (input: { title: string; text: string }) => void
  onAddFile: (file: File) => void
  onRemove?: (key: string) => void
  busy?: boolean
  error?: string | null
}) {
  const fileId = useId()
  const [pasting, setPasting] = useState(false)
  const [title, setTitle] = useState('')
  const [text, setText] = useState('')

  const submit = () => {
    if (!text.trim()) return
    onAddText({ title: title.trim(), text })
    setTitle('')
    setText('')
    setPasting(false)
  }

  return (
    <div className="flex flex-col gap-2">
      {items.length > 0 && (
        <ul className="m-0 flex list-none flex-col gap-1.5 p-0">
          {items.map((it) => (
            <li key={it.key} className="flex items-center gap-2 rounded-lg bg-surface-hover/60 px-3 py-2 text-sm">
              <FileText className="size-4 shrink-0 text-muted" />
              <span className="min-w-0 flex-1 truncate">{it.title}</span>
              {it.detail && <span className="shrink-0 text-xs text-muted">{it.detail}</span>}
              {onRemove && (
                <button
                  type="button"
                  className="btn btn-ghost size-7 px-0"
                  aria-label={`移除「${it.title}」`}
                  onClick={() => onRemove(it.key)}
                >
                  <X className="size-3.5" />
                </button>
              )}
            </li>
          ))}
        </ul>
      )}

      {pasting ? (
        <div className="flex flex-col gap-2">
          <input
            className="input"
            placeholder="標題（選填）"
            aria-label="資料標題"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
          />
          <textarea
            className="input h-32 resize-y py-2"
            placeholder="貼上會議相關的背景資料、議程、規格…"
            aria-label="資料內容"
            value={text}
            onChange={(e) => setText(e.target.value)}
          />
          <div className="flex gap-2">
            <button type="button" className="btn btn-secondary flex-1" onClick={() => setPasting(false)}>
              取消
            </button>
            <button type="button" className="btn btn-primary flex-1" disabled={!text.trim()} onClick={submit}>
              加入
            </button>
          </div>
        </div>
      ) : (
        <div className="flex gap-2">
          <button type="button" className="btn btn-secondary h-9 flex-1" onClick={() => setPasting(true)}>
            <ClipboardPaste className="size-4" />
            貼上文字
          </button>
          <label htmlFor={fileId} className="btn btn-secondary h-9 flex-1 cursor-pointer">
            <Paperclip className="size-4" />
            上傳檔案
          </label>
          <input
            id={fileId}
            type="file"
            aria-label="上傳檔案"
            accept=".txt,.md,.pdf"
            multiple
            className="sr-only"
            onChange={(e) => {
              for (const f of Array.from(e.target.files ?? [])) onAddFile(f)
              e.target.value = ''
            }}
          />
        </div>
      )}

      {busy && (
        <p className="m-0 flex items-center gap-1.5 text-xs text-muted">
          <LoaderCircle className="size-3.5 animate-spin" />
          資料處理中（PDF 需要數十秒）…
        </p>
      )}
      {error && <p className="m-0 text-xs text-red-500">{error}</p>}
      <p className="m-0 text-xs text-muted">支援 .txt、.md、.pdf（單檔 10MB），AI 只用來理解背景，不會當成會議內容。</p>
    </div>
  )
}
