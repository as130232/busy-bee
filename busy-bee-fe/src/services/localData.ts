/** 跨會議問答的上次結果（含會議內容片段，屬帳號資料）。 */
export const QA_STORAGE_KEY = 'busybee.qa.last'

/** 登出/授權失效時清除本機的帳號資料；主題等裝置偏好保留。 */
export function clearLocalUserData(): void {
  try {
    localStorage.removeItem(QA_STORAGE_KEY)
  } catch {
    // localStorage 不可用（隱私模式等）時無資料可清
  }
}
