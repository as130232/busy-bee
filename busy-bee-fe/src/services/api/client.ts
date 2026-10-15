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

let unauthorizedHandler: (() => void) | null = null

/**
 * 註冊「授權失效」處理器：任一請求回 40101（token 過期）或 HTTP 401 時觸發，
 * 由 AuthProvider 注入（登出並導回登入頁），讓 401 處理集中一處而非散落各 catch。
 */
export function setUnauthorizedHandler(handler: (() => void) | null): void {
  unauthorizedHandler = handler
}

/** 一般請求逾時；斷網或伺服器卡住時中止，避免畫面永遠轉圈。 */
export const REQUEST_TIMEOUT_MS = 30_000
/** 同步呼叫 LLM 的請求（跨會議問答）需要較長時間。 */
const LLM_REQUEST_TIMEOUT_MS = 90_000

async function request<T>(
  path: string,
  init: RequestInit,
  idToken?: string,
  timeoutMs = REQUEST_TIMEOUT_MS,
): Promise<T> {
  const headers = new Headers(init.headers)
  if (idToken) headers.set('Authorization', `Bearer ${idToken}`)

  // 逾時只代表前端放棄等待，伺服器端的副作用不一定已回滾。
  const ctrl = new AbortController()
  const timer = setTimeout(() => ctrl.abort(), timeoutMs)
  let res: Response
  let raw: string
  try {
    res = await fetch(path, { ...init, headers, signal: ctrl.signal })
    raw = await res.text()
  } catch (e) {
    if (ctrl.signal.aborted) throw new ApiError(0, '連線逾時，請檢查網路後重試', 0)
    if (e instanceof TypeError) throw new ApiError(0, '網路連線失敗，請檢查網路後重試', 0)
    throw e
  } finally {
    clearTimeout(timer)
  }

  // 容忍空 body / 非 JSON 回應（如 proxy 502、後端崩潰）：不硬解析，改回可讀的狀態訊息，
  // 避免掩蓋真實錯誤（例：502 空 body 會讓 res.json() 拋「Unexpected end of JSON input」）。
  let body: Envelope
  try {
    body = raw ? (JSON.parse(raw) as Envelope) : ({ errCode: res.status, msg: '' } as Envelope)
  } catch {
    throw new ApiError(res.status, `伺服器回應異常（HTTP ${res.status}）`, res.status)
  }

  if (!res.ok || body.errCode !== 0) {
    const code = body.errCode ?? res.status
    // 授權失效統一在此攔截，通知已註冊的處理器（登出導回登入）。
    if (code === 40101 || res.status === 401) unauthorizedHandler?.()
    const msg = body.msg || `伺服器錯誤（HTTP ${res.status}）`
    throw new ApiError(code, friendlyMessage(code, msg), res.status, body.traceId)
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
    case 42902:
      return '本月即時辨識額度已用完，錄音會照常進行。'
    case 40906:
      return '這場會議已不在即時模式中。'
    case 41301:
      return '資料太大（單檔上限 10MB，單場合計 10 萬字）。'
    case 41501:
      return '只支援 .txt、.md、.pdf 檔案。'
    case 42201:
      return '檔案內容讀取失敗，請改用貼上文字。'
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
  idea: '想法',
}

/** STT 辨識語言（中文 / 英文 / 自動偵測）。 */
export type Language = Meeting['language']

/** 語言顯示標籤（前端一律以此對應中文標籤，避免各處硬編）。 */
export const languageLabels: Record<Language, string> = {
  'zh-TW': '中文',
  'en-US': '英文',
  auto: '自動偵測',
}

export interface CreateMeetingResult {
  meeting: Meeting
  upload: { url: string; headers: Record<string, string> }
}

/** 建立會議並取得 GCS 直傳 signed URL */
export function createMeeting(
  idToken: string,
  input: { title: string; contentType: string; scenario?: Scenario; language?: Language },
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

/** 貼連結匯入（YouTube / Podcast / 直接音檔）：音訊由後端抓取後跑管線；標題留空自動用來源標題 */
export function importMeeting(
  idToken: string,
  input: { url: string; title?: string; scenario?: Scenario; language?: Language },
): Promise<{ meeting: Meeting }> {
  return request<{ meeting: Meeting }>(
    '/api/v1/meetings/import',
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
export type CorrectionState = components['schemas']['CorrectionState']

/** 取得最新一次 AI 校正狀態（未校正過時 status 為空字串）。 */
export function getCorrection(idToken: string, meetingId: string): Promise<{ correction: CorrectionState }> {
  return request<{ correction: CorrectionState }>(`/api/v1/meetings/${meetingId}/correction`, {}, idToken)
}

/** 建立 AI 錄音重新校正（背景處理；每場最多 2 次，失敗也計次）。 */
export function startCorrection(idToken: string, meetingId: string): Promise<{ correction: CorrectionState }> {
  return request<{ correction: CorrectionState }>(`/api/v1/meetings/${meetingId}/correction`, { method: 'POST' }, idToken)
}

/** 整份還原最新一次已完成的 AI 校正（行動項不動）。 */
export function revertCorrection(idToken: string, meetingId: string): Promise<{ correction: CorrectionState }> {
  return request<{ correction: CorrectionState }>(`/api/v1/meetings/${meetingId}/correction/revert`, { method: 'POST' }, idToken)
}

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

/** 刪除單筆待辦 */
export function deleteActionItem(idToken: string, id: string): Promise<void> {
  return request<void>(`/api/v1/action-items/${id}`, { method: 'DELETE' }, idToken)
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
  input: {
    title: string
    scheduledAt: string
    remindBeforeMin?: number
    scenario?: Scenario
    language?: Language
  },
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

/** 覆寫會議手動標籤（後端去空白/去重，上限 20），回傳最新詳情 */
export function updateMeetingTags(
  idToken: string,
  meetingId: string,
  tags: string[],
): Promise<{ meeting: MeetingDetail }> {
  return request<{ meeting: MeetingDetail }>(
    `/api/v1/meetings/${meetingId}/tags`,
    {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ tags }),
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
export function deleteMeeting(idToken: string, meetingId: string): Promise<void> {
  return request<void>(`/api/v1/meetings/${meetingId}`, { method: 'DELETE' }, idToken)
}

/** 取得 Web Push VAPID 公鑰 */
export function getVapidPublicKey(idToken: string): Promise<{ publicKey: string }> {
  return request<{ publicKey: string }>('/api/v1/push/vapid-public-key', { method: 'GET' }, idToken)
}

/** 註冊推播訂閱 */
export function subscribePush(idToken: string, sub: PushSubscriptionJSON): Promise<void> {
  return request<void>(
    '/api/v1/push/subscriptions',
    { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(sub) },
    idToken,
  )
}

/** 取消推播訂閱 */
export function unsubscribePush(idToken: string, endpoint: string): Promise<void> {
  return request<void>(
    '/api/v1/push/subscriptions',
    { method: 'DELETE', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ endpoint }) },
    idToken,
  )
}

/** debug / demo：對自己所有訂閱立即送測試推播，驗證顯示層（免等排程） */
export function sendTestPush(idToken: string): Promise<{ delivered: number }> {
  return request<{ delivered: number }>('/api/v1/push/test', { method: 'POST' }, idToken)
}

export interface QASource {
  index: number
  meetingId: string
  title: string
  snippet: string
}

export interface QAResult {
  /** markdown 答案，內含 [n] 引用標註，對應 sources[].index */
  answer: string
  /** true 表示無相關內容（後端此時未呼叫 LLM） */
  noMatch: boolean
  sources: QASource[]
}

/** 跨會議 RAG 問答（單次、無狀態）：依問題語意檢索本人全部會議並生成帶引用的答案 */
export function askCrossMeetingQA(idToken: string, question: string): Promise<QAResult> {
  return request<QAResult>(
    '/api/v1/meetings/qa',
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ question }),
    },
    idToken,
    LLM_REQUEST_TIMEOUT_MS,
  )
}

// ---- 即時會議（F-LIVE）----

export type LiveSummary = components['schemas']['LiveSummary']
export type LiveSource = components['schemas']['LiveSource']
export type LiveQA = components['schemas']['LiveQA']
export type LiveNotes = components['schemas']['LiveNotes']
export type LiveUsage = components['schemas']['LiveUsage']
export type MeetingContext = components['schemas']['MeetingContext']

export interface LiveToken {
  accessToken: string
  expiresIn: number
  model: string
  language: string
  keyterms: string[]
  usage: LiveUsage
}

function postJSON<T>(path: string, idToken: string, body: unknown, timeoutMs?: number): Promise<T> {
  return request<T>(
    path,
    { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) },
    idToken,
    timeoutMs,
  )
}

/** 開始即時會議（或從排程開始）；contentType 為錄音格式（audio/webm 或 audio/mp4）。 */
export function startLiveMeeting(
  idToken: string,
  input: {
    title: string
    scenario: Scenario
    language: Language
    contentType: string
    scheduledMeetingId?: string
  },
): Promise<{ meeting: Meeting }> {
  return postJSON('/api/v1/meetings/live', idToken, input)
}

/** Deepgram 串流短效金鑰（30 秒內需用來建立連線）與串流參數。 */
export function getLiveToken(idToken: string, meetingId: string): Promise<LiveToken> {
  return postJSON(`/api/v1/meetings/${meetingId}/live/token`, idToken, {})
}

export function sendLiveHeartbeat(
  idToken: string,
  meetingId: string,
  seconds: number,
): Promise<{ usage: LiveUsage }> {
  return postJSON(`/api/v1/meetings/${meetingId}/live/heartbeat`, idToken, { seconds })
}

export function updateLiveNotes(
  idToken: string,
  meetingId: string,
  notes: LiveSummary,
  newTranscript: string,
): Promise<{ notes: LiveSummary }> {
  return postJSON(
    `/api/v1/meetings/${meetingId}/live/notes`,
    idToken,
    { notes, newTranscript },
    LLM_REQUEST_TIMEOUT_MS,
  )
}

export function askLive(
  idToken: string,
  meetingId: string,
  input: { question: string; transcript: string; history: LiveQA[] },
): Promise<{ answer: string; sources: LiveSource[] }> {
  return postJSON(`/api/v1/meetings/${meetingId}/live/ask`, idToken, input, LLM_REQUEST_TIMEOUT_MS)
}

/** 結束即時會議：存會議中筆記並取得錄音直傳 URL（之後呼叫 completeUpload）。冪等。 */
export function finishLive(
  idToken: string,
  meetingId: string,
  notes: LiveNotes,
): Promise<{ meeting: Meeting; upload: { url: string; headers: Record<string, string> } }> {
  return postJSON(`/api/v1/meetings/${meetingId}/live/finish`, idToken, notes)
}

export function listMeetingContexts(
  idToken: string,
  meetingId: string,
): Promise<{ contexts: MeetingContext[] }> {
  return request(`/api/v1/meetings/${meetingId}/contexts`, { method: 'GET' }, idToken)
}

/** 新增會前資料：貼上文字。 */
export function addMeetingContextText(
  idToken: string,
  meetingId: string,
  input: { title?: string; text: string },
): Promise<{ context: MeetingContext }> {
  return postJSON(`/api/v1/meetings/${meetingId}/contexts`, idToken, input, LLM_REQUEST_TIMEOUT_MS)
}

/** 新增會前資料：上傳 .txt / .md / .pdf（PDF 由 AI 轉文字，可能需數十秒）。 */
export function addMeetingContextFile(
  idToken: string,
  meetingId: string,
  file: File,
): Promise<{ context: MeetingContext }> {
  const form = new FormData()
  form.append('file', file)
  return request(
    `/api/v1/meetings/${meetingId}/contexts`,
    { method: 'POST', body: form },
    idToken,
    LLM_REQUEST_TIMEOUT_MS,
  )
}

export function deleteMeetingContext(
  idToken: string,
  meetingId: string,
  contextId: string,
): Promise<void> {
  return request(`/api/v1/meetings/${meetingId}/contexts/${contextId}`, { method: 'DELETE' }, idToken)
}

export function updatePreferences(
  idToken: string,
  input: { liveMicTranscription: boolean },
): Promise<{ liveMicTranscription: boolean }> {
  return request(
    '/api/v1/users/me/preferences',
    { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(input) },
    idToken,
  )
}
