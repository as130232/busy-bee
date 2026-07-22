import { scenarioLabels, type Scenario } from '../services/api/client'
import { scenarioThemes } from './scenarioTheme'

const order: Scenario[] = ['meeting', 'casual', 'interview']

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
      className="inline-flex rounded-full border border-border bg-surface p-1 text-base"
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
            className={`min-w-24 rounded-full px-6 py-2.5 font-semibold transition-colors disabled:opacity-50 ${
              active ? `${scenarioThemes[s].toggleActive} shadow-sm` : 'text-muted'
            }`}
          >
            {scenarioLabels[s]}
          </button>
        )
      })}
    </div>
  )
}
