// 以「檔案」分享（AirDrop 等），不支援時退回下載。
// 不用 navigator.share({ text })：逐字稿開頭的「A: …」會被 macOS 當成 URL scheme 而無法開啟。

export type ShareResult = 'shared' | 'downloaded' | 'cancelled'

/** 檔名去除路徑與系統保留字元，避免分享／下載失敗。 */
export function safeFilename(name: string, fallback = 'busy-bee'): string {
  const cleaned = name
    .replace(/[\\/:*?"<>|]/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
    .slice(0, 80)
  return cleaned || fallback
}

export function canShareFile(file: File): boolean {
  return (
    typeof navigator !== 'undefined' &&
    typeof navigator.share === 'function' &&
    typeof navigator.canShare === 'function' &&
    navigator.canShare({ files: [file] })
  )
}

export function downloadFile(file: File): void {
  const url = URL.createObjectURL(file)
  const a = document.createElement('a')
  a.href = url
  a.download = file.name
  a.click()
  // 延後釋放：部分瀏覽器（Safari）在 click 後才開始讀取
  setTimeout(() => URL.revokeObjectURL(url), 30_000)
}

/** 必須在使用者點按當下呼叫（iOS 需要 user activation）。 */
export async function shareOrDownload(file: File): Promise<ShareResult> {
  if (canShareFile(file)) {
    try {
      await navigator.share({ files: [file], title: file.name })
      return 'shared'
    } catch (e) {
      if (e instanceof Error && e.name === 'AbortError') return 'cancelled'
      // 其他錯誤（如失去 user activation）退回下載
    }
  }
  downloadFile(file)
  return 'downloaded'
}

const AUDIO_EXT: Record<string, string> = {
  'audio/mp4': '.m4a',
  'audio/x-m4a': '.m4a',
  'audio/aac': '.m4a',
  'audio/webm': '.webm',
  'audio/mpeg': '.mp3',
  'audio/ogg': '.ogg',
  'audio/wav': '.wav',
  'audio/x-wav': '.wav',
}

/** 依 Content-Type（優先）或網址副檔名決定音檔副檔名。 */
export function audioExtension(contentType: string, url: string): string {
  const ext = AUDIO_EXT[contentType.split(';')[0].trim().toLowerCase()]
  if (ext) return ext
  const m = /\.(m4a|webm|mp3|ogg|wav|mp4)(?:$|\?)/i.exec(url)
  if (!m) return '.m4a'
  const e = m[1].toLowerCase()
  return e === 'mp4' ? '.m4a' : `.${e}`
}
