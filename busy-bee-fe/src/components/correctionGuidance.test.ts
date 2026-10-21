import { describe, expect, it } from 'vitest'

import { toGuidance } from './correctionGuidance'

describe('toGuidance', () => {
  it('沒有任何補充時不帶 guidance', () => {
    expect(toGuidance({ rows: [{ from: ' ', to: '' }], note: '  ' })).toEqual({})
  })
  it('略過空列、去空白', () => {
    expect(toGuidance({ rows: [{ from: ' DBA ', to: 'DVA' }, { from: '', to: '' }], note: ' 說明 ' })).toEqual({
      guidance: { replacements: [{ from: 'DBA', to: 'DVA' }], note: '說明' },
    })
  })
  it('兩邊相同或只填一邊回錯誤', () => {
    expect(toGuidance({ rows: [{ from: 'DBA', to: 'DBA' }], note: '' }).error).toMatch(/兩邊相同/)
    expect(toGuidance({ rows: [{ from: '', to: 'DVA' }], note: '' }).error).toMatch(/兩邊都填/)
  })
})
