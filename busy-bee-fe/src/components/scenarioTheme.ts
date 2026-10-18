import type { Scenario } from '../services/api/client'

/**
 * 每個情境一組配色，統一驅動錄音頁的情境切換、錄音大鈕、聲波環、光暈與背景暈染。
 * 會議 = 琥珀、閒聊 = 天藍、面試 = 翠綠（emerald）。
 * Record<Scenario, …> 讓後端 enum 新增情境時，型別會強制在此補齊配色，不會漏掉。
 *
 * 註：所有值都是完整的 Tailwind class 字串，確保 JIT 掃描得到（勿改成動態拼接色名）。
 */
export interface ScenarioTheme {
  /** 聲波環邊框色 */
  ring: string
  /** 錄音鈕外的呼吸光暈 */
  glow: string
  /** 整頁背景暈染（大範圍柔光） */
  tint: string
  /** 錄音鈕漸層 + 陰影色 */
  button: string
  /** 錄音鈕 hover 陰影色 */
  buttonHover: string
  /** 環繞光點（亮/中/淡） */
  dotBright: string
  dotSoft: string
  dotFaint: string
  /** 情境切換選中態 */
  toggleActive: string
  /** 深連結高亮環 */
  highlightRing: string
  /** 列表/登入頁的情境小標籤（文字色需在亮色底上達 4.5:1） */
  chip: string
  /** 列表的情境色點 */
  dot: string
}

export const scenarioThemes: Record<Scenario, ScenarioTheme> = {
  meeting: {
    ring: 'border-amber-400/40',
    glow: 'bg-amber-400/25',
    tint: 'bg-amber-500/15',
    button: 'from-amber-400 to-amber-500 shadow-amber-400/60',
    buttonHover: 'group-hover:shadow-amber-400/80',
    dotBright: 'bg-amber-300 shadow-amber-300',
    dotSoft: 'bg-amber-400/70',
    dotFaint: 'bg-amber-300/60',
    toggleActive: 'bg-amber-400 text-zinc-900',
    highlightRing: 'ring-amber-400',
    chip: 'bg-accent/10 text-accent-fg',
    dot: 'bg-amber-500',
  },
  casual: {
    ring: 'border-sky-400/40',
    glow: 'bg-sky-400/25',
    tint: 'bg-sky-500/15',
    button: 'from-sky-400 to-sky-500 shadow-sky-400/60',
    buttonHover: 'group-hover:shadow-sky-400/80',
    dotBright: 'bg-sky-300 shadow-sky-300',
    dotSoft: 'bg-sky-400/70',
    dotFaint: 'bg-sky-300/60',
    toggleActive: 'bg-sky-400 text-zinc-900',
    highlightRing: 'ring-sky-400',
    chip: 'bg-sky-500/10 text-sky-700 dark:text-sky-400',
    dot: 'bg-sky-500',
  },
  interview: {
    ring: 'border-emerald-400/40',
    glow: 'bg-emerald-400/25',
    tint: 'bg-emerald-500/15',
    button: 'from-emerald-400 to-emerald-500 shadow-emerald-400/60',
    buttonHover: 'group-hover:shadow-emerald-400/80',
    dotBright: 'bg-emerald-300 shadow-emerald-300',
    dotSoft: 'bg-emerald-400/70',
    dotFaint: 'bg-emerald-300/60',
    toggleActive: 'bg-emerald-400 text-zinc-900',
    highlightRing: 'ring-emerald-400',
    chip: 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-400',
    dot: 'bg-emerald-500',
  },
  idea: {
    ring: 'border-violet-400/40',
    glow: 'bg-violet-400/25',
    tint: 'bg-violet-500/15',
    button: 'from-violet-400 to-violet-500 shadow-violet-400/60',
    buttonHover: 'group-hover:shadow-violet-400/80',
    dotBright: 'bg-violet-300 shadow-violet-300',
    dotSoft: 'bg-violet-400/70',
    dotFaint: 'bg-violet-300/60',
    toggleActive: 'bg-violet-400 text-zinc-900',
    highlightRing: 'ring-violet-400',
    chip: 'bg-violet-500/10 text-violet-700 dark:text-violet-400',
    dot: 'bg-violet-500',
  },
}
