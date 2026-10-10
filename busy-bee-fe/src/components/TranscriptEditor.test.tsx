import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import type { MeetingDetail, TranscriptSegment } from '../services/api/client'
import { TranscriptEditor } from './TranscriptEditor'

vi.mock('../services/token', () => ({ getIdToken: vi.fn(async () => 'tok') }))
vi.mock('../services/api/client', () => ({ editMeetingSegment: vi.fn(), updateMeetingSpeakers: vi.fn() }))

const meetingWith = (segs: TranscriptSegment[]): MeetingDetail =>
  ({ id: 'm1', speakerNames: {}, transcriptSegments: segs }) as unknown as MeetingDetail

describe('TranscriptEditor AI 校正標籤', () => {
  it('有 originalText 的段落顯示 AI 已校正，點擊顯示原文', async () => {
    render(
      <TranscriptEditor
        meeting={meetingWith([
          { speaker: 'A', text: '小王處理', originalText: '小娃處理', startMs: 0, endMs: 1000, manuallyEdited: false },
        ])}
        onUpdated={() => {}}
      />,
    )
    await userEvent.click(screen.getByRole('button', { name: 'AI 已校正' }))
    expect(screen.getByText('小娃處理')).toBeInTheDocument()
  })

  it('沒有 originalText 的段落不顯示標籤', () => {
    render(
      <TranscriptEditor
        meeting={meetingWith([{ speaker: 'A', text: '原文', startMs: 0, endMs: 1000, manuallyEdited: false }])}
        onUpdated={() => {}}
      />,
    )
    expect(screen.queryByRole('button', { name: 'AI 已校正' })).not.toBeInTheDocument()
  })
})
