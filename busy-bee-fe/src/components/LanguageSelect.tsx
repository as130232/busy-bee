import { languageLabels, type Language } from '../services/api/client'

const order: Language[] = ['zh-TW', 'en-US', 'auto']

/** 語音辨識語言下拉選單（中文/英文/自動偵測）。 */
export function LanguageSelect({
  value,
  onChange,
  disabled = false,
}: {
  value: Language
  onChange: (l: Language) => void
  disabled?: boolean
}) {
  return (
    <select
      aria-label="語音辨識語言"
      className="h-10 rounded-full border border-border bg-surface px-3 text-sm font-medium text-fg disabled:opacity-50"
      value={value}
      disabled={disabled}
      onChange={(e) => onChange(e.target.value as Language)}
    >
      {order.map((l) => (
        <option key={l} value={l}>
          {languageLabels[l]}
        </option>
      ))}
    </select>
  )
}
