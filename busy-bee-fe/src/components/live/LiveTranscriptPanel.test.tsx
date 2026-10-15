import { act, fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import type { TranscriptLine } from '../../live/session'
import { LiveTranscriptPanel } from './LiveTranscriptPanel'

const at = new Date(2026, 9, 15, 14, 3).getTime()
const lines: TranscriptLine[] = [
  { id: 1, at, who: 'remote', text: '週三上線' },
  { id: 2, at, who: 'local', text: '了解' },
  { id: 3, at, who: 'remote', text: '（斷線 5 秒，正式版會補上）', gap: true },
]

function scrollState(el: HTMLElement, { top, height, client }: { top: number; height: number; client: number }) {
  Object.defineProperty(el, 'scrollHeight', { configurable: true, value: height })
  Object.defineProperty(el, 'clientHeight', { configurable: true, value: client })
  el.scrollTop = top
}

describe('LiveTranscriptPanel', () => {
  it('顯示時間、講者與暫存列', () => {
    render(<LiveTranscriptPanel lines={lines} interim={{ remote: '還在講', local: '' }} />)
    expect(screen.getAllByText('14:03')).toHaveLength(2)
    expect(screen.getAllByText('對方')).toHaveLength(2) // 一列 final ＋ 一列暫存
    expect(screen.getByText('我')).toBeInTheDocument()
    expect(screen.getByText('（斷線 5 秒，正式版會補上）')).toBeInTheDocument()
    expect(screen.getByText('還在講')).toBeInTheDocument()
  })

  it('沒有內容時顯示等待提示', () => {
    render(<LiveTranscriptPanel lines={[]} interim={{ remote: '', local: '' }} />)
    expect(screen.getByText(/等待對方說話/)).toBeInTheDocument()
  })

  it('往上捲時暫停自動捲動並出現「回到最新」，點擊後回到底部', () => {
    const { rerender } = render(<LiveTranscriptPanel lines={lines.slice(0, 1)} interim={{ remote: '', local: '' }} />)
    const box = screen.getByTestId('live-transcript')
    scrollState(box, { top: 0, height: 1000, client: 300 })
    fireEvent.scroll(box)
    rerender(<LiveTranscriptPanel lines={lines} interim={{ remote: '', local: '' }} />)
    expect(box.scrollTop).toBe(0) // 使用者在看舊內容，不被拉走
    const btn = screen.getByRole('button', { name: '回到最新' })
    act(() => btn.click())
    expect(box.scrollTop).toBe(1000)
    expect(screen.queryByRole('button', { name: '回到最新' })).toBeNull()
  })
})
