/** 從文字中抽出第一個 http(s) 連結；分享來源常是「看看這部影片 https://…」的整段文字。 */
export function extractURL(text: string): string {
  const raw = (text ?? '').trim()
  if (!raw) return ''
  const m = raw.match(/https?:\/\/[^\s]+/i)
  if (m) return m[0]
  // 純網址但沒帶協定（如 youtu.be/xxx）→ 補 https
  if (/^[\w-]+(\.[\w-]+)+\/\S*$/.test(raw)) return `https://${raw}`
  return ''
}
