import 'fake-indexeddb/auto'
import { beforeEach, describe, expect, it } from 'vitest'

import { RecordingStore } from './recordingStore'

let n = 0
let store: RecordingStore
beforeEach(() => {
  store = new RecordingStore(`test-${++n}`)
})

const meta = { meetingId: 'm1', title: '週會', startedAt: 1000, mimeType: 'audio/webm' }

describe('RecordingStore', () => {
  it('依序存 chunk、組回完整錄音', async () => {
    await store.begin(meta)
    await store.appendChunk('m1', new Blob(['ab']))
    await store.appendChunk('m1', new Blob(['cd']))
    const rec = await store.getRecording('m1')
    expect(rec?.type).toBe('audio/webm')
    expect(await rec?.text()).toBe('abcd')
  })

  it('列出未完成場次（含最新筆記快照），刪除後清空', async () => {
    await store.begin(meta)
    await store.saveNotes('m1', { notes: { current: 'c', points: ['p'] }, qa: [] })
    await store.appendChunk('m1', new Blob(['x']))
    const list = await store.list()
    expect(list).toHaveLength(1)
    expect(list[0]).toMatchObject({ meetingId: 'm1', title: '週會', notes: { notes: { current: 'c' } } })
    await store.remove('m1')
    expect(await store.list()).toEqual([])
    expect(await store.getRecording('m1')).toBeNull()
  })

  it('不同場次互不影響', async () => {
    await store.begin(meta)
    await store.begin({ ...meta, meetingId: 'm2' })
    await store.appendChunk('m1', new Blob(['1']))
    await store.appendChunk('m2', new Blob(['2']))
    await store.remove('m1')
    expect(await (await store.getRecording('m2'))?.text()).toBe('2')
  })
})
