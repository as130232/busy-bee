import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react'
import {
  onAuthStateChanged,
  signInWithPopup,
  signOut as firebaseSignOut,
  type User as FirebaseUser,
} from 'firebase/auth'

import { auth, googleProvider } from '../services/firebase'
import { syncUser, ApiError, type User } from '../services/api/client'

interface AuthState {
  /** Firebase 登入狀態尚未確定時為 true（避免閃現登入頁） */
  initializing: boolean
  /** 已通過後端同步（含白名單檢查）的用戶；null = 未登入 */
  user: User | null
  error: string | null
  signIn: () => Promise<void>
  signOut: () => Promise<void>
}

const AuthContext = createContext<AuthState | null>(null)

async function syncWithBackend(fbUser: FirebaseUser): Promise<User> {
  const idToken = await fbUser.getIdToken()
  return syncUser(idToken)
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [initializing, setInitializing] = useState(true)
  const [user, setUser] = useState<User | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    // 頁面重整後恢復登入狀態：Firebase 持久化 session，重新走一次後端同步
    const unsubscribe = onAuthStateChanged(auth, async (fbUser) => {
      try {
        setUser(fbUser ? await syncWithBackend(fbUser) : null)
        setError(null)
      } catch (e) {
        await firebaseSignOut(auth)
        setUser(null)
        setError(toMessage(e))
      } finally {
        setInitializing(false)
      }
    })
    return unsubscribe
  }, [])

  const signIn = useCallback(async () => {
    setError(null)
    try {
      const cred = await signInWithPopup(auth, googleProvider)
      setUser(await syncWithBackend(cred.user))
    } catch (e) {
      await firebaseSignOut(auth)
      setUser(null)
      setError(toMessage(e))
    }
  }, [])

  const signOut = useCallback(async () => {
    await firebaseSignOut(auth)
    setUser(null)
  }, [])

  const value = useMemo(
    () => ({ initializing, user, error, signIn, signOut }),
    [initializing, user, error, signIn, signOut],
  )
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

function toMessage(e: unknown): string {
  if (e instanceof ApiError) {
    if (e.errCode === 40301) return '此帳號沒有使用權限，請聯絡管理員加入白名單。'
    return `同步失敗（${e.errCode}）：${e.message}`
  }
  if (e instanceof Error && e.message.includes('popup-closed')) return ''
  return '登入失敗，請再試一次。'
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
