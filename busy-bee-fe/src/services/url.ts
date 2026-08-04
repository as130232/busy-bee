// 私有／保留網段主機名（阻擋 SSRF 第一層；主防線仍在後端抓取端）。
const PRIVATE_HOST =
  /^(?:localhost|0\.0\.0\.0|127\.|10\.|192\.168\.|169\.254\.|172\.(?:1[6-9]|2\d|3[01])\.|\[?::1\]?$)/i

/**
 * 從文字中抽出第一個 http(s) 連結；分享來源常是「看看這部影片 https://…」的整段文字。
 * 通過協定白名單（http/https）、長度上限與私有網段阻擋才回傳，否則回空字串。
 */
export function extractURL(text: string): string {
  const raw = (text ?? '').trim()
  if (!raw || raw.length > 2048) return ''

  let candidate = ''
  const m = raw.match(/https?:\/\/[^\s]+/i)
  if (m) {
    candidate = m[0]
  } else if (/^[\w-]+(\.[\w-]+)+\/\S*$/.test(raw)) {
    // 純網址但沒帶協定（如 youtu.be/xxx）→ 補 https
    candidate = `https://${raw}`
  }
  if (!candidate) return ''

  try {
    const u = new URL(candidate)
    if (u.protocol !== 'http:' && u.protocol !== 'https:') return ''
    if (PRIVATE_HOST.test(u.hostname)) return ''
    return u.toString()
  } catch {
    return ''
  }
}
