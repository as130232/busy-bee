/** 推播點擊的導向網址：只允許本站（回傳 path+query+hash），其他一律回首頁。 */
export function sameOriginPath(url: string | undefined, origin: string): string {
  if (!url) return '/'
  try {
    const u = new URL(url, origin)
    return u.origin === origin ? `${u.pathname}${u.search}${u.hash}` : '/'
  } catch {
    return '/'
  }
}
