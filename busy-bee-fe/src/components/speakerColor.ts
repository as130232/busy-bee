// 講者晶片配色：依首次出現順序輪替，逐字稿與摘要卡片共用同一套色，確保同一講者同色。

export const SPEAKER_COLORS = [
  'bg-blue-500/15 text-blue-600 dark:text-blue-400',
  'bg-emerald-500/15 text-emerald-600 dark:text-emerald-400',
  'bg-amber-500/15 text-amber-600 dark:text-amber-400',
  'bg-fuchsia-500/15 text-fuchsia-600 dark:text-fuchsia-400',
  'bg-rose-500/15 text-rose-600 dark:text-rose-400',
  'bg-cyan-500/15 text-cyan-600 dark:text-cyan-400',
]

// speakerColor 依講者代號在 order 中的位置取色；未知代號回退第一色。
export function speakerColor(code: string, order: string[]): string {
  const i = order.indexOf(code)
  return SPEAKER_COLORS[(i < 0 ? 0 : i) % SPEAKER_COLORS.length]
}
