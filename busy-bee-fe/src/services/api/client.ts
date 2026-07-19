// 手寫薄 client：型別來自 gen:api 產生的 schema.d.ts，統一解析 envelope 與錯誤。
import type { components } from './schema'

export type User = components['schemas']['User']
type Envelope = components['schemas']['Envelope'] & { data?: unknown }

export class ApiError extends Error {
  readonly errCode: number
  readonly status: number
  readonly traceId?: string

  constructor(errCode: number, message: string, status: number, traceId?: string) {
    super(message)
    this.name = 'ApiError'
    this.errCode = errCode
    this.status = status
    this.traceId = traceId
  }
}

async function request<T>(path: string, init: RequestInit, idToken?: string): Promise<T> {
  const headers = new Headers(init.headers)
  if (idToken) headers.set('Authorization', `Bearer ${idToken}`)

  const res = await fetch(path, { ...init, headers })
  const body = (await res.json()) as Envelope

  if (!res.ok || body.errCode !== 0) {
    throw new ApiError(body.errCode, friendlyMessage(body.errCode, body.msg), res.status, body.traceId)
  }
  return body.data as T
}

/** 常見錯誤碼轉為用戶看得懂的訊息；其餘沿用後端 msg。 */
function friendlyMessage(errCode: number, msg: string): string {
  switch (errCode) {
    case 40101:
      return '登入已過期，請重新登入。'
    case 40301:
      return '此帳號沒有使用權限。'
    case 42901:
      return '操作太頻繁，請稍後再試。'
    case 50001:
      return '系統忙碌中，請稍後再試。'
    default:
      return msg
  }
}

/** 登入後同步用戶資料（upsert by firebase_uid） */
export function syncUser(idToken: string): Promise<User> {
  return request<User>('/api/v1/users/sync', { method: 'POST' }, idToken)
}

export type Meeting = components['schemas']['Meeting']

export interface CreateMeetingResult {
  meeting: Meeting
  upload: { url: string; headers: Record<string, string> }
}

/** 建立會議並取得 GCS 直傳 signed URL */
export function createMeeting(
  idToken: string,
  input: { title: string; contentType: string },
): Promise<CreateMeetingResult> {
  return request<CreateMeetingResult>(
    '/api/v1/meetings',
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(input),
    },
    idToken,
  )
}

/** 音訊直傳完成後觸發背景處理 */
export function completeUpload(idToken: string, meetingId: string): Promise<{ meeting: Meeting }> {
  return request<{ meeting: Meeting }>(
    `/api/v1/meetings/${meetingId}/complete-upload`,
    { method: 'POST' },
    idToken,
  )
}

export type MeetingDetail = components['schemas']['MeetingDetail']
export type Artifact = components['schemas']['Artifact']

/** 本人會議列表（可帶關鍵字搜尋） */
export function listMeetings(idToken: string, search = ''): Promise<{ meetings: Meeting[] }> {
  const q = search ? `?search=${encodeURIComponent(search)}` : ''
  return request<{ meetings: Meeting[] }>(`/api/v1/meetings${q}`, { method: 'GET' }, idToken)
}

/** 會議詳情（含逐字稿） */
export function getMeeting(idToken: string, meetingId: string): Promise<{ meeting: MeetingDetail }> {
  return request<{ meeting: MeetingDetail }>(`/api/v1/meetings/${meetingId}`, { method: 'GET' }, idToken)
}

/** 會議的 AI 生成文件 */
export function listArtifacts(idToken: string, meetingId: string): Promise<{ artifacts: Artifact[] }> {
  return request<{ artifacts: Artifact[] }>(
    `/api/v1/meetings/${meetingId}/artifacts`,
    { method: 'GET' },
    idToken,
  )
}

/** 重跑失敗的會議 */
export function retryMeeting(idToken: string, meetingId: string): Promise<{ meeting: Meeting }> {
  return request<{ meeting: Meeting }>(`/api/v1/meetings/${meetingId}/retry`, { method: 'POST' }, idToken)
}
