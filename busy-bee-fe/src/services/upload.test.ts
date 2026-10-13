import { describe, expect, it, vi } from 'vitest'

vi.mock('./api/client', () => ({ createMeeting: vi.fn(), completeUpload: vi.fn() }))

import { createMeeting } from './api/client'
import { uploadAudio } from './upload'

describe('uploadAudio', () => {
  // 稽核 F3：超過 200MB 要在傳之前就擋下，而不是傳完才收到 GCS 400
  it('超過 200MB 直接拒絕，不建立會議', async () => {
    const file = new File(['x'], 'big.wav', { type: 'audio/wav' })
    Object.defineProperty(file, 'size', { value: 200 * 1024 * 1024 + 1 })
    await expect(uploadAudio('tok', 't', file, () => {})).rejects.toThrow('檔案超過 200MB')
    expect(createMeeting).not.toHaveBeenCalled()
  })
})
