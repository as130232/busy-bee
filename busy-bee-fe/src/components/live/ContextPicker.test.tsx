import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import { ContextPicker } from './ContextPicker'

describe('ContextPicker', () => {
  it('貼上文字後送出（標題選填）', async () => {
    const onAddText = vi.fn()
    render(<ContextPicker items={[]} onAddText={onAddText} onAddFile={vi.fn()} />)
    await userEvent.click(screen.getByRole('button', { name: '貼上文字' }))
    await userEvent.type(screen.getByLabelText('資料內容'), '上線流程')
    await userEvent.click(screen.getByRole('button', { name: '加入' }))
    expect(onAddText).toHaveBeenCalledWith({ title: '', text: '上線流程' })
  })

  it('空白內容不能送出', async () => {
    render(<ContextPicker items={[]} onAddText={vi.fn()} onAddFile={vi.fn()} />)
    await userEvent.click(screen.getByRole('button', { name: '貼上文字' }))
    expect(screen.getByRole('button', { name: '加入' })).toBeDisabled()
  })

  it('選檔只接受 txt/md/pdf，逐一回呼', async () => {
    const onAddFile = vi.fn()
    render(<ContextPicker items={[]} onAddText={vi.fn()} onAddFile={onAddFile} />)
    const input = screen.getByLabelText('上傳檔案') as HTMLInputElement
    expect(input.accept).toBe('.txt,.md,.pdf')
    const files = [new File(['a'], 'a.md'), new File(['b'], 'b.pdf')]
    await userEvent.upload(input, files)
    expect(onAddFile.mock.calls.map((c) => (c[0] as File).name)).toEqual(['a.md', 'b.pdf'])
  })

  it('列出已加入的資料並可移除', async () => {
    const onRemove = vi.fn()
    render(
      <ContextPicker
        items={[{ key: 'k1', title: '規格.pdf', detail: '1,200 字' }]}
        onAddText={vi.fn()}
        onAddFile={vi.fn()}
        onRemove={onRemove}
      />,
    )
    expect(screen.getByText('規格.pdf')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: '移除「規格.pdf」' }))
    expect(onRemove).toHaveBeenCalledWith('k1')
  })
})
