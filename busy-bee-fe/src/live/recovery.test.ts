import { describe, expect, it, vi } from 'vitest'

import { discardRecovered, uploadRecovered } from './recovery'

function deps(recording: Blob | null) {
  return {
    getIdToken: async () => 'id',
    store: {
      getRecording: vi.fn(async () => recording),
      remove: vi.fn(async () => {}),
    },
    api: {
      finishLive: vi.fn(async () => ({ meeting: { id: 'm1' }, upload: { url: 'https://u', headers: { h: '1' } } })),
      completeUpload: vi.fn(async () => ({ meeting: { id: 'm1' } })),
      deleteMeeting: vi.fn(async () => {}),
    },
    upload: vi.fn(async () => {}),
  }
}

const session = { meetingId: 'm1', title: '週會', startedAt: 0, mimeType: 'audio/webm', notes: { notes: { current: 'c', points: [] }, qa: [] } }

describe('uploadRecovered', () => {
  it('以保存的筆記結束、上傳本機錄音並清除備份', async () => {
    const blob = new Blob(['abc'], { type: 'audio/webm' })
    const d = deps(blob)
    await uploadRecovered(d as never, session, () => {})
    expect(d.api.finishLive).toHaveBeenCalledWith('id', 'm1', session.notes)
    expect(d.upload).toHaveBeenCalledWith('https://u', { h: '1' }, blob, expect.any(Function))
    expect(d.api.completeUpload).toHaveBeenCalledWith('id', 'm1')
    expect(d.store.remove).toHaveBeenCalledWith('m1')
  })

  it('沒有筆記快照時送空筆記', async () => {
    const d = deps(new Blob(['x']))
    await uploadRecovered(d as never, { ...session, notes: undefined }, () => {})
    expect(d.api.finishLive).toHaveBeenCalledWith('id', 'm1', { notes: { current: '', points: [] }, qa: [] })
  })

  it('本機沒有錄音資料 → 丟錯，不呼叫後端', async () => {
    const d = deps(null)
    await expect(uploadRecovered(d as never, session, () => {})).rejects.toThrow()
    expect(d.api.finishLive).not.toHaveBeenCalled()
  })
})

describe('discardRecovered', () => {
  it('刪除紀錄與本機備份；紀錄已不存在也照樣清本機', async () => {
    const d = deps(null)
    d.api.deleteMeeting.mockRejectedValueOnce(Object.assign(new Error('not found'), { status: 404 }))
    await discardRecovered(d as never, 'm1')
    expect(d.store.remove).toHaveBeenCalledWith('m1')
  })
})
