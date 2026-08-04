import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { SummarySections } from './SummarySections'

type Section = Parameters<typeof SummarySections>[0]['sections'][number]

const sections: Section[] = [
  { type: 'topic', title: '重點', items: [{ text: '要點一' }, { heading: '決議', text: '上線', speaker: 'A' }] },
  { type: 'empty', title: '空區塊', items: [] },
]

describe('SummarySections', () => {
  it('渲染有內容的區塊標題與條列，隱藏空區塊', () => {
    render(<SummarySections sections={sections} speakerOrder={['A']} speakerNames={{ A: 'Ben' }} />)

    expect(screen.getByText('重點')).toBeInTheDocument()
    expect(screen.getByText('要點一')).toBeInTheDocument()
    expect(screen.getByText('決議')).toBeInTheDocument()
    // 空 items 的區塊不應出現
    expect(screen.queryByText('空區塊')).not.toBeInTheDocument()
  })

  it('講者代號套用顯示名（A → Ben）', () => {
    render(<SummarySections sections={sections} speakerOrder={['A']} speakerNames={{ A: 'Ben' }} />)
    expect(screen.getByText('Ben')).toBeInTheDocument()
  })

  it('全部區塊為空時不渲染任何內容', () => {
    const { container } = render(<SummarySections sections={[{ type: 'x', title: 't', items: [] }]} />)
    expect(container).toBeEmptyDOMElement()
  })
})
