import { Navigate } from 'react-router-dom'
import type { ReactNode } from 'react'

import { useAuth } from '../hooks/useAuth'

export function RequireAuth({ children }: { children: ReactNode }) {
  const { user, initializing } = useAuth()

  if (initializing) return <main className="center">載入中…</main>
  if (!user) return <Navigate to="/login" replace />
  return children
}
