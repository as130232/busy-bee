import { describe, expect, it } from 'vitest'

import { displayStatus } from './meetingStatus'

describe('displayStatus', () => {
  it('即時會議已結束但錄音未上傳 → awaitingUpload；其他照原狀態', () => {
    expect(displayStatus({ status: 'scheduled', liveStartedAt: '2026-10-15T06:00:00Z' })).toBe('awaitingUpload')
    expect(displayStatus({ status: 'scheduled' })).toBe('scheduled')
    expect(displayStatus({ status: 'live', liveStartedAt: '2026-10-15T06:00:00Z' })).toBe('live')
    expect(displayStatus({ status: 'pending', liveStartedAt: '2026-10-15T06:00:00Z' })).toBe('pending')
  })
})
