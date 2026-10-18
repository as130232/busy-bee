import { scenarioLabels, type Scenario } from '../services/api/client'
import { scenarioThemes } from './scenarioTheme'

const order: Scenario[] = ['meeting', 'casual', 'interview', 'idea']

/** 情境分段切換（會議 / 閒聊 / 面試 / 想法）；高度與語言選單一致（h-11）。 */
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
      className="inline-flex h-11 rounded-full border border-border bg-surface p-1 text-sm"
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
            className={`min-w-0 rounded-full px-4 font-semibold transition-colors outline-none focus-visible:ring-2 focus-visible:ring-accent/50 disabled:opacity-50 ${
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
