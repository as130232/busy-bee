import { scenarioLabels, type Scenario } from '../services/api/client'

const order: Scenario[] = ['meeting', 'casual']

/** 情境分段切換（會議 / 閒聊）；行動優先，滿版可點區塊。 */
export function ScenarioToggle({
  value,
  onChange,
  disabled = false,
}: {
  value: Scenario
  onChange: (s: Scenario) => void
  disabled?: boolean
}) {
  return (
    <div
      role="radiogroup"
      aria-label="紀錄情境"
      className="inline-flex rounded-full bg-surface-2 p-1 text-sm"
    >
      {order.map((s) => {
        const active = s === value
        return (
          <button
            key={s}
            type="button"
            role="radio"
            aria-checked={active}
            disabled={disabled}
            onClick={() => onChange(s)}
            className={`min-w-16 rounded-full px-4 py-1.5 font-medium transition-colors disabled:opacity-50 ${
              active ? 'bg-accent text-accent-fg shadow-sm' : 'text-muted'
            }`}
          >
            {scenarioLabels[s]}
          </button>
        )
      })}
    </div>
  )
}
