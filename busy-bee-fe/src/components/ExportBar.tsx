import { useState } from 'react'
import { Check, Copy, Download, Share2 } from 'lucide-react'

import { downloadFile, safeFilename, shareOrDownload } from '../services/shareFile'

/** 文件匯出工具列：複製 Markdown / 下載 .md / 系統分享（以 .md 檔分享，AirDrop 到電腦即為檔案）。 */
export function ExportBar({ content, filename }: { content: string; filename: string }) {
  const [copied, setCopied] = useState(false)
  const canShare = typeof navigator !== 'undefined' && typeof navigator.share === 'function'

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(content)
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    } catch {
      // 剪貼簿權限被拒時靜默（桌面主流覽器多半允許）
    }
  }

  const toFile = () => {
    const base = safeFilename(filename.replace(/\.md$/i, ''))
    return new File([content], `${base}.md`, { type: 'text/markdown' })
  }

  const download = () => downloadFile(toFile())

  const share = async () => {
    await shareOrDownload(toFile())
  }

  return (
    <div className="flex items-center gap-1">
      <button
        type="button"
        className="btn btn-ghost size-9 px-0"
        aria-label={copied ? '已複製' : '複製 Markdown'}
        onClick={() => void copy()}
      >
        {copied ? (
          <Check className="animate-scale-in size-4 text-success" />
        ) : (
          <Copy className="size-4" />
        )}
      </button>
      <button
        type="button"
        className="btn btn-ghost size-9 px-0"
        aria-label="下載 .md"
        onClick={download}
      >
        <Download className="size-4" />
      </button>
      {canShare && (
        <button
          type="button"
          className="btn btn-ghost size-9 px-0"
          aria-label="分享"
          onClick={() => void share()}
        >
          <Share2 className="size-4" />
        </button>
      )}
    </div>
  )
}
