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

/** 紀錄情境（會議 / 閒聊 / 面試）；決定 AI 產出的結構化摘要區塊模板。 */
export type Scenario = Meeting['scenario']

/** 情境顯示標籤（前端一律以此對應中文標籤，避免各處硬編）。 */
export const scenarioLabels: Record<Scenario, string> = {
  meeting: '會議',
  casual: '閒聊',
  interview: '面試',
}

export interface CreateMeetingResult {
  meeting: Meeting
  upload: { url: string; headers: Record<string, string> }
}

/** 建立會議並取得 GCS 直傳 signed URL */
export function createMeeting(
  idToken: string,
  input: { title: string; contentType: string; scenario?: Scenario },
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

/** 取得會議音檔的限時播放 URL */
export function getMeetingAudioURL(idToken: string, meetingId: string): Promise<{ url: string }> {
  return request<{ url: string }>(`/api/v1/meetings/${meetingId}/audio-url`, { method: 'GET' }, idToken)
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

export type ActionItem = components['schemas']['ActionItem']
export type PendingActionItem = components['schemas']['PendingActionItem']

/** 取回某會議的行動項 */
export function listMeetingActionItems(
  idToken: string,
  meetingId: string,
): Promise<{ actionItems: ActionItem[] }> {
  return request<{ actionItems: ActionItem[] }>(
    `/api/v1/meetings/${meetingId}/action-items`,
    { method: 'GET' },
    idToken,
  )
}

/** 手動新增待辦（source=manual，重跑分析不刪；assignee 可指派講者代號） */
export function addMeetingActionItem(
  idToken: string,
  meetingId: string,
  description: string,
  assignee = '',
): Promise<{ actionItem: ActionItem }> {
  return request<{ actionItem: ActionItem }>(
    `/api/v1/meetings/${meetingId}/action-items`,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ description, assignee }),
    },
    idToken,
  )
}

/** 跨會議的未完成行動項 */
export function listPendingActionItems(
  idToken: string,
): Promise<{ actionItems: PendingActionItem[] }> {
  return request<{ actionItems: PendingActionItem[] }>('/api/v1/action-items', { method: 'GET' }, idToken)
}

/** 標記行動項完成 / 取消完成 */
export function toggleActionItem(
  idToken: string,
  id: string,
  done: boolean,
): Promise<{ actionItem: ActionItem }> {
  return request<{ actionItem: ActionItem }>(
    `/api/v1/action-items/${id}`,
    {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ done }),
    },
    idToken,
  )
}

/** 修改待辦內容 */
export function editActionItem(
  idToken: string,
  id: string,
  description: string,
): Promise<{ actionItem: ActionItem }> {
  return request<{ actionItem: ActionItem }>(
    `/api/v1/action-items/${id}`,
    {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ description }),
    },
    idToken,
  )
}

/** 建立排程會議（提醒用） */
export function createScheduledMeeting(
  idToken: string,
  input: { title: string; scheduledAt: string; remindBeforeMin?: number; scenario?: Scenario },
): Promise<{ meeting: Meeting }> {
  return request<{ meeting: Meeting }>(
    '/api/v1/meetings/scheduled',
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(input),
    },
    idToken,
  )
}

/** 修改排程會議（時間/標題/提前分鐘；會重置提醒） */
export function updateMeetingSchedule(
  idToken: string,
  meetingId: string,
  input: { title: string; scheduledAt: string; remindBeforeMin?: number },
): Promise<{ meeting: Meeting }> {
  return request<{ meeting: Meeting }>(
    `/api/v1/meetings/${meetingId}/schedule`,
    {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(input),
    },
    idToken,
  )
}

/** 重新命名會議（任何狀態） */
export function renameMeeting(
  idToken: string,
  meetingId: string,
  title: string,
): Promise<{ meeting: Meeting }> {
  return request<{ meeting: Meeting }>(
    `/api/v1/meetings/${meetingId}`,
    {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title }),
    },
    idToken,
  )
}

export type TranscriptSegment = components['schemas']['TranscriptSegment']

/** 修正單一逐字稿片段文字（校正 STT 錯字），回傳含最新逐字稿的詳情 */
export function editMeetingSegment(
  idToken: string,
  meetingId: string,
  index: number,
  text: string,
): Promise<{ meeting: MeetingDetail }> {
  return request<{ meeting: MeetingDetail }>(
    `/api/v1/meetings/${meetingId}/transcript`,
    {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ index, text }),
    },
    idToken,
  )
}

/** 更新講者代號→顯示名（如 {"A":"Ben"}），回傳含最新逐字稿的詳情 */
export function updateMeetingSpeakers(
  idToken: string,
  meetingId: string,
  speakerNames: Record<string, string>,
): Promise<{ meeting: MeetingDetail }> {
  return request<{ meeting: MeetingDetail }>(
    `/api/v1/meetings/${meetingId}/speakers`,
    {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ speakerNames }),
    },
    idToken,
  )
}

/** 刪除會議（任何狀態，本人限定；關聯資料連帶刪除） */
export function deleteMeeting(idToken: string, meetingId: string): Promise<unknown> {
  return request<unknown>(`/api/v1/meetings/${meetingId}`, { method: 'DELETE' }, idToken)
}

/** 取得 Web Push VAPID 公鑰 */
export function getVapidPublicKey(idToken: string): Promise<{ publicKey: string }> {
  return request<{ publicKey: string }>('/api/v1/push/vapid-public-key', { method: 'GET' }, idToken)
}

/** 註冊推播訂閱 */
export function subscribePush(idToken: string, sub: PushSubscriptionJSON): Promise<unknown> {
  return request(
    '/api/v1/push/subscriptions',
    { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(sub) },
    idToken,
  )
}

/** 取消推播訂閱 */
export function unsubscribePush(idToken: string, endpoint: string): Promise<unknown> {
  return request(
    '/api/v1/push/subscriptions',
    { method: 'DELETE', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ endpoint }) },
    idToken,
  )
}
