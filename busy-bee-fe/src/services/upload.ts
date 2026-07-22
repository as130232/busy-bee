// 三段式直傳：建立會議 → PUT 音訊到 GCS（XHR，含進度）→ complete-upload
import { completeUpload, createMeeting, type Meeting, type Scenario } from './api/client'

/** 支援的音訊 MIME type；瀏覽器偵測不到時以副檔名 fallback */
const extContentType: Record<string, string> = {
  mp3: 'audio/mpeg',
  m4a: 'audio/x-m4a',
  webm: 'audio/webm',
  wav: 'audio/wav',
}

const supportedTypes = new Set([
  'audio/mpeg',
  'audio/mp4',
  'audio/x-m4a',
  'audio/webm',
  'audio/wav',
])

export function resolveContentType(file: File): string | null {
  // 錄音 blob 可能帶 codec 參數（audio/webm;codecs=opus），取分號前主類型
  const base = file.type.split(';')[0].trim()
  if (supportedTypes.has(base)) return base
  const ext = file.name.split('.').pop()?.toLowerCase() ?? ''
  return extContentType[ext] ?? null
}

function putWithProgress(
  url: string,
  headers: Record<string, string>,
  file: Blob,
  onProgress: (percent: number) => void,
): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('PUT', url)
    for (const [k, v] of Object.entries(headers)) xhr.setRequestHeader(k, v)
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) onProgress(Math.round((e.loaded / e.total) * 100))
    }
    xhr.onload = () =>
      xhr.status === 200 ? resolve() : reject(new Error(`上傳失敗（HTTP ${xhr.status}）`))
    xhr.onerror = () => reject(new Error('上傳失敗，請檢查網路後重試'))
    xhr.send(file)
  })
}

export async function uploadAudio(
  idToken: string,
  title: string,
  file: File,
  onProgress: (percent: number) => void,
  scenario: Scenario = 'meeting',
): Promise<Meeting> {
  const contentType = resolveContentType(file)
  if (!contentType) throw new Error('不支援的音訊格式（支援 mp3 / m4a / webm / wav）')

  const { meeting, upload } = await createMeeting(idToken, { title, contentType, scenario })
  await putWithProgress(upload.url, upload.headers, file, onProgress)
  const result = await completeUpload(idToken, meeting.id)
  return result.meeting
}
