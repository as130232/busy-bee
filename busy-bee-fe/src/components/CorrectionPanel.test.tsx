import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { CorrectionState, MeetingDetail } from '../services/api/client'
import { getCorrection, revertCorrection, startCorrection } from '../services/api/client'
import { CorrectionPanel } from './CorrectionPanel'

vi.mock('../services/token', () => ({ getIdToken: vi.fn(async () => 'tok') }))
vi.mock('../services/api/client', () => ({
  getCorrection: vi.fn(),
  startCorrection: vi.fn(),
  revertCorrection: vi.fn(),
}))

const getMock = vi.mocked(getCorrection)
const startMock = vi.mocked(startCorrection)
const revertMock = vi.mocked(revertCorrection)

const state = (s: Partial<CorrectionState> = {}): CorrectionState => ({
  status: '', attemptsUsed: 0, attemptsMax: 2, canRevert: false, errorMessage: '', ...s,
})

const meeting = (uncertainRatio = 0): MeetingDetail =>
  ({ id: 'm1', status: 'completed', uncertainRatio }) as MeetingDetail

beforeEach(() => {
  vi.clearAllMocks()
})

describe('CorrectionPanel', () => {
  it('低信心 ≥5% 顯示建議提示', async () => {
    getMock.mockResolvedValue({ correction: state() })
    render(<CorrectionPanel meeting={meeting(0.08)} onChanged={() => {}} />)
    expect(await screen.findByText('本紀錄約 8% 內容語音辨識不確定，建議校正。')).toBeInTheDocument()
  })

  it('低於門檻不顯示提示但仍有按鈕', async () => {
    getMock.mockResolvedValue({ correction: state() })
    render(<CorrectionPanel meeting={meeting(0.02)} onChanged={() => {}} />)
    expect(await screen.findByRole('button', { name: 'AI 校正' })).toBeEnabled()
    expect(screen.queryByText(/建議校正/)).not.toBeInTheDocument()
  })

  it('已用 2 次時按鈕停用並顯示 2/2', async () => {
    getMock.mockResolvedValue({ correction: state({ status: 'failed', attemptsUsed: 2, errorMessage: 'AI 校正失敗，請稍後再試' }) })
    render(<CorrectionPanel meeting={meeting()} onChanged={() => {}} />)
    expect(await screen.findByRole('button', { name: 'AI 校正' })).toBeDisabled()
    expect(screen.getByText(/AI 校正 2\/2/)).toBeInTheDocument()
    expect(screen.getByText(/AI 校正失敗，請稍後再試/)).toBeInTheDocument()
  })

  it('進行中顯示校正中且按鈕停用', async () => {
    getMock.mockResolvedValue({ correction: state({ status: 'running', attemptsUsed: 1 }) })
    render(<CorrectionPanel meeting={meeting(0.2)} onChanged={() => {}} />)
    expect(await screen.findByText(/校正中/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'AI 校正' })).toBeDisabled()
    expect(screen.queryByText(/建議校正/)).not.toBeInTheDocument()
  })

  // 稽核 F7：還原不可復原（次數不退），需先確認
  it('completed 時可還原：先確認，確認後才呼叫 API 並通知父層重抓', async () => {
    getMock.mockResolvedValue({ correction: state({ status: 'completed', attemptsUsed: 1, canRevert: true }) })
    revertMock.mockResolvedValue({ correction: state({ status: 'reverted', attemptsUsed: 1 }) })
    const onChanged = vi.fn()
    render(<CorrectionPanel meeting={meeting()} onChanged={onChanged} />)
    await userEvent.click(await screen.findByRole('button', { name: '還原' }))
    expect(screen.getByText(/已用掉的校正次數不會退回/)).toBeInTheDocument()
    expect(revertMock).not.toHaveBeenCalled()
    await userEvent.click(screen.getByRole('button', { name: '確認還原' }))
    expect(revertMock).toHaveBeenCalledWith('tok', 'm1')
    expect(onChanged).toHaveBeenCalled()
  })

  it('按下校正先顯示確認文案（含失敗也計次），確認後才呼叫 API', async () => {
    getMock.mockResolvedValue({ correction: state() })
    startMock.mockResolvedValue({ correction: state({ status: 'queued', attemptsUsed: 1 }) })
    render(<CorrectionPanel meeting={meeting()} onChanged={() => {}} />)
    await userEvent.click(await screen.findByRole('button', { name: 'AI 校正' }))
    expect(screen.getByText(/每場最多 2 次，失敗也會計次/)).toBeInTheDocument()
    expect(startMock).not.toHaveBeenCalled()
    await userEvent.click(screen.getByRole('button', { name: '開始校正' }))
    expect(startMock).toHaveBeenCalledWith('tok', 'm1')
    expect(await screen.findByText(/校正中/)).toBeInTheDocument()
  })

  it('紀錄未完成時不渲染', async () => {
    getMock.mockResolvedValue({ correction: state() })
    const { container } = render(<CorrectionPanel meeting={{ ...meeting(), status: 'analyzing' } as MeetingDetail} onChanged={() => {}} />)
    await Promise.resolve()
    expect(container).toBeEmptyDOMElement()
  })

  // 審查 I3：開始/還原失敗要有提示，並重抓狀態（例如另一台裝置已開始校正）
  it('開始校正失敗時顯示錯誤、關閉確認並重抓狀態', async () => {
    getMock
      .mockResolvedValueOnce({ correction: state() })
      .mockResolvedValueOnce({ correction: state({ status: 'running', attemptsUsed: 1 }) })
    startMock.mockRejectedValue(new Error('correction already in progress'))
    render(<CorrectionPanel meeting={meeting()} onChanged={() => {}} />)
    await userEvent.click(await screen.findByRole('button', { name: 'AI 校正' }))
    await userEvent.click(screen.getByRole('button', { name: '開始校正' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('correction already in progress')
    expect(screen.queryByRole('button', { name: '開始校正' })).not.toBeInTheDocument()
    expect(await screen.findByText(/校正中/)).toBeInTheDocument()
    expect(getMock).toHaveBeenCalledTimes(2)
  })

  it('還原失敗時顯示錯誤', async () => {
    getMock.mockResolvedValue({ correction: state({ status: 'completed', attemptsUsed: 1, canRevert: true }) })
    revertMock.mockRejectedValue(new Error('nothing to revert'))
    const onChanged = vi.fn()
    render(<CorrectionPanel meeting={meeting()} onChanged={onChanged} />)
    await userEvent.click(await screen.findByRole('button', { name: '還原' }))
    await userEvent.click(screen.getByRole('button', { name: '確認還原' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('nothing to revert')
    expect(onChanged).not.toHaveBeenCalled()
  })
})
