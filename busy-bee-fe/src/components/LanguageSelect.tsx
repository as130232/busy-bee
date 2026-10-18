import { ChevronDown } from 'lucide-react'

import { languageLabels, type Language } from '../services/api/client'

const order: Language[] = ['zh-TW', 'en-US', 'auto']

/** 語音辨識語言下拉選單（中文/英文/自動偵測）；膠囊外觀與情境切換同高，保留原生 select 的行動端選單。 */
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
    <span className="relative inline-flex">
      <select
        aria-label="語音辨識語言"
        className="h-11 cursor-pointer appearance-none rounded-full border border-border bg-surface pr-8 pl-4 text-sm font-semibold text-fg transition outline-none hover:bg-surface-hover focus-visible:ring-2 focus-visible:ring-accent/50 disabled:opacity-50"
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
      <ChevronDown className="pointer-events-none absolute top-1/2 right-3 size-4 -translate-y-1/2 text-muted" />
    </span>
  )
}
