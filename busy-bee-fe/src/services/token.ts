import { auth } from './firebase'

/** 取得當前登入者的 ID token；未登入時丟錯（RequireAuth 已保證不會發生）。 */
export async function getIdToken(): Promise<string> {
  const user = auth.currentUser
  if (!user) throw new Error('not signed in')
  return user.getIdToken()
}
