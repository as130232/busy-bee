// AI 校正前的選填補充（錯字對照＋補充說明）：草稿型別與轉成 API 參數的驗證。
import type { CorrectionGuidance } from '../services/api/client'

/** 與後端上限一致。 */
export const MAX_GUIDANCE_RULES = 20
export const MAX_GUIDANCE_NOTE = 1000

export interface GuidanceDraft {
  rows: { from: string; to: string }[]
  note: string
}

export const emptyGuidanceDraft = (): GuidanceDraft => ({ rows: [{ from: '', to: '' }], note: '' })

/**
 * 草稿 → API 參數：略過兩邊皆空的列；只填一邊回傳錯誤訊息。
 * 沒有任何補充時回傳 undefined（API 不帶 body，與以前相同）。
 */
export function toGuidance(d: GuidanceDraft): { guidance?: CorrectionGuidance; error?: string } {
  const replacements: CorrectionGuidance['replacements'] = []
  for (const r of d.rows) {
    const from = r.from.trim()
    const to = r.to.trim()
    if (!from && !to) continue
    if (!from || !to) return { error: '錯字對照請兩邊都填，或清空該列' }
    if (from === to) return { error: `「${from}」兩邊相同，請修改或清空該列` }
    replacements.push({ from, to })
  }
  const note = d.note.trim()
  if (replacements.length === 0 && !note) return {}
  return { guidance: { replacements, note } }
}
