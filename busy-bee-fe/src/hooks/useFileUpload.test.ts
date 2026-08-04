import { act, renderHook } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

vi.mock('../services/firebase', () => ({
  auth: { currentUser: { getIdToken: vi.fn(async () => 'tok') } },
}))
vi.mock('../services/upload', () => ({ uploadAudio: vi.fn() }))

import { uploadAudio } from '../services/upload'
import { useFileUpload } from './useFileUpload'

const uploadAudioMock = vi.mocked(uploadAudio)

function audioFile(name = 'rec.webm') {
  return new File(['data'], name, { type: 'audio/webm' })
}

afterEach(() => {
  vi.clearAllMocks()
})

describe('useFileUpload', () => {
  it('初始狀態為 idle', () => {
    const { result } = renderHook(() => useFileUpload())
    expect(result.current.state).toEqual({ phase: 'idle' })
  })

  it('成功時轉為 done、帶回 meeting 並呼叫 onUploaded；預設標題去副檔名', async () => {
    const meeting = { id: 'm1', title: 'rec' }
    uploadAudioMock.mockResolvedValueOnce(meeting as never)
    const onUploaded = vi.fn()

    const { result } = renderHook(() => useFileUpload({ scenario: 'meeting', onUploaded }))
    await act(async () => {
      await result.current.upload(audioFile())
    })

    expect(uploadAudioMock).toHaveBeenCalledWith('tok', 'rec', expect.any(File), expect.any(Function), 'meeting')
    expect(result.current.state).toMatchObject({ phase: 'done', meeting })
    expect(onUploaded).toHaveBeenCalledWith(meeting)
  })

  it('titleFor 覆寫標題', async () => {
    uploadAudioMock.mockResolvedValueOnce({ id: 'm2' } as never)
    const { result } = renderHook(() =>
      useFileUpload({ titleFor: (f) => `會議${f.name.replace(/\.[^.]+$/, '')}` }),
    )
    await act(async () => {
      await result.current.upload(audioFile('abc.m4a'))
    })
    expect(uploadAudioMock).toHaveBeenCalledWith('tok', '會議abc', expect.any(File), expect.any(Function), 'meeting')
  })

  it('失敗時轉為 error、保留 file 供重試', async () => {
    uploadAudioMock.mockRejectedValueOnce(new Error('上傳中斷'))
    const file = audioFile()
    const { result } = renderHook(() => useFileUpload())
    await act(async () => {
      await result.current.upload(file)
    })
    expect(result.current.state).toMatchObject({ phase: 'error', message: '上傳中斷', file })
  })

  it('reset 回到 idle', async () => {
    uploadAudioMock.mockResolvedValueOnce({ id: 'm3' } as never)
    const { result } = renderHook(() => useFileUpload())
    await act(async () => {
      await result.current.upload(audioFile())
    })
    act(() => result.current.reset())
    expect(result.current.state).toEqual({ phase: 'idle' })
  })
})
