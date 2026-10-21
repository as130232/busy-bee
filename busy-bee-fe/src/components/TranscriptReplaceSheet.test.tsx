import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { replaceMeetingText, type MeetingDetail } from '../services/api/client'
import { TranscriptReplaceSheet } from './TranscriptReplaceSheet'

vi.mock('../services/token', () => ({ getIdToken: vi.fn(async () => 'tok') }))
vi.mock('../services/api/client', () => ({ replaceMeetingText: vi.fn() }))

const replace = vi.mocked(replaceMeetingText)
const meeting = { id: 'm1' } as MeetingDetail

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true })
  replace.mockReset()
})
afterEach(() => vi.useRealTimers())

async function fill(from: string, to: string) {
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
  await user.type(screen.getByLabelText('尋找'), from)
  await user.type(screen.getByLabelText('取代為'), to)
  await act(() => vi.advanceTimersByTimeAsync(500))
  return user
}

describe('TranscriptReplaceSheet', () => {
  it('輸入後預覽符合筆數，確認後全部取代並回傳結果', async () => {
    replace.mockImplementation(async (_t, _id, input) => ({
      meeting: { id: 'm1', summary: input.dryRun ? '舊' : '新' } as MeetingDetail,
      counts: { transcript: 3, summary: 1, actionItems: 1 },
    }))
    const onDone = vi.fn()
    render(<TranscriptReplaceSheet meeting={meeting} onClose={() => {}} onDone={onDone} />)

    const user = await fill('DBA', 'DVA')
    expect(replace).toHaveBeenLastCalledWith('tok', 'm1', { from: 'DBA', to: 'DVA', includeSummary: true, dryRun: true })
    expect(screen.getByText('逐字稿 3 處、摘要 1 處、行動項 1 處')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '全部取代（5 處）' }))
    expect(replace).toHaveBeenLastCalledWith('tok', 'm1', { from: 'DBA', to: 'DVA', includeSummary: true })
    expect(onDone).toHaveBeenCalledWith(expect.objectContaining({ summary: '新' }))
  })

  it('取消勾選摘要時只預覽逐字稿', async () => {
    replace.mockResolvedValue({ meeting, counts: { transcript: 2, summary: 0, actionItems: 0 } })
    render(<TranscriptReplaceSheet meeting={meeting} onClose={() => {}} onDone={() => {}} />)

    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    await user.click(screen.getByLabelText('同時取代摘要與行動項'))
    await fill('DBA', 'DVA')
    expect(replace).toHaveBeenLastCalledWith('tok', 'm1', { from: 'DBA', to: 'DVA', includeSummary: false, dryRun: true })
    expect(screen.getByText('逐字稿 2 處')).toBeInTheDocument()
  })

  it('沒有符合時顯示提示且不能送出', async () => {
    replace.mockResolvedValue({ meeting, counts: { transcript: 0, summary: 0, actionItems: 0 } })
    render(<TranscriptReplaceSheet meeting={meeting} onClose={() => {}} onDone={() => {}} />)
    await fill('XYZ', 'ABC')
    expect(screen.getByText('找不到符合的文字')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /全部取代/ })).toBeDisabled()
  })

  it('兩欄相同時不預覽', async () => {
    render(<TranscriptReplaceSheet meeting={meeting} onClose={() => {}} onDone={() => {}} />)
    await fill('DBA', 'DBA')
    expect(replace).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: /全部取代/ })).toBeDisabled()
  })
})
