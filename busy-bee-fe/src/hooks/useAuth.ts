import { createContext, useContext } from 'react'

import type { User } from '../services/api/client'

export interface AuthState {
  /** Firebase 登入狀態尚未確定時為 true（避免閃現登入頁） */
  initializing: boolean
  /** 已通過後端同步（含白名單檢查）的用戶；null = 未登入 */
  user: User | null
  error: string | null
  signIn: () => Promise<void>
  signOut: () => Promise<void>
}

export const AuthContext = createContext<AuthState | null>(null)

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
