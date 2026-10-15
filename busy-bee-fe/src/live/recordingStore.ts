// 即時會議錄音本機備份（F-LIVE）：錄音 chunk 每 10 秒寫入 IndexedDB，
// busy-bee 分頁意外關閉後重新開啟可救回並上傳；上傳完成即刪除。
import type { LiveNotes } from '../services/api/client'

export interface SessionMeta {
  meetingId: string
  title: string
  startedAt: number
  mimeType: string
}

export interface StoredSession extends SessionMeta {
  notes?: LiveNotes
}

const SESSIONS = 'sessions'
const CHUNKS = 'chunks'

function promisify<T>(req: IDBRequest<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
}

function done(tx: IDBTransaction): Promise<void> {
  return new Promise((resolve, reject) => {
    tx.oncomplete = () => resolve()
    tx.onerror = () => reject(tx.error)
    tx.onabort = () => reject(tx.error)
  })
}

export class RecordingStore {
  private dbPromise: Promise<IDBDatabase> | null = null
  private readonly name: string

  constructor(name = 'busybee-live') {
    this.name = name
  }

  private db(): Promise<IDBDatabase> {
    this.dbPromise ??= new Promise((resolve, reject) => {
      const req = indexedDB.open(this.name, 1)
      req.onupgradeneeded = () => {
        const db = req.result
        db.createObjectStore(SESSIONS, { keyPath: 'meetingId' })
        const chunks = db.createObjectStore(CHUNKS, { autoIncrement: true })
        chunks.createIndex('meetingId', 'meetingId')
      }
      req.onsuccess = () => resolve(req.result)
      req.onerror = () => reject(req.error)
    })
    return this.dbPromise
  }

  async begin(meta: SessionMeta): Promise<void> {
    const db = await this.db()
    const tx = db.transaction(SESSIONS, 'readwrite')
    tx.objectStore(SESSIONS).put(meta)
    await done(tx)
  }

  async saveNotes(meetingId: string, notes: LiveNotes): Promise<void> {
    const db = await this.db()
    const tx = db.transaction(SESSIONS, 'readwrite')
    const s = tx.objectStore(SESSIONS)
    const cur = (await promisify(s.get(meetingId))) as StoredSession | undefined
    if (cur) s.put({ ...cur, notes })
    await done(tx)
  }

  /** 以 ArrayBuffer 儲存（各瀏覽器 IndexedDB 存 Blob 的支援度不一）。 */
  async appendChunk(meetingId: string, blob: Blob): Promise<void> {
    const data = await blob.arrayBuffer()
    const db = await this.db()
    const tx = db.transaction(CHUNKS, 'readwrite')
    tx.objectStore(CHUNKS).add({ meetingId, data })
    await done(tx)
  }

  async list(): Promise<StoredSession[]> {
    const db = await this.db()
    return promisify(db.transaction(SESSIONS).objectStore(SESSIONS).getAll()) as Promise<StoredSession[]>
  }

  /** 依寫入順序組回錄音；沒有資料回 null。 */
  async getRecording(meetingId: string): Promise<Blob | null> {
    const db = await this.db()
    const tx = db.transaction([SESSIONS, CHUNKS])
    const meta = (await promisify(tx.objectStore(SESSIONS).get(meetingId))) as StoredSession | undefined
    const rows = (await promisify(
      tx.objectStore(CHUNKS).index('meetingId').getAll(meetingId),
    )) as { data: ArrayBuffer }[]
    if (!meta || rows.length === 0) return null
    return new Blob(
      rows.map((r) => r.data),
      { type: meta.mimeType },
    )
  }

  async remove(meetingId: string): Promise<void> {
    const db = await this.db()
    const tx = db.transaction([SESSIONS, CHUNKS], 'readwrite')
    tx.objectStore(SESSIONS).delete(meetingId)
    const idx = tx.objectStore(CHUNKS).index('meetingId')
    const keys = await promisify(idx.getAllKeys(meetingId))
    for (const k of keys) tx.objectStore(CHUNKS).delete(k)
    await done(tx)
  }
}
