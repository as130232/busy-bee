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
    throw new ApiError(body.errCode, body.msg, res.status, body.traceId)
  }
  return body.data as T
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
