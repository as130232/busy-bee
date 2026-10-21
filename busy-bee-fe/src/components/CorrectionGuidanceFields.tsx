import { Plus, X } from 'lucide-react'

import { MAX_GUIDANCE_NOTE, MAX_GUIDANCE_RULES, type GuidanceDraft } from './correctionGuidance'

/** AI 校正確認視窗內的選填補充：錯字對照（先直接套用）＋補充說明（只給校正用，不寫進報告）。 */
export function CorrectionGuidanceFields({
  draft,
  disabled,
  onChange,
}: {
  draft: GuidanceDraft
  disabled: boolean
  onChange: (d: GuidanceDraft) => void
}) {
  const setRow = (i: number, patch: Partial<GuidanceDraft['rows'][number]>) =>
    onChange({ ...draft, rows: draft.rows.map((r, j) => (j === i ? { ...r, ...patch } : r)) })

  return (
    <div className="flex flex-col gap-3 rounded-lg border border-border p-3">
      <p className="m-0 text-xs leading-5 text-muted">
        補充內容只用來幫 AI 辨認專有名詞、人名與理解上下文，不會寫進報告。錯字對照會先直接套用（你手動改過的段落除外）。
      </p>
      <div className="flex flex-col gap-2">
        <span className="text-xs font-medium">錯字對照</span>
        {draft.rows.map((r, i) => (
          <div key={i} className="flex items-center gap-2">
            <input
              className="input h-9 min-w-0 flex-1"
              aria-label={`錯字 ${i + 1}`}
              placeholder="辨識錯的，如 DBA"
              value={r.from}
              disabled={disabled}
              onChange={(e) => setRow(i, { from: e.target.value })}
            />
            <span className="text-muted">→</span>
            <input
              className="input h-9 min-w-0 flex-1"
              aria-label={`正確寫法 ${i + 1}`}
              placeholder="正確的，如 DVA"
              value={r.to}
              disabled={disabled}
              onChange={(e) => setRow(i, { to: e.target.value })}
            />
            {draft.rows.length > 1 && (
              <button
                type="button"
                className="rounded p-1 text-muted hover:text-fg"
                aria-label={`刪除第 ${i + 1} 組`}
                disabled={disabled}
                onClick={() => onChange({ ...draft, rows: draft.rows.filter((_, j) => j !== i) })}
              >
                <X className="size-4" />
              </button>
            )}
          </div>
        ))}
        {draft.rows.length < MAX_GUIDANCE_RULES && (
          <button
            type="button"
            className="inline-flex items-center gap-1 self-start text-xs text-accent"
            disabled={disabled}
            onClick={() => onChange({ ...draft, rows: [...draft.rows, { from: '', to: '' }] })}
          >
            <Plus className="size-3.5" />
            新增一組
          </button>
        )}
      </div>
      <div className="flex flex-col gap-1 text-xs font-medium">
        <span>補充說明</span>
        <textarea
          className="input min-h-20 py-2 text-sm font-normal"
          aria-label="補充說明"
          placeholder="例如：會議在討論 DVA 季報；講者 A 是王小明；10 分鐘後那段在講預算"
          maxLength={MAX_GUIDANCE_NOTE}
          value={draft.note}
          disabled={disabled}
          onChange={(e) => onChange({ ...draft, note: e.target.value })}
        />
        <span className="self-end font-normal text-muted tabular-nums">
          {draft.note.length}/{MAX_GUIDANCE_NOTE}
        </span>
      </div>
    </div>
  )
}
